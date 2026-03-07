package project

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
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
	Precondition         *PreconditionConfig `yaml:"precondition"`
	OnSuccess            string   `yaml:"on_success"`
	OnFailure            string   `yaml:"on_failure"`
	Timeout              string   `yaml:"timeout"`
	Retry                int      `yaml:"retry"`
	RetryDelay           string   `yaml:"retry_delay"`
	RetryStrategy        string   `yaml:"retry_strategy"`
	RetryJitter          float64  `yaml:"retry_jitter"`
	RetryMaxTime         string   `yaml:"retry_max_time"`
	MaxConcurrent        int      `yaml:"max_concurrent"`
	RateLimit            RateLimitConfig `yaml:"rate_limit"` // rate limiting configuration
	PersistentCooldown   string   `yaml:"persistent_cooldown"`
	PersistentMaxRuntime string   `yaml:"persistent_max_runtime"`
	PersistentBudget     string   `yaml:"persistent_budget"`
	CostBudget          string   `yaml:"cost_budget"`
	MaxLogSize           string   `yaml:"max_log_size"`
	Runner               string   `yaml:"runner"`
	PriorityAging        *PriorityAgingDefaults `yaml:"priority_aging"`
}

// RateLimitConfig defines per-task rate limiting configuration.
// When MaxPerHour or MaxPerDay > 0, the daemon tracks execution count and skips tasks that would exceed limits.
type RateLimitConfig struct {
	MaxPerHour int `yaml:"max_per_hour"` // maximum executions per hour (0 = unlimited)
	MaxPerDay  int `yaml:"max_per_day"`  // maximum executions per day (0 = unlimited)
}

// PriorityAgingDefaults contains default priority aging configuration for the project.
type PriorityAgingDefaults struct {
	Enabled   *bool  `yaml:"enabled"`
	MaxWait   string `yaml:"max_wait"`
	MaxBoost  *int   `yaml:"max_boost"`
}

