package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/project"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	Closed CircuitState = iota
	Open
	HalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case Closed:
		return "CLOSED"
	case Open:
		return "OPEN"
	case HalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerRecord represents the persisted state of a circuit breaker.
type CircuitBreakerRecord struct {
	TaskID         string      `json:"task_id"`
	State          CircuitState `json:"state"`
	FailureCount   int         `json:"failure_count"`
	LastFailureAt  *time.Time  `json:"last_failure_at,omitempty"`
	OpenedAt       *time.Time  `json:"opened_at,omitempty"`
	HalfOpenCount  int         `json:"half_open_count"`
	NextRetryAt    *time.Time  `json:"next_retry_at,omitempty"`
}

// CircuitBreakerStorage handles persisting and loading circuit breaker records.
type CircuitBreakerStorage struct {
	BasePath string
}

// NewCircuitBreakerStorage creates a new CircuitBreakerStorage.
func NewCircuitBreakerStorage(basePath string) *CircuitBreakerStorage {
	return &CircuitBreakerStorage{BasePath: basePath}
}

// SaveCircuit saves a circuit breaker record to storage.
func (s *CircuitBreakerStorage) SaveCircuit(taskID string, record CircuitBreakerRecord) error {
	taskDir := filepath.Join(s.BasePath, taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return fmt.Errorf("creating circuit directory: %w", err)
	}

	circuitFile := filepath.Join(taskDir, "circuit.json")
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling circuit: %w", err)
	}

	if err := os.WriteFile(circuitFile, data, 0644); err != nil {
		return fmt.Errorf("writing circuit file: %w", err)
	}

	return nil
}

// LoadCircuit loads a circuit breaker record for a task.
func (s *CircuitBreakerStorage) LoadCircuit(taskID string) (*CircuitBreakerRecord, error) {
	circuitFile := filepath.Join(s.BasePath, taskID, "circuit.json")

	data, err := os.ReadFile(circuitFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default closed circuit if no record exists
			return &CircuitBreakerRecord{
				TaskID: taskID,
				State:  Closed,
			}, nil
		}
		return nil, fmt.Errorf("reading circuit file: %w", err)
	}

	var record CircuitBreakerRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("parsing circuit file: %w", err)
	}

	return &record, nil
}

// getEffectiveCircuitBreaker returns the effective circuit breaker config for a task.
// Per-task config overrides global defaults.
func getEffectiveCircuitBreaker(todo project.Todo, globalCfg config.CircuitBreakerGlobalConfig) project.CircuitBreakerConfig {
	// If task has explicit circuit breaker config, use it
	if todo.CircuitBreaker.Failures > 0 {
		return todo.CircuitBreaker
	}

	// If global defaults are configured and task doesn't have explicit config, use global defaults
	if globalCfg.DefaultFailures > 0 {
		return project.CircuitBreakerConfig{
			Failures:    globalCfg.DefaultFailures,
			Timeout:     globalCfg.DefaultTimeout,
			HalfOpenMax: globalCfg.DefaultHalfOpenMax,
		}
	}

	// No circuit breaker config
	return project.CircuitBreakerConfig{}
}

// checkCircuit evaluates whether a task should run based on its circuit breaker state.
// Returns true if the task should be skipped due to an open circuit.
func checkCircuit(todo project.Todo, record *CircuitBreakerRecord, now time.Time, globalCfg config.CircuitBreakerGlobalConfig) bool {
	// If no circuit breaker config, don't skip
	config := getEffectiveCircuitBreaker(todo, globalCfg)
	if config.Failures <= 0 {
		return false
	}

	switch record.State {
	case Open:
		// Circuit is open, check if timeout has elapsed
		if record.NextRetryAt != nil && now.After(*record.NextRetryAt) {
			// Transition to half-open state
			record.State = HalfOpen
			record.HalfOpenCount = 0
			return false // Allow test request in half-open state
		}
		// Still open, skip task
		return true
	case HalfOpen:
		// In half-open state, check if we've exceeded the test request limit
		if record.HalfOpenCount >= config.HalfOpenMax && config.HalfOpenMax > 0 {
			// Exceeded test requests, remain in half-open but skip this one
			return true
		}
		// Allow test request
		return false
	default:
		// Closed state, allow task to run
		return false
	}
}

// recordFailure increments the failure count and opens the circuit if threshold is reached.
func recordFailure(todo project.Todo, record *CircuitBreakerRecord, now time.Time, globalCfg config.CircuitBreakerGlobalConfig) {
	config := getEffectiveCircuitBreaker(todo, globalCfg)
	if config.Failures <= 0 {
		return // No circuit breaker config
	}

	record.FailureCount++
	record.LastFailureAt = &now

	// Check if we should open the circuit
	if record.FailureCount >= config.Failures && record.State == Closed {
		// Open the circuit
		record.State = Open
		record.OpenedAt = &now

		// Calculate next retry time
		if config.Timeout > 0 {
			nextRetry := now.Add(config.Timeout)
			record.NextRetryAt = &nextRetry
		}
	}

	// If already in half-open state and we get a failure, reopen the circuit
	if record.State == HalfOpen {
		record.State = Open
		record.OpenedAt = &now

		// Calculate next retry time
		if config.Timeout > 0 {
			nextRetry := now.Add(config.Timeout)
			record.NextRetryAt = &nextRetry
		}
	}
}

// recordSuccess resets the failure count and closes the circuit on success.
func recordSuccess(todo project.Todo, record *CircuitBreakerRecord, now time.Time, globalCfg config.CircuitBreakerGlobalConfig) {
	config := getEffectiveCircuitBreaker(todo, globalCfg)
	if config.Failures <= 0 {
		return // No circuit breaker config
	}

	// Reset failure count on any success
	record.FailureCount = 0
	record.LastFailureAt = nil

	// If in half-open state and we get a success, close the circuit
	if record.State == HalfOpen {
		record.State = Closed
		record.OpenedAt = nil
		record.NextRetryAt = nil
		record.HalfOpenCount = 0
	}

	// If in open state and we get a success (shouldn't normally happen, but handle gracefully)
	if record.State == Open {
		record.State = Closed
		record.OpenedAt = nil
		record.NextRetryAt = nil
		record.HalfOpenCount = 0
	}
}

// runCircuitOpenHook executes the on_circuit_open hook if configured.
func runCircuitOpenHook(todo project.Todo, record *CircuitBreakerRecord) {
	if todo.OnCircuitOpen != "" {
		// In a real implementation, this would execute the shell command
		// For now, we'll just log that it would be executed
		fmt.Printf("[CIRCUIT] Would execute on_circuit_open hook for task %s: %s\n", todo.Name, todo.OnCircuitOpen)
	}
}

// runCircuitCloseHook executes the on_circuit_close hook if configured.
func runCircuitCloseHook(todo project.Todo, record *CircuitBreakerRecord) {
	if todo.OnCircuitClose != "" {
		// In a real implementation, this would execute the shell command
		// For now, we'll just log that it would be executed
		fmt.Printf("[CIRCUIT] Would execute on_circuit_close hook for task %s: %s\n", todo.Name, todo.OnCircuitClose)
	}
}