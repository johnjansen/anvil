package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/project"
)

var validActivityTypes = []string{"created", "run", "paused", "resumed", "edited", "killed", "unlocked", "force-run"}

func taskActivityCmd(args []string) {
	typeFilter := ""
	sinceDate := ""
	limit := 100
	exportPath := ""
	jsonOutput := false
	var rest []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-t", "--type":
			if i+1 >= len(args) {
				log.Fatal("missing value for --type")
			}
			i++
			typeFilter = args[i]
		case "--since":
			if i+1 >= len(args) {
				log.Fatal("missing value for --since")
			}
			i++
			sinceDate = args[i]
		case "-l", "--limit":
			if i+1 >= len(args) {
				log.Fatal("missing value for --limit")
			}
			i++
			fmt.Sscanf(args[i], "%d", &limit)
		case "-e", "--export":
			if i+1 >= len(args) {
				log.Fatal("missing value for --export")
			}
			i++
			exportPath = args[i]
		case "--json":
			jsonOutput = true
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `usage: anvil task activity <name> [options]

Show task activity history.

Options:
  -t, --type <type>    Filter by activity type (created, run, paused, resumed, edited, killed, unlocked, force-run)
  --since <date>       Show entries since date (YYYY-MM-DD)
  -l, --limit <n>      Maximum entries to display (default: 100)
  -e, --export <path>  Export entries to JSON file
  --json               Output as JSON to stdout
`)
			os.Exit(0)
		default:
			rest = append(rest, args[i])
		}
	}

	if len(rest) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task activity <name> [--type TYPE] [--since DATE] [--limit N] [--export FILE] [--json]\n")
		os.Exit(1)
	}

	// Validate type filter
	if typeFilter != "" {
		valid := false
		for _, t := range validActivityTypes {
			if t == typeFilter {
				valid = true
				break
			}
		}
		if !valid {
			fmt.Fprintf(os.Stderr, "invalid activity type: %s. Valid types: %s\n", typeFilter, strings.Join(validActivityTypes, ", "))
			os.Exit(1)
		}
	}

	// Parse since date
	var sinceTime time.Time
	if sinceDate != "" {
		t, err := time.Parse("2006-01-02", sinceDate)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid date format: %s. Use YYYY-MM-DD\n", sinceDate)
			os.Exit(1)
		}
		sinceTime = t
	}

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

	todo := findTodo(todos, rest[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", rest[0])
		os.Exit(1)
	}

	entries, err := project.ReadActivities(abs, todo.ID)
	if err != nil {
		log.Fatalf("failed to read activities: %v", err)
	}

	if len(entries) == 0 {
		fmt.Printf("No activity entries for task %s\n", rest[0])
		return
	}

	// Apply filters
	var filtered []project.ActivityEntry
	for _, e := range entries {
		if typeFilter != "" && e.Action != typeFilter {
			continue
		}
		if !sinceTime.IsZero() && e.Timestamp.Before(sinceTime) {
			continue
		}
		filtered = append(filtered, e)
	}

	if len(filtered) == 0 {
		fmt.Println("No matching activity entries")
		return
	}

	// Reverse chronological order (newest first)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	// Apply limit
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	// Export to file
	if exportPath != "" {
		data, err := json.MarshalIndent(filtered, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		if err := os.WriteFile(exportPath, data, 0644); err != nil {
			log.Fatalf("failed to write export file: %v", err)
		}
		fmt.Printf("Exported %d activity entries to %s\n", len(filtered), exportPath)
		return
	}

	// JSON output to stdout
	if jsonOutput {
		data, err := json.MarshalIndent(filtered, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	// Table output
	fmt.Printf("%-22s  %-12s  %s\n", "TIMESTAMP", "ACTION", "DETAILS")
	for _, e := range filtered {
		details := formatActivityDetails(e.Details)
		fmt.Printf("%-22s  %-12s  %s\n", e.Timestamp.Format("2006-01-02 15:04:05"), e.Action, details)
	}
}

func formatActivityDetails(details map[string]string) string {
	if len(details) == 0 {
		return ""
	}
	var parts []string
	// Show key fields in a specific order for readability
	keyOrder := []string{"run_id", "exit_code", "success", "error", "duration", "priority", "schedule", "graceful", "changed_fields"}
	seen := make(map[string]bool)
	for _, k := range keyOrder {
		if v, ok := details[k]; ok {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
			seen[k] = true
		}
	}
	// Show remaining keys
	for k, v := range details {
		if !seen[k] {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return strings.Join(parts, " ")
}
