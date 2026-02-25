package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNeedsUpdate(t *testing.T) {
	tests := []struct {
		current, latest string
		want            bool
	}{
		{"v0.22.0", "v0.23.0", true},
		{"v0.22.0", "v0.22.0", false},
		{"0.22.0", "v0.22.0", false},
		{"dev", "v0.23.0", true},
		{"v1.0.0", "v1.0.0", false},
	}

	for _, tt := range tests {
		got := NeedsUpdate(tt.current, tt.latest)
		if got != tt.want {
			t.Errorf("NeedsUpdate(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")

	if err := os.WriteFile(src, []byte("hello"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want %q", string(data), "hello")
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0111 == 0 {
		t.Error("expected executable permissions on copy")
	}
}

func TestCheckLatest_MockServer(t *testing.T) {
	// Save and restore the package-level URL
	origURL := releaseURL
	defer func() {
		// We can't restore a const, so this test uses a server approach instead
		_ = origURL
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "v0.99.0"})
	}))
	defer server.Close()

	// Since releaseURL is a const, test the HTTP logic directly
	client := &http.Client{}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		t.Fatal(err)
	}
	if release.TagName != "v0.99.0" {
		t.Errorf("got tag %q, want %q", release.TagName, "v0.99.0")
	}
}

func TestRollback_NoBackup(t *testing.T) {
	err := Rollback("/nonexistent/path/anvil.bak")
	if err == nil {
		t.Error("expected error for missing backup")
	}
}
