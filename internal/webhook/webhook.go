package webhook

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/johnjansen/anvil/internal/config"
)

// Event represents a task lifecycle event type.
const (
	EventStart          = "task_start"
	EventSuccess        = "task_success"
	EventFailure        = "task_failure"
	EventTimeout        = "task_timeout"
	EventPersistentCycle = "persistent_cycle"
)

// Payload is the JSON body sent to webhook endpoints.
type Payload struct {
	Event            string  `json:"event"`
	TaskName         string  `json:"task_name"`
	Project          string  `json:"project"`
	Status           string  `json:"status"`
	RunID            string  `json:"run_id,omitempty"`
	StartedAt        string  `json:"started_at,omitempty"`
	FinishedAt       string  `json:"finished_at,omitempty"`
	DurationSeconds  float64 `json:"duration_seconds,omitempty"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd,omitempty"`
	Error            string  `json:"error,omitempty"`
}

// Sender dispatches webhook notifications for task lifecycle events.
type Sender struct {
	webhooks map[string]config.WebhookConfig
	client   *http.Client
}

// New creates a webhook Sender from config. Returns nil if no webhooks configured.
func New(webhooks map[string]config.WebhookConfig) *Sender {
	if len(webhooks) == 0 {
		return nil
	}
	return &Sender{
		webhooks: webhooks,
		client:   &http.Client{},
	}
}

// UpdateConfig replaces the webhook configuration (e.g. after config reload).
func (s *Sender) UpdateConfig(webhooks map[string]config.WebhookConfig) {
	s.webhooks = webhooks
}

// Fire sends the payload to all webhooks subscribed to the given event.
// It runs asynchronously and never blocks the caller.
func (s *Sender) Fire(event string, payload Payload) {
	if s == nil {
		return
	}
	payload.Event = event
	payload.Status = eventToStatus(event)

	for name, wh := range s.webhooks {
		if !matchesEvent(wh.Events, event) {
			continue
		}
		go s.deliver(name, wh, payload)
	}
}

// FireURL sends the payload to a single URL (for per-task webhook overrides).
func (s *Sender) FireURL(url, event string, payload Payload) {
	if s == nil || url == "" {
		return
	}
	payload.Event = event
	payload.Status = eventToStatus(event)
	wh := config.WebhookConfig{
		URL:     url,
		Method:  "POST",
		Timeout: 10 * time.Second,
	}
	go s.deliver("task-override", wh, payload)
}

// deliver sends the webhook with retry (3 attempts, exponential backoff).
func (s *Sender) deliver(name string, wh config.WebhookConfig, payload Payload) {
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[webhook] %s: failed to marshal payload: %v", name, err)
		return
	}

	method := wh.Method
	if method == "" {
		method = "POST"
	}
	timeout := wh.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	client := &http.Client{Timeout: timeout}

	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequest(method, wh.URL, bytes.NewReader(body))
		if err != nil {
			log.Printf("[webhook] %s: invalid request: %v", name, err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range wh.Headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[webhook] %s: attempt %d/%d failed: %v", name, attempt, maxAttempts, err)
			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
			}
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return // success
		}
		log.Printf("[webhook] %s: attempt %d/%d got status %d", name, attempt, maxAttempts, resp.StatusCode)
		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}
	log.Printf("[webhook] %s: all %d attempts failed for %s", name, maxAttempts, wh.URL)
}

// matchesEvent checks if the webhook's event filter includes the given event.
// An empty events list means "all events".
func matchesEvent(events []string, event string) bool {
	if len(events) == 0 {
		return true
	}
	short := eventToShort(event)
	for _, e := range events {
		if e == event || e == short {
			return true
		}
	}
	return false
}

// eventToStatus maps event names to status strings.
func eventToStatus(event string) string {
	switch event {
	case EventSuccess:
		return "success"
	case EventFailure:
		return "failure"
	case EventStart:
		return "started"
	case EventTimeout:
		return "timeout"
	case EventPersistentCycle:
		return "force_cycled"
	default:
		return event
	}
}

// eventToShort maps full event names to short config names.
func eventToShort(event string) string {
	switch event {
	case EventSuccess:
		return "success"
	case EventFailure:
		return "failure"
	case EventStart:
		return "start"
	case EventTimeout:
		return "timeout"
	case EventPersistentCycle:
		return "persistent_cycle"
	default:
		return event
	}
}

// BuildPayload constructs a webhook payload from task execution data.
func BuildPayload(taskName, projectPath, runID string, startedAt, finishedAt time.Time, costUSD float64, errMsg string) Payload {
	p := Payload{
		TaskName:  taskName,
		Project:   projectPath,
		RunID:     runID,
		StartedAt: startedAt.Format(time.RFC3339),
	}
	if !finishedAt.IsZero() {
		p.FinishedAt = finishedAt.Format(time.RFC3339)
		p.DurationSeconds = finishedAt.Sub(startedAt).Seconds()
	}
	p.EstimatedCostUSD = costUSD
	if errMsg != "" {
		p.Error = errMsg
	}
	return p
}
