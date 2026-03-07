package daemon

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/project"
)

func newWebhookTestDaemon() *Daemon {
	cfg := config.Default()
	return &Daemon{
		config:    cfg,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
		workQueue: make(chan workItem, 10),
	}
}

func TestWebhookServer_StartStop(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	if err := ws.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	ws.mu.RLock()
	running := ws.running
	ws.mu.RUnlock()
	if !running {
		t.Error("expected server to be running")
	}

	if err := ws.Stop(context.Background()); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	ws.mu.RLock()
	running = ws.running
	ws.mu.RUnlock()
	if running {
		t.Error("expected server to not be running after Stop")
	}
}

func TestWebhookServer_StopWhenNotRunning(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	if err := ws.Stop(context.Background()); err != nil {
		t.Fatalf("Stop on non-running server should not error: %v", err)
	}
}

func TestWebhookServer_POSTOnly(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	proj := &project.Project{Path: "./test"}
	task := project.Todo{
		Name: "post-only-task",
		Subscription: &project.SubscriptionConfig{
			Type:        "webhook",
			WebhookPath: "/test-post",
		},
	}

	if err := ws.StartSubscription(proj, task); err != nil {
		t.Fatalf("StartSubscription failed: %v", err)
	}

	ws.mu.RLock()
	handler := ws.handlers["/test-post"]
	ws.mu.RUnlock()

	// Test POST succeeds
	req := httptest.NewRequest("POST", "/test-post", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("POST expected 200, got %d", w.Code)
	}

	// Test GET returns 405
	req = httptest.NewRequest("GET", "/test-post", nil)
	w = httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET expected 405, got %d", w.Code)
	}

	// Test PUT returns 405
	req = httptest.NewRequest("PUT", "/test-post", strings.NewReader("{}"))
	w = httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT expected 405, got %d", w.Code)
	}
}

func TestWebhookServer_DuplicatePath(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	proj := &project.Project{Path: "./test"}
	task1 := project.Todo{
		Name: "task1",
		Subscription: &project.SubscriptionConfig{
			Type:        "webhook",
			WebhookPath: "/duplicate-path",
		},
	}
	task2 := project.Todo{
		Name: "task2",
		Subscription: &project.SubscriptionConfig{
			Type:        "webhook",
			WebhookPath: "/duplicate-path",
		},
	}

	if err := ws.StartSubscription(proj, task1); err != nil {
		t.Fatalf("First subscription should succeed: %v", err)
	}

	err := ws.StartSubscription(proj, task2)
	if err == nil {
		t.Fatal("Second subscription with same path should fail")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("Expected 'already registered' error, got: %v", err)
	}
}

func TestWebhookServer_BodySizeLimit(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	proj := &project.Project{Path: "./test"}
	task := project.Todo{
		Name: "body-limit-task",
		Subscription: &project.SubscriptionConfig{
			Type:        "webhook",
			WebhookPath: "/test-body-limit",
		},
	}

	if err := ws.StartSubscription(proj, task); err != nil {
		t.Fatalf("StartSubscription failed: %v", err)
	}

	bigBody := make([]byte, maxWebhookBodySize+1024)
	for i := range bigBody {
		bigBody[i] = 'x'
	}

	req := httptest.NewRequest("POST", "/test-body-limit", bytes.NewReader(bigBody))
	w := httptest.NewRecorder()
	ws.mu.RLock()
	handler := ws.handlers["/test-body-limit"]
	ws.mu.RUnlock()
	handler(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected 413 for oversized body, got %d", w.Code)
	}
}

