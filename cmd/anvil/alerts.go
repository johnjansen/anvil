package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"path/filepath"

	"github.com/johnjansen/anvil/internal/project"
)

func alertsCmd(args []string) {
	if len(args) > 0 {
		switch args[0] {
		case "ack":
			alertsAckCmd(args[1:])
			return
		case "history":
			alertsHistoryCmd(args[1:])
			return
		}
	}

	// List active (unacknowledged) alerts
	taskFilter := ""
	severityFilter := ""
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--task":
			if i+1 < len(args) {
				i++
				taskFilter = args[i]
			}
		case "--severity":
			if i+1 < len(args) {
				i++
				severityFilter = args[i]
			}
		case "--json":
			jsonOut = true
		}
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	proj, err := project.Load(abs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	records, err := project.ReadAllAlertRecords(proj.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading alerts: %v\n", err)
		os.Exit(1)
	}

	// Filter to unacknowledged only
	var active []project.AlertRecord
	for _, r := range records {
		if r.Acknowledged {
			continue
		}
		if taskFilter != "" && r.TaskName != taskFilter {
			continue
		}
		if severityFilter != "" && r.Severity != severityFilter {
			continue
		}
		active = append(active, r)
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(active)
		return
	}

	if len(active) == 0 {
		fmt.Println("No active alerts.")
		return
	}

	for _, a := range active {
		runPrefix := a.RunID
		if len(runPrefix) > 8 {
			runPrefix = runPrefix[:8]
		}
		fmt.Printf("[%s] %s  %s/%s  %s  (%s)\n",
			a.Severity, a.ID, a.TaskName, runPrefix, a.AlertName, a.Timestamp.Format(time.RFC3339))
		if a.Message != "" {
			fmt.Printf("  %s\n", a.Message)
		}
	}
}

func alertsAckCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: anvil alerts ack <alert-id>")
		os.Exit(1)
	}
	alertID := args[0]

	abs, err := filepath.Abs(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	proj, err := project.Load(abs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := project.AckAlertRecord(proj.Path, alertID); err != nil {
		fmt.Fprintf(os.Stderr, "error acknowledging alert: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Alert %s acknowledged.\n", alertID)
}

func alertsHistoryCmd(args []string) {
	taskFilter := ""
	limit := 50
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--task":
			if i+1 < len(args) {
				i++
				taskFilter = args[i]
			}
		case "--limit":
			if i+1 < len(args) {
				i++
				fmt.Sscanf(args[i], "%d", &limit)
			}
		case "--json":
			jsonOut = true
		}
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	proj, err := project.Load(abs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	records, err := project.ReadAllAlertRecords(proj.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading alerts: %v\n", err)
		os.Exit(1)
	}

	// Filter by task
	if taskFilter != "" {
		var filtered []project.AlertRecord
		for _, r := range records {
			if r.TaskName == taskFilter {
				filtered = append(filtered, r)
			}
		}
		records = filtered
	}

	// Apply limit
	if len(records) > limit {
		records = records[:limit]
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(records)
		return
	}

	if len(records) == 0 {
		fmt.Println("No alert history.")
		return
	}

	for _, a := range records {
		ackStr := ""
		if a.Acknowledged {
			ackStr = " [acked]"
		}
		runPrefix := a.RunID
		if len(runPrefix) > 8 {
			runPrefix = runPrefix[:8]
		}
		fmt.Printf("[%s] %s  %s/%s  %s  (%s)%s\n",
			a.Severity, a.ID, a.TaskName, runPrefix, a.AlertName, a.Timestamp.Format(time.RFC3339), ackStr)
		if a.Message != "" {
			fmt.Printf("  %s\n", a.Message)
		}
	}
}
// compile guard
var _ = strings.HasPrefix
