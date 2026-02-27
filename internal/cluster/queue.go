package cluster

import "time"

// TaskAssignment carries the data needed for a follower to execute a task.
type TaskAssignment struct {
	AssignmentID string            `json:"assignment_id"`
	TaskName     string            `json:"task_name"`
	TaskID       string            `json:"task_id"`
	ProjectPath  string            `json:"project_path"`
	Content      string            `json:"content"`
	Runner       string            `json:"runner,omitempty"`
	RunnerChain  []string          `json:"runner_chain,omitempty"`
	Timeout      time.Duration     `json:"timeout,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	Priority     int               `json:"priority"`
	Labels       []string          `json:"labels,omitempty"`
	NodeAffinity string            `json:"node_affinity,omitempty"`
	TargetNodeID string            `json:"target_node_id"`
}

// TaskResult is sent from a follower back to the leader after task execution.
type TaskResult struct {
	AssignmentID  string    `json:"assignment_id"`
	TaskName      string    `json:"task_name"`
	TaskID        string    `json:"task_id"`
	NodeID        string    `json:"node_id"`
	ProjectPath   string    `json:"project_path"`
	Success       bool      `json:"success"`
	Started       time.Time `json:"started"`
	Finished      time.Time `json:"finished"`
	Error         string    `json:"error,omitempty"`
	OutputSummary string    `json:"output_summary,omitempty"`
}

// WorkerReport carries worker availability data, piggybacked on heartbeat_ack.
type WorkerReport struct {
	TotalWorkers int `json:"total_workers"`
	BusyWorkers  int `json:"busy_workers"`
	IdleWorkers  int `json:"idle_workers"`
}