// PriorityAgingConfig contains priority aging configuration for a task.
type PriorityAgingConfig struct {
	Enabled   *bool         `yaml:"enabled"`
	MaxWait   time.Duration `yaml:"max_wait"`
	MaxBoost  int           `yaml:"max_boost"`
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

// AllowedWindow defines a time window during which a task is allowed to execute.
// If set, the task will only run when the current time falls within the window.
type AllowedWindow struct {
	Start string `yaml:"start"` // HH:MM format (24h), e.g. "09:00"
	End   string `yaml:"end"`   // HH:MM format (24h), e.g. "18:00"
	Days  string `yaml:"days"`  // allowed days: range "1-5", list "1,3,5", or combined "1-5,0" (0=Sunday)
}

// SLAConfig defines per-task SLA tracking configuration.
// When MaxDelay > 0, the daemon checks dispatch delay against the threshold.
type SLAConfig struct {
	MaxDelay time.Duration // maximum allowed delay before SLA violation
	Strict   bool          // if true, skip task instead of running late
}

// CircuitBreakerConfig defines per-task circuit breaker configuration.
// When Failures > 0, the daemon tracks consecutive failures and opens circuit after threshold.
type CircuitBreakerConfig struct {
	Failures    int           `yaml:"failures"`     // number of consecutive failures before opening circuit
	Timeout     time.Duration `yaml:"timeout"`      // duration to wait before attempting recovery
	HalfOpenMax int           `yaml:"half_open_max"` // maximum test requests in half-open state
}


// AlertCondition defines when an alert should trigger.
type AlertCondition struct {
	Type      string `yaml:"type"`       // "cost", "duration", or "output"
	Threshold string `yaml:"threshold"`   // threshold value
	Pattern   string `yaml:"pattern"`   // regex pattern for output type
}

// AlertAction defines what happens when an alert fires.
type AlertAction struct {
	Webhook string   `yaml:"webhook"` // URL to POST alert payload
	Notify  []string `yaml:"notify"`  // list of recipients to notify
	Retry   int      `yaml:"retry"`   // webhook retry count
}

// AlertRule defines a complete alert with condition and action.
type AlertRule struct {
	Name      string        `yaml:"name"`       // unique identifier
	Condition AlertCondition `yaml:"condition"` // when to trigger
	Message   string        `yaml:"message"`   // human-readable message
	Severity  string        `yaml:"severity"`  // "warning", "error", or "critical"
	Action    AlertAction   `yaml:"action"`    // what to do when triggered
}

// AlertConfig defines per-task alert configuration.
type AlertConfig struct {
	Rules []AlertRule `yaml:"rules"` // list of alert rules for this task
}

// TaskStateConfig defines per-task state management configuration.
type TaskStateConfig struct {
	Bucket string `yaml:"bucket"` // state bucket name
	Key    string `yaml:"key"`    // state key (supports {{ .TaskID }} template)
}

// SubscriptionConfig defines per-task message subscription configuration.
type SubscriptionConfig struct {
	Type     string `yaml:"type"`     // subscription type: "amqp", "file", "fs", "webhook", "git"
	URL      string `yaml:"url"`      // connection URL (e.g. AMQP broker URL)
	Queue    string `yaml:"queue"`    // queue name (for amqp type)
	Path     string `yaml:"path"`     // file path (for file type)
	FsPath      string   `yaml:"fs_path"`      // filesystem path (for fs type)
	FsEvents    []string `yaml:"fs_events"`    // event types to watch: create, modify, delete, rename (for fs type)
	FsDebounce  string   `yaml:"fs_debounce"`  // debounce duration (e.g. "5s", "500ms") for fs type
	FsGlob      string   `yaml:"fs_glob"`      // glob pattern to filter file events (e.g. "*.json") for fs type
	FsRecursive bool     `yaml:"fs_recursive"` // watch subdirectories recursively (for fs type)
	Prefetch    int      `yaml:"prefetch"`     // prefetch count (for amqp type)
	// Webhook subscription fields
	WebhookPath   string `yaml:"webhook_path"`   // HTTP path for webhook endpoint (for webhook type)
	WebhookMethod string `yaml:"webhook_method"` // HTTP method for webhook endpoint (default: POST)
	WebhookSecret string `yaml:"webhook_secret"` // secret for webhook authentication (can be env var reference)
	// Simplified webhook configuration (alternative to type: webhook)
	Webhook string `yaml:"webhook"` // simplified webhook URL for direct webhook triggering
	// Git subscription fields
	GitEvents       []string `yaml:"git_events"`        // event types to watch: push (for git type)
	GitBranch       string   `yaml:"git_branch"`        // branch name to filter (empty = all branches)
	GitPath         string   `yaml:"git_path"`          // glob pattern to filter by changed file paths (for git type)
	GitPollInterval string   `yaml:"git_poll_interval"` // polling interval duration (e.g. "30s", "1m") for git type
}

// AssertConfig defines per-task output assertion configuration.
type AssertConfig struct {
	Stdout *StdoutAssertion `yaml:"stdout,omitempty"` // stdout assertion configuration
	Stderr *StderrAssertion `yaml:"stderr,omitempty"` // stderr assertion configuration
	Files  []FileAssertion  `yaml:"files,omitempty"`  // file assertion configuration
	Soft   bool             `yaml:"soft,omitempty"`   // if true, failed assertions log warnings instead of failing task
}

// StdoutAssertion defines stdout content assertions.
type StdoutAssertion struct {
	Contains  string `yaml:"contains,omitempty"`   // must contain this string
	Matches   string `yaml:"matches,omitempty"`    // must match this regex pattern
	JSONValid bool   `yaml:"json_valid,omitempty"` // must be valid JSON
}

// StderrAssertion defines stderr content assertions.
type StderrAssertion struct {
	Empty bool `yaml:"empty,omitempty"` // must be empty
}

// FileAssertion defines file content assertions.
type FileAssertion struct {
	Path    string `yaml:"path"`              // path to file (relative to project root)
	Exists  *bool  `yaml:"exists,omitempty"`  // file must exist (nil = don't check)
	Contains string `yaml:"contains,omitempty"` // file must contain this string
	SizeMin  int64 `yaml:"size_min,omitempty"`  // file size must be at least this many bytes
	SizeMax  int64 `yaml:"size_max,omitempty"`  // file size must be at most this many bytes
}

// CacheConfig defines per-task output caching configuration.
type CacheConfig struct {
	Enabled bool   `yaml:"enabled"` // whether caching is enabled
	Key     string `yaml:"key"`     // cache key template
	TTL     string `yaml:"ttl"`     // cache TTL duration string
}

// PreconditionConfig defines task precondition checks for conditional execution.
// All conditions must pass for the task to run.
type PreconditionConfig struct {
	DayOfWeek   string `yaml:"day_of_week,omitempty"`   // allowed days: range "1-5", list "1,3,5", or combined "1-5,0" (0=Sunday)
	TimeRange   string `yaml:"time_range,omitempty"`    // allowed time range in "HH:MM-HH:MM" format (24h)
	EnvSet      string `yaml:"env_set,omitempty"`       // environment variable that must be set
	Expr        string `yaml:"expr,omitempty"`          // expression to evaluate (Go template with conditionals)
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
	ConcurrencyGroup string       // concurrency group name (empty = default group)
	SkipPermissions bool          // if true, append --dangerously-skip-permissions to runner command
	AllowedTools    []string      // if non-empty, append --allowedTools <tools> to runner command
	PreCheck        string        // optional shell command; task is skipped silently if it exits non-zero
	Precondition    *PreconditionConfig // optional precondition logic; task is skipped if condition evaluates to false
	OnSuccess       string        // optional shell command to run after successful completion
	OnFailure       string        // optional shell command to run after failed completion
	IsLocked        bool          // true if a stale lock file exists
	Disabled        bool          // if true, task is paused and skipped during tick evaluation
	Timeout         time.Duration // task-specific timeout (0 = use global default)
	Retry           int           // number of retries on failure (0 = no retry)
	RetryDelay      time.Duration // delay between retries (default 1m, used with Retry)
	RetryStrategy   string        // backoff strategy: "exponential", "linear", "constant" (default "exponential")
	RetryJitter     float64       // jitter percentage 0.0-1.0 to randomize retry delays
	RetryMaxTime    time.Duration // max total wall-clock time for retries (0 = unlimited)
	// Persistent task configuration
	PersistentCooldown   time.Duration // cooldown between restart cycles (default 0 = immediate)
	PersistentMaxRuntime time.Duration // max runtime before forced restart (0 = no limit)
	PersistentBudget     time.Duration // cumulative wall-clock budget per daemon lifetime (0 = unlimited)
	CostBudget           float64       // cumulative USD budget per daemon lifetime (0 = unlimited)
	MaxLogSize           int64         // max log file size in bytes (0 = use global default)
	Runner               string        // per-task runner command override (empty = use global runner chain)
	RunnerChain          []string      // per-task runner chain (tried in sequence on failure)
	RunnerOnTimeout      string        // fallback runner when task times out
	Webhook              string        // per-task webhook URL override (empty = use global webhooks only)
	Labels               []string      // user-defined labels for organizing and filtering tasks
	Env                  map[string]string // environment variables injected into task execution
	CaptureOutput        bool                 // if true, capture ##anvil:result output and store in run record
	Checkpoint           bool                 // if true, capture ##anvil:checkpoint output and inject on resume
	CheckpointGracePeriod time.Duration       // max wait time after SIGTERM before SIGKILL (default 30s)
	Window               AllowedWindow // per-task execution time window (empty = no restriction)
	ForceWindow          bool          // if true, bypass time window and quiet hours checks (set by force-run)
	SLA                  SLAConfig     // per-task SLA tracking configuration
	PriorityAging        *PriorityAgingConfig // per-task priority aging configuration
	OnSLAViolation       string        // shell command to run when SLA is violated
	CircuitBreaker       CircuitBreakerConfig // per-task circuit breaker configuration
	OnCircuitOpen        string        // shell command to run when circuit opens
	OnCircuitClose       string        // shell command to run when circuit closes
	Alerts               AlertConfig   // per-task alert configuration
	State                *TaskStateConfig // optional task state management config
	NotifyOnFailure      *bool         // per-task override for failure notifications (nil = use global)
	NotifyOnSuccess      *bool         // per-task override for success notifications (nil = use global)
	NodeAffinity         string        // cluster node ID for affinity-based execution (empty = any node)
	OnRollback           string        // shell command to run before rollback (supports {{ .RunID }}, {{ .TaskName }} templates)
	HealthCheck          string        // shell command for health check (empty = no health check)
	Subscription         *SubscriptionConfig // message subscription configuration (nil = no subscription)
	Cache                *CacheConfig  // task output caching configuration (nil = no caching)
	// Rate limiting configuration
	// Replay functionality
	Replay               bool   // if true, save successful outputs for replay
	PinnedRun            string // if set, always use output from this specific run ID
	RateLimit            RateLimitConfig // per-task rate limiting configuration
	// Assertion configuration for validating task output
	Assert               *AssertConfig   // output assertion configuration (nil = no assertions)
	// Backfill configuration for missed cron windows
	Backfill             *BackfillConfig // backfill configuration (nil = no backfill)
	// Trigger conditions for multi-criteria triggering
	Trigger              *TaskTrigger // trigger configuration (nil = schedule-only)
	// Timeout escalation configuration
	TimeoutWarning       time.Duration          // duration before timeout to trigger warning (0 = no warning)
	OnTimeoutWarning     string                 // shell command to run when timeout warning is triggered
	OnTimeout            string                 // shell command to run when task times out
	AdaptiveTimeout      *AdaptiveTimeoutConfig // adaptive timeout configuration (nil = no adaptive timeout)
}

// AdaptiveTimeoutConfig defines adaptive timeout behavior that extends deadlines based on task progress.
type AdaptiveTimeoutConfig struct {
	Enabled           bool          `yaml:"enabled"`            // whether adaptive timeout is enabled
	ExtendIf          string        `yaml:"extend_if"`          // condition that triggers extension ("checkpoint_exists")
	MaxExtensions     int           `yaml:"max_extensions"`     // maximum number of extensions allowed (0 = unlimited)
	ExtensionDuration time.Duration `yaml:"extension_duration"` // duration to extend by (0 = use original timeout)
}

// BackfillConfig defines backfill configuration for missed cron windows.
type BackfillConfig struct {
	Enabled   bool          `yaml:"enabled"`    // whether backfill is enabled
	MaxDelay  time.Duration `yaml:"max_delay"`  // maximum delay to allow backfill (0 = unlimited)
	Mode      string        `yaml:"mode"`       // backfill mode: "exact" or "when_idle"
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
	RetryStrategy    string    `json:"retry_strategy,omitempty"`    // backoff strategy used: "exponential", "linear", "constant"
	RetryDelaysUsed  []string  `json:"retry_delays_used,omitempty"` // actual delays between each retry attempt
	ScheduledTime    time.Time     `json:"scheduled_time,omitempty"`    // when the task was supposed to run (cron prev match)
	DispatchDelay    time.Duration `json:"dispatch_delay,omitempty"`    // actual delay from scheduled time
	SLAViolation     bool          `json:"sla_violation,omitempty"`     // whether this run violated SLA
	SLAMaxDelay      time.Duration `json:"sla_max_delay,omitempty"`     // configured max_delay at time of dispatch
	SLASkipped       bool          `json:"sla_skipped,omitempty"`       // true if strict mode skipped this run
	RunnerIndex      int           `json:"runner_index,omitempty"`      // which runner in the chain was used (0-based; 100+ means timeout fallback)
	RunnerCommand    string        `json:"runner_command,omitempty"`    // the actual runner command that was used
	NodeID           string        `json:"node_id,omitempty"`            // cluster node that executed this run
	ResultData       string        `json:"result_data,omitempty"`        // captured result from ##anvil:result output
}
// TaskVersion is a snapshot of a task file at a specific point in time.
type TaskVersion struct {
	VersionNumber int       `json:"version_number"`
	TaskName      string    `json:"task_name"`
	Content       string    `json:"content"`
	ContentHash   string    `json:"content_hash"`
	Timestamp     time.Time `json:"timestamp"`
	Author        string    `json:"author"`
	Summary       string    `json:"summary,omitempty"`
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
			retryStrategy := ""
			retryJitter := 0.0
			var retryMaxTime time.Duration
			var persistentCooldown time.Duration
			var persistentMaxRuntime time.Duration
			var persistentBudget time.Duration
			var costBudget float64
			var maxLogSize int64
			runnerOverride := ""
			var runnerChain []string
			runnerOnTimeout := ""
			webhookURL := ""
			var labels []string
			var nodeAffinity string
			var envVars map[string]string
			captureOutput := false
			checkpoint := false
			var checkpointGracePeriod time.Duration
			healthCheck := ""
			var allowedWindow AllowedWindow
			var slaConfig SLAConfig
			var priorityAgingConfig *PriorityAgingConfig
			onSLAViolation := ""
			onRollback := ""
			replay := false
			pinnedRun := ""
			var precondition *PreconditionConfig
			var trigger *TaskTrigger
			var timeoutWarning time.Duration
			onTimeoutWarning := ""
			onTimeout := ""
			var adaptiveTimeout *AdaptiveTimeoutConfig
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
						Precondition         *PreconditionConfig `yaml:"precondition"`
						OnSuccess            string   `yaml:"on_success"`
						OnFailure            string   `yaml:"on_failure"`
						Disabled             bool     `yaml:"disabled"`
						Timeout              string   `yaml:"timeout"`
						Retry                int      `yaml:"retry"`
						RetryDelay           string   `yaml:"retry_delay"`
						RetryStrategy        string   `yaml:"retry_strategy"`
						RetryJitter          float64  `yaml:"retry_jitter"`
						RetryMaxTime         string   `yaml:"retry_max_time"`
						PersistentCooldown   string   `yaml:"persistent_cooldown"`
						PersistentMaxRuntime string   `yaml:"persistent_max_runtime"`
						PersistentBudget     string   `yaml:"persistent_budget"`
						CostBudget          string   `yaml:"cost_budget"`
						MaxLogSize           string   `yaml:"max_log_size"`
						Runner               string   `yaml:"runner"`
						RunnerChain          []string `yaml:"runner_chain"`
						RunnerOnTimeout      string   `yaml:"runner_on_timeout"`
						Webhook              string   `yaml:"webhook"`
						Labels               []string          `yaml:"labels"`
						Env                  map[string]string `yaml:"env"`
						CaptureOutput        bool              `yaml:"capture_output"`
						Checkpoint           bool              `yaml:"checkpoint"`
						CheckpointGracePeriod string           `yaml:"checkpoint_grace_period"`
						HealthCheck          string            `yaml:"health_check"`
						AllowedWindow        *AllowedWindow    `yaml:"allowed_window"`
						SLA                  *struct {
							MaxDelay string `yaml:"max_delay"`
							Strict   bool   `yaml:"strict"`
						} `yaml:"sla"`
						PriorityAging        *struct {
							Enabled   *bool         `yaml:"enabled"`
							MaxWait   string        `yaml:"max_wait"`
							MaxBoost  *int          `yaml:"max_boost"`
						} `yaml:"priority_aging"`
						OnSLAViolation       string            `yaml:"on_sla_violation"`
						OnRollback           string            `yaml:"on_rollback"`
						NodeAffinity         string            `yaml:"node"`
						// Replay functionality
						Replay               bool   `yaml:"replay"`
						PinnedRun            string `yaml:"pinned_run"`
						Trigger              *TaskTrigger      `yaml:"trigger"`
						// Timeout escalation
						TimeoutWarning       string `yaml:"timeout_warning"`
						OnTimeoutWarning     string `yaml:"on_timeout_warning"`
						OnTimeout            string `yaml:"on_timeout"`
						AdaptiveTimeout      *struct {
							Enabled           bool   `yaml:"enabled"`
							ExtendIf          string `yaml:"extend_if"`
							MaxExtensions     int    `yaml:"max_extensions"`
							ExtensionDuration string `yaml:"extension_duration"`
						} `yaml:"adaptive_timeout"`
					}
					if err := yaml.Unmarshal([]byte(fm), &fmData); err != nil {
						// Log the error but continue - the task will load with defaults
						// This is safer than silently skipping, which causes tasks to "disappear"
						log.Printf("WARN: failed to parse frontmatter for %s: %v (using defaults)", e.Name(), err)
					} else {
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
						// T002/T006/T007: Parse retry strategy fields with validation
						if fmData.RetryStrategy != "" {
							switch strings.ToLower(fmData.RetryStrategy) {
							case "exponential", "linear", "constant":
								retryStrategy = strings.ToLower(fmData.RetryStrategy)
							default:
								log.Printf("WARN: invalid retry_strategy %q for %s, defaulting to exponential", fmData.RetryStrategy, e.Name())
								retryStrategy = "exponential"
							}
						}
						if fmData.RetryJitter != 0 {
							retryJitter = fmData.RetryJitter
							if retryJitter < 0 {
								log.Printf("WARN: retry_jitter %.2f for %s clamped to 0.0", retryJitter, e.Name())
								retryJitter = 0
							} else if retryJitter > 1.0 {
								log.Printf("WARN: retry_jitter %.2f for %s clamped to 1.0", retryJitter, e.Name())
								retryJitter = 1.0
							}
						}
						if fmData.RetryMaxTime != "" {
							retryMaxTime, _ = time.ParseDuration(fmData.RetryMaxTime)
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
						if fmData.CostBudget != "" {
							costBudget, _ = strconv.ParseFloat(fmData.CostBudget, 64)
						}
						if fmData.MaxLogSize != "" {
							maxLogSize, _ = config.ParseByteSize(fmData.MaxLogSize)
						}
						runnerOverride = fmData.Runner
						runnerChain = fmData.RunnerChain
						runnerOnTimeout = fmData.RunnerOnTimeout
						webhookURL = fmData.Webhook
						labels = fmData.Labels
						envVars = fmData.Env
						captureOutput = fmData.CaptureOutput
						checkpoint = fmData.Checkpoint
						if fmData.CheckpointGracePeriod != "" {
							checkpointGracePeriod, _ = time.ParseDuration(fmData.CheckpointGracePeriod)
						}
						if fmData.AllowedWindow != nil {
							allowedWindow = *fmData.AllowedWindow
						}
						if fmData.SLA != nil && fmData.SLA.MaxDelay != "" {
							if d, err := time.ParseDuration(fmData.SLA.MaxDelay); err == nil {
								slaConfig.MaxDelay = d
								slaConfig.Strict = fmData.SLA.Strict
							}
						}
						// Process priority aging configuration
						if fmData.PriorityAging != nil {
							priorityAgingConfig = &PriorityAgingConfig{}
							if fmData.PriorityAging.Enabled != nil {
								priorityAgingConfig.Enabled = fmData.PriorityAging.Enabled
							}
							if fmData.PriorityAging.MaxWait != "" {
								if d, err := time.ParseDuration(fmData.PriorityAging.MaxWait); err == nil {
									priorityAgingConfig.MaxWait = d
								}
							}
							if fmData.PriorityAging.MaxBoost != nil {
								priorityAgingConfig.MaxBoost = *fmData.PriorityAging.MaxBoost
							}
						}
						onSLAViolation = fmData.OnSLAViolation
						healthCheck = fmData.HealthCheck
						onRollback = fmData.OnRollback
						trigger = fmData.Trigger
						replay = fmData.Replay
						pinnedRun = fmData.PinnedRun
						// Timeout escalation fields
						if fmData.TimeoutWarning != "" {
							timeoutWarning, _ = time.ParseDuration(fmData.TimeoutWarning)
						}
						onTimeoutWarning = fmData.OnTimeoutWarning
						onTimeout = fmData.OnTimeout
						if fmData.AdaptiveTimeout != nil && fmData.AdaptiveTimeout.Enabled {
							adaptiveTimeout = &AdaptiveTimeoutConfig{
								Enabled:       true,
								ExtendIf:      fmData.AdaptiveTimeout.ExtendIf,
								MaxExtensions: fmData.AdaptiveTimeout.MaxExtensions,
							}
							if fmData.AdaptiveTimeout.ExtensionDuration != "" {
								adaptiveTimeout.ExtensionDuration, _ = time.ParseDuration(fmData.AdaptiveTimeout.ExtensionDuration)
							}
						}
					}
				}
			}

			// Apply project defaults for fields not explicitly set in frontmatter.
			applyDefaults(defaults, fmKeys, &skipPermissions, &allowedTools, &preCheck, &precondition,
				&onSuccess, &onFailure, &timeout, &retry, &retryDelay,
				&maxConcurrent, &persistentCooldown, &persistentMaxRuntime, &persistentBudget, &maxLogSize, &runnerOverride,
				&retryStrategy, &retryJitter, &retryMaxTime)

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
				Precondition:         precondition,
				OnSuccess:            onSuccess,
				OnFailure:            onFailure,
				IsLocked:             hasLock,
				Disabled:             disabled,
				Timeout:              timeout,
				Retry:                retry,
				RetryDelay:           retryDelay,
				RetryStrategy:        retryStrategy,
				RetryJitter:          retryJitter,
				RetryMaxTime:         retryMaxTime,
				PersistentCooldown:   persistentCooldown,
				PersistentMaxRuntime: persistentMaxRuntime,
				PersistentBudget:     persistentBudget,
				CostBudget:           costBudget,
				MaxLogSize:           maxLogSize,
				Runner:               runnerOverride,
				RunnerChain:          runnerChain,
				RunnerOnTimeout:      runnerOnTimeout,
				Webhook:              webhookURL,
				Labels:               labels,
				Env:                  resolvedEnv,
				CaptureOutput:        captureOutput,
				Checkpoint:           checkpoint,
				CheckpointGracePeriod: checkpointGracePeriod,
				HealthCheck:          healthCheck,
				Window:               allowedWindow,
				SLA:                  slaConfig,
				PriorityAging:        priorityAgingConfig,
				OnSLAViolation:       onSLAViolation,
				NodeAffinity:         nodeAffinity,
				OnRollback:           onRollback,
				Trigger:              trigger,
				Replay:               replay,
				PinnedRun:            pinnedRun,
				TimeoutWarning:       timeoutWarning,
				OnTimeoutWarning:     onTimeoutWarning,
				OnTimeout:            onTimeout,
				AdaptiveTimeout:      adaptiveTimeout,
			})
		}
	}

	return todos, nil
}

