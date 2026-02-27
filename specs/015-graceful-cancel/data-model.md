# Data Model: Graceful Cancel with Partial Result Capture

**Date**: 2026-02-27
**Feature**: 015-graceful-cancel

## Modified Entities

### RunRecord (modified)

New fields added to existing struct:

| Field | Type | Description |
|-------|------|-------------|
| PartialResults | string | Latest partial result JSON emitted by the task via `##anvil:partial` |
| TerminationMethod | string | How the task ended: "normal", "graceful", "force", "timeout" |

### Todo (modified)

New field added to existing struct:

| Field | Type | Description |
|-------|------|-------------|
| OnKill | string | Shell command to run before task termination during graceful kill |

### KillRequest (modified)

New field added to existing struct:

| Field | Type | Description |
|-------|------|-------------|
| Graceful | bool | If true, send SIGTERM + run on_kill hook before force kill |

### RunRequest (modified)

New field added to existing struct:

| Field | Type | Description |
|-------|------|-------------|
| Resume | bool | If true, inject previous run's partial results into env |

## State Transitions

```text
Running  -> [kill --graceful] -> Graceful Shutdown (SIGTERM sent, on_kill runs)
Graceful Shutdown -> [task exits] -> Completed (graceful)
Graceful Shutdown -> [grace period expires] -> Completed (force)
Running  -> [kill --force] -> Completed (force)
Running  -> [kill (no flag)] -> Completed (force, backward compat)
Running  -> [timeout] -> Completed (timeout)
Running  -> [success] -> Completed (normal)
```
