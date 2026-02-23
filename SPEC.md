# Anvil Specification

## Overview

Anvil is a single-binary task scheduler and executor. It replaces multi-daemon architectures with a unified queue-based system.

## Core Principles

1. **Single source of truth** - The queue holds all state
2. **Serial execution** - One task at a time, no race conditions
3. **Atomic state transitions** - No partial updates
4. **Observable** - Every state change is logged and queryable
5. **Generic** - Executor is pluggable, not tied to any specific command

---

## Data Structures

### Task

```go
type Task struct {
    ID          string       // UUID
    Name        string       // Human readable
    Command     string       // Shell command to execute
    Schedule    string       // Cron expression (empty = one-shot)
    Priority    int          // Higher = runs first (default 0)
    
    Status      TaskStatus   // pending, running, completed, failed, dead_letter
    CreatedAt   time.Time
    ScheduledAt time.Time    // When it should run
    StartedAt   *time.Time
    CompletedAt *time.Time
    
    Attempts    int          // How many times we've tried
    MaxRetries  int          // Max attempts before dead letter
    LastError   string       // Error from last attempt
    
    Metadata    map[string]string // Arbitrary key-values
}
```

### TaskStatus

```go
type TaskStatus int

const (
    StatusPending TaskStatus = iota
    StatusRunning
    StatusCompleted
    StatusFailed
    StatusDeadLetter
)
```

### Queue

```go
type Queue struct {
    mu       sync.Mutex
    pending  []*Task  // Ordered by (priority DESC, scheduled_at ASC)
    running  *Task    // Currently executing (max 1)
    completed []*Task // Recent completions (capped at 100)
    failed    []*Task // Recent failures (capped at 100)
    deadLetter []*Task // Permanent failures (capped at 100)
}
```

---

## State Transitions

```
                    ┌──────────────┐
                    │   PENDING    │
                    └──────┬───────┘
                           │ pop()
                           ▼
                    ┌──────────────┐
                    │   RUNNING    │
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
              ▼            ▼            ▼
       ┌──────────┐ ┌──────────┐ ┌──────────┐
       │ COMPLETED│ │  FAILED  │ │ CANCELLED│
       └──────────┘ └────┬─────┘ └──────────┘
                         │
                    retry?│
                         ▼
                    ┌──────────┐
                    │  PENDING │ (if attempts < max_retries)
                    └──────────┘
                         │
               no retries│
                         ▼
                    ┌──────────┐
                    │DEAD_LETTER│
                    └──────────┘
```

---

## Components

### 1. Scheduler

**Responsibility**: Check for due tasks and push them to the queue.

```go
func (s *Scheduler) Run() {
    ticker := time.NewTicker(s.tickInterval)
    for range ticker.C {
        s.checkDueTasks()
    }
}

func (s *Scheduler) checkDueTasks() {
    now := time.Now()
    for _, task := range s.tasks {
        if cronMatches(task.Schedule, now) {
            s.queue.Push(task.Clone())
        }
    }
}
```

**Thread safety**: Scheduler writes to queue via `Push()` which is mutex-protected.

### 2. Queue

**Responsibility**: Ordered storage with atomic operations.

```go
func (q *Queue) Push(task *Task) {
    q.mu.Lock()
    defer q.mu.Unlock()
    
    task.Status = StatusPending
    task.ID = uuid.New()
    task.CreatedAt = time.Now()
    
    // Insert in priority order
    q.pending = append(q.pending, task)
    sort.Slice(q.pending, func(i, j int) bool {
        if q.pending[i].Priority != q.pending[j].Priority {
            return q.pending[i].Priority > q.pending[j].Priority
        }
        return q.pending[i].ScheduledAt.Before(q.pending[j].ScheduledAt)
    })
}

func (q *Queue) Pop() *Task {
    q.mu.Lock()
    defer q.mu.Unlock()
    
    if len(q.pending) == 0 {
        return nil
    }
    
    task := q.pending[0]
    q.pending = q.pending[1:]
    q.running = task
    task.Status = StatusRunning
    task.StartedAt = ptr(time.Now())
    
    return task
}

func (q *Queue) Complete(task *Task, err error) {
    q.mu.Lock()
    defer q.mu.Unlock()
    
    q.running = nil
    
    if err != nil {
        task.Attempts++
        task.LastError = err.Error()
        
        if task.Attempts < task.MaxRetries {
            task.Status = StatusPending
            task.ScheduledAt = time.Now().Add(retryDelay(task.Attempts))
            q.pending = append(q.pending, task)
            sortQueue(q.pending)
        } else {
            task.Status = StatusDeadLetter
            task.CompletedAt = ptr(time.Now())
            q.deadLetter = append(q.deadLetter, task)
            capSlice(&q.deadLetter, 100)
        }
    } else {
        task.Status = StatusCompleted
        task.CompletedAt = ptr(time.Now())
        q.completed = append(q.completed, task)
        capSlice(&q.completed, 100)
    }
}
```

### 3. Executor

**Responsibility**: Run shell commands, capture output.

```go
func (e *Executor) Run(ctx context.Context, task *Task) error {
    ctx, cancel := context.WithTimeout(ctx, e.timeout)
    defer cancel()
    
    cmd := exec.CommandContext(ctx, "sh", "-c", task.Command)
    
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("exit %d: %s", cmd.ProcessState.ExitCode(), stderr.String())
    }
    
    task.Metadata["stdout"] = stdout.String()
    return nil
}
```

### 4. Worker

