package daemon

import (
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/cron"
	"github.com/johnjansen/anvil/internal/project"
)

// slaResult holds the outcome of an SLA check for a dispatched task.
type slaResult struct {
	HasSLA        bool          // whether SLA tracking is configured for this task
	Violation     bool          // whether the dispatch delay exceeds max_delay
	Strict        bool          // whether strict mode is enabled (skip instead of run)
	ScheduledTime time.Time     // when the task was supposed to run
	Delay         time.Duration // actual delay from scheduled time
	MaxDelay      time.Duration // configured max_delay threshold
}

// getEffectiveSLA returns the effective max_delay and strict flag for a task.
// Per-task SLA overrides global default. Returns zero duration if no SLA configured.
func getEffectiveSLA(todo project.Todo, globalCfg config.SLAGlobalConfig) (maxDelay time.Duration, strict bool) {
	if todo.SLA.MaxDelay > 0 {
		return todo.SLA.MaxDelay, todo.SLA.Strict
	}
	if globalCfg.DefaultMaxDelay != "" {
		if d, err := time.ParseDuration(globalCfg.DefaultMaxDelay); err == nil && d > 0 {
			return d, false // global default never implies strict
		}
	}
	return 0, false
}

// checkSLA evaluates whether a task's dispatch is within its SLA threshold.
// Returns an slaResult with violation details. Only applies to cron-scheduled tasks.
func checkSLA(todo project.Todo, globalCfg config.SLAGlobalConfig, now time.Time) slaResult {
	maxDelay, strict := getEffectiveSLA(todo, globalCfg)
	if maxDelay <= 0 {
		return slaResult{} // no SLA configured
	}

	// SLA only applies to cron-scheduled tasks
	if todo.Schedule == "" || todo.IsPersistent() {
		return slaResult{}
	}

	p, err := cron.Parse(todo.Schedule)
	if err != nil {
		return slaResult{}
	}

	scheduledTime, err := p.Prev(now)
	if err != nil {
		return slaResult{}
	}

	delay := now.Sub(scheduledTime)

	return slaResult{
		HasSLA:        true,
		Violation:     delay > maxDelay,
		Strict:        strict,
		ScheduledTime: scheduledTime,
		Delay:         delay,
		MaxDelay:      maxDelay,
	}
}
