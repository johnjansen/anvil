package config

import (
	"fmt"
	"os"
	"path/filepath"
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
	Hooks        HooksConfig    `yaml:"hooks"`
	Retention    RetentionConfig `yaml:"retention"`
}

// RetentionConfig defines data retention policies for logs and runs.
type RetentionConfig struct {
	MaxAge  time.Duration `yaml:"max_age"`  // delete logs/runs older than this
	MaxRuns int           `yaml:"max_runs"` // keep at most this many runs per task
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
	return os.MkdirAll(WatchedDir(), 0755)
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
