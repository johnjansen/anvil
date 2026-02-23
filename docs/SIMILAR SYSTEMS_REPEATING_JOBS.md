# Repeating Jobs Requirements

## Overview

Anvil must support scheduled, repeating tasks (jobs) that execute prompts on a cron schedule. This document defines the requirements based on the existing implementation.

---

## Data Model

### Job Definition

```go
type Job struct {
    // Identity
    Name        string `yaml:"name"`        // Unique identifier (filename without .md)
    
    // Scheduling
    Schedule    string `yaml:"schedule"`    // Cron expression (5 fields)
    Recurring   bool   `yaml:"recurring"`   // If false, one-shot (schedule removed after run)
    Timezone    string `yaml:"timezone"`    // IANA timezone or UTC offset
    
    // Execution
    Prompt      string `yaml:"prompt"`      // The prompt to execute
    Model       string `yaml:"model"`       // Optional: override model for this job
    
    // Notification
    Notify      NotifyPolicy `yaml:"notify"` // true, false, "error"
}
```

### Notify Policy

```go
type NotifyPolicy int

const (
    NotifyAlways NotifyPolicy = iota  // Send notification on every run
    NotifyNever                        // Never send notification
    NotifyOnError                      // Only send on non-zero exit
)
```

---

## Cron Expression Format

Standard 5-field cron syntax:

```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6, Sunday = 0)
│ │ │ │ │
* * * * *
```

### Supported Syntax

| Pattern | Meaning | Example |
|---------|---------|---------|
| `*` | Any value | `* * * * *` = every minute |
| `n` | Specific value | `0 * * * *` = at minute 0 |
| `n,m` | List | `0,30 * * * *` = at 0 and 30 |
| `n-m` | Range | `0-5 * * * *` = minutes 0-5 |
| `*/n` | Step | `*/15 * * * *` = every 15 minutes |
| Combined | Range + step | `0-30/5 * * * *` = every 5 min in first 30 min |

### Common Examples

| Expression | Meaning |
|------------|---------|
| `* * * * *` | Every minute |
| `0 * * * *` | Every hour |
| `*/15 * * * *` | Every 15 minutes |
| `0 9 * * *` | Daily at 9:00 AM |
| `0 9 * * 1-5` | Weekdays at 9:00 AM |
| `0 0 * * *` | Daily at midnight |
| `0 */6 * * *` | Every 6 hours |
| `0 9,18 * * *` | At 9 AM and 6 PM |

---

## File Format

Jobs are stored as YAML frontmatter + markdown body:

```markdown
---
schedule: "0 9 * * *"
recurring: true
notify: true
model: ""  # optional
---
Summarize yesterday's git commits and list any open issues.
```

### File Location

```
<project>/.claude/anvil/jobs/<job-name>.md
```

### Naming Rules

- Filename becomes job name (without `.md` extension)
- Allowed characters: `a-z`, `0-9`, `-`, `_`
- Must be unique within project
- Case-insensitive (normalized to lowercase)

---

## Timezone Handling

### Requirements

1. Jobs must be evaluated in the configured timezone
2. Timezone is set in `settings.json`, not per-job
3. Supports both IANA names (`America/New_York`) and UTC offsets (`UTC+5`)
4. DST transitions handled automatically

### Implementation

```go
// Convert UTC time to local time using offset
func shiftToTimezone(t time.Time, offsetMinutes int) time.Time {
    return t.Add(time.Duration(offsetMinutes) * time.Minute)
}

// Check if cron matches at given time
func cronMatches(expr string, t time.Time, offsetMinutes int) bool {
    local := shiftToTimezone(t, offsetMinutes)
    // Match against local time fields
}
```

---

## Execution Semantics

### One-Shot vs Recurring

| Type | `recurring` | Behavior |
|------|-------------|----------|
| Recurring | `true` | Runs on schedule indefinitely |
| One-shot | `false` or omitted | Runs once, then schedule is removed |

### One-Shot Behavior

After a one-shot job executes:

1. The schedule field is removed from the frontmatter
2. The job file remains (with prompt intact)
3. The job will not run again unless rescheduled

```go
func clearSchedule(jobName string) error {
    // Read job file
    // Remove "schedule:" line from frontmatter
    // Write back to disk
}
```

### Model Override

Jobs can optionally specify a different model:

```markdown
---
schedule: "0 9 * * *"
model: "gpt-4o-mini"
---
Quick daily summary.
```

