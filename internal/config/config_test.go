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
