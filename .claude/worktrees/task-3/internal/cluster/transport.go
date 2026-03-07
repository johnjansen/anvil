package cluster

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// Transport handles TCP communication between cluster nodes.
type Transport struct {
	listener net.Listener
	addr     string

	peersMu sync.RWMutex
	peers   map[string]net.Conn // address -> connection

	incoming chan Message
	done     chan struct{}
}

// NewTransport creates a new cluster transport.
func NewTransport(addr string) *Transport {
	return &Transport{
		addr:     addr,
		peers:    make(map[string]net.Conn),
		incoming: make(chan Message, 256),
		done:     make(chan struct{}),
	}
}

// Listen starts accepting connections on the configured address.
func (t *Transport) Listen() error {
	ln, err := net.Listen("tcp", t.addr)
	if err != nil {
		return fmt.Errorf("cluster transport: listen on %s: %w", t.addr, err)
	}
	t.listener = ln

	go t.acceptLoop()
	return nil
}

// acceptLoop accepts incoming TCP connections.
func (t *Transport) acceptLoop() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-t.done:
				return
			default:
				continue
			}
		}
		go t.handleConn(conn)
	}
}

// handleConn reads messages from a connection.
func (t *Transport) handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)
	for scanner.Scan() {
		select {
		case <-t.done:
			return
		default:
		}
		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		select {
		case t.incoming <- msg:
		default:
			// Drop message if channel full
		}
	}
}

// Connect establishes a connection to a peer.
func (t *Transport) Connect(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return err
	}
	t.peersMu.Lock()
	t.peers[addr] = conn
	t.peersMu.Unlock()

	// Read responses from this peer
	go t.handleConn(conn)
	return nil
}

// Send sends a message to a specific peer address.
func (t *Transport) Send(addr string, msg Message) error {
	t.peersMu.RLock()
	conn, ok := t.peers[addr]
	t.peersMu.RUnlock()
	if !ok {
		// Try to reconnect
		if err := t.Connect(addr); err != nil {
			return err
		}
		t.peersMu.RLock()
		conn = t.peers[addr]
		t.peersMu.RUnlock()
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Write(data)
	if err != nil {
		// Remove dead connection
		t.peersMu.Lock()
		delete(t.peers, addr)
		t.peersMu.Unlock()
		return err
	}
	return nil
}

// Broadcast sends a message to all known peers.
func (t *Transport) Broadcast(msg Message) {
	t.peersMu.RLock()
	addrs := make([]string, 0, len(t.peers))
	for addr := range t.peers {
		addrs = append(addrs, addr)
	}
	t.peersMu.RUnlock()

	for _, addr := range addrs {
		t.Send(addr, msg)
	}
}

// Receive returns the channel for incoming messages.
func (t *Transport) Receive() <-chan Message {
	return t.incoming
}

// PeerCount returns the number of connected peers.
func (t *Transport) PeerCount() int {
	t.peersMu.RLock()
	defer t.peersMu.RUnlock()
	return len(t.peers)
}

// Close shuts down the transport.
func (t *Transport) Close() {
	close(t.done)
	if t.listener != nil {
		t.listener.Close()
	}
	t.peersMu.Lock()
	for addr, conn := range t.peers {
		conn.Close()
		delete(t.peers, addr)
	}
	t.peersMu.Unlock()
}
