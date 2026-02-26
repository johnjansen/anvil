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

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/cron"
	"gopkg.in/yaml.v3"
)

// Config holds project-level configuration defaults loaded from .anvil/config.yaml.
// These defaults apply to all tasks unless explicitly overridden in individual task frontmatter.
type Config struct {
	Defaults TaskDefaults `yaml:"defaults"`
}

// TaskDefaults contains fields that can be set at the project level and inherited by all tasks.
// Task frontmatter overrides project defaults (task-level wins).
type TaskDefaults struct {
	SkipPermissions      bool     `yaml:"skip_permissions"`
	AllowedTools         []string `yaml:"allowed_tools"`
	PreCheck             string   `yaml:"pre_check"`
	OnSuccess            string   `yaml:"on_success"`
	OnFailure            string   `yaml:"on_failure"`
	Timeout              string   `yaml:"timeout"`
	Retry                int      `yaml:"retry"`
	RetryDelay           string   `yaml:"retry_delay"`
	MaxConcurrent        int      `yaml:"max_concurrent"`
	PersistentCooldown   string   `yaml:"persistent_cooldown"`
	PersistentMaxRuntime string   `yaml:"persistent_max_runtime"`
	PersistentBudget     string   `yaml:"persistent_budget"`
	MaxLogSize           string   `yaml:"max_log_size"`
	Runner               string   `yaml:"runner"`
}

// ConfigPath returns the path to the project config file.
func ConfigPath(projectPath string) string {
	return filepath.Join(projectPath, ".anvil", "config.yaml")
}

// LoadConfig reads the project-level config from .anvil/config.yaml.
// Returns a zero-value Config if the file does not exist.
func LoadConfig(projectPath string) (Config, error) {
	data, err := os.ReadFile(ConfigPath(projectPath))
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("reading project config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing project config: %w", err)
	}
	return cfg, nil
}

// Project represents a watched project directory
type Project struct {
	Path   string
	Config Config
}

// Todo is a single todo file from the project's .anvil/todos/ tree
type Todo struct {
	Path            string        // absolute path to the file
	Name            string        // filename
	Priority        int           // 0-9, from pN/ directory
	Content         string        // file contents (after front-matter)
	Schedule        string        // cron expression from front-matter
	ID              string        // UUID for session tracking
	Resume          *bool         // nil = default (true for recurring, false for one-shot), explicit overrides
	MaxConcurrent   int           // max simultaneous instances (0 = default 1)
	SkipPermissions bool          // if true, append --dangerously-skip-permissions to runner command
	AllowedTools    []string      // if non-empty, append --allowedTools <tools> to runner command
	PreCheck        string        // optional shell command; task is skipped silently if it exits non-zero
	OnSuccess       string        // optional shell command to run after successful completion
	OnFailure       string        // optional shell command to run after failed completion
	IsLocked        bool          // true if a stale lock file exists
	Disabled        bool          // if true, task is paused and skipped during tick evaluation
	Timeout         time.Duration // task-specific timeout (0 = use global default)
	Retry           int           // number of retries on failure (0 = no retry)
	RetryDelay      time.Duration // delay between retries (default 1m, used with Retry)
	// Persistent task configuration
	PersistentCooldown   time.Duration // cooldown between restart cycles (default 0 = immediate)
	PersistentMaxRuntime time.Duration // max runtime before forced restart (0 = no limit)
	PersistentBudget     time.Duration // cumulative wall-clock budget per daemon lifetime (0 = unlimited)
	MaxLogSize           int64         // max log file size in bytes (0 = use global default)
	Runner               string        // per-task runner command override (empty = use global runner chain)
	Webhook              string        // per-task webhook URL override (empty = use global webhooks only)
	Labels               []string      // user-defined labels for organizing and filtering tasks
	Env                  map[string]string // environment variables injected into task execution
	DependsOn            []string      // list of task names this task depends on (all must succeed before running)
	Checkpoint           bool          // if true, capture ##anvil:checkpoint output and inject on resume
	NotifyOnFailure      *bool         // per-task override: notify on failure (nil = use global config)
	NotifyOnSuccess      *bool         // per-task override: notify on success (nil = use global config)
}

