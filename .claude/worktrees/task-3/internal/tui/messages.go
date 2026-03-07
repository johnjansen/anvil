package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/johnjansen/anvil/internal/daemon"
)

type tickMsg time.Time

type dataMsg struct {
	tasks   []daemon.TaskInfo
	queue   []daemon.TaskQueueInfo
	status  *daemon.DaemonStatus
	watched []watchFrontmatter
}

type errMsg error

type watchFrontmatter struct {
	Path      string    `yaml:"path"`
	WatchedAt time.Time `yaml:"watched_at"`
}

func refreshData() tea.Cmd {
	return func() tea.Msg {
		// Gather data
		tasks, _ := daemon.SendPsRequest()
		queue, _ := daemon.SendQueueRequest()
		status, _ := daemon.SendStatusRequest()
		watched, _ := loadAllWatched()

		return dataMsg{
			tasks:   tasks,
			queue:   queue,
			status:  status,
			watched: watched,
		}
	}
}

// loadAllWatched loads all watched projects
// This is a placeholder - would need to properly implement
func loadAllWatched() ([]watchFrontmatter, error) {
	// This would need to be implemented properly based on the existing code
	return []watchFrontmatter{}, nil
}