// applyDefaults fills in project-level defaults for any task field not explicitly set
// in the task's frontmatter. fmKeys is the set of keys present in the parsed YAML;
// a nil map means no frontmatter was present (all defaults apply).
func applyDefaults(defaults TaskDefaults, fmKeys map[string]interface{},
	skipPermissions *bool, allowedTools *[]string, preCheck *string, precondition **PreconditionConfig,
	onSuccess *string, onFailure *string, timeout *time.Duration,
	retry *int, retryDelay *time.Duration, maxConcurrent *int,
	persistentCooldown *time.Duration, persistentMaxRuntime *time.Duration,
	persistentBudget *time.Duration, maxLogSize *int64, runnerOverride *string,
	retryStrategy *string, retryJitter *float64, retryMaxTime *time.Duration) {

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
	if !has("precondition") && defaults.Precondition != nil {
		*precondition = defaults.Precondition
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
	if !has("retry_strategy") && defaults.RetryStrategy != "" {
		*retryStrategy = defaults.RetryStrategy
	}
	if !has("retry_jitter") && defaults.RetryJitter != 0 {
		*retryJitter = defaults.RetryJitter
	}
	if !has("retry_max_time") && defaults.RetryMaxTime != "" {
		if d, err := time.ParseDuration(defaults.RetryMaxTime); err == nil {
			*retryMaxTime = d
		}
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

// InitResult holds metadata about an Init operation.
type InitResult struct {
	AlreadyExists bool   // true if .anvil/ already existed
	TaskCount     int    // number of existing tasks found
	BackupPath    string // path to backup directory if tasks were backed up
}

// Init creates the .anvil/ directory structure and writes embedded tools into .claude/.
// The toolsFS should contain a "skills" directory at its root.
// If force is true and a project already exists, existing tasks are preserved.
// Priority subdirectories are created on-demand when tasks are added.
func Init(path string, toolsFS fs.FS, force bool) (InitResult, error) {
	result := InitResult{}
	todosDir := filepath.Join(path, ".anvil", "todos")

	// Check if project already exists
	if _, err := os.Stat(todosDir); err == nil {
		result.AlreadyExists = true
		// Count existing tasks
		entries, _ := os.ReadDir(todosDir)
		for _, e := range entries {
			if e.IsDir() {
				subEntries, _ := os.ReadDir(filepath.Join(todosDir, e.Name()))
				result.TaskCount += len(subEntries)
			}
		}
	}

	if err := os.MkdirAll(todosDir, 0755); err != nil {
		return result, fmt.Errorf("creating .anvil/todos: %w", err)
	}

	claudeDir := filepath.Join(path, ".claude")
	if err := writeEmbeddedFS(claudeDir, toolsFS); err != nil {
		return result, fmt.Errorf("writing .claude/ tools: %w", err)
	}

	return result, nil
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
		sb.WriteString(fmt.Sprintf("allowed_tools: [%s]\n", allowedTools))
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

	// Log task creation activity
	WriteActivity(p.Path, ActivityEntry{
		Timestamp: time.Now(),
		Action:    "created",
		TaskID:    id,
		TaskName:  strings.TrimSuffix(filename, ".md"),
		Details:   map[string]string{"priority": fmt.Sprintf("%d", priority), "schedule": schedule},
	})

	return fmt.Sprintf("p%d/%s", priority, filename), nil
}

// AddTodoWithID creates a new task and returns both the relative path and the task ID.
// This is useful for commands that need to track the task after creation.
func (p *Project) AddTodoWithID(priority int, schedule string, content string, preCheck string, allowedTools string, maxConcurrent int, skipPermissions bool, runnerCmd string) (string, string, error) {
	if priority < 0 || priority > 9 {
		return "", "", fmt.Errorf("priority must be 0-9, got %d", priority)
	}
	if strings.TrimSpace(content) == "" {
		return "", "", fmt.Errorf("task content must not be empty")
	}

	// Validate cron expression before writing the task file.
	// Skip validation for "persistent" since it's a special keyword, not a cron expression.
	if schedule != "" && schedule != "persistent" {
		if _, err := cron.Parse(schedule); err != nil {
			return "", "", fmt.Errorf("invalid schedule %q: %w", schedule, err)
		}
	}

	dir := filepath.Join(p.Path, ".anvil", "todos", fmt.Sprintf("p%d", priority))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", "", fmt.Errorf("creating todos/p%d: %w", priority, err)
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
		sb.WriteString(fmt.Sprintf("allowed_tools: [%s]\n", allowedTools))
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
		return "", "", fmt.Errorf("writing todo file: %w", err)
	}

	// Log task creation activity
	WriteActivity(p.Path, ActivityEntry{
		Timestamp: time.Now(),
		Action:    "created",
		TaskID:    id,
		TaskName:  strings.TrimSuffix(filename, ".md"),
		Details:   map[string]string{"priority": fmt.Sprintf("%d", priority), "schedule": schedule},
	})

	return fmt.Sprintf("p%d/%s", priority, filename), id, nil
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

// rateLimitsDir returns the path to the rate limits directory for a task.
func rateLimitsDir(projectPath, taskID string) string {
	return filepath.Join(projectPath, ".anvil", "rate-limits", taskID)
}

// RateLimitCounter represents the execution counters for rate limiting.
type RateLimitCounter struct {
	ThisHourCount int       `json:"this_hour_count"`
	ThisHourStart time.Time `json:"this_hour_start"`
	ThisDayCount  int       `json:"this_day_count"`
	ThisDayStart  time.Time `json:"this_day_start"`
}

// ReadRateLimitCounter reads the current rate limit counter for a task.
func ReadRateLimitCounter(projectPath, taskID string) (RateLimitCounter, error) {
	dir := rateLimitsDir(projectPath, taskID)
	path := filepath.Join(dir, "counter.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return zero-value counter if file doesn't exist
			return RateLimitCounter{}, nil
		}
		return RateLimitCounter{}, err
	}

	var counter RateLimitCounter
	if err := json.Unmarshal(data, &counter); err != nil {
		return RateLimitCounter{}, err
	}

	// Reset counters if periods have passed
	now := time.Now()
	resetHour := counter.ThisHourStart.Add(time.Hour).Before(now)
	resetDay := counter.ThisDayStart.Add(24 * time.Hour).Before(now)

	if resetHour {
		counter.ThisHourCount = 0
		counter.ThisHourStart = now.Truncate(time.Hour)
	}

	if resetDay {
		counter.ThisDayCount = 0
		counter.ThisDayStart = now.Truncate(24 * time.Hour)
	}

	return counter, nil
}

// WriteRateLimitCounter writes the rate limit counter for a task.
func WriteRateLimitCounter(projectPath, taskID string, counter RateLimitCounter) error {
	dir := rateLimitsDir(projectPath, taskID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, "counter.json")
	data, err := json.Marshal(counter)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// ResetRateLimitCounter resets the rate limit counter for a task.
func ResetRateLimitCounter(projectPath, taskID string) error {
	dir := rateLimitsDir(projectPath, taskID)
	path := filepath.Join(dir, "counter.json")
	return os.Remove(path)
}


// VersionsDir returns the path to the versions directory for a task.
func VersionsDir(projectPath, taskName string) string {
	return filepath.Join(projectPath, ".anvil", "versions", taskName)
}

// VersionPath returns the path to a specific version snapshot.
func VersionPath(projectPath, taskName string, versionNum int) string {
	return filepath.Join(VersionsDir(projectPath, taskName), fmt.Sprintf("v%d.json", versionNum))
}


// GetAuthor returns the author name from git config or system username.
func GetAuthor(projectPath string) string {
	// Try git config user.name
	cmd := exec.Command("git", "config", "user.name")
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err == nil {
		name := strings.TrimSpace(string(out))
		if name != "" {
			return name
		}
	}
	// Fall back to system username
	u, err := user.Current()
	if err == nil && u.Username != "" {
		return u.Username
	}
	return "unknown"
}

// ComputeFileHash returns the SHA256 hex hash of the given content.
func ComputeFileHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
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

// ReadRunRecord reads a specific run record by task ID and run ID.
func ReadRunRecord(projectPath, taskID, runID string) (*RunRecord, error) {
	recPath := RunPath(projectPath, taskID, runID)
	data, err := os.ReadFile(recPath)
	if err != nil {
		return nil, fmt.Errorf("reading run record: %w", err)
	}

	var rec RunRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("parsing run record: %w", err)
	}
	return &rec, nil
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


// WriteTaskVersion writes a version snapshot for a task.
func WriteTaskVersion(projectPath, taskName, content, author, summary string) (TaskVersion, error) {
	dir := VersionsDir(projectPath, taskName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return TaskVersion{}, fmt.Errorf("creating versions dir: %w", err)
	}

	// Determine next version number
	versions, _ := ReadAllVersions(projectPath, taskName)
	nextNum := 1
	for _, v := range versions {
		if v.VersionNumber >= nextNum {
			nextNum = v.VersionNumber + 1
		}
	}

	if summary == "" {
		if nextNum == 1 {
			summary = "initial version"
		} else {
			summary = "modified"
		}
	}

	tv := TaskVersion{
		VersionNumber: nextNum,
		TaskName:      taskName,
		Content:       content,
		ContentHash:   ComputeFileHash(content),
		Timestamp:     time.Now(),
		Author:        author,
		Summary:       summary,
	}

	data, err := json.MarshalIndent(tv, "", "  ")
	if err != nil {
		return TaskVersion{}, fmt.Errorf("marshaling version: %w", err)
	}

	vPath := VersionPath(projectPath, taskName, nextNum)
	if err := os.WriteFile(vPath, data, 0644); err != nil {
		return TaskVersion{}, fmt.Errorf("writing version: %w", err)
	}

	return tv, nil
}

// ReadAllVersions reads all version snapshots for a task, sorted by version number (newest first).
func ReadAllVersions(projectPath, taskName string) ([]TaskVersion, error) {
	dir := VersionsDir(projectPath, taskName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading versions dir: %w", err)
	}

	var versions []TaskVersion
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		vPath := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(vPath)
		if err != nil {
			continue
		}
		var tv TaskVersion
		if err := json.Unmarshal(data, &tv); err != nil {
			continue
		}
		versions = append(versions, tv)
	}

	// Sort by version number, newest first
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].VersionNumber > versions[j].VersionNumber
	})

	return versions, nil
}

