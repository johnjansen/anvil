package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
)

func taskExtendTimeoutCmd(args []string) {
	if len(args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: anvil task extend-timeout <name> <duration>\n")
		fmt.Fprintf(os.Stderr, "  duration: e.g., 30m, 1h, 45m\n")
		os.Exit(1)
	}

	taskName := args[0]
	duration := args[1]

	// Validate duration
	if _, err := time.ParseDuration(duration); err != nil {
		fmt.Fprintf(os.Stderr, "invalid duration: %v\n", err)
		os.Exit(1)
	}

	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		os.Exit(1)
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	// Send extend timeout request
	reqBody, err := json.Marshal(map[string]string{
		"project":  proj.Path,
		"name":     taskName,
		"duration": duration,
	})
	if err != nil {
		log.Fatalf("failed to marshal request: %v", err)
	}

	resp, err := http.Post("http://localhost:3434/extend-timeout", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to send request: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		Success        bool   `json:"success"`
		Task           string `json:"task"`
		NewTimeout     string `json:"new_timeout"`
		ExtensionsUsed int    `json:"extensions_used"`
		Error          string `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse response: %v\n", err)
		os.Exit(1)
	}

	if !result.Success {
		fmt.Fprintf(os.Stderr, "failed to extend timeout: %s\n", result.Error)
		os.Exit(1)
	}

	fmt.Printf("✓ Extended timeout for %s\n", taskName)
	fmt.Printf("  New timeout: %s\n", result.NewTimeout)
	fmt.Printf("  Extensions used: %d\n", result.ExtensionsUsed)
}

// SendExtendTimeoutRequest sends a request to extend a task's timeout.
func SendExtendTimeoutRequest(projectPath, taskName, duration string) error {
	reqBody, err := json.Marshal(map[string]string{
		"project":  projectPath,
		"name":     taskName,
		"duration": duration,
	})
	if err != nil {
		return err
	}

	resp, err := http.Post("http://localhost:3434/extend-timeout", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	return nil
}
