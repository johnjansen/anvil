package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/johnjansen/anvil/internal/daemon"
)

// taskHealthCmd shows task health status.
func taskHealthCmd(args []string) {
	fs := flag.NewFlagSet("task health", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: anvil task health [task-name] [options]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Show health status for tasks with health checks configured.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  --check     Force a health check run")
		fmt.Fprintln(os.Stderr, "  --verbose   Show detailed health information")
		fmt.Fprintln(os.Stderr, "  --json      Output in JSON format")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  anvil task health              # Show all task health")
		fmt.Fprintln(os.Stderr, "  anvil task health my-task      # Show specific task health")
		fmt.Fprintln(os.Stderr, "  anvil task health --check      # Force health check")
		os.Exit(1)
	}

	var checkFlag, verboseFlag, jsonFlag bool
	fs.BoolVar(&checkFlag, "check", false, "Force a health check run")
	fs.BoolVar(&verboseFlag, "verbose", false, "Show detailed health information")
	fs.BoolVar(&jsonFlag, "json", false, "Output in JSON format")

	if err := fs.Parse(args); err != nil {
		fs.Usage()
	}

	taskName := ""
	if len(fs.Args()) > 0 {
		taskName = fs.Args()[0]
	}

	// Communicate with the daemon to get health information
	if !daemon.IsDaemonRunning() {
		fmt.Fprintln(os.Stderr, "Error: daemon is not running")
		os.Exit(1)
	}

	// Get detailed health information which includes task health
	health, err := daemon.SendHealthRequest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting health status: %v\n", err)
		os.Exit(1)
	}

	if jsonFlag {
		if taskName != "" {
			// Find specific task health
			for _, taskHealth := range health.TaskHealth {
				if taskHealth.Name == taskName {
					jsonData, _ := json.Marshal(taskHealth)
					fmt.Println(string(jsonData))
					return
				}
			}
			fmt.Printf(`{"task": "%s", "health": "not_found", "message": "Task not found or no health check configured"}`, taskName)
			fmt.Println()
		} else {
			// Show all task health
			jsonData, _ := json.Marshal(health.TaskHealth)
			fmt.Println(string(jsonData))
		}
		return
	}

	if taskName != "" {
		// Show specific task health
		for _, taskHealth := range health.TaskHealth {
			if taskHealth.Name == taskName {
				fmt.Printf("Health status for task '%s':\n", taskName)
				if taskHealth.Healthy {
					fmt.Println("  Status: Healthy")
				} else {
					fmt.Println("  Status: Unhealthy")
				}
				fmt.Printf("  Last check: %s ago\n", time.Since(taskHealth.LastCheck).Round(time.Second))
				fmt.Printf("  Exit code: %d\n", taskHealth.ExitCode)
				if taskHealth.Error != "" {
					fmt.Printf("  Error: %s\n", taskHealth.Error)
				}
				return
			}
		}
		fmt.Printf("Task '%s' not found or no health check configured\n", taskName)
	} else {
		// Show all task health
		if len(health.TaskHealth) == 0 {
			fmt.Println("No tasks with health checks configured")
			return
		}

		fmt.Println("Task Health Status:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TASK\tHEALTH\tLAST_CHECK")

		for _, taskHealth := range health.TaskHealth {
			status := "healthy"
			if !taskHealth.Healthy {
				status = "unhealthy"
			}
			lastCheck := time.Since(taskHealth.LastCheck).Round(time.Second)
			fmt.Fprintf(w, "%s\t%s\t%s ago\n", taskHealth.Name, status, lastCheck)
		}
		w.Flush()

		if checkFlag {
			fmt.Println("\nNote: Health checks are run automatically every 30 seconds")
		}
	}
}