// ReadVersion reads a specific version of a task by version number.
func ReadVersion(projectPath, taskName string, versionNum int) (TaskVersion, error) {
	vPath := VersionPath(projectPath, taskName, versionNum)
	data, err := os.ReadFile(vPath)
	if err != nil {
		if os.IsNotExist(err) {
			return TaskVersion{}, fmt.Errorf("version not found: v%d", versionNum)
		}
		return TaskVersion{}, fmt.Errorf("reading version: %w", err)
	}
	var tv TaskVersion
	if err := json.Unmarshal(data, &tv); err != nil {
		return TaskVersion{}, fmt.Errorf("parsing version: %w", err)
	}
	return tv, nil
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

// LatestResultData returns the most recent result data for a task.
// Returns empty string if no result data exists.
func LatestResultData(projectPath, taskID string) string {
	rec, err := ReadCurrentRunRecord(projectPath, taskID)
	if err != nil {
		return ""
	}
	return rec.ResultData
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

// stateDir returns the path to the state directory for a bucket.
func stateDir(projectPath, bucket string) string {
	return filepath.Join(projectPath, ".anvil", "state", bucket)
}

// ReadTaskState reads a task state from the state bucket.
func ReadTaskState(projectPath, bucket, key string) (map[string]interface{}, error) {
	p := filepath.Join(stateDir(projectPath, bucket), key+".json")
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return state, nil
}

// WriteTaskState writes a task state to the state bucket.
func WriteTaskState(projectPath, bucket, key string, state map[string]interface{}) error {
	dir := stateDir(projectPath, bucket)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, key+".json"), data, 0644)
}

