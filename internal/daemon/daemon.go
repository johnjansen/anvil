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

	"github.com/johnjansen/anvil-go/internal/config"
	"github.com/johnjansen/anvil-go/internal/cron"
	"github.com/johnjansen/anvil-go/internal/project"
	"github.com/johnjansen/anvil-go/internal/runner"

	"gopkg.in/yaml.v3"
)

type Daemon struct {
	config *config.Config
	runner *runner.Runner
	busy   map[string]bool
	mu     sync.Mutex
	stop   chan struct{}
	done   chan struct{}
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
	paths := loadWatchedPaths()
	if len(paths) == 0 {
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

	// Sort by priority (lower number = first)
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Config.Priority < projects[j].Config.Priority
	})

	for _, proj := range projects {
		// Skip if busy from a previous tick
		d.mu.Lock()
		busy := d.busy[proj.Path]
		d.mu.Unlock()
		if busy {
			log.Printf("skip %s: still busy", proj.Path)
			continue
		}

		// Check cron
		if proj.Config.Schedule == "" || !cron.Matches(proj.Config.Schedule, now) {
			continue
		}

		// Load todos
		todos, err := proj.LoadTodos()
		if err != nil {
			log.Printf("skip %s: %v", proj.Path, err)
			continue
		}
		if len(todos) == 0 {
			continue
		}

		log.Printf("processing %s: %d todos", proj.Path, len(todos))

		d.mu.Lock()
		d.busy[proj.Path] = true
		d.mu.Unlock()

		go d.processProject(proj, todos)
	}
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

				output, err := d.runner.Run(ctx, t.Content)
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
