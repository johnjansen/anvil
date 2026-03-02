package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	TickInterval time.Duration   `yaml:"tick_interval"`
	Runner       string         `yaml:"runner"`
	Runners      []string       `yaml:"runners"`
	Timeout      time.Duration  `yaml:"timeout"`
	MaxWorkers   int            `yaml:"max_workers"`
	MaxTodos     int            `yaml:"max_todos"` // deprecated: use max_workers
	Hooks           HooksConfig        `yaml:"hooks"`
	Retention       RetentionConfig    `yaml:"retention"`
	Webhooks        map[string]WebhookConfig `yaml:"webhooks"`
	InputTokenRate  float64         `yaml:"input_token_rate"`  // cost per 1M input tokens in USD (default: 3.0)
	OutputTokenRate float64         `yaml:"output_token_rate"` // cost per 1M output tokens in USD (default: 15.0)
	AutoUpdate      bool            `yaml:"auto_update"`       // opt-in: auto-update binary on daemon startup
	Env                      map[string]string `yaml:"env"`                        // global environment variables injected into all task executions
	GracefulShutdownTimeout  time.Duration     `yaml:"graceful_shutdown_timeout"`   // max wait for running tasks on graceful stop (default: 5m)
	QuietHours               QuietHoursConfig  `yaml:"quiet_hours"`                // global quiet hours to restrict non-critical task execution
	SLA                      SLAGlobalConfig   `yaml:"sla"`                        // global SLA defaults for task delay tracking
	Notifications            NotificationsConfig `yaml:"notifications"`              // desktop notification settings
	HealthPort               int                 `yaml:"health_port"`                 // optional TCP port for health probe endpoints (0 = disabled)
	Health                   HealthConfig        `yaml:"health"`                      // health check configuration
	Cluster                  ClusterConfig       `yaml:"cluster"`                     // multi-daemon cluster coordination
	Alerts                  AlertGlobalConfig  `yaml:"alerts"`                      // global alert settings
	CircuitBreaker          CircuitBreakerGlobalConfig `yaml:"circuit_breaker"`         // global circuit breaker defaults
	PriorityAging           PriorityAgingConfig `yaml:"priority_aging"`              // global priority aging settings
}

// SLAGlobalConfig defines global SLA defaults for task delay tracking.
type SLAGlobalConfig struct {
	DefaultMaxDelay string `yaml:"default_max_delay"` // default max_delay for tasks without per-task SLA (e.g., "30m")
}

// AlertGlobalConfig defines global alert settings.
type AlertGlobalConfig struct {
	Enabled          bool   `yaml:"enabled"`           // whether alerts are globally enabled
	DefaultWebhook   string `yaml:"default_webhook"`   // default webhook URL for all alerts
}

// CircuitBreakerGlobalConfig defines global circuit breaker defaults.
type CircuitBreakerGlobalConfig struct {
	DefaultFailures    int           `yaml:"default_failures"`    // default failures threshold (0 = disabled)
	DefaultTimeout     time.Duration `yaml:"default_timeout"`     // default timeout duration
	DefaultHalfOpenMax int           `yaml:"default_half_open_max"` // default half-open test requests
}

// PriorityAgingConfig defines global priority aging settings to prevent low-priority task starvation.
type PriorityAgingConfig struct {
	Enabled        bool          `yaml:"enabled"`          // whether priority aging is globally enabled
	DefaultMaxWait time.Duration `yaml:"default_max_wait"` // default wait time before boosting priority
	DefaultMaxBoost int          `yaml:"default_max_boost"` // default maximum priority boost levels (e.g., 2 means p3 can become p1)
}

// HealthConfig defines health check configuration.
type HealthConfig struct {
	IncludeTasks       bool `yaml:"include_tasks"`        // whether to include task health in daemon health endpoint
	UnhealthyThreshold int  `yaml:"unhealthy_threshold"`  // mark daemon unhealthy if this many tasks are unhealthy (0 = disabled)
}

// NotificationsConfig defines desktop notification settings.
type NotificationsConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Command        string `yaml:"command"`          // custom notification command
	OnSuccess      bool   `yaml:"on_success"`       // notify on task success
	OnFailure      bool   `yaml:"on_failure"`       // notify on task failure
}


// ClusterConfig defines multi-daemon cluster coordination settings.
type ClusterConfig struct {
	Enabled           bool          `yaml:"enabled"`
	Name              string        `yaml:"name"`
	Listen            string        `yaml:"listen"`              // TCP address for cluster protocol (default: ":9091")
	Peers             []string      `yaml:"peers"`              // static peer list
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"` // leader heartbeat interval (default: 5s)
	ElectionTimeout   time.Duration `yaml:"election_timeout"`   // follower election timeout (default: 30s)
}

// QuietHoursConfig defines a global time window during which only high-priority tasks may run.
type QuietHoursConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Start           string `yaml:"start"`            // HH:MM format (24h), e.g. "22:00"
	End             string `yaml:"end"`              // HH:MM format (24h), e.g. "07:00"
	ExcludePriority int    `yaml:"exclude_priority"` // tasks with priority <= this value bypass quiet hours (default: 0 = only p0)
}

