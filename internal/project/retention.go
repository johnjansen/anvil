package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PruneResult holds the outcome of a pruning operation.
type PruneResult struct {
	LogsDeleted int
	RunsDeleted int
	Errors      []error
}

// PruneOptions controls what gets pruned.
type PruneOptions struct {
	MaxAge  time.Duration // delete files older than this (0 = no age limit)
	MaxRuns int           // keep at most this many runs per task (0 = unlimited)
	DryRun  bool          // if true, report what would be deleted without deleting
	Now     time.Time     // reference time for age calculations
}

// PruneProject removes old log and run files for all tasks in a project.
func PruneProject(projectPath string, opts PruneOptions) PruneResult {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}

	var result PruneResult

	// Find all task IDs by scanning the runs directory
	logsBase := filepath.Join(projectPath, ".anvil", "logs")
	runsBase := filepath.Join(projectPath, ".anvil", "runs")

	taskIDs := discoverTaskIDs(logsBase, runsBase)

	for _, taskID := range taskIDs {
		r := pruneTask(logsBase, runsBase, taskID, opts)
		result.LogsDeleted += r.LogsDeleted
		result.RunsDeleted += r.RunsDeleted
		result.Errors = append(result.Errors, r.Errors...)
	}

	return result
}

// discoverTaskIDs finds all unique task ID subdirectories across logs and runs.
func discoverTaskIDs(logsBase, runsBase string) []string {
	seen := make(map[string]bool)

	for _, base := range []string{logsBase, runsBase} {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				seen[e.Name()] = true
			}
		}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

type fileEntry struct {
	path    string
	modTime time.Time
	name    string
}

// pruneTask prunes logs and runs for a single task.
func pruneTask(logsBase, runsBase, taskID string, opts PruneOptions) PruneResult {
	var result PruneResult

	logDir := filepath.Join(logsBase, taskID)
	runDir := filepath.Join(runsBase, taskID)

	// Prune runs (and corresponding logs)
	runFiles := listDataFiles(runDir, ".json")
	logFiles := listDataFiles(logDir, ".log")

	// Build a map of log files by run ID for easy lookup
	logByRunID := make(map[string]fileEntry)
	for _, lf := range logFiles {
		runID := strings.TrimSuffix(lf.name, ".log")
		logByRunID[runID] = lf
	}

	// Sort runs by mod time, newest first
	sort.Slice(runFiles, func(i, j int) bool {
		return runFiles[i].modTime.After(runFiles[j].modTime)
	})

	for i, rf := range runFiles {
		runID := strings.TrimSuffix(rf.name, ".json")
		shouldPrune := false

		// Check max_runs limit
		if opts.MaxRuns > 0 && i >= opts.MaxRuns {
			shouldPrune = true
		}

		// Check max_age limit
		if opts.MaxAge > 0 && opts.Now.Sub(rf.modTime) > opts.MaxAge {
			shouldPrune = true
		}

		if !shouldPrune {
			continue
		}

		// Delete run record
		if !opts.DryRun {
			if err := os.Remove(rf.path); err != nil && !os.IsNotExist(err) {
				result.Errors = append(result.Errors, fmt.Errorf("removing run %s: %w", rf.path, err))
			} else {
				result.RunsDeleted++
			}
		} else {
			result.RunsDeleted++
		}

		// Delete corresponding log file
		if lf, ok := logByRunID[runID]; ok {
			if !opts.DryRun {
				if err := os.Remove(lf.path); err != nil && !os.IsNotExist(err) {
					result.Errors = append(result.Errors, fmt.Errorf("removing log %s: %w", lf.path, err))
				} else {
					result.LogsDeleted++
				}
			} else {
				result.LogsDeleted++
			}
			delete(logByRunID, runID)
		}
	}

	// Also prune orphan log files (logs without a corresponding run record)
	for _, lf := range logByRunID {
		shouldPrune := false

		if opts.MaxAge > 0 && opts.Now.Sub(lf.modTime) > opts.MaxAge {
			shouldPrune = true
		}

		if !shouldPrune {
			continue
		}

		if !opts.DryRun {
			if err := os.Remove(lf.path); err != nil && !os.IsNotExist(err) {
				result.Errors = append(result.Errors, fmt.Errorf("removing orphan log %s: %w", lf.path, err))
			} else {
				result.LogsDeleted++
			}
		} else {
			result.LogsDeleted++
		}
	}

	return result
}

// listDataFiles lists files in a directory matching the given suffix, excluding special files.
func listDataFiles(dir, suffix string) []fileEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var files []fileEntry
	for _, e := range entries {
		if e.IsDir() || e.Name() == "current" || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileEntry{
			path:    filepath.Join(dir, e.Name()),
			modTime: info.ModTime(),
			name:    e.Name(),
		})
	}
	return files
}
