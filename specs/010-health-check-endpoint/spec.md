# Feature Specification: Task Health Check Endpoint for Container Orchestration

## Overview

When running anvil in containers or Kubernetes, operators need proper readiness and liveness probes to manage daemon lifecycle. Currently, there is no way to distinguish "daemon running" from "daemon ready to process tasks," which causes container orchestrators to mismanage anvil containers. This feature adds dedicated HTTP health check endpoints that integrate with standard container orchestration patterns.

## User Stories

### US1: Readiness Probe (Priority: P1)
**As a** DevOps engineer deploying anvil in Kubernetes,
**I want** a readiness endpoint that reports whether the daemon can accept new tasks,
**So that** my orchestrator only routes work to healthy, ready instances.

**Acceptance Criteria:**
- Endpoint returns HTTP 200 when daemon is ready to accept tasks
- Endpoint returns HTTP 503 when daemon cannot accept tasks (worker pool full, no projects loaded, shutting down)
- Response includes a brief JSON body explaining readiness status

### US2: Liveness Probe (Priority: P1)
**As a** DevOps engineer running anvil in containers,
**I want** a liveness endpoint that confirms the daemon process is healthy,
**So that** my orchestrator can restart unresponsive or deadlocked instances.

**Acceptance Criteria:**
- Endpoint returns HTTP 200 when daemon is alive and responsive
- Endpoint must respond quickly (suitable for frequent polling)
- A non-responsive endpoint signals the orchestrator to restart the container

### US3: Detailed Status Endpoint (Priority: P1)
**As an** operator monitoring anvil infrastructure,
**I want** a detailed status endpoint showing worker, project, and task counts,
**So that** I can build dashboards and alerting around daemon capacity.

**Acceptance Criteria:**
- Endpoint returns a JSON object with readiness status, worker availability, project count, running task count, and pending task count
- Response format is stable and documented for integration with monitoring tools
- Data reflects real-time daemon state

## Functional Requirements

### FR1: Readiness Endpoint
The system shall expose a `/ready` HTTP endpoint that returns:
- HTTP 200 with `{"ready": true}` when the daemon can accept new tasks
- HTTP 503 with `{"ready": false, "reason": "<explanation>"}` when it cannot

Readiness conditions (all must be true for 200):
- Daemon is running and event loop is active
- At least one worker slot is available
- At least one project is loaded
- Daemon is not in drain/shutdown mode

### FR2: Liveness Endpoint
The system shall expose a `/live` HTTP endpoint that returns:
- HTTP 200 with `{"alive": true}` when the daemon process is responsive

This endpoint must have minimal overhead and always return quickly as long as the process is running.

### FR3: Status Endpoint
The system shall expose a `/status` HTTP endpoint that returns a JSON object with:
- `ready`: boolean readiness status
- `workers`: object with `available` and `max` counts
- `projects`: integer count of loaded projects
- `running_tasks`: integer count of currently executing tasks
- `pending_tasks`: integer count of tasks queued for execution
- `uptime`: string representing daemon uptime

### FR4: Transport Support
The health endpoints shall be accessible via the daemon's existing HTTP server (Unix socket or TCP port), using the same transport configuration as other daemon endpoints.

### FR5: Backward Compatibility
The existing `/health` endpoint (if present) shall continue to function unchanged. The new endpoints are additive.

## User Scenarios & Testing

### Scenario 1: Kubernetes Readiness Check
1. Deploy anvil daemon in a Kubernetes pod
2. Configure readiness probe to `/ready` endpoint
3. Pod starts and daemon initializes
4. `/ready` returns 503 during startup (no projects loaded yet)
5. Projects load successfully
6. `/ready` returns 200 — pod marked as ready
7. All worker slots fill up
8. `/ready` returns 503 — orchestrator stops routing new work
9. Worker completes, slot opens
10. `/ready` returns 200 again

### Scenario 2: Liveness Monitoring
1. Daemon starts and begins processing
2. `/live` returns 200 consistently
3. If daemon hangs or deadlocks, `/live` stops responding
4. Orchestrator detects timeout and restarts container

### Scenario 3: Capacity Dashboard
1. Operator configures monitoring tool to poll `/status`
2. Dashboard shows real-time worker utilization
3. Alerts fire when available workers drop below threshold
4. Operator scales up based on pending task count trends

## Success Criteria

- Health check endpoints respond within 100 milliseconds under normal load
- Readiness endpoint accurately reflects daemon capacity (no false positives when workers are full)
- Operators can configure Kubernetes probes using standard YAML patterns
- Status endpoint provides sufficient data for capacity monitoring and alerting
- No regression in existing daemon behavior or performance

## Assumptions

- The daemon already has an HTTP server for API endpoints (Unix socket or TCP)
- Worker pool size is known and trackable at runtime
- Project count and task counts are available from existing daemon state
- The daemon has a concept of "shutting down" state that can be queried

## Dependencies

- Existing daemon HTTP server infrastructure
- Worker pool management (for available/max counts)
- Project loader (for project count)
- Task scheduler (for running/pending counts)

## Out of Scope

- Custom health check logic per task
- Health check authentication/authorization
- Distributed health aggregation across multiple daemon instances
- Prometheus metrics format (separate feature)
