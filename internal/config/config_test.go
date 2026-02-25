package config

import (
	"os"
	"path/filepath"
	"testing"
)

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