func TestWebhookServer_HMACValidSignature(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	secret := "test-secret-key"
	proj := &project.Project{Path: "./test"}
	task := project.Todo{
		Name: "hmac-task",
		Subscription: &project.SubscriptionConfig{
			Type:          "webhook",
			WebhookPath:   "/test-hmac",
			WebhookSecret: secret,
		},
	}

	if err := ws.StartSubscription(proj, task); err != nil {
		t.Fatalf("StartSubscription failed: %v", err)
	}

	body := []byte(`{"event": "push"}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/test-hmac", bytes.NewReader(body))
	req.Header.Set("X-Signature", signature)
	w := httptest.NewRecorder()
	ws.mu.RLock()
	handler := ws.handlers["/test-hmac"]
	ws.mu.RUnlock()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Valid HMAC expected 200, got %d", w.Code)
	}
}

func TestWebhookServer_HMACInvalidSignature(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	proj := &project.Project{Path: "./test"}
	task := project.Todo{
		Name: "hmac-invalid-task",
		Subscription: &project.SubscriptionConfig{
			Type:          "webhook",
			WebhookPath:   "/test-hmac-invalid",
			WebhookSecret: "correct-secret",
		},
	}

	if err := ws.StartSubscription(proj, task); err != nil {
		t.Fatalf("StartSubscription failed: %v", err)
	}

	body := []byte(`{"event": "push"}`)
	mac := hmac.New(sha256.New, []byte("wrong-secret"))
	mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/test-hmac-invalid", bytes.NewReader(body))
	req.Header.Set("X-Signature", signature)
	w := httptest.NewRecorder()
	ws.mu.RLock()
	handler := ws.handlers["/test-hmac-invalid"]
	ws.mu.RUnlock()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Invalid HMAC expected 401, got %d", w.Code)
	}
}

func TestWebhookServer_HMACMissingSignature(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	proj := &project.Project{Path: "./test"}
	task := project.Todo{
		Name: "hmac-missing-task",
		Subscription: &project.SubscriptionConfig{
			Type:          "webhook",
			WebhookPath:   "/test-hmac-missing",
			WebhookSecret: "some-secret",
		},
	}

	if err := ws.StartSubscription(proj, task); err != nil {
		t.Fatalf("StartSubscription failed: %v", err)
	}

	req := httptest.NewRequest("POST", "/test-hmac-missing", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	ws.mu.RLock()
	handler := ws.handlers["/test-hmac-missing"]
	ws.mu.RUnlock()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Missing signature expected 401, got %d", w.Code)
	}
}

func TestWebhookServer_HMACEnvVarNotSet(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	os.Unsetenv("MISSING_WEBHOOK_SECRET")

	proj := &project.Project{Path: "./test"}
	task := project.Todo{
		Name: "hmac-env-missing",
		Subscription: &project.SubscriptionConfig{
			Type:          "webhook",
			WebhookPath:   "/test-env-missing",
			WebhookSecret: "env:MISSING_WEBHOOK_SECRET",
		},
	}

	err := ws.StartSubscription(proj, task)
	if err == nil {
		t.Fatal("Expected error when env var is not set")
	}
	if !strings.Contains(err.Error(), "MISSING_WEBHOOK_SECRET") {
		t.Errorf("Expected env var name in error, got: %v", err)
	}
}

func TestWebhookServer_HMACEnvVarSet(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	secret := "env-based-secret"
	os.Setenv("TEST_WEBHOOK_SECRET_366", secret)
	defer os.Unsetenv("TEST_WEBHOOK_SECRET_366")

	proj := &project.Project{Path: "./test"}
	task := project.Todo{
		Name: "hmac-env-set",
		Subscription: &project.SubscriptionConfig{
			Type:          "webhook",
			WebhookPath:   "/test-env-set",
			WebhookSecret: "env:TEST_WEBHOOK_SECRET_366",
		},
	}

	if err := ws.StartSubscription(proj, task); err != nil {
		t.Fatalf("StartSubscription should succeed when env var is set: %v", err)
	}

	body := []byte(`{"test": true}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/test-env-set", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", signature)
	w := httptest.NewRecorder()
	ws.mu.RLock()
	handler := ws.handlers["/test-env-set"]
	ws.mu.RUnlock()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Valid HMAC with env var expected 200, got %d", w.Code)
	}
}

func TestWebhookServer_NoSecretAllowsAll(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	proj := &project.Project{Path: "./test"}
	task := project.Todo{
		Name: "no-secret-task",
		Subscription: &project.SubscriptionConfig{
			Type:        "webhook",
			WebhookPath: "/test-no-secret",
		},
	}

	if err := ws.StartSubscription(proj, task); err != nil {
		t.Fatalf("StartSubscription failed: %v", err)
	}

	req := httptest.NewRequest("POST", "/test-no-secret", strings.NewReader(`{"open": true}`))
	w := httptest.NewRecorder()
	ws.mu.RLock()
	handler := ws.handlers["/test-no-secret"]
	ws.mu.RUnlock()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("No secret should allow all requests, got %d", w.Code)
	}
}

