package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestHooksConfig_Unmarshal(t *testing.T) {
	configContent := `runners:
  - claude
hooks:
  on_success: "echo success"
  on_failure: "curl -X POST https://slack.example.com/webhook"
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(configContent), &cfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if cfg.Hooks.OnSuccess != "echo success" {
		t.Errorf("expected on_success='echo success', got %q", cfg.Hooks.OnSuccess)
	}
	if cfg.Hooks.OnFailure != "curl -X POST https://slack.example.com/webhook" {
		t.Errorf("unexpected on_failure: %q", cfg.Hooks.OnFailure)
	}
}

func TestHooksConfig_Empty(t *testing.T) {
	configContent := `runners:
  - claude
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(configContent), &cfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if cfg.Hooks.OnSuccess != "" {
		t.Errorf("expected empty on_success, got %q", cfg.Hooks.OnSuccess)
	}
	if cfg.Hooks.OnFailure != "" {
		t.Errorf("expected empty on_failure, got %q", cfg.Hooks.OnFailure)
	}
}

func TestLoadAutoUpdate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	anvilDir := filepath.Join(dir, ".anvil")
	os.MkdirAll(anvilDir, 0755)

	cfgContent := `auto_update: true
runners:
  - echo
`
	os.WriteFile(filepath.Join(anvilDir, "config.yaml"), []byte(cfgContent), 0644)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if !cfg.AutoUpdate {
		t.Error("expected AutoUpdate to be true")
	}
}

func TestParseByteSize(t *testing.T) {
	tests := []struct {
		input string
		want  int64
		err   bool
	}{
		{"0", 0, false},
		{"", 0, false},
		{"1024", 1024, false},
		{"10b", 10, false},
		{"1kb", 1024, false},
		{"10kb", 10240, false},
		{"1mb", 1048576, false},
		{"50mb", 52428800, false},
		{"1gb", 1073741824, false},
		{"1.5mb", 1572864, false},
		{"10MB", 10485760, false},
		{"50Mb", 52428800, false},
		{"abc", 0, true},
		{"mb", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseByteSize(tt.input)
		if (err != nil) != tt.err {
			t.Errorf("ParseByteSize(%q) error = %v, wantErr %v", tt.input, err, tt.err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseByteSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestRetentionConfigMaxLogSize(t *testing.T) {
	configContent := `runners:
  - claude
retention:
  max_age: 168h
  max_runs: 50
  max_log_size: 50mb
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(configContent), &cfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if cfg.Retention.MaxLogSize != 52428800 {
		t.Errorf("expected max_log_size=52428800, got %d", cfg.Retention.MaxLogSize)
	}
}

func TestRetentionConfigMaxLogSizeDefault(t *testing.T) {
	configContent := `runners:
  - claude
retention:
  max_age: 24h
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(configContent), &cfg); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	if cfg.Retention.MaxLogSize != 0 {
		t.Errorf("expected max_log_size=0 (unlimited by default), got %d", cfg.Retention.MaxLogSize)
	}
}

func TestLoadAutoUpdateDefaultFalse(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	anvilDir := filepath.Join(dir, ".anvil")
	os.MkdirAll(anvilDir, 0755)

	cfgContent := `runners:
  - echo
`
	os.WriteFile(filepath.Join(anvilDir, "config.yaml"), []byte(cfgContent), 0644)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.AutoUpdate {
		t.Error("expected AutoUpdate to default to false")
	}
}
