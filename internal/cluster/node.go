package cluster

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Node represents this daemon's participation in the cluster.
type Node struct {
	id        string
	config    Config
	transport *Transport
	state     ElectionState
	stateMu   sync.RWMutex

	peers        []string
	peerLastSeen map[string]time.Time
	peersMu      sync.RWMutex

	electionTimer *time.Timer
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup

	// Logging callback (set by daemon)
	Logf func(format string, args ...any)

	// Worker availability per peer (updated from heartbeat acks)
	peerWorkers   map[string]int // nodeID -> idle worker count
	peerWorkersMu sync.RWMutex

	// Task distribution callbacks (set by daemon)
	WorkerReportFn func() WorkerReport             // returns this node's worker availability
	OnTaskAssign   func(assignment TaskAssignment)  // called when follower receives task assignment
	OnTaskResult   func(result TaskResult)          // called when leader receives task result
}

// Config holds the cluster configuration for a Node.
type Config struct {
	Listen            string
	Peers             []string
	HeartbeatInterval time.Duration
	ElectionTimeout   time.Duration
}

// NewNode creates a new cluster node.
func NewNode(dataDir string, cfg Config) (*Node, error) {
	id, err := loadOrCreateNodeID(dataDir)
	if err != nil {
		return nil, err
	}

	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 5 * time.Second
	}
	if cfg.ElectionTimeout <= 0 {
		cfg.ElectionTimeout = 30 * time.Second
	}
	if cfg.Listen == "" {
		cfg.Listen = ":9091"
	}

	return &Node{
		id:           id,
		config:       cfg,
		transport:    NewTransport(cfg.Listen),
		peers:        cfg.Peers,
		peerLastSeen: make(map[string]time.Time),
		peerWorkers:  make(map[string]int),
		state: ElectionState{
			Role: Follower,
		},
		Logf: func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, "cluster: "+format+"\n", args...)
		},
	}, nil
}

// loadOrCreateNodeID loads or generates a persistent node ID.
func loadOrCreateNodeID(dataDir string) (string, error) {
	idPath := filepath.Join(dataDir, ".anvil", "node-id")

	data, err := os.ReadFile(idPath)
	if err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id, nil
		}
	}

	// Generate new UUID-like ID
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating node ID: %w", err)
	}
	id := hex.EncodeToString(b[:4]) + "-" + hex.EncodeToString(b[4:6]) + "-" + hex.EncodeToString(b[6:8])

	if err := os.MkdirAll(filepath.Dir(idPath), 0755); err != nil {
		return "", fmt.Errorf("creating .anvil dir: %w", err)
	}
	if err := os.WriteFile(idPath, []byte(id+"\n"), 0644); err != nil {
		return "", fmt.Errorf("writing node ID: %w", err)
	}

	return id, nil
}

// Start begins cluster participation.
func (n *Node) Start() error {
	n.ctx, n.cancel = context.WithCancel(context.Background())

	// Start transport
	if err := n.transport.Listen(); err != nil {
		return err
	}

	// Connect to peers
	for _, peer := range n.peers {
		go n.transport.Connect(peer)
	}

	// Start message handler
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		n.messageLoop()
	}()

	// Start election timer
	n.resetElectionTimer()

	n.Logf("node %s started as follower (peers: %d)", n.id, len(n.peers))
	return nil
}

// Stop gracefully shuts down cluster participation.
func (n *Node) Stop() {
	if n.cancel != nil {
		n.cancel()
	}
	if n.electionTimer != nil {
		n.electionTimer.Stop()
	}
	n.transport.Close()
	n.wg.Wait()
	n.Logf("node %s stopped", n.id)
}

// messageLoop processes incoming messages.
func (n *Node) messageLoop() {
	for {
		select {
		case <-n.ctx.Done():
			return
		case msg := <-n.transport.Receive():
			n.handleMessage(msg)
		}
	}
}

// handleMessage dispatches a message to the appropriate handler.
func (n *Node) handleMessage(msg Message) {
	// Update peer last seen
	n.peersMu.Lock()
	n.peerLastSeen[msg.FromID] = time.Now()
	n.peersMu.Unlock()

	// Step down if we see a higher term
	n.stateMu.RLock()
	currentTerm := n.state.CurrentTerm
	n.stateMu.RUnlock()

	if msg.Term > currentTerm {
		n.stepDown(msg.Term)
	}

	switch msg.Type {
	case MsgVoteRequest:
		n.handleVoteRequest(msg)
	case MsgVoteResponse:
		n.handleVoteResponse(msg)
	case MsgHeartbeat:
		n.handleHeartbeat(msg)
	case MsgHeartbeatAck:
		n.handleHeartbeatAck(msg)
	case MsgTaskAssign:
		n.handleTaskAssign(msg)
	case MsgTaskResult:
		n.handleTaskResult(msg)
	}
}

