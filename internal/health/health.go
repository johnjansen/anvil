package health

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/johnjansen/anvil/internal/project"
)

// HealthStatus represents the health status of a task
type HealthStatus struct {
	TaskName   string    `json:"task_name"`
	Healthy    bool      `json:"healthy"`
	LastCheck  time.Time `json:"last_check"`
	ExitCode   int       `json:"exit_code"`
	Error      string    `json:"error,omitempty"`
}

// Manager manages task health checks
type Manager struct {
	healthCache map[string]*HealthStatus
	cacheMutex  sync.RWMutex
}

// NewManager creates a new health manager
func NewManager() *Manager {
	return &Manager{
		healthCache: make(map[string]*HealthStatus),
	}
}

// RunHealthCheck executes a health check for a task
func (hm *Manager) RunHealthCheck(ctx context.Context, proj *project.Project, task project.Todo) *HealthStatus {
	status := &HealthStatus{
		TaskName:  task.Name,
		LastCheck: time.Now(),
	}

	// If no health check command is configured, consider it healthy
	if task.HealthCheck == "" {
		status.Healthy = true
		status.ExitCode = 0
		hm.cacheHealthStatus(task.Name, status)
		return status
	}

	// Execute the health check command
	cmd := exec.CommandContext(ctx, "sh", "-c", task.HealthCheck)
	cmd.Dir = proj.Path

	// Set environment variables from task config
	for k, v := range task.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Inherit parent environment
	cmd.Env = append(cmd.Env, os.Environ()...)

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			status.ExitCode = exitErr.ExitCode()
			status.Healthy = status.ExitCode == 0
		} else {
			status.Error = err.Error()
			status.Healthy = false
		}
	} else {
		status.ExitCode = 0
		status.Healthy = true
	}

	hm.cacheHealthStatus(task.Name, status)
	return status
}

// cacheHealthStatus stores the health status in the cache
func (hm *Manager) cacheHealthStatus(taskName string, status *HealthStatus) {
	hm.cacheMutex.Lock()
	defer hm.cacheMutex.Unlock()
	hm.healthCache[taskName] = status
}

// GetHealthStatus retrieves the cached health status for a task
func (hm *Manager) GetHealthStatus(taskName string) *HealthStatus {
	hm.cacheMutex.RLock()
	defer hm.cacheMutex.RUnlock()
	if status, exists := hm.healthCache[taskName]; exists {
		return status
	}
	return nil
}

// GetAllHealthStatus returns all cached health statuses
func (hm *Manager) GetAllHealthStatus() map[string]*HealthStatus {
	hm.cacheMutex.RLock()
	defer hm.cacheMutex.RUnlock()

	// Return a copy of the map to avoid race conditions
	result := make(map[string]*HealthStatus)
	for k, v := range hm.healthCache {
		result[k] = v
	}
	return result
}

// RunAllHealthChecks runs health checks for all tasks with health check configurations
func (hm *Manager) RunAllHealthChecks(ctx context.Context, watchedPaths []string) {
	for _, projectPath := range watchedPaths {
		proj, err := project.Load(projectPath)
		if err != nil {
			continue
		}

		todos, err := proj.LoadTodos()
		if err != nil {
			continue
		}

		for _, task := range todos {
			// Only run health checks for tasks that have a health check configured
			if task.HealthCheck != "" {
				hm.RunHealthCheck(ctx, proj, task)
			}
		}
	}
}