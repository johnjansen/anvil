package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/johnjansen/anvil/internal/cache"
	"github.com/johnjansen/anvil/internal/project"
)

func taskCacheCmd(args []string) {
	fs := flag.NewFlagSet("task cache", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: anvil task cache <task-name> [options]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Show cache status for a task or clear cached output.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  --clear     Clear cached output for the task")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  anvil task cache my-task      # Show cache status")
		fmt.Fprintln(os.Stderr, "  anvil task cache my-task --clear  # Clear cache")
		os.Exit(1)
	}

	var clearFlag bool
	fs.BoolVar(&clearFlag, "clear", false, "Clear cached output for the task")

	if err := fs.Parse(args); err != nil {
		fs.Usage()
	}

	if len(fs.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "error: task name required\n")
		fs.Usage()
	}

	taskName := fs.Args()[0]

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		log.Fatalf("failed to load todos: %v", err)
	}

	todo := findTodo(todos, taskName)
	if todo == nil {
		fmt.Fprintf(os.Stderr, "error: task not found: %s\n", taskName)
		os.Exit(1)
	}

	if !cache.IsCacheEnabled(*todo) {
		fmt.Printf("Cache status: DISABLED\n")
		return
	}

	cacheKey, err := cache.CalculateCacheKey(*todo, abs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error calculating cache key: %v\n", err)
		os.Exit(1)
	}

	if clearFlag {
		cachePath := cache.GetCacheFilePath(abs, cacheKey)
		if _, err := os.Stat(cachePath); err == nil {
			if err := os.Remove(cachePath); err != nil {
				fmt.Fprintf(os.Stderr, "error clearing cache: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Cache cleared for %s\n", taskName)
		} else {
			fmt.Printf("No cache found for %s\n", taskName)
		}
		return
	}

	cacheEntry, err := cache.GetCache(abs, cacheKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading cache: %v\n", err)
		os.Exit(1)
	}

	if cacheEntry == nil {
		fmt.Printf("Cache status: MISS (no cached output)\n")
		fmt.Printf("Cache key: %s\n", cacheKey[:8])
	} else {
		// Calculate cache size
		size := len(cacheEntry.Content)
		sizeStr := fmt.Sprintf("%dB", size)
		if size > 1024 {
			sizeStr = fmt.Sprintf("%.1fKB", float64(size)/1024)
		}
		if size > 1024*1024 {
			sizeStr = fmt.Sprintf("%.1fMB", float64(size)/(1024*1024))
		}

		expiresIn := time.Until(cacheEntry.ExpiresAt)
		status := "HIT"
		if expiresIn < 0 {
			status = "EXPIRED"
		}

		fmt.Printf("Cache status: %s (expires in %v)\n", status, expiresIn.Round(time.Second))
		fmt.Printf("Cache key: %s\n", cacheKey[:8])
		fmt.Printf("Cached output: %s\n", sizeStr)
	}
}

func taskBlameCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: anvil task blame <name>\n")
		os.Exit(1)
	}

	taskName := args[0]

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		log.Fatalf("failed to load todos: %v", err)
	}

	todo := findTodo(todos, taskName)
	if todo == nil {
		fmt.Fprintf(os.Stderr, "error: task not found: %s\n", taskName)
		os.Exit(1)
	}

	// Check if project is git-tracked
	checkCmd := exec.Command("git", "rev-parse", "--git-dir")
	checkCmd.Dir = abs
	if err := checkCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "git blame not available: project is not in a git repository\n")
		os.Exit(1)
	}

	// Run git blame
	blameCmd := exec.Command("git", "blame", todo.Path)
	blameCmd.Dir = abs
	blameCmd.Stdout = os.Stdout
	blameCmd.Stderr = os.Stderr
	if err := blameCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "git blame failed: %v\n", err)
		os.Exit(1)
	}
}

// taskSubscriptionCmd handles the 'anvil task subscription' command