**Responsibility**: Pop from queue and execute.

```go
func (w *Worker) Run() {
    for {
        task := w.queue.Pop()
        if task == nil {
            time.Sleep(w.pollInterval)
            continue
        }
        
        err := w.executor.Run(context.Background(), task)
        w.queue.Complete(task, err)
        
        if err != nil {
            w.logger.Printf("task %s failed: %v", task.ID, err)
        }
    }
}
```

---

## Cron Parser

Simple cron expression parser supporting standard 5-field format:

```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday = 0)
│ │ │ │ │
* * * * *
```

Supports:
- `*` (any)
- `*/n` (every n)
- `n` (specific)
- `n,m` (list)
- `n-m` (range)

---

## HTTP API

### GET /status

```json
{
  "uptime": "2h30m",
  "queue": {
    "pending": 3,
    "running": 1,
    "completed": 127,
    "failed": 2,
    "dead_letter": 1
  },
  "last_tick": "2026-02-22T12:34:56Z",
  "last_task": "heartbeat-123"
}
```

### GET /queue

```json
{
  "pending": [
    {
      "id": "abc-123",
      "name": "heartbeat",
      "status": "pending",
      "scheduled_at": "2026-02-22T12:35:00Z",
      "priority": 0
    }
  ],
  "running": null,
  "completed": [...],
  "dead_letter": [...]
}
```

### POST /tasks

```json
{
  "name": "one-off-task",
  "command": "echo hello",
  "priority": 10
}
```

### GET /tasks/{id}

```json
{
  "id": "abc-123",
  "name": "heartbeat",
  "command": "claude -p 'check status'",
  "status": "completed",
  "created_at": "2026-02-22T12:00:00Z",
  "started_at": "2026-02-22T12:35:00Z",
  "completed_at": "2026-02-22T12:35:05Z",
  "attempts": 1,
  "metadata": {
    "stdout": "All systems nominal"
  }
}
```

### POST /tasks/{id}/retry

Retry a task in the dead letter queue.

### GET /dlq

List all dead-lettered tasks.

---

## Configuration

```yaml
# anvil.yaml

server:
  addr: ":8080"
  read_timeout: 30s
  write_timeout: 30s

executor:
  timeout: 5m
  shell: "/bin/sh"

scheduler:
  tick_interval: 10s

queue:
  max_retries: 3
  retry_base_delay: 1m
  retry_max_delay: 10m

logging:
  level: info
  format: json

tasks:
  - name: heartbeat
    schedule: "*/15 * * * *"
    command: "claude -p --resume ${SESSION} 'check status'"
    priority: 0
    max_retries: 1
    
  - name: daily-summary
    schedule: "0 9 * * *"
    command: "claude -p --resume ${SESSION} 'summarize yesterday'"
    priority: 5
```

---

## Retry Strategy

Exponential backoff with jitter:

```go
func retryDelay(attempts int) time.Duration {
    base := time.Minute
    max := 10 * time.Minute
    
    delay := base * time.Duration(1<<attempts) // 1m, 2m, 4m, 8m, ...
    if delay > max {
        delay = max
    }
    
    // Add jitter (±10%)
    jitter := time.Duration(rand.Float64() * 0.2 * float64(delay))
    return delay - delay/10 + jitter
}
```

---

## Observability

### Metrics (Prometheus format)

```
anvil_tasks_total{status="completed"} 127
anvil_tasks_total{status="failed"} 2
anvil_tasks_total{status="dead_letter"} 1
anvil_queue_pending 3
anvil_queue_running 1
anvil_executor_duration_seconds{task="heartbeat"} 5.2
```

### Health Check

```
GET /health → 200 OK if scheduler and worker are running
```

---

## File Structure

```
anvil/
├── cmd/
│   └── anvil/
│       └── main.go
├── internal/
│   ├── queue/
│   │   └── queue.go
│   ├── scheduler/
│   │   └── scheduler.go
│   ├── executor/
│   │   └── executor.go
│   ├── worker/
│   │   └── worker.go
│   ├── api/
│   │   └── server.go
│   └── cron/
│       └── parser.go
├── pkg/
│   └── task/
│       └── task.go
├── config/
│   └── config.go
├── go.mod
├── go.sum
├── README.md
└── SPEC.md
```

---

## Implementation Order

1. `pkg/task/task.go` - Task struct and status
2. `internal/cron/parser.go` - Cron matching
3. `internal/queue/queue.go` - Queue with atomic operations
4. `internal/executor/executor.go` - Shell execution
5. `internal/worker/worker.go` - Queue consumer
6. `internal/scheduler/scheduler.go` - Cron to queue
7. `internal/api/server.go` - HTTP endpoints
8. `config/config.go` - YAML loading
9. `cmd/anvil/main.go` - Wire everything together

---

## Testing Strategy

| Component | Test Type | Focus |
|-----------|-----------|-------|
| cron/parser | Unit | Expression parsing, edge cases |
| queue/queue | Unit | Concurrency, ordering |
| executor/executor | Integration | Shell execution, timeouts |
| scheduler/scheduler | Unit | Tick timing, task creation |
| api/server | Integration | Endpoints, JSON |
| End-to-end | Integration | Full flow |

---

## Future Considerations

- **Persistence**: SQLite or BadgerDB for queue state
- **Clustering**: Multiple workers with distributed lock
- **Web UI**: Dashboard for queue visualization
- **Notifications**: Webhooks on task completion/failure
- **Task dependencies**: Run B after A completes