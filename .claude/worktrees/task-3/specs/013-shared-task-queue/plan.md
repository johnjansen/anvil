# Implementation Plan: Shared Task Queue

**Branch**: `013-shared-task-queue` | **Date**: 2026-02-27 | **Spec**: [spec.md](spec.md)

## Summary

Implement distributed task execution across cluster daemons. The leader collects due tasks in tick(), assigns them to followers based on worker availability (reported via heartbeat_ack), and followers execute and report results back. Supports node affinity for tasks that must run on specific nodes.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: internal/cluster (transport, types, node), internal/daemon (tick, worker), internal/project (Todo, RunRecord)
**Storage**: JSON files in .anvil/runs/ (existing RunRecord system)
**Testing**: go build ./cmd/anvil/ (compilation check)
**Target Platform**: macOS, Linux
**Project Type**: CLI/Daemon
**Performance Goals**: Tasks distributed within 1 tick interval
**Constraints**: Must use existing TCP transport, leader-follower model
**Scale/Scope**: ~400 lines across 5 files

## Constitution Check

No violations. Feature extends existing cluster and daemon infrastructure.

## Project Structure

### Source Code

```text
internal/cluster/types.go      # Add new message types + Payload field
internal/cluster/node.go       # Add task message handlers, worker report callback
internal/cluster/queue.go      # NEW: TaskAssignment, TaskResult structs, queue logic
internal/project/project.go    # Add NodeAffinity to Todo, NodeID to RunRecord
internal/daemon/daemon.go      # Modify tick() for distributed dispatch, add follower task execution
```

**Structure Decision**: One new file (queue.go) for task queue types. All other changes in existing files.