---

## Notification System

### Notify Field Values

| Value | Behavior |
|-------|----------|
| `true` | Always send notification |
| `false` | Never send notification |
| `"error"` | Send only if exit code != 0 |

### Implementation

```go
func shouldNotify(job *Job, result *Result) bool {
    switch job.Notify {
    case NotifyNever:
        return false
    case NotifyOnError:
        return result.ExitCode != 0
    default:
        return true
    }
}
```

---

## Hot Reload

### Requirements

1. Job files are reloaded every 30 seconds
2. No daemon restart required for:
   - Adding new jobs
   - Modifying existing jobs
   - Deleting jobs
3. Changes are detected by scanning the jobs directory

### Implementation

```go
func (s *Scheduler) reloadJobs() {
    files, _ := os.ReadDir(s.jobsDir)
    for _, f := range files {
        if !strings.HasSuffix(f.Name(), ".md") {
            continue
        }
        job := parseJobFile(f.Name())
        s.jobs[job.Name] = job
    }
}

// Run every 30 seconds
ticker := time.NewTicker(30 * time.Second)
for range ticker.C {
    s.reloadJobs()
}
```

---

## Scheduling Algorithm

### Tick Interval

- Check for due jobs every **60 seconds**
- Align tick to the start of each minute (seconds = 0)

### Matching Logic

```go
func (s *Scheduler) tick() {
    now := time.Now().Truncate(time.Minute)
    offset := s.settings.TimezoneOffsetMinutes
    
    for _, job := range s.jobs {
        if cronMatches(job.Schedule, now, offset) {
            s.queue.Push(job)
        }
    }
}
```

### Multiple Jobs at Same Time

When multiple jobs match the same tick:

1. Add all to queue
2. Queue orders by priority (if specified)
3. Execute serially (one at a time)

---

## Error Handling

### Execution Failure

```go
type Result struct {
    ExitCode int
    Stdout   string
    Stderr   string
    Error    error
}
```

### Retry Behavior

- **Recurring jobs**: Failed runs do NOT automatically retry
- **One-shot jobs**: Failed runs do NOT automatically retry
- User can manually re-run via API

### Dead Letter Queue

Failed jobs are logged but NOT moved to DLQ (unlike scheduled tasks).
Jobs are user-defined and should be fixed by the user.

---

## API Requirements

### List Jobs

```
GET /jobs
```

Response:
```json
{
  "jobs": [
    {
      "name": "daily-summary",
      "schedule": "0 9 * * *",
      "recurring": true,
      "next_run": "2026-02-23T09:00:00Z"
    }
  ]
}
```

### Create Job

```
POST /jobs
{
  "name": "daily-summary",
  "schedule": "0 9 * * *",
  "recurring": true,
  "prompt": "Summarize yesterday's commits",
  "notify": true
}
```

### Run Job Immediately

```
POST /jobs/{name}/run
```

Triggers job execution regardless of schedule.

### Delete Job

```
DELETE /jobs/{name}
```

---

## Backward Compatibility

### Legacy Field: `daily`

The `daily` field is deprecated but still accepted:

```markdown
---
schedule: "0 9 * * *"
daily: true
---
```

Maps to `recurring: true`.

### Migration

```go
func parseRecurring(frontmatter map[string]any) bool {
    if v, ok := frontmatter["recurring"]; ok {
        return parseBool(v)
    }
    if v, ok := frontmatter["daily"]; ok {
        return parseBool(v) // legacy support
    }
    return false
}
```

---

## State Tracking

### Job State (In-Memory)

```go
type JobState struct {
    Name        string
    LastRun     *time.Time
    NextRun     *time.Time
    LastResult  *Result
    Runs        int  // Total execution count
    Failures    int  // Failed execution count
}
```

### Persistence

- State is NOT persisted to disk
- State resets on daemon restart
- Next run is calculated from schedule on startup

---

## Summary

| Feature | Requirement |
|---------|-------------|
| Cron format | 5-field standard |
| Timezone | Configurable per-project |
| Recurring | `recurring: true/false` |
| One-shot | `recurring: false` removes schedule after run |
| Notify | `true/false/"error"` |
| Model override | Optional `model` field |
| Hot reload | Every 30 seconds |
| Tick interval | Every 60 seconds |
| Execution | Serial, one at a time |
| File format | YAML frontmatter + markdown body |