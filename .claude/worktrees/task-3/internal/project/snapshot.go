package project

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

// Snapshot represents a collection of files capturing the complete execution context for a single task run.
type Snapshot struct {
	TaskID    string    `json:"task_id"`
	RunID     string    `json:"run_id"`
	CreatedAt time.Time `json:"created_at"`
}

// SnapshotFile represents an individual file within a snapshot directory.
type SnapshotFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// GetSnapshotPath returns the path to the snapshot directory for a given task and run.
func GetSnapshotPath(projectRoot, taskID, runID string) string {
	return filepath.Join(projectRoot, ".anvil", "runs", taskID, runID, "snapshot")
}

// WriteSnapshot creates a snapshot directory and writes all snapshot files.
func WriteSnapshot(projectRoot, taskID, runID string, config interface{}, envVars map[string]string, prompt, workDir string, runRecord interface{}) error {
	snapshotPath := GetSnapshotPath(projectRoot, taskID, runID)

	// Create the snapshot directory
	if err := os.MkdirAll(snapshotPath, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// Write config.yaml
	configData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(snapshotPath, "config.yaml"), configData, 0644); err != nil {
		return fmt.Errorf("failed to write config.yaml: %w", err)
	}

	// Write env.yaml
	envData, err := yaml.Marshal(envVars)
	if err != nil {
		return fmt.Errorf("failed to marshal env vars: %w", err)
	}
	if err := os.WriteFile(filepath.Join(snapshotPath, "env.yaml"), envData, 0644); err != nil {
		return fmt.Errorf("failed to write env.yaml: %w", err)
	}

	// Write prompt.txt
	if err := os.WriteFile(filepath.Join(snapshotPath, "prompt.txt"), []byte(prompt), 0644); err != nil {
		return fmt.Errorf("failed to write prompt.txt: %w", err)
	}

	// Write files.json (directory listing)
	fileListing, err := generateFileListing(workDir)
	if err != nil {
		return fmt.Errorf("failed to generate file listing: %w", err)
	}
	fileListingData, err := json.MarshalIndent(fileListing, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal file listing: %w", err)
	}
	if err := os.WriteFile(filepath.Join(snapshotPath, "files.json"), fileListingData, 0644); err != nil {
		return fmt.Errorf("failed to write files.json: %w", err)
	}

	// Write run_record.json
	runRecordData, err := json.MarshalIndent(runRecord, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal run record: %w", err)
	}
	if err := os.WriteFile(filepath.Join(snapshotPath, "run_record.json"), runRecordData, 0644); err != nil {
		return fmt.Errorf("failed to write run_record.json: %w", err)
	}

	return nil
}

// generateFileListing creates a directory listing for the given path.
func generateFileListing(dirPath string) ([]map[string]interface{}, error) {
	var files []map[string]interface{}

	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if path == dirPath {
			return nil
		}

		// Get relative path from the working directory
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		fileInfo := map[string]interface{}{
			"name":    d.Name(),
			"path":    relPath,
			"size":    info.Size(),
			"mode":    info.Mode().String(),
			"modTime": info.ModTime(),
			"isDir":   d.IsDir(),
		}

		files = append(files, fileInfo)
		return nil
	})

	// Sort files by path for consistent output
	sort.Slice(files, func(i, j int) bool {
		return files[i]["path"].(string) < files[j]["path"].(string)
	})

	return files, err
}

// ReadSnapshot reads a snapshot from the snapshot directory.
func ReadSnapshot(projectRoot, taskID, runID string) (*Snapshot, error) {
	snapshotPath := GetSnapshotPath(projectRoot, taskID, runID)

	// Check if snapshot exists
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("snapshot does not exist: %w", err)
	}

	snapshot := &Snapshot{
		TaskID:    taskID,
		RunID:     runID,
		CreatedAt: time.Now(),
	}

	return snapshot, nil
}

// ReadSnapshotFile reads a specific file from a snapshot.
func ReadSnapshotFile(projectRoot, taskID, runID, filename string) ([]byte, error) {
	snapshotPath := GetSnapshotPath(projectRoot, taskID, runID)
	filePath := filepath.Join(snapshotPath, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("snapshot file does not exist: %s", filename)
	}

	return os.ReadFile(filePath)
}