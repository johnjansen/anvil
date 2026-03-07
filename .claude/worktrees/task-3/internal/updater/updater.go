package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	releaseURL = "https://api.github.com/repos/johnjansen/anvil/releases/latest"
	// CheckInterval is how often the daemon should check for updates (once per day).
	CheckInterval = 24 * time.Hour
)

// Result describes the outcome of an update check or apply.
type Result struct {
	CurrentVersion string
	LatestVersion  string
	Updated        bool
	BackupPath     string
	Error          error
}

// CheckLatest fetches the latest release tag from GitHub.
// Returns the tag name (e.g. "v0.23.0") or an error.
func CheckLatest() (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(releaseURL)
	if err != nil {
		return "", fmt.Errorf("update check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update check failed: HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse release info: %w", err)
	}

	if release.TagName == "" {
		return "", fmt.Errorf("no releases found")
	}

	return release.TagName, nil
}

// NeedsUpdate returns true if latest is a different version from current.
func NeedsUpdate(current, latest string) bool {
	return normalize(current) != normalize(latest)
}

// Apply downloads the latest release binary and replaces the current executable.
// It keeps a backup of the previous binary at <exec>.bak for rollback.
// Returns a Result with details.
func Apply(currentVersion, latestTag string) Result {
	res := Result{
		CurrentVersion: currentVersion,
		LatestVersion:  latestTag,
	}

	execPath, err := os.Executable()
	if err != nil {
		res.Error = fmt.Errorf("failed to determine executable path: %w", err)
		return res
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		res.Error = fmt.Errorf("failed to resolve executable path: %w", err)
		return res
	}

	binaryURL := fmt.Sprintf(
		"https://github.com/johnjansen/anvil/releases/download/%s/anvil-%s-%s",
		latestTag, runtime.GOOS, runtime.GOARCH,
	)

	client := &http.Client{Timeout: 120 * time.Second}
	dlResp, err := client.Get(binaryURL)
	if err != nil {
		res.Error = fmt.Errorf("download failed: %w", err)
		return res
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		res.Error = fmt.Errorf("download failed: HTTP %d from %s", dlResp.StatusCode, binaryURL)
		return res
	}

	// Download to temp file in same directory for atomic rename
	execDir := filepath.Dir(execPath)
	tmpFile, err := os.CreateTemp(execDir, ".anvil-update-*")
	if err != nil {
		res.Error = fmt.Errorf("failed to create temp file: %w", err)
		return res
	}
	tmpPath := tmpFile.Name()

	if _, err := io.Copy(tmpFile, dlResp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		res.Error = fmt.Errorf("download write failed: %w", err)
		return res
	}
	tmpFile.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		res.Error = fmt.Errorf("failed to set permissions: %w", err)
		return res
	}

	// Back up the current binary for rollback
	backupPath := execPath + ".bak"
	if err := copyFile(execPath, backupPath); err != nil {
		os.Remove(tmpPath)
		res.Error = fmt.Errorf("failed to create backup: %w", err)
		return res
	}
	res.BackupPath = backupPath

	// Atomically replace the binary
	if err := os.Rename(tmpPath, execPath); err != nil {
		os.Remove(tmpPath)
		res.Error = fmt.Errorf("failed to replace binary: %w", err)
		return res
	}

	res.Updated = true
	return res
}

// Rollback restores the previous binary from the backup path.
func Rollback(backupPath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("backup not found at %s: %w", backupPath, err)
	}

	return os.Rename(backupPath, execPath)
}

func normalize(v string) string {
	return strings.TrimPrefix(v, "v")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	// Preserve executable permissions
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}
