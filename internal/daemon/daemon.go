package daemon

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/project"
	"github.com/johnjansen/anvil/internal/runner"

	"gopkg.in/yaml.v3"
)

type Daemon struct {
	config   *config.Config
	runner   *runner.Runner
	busy     map[string]bool
	mu       sync.Mutex
	stop     chan struct{}
	done     chan struct{}
	lastTick time.Time // last minute we processed (truncated to minute)
}

func New(cfg *config.Config) *Daemon {
	return &Daemon{
		config: cfg,
		runner: runner.New(cfg.Runner, cfg.Timeout),
		busy:   make(map[string]bool),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

func (d *Daemon) Run() {
	defer close(d.done)

	ticker := time.NewTicker(d.config.TickInterval)
	defer ticker.Stop()

	log.Printf("daemon started (tick=%s, runner=%q, max_todos=%d)",
		d.config.TickInterval, d.config.Runner, d.config.MaxTodos)

	for {
		select {
		case <-d.stop:
			log.Println("daemon stopping")
			return
		case now := <-ticker.C:
			d.tick(now)
		}
	}
}

func (d *Daemon) Stop() {
	close(d.stop)
	<-d.done
}

func (d *Daemon) tick(now time.Time) {
	thisMinute := now.Truncate(time.Minute)

	// Only evaluate cron schedules once per minute
	cronTick := !thisMinute.Equal(d.lastTick)
	if cronTick {
		d.lastTick = thisMinute
	}

	paths := loadWatchedPaths()
	if len(paths) == 0 {
		if cronTick {
			log.Printf("tick %s — no watched projects", now.Format("15:04:05"))
		}
		return
	}

	// Load all projects
	var projects []*project.Project
	for _, p := range paths {
		proj, err := project.Load(p)
		if err != nil {
			log.Printf("skip %s: %v", p, err)
			continue
		}
		projects = append(projects, proj)
	}

	// Sort projects alphabetically by path (no project priority)
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Path < projects[j].Path
	})

	// On non-cron ticks, log heartbeat and any busy projects
	if !cronTick {
		var busyNames []string
		for _, proj := range projects {
			d.mu.Lock()
			busy := d.busy[proj.Path]
			d.mu.Unlock()
			if busy {
				busyNames = append(busyNames, filepath.Base(proj.Path))
			}
		}
		if len(busyNames) > 0 {
			log.Printf("tick %s — waiting on: %s", now.Format("15:04:05"), strings.Join(busyNames, ", "))
		} else {
			log.Printf("tick %s — idle", now.Format("15:04:05"))
		}
		return
	}

	totalTodos := 0
	totalMatched := 0
	dispatched := 0

	for _, proj := range projects {
		projName := filepath.Base(proj.Path)

		// Skip if busy from a previous tick
		d.mu.Lock()
		busy := d.busy[proj.Path]
		d.mu.Unlock()
		if busy {
			log.Printf("  %s: busy (previous dispatch still running)", projName)
			continue
		}

		// Load all todos
		allTodos, err := proj.LoadTodos()
		if err != nil {
			log.Printf("  %s: error loading todos: %v", projName, err)
			continue
		}
		totalTodos += len(allTodos)

		// Select those whose schedule matches this minute
		var todos []project.Todo
		for _, t := range allTodos {
			if t.Schedule != "" && cron.Matches(t.Schedule, thisMinute) {
				todos = append(todos, t)
			}
		}
		totalMatched += len(todos)

		if len(todos) == 0 {
			continue
		}

		for _, t := range todos {
			log.Printf("  dispatch %s/%s (p%d, schedule=%q)", projName, t.Name, t.Priority, t.Schedule)
		}
		dispatched += len(todos)

		d.mu.Lock()
		d.busy[proj.Path] = true
		d.mu.Unlock()

		go d.processProject(proj, todos)
	}

	log.Printf("tick %s — %d projects, %d todos, %d matched, %d dispatched",
		now.Format("15:04:05"), len(projects), totalTodos, totalMatched, dispatched)
}

func (d *Daemon) processProject(proj *project.Project, todos []project.Todo) {
	defer func() {
		d.mu.Lock()
		d.busy[proj.Path] = false
		d.mu.Unlock()
		log.Printf("finished %s", proj.Path)
	}()

	batchSize := d.config.MaxTodos
	if batchSize < 1 {
		batchSize = 1
	}

	for i := 0; i < len(todos); i += batchSize {
		end := i + batchSize
		if end > len(todos) {
			end = len(todos)
		}
		batch := todos[i:end]

		var wg sync.WaitGroup
		for _, todo := range batch {
			wg.Add(1)
			go func(t project.Todo) {
				defer wg.Done()

				log.Printf("run %s: %s (p%d)", proj.Path, t.Name, t.Priority)

				ctx, cancel := context.WithTimeout(context.Background(), d.config.Timeout)
				defer cancel()

				output, err := d.runner.Run(ctx, proj.Path, t.ID, t.Content)
				if err != nil {
					log.Printf("fail %s: %s: %v", proj.Path, t.Name, err)
				} else {
					log.Printf("done %s: %s", proj.Path, t.Name)
					// Remove the todo file after successful execution
					if removeErr := os.Remove(t.Path); removeErr != nil {
						log.Printf("warn: could not remove %s: %v", t.Path, removeErr)
					}
				}
				if output != "" {
					log.Printf("output %s: %s: %s", proj.Path, t.Name, output)
				}
			}(todo)
		}
		wg.Wait()
	}
}

// loadWatchedPaths scans ~/.anvil/watched/ and returns project paths
func loadWatchedPaths() []string {
	watchedDir := config.WatchedDir()
	dirs, err := os.ReadDir(watchedDir)
	if err != nil {
		return nil
	}

	var paths []string
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}

		dirPath := filepath.Join(watchedDir, d.Name())
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		// Sort descending to get latest file first
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() > entries[j].Name()
		})

		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}

			data, err := os.ReadFile(filepath.Join(dirPath, e.Name()))
			if err != nil {
				break
			}

			p := parseWatchedPath(string(data))
			if p != "" {
				paths = append(paths, p)
			}
			break
		}
	}

	return paths
}

type watchFrontmatter struct {
	Path string `yaml:"path"`
}

func parseWatchedPath(content string) string {
	start := strings.Index(content, "---\n")
	if start == -1 {
		return ""
	}
	end := strings.Index(content[start+4:], "\n---")
	if end == -1 {
		return ""
	}

	var fm watchFrontmatter
	if err := yaml.Unmarshal([]byte(content[start+4:start+4+end]), &fm); err != nil {
		return ""
	}
	return fm.Path
}
