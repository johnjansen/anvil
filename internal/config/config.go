package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	TickInterval time.Duration `yaml:"tick_interval"`
	Runner       string        `yaml:"runner"`
	Runners      []string      `yaml:"runners"`
	Timeout      time.Duration `yaml:"timeout"`
	MaxTodos     int           `yaml:"max_todos"`
}

func Default() *Config {
	return &Config{
		TickInterval: 10 * time.Second,
		Runner:       "echo",
		Timeout:      5 * time.Minute,
		MaxTodos:     1,
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

func EnsureDir() error {
	if err := os.MkdirAll(Dir(), 0755); err != nil {
		return err
	}
	return os.MkdirAll(WatchedDir(), 0755)
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
	if cfg.MaxTodos <= 0 {
		cfg.MaxTodos = 1
	}

	// Backwards compatibility: if Runners is empty but Runner is set, use Runner as single entry
	if len(cfg.Runners) == 0 && cfg.Runner != "" {
		cfg.Runners = []string{cfg.Runner}
	}

	return cfg, nil
}
