package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/johnjansen/anvil/internal/project"
)

// GitRefState persists the last-seen refs per task to detect changes across daemon restarts.
type GitRefState struct {
	Refs     map[string]string `json:"refs"`      // branch ref path -> commit SHA
	LastPoll time.Time         `json:"last_poll"`
}

// gitSubscription tracks a single git-triggered task's polling configuration.
type gitSubscription struct {
	taskID       string
	todo         project.Todo
	proj         *project.Project
	branch       string        // branch filter (empty = all branches)
	pathGlob     string        // path filter glob pattern (empty = all paths)
	pollInterval time.Duration // polling frequency (default: 30s)
	events       []string      // event types to watch (default: ["push"])
	repoPath     string        // git repository root path
	cancel       context.CancelFunc
}

// GitWatcher manages git-triggered task subscriptions.
type GitWatcher struct {
	daemon        *Daemon
	mu            sync.Mutex
	subscriptions map[string]*gitSubscription // taskID -> subscription
}

// NewGitWatcher creates a new git watcher manager.
func NewGitWatcher(daemon *Daemon) *GitWatcher {
	return &GitWatcher{
		daemon:        daemon,
		subscriptions: make(map[string]*gitSubscription),
	}
}

// StartSubscription starts polling git refs for a task with git subscription.
func (gw *GitWatcher) StartSubscription(proj *project.Project, todo project.Todo) error {
	if todo.Subscription == nil || todo.Subscription.Type != "git" {
		return nil
	}

	// Validate required fields
	if len(todo.Subscription.GitEvents) == 0 {
		return fmt.Errorf("git subscription requires git_events")
	}
	for _, ev := range todo.Subscription.GitEvents {
		if ev != "push" {
			return fmt.Errorf("invalid git_events value %q: only \"push\" is supported", ev)
		}
	}

	// Parse poll interval (default 30s)
	pollInterval := 30 * time.Second
	if todo.Subscription.GitPollInterval != "" {
		d, err := time.ParseDuration(todo.Subscription.GitPollInterval)
		if err != nil {
			return fmt.Errorf("invalid git_poll_interval %q: %w", todo.Subscription.GitPollInterval, err)
		}
		if d < time.Second {
			return fmt.Errorf("git_poll_interval must be at least 1s, got %s", d)
		}
		pollInterval = d
	}

	// Resolve repo path (default to project path)
	repoPath := proj.Path

	// Verify git repo exists
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("no git repository found at %s", repoPath)
	}

	gw.mu.Lock()
	defer gw.mu.Unlock()

	if _, exists := gw.subscriptions[todo.ID]; exists {
		return nil // Already watching
	}

	ctx, cancel := context.WithCancel(context.Background())
	sub := &gitSubscription{
		taskID:       todo.ID,
		todo:         todo,
		proj:         proj,
		branch:       todo.Subscription.GitBranch,
		pathGlob:     todo.Subscription.GitPath,
		pollInterval: pollInterval,
		events:       todo.Subscription.GitEvents,
		repoPath:     repoPath,
		cancel:       cancel,
	}
	gw.subscriptions[todo.ID] = sub

	go gw.pollLoop(ctx, sub)

	log.Printf("Started git watcher for task %s (branch=%q, path=%q, interval=%s)", todo.Name, sub.branch, sub.pathGlob, sub.pollInterval)
	return nil
}

// pollLoop runs the periodic git ref polling for a single subscription.
func (gw *GitWatcher) pollLoop(ctx context.Context, sub *gitSubscription) {
	ticker := time.NewTicker(sub.pollInterval)
	defer ticker.Stop()

	// Do an initial poll to set baseline
	gw.pollOnce(ctx, sub)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			gw.pollOnce(ctx, sub)
		}
	}
}

// pollOnce checks for git ref changes and triggers the task if changes are detected.
func (gw *GitWatcher) pollOnce(ctx context.Context, sub *gitSubscription) {
	// Load persisted state
	state, err := gw.loadRefState(sub)
	if err != nil {
		log.Printf("Git watcher: failed to load ref state for task %s: %v", sub.todo.Name, err)
		state = &GitRefState{Refs: make(map[string]string)}
	}

	isBaseline := len(state.Refs) == 0

	// Get current refs
	currentRefs, err := gw.getCurrentRefs(sub)
	if err != nil {
		log.Printf("Git watcher: failed to get current refs for task %s: %v", sub.todo.Name, err)
		return
	}

	if isBaseline {
		// First run: record baseline, do NOT trigger
		state.Refs = currentRefs
		state.LastPoll = time.Now()
		if err := gw.saveRefState(sub, state); err != nil {
			log.Printf("Git watcher: failed to save baseline ref state for task %s: %v", sub.todo.Name, err)
		}
		log.Printf("Git watcher: recorded baseline refs for task %s", sub.todo.Name)
		return
	}

	// Compare refs for changes
	for branch, currentSHA := range currentRefs {
		prevSHA, existed := state.Refs[branch]
		if existed && prevSHA == currentSHA {
			continue // No change
		}

		// Ref changed (or new branch appeared)
		branchName := strings.TrimPrefix(branch, "refs/heads/")

		// Path filtering: check if any changed files match the glob
		if sub.pathGlob != "" && prevSHA != "" {
			changedFiles, err := gw.getChangedFiles(sub, prevSHA, currentSHA)
			if err != nil {
				log.Printf("Git watcher: failed to get changed files for task %s: %v", sub.todo.Name, err)
				// On error, skip path filtering and trigger anyway
			} else if !gw.matchesPathFilter(changedFiles, sub.pathGlob) {
				log.Printf("Git watcher: no changed files match path filter %q for task %s on branch %s", sub.pathGlob, sub.todo.Name, branchName)
				// Update ref state but don't trigger
				state.Refs[branch] = currentSHA
				continue
			}
		}

		// Dispatch the task
		gw.dispatch(sub, branchName, currentSHA, prevSHA)

		// Update ref state
		state.Refs[branch] = currentSHA
	}

	state.LastPoll = time.Now()
	if err := gw.saveRefState(sub, state); err != nil {
		log.Printf("Git watcher: failed to save ref state for task %s: %v", sub.todo.Name, err)
	}
}

