package project

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/cron"
	"gopkg.in/yaml.v3"
)

// Config is the project-level config from <project>/.anvil/anvil.yaml
type Config struct {
	// No project‑level configuration needed; todos carry their own schedule
}

// Project represents a watched project directory
type Project struct {
	Path string
}

// Todo is a single todo file from the project's .anvil/todos/ tree
type Todo struct {
	Path            string // absolute path to the file
	Name            string // filename
	Priority        int    // 0-9, from pN/ directory
	Content         string // file contents (after front‑matter)
	Schedule        string // cron expression from front‑matter
	ID              string // UUID for session tracking
	Resume          *bool    // nil = default (true for recurring, false for one-shot), explicit overrides
	MaxConcurrent   int      // max simultaneous instances (0 = default 1)
	SkipPermissions bool     // if true, append --dangerously-skip-permissions to runner command
	AllowedTools    []string // if non-empty, append --allowedTools <tools> to runner command
	PreCheck        string   // optional shell command; task is skipped silently if it exits non-zero
	OnSuccess       string   // optional shell command to run after successful completion
	OnFailure       string   // optional shell command to run after failed completion
	IsLocked        bool     // true if a stale lock file exists
}

// RunRecord persists metadata for a single task dispatch, written after completion.
// It links a task ID to the Claude session ID and child process PID used for that run.
type RunRecord struct {
	RunID     string    `json:"run_id"`
	TaskID    string    `json:"task_id"`
	SessionID string    `json:"session_id"`
	PID       int       `json:"pid"`
	Started   time.Time `json:"started"`
}

// Load reads a project's .anvil/anvil.yaml and returns a Project
func Load(path string) (*Project, error) {
	// No per‑project configuration file is required; simply return the Project.
	return &Project{Path: path}, nil
}

// LoadTodos returns all todo files sorted by priority (p0 first) then by name (oldest first)
func (p *Project) LoadTodos() ([]Todo, error) {
	todosDir := filepath.Join(p.Path, ".anvil", "todos")
	var todos []Todo

	for pri := 0; pri <= 9; pri++ {
		dir := filepath.Join(todosDir, fmt.Sprintf("p%d", pri))
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading todos p%d: %w", pri, err)
		}

		// Sort by name so oldest‑timestamped files come first
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})

		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			fp := filepath.Join(dir, e.Name())
			raw, err := os.ReadFile(fp)
			if err != nil {
				continue
			}

			// Check for lock file (stale lock indicates daemon crashed mid-execution)
			lockPath := fp + ".lock"
			_, lockErr := os.Stat(lockPath)
			hasLock := lockErr == nil

			// Parse optional front‑matter for a schedule.
			// Expected format:
			// ---
			// schedule: "*/15 * * * *"
			// ---
			// <content>
			contentStr := string(raw)
			schedule := ""
			id := ""
			var resume *bool
			maxConcurrent := 0
			skipPermissions := false
			var allowedTools []string
			preCheck := ""
			onSuccess := ""
			onFailure := ""
			body := contentStr

			if strings.HasPrefix(contentStr, "---\n") {
				// Find closing front‑matter delimiter.
				parts := strings.SplitN(contentStr[4:], "\n---\n", 2)
				if len(parts) == 2 {
					fm := parts[0]
					body = parts[1]
					var fmData struct {
						Schedule        string   `yaml:"schedule"`
						ID              string   `yaml:"id"`
						Resume          *bool    `yaml:"resume"`
						MaxConcurrent   int      `yaml:"max_concurrent"`
						SkipPermissions bool     `yaml:"skip_permissions"`
						AllowedTools    []string `yaml:"allowed_tools"`
						PreCheck        string   `yaml:"pre_check"`
						OnSuccess       string   `yaml:"on_success"`
						OnFailure       string   `yaml:"on_failure"`
					}
					if err := yaml.Unmarshal([]byte(fm), &fmData); err == nil {
						schedule = fmData.Schedule
						id = fmData.ID
						resume = fmData.Resume
						maxConcurrent = fmData.MaxConcurrent
						skipPermissions = fmData.SkipPermissions
						allowedTools = fmData.AllowedTools
						preCheck = fmData.PreCheck
						onSuccess = fmData.OnSuccess
						onFailure = fmData.OnFailure
					}
				}
			}

			todos = append(todos, Todo{
				Path:            fp,
				Name:            e.Name(),
				Priority:        pri,
				Content:         body,
				Schedule:        schedule,
				ID:              id,
				Resume:          resume,
				MaxConcurrent:   maxConcurrent,
				SkipPermissions: skipPermissions,
				AllowedTools:    allowedTools,
				PreCheck:        preCheck,
				OnSuccess:       onSuccess,
				OnFailure:       onFailure,
				IsLocked:        hasLock,
			})
		}
	}

	return todos, nil
}

