package cluster

import (
	"encoding/json"
	"time"
)

// Role represents a node's role in the cluster.
type Role int

const (
	Follower  Role = iota
	Candidate
	Leader
)

func (r Role) String() string {
	switch r {
	case Follower:
		return "follower"
	case Candidate:
		return "candidate"
	case Leader:
		return "leader"
	default:
		return "unknown"
	}
}

func (r Role) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.String())
}

// MessageType identifies the type of cluster protocol message.
type MessageType string

const (
	MsgVoteRequest  MessageType = "vote_request"
	MsgVoteResponse MessageType = "vote_response"
	MsgHeartbeat    MessageType = "heartbeat"
	MsgHeartbeatAck MessageType = "heartbeat_ack"
)

// Message is the protocol message exchanged between cluster nodes.
type Message struct {
	Type    MessageType `json:"type"`
	Term    uint64      `json:"term"`
	FromID  string      `json:"from_id"`
	ToID    string      `json:"to_id,omitempty"`
	Granted bool        `json:"granted,omitempty"`
}

// ElectionState tracks the election protocol state for a node.
type ElectionState struct {
	CurrentTerm uint64
	VotedFor    string
	Role        Role
	LeaderID    string
	Votes       int
}

// MemberInfo describes a cluster member for status reporting.
type MemberInfo struct {
	ID       string    `json:"id"`
	Address  string    `json:"address"`
	Role     Role      `json:"role"`
	LastSeen time.Time `json:"last_seen"`
}

// ClusterStatus is the JSON response for /cluster/status.
type ClusterStatus struct {
	NodeID      string       `json:"node_id"`
	Role        Role         `json:"role"`
	Term        uint64       `json:"term"`
	LeaderID    string       `json:"leader_id"`
	ClusterSize int          `json:"cluster_size"`
	Members     []MemberInfo `json:"members"`
}