// WebhookConfig defines a single webhook endpoint that receives task lifecycle events.
type WebhookConfig struct {
	URL     string            `yaml:"url"`
	Method  string            `yaml:"method"`  // default: POST
	Headers map[string]string `yaml:"headers"`
	Events  []string          `yaml:"events"`  // e.g. ["success", "failure", "start", "timeout"]
	Timeout time.Duration     `yaml:"timeout"` // default: 10s
}

// RetentionConfig defines data retention policies for logs and runs.
type RetentionConfig struct {
	MaxAge     time.Duration `yaml:"max_age"`      // delete logs/runs older than this
	MaxRuns    int           `yaml:"max_runs"`      // keep at most this many runs per task
	MaxLogSize int64         `yaml:"max_log_size"` // max size per log file in bytes (0 = unlimited)
}

// UnmarshalYAML implements custom YAML unmarshaling for RetentionConfig
// to support human-readable byte sizes for max_log_size (e.g., "50mb", "1gb").
func (r *RetentionConfig) UnmarshalYAML(value *yaml.Node) error {
	// Decode into a raw map to handle max_log_size as a string
	var raw struct {
		MaxAge     time.Duration `yaml:"max_age"`
		MaxRuns    int           `yaml:"max_runs"`
		MaxLogSize string        `yaml:"max_log_size"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	r.MaxAge = raw.MaxAge
	r.MaxRuns = raw.MaxRuns
	if raw.MaxLogSize != "" {
		size, err := ParseByteSize(raw.MaxLogSize)
		if err != nil {
			return fmt.Errorf("parsing max_log_size: %w", err)
		}
		r.MaxLogSize = size
	}
	return nil
}

// ParseByteSize parses a human-readable byte size string into bytes.
// Supported suffixes: b, kb, mb, gb (case-insensitive).
// Plain numbers are treated as bytes.
func ParseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "0" {
		return 0, nil
	}

	// Check longest suffixes first to avoid "b" matching before "mb"
	suffixes := []struct {
		suffix string
		mult   int64
	}{
		{"gb", 1024 * 1024 * 1024},
		{"mb", 1024 * 1024},
		{"kb", 1024},
		{"b", 1},
	}

	for _, entry := range suffixes {
		if strings.HasSuffix(s, entry.suffix) {
			numStr := strings.TrimSpace(s[:len(s)-len(entry.suffix)])
			if numStr == "" {
				return 0, fmt.Errorf("invalid byte size %q: missing number", s)
			}
			n, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid byte size %q: %w", s, err)
			}
			return int64(n * float64(entry.mult)), nil
		}
	}

	// Plain number (bytes)
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid byte size %q: %w", s, err)
	}
	return n, nil
}

// HooksConfig defines global lifecycle hooks that run for all tasks.
type HooksConfig struct {
	OnSuccess string `yaml:"on_success"`
	OnFailure string `yaml:"on_failure"`
}

func Default() *Config {
	return &Config{
		TickInterval: 10 * time.Second,
		Runner:       "echo",
		Timeout:      5 * time.Minute,
		MaxWorkers:   1,
		CircuitBreaker: CircuitBreakerGlobalConfig{
			DefaultFailures:    0, // disabled by default
			DefaultTimeout:     30 * time.Minute,
			DefaultHalfOpenMax: 3,
		},
		Health: HealthConfig{
			IncludeTasks:       false, // disabled by default
			UnhealthyThreshold: 0,     // disabled by default
		},
		PriorityAging: PriorityAgingConfig{
			Enabled:        false, // disabled by default
			DefaultMaxWait: 30 * time.Minute,
			DefaultMaxBoost: 2,
		},
	}
}

func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".anvil")
}

func Path() string {
	return filepath.Join(Dir(), "config.yaml")
}

func WatchedDir() string {
	return filepath.Join(Dir(), "watched")
}

func PidFile() string {
	return filepath.Join(Dir(), "daemon.pid")
}

func DaemonLogPath() string {
	return filepath.Join(Dir(), "daemon.log")
}

func EnsureDir() error {
	if err := os.MkdirAll(Dir(), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(WatchedDir(), 0755); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(Dir(), "circuits"), 0755)
}

// EnsureConfig creates ~/.anvil/config.yaml with sensible defaults if it does not exist.
// If the file already exists, it is left untouched.
// Returns true if the file was created, false if it already existed.
func EnsureConfig() (bool, error) {
	if err := EnsureDir(); err != nil {
		return false, err
	}

	p := Path()
	if _, err := os.Stat(p); err == nil {
		return false, nil // already exists
	}

	defaults := `runners:
  - claude
max_workers: 10
timeout: 15m
tick_interval: 10s
`
	return true, os.WriteFile(p, []byte(defaults), 0644)
}

func Load() (*Config, error) {
	data, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.TickInterval <= 0 {
		cfg.TickInterval = 10 * time.Second
	}
	// Backwards compatibility: max_todos maps to max_workers
	if cfg.MaxWorkers <= 0 && cfg.MaxTodos > 0 {
		cfg.MaxWorkers = cfg.MaxTodos
	}
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 1
	}

	// Backwards compatibility: if Runners is empty but Runner is set, use Runner as single entry
	if len(cfg.Runners) == 0 && cfg.Runner != "" {
		cfg.Runners = []string{cfg.Runner}
	}

	return cfg, nil
}