// getCurrentRefs gets the current git refs for the subscription.
func (gw *GitWatcher) getCurrentRefs(sub *gitSubscription) (map[string]string, error) {
	refs := make(map[string]string)

	if sub.branch != "" {
		// Specific branch: just get that ref
		refPath := "refs/heads/" + sub.branch
		sha, err := gw.gitRevParse(sub.repoPath, refPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get ref for branch %s: %w", sub.branch, err)
		}
		refs[refPath] = sha
	} else {
		// All branches: use git for-each-ref
		output, err := gw.gitForEachRef(sub.repoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to list refs: %w", err)
		}
		for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
			if line == "" {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				refs[parts[1]] = parts[0]
			}
		}
	}

	return refs, nil
}

// gitRevParse runs git rev-parse for a specific ref.
func (gw *GitWatcher) gitRevParse(repoPath, ref string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", ref)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// gitForEachRef lists all branch refs with their SHAs.
func (gw *GitWatcher) gitForEachRef(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "for-each-ref", "--format=%(objectname) %(refname)", "refs/heads/")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git for-each-ref: %w", err)
	}
	return string(out), nil
}

// getChangedFiles returns the list of files changed between two commits.
func (gw *GitWatcher) getChangedFiles(sub *gitSubscription, oldSHA, newSHA string) ([]string, error) {
	cmd := exec.Command("git", "-C", sub.repoPath, "diff", "--name-only", oldSHA, newSHA)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only %s %s: %w", oldSHA, newSHA, err)
	}
	var files []string
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

// matchesPathFilter checks if any changed file matches the path glob pattern.
func (gw *GitWatcher) matchesPathFilter(files []string, pattern string) bool {
	for _, f := range files {
		matched, err := filepath.Match(pattern, f)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
		// Also try matching against just the filename for simple globs
		matched, err = filepath.Match(pattern, filepath.Base(f))
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// dispatch sends a triggered task to the daemon work queue with git context env vars.
func (gw *GitWatcher) dispatch(sub *gitSubscription, branch, commitSHA, prevSHA string) {
	env := make(map[string]string)
	for k, v := range sub.todo.Env {
		env[k] = v
	}

	env["ANVIL_GIT_EVENT"] = "push"
	env["ANVIL_GIT_BRANCH"] = branch
	env["ANVIL_GIT_COMMIT"] = commitSHA
	env["ANVIL_GIT_PREV_COMMIT"] = prevSHA
	absPath, err := filepath.Abs(sub.repoPath)
	if err != nil {
		absPath = sub.repoPath
	}
	env["ANVIL_GIT_REPO"] = absPath

	taskWithEnv := sub.todo
	taskWithEnv.Env = env

	// Check if daemon is shutting down
	select {
	case <-gw.daemon.stop:
		return
	default:
	}

	select {
	case gw.daemon.workQueue <- workItem{project: sub.proj, todo: taskWithEnv}:
		log.Printf("Enqueued task %s from git subscription (branch=%s, commit=%s)", taskWithEnv.Name, branch, commitSHA[:minLen(len(commitSHA), 8)])
	default:
		log.Printf("Work queue full, dropping task %s from git subscription", taskWithEnv.Name)
	}
}

func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// loadRefState loads the persisted ref state for a task.
func (gw *GitWatcher) loadRefState(sub *gitSubscription) (*GitRefState, error) {
	path := gw.statePath(sub)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &GitRefState{Refs: make(map[string]string)}, nil
		}
		return nil, err
	}
	var state GitRefState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	if state.Refs == nil {
		state.Refs = make(map[string]string)
	}
	return &state, nil
}

// saveRefState persists the ref state for a task.
func (gw *GitWatcher) saveRefState(sub *gitSubscription, state *GitRefState) error {
	path := gw.statePath(sub)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create git-state directory: %w", err)
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// statePath returns the path to the ref state file for a task.
func (gw *GitWatcher) statePath(sub *gitSubscription) string {
	return filepath.Join(sub.repoPath, ".anvil", "git-state", sub.taskID+".json")
}

// StopSubscription stops watching git refs for a specific task.
func (gw *GitWatcher) StopSubscription(taskID string) {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	sub, exists := gw.subscriptions[taskID]
	if !exists {
		return
	}
	sub.cancel()
	delete(gw.subscriptions, taskID)
	log.Printf("Stopped git watcher for task %s", sub.todo.Name)
}

// StopAll stops all git watcher subscriptions.
func (gw *GitWatcher) StopAll() {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	for taskID, sub := range gw.subscriptions {
		sub.cancel()
		delete(gw.subscriptions, taskID)
	}
	log.Printf("Stopped all git watchers")
}