// RunRecord persists metadata for a single task dispatch, written after completion.
// It links a task ID to the Claude session ID and child process PID used for that run.
type RunRecord struct {
	RunID            string    `json:"run_id"`
	TaskID           string    `json:"task_id"`
	SessionID        string    `json:"session_id"`
	PID              int       `json:"pid"`
	Started          time.Time `json:"started"`
	Finished         time.Time `json:"finished,omitempty"`        // when the run ended
	Success          bool      `json:"success"`                   // whether the runner returned nil error
	OutputSummary    string    `json:"output_summary,omitempty"`  // first and last N lines of output
	Error            string    `json:"error,omitempty"`           // last runner error message if failed
	InputTokens      int       `json:"input_tokens,omitempty"`    // tokens sent to the model
	OutputTokens     int       `json:"output_tokens,omitempty"`   // tokens received from the model
	EstimatedCostUSD float64   `json:"estimated_cost_usd,omitempty"` // estimated cost in USD
	CheckpointData   string    `json:"checkpoint_data,omitempty"`   // last checkpoint data emitted by the task
	Attempt          int       `json:"attempt,omitempty"`           // final attempt number (1-based), 0 if no retries configured
	MaxRetries       int       `json:"max_retries,omitempty"`       // configured max retries for this task
	RetryDelay       string    `json:"retry_delay,omitempty"`       // configured base delay between retries
}

// Load reads a project's .anvil/config.yaml and returns a Project.
// If the config file does not exist, the project loads with zero-value defaults.
func Load(path string) (*Project, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	return &Project{Path: path, Config: cfg}, nil
}

