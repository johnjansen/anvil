package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/johnjansen/anvil/internal/config"
)

// ThrottleState represents the persistent throttle configuration.
// It is stored in ~/.anvil/throttle.json and survives daemon restarts.
type ThrottleState struct {
	Paused       bool              `json:"paused"`                  // global pause
	RatePerMin   int               `json:"rate_per_min,omitempty"`  // max tasks per minute (0 = unlimited)
	PausedLabels map[string]bool   `json:"paused_labels,omitempty"` // label -> paused
	UpdatedAt    time.Time         `json:"updated_at"`
}

// throttleManager handles loading, saving, and querying throttle state.
type throttleManager struct {
	mu    sync.RWMutex
	state ThrottleState
	// dispatched tracks how many tasks were dispatched in the current minute window
	dispatched   int
	windowStart  time.Time
}

func newThrottleManager() *throttleManager {
	return &throttleManager{
		state: ThrottleState{
			PausedLabels: make(map[string]bool),
		},
	}
}

func throttleFilePath() string {
	return filepath.Join(config.Dir(), "throttle.json")
}

// load reads throttle state from disk. If the file doesn't exist, state is default (unpaused, no throttle).
func (tm *throttleManager) load() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	data, err := os.ReadFile(throttleFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			tm.state = ThrottleState{PausedLabels: make(map[string]bool)}
			return nil
		}
		return fmt.Errorf("reading throttle state: %w", err)
	}

	var s ThrottleState
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("parsing throttle state: %w", err)
	}
	if s.PausedLabels == nil {
		s.PausedLabels = make(map[string]bool)
	}
	tm.state = s
	return nil
}

// save persists throttle state to disk.
func (tm *throttleManager) save() error {
	tm.state.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(tm.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling throttle state: %w", err)
	}
	return os.WriteFile(throttleFilePath(), data, 0644)
}

// SetPaused sets global pause state.
func (tm *throttleManager) SetPaused(paused bool) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.state.Paused = paused
	return tm.save()
}

// SetRate sets the global throttle rate (tasks per minute). 0 = unlimited.
func (tm *throttleManager) SetRate(ratePerMin int) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.state.RatePerMin = ratePerMin
	return tm.save()
}

// PauseLabel pauses tasks with the given label.
func (tm *throttleManager) PauseLabel(label string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.state.PausedLabels[label] = true
	return tm.save()
}

// ResumeLabel resumes tasks with the given label.
func (tm *throttleManager) ResumeLabel(label string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.state.PausedLabels, label)
	return tm.save()
}

// IsPaused returns whether global pause is active.
func (tm *throttleManager) IsPaused() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.state.Paused
}

// IsLabelPaused returns whether any of the given labels are paused.
func (tm *throttleManager) IsLabelPaused(labels []string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	for _, l := range labels {
		if tm.state.PausedLabels[l] {
			return true
		}
	}
	return false
}

// AllowDispatch checks whether the throttle rate allows another dispatch.
// Call this before dispatching a task. Returns true if allowed.
func (tm *throttleManager) AllowDispatch(now time.Time) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.state.RatePerMin <= 0 {
		return true // no rate limit
	}

	minute := now.Truncate(time.Minute)
	if !minute.Equal(tm.windowStart) {
		// New minute window, reset counter
		tm.dispatched = 0
		tm.windowStart = minute
	}

	if tm.dispatched >= tm.state.RatePerMin {
		return false
	}

	tm.dispatched++
	return true
}

// GetState returns a copy of the current throttle state.
func (tm *throttleManager) GetState() ThrottleState {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	// Copy paused labels map
	labels := make(map[string]bool, len(tm.state.PausedLabels))
	for k, v := range tm.state.PausedLabels {
		labels[k] = v
	}
	return ThrottleState{
		Paused:       tm.state.Paused,
		RatePerMin:   tm.state.RatePerMin,
		PausedLabels: labels,
		UpdatedAt:    tm.state.UpdatedAt,
	}
}

// ParseRate parses a rate string like "5/m" into tasks per minute.
func ParseRate(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "off" || s == "0" {
		return 0, nil
	}

	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid rate format %q: expected N/m (e.g. 5/m)", s)
	}

	n, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid rate number %q: must be a positive integer", parts[0])
	}

	unit := strings.TrimSpace(parts[1])
	if unit != "m" {
		return 0, fmt.Errorf("invalid rate unit %q: only 'm' (per minute) is supported", unit)
	}

	return n, nil
}

// ThrottlePauseRequest is the JSON payload for /throttle/pause.
type ThrottlePauseRequest struct {
	Label string `json:"label,omitempty"` // empty = global pause
}

// ThrottleResumeRequest is the JSON payload for /throttle/resume.
type ThrottleResumeRequest struct {
	Label string `json:"label,omitempty"` // empty = global resume
}

// ThrottleRateRequest is the JSON payload for /throttle/rate.
type ThrottleRateRequest struct {
	RatePerMin int `json:"rate_per_min"` // 0 = disable throttle
}
