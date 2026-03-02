package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
	"github.com/johnjansen/anvil/internal/cron"
)

type model struct {
	DaemonPID   int
	MaxWorkers  int
	tasks       []daemon.TaskInfo
	queue       []daemon.TaskQueueInfo
	status      *daemon.DaemonStatus
	watched     []watchFrontmatter
	width       int
	height      int
	quitting    bool
	err         error
	lastUpdated time.Time
}

func NewModel(daemonPID, maxWorkers int) model {
	return model{
		DaemonPID:   daemonPID,
		MaxWorkers:  maxWorkers,
		lastUpdated: time.Now(),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		refreshData(),
		tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case dataMsg:
		m.tasks = msg.tasks
		m.queue = msg.queue
		m.status = msg.status
		m.watched = msg.watched
		m.lastUpdated = time.Now()
	case tickMsg:
		return m, refreshData()
	case errMsg:
		m.err = msg
		return m, nil
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit", m.err)
	}

	var sb strings.Builder

	// Header
	sb.WriteString(m.renderHeader())
	sb.WriteString("\n\n")

	// Running tasks
	sb.WriteString(m.renderRunningTasks())
	sb.WriteString("\n\n")

	// Idle workers
	sb.WriteString(m.renderIdleWorkers())
	sb.WriteString("\n\n")

	// Pending tasks
	sb.WriteString(m.renderPendingTasks())
	sb.WriteString("\n\n")

	// Skipped tasks
	sb.WriteString(m.renderSkippedTasks())
	sb.WriteString("\n\n")

	// Next scheduled
	sb.WriteString(m.renderNextScheduled())
	sb.WriteString("\n\n")

	// Footer
	sb.WriteString("Press 'q' to quit • Last updated: " + m.lastUpdated.Format("15:04:05"))

	return sb.String()
}

func (m model) renderHeader() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		MarginBottom(1)

	// Count total todos across projects
	totalTasks := 0
	for _, wp := range m.watched {
		proj, err := project.Load(wp.Path)
		if err != nil {
			continue
		}
		todos, _ := proj.LoadTodos()
		totalTasks += len(todos)
	}

	drainNote := ""
	if m.status != nil && m.status.Draining {
		drainNote = " (draining)"
	}

	headerText := fmt.Sprintf("anvil daemon (PID %d), %d projects, %d tasks — %d workers%s",
		m.DaemonPID, len(m.watched), totalTasks, m.MaxWorkers, drainNote)

	return headerStyle.Render(headerText)
}

func (m model) renderRunningTasks() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("2"))

	countStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		PaddingLeft(1)

	content := fmt.Sprintf("RUNNING (%d)", len(m.tasks))
	if len(m.tasks) == 0 {
		content += "\n  (none)"
	} else {
		for _, t := range m.tasks {
			projectName := projectNameFromPath(t.Project)
			statusStr := ""
			if t.Status != "" {
				statusStr = "  " + t.Status
			}
			content += fmt.Sprintf("\n  %-40s %8s%s",
				truncate(projectName+"/"+t.Name, 40),
				t.Elapsed,
				statusStr)
		}
	}

	return titleStyle.Render("RUNNING") + countStyle.Render(fmt.Sprintf("(%d)", len(m.tasks))) + "\n" + content
}

func (m model) renderIdleWorkers() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("3"))

	idleCount := m.MaxWorkers - len(m.tasks)
	if idleCount < 0 {
		idleCount = 0
	}

	content := fmt.Sprintf("IDLE (%d)", idleCount)

	return titleStyle.Render("IDLE") + " " + content
}

func (m model) renderPendingTasks() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("4"))

	var pending []daemon.TaskQueueInfo
	for _, q := range m.queue {
		if q.Status == "pending" {
			pending = append(pending, q)
		}
	}

	if len(pending) == 0 {
		return ""
	}

	content := fmt.Sprintf("PENDING (%d)", len(pending))
	for _, q := range pending {
		projectName := projectNameFromPath(q.Project)
		content += fmt.Sprintf("\n  %-40s %s",
			truncate(projectName+"/"+q.Name, 40),
			q.Schedule)
	}

	return titleStyle.Render("PENDING") + " " + content
}

func (m model) renderSkippedTasks() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("1"))

	var skipped []daemon.TaskQueueInfo
	for _, q := range m.queue {
		if q.Status == "skipped" {
			skipped = append(skipped, q)
		}
	}

	if len(skipped) == 0 {
		return ""
	}

	content := fmt.Sprintf("SKIPPED (%d)", len(skipped))
	for _, q := range skipped {
		projectName := projectNameFromPath(q.Project)
		reason := q.SkipReason
		if reason == "" {
			reason = "unknown"
		}
		content += fmt.Sprintf("\n  %-40s %s",
			truncate(projectName+"/"+q.Name, 40),
			reason)
	}

	return titleStyle.Render("SKIPPED") + " " + content
}

func (m model) renderNextScheduled() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("5"))

	type nextTask struct {
		project  string
		name     string
		schedule string
		nextRun  time.Time
	}
	var upcoming []nextTask
	now := time.Now()
	for _, wp := range m.watched {
		proj, err := project.Load(wp.Path)
		if err != nil {
			continue
		}
		todos, _ := proj.LoadTodos()
		for _, t := range todos {
			if t.Schedule == "" || t.Schedule == "once" {
				continue
			}
			p, err := ParseSchedule(t.Schedule)
			if err != nil {
				continue
			}
			next, err := p.Next(now)
			if err != nil {
				continue
			}
			upcoming = append(upcoming, nextTask{
				project:  projectNameFromPath(wp.Path),
				name:     t.Name,
				schedule: t.Schedule,
				nextRun:  next,
			})
		}
	}
	// Sort by next run time
	// sort.Slice(upcoming, func(i, j int) bool {
	// 	return upcoming[i].nextRun.Before(upcoming[j].nextRun)
	// })
	// Show up to 5
	if len(upcoming) > 5 {
		upcoming = upcoming[:5]
	}

	if len(upcoming) == 0 {
		return ""
	}

	content := "NEXT SCHEDULED"
	for _, u := range upcoming {
		until := time.Until(u.nextRun)
		var untilStr string
		if until < time.Minute {
			untilStr = fmt.Sprintf("in %ds", int(until.Seconds()))
		} else if until < time.Hour {
			untilStr = fmt.Sprintf("in %dm%ds", int(until.Minutes()), int(until.Seconds())%60)
		} else {
			untilStr = fmt.Sprintf("in %dh%dm", int(until.Hours()), int(until.Minutes())%60)
		}
		content += fmt.Sprintf("\n  %-6s %-34s %s",
			u.schedule,
			truncate(u.project+"/"+u.name, 34),
			untilStr)
	}

	return titleStyle.Render("NEXT SCHEDULED") + "\n" + content
}

// Helper functions
func projectNameFromPath(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func truncate(s string, maxLen int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// Placeholder for schedule parsing - would need to import the cron package
func ParseSchedule(schedule string) (*cron.Parser, error) {
	// This is a placeholder - would need to properly implement with the cron package
	return cron.Parse(schedule)
}