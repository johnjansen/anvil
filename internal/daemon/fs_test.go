package daemon

import (
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/project"
)

func TestEventNameToOp(t *testing.T) {
	tests := []struct {
		name   string
		wantOp fsnotify.Op
		wantOk bool
	}{
		{"create", fsnotify.Create, true},
		{"modify", fsnotify.Write, true},
		{"delete", fsnotify.Remove, true},
		{"rename", fsnotify.Rename, true},
		{"invalid", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		op, ok := eventNameToOp(tt.name)
		if ok != tt.wantOk {
			t.Errorf("eventNameToOp(%q) ok = %v, want %v", tt.name, ok, tt.wantOk)
		}
		if op != tt.wantOp {
			t.Errorf("eventNameToOp(%q) op = %v, want %v", tt.name, op, tt.wantOp)
		}
	}
}

func TestOpToEventName(t *testing.T) {
	tests := []struct {
		op   fsnotify.Op
		want string
	}{
		{fsnotify.Create, "create"},
		{fsnotify.Write, "modify"},
		{fsnotify.Remove, "delete"},
		{fsnotify.Rename, "rename"},
	}

	for _, tt := range tests {
		got := opToEventName(tt.op)
		if got != tt.want {
			t.Errorf("opToEventName(%v) = %q, want %q", tt.op, got, tt.want)
		}
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"*.json", "/data/file.json", true},
		{"*.json", "/data/file.csv", false},
		{"*", "/data/anything", true},
		{"data-*", "/path/data-123", true},
		{"data-*", "/path/other-123", false},
		{"*.yaml", "/path/config.yaml", true},
		{"*.yaml", "/path/config.yml", false},
	}

	for _, tt := range tests {
		got := matchGlob(tt.pattern, tt.name)
		if got != tt.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
		}
	}
}

func TestBuildAllowedEvents(t *testing.T) {
	// Empty list = all events allowed
	all := buildAllowedEvents(nil)
	if len(all) != 4 {
		t.Errorf("empty event list should allow 4 event types, got %d", len(all))
	}
	if !all[fsnotify.Create] || !all[fsnotify.Write] || !all[fsnotify.Remove] || !all[fsnotify.Rename] {
		t.Error("empty event list should allow all event types")
	}

	// Specific list
	specific := buildAllowedEvents([]string{"create", "modify"})
	if len(specific) != 2 {
		t.Errorf("expected 2 allowed events, got %d", len(specific))
	}
	if !specific[fsnotify.Create] {
		t.Error("expected create to be allowed")
	}
	if !specific[fsnotify.Write] {
		t.Error("expected modify (Write) to be allowed")
	}
	if specific[fsnotify.Remove] {
		t.Error("expected delete (Remove) to NOT be allowed")
	}
}

