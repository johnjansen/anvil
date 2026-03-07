package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func updateCmd(args []string) {
	checkOnly := false
	for _, a := range args {
		if a == "--check" {
			checkOnly = true
		}
	}

	// Fetch latest release from GitHub
	resp, err := http.Get("https://api.github.com/repos/johnjansen/anvil/releases/latest")
	if err != nil {
		fmt.Fprintf(os.Stderr, "update check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "update check failed: HTTP %d\n", resp.StatusCode)
		os.Exit(1)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse release info: %v\n", err)
		os.Exit(1)
	}

	latest := release.TagName
	if latest == "" {
		fmt.Fprintf(os.Stderr, "no releases found\n")
		os.Exit(1)
	}

	// Compare versions (strip leading 'v' for comparison)
	normalizedCurrent := strings.TrimPrefix(version, "v")
	normalizedLatest := strings.TrimPrefix(latest, "v")

	if normalizedCurrent == normalizedLatest {
		fmt.Printf("already up to date (%s)\n", version)
		return
	}

	if checkOnly {
		fmt.Printf("update available: %s → %s\n", version, latest)
		return
	}

	fmt.Printf("updating: %s → %s\n", version, latest)

	// Build download URL matching install.sh conventions
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	binaryURL := fmt.Sprintf("https://github.com/johnjansen/anvil/releases/download/%s/anvil-%s-%s", latest, goos, goarch)

	// Find the path to the running binary
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to determine executable path: %v\n", err)
		os.Exit(1)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve executable path: %v\n", err)
		os.Exit(1)
	}

	// Download new binary to a temp file in the same directory for atomic rename
	fmt.Printf("downloading %s...\n", binaryURL)
	dlResp, err := http.Get(binaryURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "download failed: %v\n", err)
		os.Exit(1)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "download failed: HTTP %d from %s\n", dlResp.StatusCode, binaryURL)
		os.Exit(1)
	}

	execDir := filepath.Dir(execPath)
	tmpFile, err := os.CreateTemp(execDir, ".anvil-update-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp file (permission denied?): %v\n", err)
		fmt.Fprintf(os.Stderr, "try: sudo anvil update\n")
		os.Exit(1)
	}
	tmpPath := tmpFile.Name()

	if _, err := io.Copy(tmpFile, dlResp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "download failed: %v\n", err)
		os.Exit(1)
	}
	tmpFile.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "failed to set permissions: %v\n", err)
		os.Exit(1)
	}

	// Atomically replace the binary
	if err := os.Rename(tmpPath, execPath); err != nil {
		os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "failed to replace binary: %v\n", err)
		fmt.Fprintf(os.Stderr, "try: sudo anvil update\n")
		os.Exit(1)
	}

	fmt.Printf("updated to %s\n", latest)
}