// RemoveTodo deletes a todo file from disk
func RemoveTodo(todo Todo) error {
	return os.Remove(todo.Path)
}

// RemoveLock removes the lock file for a todo, if it exists
func RemoveLock(todo Todo) error {
	lockPath := todo.Path + ".lock"
	_, err := os.Stat(lockPath)
	if os.IsNotExist(err) {
		// No lock file to remove
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking lock file %s: %w", lockPath, err)
	}
	return os.Remove(lockPath)
}

// Init creates the .anvil/ directory structure and writes embedded tools into .claude/.
// The toolsFS should contain a "skills" directory at its root.
// Priority subdirectories are created on-demand when tasks are added.
func Init(path string, toolsFS fs.FS) error {
	todosDir := filepath.Join(path, ".anvil", "todos")
	if err := os.MkdirAll(todosDir, 0755); err != nil {
		return fmt.Errorf("creating .anvil/todos: %w", err)
	}

	claudeDir := filepath.Join(path, ".claude")
	if err := writeEmbeddedFS(claudeDir, toolsFS); err != nil {
		return fmt.Errorf("writing .claude/ tools: %w", err)
	}

	return nil
}

// writeEmbeddedFS walks an fs.FS and writes all files into destDir,
// preserving the directory structure.
func writeEmbeddedFS(destDir string, fsys fs.FS) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, path)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("reading embedded %s: %w", path, err)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		return os.WriteFile(target, data, 0644)
	})
}

// AddTodo writes a new todo file into the project's .anvil/todos/pN/ directory.
// It returns the relative path like "p1/check-github-for-issues.md".
func (p *Project) AddTodo(priority int, schedule string, content string) (string, error) {
	if priority < 0 || priority > 9 {
		return "", fmt.Errorf("priority must be 0-9, got %d", priority)
	}
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("task content must not be empty")
	}

	// Validate cron expression before writing the task file.
	if schedule != "" {
		if _, err := cron.Parse(schedule); err != nil {
			return "", fmt.Errorf("invalid schedule %q: %w", schedule, err)
		}
	}

	dir := filepath.Join(p.Path, ".anvil", "todos", fmt.Sprintf("p%d", priority))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating todos/p%d: %w", priority, err)
	}

	base := slugify(content)
	filename := base + ".md"
	// Avoid silent overwrites on slug collision: append -2, -3, … if file exists.
	fullCheck := filepath.Join(dir, filename)
	if _, err := os.Stat(fullCheck); err == nil {
		for i := 2; ; i++ {
			candidate := fmt.Sprintf("%s-%d.md", base, i)
			if _, err := os.Stat(filepath.Join(dir, candidate)); os.IsNotExist(err) {
				filename = candidate
				break
			}
		}
	}
	id := newUUID()

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("id: %q\n", id))
	sb.WriteString(fmt.Sprintf("schedule: %q\n", schedule))
	// One-shot tasks: empty schedule means run once and delete after completion
	// Set resume: false explicitly for one-shot tasks
	if schedule == "" {
		sb.WriteString("resume: false\n")
	}
	sb.WriteString("---\n")
	sb.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		sb.WriteString("\n")
	}

	fullPath := filepath.Join(dir, filename)
	if err := os.WriteFile(fullPath, []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("writing todo file: %w", err)
	}

	return fmt.Sprintf("p%d/%s", priority, filename), nil
}

