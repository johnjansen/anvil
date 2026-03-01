package project

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"
)

// RollbackEvent records when a rollback occurred
type RollbackEvent struct {
	EventID     string    `json:"event_id"`
	TaskID      string    `json:"task_id"`
	RunID       string    `json:"run_id"`
	Timestamp   time.Time `json:"timestamp"`
	User        string    `json:"user"`
	Description string    `json:"description"`
}

// ListRestorePoints returns all successful run records for a task, sorted by timestamp (newest first)
func ListRestorePoints(projectPath, taskID string) ([]RunRecord, error) {
	records, err := ReadAllRunRecords(projectPath, taskID)
	if err != nil {
		return nil, err
	}

	// Filter for successful runs only
	var successful []RunRecord
	for _, rec := range records {
		if rec.Success {
			successful = append(successful, rec)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(successful, func(i, j int) bool {
		return successful[i].Started.After(successful[j].Started)
	})

	return successful, nil
}

// GetRunRecord loads a specific run record by ID
func GetRunRecord(projectPath, taskID, runID string) (RunRecord, error) {
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

// RestoreFiles restores files from a specific run to the current working directory
func RestoreFiles(projectPath, taskID, runID string, files []string, dryRun bool) ([]string, error) {
	rec, err := GetRunRecord(projectPath, taskID, runID)
	if err != nil {
		return nil, fmt.Errorf("getting run record: %w", err)
	}

	if !rec.Success {
		return nil, fmt.Errorf("cannot restore from failed run %s", runID)
	}

	// Get the run directory
	runDir := filepath.Join(projectPath, ".anvil", "runs", taskID, runID)

	// Check if run directory exists
	if _, err := os.Stat(runDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("run output directory not found: %s", runDir)
	}

	// Get list of files to restore
	var filesToRestore []string
	if len(files) == 0 {
		// Restore all files in the run directory
		entries, err := os.ReadDir(runDir)
		if err != nil {
			return nil, fmt.Errorf("reading run directory: %w", err)
		}

		for _, entry := range entries {
			// Skip the JSON metadata file and "current" file
			if entry.Name() == fmt.Sprintf("%s.json", runID) || entry.Name() == "current" {
				continue
			}
			filesToRestore = append(filesToRestore, entry.Name())
		}
	} else {
		// Restore only specified files
		filesToRestore = files
	}

	// Validate that all specified files exist in the restore point
	if len(files) > 0 {
		for _, file := range filesToRestore {
			filePath := filepath.Join(runDir, file)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				return nil, fmt.Errorf("file %s not found in restore point", file)
			}
		}
	}

	// If this is a dry run, just return the list of files that would be restored
	if dryRun {
		return filesToRestore, nil
	}

	// Actually restore the files
	restoredFiles := []string{}
	for _, file := range filesToRestore {
		srcPath := filepath.Join(runDir, file)
		dstPath := filepath.Join(projectPath, file)

		// Copy file from run directory to project directory
		if err := copyFile(srcPath, dstPath); err != nil {
			return restoredFiles, fmt.Errorf("copying file %s: %w", file, err)
		}
		restoredFiles = append(restoredFiles, file)
	}

	// Record the rollback event
	if err := recordRollbackEvent(projectPath, taskID, runID, restoredFiles); err != nil {
		// Log error but don't fail the restore
		fmt.Fprintf(os.Stderr, "Warning: failed to record rollback event: %v\n", err)
	}

	return restoredFiles, nil
}

// RunRollbackHook executes the on_rollback hook if configured
func RunRollbackHook(todo Todo, runID string) error {
	if todo.OnRollback == "" {
		return nil // No hook configured
	}

	// Parse the template
	tmpl, err := template.New("rollback_hook").Parse(todo.OnRollback)
	if err != nil {
		return fmt.Errorf("parsing rollback hook template: %w", err)
	}

	// Prepare template data
	data := struct {
		RunID    string
		TaskName string
		TaskID   string
	}{
		RunID:    runID,
		TaskName: strings.TrimSuffix(todo.Name, ".md"),
		TaskID:   todo.ID,
	}

	// Execute template to get command
	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("executing rollback hook template: %w", err)
	}

	command := buf.String()

	// Execute the command
	// Note: This is a simplified implementation - in practice, you'd want to use proper command execution
	// with timeout, environment variables, etc.
	fmt.Fprintf(os.Stderr, "Executing rollback hook: %s\n", command)
	// Actual implementation would execute the command here

	return nil
}

// Helper function to copy a file
func copyFile(src, dst string) error {
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy file contents
	_, err = io.Copy(dstFile, srcFile)
	return err
}

// recordRollbackEvent records a rollback event to the rollback log
func recordRollbackEvent(projectPath, taskID, runID string, restoredFiles []string) error {
	event := RollbackEvent{
		EventID:     generateID(),
		TaskID:      taskID,
		RunID:       runID,
		Timestamp:   time.Now(),
		User:        getCurrentUser(),
		Description: fmt.Sprintf("Restored files: %s", strings.Join(restoredFiles, ", ")),
	}

	// Create rollback events directory
	eventsDir := filepath.Join(projectPath, ".anvil", "runs", taskID, "rollbacks")
	if err := os.MkdirAll(eventsDir, 0755); err != nil {
		return fmt.Errorf("creating rollback events directory: %w", err)
	}

	// Write event to file
	eventPath := filepath.Join(eventsDir, fmt.Sprintf("%s.json", event.EventID))
	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling rollback event: %w", err)
	}

	return os.WriteFile(eventPath, data, 0644)
}

// Helper function to get current user
func getCurrentUser() string {
	user, err := user.Current()
	if err != nil {
		return "unknown"
	}
	return user.Username
}

// Helper function to generate a unique ID
func generateID() string {
	// This is a simplified implementation - you might want to use a proper UUID generator
	return fmt.Sprintf("%d", time.Now().UnixNano())
}