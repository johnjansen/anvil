package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/project"
)

func suggestStagger(schedule string, overlapCount int) {
	// Parse the cron fields to offer meaningful suggestions
	fields := strings.Fields(schedule)
	if len(fields) < 5 {
		return
	}
	minute := fields[0]
	// If it's a fixed minute (e.g. "0" or "30"), suggest offsetting
	if _, err := fmt.Sscanf(minute, "%d", new(int)); err == nil {
		offset := (overlapCount + 1) * 5 // suggest 5-min offsets
		if offset >= 60 {
			offset = offset % 60
		}
		suggested := make([]string, len(fields))
		copy(suggested, fields)
		suggested[0] = fmt.Sprintf("%d", offset)
		fmt.Fprintf(os.Stderr, "Hint: Consider staggering with schedule %q to avoid overlap\n", strings.Join(suggested, " "))
	} else if strings.HasPrefix(minute, "*/") {
		// Interval-based, suggest a different offset start
		fmt.Fprintf(os.Stderr, "Hint: Consider staggering schedules or increasing max_workers in .anvil/config.yaml\n")
	}
}

func taskOverlapsCmd(args []string) {
	allProjects := false
	for _, a := range args {
		switch a {
		case "-a", "--all":
			allProjects = true
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `usage: anvil task overlaps [-a|--all]

Show all schedule conflicts and overlapping task runs.

Groups tasks by time slot to identify scheduling bottlenecks.

Options:
  -a, --all    Check across all watched projects
`)
			os.Exit(0)
		}
	}

	type parsedTask struct {
		name     string
		project  string
		schedule string
		parser   *cron.Parser
	}

	var tasks []parsedTask

	type projectTodos struct {
		path  string
		todos []project.Todo
	}
	var projects []projectTodos

	if allProjects {
		watched, err := loadAllWatched()
		if err != nil {
			log.Fatalf("failed to read watched: %v", err)
		}
		for _, w := range watched {
			proj, err := project.Load(w.Path)
			if err != nil {
				continue
			}
			todos, _ := proj.LoadTodos()
			projects = append(projects, projectTodos{path: w.Path, todos: todos})
		}
	} else {
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
		projects = append(projects, projectTodos{path: abs, todos: todos})
	}

	// Load max_workers for display
	maxWorkers := 4
	cfg, _ := config.Load()
	if cfg != nil && cfg.MaxWorkers > 0 {
		maxWorkers = cfg.MaxWorkers
	}

	// Parse schedules
	for _, p := range projects {
		projName := filepath.Base(p.path)
		for _, t := range p.todos {
			if t.Schedule == "" || t.Schedule == "persistent" || t.Disabled {
				continue
			}
			parser, err := cron.Parse(t.Schedule)
			if err != nil {
				continue
			}
			name := strings.TrimSuffix(t.Name, ".md")
			tasks = append(tasks, parsedTask{
				name:     name,
				project:  projName,
				schedule: t.Schedule,
				parser:   parser,
			})
		}
	}

	if len(tasks) == 0 {
		fmt.Println("no scheduled tasks to analyze")
		return
	}

	// Generate next 60 minutes of run times and group by minute
	now := time.Now().Truncate(time.Minute)
	window := 24 * time.Hour
	minuteMap := make(map[time.Time][]string) // minute -> task names
	for _, t := range tasks {
		cur := now
		for i := 0; i < 1440; i++ { // up to 1440 minutes in a day
			next, err := t.parser.Next(cur)
			if err != nil {
				break
			}
			if next.After(now.Add(window)) {
				break
			}
			key := next.Truncate(time.Minute)
			minuteMap[key] = append(minuteMap[key], t.name)
			cur = next
		}
	}

	// Collect conflicts (minutes with >1 task), sorted by time
	type conflict struct {
		minute time.Time
		tasks  []string
	}
	var conflicts []conflict
	for minute, names := range minuteMap {
		if len(names) > 1 {
			conflicts = append(conflicts, conflict{minute: minute, tasks: names})
		}
	}

	// Sort by time
	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].minute.Before(conflicts[j].minute)
	})

	// Deduplicate by task group (same set of tasks at different times)
	type groupKey struct {
		key   string
		times []string
		tasks []string
		count int
	}
	groups := make(map[string]*groupKey)
	for _, c := range conflicts {
		sorted := make([]string, len(c.tasks))
		copy(sorted, c.tasks)
		sort.Strings(sorted)
		key := strings.Join(sorted, "+")
		if g, ok := groups[key]; ok {
			g.count++
			if len(g.times) < 3 {
				g.times = append(g.times, c.minute.Format("15:04"))
			}
		} else {
			groups[key] = &groupKey{
				key:   key,
				times: []string{c.minute.Format("15:04")},
				tasks: sorted,
				count: 1,
			}
		}
	}

	if len(groups) == 0 {
		fmt.Printf("OK: %d scheduled task(s) — no overlapping runs detected in the next 24h\n", len(tasks))
		return
	}

	// Print results in table format
	fmt.Printf("Schedule overlaps found (next 24h, %d worker slots):\n\n", maxWorkers)
	fmt.Printf("%-12s  %-6s  %s\n", "TIME", "TASKS", "NAMES")
	fmt.Printf("%-12s  %-6s  %s\n", "----", "-----", "-----")

	// Sort groups by first occurrence time
	var groupList []*groupKey
	for _, g := range groups {
		groupList = append(groupList, g)
	}
	sort.Slice(groupList, func(i, j int) bool {
		return groupList[i].times[0] < groupList[j].times[0]
	})

	totalConflicts := 0
	for _, g := range groupList {
		timeStr := strings.Join(g.times, ", ")
		if g.count > len(g.times) {
			timeStr += fmt.Sprintf(" (+%d more)", g.count-len(g.times))
		}
		severity := "Note"
		if len(g.tasks) >= maxWorkers {
			severity = "WARNING"
		} else if len(g.tasks) >= 3 {
			severity = "Warning"
		}

		fmt.Printf("%-12s  %-6d  %s", g.times[0], len(g.tasks), strings.Join(g.tasks, ", "))
		if len(g.tasks) >= maxWorkers {
			fmt.Printf(" (%d tasks >= %d workers)", len(g.tasks), maxWorkers)
		}
		fmt.Println()

		if g.count > 1 {
			fmt.Printf("  %s: repeats %d times in 24h (%s)\n", severity, g.count, timeStr)
		} else {
			if severity != "Note" {
				fmt.Printf("  %s: %d tasks competing for %d worker slots\n", severity, len(g.tasks), maxWorkers)
			}
		}
		totalConflicts += g.count
	}

	fmt.Printf("\n%d overlap(s) across %d time group(s). Consider staggering schedules or increasing max_workers.\n", totalConflicts, len(groupList))
	os.Exit(1)
}

// formatDurationShort formats a duration as short human-readable relative time.
func formatDurationShort(d time.Duration) string {
	if d < 0 {
		d = -d
		if d < time.Minute {
			return "now"
		}
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd%dh", int(d.Hours()/24), int(d.Hours())%24)
}