// LoadTodos returns all todo files sorted by priority (p0 first) then by name (oldest first).
// Project-level defaults from .anvil/config.yaml are applied to any task field not explicitly
// set in the task's frontmatter.
func (p *Project) LoadTodos() ([]Todo, error) {
	todosDir := filepath.Join(p.Path, ".anvil", "todos")
	defaults := p.Config.Defaults
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

		// Sort by name so oldest-timestamped files come first
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

			// Parse optional front-matter for a schedule.
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
			disabled := false
			var timeout time.Duration
			retry := 0
			var retryDelay time.Duration
			var persistentCooldown time.Duration
			var persistentMaxRuntime time.Duration
			var persistentBudget time.Duration
			var maxLogSize int64
			runnerOverride := ""
			webhookURL := ""
			var labels []string
			var envVars map[string]string
			var dependsOn []string
			checkpoint := false
			var notifyOnFailure *bool
			var notifyOnSuccess *bool
			body := contentStr

			// Track which frontmatter keys were explicitly set so project defaults
			// only apply to fields omitted from the task frontmatter.
			var fmKeys map[string]interface{}

			if strings.HasPrefix(contentStr, "---\n") {
				// Find closing front-matter delimiter.
				parts := strings.SplitN(contentStr[4:], "\n---\n", 2)
				if len(parts) == 2 {
					fm := parts[0]
					body = parts[1]
					var fmData struct {
						Schedule             string   `yaml:"schedule"`
						ID                   string   `yaml:"id"`
						Resume               *bool    `yaml:"resume"`
						MaxConcurrent        int      `yaml:"max_concurrent"`
						SkipPermissions      bool     `yaml:"skip_permissions"`
						AllowedTools         []string `yaml:"allowed_tools"`
						PreCheck             string   `yaml:"pre_check"`
						OnSuccess            string   `yaml:"on_success"`
						OnFailure            string   `yaml:"on_failure"`
						Disabled             bool     `yaml:"disabled"`
						Timeout              string   `yaml:"timeout"`
						Retry                int      `yaml:"retry"`
						RetryDelay           string   `yaml:"retry_delay"`
						PersistentCooldown   string   `yaml:"persistent_cooldown"`
						PersistentMaxRuntime string   `yaml:"persistent_max_runtime"`
						PersistentBudget     string   `yaml:"persistent_budget"`
						MaxLogSize           string   `yaml:"max_log_size"`
						Runner               string   `yaml:"runner"`
						Webhook              string   `yaml:"webhook"`
						Labels               []string          `yaml:"labels"`
						Env                  map[string]string `yaml:"env"`
						DependsOn            []string          `yaml:"depends_on"`
						Checkpoint           bool              `yaml:"checkpoint"`
						NotifyOnFailure      *bool             `yaml:"notify_on_failure"`
						NotifyOnSuccess      *bool             `yaml:"notify_on_success"`
					}
					if err := yaml.Unmarshal([]byte(fm), &fmData); err == nil {
						// Parse raw keys to detect which fields were explicitly set.
						_ = yaml.Unmarshal([]byte(fm), &fmKeys)

						schedule = fmData.Schedule
						id = fmData.ID
						resume = fmData.Resume
						maxConcurrent = fmData.MaxConcurrent
						skipPermissions = fmData.SkipPermissions
						allowedTools = fmData.AllowedTools
						preCheck = fmData.PreCheck
						onSuccess = fmData.OnSuccess
						onFailure = fmData.OnFailure
						disabled = fmData.Disabled
						if fmData.Timeout != "" {
							timeout, _ = time.ParseDuration(fmData.Timeout)
						}
						retry = fmData.Retry
						if fmData.RetryDelay != "" {
							retryDelay, _ = time.ParseDuration(fmData.RetryDelay)
						}
						if fmData.PersistentCooldown != "" {
							persistentCooldown, _ = time.ParseDuration(fmData.PersistentCooldown)
						}
						if fmData.PersistentMaxRuntime != "" {
							persistentMaxRuntime, _ = time.ParseDuration(fmData.PersistentMaxRuntime)
						}
						if fmData.PersistentBudget != "" {
							persistentBudget, _ = time.ParseDuration(fmData.PersistentBudget)
						}
						if fmData.MaxLogSize != "" {
							maxLogSize, _ = config.ParseByteSize(fmData.MaxLogSize)
						}
						runnerOverride = fmData.Runner
						webhookURL = fmData.Webhook
						labels = fmData.Labels
						envVars = fmData.Env
						dependsOn = fmData.DependsOn
						checkpoint = fmData.Checkpoint
						notifyOnFailure = fmData.NotifyOnFailure
						notifyOnSuccess = fmData.NotifyOnSuccess
					}
				}
			}

			// Apply project defaults for fields not explicitly set in frontmatter.
			applyDefaults(defaults, fmKeys, &skipPermissions, &allowedTools, &preCheck,
				&onSuccess, &onFailure, &timeout, &retry, &retryDelay,
				&maxConcurrent, &persistentCooldown, &persistentMaxRuntime, &persistentBudget, &maxLogSize, &runnerOverride)

			// Resolve env: prefixed values from the current environment
			resolvedEnv := resolveEnvVars(envVars)

			todos = append(todos, Todo{
				Path:                 fp,
				Name:                 e.Name(),
				Priority:             pri,
				Content:              body,
				Schedule:             schedule,
				ID:                   id,
				Resume:               resume,
				MaxConcurrent:        maxConcurrent,
				SkipPermissions:      skipPermissions,
				AllowedTools:         allowedTools,
				PreCheck:             preCheck,
				OnSuccess:            onSuccess,
				OnFailure:            onFailure,
				IsLocked:             hasLock,
				Disabled:             disabled,
				Timeout:              timeout,
				Retry:                retry,
				RetryDelay:           retryDelay,
				PersistentCooldown:   persistentCooldown,
				PersistentMaxRuntime: persistentMaxRuntime,
				PersistentBudget:     persistentBudget,
				MaxLogSize:           maxLogSize,
				Runner:               runnerOverride,
				Webhook:              webhookURL,
				Labels:               labels,
				Env:                  resolvedEnv,
				DependsOn:            dependsOn,
				Checkpoint:           checkpoint,
				NotifyOnFailure:      notifyOnFailure,
				NotifyOnSuccess:      notifyOnSuccess,
			})
		}
	}

	return todos, nil
}

