package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/johnjansen/anvil/internal/project"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState string

const (
	CircuitStateClosed   CircuitState = "closed"
	CircuitStateOpen     CircuitState = "open"
	CircuitStateHalfOpen CircuitState = "half_open"
)

// CircuitBreakerRecord represents the persisted state of a circuit breaker.
type CircuitBreakerRecord struct {
	TaskID            string        `json:"task_id"`
	State             CircuitState  `json:"state"`
	FailureCount      int           `json:"failure_count"`
	LastFailure       *time.Time    `json:"last_failure,omitempty"`
	OpenedAt          *time.Time    `json:"opened_at,omitempty"`
	HalfOpenRequests  int           `json:"half_open_requests"`
	LastSuccess       *time.Time    `json:"last_success,omitempty"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

// CircuitStorage handles persisting and loading circuit breaker records.
type CircuitStorage struct {
	BasePath string
}

// NewCircuitStorage creates a new CircuitStorage.
func NewCircuitStorage(basePath string) *CircuitStorage {
	return &CircuitStorage{BasePath: basePath}
}

// GetCircuit returns the circuit breaker record for a task.
// Returns nil if no record exists.
func (s *CircuitStorage) GetCircuit(taskID string) (*CircuitBreakerRecord, error) {
	circuitFile := s.circuitFile(taskID)
	data, err := os.ReadFile(circuitFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading circuit breaker record: %w", err)
	}

	var record CircuitBreakerRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("parsing circuit breaker record: %w", err)
	}

	return &record, nil
}

// SaveCircuit saves the circuit breaker record for a task.
func (s *CircuitStorage) SaveCircuit(taskID string, record *CircuitBreakerRecord) error {
	if err := os.MkdirAll(s.BasePath, 0755); err != nil {
		return fmt.Errorf("creating circuit breaker directory: %w", err)
	}

	record.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling circuit breaker record: %w", err)
	}

	circuitFile := s.circuitFile(taskID)
	if err := os.WriteFile(circuitFile, data, 0644); err != nil {
		return fmt.Errorf("writing circuit breaker record: %w", err)
	}

	return nil
}

// DeleteCircuit removes the circuit breaker record for a task.
func (s *CircuitStorage) DeleteCircuit(taskID string) error {
	circuitFile := s.circuitFile(taskID)
	if err := os.Remove(circuitFile); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("deleting circuit breaker record: %w", err)
	}
	return nil
}

// circuitFile returns the path to the circuit breaker record file.
func (s *CircuitStorage) circuitFile(taskID string) string {
	return filepath.Join(s.BasePath, fmt.Sprintf("%s.json", taskID))
}

// circuitResult holds the outcome of a circuit breaker check.
type circuitResult struct {
	ShouldRun     bool          // whether the task should run
	State         CircuitState  // current circuit state
	FailureCount  int           // current failure count
	LastFailure   *time.Time    // time of last failure
	OpenedAt      *time.Time    // when circuit opened
	NextRetryAt   *time.Time    // when circuit will retry (if open)
	Message       string        // reason if should not run
}

// getEffectiveCircuitBreaker returns the effective circuit breaker config for a task.
func getEffectiveCircuitBreaker(todo project.Todo) project.CircuitBreakerConfig {
	// Return the per-task config if set
	if todo.CircuitBreaker.Failures > 0 || todo.CircuitBreaker.Timeout > 0 || todo.CircuitBreaker.HalfOpenMax > 0 {
		cfg := todo.CircuitBreaker
		if cfg.Failures <= 0 {
			cfg.Failures = 5 // default
		}
		if cfg.Timeout <= 0 {
			cfg.Timeout = 30 * time.Minute // default
		}
		if cfg.HalfOpenMax <= 0 {
			cfg.HalfOpenMax = 2 // default
		}
		return cfg
	}
	// No circuit breaker configured
	return project.CircuitBreakerConfig{}
}

// checkCircuit evaluates whether a task should run based on its circuit breaker state.
// Returns a circuitResult with the decision and current state.
func checkCircuit(todo project.Todo, storage *CircuitStorage, now time.Time) circuitResult {
	cfg := getEffectiveCircuitBreaker(todo)
	if cfg.Failures == 0 && cfg.Timeout == 0 {
		// No circuit breaker configured
		return circuitResult{ShouldRun: true, State: CircuitStateClosed}
	}

	record, err := storage.GetCircuit(todo.Name)
	if err != nil {
		// On error, allow the task to run (fail open)
		return circuitResult{ShouldRun: true, State: CircuitStateClosed, Message: fmt.Sprintf("circuit check error: %v", err)}
	}

	if record == nil {
		// No record exists, start fresh
		return circuitResult{ShouldRun: true, State: CircuitStateClosed}
	}

	switch record.State {
	case CircuitStateClosed:
		return circuitResult{ShouldRun: true, State: CircuitStateClosed, FailureCount: record.FailureCount, LastFailure: record.LastFailure}

	case CircuitStateOpen:
		// Check if timeout has elapsed to transition to half-open
		if record.OpenedAt != nil && now.Sub(*record.OpenedAt) >= cfg.Timeout {
			record.State = CircuitStateHalfOpen
			record.HalfOpenRequests = 0
			if err := storage.SaveCircuit(todo.Name, record); err != nil {
				return circuitResult{ShouldRun: true, State: CircuitStateClosed, Message: fmt.Sprintf("circuit update error: %v", err)}
			}
			// Allow this request through as a test
			return circuitResult{ShouldRun: true, State: CircuitStateHalfOpen, FailureCount: record.FailureCount, LastFailure: record.LastFailure, OpenedAt: record.OpenedAt}
		}
		// Still open, skip
		nextRetry := record.OpenedAt.Add(cfg.Timeout)
		return circuitResult{
			ShouldRun:   false,
			State:       CircuitStateOpen,
			FailureCount: record.FailureCount,
			LastFailure: record.LastFailure,
			OpenedAt:    record.OpenedAt,
			NextRetryAt: &nextRetry,
			Message:     "circuit open",
		}

	case CircuitStateHalfOpen:
		// Allow limited test requests
		if record.HalfOpenRequests < cfg.HalfOpenMax {
			record.HalfOpenRequests++
			if err := storage.SaveCircuit(todo.Name, record); err != nil {
				return circuitResult{ShouldRun: true, State: CircuitStateHalfOpen, Message: fmt.Sprintf("circuit update error: %v", err)}
			}
			return circuitResult{ShouldRun: true, State: CircuitStateHalfOpen, FailureCount: record.FailureCount, LastFailure: record.LastFailure, OpenedAt: record.OpenedAt}
		}
		// Too many test requests, skip
		return circuitResult{
			ShouldRun:   false,
			State:       CircuitStateHalfOpen,
			FailureCount: record.FailureCount,
			LastFailure: record.LastFailure,
			OpenedAt:    record.OpenedAt,
			Message:     "circuit half-open: max test requests reached",
		}
	}

	return circuitResult{ShouldRun: true, State: CircuitStateClosed}
}

// recordFailure records a failure and updates circuit state if threshold reached.
func recordFailure(todo project.Todo, storage *CircuitStorage, now time.Time) error {
	cfg := getEffectiveCircuitBreaker(todo)
	if cfg.Failures == 0 {
		return nil // no circuit breaker configured
	}

	record, err := storage.GetCircuit(todo.Name)
	if err != nil {
		return err
	}

	if record == nil {
		record = &CircuitBreakerRecord{
			TaskID:       todo.Name,
			State:        CircuitStateClosed,
			FailureCount: 0,
		}
	}

	// Increment failure count
	record.FailureCount++
	record.LastFailure = &now

	// Check if we need to open the circuit
	if record.State == CircuitStateClosed && record.FailureCount >= cfg.Failures {
		record.State = CircuitStateOpen
		record.OpenedAt = &now
		// Fire the on_circuit_open hook
		if todo.OnCircuitOpen != "" {
			go runCircuitHook(todo.OnCircuitOpen, todo.Name, record.FailureCount, "")
		}
	} else if record.State == CircuitStateHalfOpen {
		// Failure in half-open reopens the circuit
		record.State = CircuitStateOpen
		record.OpenedAt = &now
		record.HalfOpenRequests = 0
		// Fire the on_circuit_open hook
		if todo.OnCircuitOpen != "" {
			go runCircuitHook(todo.OnCircuitOpen, todo.Name, record.FailureCount, "")
		}
	}

	return storage.SaveCircuit(todo.Name, record)
}

// recordSuccess records a success and resets failure count or closes circuit.
func recordSuccess(todo project.Todo, storage *CircuitStorage, now time.Time) error {
	cfg := getEffectiveCircuitBreaker(todo)
	if cfg.Failures == 0 {
		return nil // no circuit breaker configured
	}

	record, err := storage.GetCircuit(todo.Name)
	if err != nil {
		return err
	}

	if record == nil {
		// No record exists, nothing to do
		return nil
	}

	record.LastSuccess = &now

	if record.State == CircuitStateHalfOpen {
		// Success in half-open closes the circuit
		record.State = CircuitStateClosed
		record.FailureCount = 0
		record.OpenedAt = nil
		record.HalfOpenRequests = 0
		// Fire the on_circuit_close hook
		if todo.OnCircuitClose != "" {
			go runCircuitHook(todo.OnCircuitClose, todo.Name, 0, "")
		}
	} else if record.State == CircuitStateClosed {
		// Success in closed resets failure count
		record.FailureCount = 0
	}

	return storage.SaveCircuit(todo.Name, record)
}

// runCircuitHook executes a circuit breaker hook command.
func runCircuitHook(hook string, taskName string, failureCount int, lastError string) {
	// For now, just log that we would run the hook
	// The actual hook execution would use the runner's shell command execution
	_ = taskName
	_ = failureCount
	_ = lastError
	_ = hook
}

// GetCircuitStatus returns the current circuit breaker status for display.
func GetCircuitStatus(todo project.Todo, storage *CircuitStorage) (*CircuitBreakerRecord, error) {
	cfg := getEffectiveCircuitBreaker(todo)
	if cfg.Failures == 0 {
		return nil, nil
	}
	return storage.GetCircuit(todo.Name)
}