// DeleteTaskState removes a task state from the state bucket.
func DeleteTaskState(projectPath, bucket, key string) error {
	return os.Remove(filepath.Join(stateDir(projectPath, bucket), key+".json"))
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
	Name        string `yaml:"name,omitempty"`
	Version     string `yaml:"version,omitempty"`
	Description string `yaml:"description,omitempty"`
	Schedule              string            `yaml:"schedule,omitempty"`
	Priority              int               `yaml:"priority,omitempty"`
	AllowedTools          []string          `yaml:"allowed_tools,omitempty"`
	PreCheck              string            `yaml:"pre_check,omitempty"`
	Precondition          *PreconditionConfig `yaml:"precondition,omitempty"`
	OnSuccess             string            `yaml:"on_success,omitempty"`
	OnFailure             string            `yaml:"on_failure,omitempty"`
	SkipPermissions       bool              `yaml:"skip_permissions,omitempty"`
	Timeout               string            `yaml:"timeout,omitempty"`
	Retry                int               `yaml:"retry,omitempty"`
	RetryDelay           string            `yaml:"retry_delay,omitempty"`
	RetryStrategy        string            `yaml:"retry_strategy,omitempty"`
	RetryJitter          float64           `yaml:"retry_jitter,omitempty"`
	RetryMaxTime         string            `yaml:"retry_max_time,omitempty"`
	MaxConcurrent        int               `yaml:"max_concurrent,omitempty"`
	PersistentCooldown    string            `yaml:"persistent_cooldown,omitempty"`
	PersistentMaxRuntime string            `yaml:"persistent_max_runtime,omitempty"`
	PersistentBudget     string            `yaml:"persistent_budget,omitempty"`
	MaxLogSize           int64             `yaml:"max_log_size,omitempty"`
	Runner               string            `yaml:"runner,omitempty"`
	Webhook              string            `yaml:"webhook,omitempty"`
	Labels               []string          `yaml:"labels,omitempty"`
	Env                  map[string]string `yaml:"env,omitempty"`
	CaptureOutput        bool              `yaml:"capture_output,omitempty"`
	Checkpoint           bool              `yaml:"checkpoint,omitempty"`
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


// ActivityEntry represents a single event in a task's lifecycle.
type ActivityEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Action    string            `json:"action"`
	TaskID    string            `json:"task_id"`
	TaskName  string            `json:"task_name"`
	Details   map[string]string `json:"details,omitempty"`
}

