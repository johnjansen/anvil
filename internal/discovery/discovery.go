package discovery

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Message types for cluster communication
const (
	MsgTypeJoin    = "join"
	MsgTypeLeave   = "leave"
	MsgTypeAlive   = "alive"
)

// NodeInfo represents a discovered daemon node
type NodeInfo struct {
	NodeID        string    `json:"node_id"`
	AdvertiseAddr string    `json:"advertise_addr"`
	Hostname      string    `json:"hostname"`
	LastSeen      time.Time `json:"last_seen"`
	Status        string    `json:"status"` // "alive", "suspect", "dead"
}

// Message is the wire format for discovery messages
type Message struct {
	Type      string    `json:"type"`
	NodeID    string    `json:"node_id"`
	Timestamp time.Time `json:"timestamp"`
	Payload   NodeInfo  `json:"payload"`
}

// Discovery handles multicast-based daemon discovery
type Discovery struct {
	config       Config
	nodeID       string
	localAddr    string
	members      map[string]*NodeInfo
	membersMu    sync.RWMutex
	conn         *net.UDPConn
	listener     *net.UDPConn
	running      bool
	runningMu    sync.Mutex
	stopChan     chan struct{}
	doneChan     chan struct{}
}

// Config holds discovery configuration
type Config struct {
	MulticastAddr string // multicast address (e.g., "239.255.0.1:9091")
	MulticastIface string // network interface name (empty = auto-detect)
	StaticHosts   []string // static list of hosts as alternative to multicast
	NodeID       string   // unique node identifier
	AdvertiseAddr string  // address to advertise to other nodes
}

// New creates a new Discovery instance
func New(cfg Config) *Discovery {
	if cfg.NodeID == "" {
		cfg.NodeID = uuid.New().String()
	}
	return &Discovery{
		config:    cfg,
		nodeID:    cfg.NodeID,
		localAddr: cfg.AdvertiseAddr,
		members:   make(map[string]*NodeInfo),
		stopChan:  make(chan struct{}),
		doneChan:  make(chan struct{}),
	}
}

// Start begins the discovery process
func (d *Discovery) Start() error {
	d.runningMu.Lock()
	if d.running {
		d.runningMu.Unlock()
		return nil
	}
	d.running = true
	d.runningMu.Unlock()

	// Start multicast listener
	if err := d.startMulticastListener(); err != nil {
		d.running = false
		return fmt.Errorf("failed to start multicast listener: %w", err)
	}

	// Start static hosts connections if configured
	if len(d.config.StaticHosts) > 0 {
		go d.pollStaticHosts()
	}

	// Announce our presence
	go d.announceLoop()

	// Start cleanup of stale members
	go d.cleanupLoop()

	return nil
}

// Stop halts discovery
func (d *Discovery) Stop() error {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()
	if !d.running {
		return nil
	}

	close(d.stopChan)

	if d.listener != nil {
		d.listener.Close()
	}
	if d.conn != nil {
		d.conn.Close()
	}

	<-d.doneChan
	d.running = false
	return nil
}

// startMulticastListener sets up the UDP multicast listener
func (d *Discovery) startMulticastListener() error {
	addr, err := net.ResolveUDPAddr("udp", d.config.MulticastAddr)
	if err != nil {
		return fmt.Errorf("resolve multicast addr: %w", err)
	}

	// Determine interface to use
	var iface *net.Interface
	if d.config.MulticastIface != "" {
		iface, err = net.InterfaceByName(d.config.MulticastIface)
		if err != nil {
			return fmt.Errorf("get interface %s: %w", d.config.MulticastIface, err)
		}
	} else {
		// Use default interface
		ifaces, err := net.Interfaces()
		if err != nil {
			return fmt.Errorf("list interfaces: %w", err)
		}
		for _, i := range ifaces {
			if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagMulticast == 0 {
				continue
			}
			iface = &i
			break
		}
	}

	if iface == nil {
		return fmt.Errorf("no suitable network interface found")
	}

	// Listen on multicast group
	listener, err := net.ListenMulticastUDP("udp", iface, addr)
	if err != nil {
		return fmt.Errorf("listen on multicast: %w", err)
	}
	d.listener = listener

	// Set up writer connection
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		listener.Close()
		return fmt.Errorf("dial udp: %w", err)
	}
	d.conn = conn

	// Start receiving messages
	go d.receiveLoop()

	return nil
}

