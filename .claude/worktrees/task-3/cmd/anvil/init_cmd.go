package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
	"github.com/johnjansen/anvil/tools"
	"gopkg.in/yaml.v3"
)

func initCmd(args []string) {
	path := "."
	force := false
	var filtered []string
	for _, a := range args {
		if a == "--force" || a == "-f" {
			force = true
		} else {
			filtered = append(filtered, a)
		}
	}
	if len(filtered) > 0 {
		path = filtered[0]
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	result, err := project.Init(abs, tools.FS, force)
	if err != nil {
		log.Fatalf("failed to init project: %v", err)
	}

	if result.AlreadyExists {
		if result.BackupPath != "" {
			fmt.Printf("backed up %d existing task(s) to %s\n", result.TaskCount, result.BackupPath)
		} else {
			fmt.Printf("existing project with %d task(s) preserved\n", result.TaskCount)
		}
	}

	created, err := config.EnsureConfig()
	if err != nil {
		log.Fatalf("failed to create ~/.anvil/config.yaml: %v", err)
	}
	if created {
		fmt.Printf("created %s\n", config.Path())
	}

	fmt.Printf("initialized %s\n", abs)

	// Register project for watching (fold in old 'anvil watch' behavior)
	registerProject(abs)

	// Warn if daemon is not running
	if !daemon.IsDaemonRunning() {
		fmt.Fprintf(os.Stderr, "⚠ Daemon is not running. Run 'anvil watch' to start executing tasks.\n")
	}
}

func registerCmd(args []string) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	// Verify the directory exists
	info, err := os.Stat(abs)
	if err != nil {
		log.Fatalf("path does not exist: %s", abs)
	}
	if !info.IsDir() {
		log.Fatalf("not a directory: %s", abs)
	}

	registerProject(abs)

	if !daemon.IsDaemonRunning() {
		fmt.Fprintf(os.Stderr, "⚠ Daemon is not running. Run 'anvil watch' to start executing tasks.\n")
	}
}

// registerProject registers a project directory with the daemon watcher.
// Extracted so both initCmd and watchCmd (legacy) can share this logic.
func registerProject(abs string) {
	if err := config.EnsureDir(); err != nil {
		log.Fatalf("failed to create ~/.anvil: %v", err)
	}

	hash := projectHash(abs)
	watchDir := filepath.Join(config.WatchedDir(), hash)

	if entries, err := os.ReadDir(watchDir); err == nil && len(entries) > 0 {
		fmt.Printf("already watching %s\n", abs)
		return
	}

	if err := os.MkdirAll(watchDir, 0755); err != nil {
		log.Fatalf("failed to create watch dir: %v", err)
	}

	now := time.Now()
	filename := now.Format("2006-01-02T15-04-05") + ".md"

	frontmatter := watchFrontmatter{
		Path:      abs,
		WatchedAt: now,
	}
	data, err := yaml.Marshal(frontmatter)
	if err != nil {
		log.Fatalf("failed to marshal: %v", err)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(data)
	sb.WriteString("---\n")

	if err := os.WriteFile(filepath.Join(watchDir, filename), []byte(sb.String()), 0644); err != nil {
		log.Fatalf("failed to write watch file: %v", err)
	}

	fmt.Printf("watching %s\n", abs)
}