// ActivitiesPath returns the path to the activity log file for a task.
func ActivitiesPath(projectPath, taskID string) string {
	return filepath.Join(projectPath, ".anvil", "activities", taskID+".jsonl")
}

// WriteActivity appends an activity entry to the task's activity log.
func WriteActivity(projectPath string, entry ActivityEntry) error {
	if entry.TaskID == "" {
		return nil // silently skip if no task ID
	}
	dir := filepath.Join(projectPath, ".anvil", "activities")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating activities dir: %w", err)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling activity entry: %w", err)
	}
	f, err := os.OpenFile(ActivitiesPath(projectPath, entry.TaskID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening activity file: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("writing activity entry: %w", err)
	}
	return nil
}

// ReadActivities reads all activity entries for a task, returning them in chronological order.
func ReadActivities(projectPath, taskID string) ([]ActivityEntry, error) {
	data, err := os.ReadFile(ActivitiesPath(projectPath, taskID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading activity file: %w", err)
	}
	var entries []ActivityEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var entry ActivityEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// EvaluatePrecondition checks if the task's precondition is satisfied.
// Returns true and empty string if precondition passes, false and reason if it fails.
func (t Todo) EvaluatePrecondition(projectPath string) (bool, string) {
	// Use current time for evaluation
	now := time.Now()

	passed, err := EvaluatePrecondition(t.Precondition, now)
	if err != nil {
		return false, fmt.Sprintf("error evaluating precondition: %v", err)
	}

	if !passed {
		// Provide specific reason based on which condition failed
		if t.Precondition.DayOfWeek != "" {
			return false, fmt.Sprintf("day of week condition '%s' not met", t.Precondition.DayOfWeek)
		}
		if t.Precondition.TimeRange != "" {
			return false, fmt.Sprintf("time range condition '%s' not met", t.Precondition.TimeRange)
		}
		if t.Precondition.EnvSet != "" {
			return false, fmt.Sprintf("environment variable '%s' not set", t.Precondition.EnvSet)
		}
		if t.Precondition.Expr != "" {
			return false, fmt.Sprintf("expression condition '%s' not met", t.Precondition.Expr)
		}
		return false, "precondition not met"
	}

	return true, ""
}

// EvaluatePrecondition checks if a task's precondition is satisfied.
// Returns true if the task should run, false if it should be skipped.
// If precondition is nil or all conditions pass, returns true.
func EvaluatePrecondition(precond *PreconditionConfig, now time.Time) (bool, error) {
	if precond == nil {
		return true, nil // No precondition, so condition passes
	}

	// Check day of week condition
	if precond.DayOfWeek != "" {
		if !matchesDayOfWeek(precond.DayOfWeek, now.Weekday()) {
			return false, nil // Day of week condition failed
		}
	}

	// Check time range condition
	if precond.TimeRange != "" {
		if !matchesTimeRange(precond.TimeRange, now) {
			return false, nil // Time range condition failed
		}
	}

	// Check environment variable condition
	if precond.EnvSet != "" {
		if os.Getenv(precond.EnvSet) == "" {
			return false, nil // Environment variable not set
		}
	}

	// Check expression condition
	if precond.Expr != "" {
		passed, err := evaluateExpression(precond.Expr, now)
		if err != nil {
			return false, fmt.Errorf("error evaluating precondition expression: %w", err)
		}
		if !passed {
			return false, nil // Expression condition failed
		}
	}

	return true, nil // All conditions passed
}

// matchesDayOfWeek checks if the current weekday matches the allowed days specification.
// Format: "1-5" (Monday-Friday), "1,3,5" (Monday,Wednesday,Friday), "1-5,0" (Mon-Fri,Sun)
func matchesDayOfWeek(spec string, weekday time.Weekday) bool {
	// Convert Go's Sunday=0 to our format where Sunday=0
	dayNum := int(weekday)

	// Handle comma-separated values
	parts := strings.Split(spec, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			// Range format (e.g., "1-5")
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				continue
			}
			start, err1 := strconv.Atoi(rangeParts[0])
			end, err2 := strconv.Atoi(rangeParts[1])
			if err1 != nil || err2 != nil {
				continue
			}
			if dayNum >= start && dayNum <= end {
				return true
			}
		} else {
			// Single day format (e.g., "1")
			day, err := strconv.Atoi(part)
			if err != nil {
				continue
			}
			if day == dayNum {
				return true
			}
		}
	}
	return false
}