func TestWebhookServer_HeadersPassed(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	proj := &project.Project{Path: "./test"}
	task := project.Todo{
		Name: "headers-task",
		Subscription: &project.SubscriptionConfig{
			Type:        "webhook",
			WebhookPath: "/test-headers",
		},
	}

	if err := ws.StartSubscription(proj, task); err != nil {
		t.Fatalf("StartSubscription failed: %v", err)
	}

	req := httptest.NewRequest("POST", "/test-headers", strings.NewReader(`{"ref": "main"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	w := httptest.NewRecorder()
	ws.mu.RLock()
	handler := ws.handlers["/test-headers"]
	ws.mu.RUnlock()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	select {
	case item := <-d.workQueue:
		if item.todo.Env["ANVIL_WEBHOOK_CONTENT_TYPE"] != "application/json" {
			t.Errorf("Expected Content-Type env var, got %q", item.todo.Env["ANVIL_WEBHOOK_CONTENT_TYPE"])
		}
		if item.todo.Env["ANVIL_WEBHOOK_EVENT"] != "push" {
			t.Errorf("Expected event env var, got %q", item.todo.Env["ANVIL_WEBHOOK_EVENT"])
		}
		if item.todo.Env["ANVIL_WEBHOOK_PAYLOAD"] != `{"ref": "main"}` {
			t.Errorf("Expected payload env var, got %q", item.todo.Env["ANVIL_WEBHOOK_PAYLOAD"])
		}
		if item.todo.Env["ANVIL_WEBHOOK_HEADERS"] == "" {
			t.Error("Expected headers JSON env var to be set")
		}
	case <-time.After(time.Second):
		t.Error("Timed out waiting for work item")
	}
}

func TestWebhookServer_SimplifiedConfig(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	proj := &project.Project{Path: "./test"}
	task := project.Todo{
		Name: "simple-task",
		Subscription: &project.SubscriptionConfig{
			Webhook: "/simple-webhook",
		},
	}

	if err := ws.StartSubscription(proj, task); err != nil {
		t.Fatalf("StartSubscription with simplified config failed: %v", err)
	}

	ws.mu.RLock()
	_, exists := ws.handlers["/simple-webhook"]
	ws.mu.RUnlock()
	if !exists {
		t.Error("Expected handler registered for simplified webhook path")
	}
}

func TestWebhookServer_StopSubscription(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	proj := &project.Project{Path: "./test"}
	task := project.Todo{
		Name: "removable-task",
		Subscription: &project.SubscriptionConfig{
			Type:        "webhook",
			WebhookPath: "/removable",
		},
	}

	if err := ws.StartSubscription(proj, task); err != nil {
		t.Fatalf("StartSubscription failed: %v", err)
	}

	ws.mu.RLock()
	_, exists := ws.handlers["/removable"]
	ws.mu.RUnlock()
	if !exists {
		t.Fatal("Expected handler to exist after registration")
	}

	ws.StopSubscription("removable-task")

	ws.mu.RLock()
	_, exists = ws.handlers["/removable"]
	_, pathExists := ws.taskPaths["removable-task"]
	ws.mu.RUnlock()
	if exists {
		t.Error("Expected handler to be removed after StopSubscription")
	}
	if pathExists {
		t.Error("Expected taskPath to be removed after StopSubscription")
	}
}

func TestWebhookServer_StopSubscriptionUnknownTask(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	// Should be a no-op, not panic
	ws.StopSubscription("nonexistent-task")
}

func TestWebhookServer_PathNormalization(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	proj := &project.Project{Path: "./test"}
	task := project.Todo{
		Name: "no-slash-task",
		Subscription: &project.SubscriptionConfig{
			Type:        "webhook",
			WebhookPath: "no-leading-slash",
		},
	}

	if err := ws.StartSubscription(proj, task); err != nil {
		t.Fatalf("StartSubscription failed: %v", err)
	}

	ws.mu.RLock()
	_, exists := ws.handlers["/no-leading-slash"]
	ws.mu.RUnlock()
	if !exists {
		t.Error("Expected path to be normalized with leading slash")
	}
}

func TestWebhookServer_NilSubscription(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	proj := &project.Project{Path: "./test"}
	task := project.Todo{Name: "no-sub-task"}

	if err := ws.StartSubscription(proj, task); err != nil {
		t.Fatalf("Nil subscription should be a no-op, got error: %v", err)
	}
}

func TestWebhookServer_NonWebhookType(t *testing.T) {
	d := newWebhookTestDaemon()
	ws := NewWebhookServer(":0", d)

	proj := &project.Project{Path: "./test"}
	task := project.Todo{
		Name: "amqp-task",
		Subscription: &project.SubscriptionConfig{
			Type: "amqp",
		},
	}

	if err := ws.StartSubscription(proj, task); err != nil {
		t.Fatalf("Non-webhook type should be a no-op, got error: %v", err)
	}
}

func TestWebhookServer_DefaultPort(t *testing.T) {
	ws := NewWebhookServer(":9090", nil)
	if ws.server.Addr != ":9090" {
		t.Errorf("Expected default port :9090, got %s", ws.server.Addr)
	}
}

func TestWebhookServer_CustomPort(t *testing.T) {
	ws := NewWebhookServer(":8080", nil)
	if ws.server.Addr != ":8080" {
		t.Errorf("Expected custom port :8080, got %s", ws.server.Addr)
	}
}