// applyDefaults fills in project-level defaults for any task field not explicitly set
// in the task's frontmatter. fmKeys is the set of keys present in the parsed YAML;
// a nil map means no frontmatter was present (all defaults apply).
func applyDefaults(defaults TaskDefaults, fmKeys map[string]interface{},
	skipPermissions *bool, allowedTools *[]string, preCheck *string,
	onSuccess *string, onFailure *string, timeout *time.Duration,
	retry *int, retryDelay *time.Duration, maxConcurrent *int,
	persistentCooldown *time.Duration, persistentMaxRuntime *time.Duration,
	persistentBudget *time.Duration, maxLogSize *int64, runnerOverride *string) {

	has := func(key string) bool {
		if fmKeys == nil {
			return false
		}
		_, ok := fmKeys[key]
		return ok
	}

	if !has("skip_permissions") && defaults.SkipPermissions {
		*skipPermissions = defaults.SkipPermissions
	}
	if !has("allowed_tools") && len(defaults.AllowedTools) > 0 {
		*allowedTools = defaults.AllowedTools
	}
	if !has("pre_check") && defaults.PreCheck != "" {
		*preCheck = defaults.PreCheck
	}
	if !has("on_success") && defaults.OnSuccess != "" {
		*onSuccess = defaults.OnSuccess
	}
	if !has("on_failure") && defaults.OnFailure != "" {
		*onFailure = defaults.OnFailure
	}
	if !has("timeout") && defaults.Timeout != "" {
		if d, err := time.ParseDuration(defaults.Timeout); err == nil {
			*timeout = d
		}
	}
	if !has("retry") && defaults.Retry != 0 {
		*retry = defaults.Retry
	}
	if !has("retry_delay") && defaults.RetryDelay != "" {
		if d, err := time.ParseDuration(defaults.RetryDelay); err == nil {
			*retryDelay = d
		}
	}
	if !has("max_concurrent") && defaults.MaxConcurrent != 0 {
		*maxConcurrent = defaults.MaxConcurrent
	}
	if !has("persistent_cooldown") && defaults.PersistentCooldown != "" {
		if d, err := time.ParseDuration(defaults.PersistentCooldown); err == nil {
			*persistentCooldown = d
		}
	}
	if !has("persistent_max_runtime") && defaults.PersistentMaxRuntime != "" {
		if d, err := time.ParseDuration(defaults.PersistentMaxRuntime); err == nil {
			*persistentMaxRuntime = d
		}
	}
	if !has("persistent_budget") && defaults.PersistentBudget != "" {
		if d, err := time.ParseDuration(defaults.PersistentBudget); err == nil {
			*persistentBudget = d
		}
	}
	if !has("max_log_size") && defaults.MaxLogSize != "" {
		if size, err := config.ParseByteSize(defaults.MaxLogSize); err == nil {
			*maxLogSize = size
		}
	}
	if !has("runner") && defaults.Runner != "" {
		*runnerOverride = defaults.Runner
	}
}

