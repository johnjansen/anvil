package cluster

import (
	"math/rand"
	"time"
)

// startElection transitions to candidate and requests votes.
func (n *Node) startElection() {
	n.stateMu.Lock()
	n.state.CurrentTerm++
	n.state.Role = Candidate
	n.state.VotedFor = n.id
	n.state.LeaderID = ""
	n.state.Votes = 1 // vote for self
	term := n.state.CurrentTerm
	n.stateMu.Unlock()

	n.Logf("node %s starting election for term %d", n.id, term)

	// Broadcast vote request
	n.transport.Broadcast(Message{
		Type:   MsgVoteRequest,
		Term:   term,
		FromID: n.id,
	})

	// Reset election timer with jitter
	n.resetElectionTimer()
}

// handleVoteRequest processes an incoming vote request.
func (n *Node) handleVoteRequest(msg Message) {
	n.stateMu.Lock()
	defer n.stateMu.Unlock()

	granted := false

	if msg.Term >= n.state.CurrentTerm {
		// Update term if needed
		if msg.Term > n.state.CurrentTerm {
			n.state.CurrentTerm = msg.Term
			n.state.VotedFor = ""
			n.state.Role = Follower
		}
		// Grant vote if we haven't voted in this term
		if n.state.VotedFor == "" || n.state.VotedFor == msg.FromID {
			n.state.VotedFor = msg.FromID
			granted = true
		}
	}

	// Find a peer address to send the response to
	for _, addr := range n.peers {
		n.transport.Send(addr, Message{
			Type:    MsgVoteResponse,
			Term:    n.state.CurrentTerm,
			FromID:  n.id,
			ToID:    msg.FromID,
			Granted: granted,
		})
	}
}

// handleVoteResponse processes a vote response.
func (n *Node) handleVoteResponse(msg Message) {
	n.stateMu.Lock()
	defer n.stateMu.Unlock()

	// Only process if we're still a candidate for this term
	if n.state.Role != Candidate || msg.Term != n.state.CurrentTerm {
		return
	}

	if msg.Granted {
		n.state.Votes++
	}

	// Check if we have majority
	// Cluster size = peers + self
	clusterSize := len(n.peers) + 1
	majority := clusterSize/2 + 1

	if n.state.Votes >= majority {
		n.state.Role = Leader
		n.state.LeaderID = n.id
		n.Logf("node %s elected leader for term %d (votes: %d/%d)", n.id, n.state.CurrentTerm, n.state.Votes, clusterSize)

		// Start sending heartbeats
		go n.heartbeatLoop()
	}
}

// stepDown transitions to follower for the given term.
func (n *Node) stepDown(term uint64) {
	n.stateMu.Lock()
	wasLeader := n.state.Role == Leader
	n.state.CurrentTerm = term
	n.state.Role = Follower
	n.state.VotedFor = ""
	n.state.Votes = 0
	n.stateMu.Unlock()

	if wasLeader {
		n.Logf("node %s stepped down from leader (higher term %d seen)", n.id, term)
	}

	n.resetElectionTimer()
}

// resetElectionTimer resets the election timeout with random jitter.
func (n *Node) resetElectionTimer() {
	if n.electionTimer != nil {
		n.electionTimer.Stop()
	}

	// Random timeout between 1x and 2x election timeout
	timeout := n.config.ElectionTimeout + time.Duration(rand.Int63n(int64(n.config.ElectionTimeout)))

	n.electionTimer = time.AfterFunc(timeout, func() {
		n.stateMu.RLock()
		role := n.state.Role
		n.stateMu.RUnlock()

		if role != Leader {
			n.startElection()
		}
	})
}