func TestDebouncer(t *testing.T) {
	flushed := make(chan []FileEvent, 1)
	d := newDebouncer(50*time.Millisecond, func(events []FileEvent) {
		flushed <- events
	})
	defer d.stop()

	// Add multiple events rapidly
	d.add(FileEvent{Path: "/a.json", Event: "create"})
	d.add(FileEvent{Path: "/b.json", Event: "create"})
	d.add(FileEvent{Path: "/c.json", Event: "modify"})

	// Should flush once after debounce period
	select {
	case events := <-flushed:
		if len(events) != 3 {
			t.Errorf("expected 3 events in batch, got %d", len(events))
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("debouncer did not flush within timeout")
	}
}

func TestDebouncerStop(t *testing.T) {
	flushed := make(chan []FileEvent, 1)
	d := newDebouncer(100*time.Millisecond, func(events []FileEvent) {
		flushed <- events
	})

	d.add(FileEvent{Path: "/a.json", Event: "create"})
	d.stop()

	// Should NOT flush after stop
	select {
	case <-flushed:
		t.Fatal("debouncer should not flush after stop")
	case <-time.After(200 * time.Millisecond):
		// Expected
	}
}

func TestFSWatcherStartSubscriptionValidation(t *testing.T) {
	cfg := config.Default()
	cfg.Runners = []string{"echo"}
	d := New(cfg)
	fw := NewFSWatcher(d)

	// Non-fs subscription should be a no-op
	err := fw.StartSubscription(&project.Project{}, project.Todo{
		Subscription: &project.SubscriptionConfig{Type: "amqp"},
	})
	if err != nil {
		t.Errorf("non-fs subscription should not error: %v", err)
	}

	// Missing fs_path
	err = fw.StartSubscription(&project.Project{}, project.Todo{
		Subscription: &project.SubscriptionConfig{Type: "fs"},
	})
	if err == nil {
		t.Error("expected error for missing fs_path")
	}

	// Invalid fs_events
	err = fw.StartSubscription(&project.Project{}, project.Todo{
		Subscription: &project.SubscriptionConfig{
			Type:     "fs",
			FsPath:   "/tmp",
			FsEvents: []string{"invalid"},
		},
	})
	if err == nil {
		t.Error("expected error for invalid fs_events value")
	}

	// Invalid fs_debounce
	err = fw.StartSubscription(&project.Project{}, project.Todo{
		Subscription: &project.SubscriptionConfig{
			Type:       "fs",
			FsPath:     "/tmp",
			FsDebounce: "not-a-duration",
		},
	})
	if err == nil {
		t.Error("expected error for invalid fs_debounce")
	}

	// Debounce too small
	err = fw.StartSubscription(&project.Project{}, project.Todo{
		Subscription: &project.SubscriptionConfig{
			Type:       "fs",
			FsPath:     "/tmp",
			FsDebounce: "10ms",
		},
	})
	if err == nil {
		t.Error("expected error for debounce < 100ms")
	}

	// Invalid glob pattern
	err = fw.StartSubscription(&project.Project{}, project.Todo{
		Subscription: &project.SubscriptionConfig{
			Type:   "fs",
			FsPath: "/tmp",
			FsGlob: "[invalid",
		},
	})
	if err == nil {
		t.Error("expected error for invalid glob pattern")
	}
}

func TestFSWatcherStartStop(t *testing.T) {
	cfg := config.Default()
	cfg.Runners = []string{"echo"}
	d := New(cfg)
	fw := NewFSWatcher(d)

	dir := t.TempDir()

	task := project.Todo{
		ID:   "test-fs-task",
		Name: "test-fs-task",
		Subscription: &project.SubscriptionConfig{
			Type:       "fs",
			FsPath:     dir,
			FsGlob:     "*.json",
			FsEvents:   []string{"create", "modify"},
			FsDebounce: "100ms",
		},
	}

	err := fw.StartSubscription(&project.Project{}, task)
	if err != nil {
		t.Fatalf("failed to start subscription: %v", err)
	}

	// Verify watcher was created
	fw.watchersMu.RLock()
	_, exists := fw.watchers[task.ID]
	fw.watchersMu.RUnlock()
	if !exists {
		t.Fatal("expected watcher to exist after start")
	}

	// Stop subscription
	err = fw.StopSubscription(task.ID)
	if err != nil {
		t.Fatalf("failed to stop subscription: %v", err)
	}

	// Verify watcher was removed
	fw.watchersMu.RLock()
	_, exists = fw.watchers[task.ID]
	fw.watchersMu.RUnlock()
	if exists {
		t.Fatal("expected watcher to be removed after stop")
	}
}

func TestFSWatcherNonExistentPath(t *testing.T) {
	cfg := config.Default()
	cfg.Runners = []string{"echo"}
	d := New(cfg)
	fw := NewFSWatcher(d)

	task := project.Todo{
		ID:   "test-nonexistent",
		Name: "test-nonexistent",
		Subscription: &project.SubscriptionConfig{
			Type:   "fs",
			FsPath: "/tmp/anvil-test-nonexistent-dir-12345",
		},
	}

	// Should not error — starts polling instead
	err := fw.StartSubscription(&project.Project{}, task)
	if err != nil {
		t.Fatalf("expected no error for non-existent path, got: %v", err)
	}

	// Clean up
	fw.StopAll()
}

func TestFSWatcherMatchesEventFilter(t *testing.T) {
	cfg := config.Default()
	cfg.Runners = []string{"echo"}
	d := New(cfg)
	fw := NewFSWatcher(d)

	w := &watcher{
		events: map[fsnotify.Op]bool{
			fsnotify.Create: true,
			fsnotify.Write:  true,
		},
	}

	if !fw.matchesEventFilter(w, fsnotify.Create) {
		t.Error("expected Create to match filter")
	}
	if !fw.matchesEventFilter(w, fsnotify.Write) {
		t.Error("expected Write to match filter")
	}
	if fw.matchesEventFilter(w, fsnotify.Remove) {
		t.Error("expected Remove to NOT match filter")
	}
	if fw.matchesEventFilter(w, fsnotify.Rename) {
		t.Error("expected Rename to NOT match filter")
	}
}
