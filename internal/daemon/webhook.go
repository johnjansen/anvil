package daemon

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/johnjansen/anvil/internal/project"
)

const maxWebhookBodySize = 1 << 20 // 1 MB

// WebhookServer handles HTTP webhook subscriptions for tasks
type WebhookServer struct {
	server    *http.Server
	mux       *http.ServeMux
	daemon    *Daemon
	handlers  map[string]http.HandlerFunc // path -> handler
	taskPaths map[string]string           // taskName -> path
	running   bool
	mu        sync.RWMutex
}

// NewWebhookServer creates a new webhook server
func NewWebhookServer(addr string, daemon *Daemon) *WebhookServer {
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return &WebhookServer{
		server:    server,
		mux:       mux,
		daemon:    daemon,
		handlers:  make(map[string]http.HandlerFunc),
		taskPaths: make(map[string]string),
	}
}

// Start starts the webhook server
func (ws *WebhookServer) Start() error {
	ws.mu.Lock()
	if ws.running {
		ws.mu.Unlock()
		return nil
	}
	ws.running = true
	ws.mu.Unlock()

	go func() {
		log.Printf("[webhook-server] Started on %s", ws.server.Addr)
		if err := ws.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[webhook-server] Failed to start: %v", err)
			ws.mu.Lock()
			ws.running = false
			ws.mu.Unlock()
		}
	}()

	return nil
}

// Stop stops the webhook server
func (ws *WebhookServer) Stop(ctx context.Context) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if !ws.running {
		return nil
	}
	ws.running = false
	log.Printf("[webhook-server] Stopping webhook server")
	return ws.server.Shutdown(ctx)
}

// StartSubscription starts handling webhook requests for a task
func (ws *WebhookServer) StartSubscription(proj *project.Project, task project.Todo) error {
	if task.Subscription == nil {
		return nil
	}

	// Handle simplified webhook configuration (subscription.webhook: "/path")
	if task.Subscription.Webhook != "" {
		task.Subscription.Type = "webhook"
		task.Subscription.WebhookPath = task.Subscription.Webhook
		if task.Subscription.WebhookMethod == "" {
			task.Subscription.WebhookMethod = "POST"
		}
	}

	if task.Subscription.Type != "webhook" {
		return nil
	}

	if task.Subscription.WebhookPath == "" {
		return fmt.Errorf("webhook subscription requires webhook_path")
	}

	// Validate secret env var at startup
	if strings.HasPrefix(task.Subscription.WebhookSecret, "env:") {
		envVar := strings.TrimPrefix(task.Subscription.WebhookSecret, "env:")
		if os.Getenv(envVar) == "" {
			return fmt.Errorf("webhook secret environment variable %s is not set for task %s", envVar, task.Name)
		}
	}

	path := task.Subscription.WebhookPath
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Check for duplicate paths
	ws.mu.Lock()
	if _, exists := ws.handlers[path]; exists {
		ws.mu.Unlock()
		return fmt.Errorf("webhook path %s is already registered by another task", path)
	}
	ws.mu.Unlock()

	// Determine allowed method (default POST)
	allowedMethod := strings.ToUpper(task.Subscription.WebhookMethod)
	if allowedMethod == "" {
		allowedMethod = "POST"
	}

	webhookSecret := task.Subscription.WebhookSecret

	handler := func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[webhook-server] Received %s request on %s", r.Method, path)

		if r.Method != allowedMethod {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Enforce body size limit
		r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBodySize)

		// Validate signature if secret is provided
		if webhookSecret != "" {
			if !ws.validateSignature(r, webhookSecret) {
				log.Printf("[webhook-server] Authentication failed for %s", path)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		// Read payload
		body, err := io.ReadAll(r.Body)
		if err != nil {
			if err.Error() == "http: request body too large" {
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		// Extract headers for the task
		headers := make(map[string]string)
		for key := range r.Header {
			headers[key] = r.Header.Get(key)
		}

		ws.daemon.triggerWebhookTask(proj, task, body, headers)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Task triggered successfully"))
	}

	ws.mu.Lock()
	ws.handlers[path] = handler
	ws.taskPaths[task.Name] = path
	ws.mux.HandleFunc(path, handler)
	ws.mu.Unlock()

	log.Printf("[webhook-server] Registered webhook for task %s at path %s (method: %s)", task.Name, path, allowedMethod)

	// Start server on-demand if not already running
	ws.mu.RLock()
	isRunning := ws.running
	ws.mu.RUnlock()
	if !isRunning {
		if err := ws.Start(); err != nil {
			return fmt.Errorf("failed to start webhook server: %w", err)
		}
	}

	return nil
}

// StopSubscription removes a webhook handler for a task and stops the server if no handlers remain
func (ws *WebhookServer) StopSubscription(taskID string) {
	ws.mu.Lock()
	path, ok := ws.taskPaths[taskID]
	if !ok {
		ws.mu.Unlock()
		return
	}
	delete(ws.handlers, path)
	delete(ws.taskPaths, taskID)
	remaining := len(ws.handlers)
	ws.mu.Unlock()

	log.Printf("[webhook-server] Removed webhook for task %s at path %s", taskID, path)

	if remaining == 0 {
		ws.Stop(context.Background())
	}
}

// validateSignature validates the webhook signature using HMAC-SHA256
func (ws *WebhookServer) validateSignature(r *http.Request, secret string) bool {
	if strings.HasPrefix(secret, "env:") {
		envVar := strings.TrimPrefix(secret, "env:")
		secret = os.Getenv(envVar)
		if secret == "" {
			log.Printf("[webhook-server] Environment variable %s not set for webhook secret", envVar)
			return false
		}
	}

	var signature string
	if sig := r.Header.Get("X-Signature"); sig != "" {
		signature = strings.TrimPrefix(sig, "sha256=")
	} else if sig := r.Header.Get("X-Hub-Signature-256"); sig != "" {
		signature = strings.TrimPrefix(sig, "sha256=")
	} else {
		return false
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}

	// Reset body for further reading
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}