// IsLeader returns true if this node is the cluster leader.
func (n *Node) IsLeader() bool {
	n.stateMu.RLock()
	defer n.stateMu.RUnlock()
	return n.state.Role == Leader
}

// LeaderID returns the ID of the current known leader.
func (n *Node) LeaderID() string {
	n.stateMu.RLock()
	defer n.stateMu.RUnlock()
	return n.state.LeaderID
}

// ID returns this node's unique identifier.
func (n *Node) ID() string {
	return n.id
}

// Status returns the current cluster status for this node.
func (n *Node) Status() ClusterStatus {
	n.stateMu.RLock()
	role := n.state.Role
	term := n.state.CurrentTerm
	leaderID := n.state.LeaderID
	n.stateMu.RUnlock()

	n.peersMu.RLock()
	members := make([]MemberInfo, 0, len(n.peerLastSeen)+1)
	// Add self
	members = append(members, MemberInfo{
		ID:       n.id,
		Address:  n.config.Listen,
		Role:     role,
		LastSeen: time.Now(),
	})
	for id, lastSeen := range n.peerLastSeen {
		peerRole := Follower
		if id == leaderID {
			peerRole = Leader
		}
		members = append(members, MemberInfo{
			ID:       id,
			Role:     peerRole,
			LastSeen: lastSeen,
		})
	}
	n.peersMu.RUnlock()

	return ClusterStatus{
		NodeID:      n.id,
		Role:        role,
		Term:        term,
		LeaderID:    leaderID,
		ClusterSize: len(members),
		Members:     members,
	}
}

// handleTaskAssign processes a task assignment from the leader.
func (n *Node) handleTaskAssign(msg Message) {
	if n.OnTaskAssign == nil {
		return
	}
	var assignment TaskAssignment
	if err := json.Unmarshal(msg.Payload, &assignment); err != nil {
		n.Logf("failed to unmarshal task assignment: %v", err)
		return
	}
	n.Logf("received task assignment: %s (from leader %s)", assignment.TaskName, msg.FromID)
	n.OnTaskAssign(assignment)
}

// handleTaskResult processes a task result from a follower.
func (n *Node) handleTaskResult(msg Message) {
	if n.OnTaskResult == nil {
		return
	}
	var result TaskResult
	if err := json.Unmarshal(msg.Payload, &result); err != nil {
		n.Logf("failed to unmarshal task result: %v", err)
		return
	}
	n.Logf("received task result: %s from %s (success=%v)", result.TaskName, result.NodeID, result.Success)
	n.OnTaskResult(result)
}

// PeerWorkers returns the map of peer node IDs to their idle worker counts.
func (n *Node) PeerWorkers() map[string]int {
	n.peerWorkersMu.RLock()
	defer n.peerWorkersMu.RUnlock()
	result := make(map[string]int, len(n.peerWorkers))
	for k, v := range n.peerWorkers {
		result[k] = v
	}
	return result
}

// AssignTask sends a task assignment to a specific peer node.
func (n *Node) AssignTask(addr string, assignment TaskAssignment) error {
	payload, err := json.Marshal(assignment)
	if err != nil {
		return err
	}
	n.stateMu.RLock()
	term := n.state.CurrentTerm
	n.stateMu.RUnlock()

	return n.transport.Send(addr, Message{
		Type:    MsgTaskAssign,
		Term:    term,
		FromID:  n.id,
		ToID:    assignment.TargetNodeID,
		Payload: payload,
	})
}

// ReportResult sends a task result back to the leader.
func (n *Node) ReportResult(addr string, result TaskResult) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	n.stateMu.RLock()
	term := n.state.CurrentTerm
	n.stateMu.RUnlock()

	return n.transport.Send(addr, Message{
		Type:    MsgTaskResult,
		Term:    term,
		FromID:  n.id,
		Payload: payload,
	})
}

// PeerAddress returns the address for a given peer node ID (empty if unknown).
func (n *Node) PeerAddress(nodeID string) string {
	// For now, we use the peers list which is address-based
	// The mapping from ID to address is tracked via the transport
	n.peersMu.RLock()
	defer n.peersMu.RUnlock()
	// We need to iterate members to find the matching address
	// Since our peer list is addresses, we track id->addr mapping
	return ""
}

// PeerAddresses returns all known peer addresses.
func (n *Node) PeerAddresses() []string {
	return n.peers
}
