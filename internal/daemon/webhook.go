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

// WebhookServer handles HTTP webhook subscriptions for tasks
type WebhookServer struct {
	server   *http.Server
	mux      *http.ServeMux
	daemon   *Daemon
	handlers map[string]http.HandlerFunc // path -> handler
	mu       sync.RWMutex
}

// NewWebhookServer creates a new webhook server
func NewWebhookServer(addr string, daemon *Daemon) *WebhookServer {
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return &WebhookServer{
		server:   server,
		mux:      mux,
		daemon:   daemon,
		handlers: make(map[string]http.HandlerFunc),
	}
}

// Start starts the webhook server
func (ws *WebhookServer) Start() error {
	// Start server in goroutine
	go func() {
		if err := ws.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[webhook-server] Server failed to start: %v", err)
		}
	}()

	log.Printf("[webhook-server] Started webhook server on %s", ws.server.Addr)
	return nil
}

// Stop stops the webhook server
func (ws *WebhookServer) Stop(ctx context.Context) error {
	return ws.server.Shutdown(ctx)
}

// StartSubscription starts handling webhook requests for a task
func (ws *WebhookServer) StartSubscription(proj *project.Project, task project.Todo) error {
	if task.Subscription == nil || task.Subscription.Type != "webhook" {
		return nil // Not a webhook subscription
	}

	// Validate required fields
	if task.Subscription.WebhookPath == "" {
		return fmt.Errorf("webhook subscription requires webhook_path")
	}

	// Create handler function
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Validate method if specified
		if task.Subscription.WebhookMethod != "" && r.Method != strings.ToUpper(task.Subscription.WebhookMethod) {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Validate signature if secret is provided
		if task.Subscription.WebhookSecret != "" {
			if !ws.validateSignature(r, task.Subscription.WebhookSecret) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		// Read payload
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		// Trigger task execution
		ws.daemon.triggerWebhookTask(proj, task, body)

		// Respond with success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Task triggered successfully"))
	}

	// Register handler
	path := task.Subscription.WebhookPath
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	ws.mu.Lock()
	ws.handlers[path] = handler
	ws.mux.HandleFunc(path, handler)
	ws.mu.Unlock()

	log.Printf("[webhook-server] Registered webhook for task %s at path %s", task.Name, path)
	return nil
}

// validateSignature validates the webhook signature using HMAC-SHA256
func (ws *WebhookServer) validateSignature(r *http.Request, secret string) bool {
	// Handle environment variable references
	if strings.HasPrefix(secret, "env:") {
		envVar := strings.TrimPrefix(secret, "env:")
		secret = getEnv(envVar)
		if secret == "" {
			log.Printf("[webhook-server] Environment variable %s not set for webhook secret", envVar)
			return false
		}
	}

	// Get signature from header (support common header formats)
	var signature string
	if sig := r.Header.Get("X-Signature"); sig != "" {
		signature = strings.TrimPrefix(sig, "sha256=")
	} else if sig := r.Header.Get("X-Hub-Signature-256"); sig != "" {
		signature = strings.TrimPrefix(sig, "sha256=")
	} else {
		// No signature provided, fail validation if secret is required
		return false
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}

	// Reset body for further reading
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	// Compare signatures (constant time comparison to prevent timing attacks)
	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}

// getEnv retrieves an environment variable value
func getEnv(name string) string {
	return os.Getenv(name)
}