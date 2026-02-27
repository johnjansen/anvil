package cluster

import (
	"sync"
	"time"
)

// heartbeatLoop sends periodic heartbeats while this node is leader.
func (n *Node) heartbeatLoop() {
	ticker := time.NewTicker(n.config.HeartbeatInterval)
	defer ticker.Stop()

	// Track heartbeat acks for majority detection
	var ackMu sync.Mutex
	ackCount := 0
	lastAckReset := time.Now()

	// Store ack handler reference
	n.stateMu.Lock()
	n.stateMu.Unlock()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.stateMu.RLock()
			if n.state.Role != Leader {
				n.stateMu.RUnlock()
				return
			}
			term := n.state.CurrentTerm
			n.stateMu.RUnlock()

			// Check majority acks from previous interval
			ackMu.Lock()
			if time.Since(lastAckReset) > n.config.ElectionTimeout {
				clusterSize := len(n.peers) + 1
				majority := clusterSize/2 + 1
				// Include self in ack count
				if ackCount+1 < majority && clusterSize > 1 {
					ackMu.Unlock()
					n.Logf("node %s lost majority (acks: %d, need: %d), stepping down", n.id, ackCount+1, majority)
					n.stepDown(term)
					return
				}
				ackCount = 0
				lastAckReset = time.Now()
			}
			ackMu.Unlock()

			// Send heartbeat to all peers
			n.transport.Broadcast(Message{
				Type:   MsgHeartbeat,
				Term:   term,
				FromID: n.id,
			})
		}
	}
}

// handleHeartbeat processes a heartbeat from the leader.
func (n *Node) handleHeartbeat(msg Message) {
	n.stateMu.Lock()
	if msg.Term >= n.state.CurrentTerm {
		n.state.CurrentTerm = msg.Term
		n.state.Role = Follower
		n.state.LeaderID = msg.FromID
	}
	n.stateMu.Unlock()

	// Reset election timer — leader is alive
	n.resetElectionTimer()

	// Send ack
	for _, addr := range n.peers {
		n.transport.Send(addr, Message{
			Type:   MsgHeartbeatAck,
			Term:   msg.Term,
			FromID: n.id,
			ToID:   msg.FromID,
		})
	}
}

// handleHeartbeatAck processes a heartbeat acknowledgment from a follower.
func (n *Node) handleHeartbeatAck(msg Message) {
	// Update peer tracking
	n.peersMu.Lock()
	n.peerLastSeen[msg.FromID] = time.Now()
	n.peersMu.Unlock()
}