// matchesTimeRange checks if the current time falls within the specified time range.
// Format: "09:00-17:00" (24-hour format)
func matchesTimeRange(spec string, now time.Time) bool {
	parts := strings.Split(spec, "-")
	if len(parts) != 2 {
		return false
	}

	startTime := strings.TrimSpace(parts[0])
	endTime := strings.TrimSpace(parts[1])

	start, err1 := time.Parse("15:04", startTime)
	end, err2 := time.Parse("15:04", endTime)
	if err1 != nil || err2 != nil {
		return false
	}

	// Get current time components
	current := time.Date(0, 1, 1, now.Hour(), now.Minute(), 0, 0, time.UTC)
	start = time.Date(0, 1, 1, start.Hour(), start.Minute(), 0, 0, time.UTC)
	end = time.Date(0, 1, 1, end.Hour(), end.Minute(), 0, 0, time.UTC)

	// Handle overnight ranges (e.g., 22:00-06:00)
	if end.Before(start) {
		return current.After(start) || current.Before(end)
	}

	return current.After(start) && current.Before(end)
}

// evaluateExpression evaluates a Go template expression with time context.
// The expression should evaluate to a boolean value.
func evaluateExpression(expr string, now time.Time) (bool, error) {
	// Create template context with time variables
	context := map[string]interface{}{
		"Hour":       now.Hour(),
		"Minute":     now.Minute(),
		"DayOfWeek":  int(now.Weekday()),
		"DayOfMonth": now.Day(),
		"Month":      int(now.Month()),
		"IsWeekend":  now.Weekday() == time.Saturday || now.Weekday() == time.Sunday,
		// Helper functions for comparisons
		"gt": func(a, b int) bool { return a > b },
		"lt": func(a, b int) bool { return a < b },
		"ge": func(a, b int) bool { return a >= b },
		"le": func(a, b int) bool { return a <= b },
		"eq": func(a, b int) bool { return a == b },
		"ne": func(a, b int) bool { return a != b },
		"and": func(a, b bool) bool { return a && b },
		"or":  func(a, b bool) bool { return a || b },
	}

	// Parse and execute template
	tmpl, err := template.New("expr").Parse(expr)
	if err != nil {
		return false, fmt.Errorf("failed to parse expression: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, context); err != nil {
		return false, fmt.Errorf("failed to execute expression: %w", err)
	}

	// Parse result as boolean
	result := strings.TrimSpace(buf.String())
	return result == "true" || result == "1", nil
}