// newUUID generates a random UUID v4.
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 1
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// SessionPath returns the path to the claude session JSONL for a todo.
// Claude stores sessions at ~/.claude/projects/<slug>/<session-id>.jsonl
// where slug is the project path with / and _ replaced by -.
// Deprecated: use SessionPathBySessionID for per-run session lookup.
func SessionPath(projectPath string, todoID string) string {
	return SessionPathBySessionID(projectPath, todoID)
}

// SessionPathBySessionID returns the path to the claude session JSONL for a given session ID.
func SessionPathBySessionID(projectPath string, sessionID string) string {
	home, _ := os.UserHomeDir()
	slug := strings.ReplaceAll(projectPath, "/", "-")
	slug = strings.ReplaceAll(slug, "_", "-")
	return filepath.Join(home, ".claude", "projects", slug, sessionID+".jsonl")
}

// runsDir returns the path to the runs directory for a task.
func runsDir(projectPath, taskID string) string {
	return filepath.Join(projectPath, ".anvil", "runs", taskID)
}

// RunPath returns the path to a specific run record JSON file.
func RunPath(projectPath, taskID, runID string) string {
	return filepath.Join(runsDir(projectPath, taskID), runID+".json")
}

// CurrentRunPath returns the path to the "current" run ID file for a task.
func CurrentRunPath(projectPath, taskID string) string {
	return filepath.Join(runsDir(projectPath, taskID), "current")
}

// WriteRunRecord writes a run record and updates the "current" pointer.
func WriteRunRecord(projectPath string, rec RunRecord) error {
	dir := runsDir(projectPath, rec.TaskID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating runs dir: %w", err)
	}

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshaling run record: %w", err)
	}

	recPath := filepath.Join(dir, rec.RunID+".json")
	if err := os.WriteFile(recPath, data, 0644); err != nil {
		return fmt.Errorf("writing run record: %w", err)
	}

	// Update "current" pointer to the latest runID
	currentPath := filepath.Join(dir, "current")
	return os.WriteFile(currentPath, []byte(rec.RunID), 0644)
}

// ReadCurrentRunRecord reads the most recent run record for a task.
func ReadCurrentRunRecord(projectPath, taskID string) (RunRecord, error) {
	currentPath := CurrentRunPath(projectPath, taskID)
	runIDBytes, err := os.ReadFile(currentPath)
	if err != nil {
		return RunRecord{}, fmt.Errorf("reading current run pointer: %w", err)
	}

	runID := strings.TrimSpace(string(runIDBytes))
	recPath := RunPath(projectPath, taskID, runID)
	data, err := os.ReadFile(recPath)
	if err != nil {
		return RunRecord{}, fmt.Errorf("reading run record: %w", err)
	}

	var rec RunRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return RunRecord{}, fmt.Errorf("parsing run record: %w", err)
	}
	return rec, nil
}

// LatestSessionID resolves the session ID for the most recent run of a task.
// Returns an error if no run record exists (caller should start a fresh session).
func LatestSessionID(projectPath, taskID string) (string, error) {
	rec, err := ReadCurrentRunRecord(projectPath, taskID)
	if err == nil {
		return rec.SessionID, nil
	}
	return "", fmt.Errorf("no run record found for task %s", taskID)
}

var (
	reNonAlphaNum    = regexp.MustCompile(`[^a-z0-9]+`)
	reMultipleHyphen = regexp.MustCompile(`-{2,}`)
)

// slugify converts a string to a URL-safe slug suitable for use as a filename.
func slugify(s string) string {
	s = strings.ToLower(s)
	s = reNonAlphaNum.ReplaceAllString(s, "-")
	s = reMultipleHyphen.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 50 {
		s = s[:50]
		s = strings.TrimRight(s, "-")
	}
	if s == "" {
		s = "task"
	}
	return s
}
