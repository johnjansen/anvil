# Data Model: Git Event Trigger

## Entities

### GitWatcher

Runtime component managed by the daemon. One instance per daemon, manages all git-triggered tasks.

| Field | Type | Description |
|-------|------|-------------|
| daemon | *Daemon | Back-reference to daemon for task dispatching |
| mu | sync.Mutex | Protects subscriptions map |
| subscriptions | map[string]*gitSubscription | Task ID to subscription mapping |
| stopCh | chan struct{} | Signal to stop all polling |

### gitSubscription

Per-task subscription tracking a set of git refs.

| Field | Type | Description |
|-------|------|-------------|
| taskID | string | Task identifier |
| todo | *Todo | Reference to task configuration |
| branch | string | Branch filter (empty = all branches) |
| pathGlob | string | Path filter glob pattern (empty = all paths) |
| pollInterval | time.Duration | Polling frequency (default: 30s) |
| events | []string | Event types to watch (default: ["push"]) |
| repoPath | string | Git repository root path |
| cancel | context.CancelFunc | Cancel function for this subscription's goroutine |

### GitRefState

Persisted state for last-seen refs. Stored as JSON in `.anvil/git-state/<task-id>.json`.

| Field | Type | Description |
|-------|------|-------------|
| Refs | map[string]string | Branch ref path to commit SHA mapping |
| LastPoll | time.Time | Timestamp of last successful poll |

### GitEvent

Transient event emitted when a ref change is detected.

| Field | Type | Description |
|-------|------|-------------|
| Type | string | Event type: "push" |
| Branch | string | Branch name (e.g., "main") |
| CommitSHA | string | New HEAD commit SHA |
| PrevSHA | string | Previous HEAD commit SHA |
| RepoPath | string | Repository root path |
| ChangedFiles | []string | Files changed between prev and current (for path filtering) |

## Relationships

```
Daemon 1──1 GitWatcher
GitWatcher 1──* gitSubscription
gitSubscription 1──1 Todo (reference)
gitSubscription 1──1 GitRefState (persisted)
gitSubscription ──> GitEvent (emits on ref change)
GitEvent ──> workItem (dispatches to work queue)
```

## State Transitions

### GitWatcher Lifecycle

```
Created → Started (startSubscriptions) → Running (polling) → Stopped (daemon shutdown)
```

### gitSubscription Lifecycle

```
Registered → Polling → (ref change detected) → Filtering → (match) → Dispatched
                                                          → (no match) → Continue Polling
```

### Ref State

```
Empty (first run) → Baseline Set (record current refs, no trigger) → Tracking (detect changes on each poll)
```