// receiveLoop processes incoming discovery messages
func (d *Discovery) receiveLoop() {
	defer func() {
		close(d.doneChan)
	}()

	buf := make([]byte, 4096)
	for {
		select {
		case <-d.stopChan:
			return
		default:
		}

		d.listener.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, src, err := d.listener.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(buf[:n], &msg); err != nil {
			continue
		}

		// Ignore our own messages
		if msg.NodeID == d.nodeID {
			continue
		}

		d.handleMessage(msg, src)
	}
}

// handleMessage processes a single discovery message
func (d *Discovery) handleMessage(msg Message, src *net.UDPAddr) {
	d.membersMu.Lock()
	defer d.membersMu.Unlock()

	switch msg.Type {
	case MsgTypeJoin, MsgTypeAlive:
		node := msg.Payload
		node.LastSeen = time.Now()
		node.Status = "alive"
		if node.AdvertiseAddr == "" {
			node.AdvertiseAddr = src.IP.String()
		}
		d.members[msg.NodeID] = &node

	case MsgTypeLeave:
		delete(d.members, msg.NodeID)
	}
}

// announceLoop periodically sends our presence
func (d *Discovery) announceLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopChan:
			return
		case <-ticker.C:
			d.sendMessage(MsgTypeAlive)
		}
	}
}

// sendMessage broadcasts a discovery message
func (d *Discovery) sendMessage(msgType string) {
	if d.conn == nil {
		return
	}

	msg := Message{
		Type:      msgType,
		NodeID:    d.nodeID,
		Timestamp: time.Now(),
		Payload: NodeInfo{
			NodeID:        d.nodeID,
			AdvertiseAddr: d.localAddr,
			Hostname:      getHostname(),
			LastSeen:      time.Now(),
			Status:        "alive",
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	d.conn.Write(data)
}

// pollStaticHosts checks connectivity to static hosts
func (d *Discovery) pollStaticHosts() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopChan:
			return
		case <-ticker.C:
			d.checkStaticHosts()
		}
	}
}

// checkStaticHosts attempts to connect to static hosts
func (d *Discovery) checkStaticHosts() {
	d.membersMu.Lock()
	defer d.membersMu.Unlock()

	now := time.Now()
	for _, host := range d.config.StaticHosts {
		// Simple TCP check - could be enhanced with HTTP health endpoint
		conn, err := net.DialTimeout("tcp", host, 2*time.Second)
		if err != nil {
			// Mark as suspect/dead
			if node, ok := d.members[host]; ok {
				node.LastSeen = now
				node.Status = "dead"
			}
			continue
		}
		conn.Close()

		// Update member status
		d.members[host] = &NodeInfo{
			NodeID:        host,
			AdvertiseAddr: host,
			Hostname:      host,
			LastSeen:      now,
			Status:        "alive",
		}
	}
}

// cleanupLoop removes stale members
func (d *Discovery) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopChan:
			return
		case <-ticker.C:
			d.cleanupStale()
		}
	}
}

// cleanupStale removes nodes that haven't been seen recently
func (d *Discovery) cleanupStale() {
	d.membersMu.Lock()
	defer d.membersMu.Unlock()

	now := time.Now()
	staleThreshold := 30 * time.Second

	for id, node := range d.members {
		if now.Sub(node.LastSeen) > staleThreshold {
			delete(d.members, id)
		}
	}
}

// GetMembers returns the current list of cluster members
func (d *Discovery) GetMembers() []NodeInfo {
	d.membersMu.RLock()
	defer d.membersMu.RUnlock()

	result := make([]NodeInfo, 0, len(d.members))
	for _, node := range d.members {
		result = append(result, *node)
	}
	return result
}

// GetMember returns a specific member by ID
func (d *Discovery) GetMember(nodeID string) (NodeInfo, bool) {
	d.membersMu.RLock()
	defer d.membersMu.RUnlock()

	node, ok := d.members[nodeID]
	if !ok {
		return NodeInfo{}, false
	}
	return *node, true
}

// GetNodeID returns the local node ID
func (d *Discovery) GetNodeID() string {
	return d.nodeID
}

// IsRunning returns whether discovery is active
func (d *Discovery) IsRunning() bool {
	d.runningMu.Lock()
	defer d.runningMu.Unlock()
	return d.running
}

// AnnounceLeave sends a leave message and stops discovery
func (d *Discovery) AnnounceLeave() {
	d.sendMessage(MsgTypeLeave)
	d.Stop()
}

func getHostname() string {
	name, _ := net.LookupAddr("")
	if len(name) > 0 {
		return name[0]
	}
	return "unknown"
}
