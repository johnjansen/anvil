# Implementation Plan: Health Check Endpoint

## Technical Context

- **Language**: Go 1.24.6 (stdlib net/http, encoding/json, sync/atomic)
- **Architecture**: Extends existing daemon HTTP server (Unix socket at ~/.anvil/daemon.sock)
- **Existing infrastructure**: Daemon has full HTTP mux with /health, /status, /metrics, /ps, /queue endpoints
- **Key data sources**: d.tasks (running), d.pendingTasks (pending), d.config.MaxWorkers, d.inFlight, d.draining, d.startedAt

## File Structure

```text
internal/daemon/daemon.go     — Add /ready, /live handlers + register in mux
internal/daemon/daemon.go     — Enhance /status handler with new fields
internal/config/config.go     — Add optional health_port config field
cmd/anvil/main.go             — (optional) Add anvil status --ready CLI flag
```

## Implementation Approach

### Phase 1: Core Endpoints (P1)
1. Add handleReady() — checks worker availability, project count, drain state; returns 200/503
2. Add handleLive() — returns 200 always (process alive = endpoint responsive)
3. Enhance handleStatus() — add ready, workers, pending_tasks, draining fields to JSON response
4. Register /ready and /live in startSocketServer() mux

### Phase 2: Optional TCP Health Port (P1)
5. Add HealthPort field to config.Config
6. Start separate TCP listener for health endpoints only (when configured)

### Phase 3: CLI Integration (P2)
7. (Optional) Expose readiness/status check via CLI for non-Kubernetes users

## Dependencies
- internal/daemon/daemon.go (existing HTTP server)
- internal/config/config.go (config parsing)
- No external dependencies needed

## Constitution Check
- All stdlib, no new dependencies ✓
- Backward compatible (additive endpoints only) ✓
- Best-effort approach (health endpoints never block tasks) ✓