// IsPersistent returns true if this task is configured to run persistently.
// Persistent tasks exit after each unit of work and are immediately re-dispatched.
func (t Todo) IsPersistent() bool {
	return t.Schedule == "persistent"
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
func (p *Project) AddTodo(priority int, schedule string, content string, preCheck string, allowedTools string, maxConcurrent int, skipPermissions bool, runnerCmd string) (string, error) {
	if priority < 0 || priority > 9 {
		return "", fmt.Errorf("priority must be 0-9, got %d", priority)
	}
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("task content must not be empty")
	}

	// Validate cron expression before writing the task file.
	// Skip validation for "persistent" since it's a special keyword, not a cron expression.
	if schedule != "" && schedule != "persistent" {
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
	// Avoid silent overwrites on slug collision: append -2, -3, ... if file exists.
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
	if preCheck != "" {
		sb.WriteString(fmt.Sprintf("pre_check: %q\n", preCheck))
	}
	if allowedTools != "" {
		sb.WriteString(fmt.Sprintf("allowed_tools: %q\n", allowedTools))
	}
	if maxConcurrent != 0 {
		sb.WriteString(fmt.Sprintf("max_concurrent: %d\n", maxConcurrent))
	}
	if skipPermissions {
		sb.WriteString("skip_permissions: true\n")
	}
	if runnerCmd != "" {
		sb.WriteString(fmt.Sprintf("runner: %q\n", runnerCmd))
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

// ReadAllRunRecords reads all run records for a task, sorted by start time (newest first).
func ReadAllRunRecords(projectPath, taskID string) ([]RunRecord, error) {
	dir := runsDir(projectPath, taskID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading runs dir: %w", err)
	}

	var records []RunRecord
	for _, e := range entries {
		if e.IsDir() || e.Name() == "current" || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		recPath := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(recPath)
		if err != nil {
			continue // skip unreadable records
		}
		var rec RunRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			continue // skip malformed records
		}
		records = append(records, rec)
	}

	// Sort by start time, newest first
	sort.Slice(records, func(i, j int) bool {
		return records[i].Started.After(records[j].Started)
	})

	return records, nil
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

// LatestCheckpointData returns the most recent checkpoint data for a task.
// Returns empty string if no checkpoint data exists.
func LatestCheckpointData(projectPath, taskID string) string {
	rec, err := ReadCurrentRunRecord(projectPath, taskID)
	if err != nil {
		return ""
	}
	return rec.CheckpointData
}

var (
	reNonAlphaNum    = regexp.MustCompile(`[^a-z0-9]+`)
	reMultipleHyphen = regexp.MustCompile(`-{2,}`)
)

// resolveEnvVars processes a map of environment variable definitions.
// Values prefixed with "env:" are resolved from the current environment.
// Empty string values inherit from the parent environment (included as-is for merging).
// All other values are used as literal strings.
func resolveEnvVars(vars map[string]string) map[string]string {
	if len(vars) == 0 {
		return nil
	}
	resolved := make(map[string]string, len(vars))
	for k, v := range vars {
		if strings.HasPrefix(v, "env:") {
			envKey := v[4:]
			resolved[k] = os.Getenv(envKey)
		} else {
			resolved[k] = v
		}
	}
	return resolved
}

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

// TemplateSpec represents a task template with all configurable fields.
// These fields map to the same frontmatter fields available in task files.
type TemplateSpec struct {
	Schedule              string            `yaml:"schedule,omitempty"`
	Priority              int               `yaml:"priority,omitempty"`
	AllowedTools          []string          `yaml:"allowed_tools,omitempty"`
	PreCheck              string            `yaml:"pre_check,omitempty"`
	OnSuccess             string            `yaml:"on_success,omitempty"`
	OnFailure             string            `yaml:"on_failure,omitempty"`
	SkipPermissions       bool              `yaml:"skip_permissions,omitempty"`
	Timeout               string            `yaml:"timeout,omitempty"`
	Retry                int               `yaml:"retry,omitempty"`
	RetryDelay           string            `yaml:"retry_delay,omitempty"`
	MaxConcurrent        int               `yaml:"max_concurrent,omitempty"`
	PersistentCooldown    string            `yaml:"persistent_cooldown,omitempty"`
	PersistentMaxRuntime string            `yaml:"persistent_max_runtime,omitempty"`
	PersistentBudget     string            `yaml:"persistent_budget,omitempty"`
	MaxLogSize           int64             `yaml:"max_log_size,omitempty"`
	Runner               string            `yaml:"runner,omitempty"`
	Webhook              string            `yaml:"webhook,omitempty"`
	Labels               []string          `yaml:"labels,omitempty"`
	Env                  map[string]string `yaml:"env,omitempty"`
	DependsOn            []string          `yaml:"depends_on,omitempty"`
	Checkpoint           bool              `yaml:"checkpoint,omitempty"`
	NotifyOnFailure      *bool             `yaml:"notify_on_failure,omitempty"`
	NotifyOnSuccess      *bool             `yaml:"notify_on_success,omitempty"`
}

// Template represents a loaded template with its name and spec.
type Template struct {
	Name string
	Spec TemplateSpec
}

// templatePaths returns the search paths for templates (project then global).
func templatePaths(projectPath string) []string {
	homeDir, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(projectPath, ".anvil", "templates"),
		filepath.Join(homeDir, ".anvil", "templates"),
	}
	return paths
}

// LoadTemplate loads a template by name from project or global templates.
// Returns the template if found, or an error if not found.
func LoadTemplate(projectPath, name string) (*Template, error) {
	// Prevent directory traversal
	name = filepath.Clean(name)
	if strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
		return nil, fmt.Errorf("invalid template name: %s", name)
	}

	for _, basePath := range templatePaths(projectPath) {
		templatePath := filepath.Join(basePath, name)
		// Try with .yaml extension first
		if _, err := os.Stat(templatePath + ".yaml"); err == nil {
			templatePath += ".yaml"
		}

		data, err := os.ReadFile(templatePath)
		if err == nil {
			var spec TemplateSpec
			if err := yaml.Unmarshal(data, &spec); err != nil {
				return nil, fmt.Errorf("failed to parse template %s: %w", name, err)
			}
			return &Template{Name: name, Spec: spec}, nil
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read template %s: %w", name, err)
		}
	}
	return nil, fmt.Errorf("template not found: %s", name)
}

// ListTemplates returns all available templates from project and global locations.
func ListTemplates(projectPath string) ([]Template, error) {
	var templates []Template
	seen := make(map[string]bool)

	for _, basePath := range templatePaths(projectPath) {
		entries, err := os.ReadDir(basePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to read templates directory: %w", err)
		}

		for _, entry := range entries {
			if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml")) {
				name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
				if !seen[name] {
					seen[name] = true
					templates = append(templates, Template{Name: name})
				}
			}
		}
	}
	return templates, nil
}
