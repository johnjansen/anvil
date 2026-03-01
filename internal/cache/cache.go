package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/project"
)

// CacheEntry represents a cached task output
type CacheEntry struct {
	Key       string    `json:"key"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CalculateCacheKey calculates a cache key based on the task configuration and templates
func CalculateCacheKey(task project.Todo, projectPath string) (string, error) {
	if task.Cache == nil || !task.Cache.Enabled || task.Cache.Key == "" {
		// Default to hashing the task content
		hash := sha256.Sum256([]byte(task.Content))
		return fmt.Sprintf("%x", hash[:]), nil
	}

	key := task.Cache.Key

	// Replace template variables
	if strings.Contains(key, "{{ .GitSHA }}") {
		gitSHA, err := getGitSHA(projectPath)
		if err != nil {
			return "", fmt.Errorf("failed to get git SHA: %w", err)
		}
		key = strings.ReplaceAll(key, "{{ .GitSHA }}", gitSHA)
	}

	if strings.Contains(key, "{{ .FileHash:") {
		// Extract file pattern from {{ .FileHash:pattern }}
		start := strings.Index(key, "{{ .FileHash:")
		end := strings.Index(key[start:], "}}")
		if start >= 0 && end > start {
			pattern := key[start+13 : start+end] // 13 is length of "{{ .FileHash:"
			fileHash, err := getFileHash(projectPath, pattern)
			if err != nil {
				return "", fmt.Errorf("failed to get file hash for pattern %s: %w", pattern, err)
			}
			placeholder := key[start : start+end+2] // Include the closing }}
			key = strings.ReplaceAll(key, placeholder, fileHash)
		}
	}

	if strings.Contains(key, "{{ .Environment:") {
		// Extract environment variable name from {{ .Environment:name }}
		start := strings.Index(key, "{{ .Environment:")
		end := strings.Index(key[start:], "}}")
		if start >= 0 && end > start {
			envName := key[start+17 : start+end] // 17 is length of "{{ .Environment:"
			envValue := os.Getenv(envName)
			placeholder := key[start : start+end+2] // Include the closing }}
			key = strings.ReplaceAll(key, placeholder, envValue)
		}
	}

	// Hash the final key to ensure consistent length
	hash := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", hash[:]), nil
}

// getGitSHA gets the current git commit SHA
func getGitSHA(projectPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = projectPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getFileHash calculates a hash of files matching the given pattern
func getFileHash(projectPath, pattern string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(projectPath, pattern))
	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no files matched pattern: %s", pattern)
	}

	h := sha256.New()
	for _, match := range matches {
		content, err := os.ReadFile(match)
		if err != nil {
			return "", err
		}
		h.Write(content)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// GetCacheDir returns the cache directory path
func GetCacheDir(projectPath string) string {
	return filepath.Join(projectPath, ".anvil", "cache")
}

// GetCacheFilePath returns the path to a cache file for a given key
func GetCacheFilePath(projectPath, key string) string {
	return filepath.Join(GetCacheDir(projectPath), key+".json")
}

// GetCache retrieves a cached entry for the given key
func GetCache(projectPath, key string) (*CacheEntry, error) {
	cachePath := GetCacheFilePath(projectPath, key)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Cache miss
		}
		return nil, err
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}

	// Check if cache entry is expired
	if time.Now().After(entry.ExpiresAt) {
		// Delete expired cache entry
		os.Remove(cachePath)
		return nil, nil // Cache miss
	}

	return &entry, nil
}

// PutCache stores a cache entry
func PutCache(projectPath, key, content string, ttl time.Duration) error {
	cacheDir := GetCacheDir(projectPath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	entry := CacheEntry{
		Key:       key,
		Content:   content,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	cachePath := GetCacheFilePath(projectPath, key)
	return os.WriteFile(cachePath, data, 0644)
}

// IsCacheEnabled checks if caching is enabled for a task
func IsCacheEnabled(task project.Todo) bool {
	return task.Cache != nil && task.Cache.Enabled
}

// GetCacheTTL returns the cache TTL for a task
func GetCacheTTL(task project.Todo) (time.Duration, error) {
	if task.Cache == nil || task.Cache.TTL == "" {
		return 1 * time.Hour, nil // Default TTL
	}
	return time.ParseDuration(task.Cache.TTL)
}