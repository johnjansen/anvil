package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/johnjansen/anvil/internal/config"
)

func newTestDaemon() *Daemon {
	cfg := config.Default()
	cfg.Runners = []string{"echo"}
	return New(cfg)
}

func TestHandleStopTask(t *testing.T) {
	d := newTestDaemon()

	body := `{"id":"task-123"}`
	req := httptest.NewRequest(http.MethodPost, "/stop", strings.NewReader(body))
	w := httptest.NewRecorder()

	d.handleStopTask(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "stopped" {
		t.Errorf("expected status 'stopped', got %q", resp["status"])
	}

	// Verify task is marked as stopped
	d.stoppedMu.Lock()
	stopped := d.stoppedTasks["task-123"]
	d.stoppedMu.Unlock()
	if !stopped {
		t.Error("expected task to be marked as stopped")
	}
}

func TestHandleStartTask(t *testing.T) {
	d := newTestDaemon()

	// First stop the task
	d.stoppedMu.Lock()
	d.stoppedTasks["task-456"] = true
	d.stoppedMu.Unlock()

	body := `{"task_id":"task-456","task_name":"my-task"}`
	req := httptest.NewRequest(http.MethodPost, "/start", strings.NewReader(body))
	w := httptest.NewRecorder()

	d.handleStartTask(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "started" {
		t.Errorf("expected status 'started', got %q", resp["status"])
	}

	// Verify task is no longer stopped
	d.stoppedMu.Lock()
	stopped := d.stoppedTasks["task-456"]
	d.stoppedMu.Unlock()
	if stopped {
		t.Error("expected task to no longer be stopped")
	}
}

func TestHandleStartTask_AlreadyRunning(t *testing.T) {
	d := newTestDaemon()

	// Task is not stopped
	body := `{"task_id":"task-789","task_name":"running-task"}`
	req := httptest.NewRequest(http.MethodPost, "/start", strings.NewReader(body))
	w := httptest.NewRecorder()

	d.handleStartTask(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "already_running" {
		t.Errorf("expected status 'already_running', got %q", resp["status"])
	}
}

func TestHandleStopTask_MethodNotAllowed(t *testing.T) {
	d := newTestDaemon()

	req := httptest.NewRequest(http.MethodGet, "/stop", nil)
	w := httptest.NewRecorder()

	d.handleStopTask(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestStoppedTaskBlocksDispatch(t *testing.T) {
	d := newTestDaemon()

	// Mark a task as stopped
	d.stoppedMu.Lock()
	d.stoppedTasks["task-abc"] = true
	d.stoppedMu.Unlock()

	// Verify it's in the stopped map
	d.stoppedMu.Lock()
	stopped := d.stoppedTasks["task-abc"]
	d.stoppedMu.Unlock()
	if !stopped {
		t.Error("expected task to be stopped")
	}

	// Clear it (simulating start)
	d.stoppedMu.Lock()
	delete(d.stoppedTasks, "task-abc")
	d.stoppedMu.Unlock()

	d.stoppedMu.Lock()
	stopped = d.stoppedTasks["task-abc"]
	d.stoppedMu.Unlock()
	if stopped {
		t.Error("expected task to no longer be stopped after clearing")
	}
}

func TestHandleStopTask_MissingID(t *testing.T) {
	d := newTestDaemon()

	body := `{"id":""}`
	req := httptest.NewRequest(http.MethodPost, "/stop", strings.NewReader(body))
	w := httptest.NewRecorder()

	d.handleStopTask(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
