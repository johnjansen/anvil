package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
)

func taskResetBudgetCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task reset-budget <name>\n")
		os.Exit(1)
	}

	if !daemon.IsDaemonRunning() {
		fmt.Fprintf(os.Stderr, "daemon is not running\n")
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

	todos, err := proj.LoadTodos()
	if err != nil {
		log.Fatalf("failed to load todos: %v", err)
	}

	todo := findTodo(todos, args[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", args[0])
		os.Exit(1)
	}

	if !todo.IsPersistent() {
		fmt.Fprintf(os.Stderr, "task %s is not a persistent task\n", todo.Name)
		os.Exit(1)
	}

	taskKey := fmt.Sprintf("%s/%s", abs, todo.Name)
	if err := daemon.SendResetBudgetRequest(taskKey); err != nil {
		log.Fatalf("failed to reset budget: %v", err)
	}

	fmt.Printf("Budget reset for %s\n", todo.Name)
}

func taskSlaCmd(args []string) {
	verbose := false
	reset := false
	jsonOutput := false
	for _, a := range args {
		switch a {
		case "--verbose", "-v":
			verbose = true
		case "--reset":
			reset = true
		case "--json":
			jsonOutput = true
		}
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

	// Filter tasks with SLA configured
	type slaEntry struct {
		Name         string `json:"name"`
		MaxDelay     string `json:"max_delay"`
		Strict       bool   `json:"strict,omitempty"`
		LastDelay    string `json:"last_delay,omitempty"`
		Violation    bool   `json:"violation"`
		ScheduledAt  string `json:"scheduled_at,omitempty"`
		DispatchedAt string `json:"dispatched_at,omitempty"`
	}

	var entries []slaEntry
	for _, t := range todos {
		if t.SLA.MaxDelay <= 0 {
			continue
		}

		entry := slaEntry{
			Name:     t.Name,
			MaxDelay: t.SLA.MaxDelay.String(),
			Strict:   t.SLA.Strict,
		}

		rec, recErr := project.ReadCurrentRunRecord(abs, t.ID)
		if recErr == nil && rec.SLAMaxDelay > 0 {
			entry.LastDelay = rec.DispatchDelay.String()
			entry.Violation = rec.SLAViolation
			if !rec.ScheduledTime.IsZero() {
				entry.ScheduledAt = rec.ScheduledTime.Format(time.RFC3339)
			}
			if !rec.Started.IsZero() {
				entry.DispatchedAt = rec.Started.Format(time.RFC3339)
			}
		}

		if verbose || entry.Violation {
			entries = append(entries, entry)
		}
	}

	if reset {
		resetCount := 0
		for _, t := range todos {
			if t.SLA.MaxDelay <= 0 {
				continue
			}
			records, err := project.ReadAllRunRecords(abs, t.ID)
			if err != nil {
				continue
			}
			for _, rec := range records {
				if rec.SLAViolation {
					rec.SLAViolation = false
					if writeErr := project.WriteRunRecord(abs, rec); writeErr == nil {
						resetCount++
					}
				}
			}
		}
		fmt.Printf("Reset %d SLA violation(s)\n", resetCount)
		return
	}

	if len(entries) == 0 {
		if verbose {
			fmt.Println("No tasks have SLA tracking enabled.")
		} else {
			fmt.Println("No SLA violations found.")
		}
		return
	}

	if jsonOutput {
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	// Human-readable output
	for _, e := range entries {
		status := "OK"
		if e.Violation {
			status = "VIOLATION"
		}
		fmt.Printf("%-30s  SLA: %s max delay  %s", e.Name, e.MaxDelay, status)
		if e.LastDelay != "" {
			fmt.Printf("  (last: %s delay)", e.LastDelay)
		}
		fmt.Println()
	}
}

func taskAlertsCmd(args []string) {
	// Handle subcommands: list, ack, history
	if len(args) == 0 {
		// Default: show active alerts (list)
		showAlerts(false)
		return
	}

	switch args[0] {
	case "list":
		showAlerts(false)
	case "history":
		showAlerts(true)
	case "ack":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "usage: anvil task alerts ack <alert-id>\n")
			os.Exit(1)
		}
		acknowledgeAlert(args[1])
	default:
		fmt.Fprintf(os.Stderr, "unknown alerts subcommand: %s\n", args[0])
		fmt.Fprintf(os.Stderr, "Usage: anvil task alerts [list|history|ack <id>]\n")
		os.Exit(1)
	}
}

func taskRateLimitsCmd(args []string) {
	reset := false
	jsonOutput := false
	for _, a := range args {
		switch a {
		case "--reset":
			reset = true
		case "--json":
			jsonOutput = true
		}
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

	// Filter tasks with rate limits configured
	type rateLimitEntry struct {
		Name        string  `json:"name"`
		ThisHour    int     `json:"this_hour"`
		MaxPerHour  int     `json:"max_per_hour"`
		HourPercent float64 `json:"hour_percent"`
		ThisDay     int     `json:"this_day"`
		MaxPerDay   int     `json:"max_per_day"`
		DayPercent  float64 `json:"day_percent"`
	}

	var entries []rateLimitEntry
	now := time.Now()

	for _, t := range todos {
		if t.RateLimit.MaxPerHour <= 0 && t.RateLimit.MaxPerDay <= 0 {
			continue
		}

		counter, counterErr := project.ReadRateLimitCounter(abs, t.ID)
		if counterErr != nil {
			log.Printf("Warning: failed to read rate limit counter for %s: %v", t.Name, counterErr)
			continue
		}

		// Reset counters if periods have passed
		if counter.ThisHourStart.Add(time.Hour).Before(now) {
			counter.ThisHourCount = 0
			counter.ThisHourStart = now.Truncate(time.Hour)
		}
		if counter.ThisDayStart.Add(24 * time.Hour).Before(now) {
			counter.ThisDayCount = 0
			counter.ThisDayStart = now.Truncate(24 * time.Hour)
		}

		entry := rateLimitEntry{
			Name: t.Name,
		}

		if t.RateLimit.MaxPerHour > 0 {
			entry.ThisHour = counter.ThisHourCount
			entry.MaxPerHour = t.RateLimit.MaxPerHour
			entry.HourPercent = float64(counter.ThisHourCount) / float64(t.RateLimit.MaxPerHour) * 100
		}

		if t.RateLimit.MaxPerDay > 0 {
			entry.ThisDay = counter.ThisDayCount
			entry.MaxPerDay = t.RateLimit.MaxPerDay
			entry.DayPercent = float64(counter.ThisDayCount) / float64(t.RateLimit.MaxPerDay) * 100
		}

		entries = append(entries, entry)
	}

	if reset {
		resetCount := 0
		for _, t := range todos {
			if t.RateLimit.MaxPerHour > 0 || t.RateLimit.MaxPerDay > 0 {
				if resetErr := project.ResetRateLimitCounter(abs, t.ID); resetErr == nil {
					resetCount++
				}
			}
		}
		fmt.Printf("Reset %d rate limit counter(s)\n", resetCount)
		return
	}

	if len(entries) == 0 {
		if jsonOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("No tasks have rate limits configured.")
		}
		return
	}

	if jsonOutput {
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	// Human-readable output
	fmt.Printf("%-30s  %-15s  %-15s\n", "TASK", "THIS_HOUR", "THIS_DAY")
	fmt.Printf("%-30s  %-15s  %-15s\n", strings.Repeat("-", 30), strings.Repeat("-", 15), strings.Repeat("-", 15))
	for _, e := range entries {
		hourDisplay := "N/A"
		if e.MaxPerHour > 0 {
			hourDisplay = fmt.Sprintf("%d/%d", e.ThisHour, e.MaxPerHour)
			if e.HourPercent >= 80 {
				hourDisplay += fmt.Sprintf(" (%.0f%%)", e.HourPercent)
			}
		}

		dayDisplay := "N/A"
		if e.MaxPerDay > 0 {
			dayDisplay = fmt.Sprintf("%d/%d", e.ThisDay, e.MaxPerDay)
			if e.DayPercent >= 80 {
				dayDisplay += fmt.Sprintf(" (%.0f%%)", e.DayPercent)
			}
		}

		fmt.Printf("%-30s  %-15s  %-15s\n", e.Name, hourDisplay, dayDisplay)
	}
}

func showAlerts(showHistory bool) {
	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	alertStorage := daemon.NewAlertStorage(filepath.Join(abs, ".anvil", "alerts"))
	allAlerts, err := alertStorage.LoadAllAlerts()
	if err != nil {
		log.Fatalf("failed to load alerts: %v", err)
	}

	if len(allAlerts) == 0 {
		fmt.Println("No alerts found")
		return
	}

	for taskID, alerts := range allAlerts {
		for _, alert := range alerts {
			if !showHistory && alert.Acknowledged {
				continue
			}
			fmt.Printf("[%s] %s | %s | %s | %s\n",
				alert.ID[:8],
				taskID,
				alert.RuleName,
				alert.Severity,
				alert.Message)
			if alert.Acknowledged && alert.AcknowledgedAt != nil {
				fmt.Printf("  Acknowledged: %s\n", alert.AcknowledgedAt.Format(time.RFC3339))
			}
		}
	}
}

func acknowledgeAlert(alertID string) {
	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	alertStorage := daemon.NewAlertStorage(filepath.Join(abs, ".anvil", "alerts"))
	allAlerts, err := alertStorage.LoadAllAlerts()
	if err != nil {
		log.Fatalf("failed to load alerts: %v", err)
	}

	// Find the alert and its task
	found := false
	for taskID, alerts := range allAlerts {
		for _, alert := range alerts {
			if alert.ID == alertID || alert.ID[:8] == alertID {
				if err := alertStorage.AcknowledgeAlert(taskID, alert.ID); err != nil {
					log.Fatalf("failed to acknowledge alert: %v", err)
				}
				fmt.Printf("Alert %s acknowledged\n", alert.ID[:8])
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		log.Fatalf("alert not found: %s", alertID)
	}
}
