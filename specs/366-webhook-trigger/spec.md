# Feature Specification: Webhook Trigger for Tasks

**Feature Branch**: `366-webhook-trigger`
**Created**: 2026-03-07
**Status**: Draft
**Input**: User description: "Add webhook trigger for tasks"
**Dependency**: Requires trigger framework from #363; sibling to #364 (file watcher trigger) and #365 (git event trigger)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Trigger Task via HTTP Webhook (Priority: P1)

A user has a task that should execute when an external system (e.g., GitHub, CI pipeline, monitoring tool) sends an HTTP request. They configure the task with a `webhook` trigger specifying a unique endpoint path. When a POST request arrives at that path, the task executes automatically.

**Why this priority**: This is the core use case — receiving external HTTP requests to trigger task execution. Without it, the feature delivers no value.

**Independent Test**: Can be fully tested by creating a task with `trigger: { type: webhook, path: /trigger/my-task }`, sending a POST request to the endpoint, and verifying the task executes.

**Acceptance Scenarios**:

1. **Given** a task configured with `trigger: { type: webhook, path: /trigger/my-task }`, **When** a POST request is sent to `http://localhost:9090/trigger/my-task`, **Then** the task is triggered and executes.
2. **Given** a task configured with a webhook trigger, **When** a GET request is sent to the webhook path, **Then** the request is rejected with an appropriate error response.
3. **Given** a task configured with a webhook trigger, **When** a POST request is sent to a non-existent path, **Then** a 404 response is returned.

---

### User Story 2 - Verify Webhook Authenticity with HMAC Secret (Priority: P1)

A user wants to ensure that only authorized systems can trigger their task. They configure an HMAC secret (referenced from an environment variable) and the system verifies incoming requests against that secret before executing the task.

**Why this priority**: Without secret verification, any network-accessible system can trigger tasks, creating a security risk. This is essential for production use.

**Independent Test**: Can be fully tested by configuring a task with a webhook secret, sending a request with a valid HMAC signature (task triggers), and sending a request with an invalid signature (task does not trigger).

**Acceptance Scenarios**:

1. **Given** a task with `secret: env:WEBHOOK_SECRET`, **When** a request arrives with a valid HMAC signature matching the secret, **Then** the task is triggered.
2. **Given** a task with a configured secret, **When** a request arrives with an invalid or missing signature, **Then** the request is rejected with a 401/403 response and the task does NOT execute.
3. **Given** a task with no secret configured, **When** any POST request arrives at the webhook path, **Then** the task is triggered without signature verification.

---

### User Story 3 - Pass Webhook Payload to Task (Priority: P2)

When a task is triggered by a webhook, the user wants access to the request payload so the task can act on the data sent by the external system (e.g., a GitHub event payload containing commit details).

**Why this priority**: Enables the task to process webhook-specific data rather than performing generic actions, but the feature is still useful without payload passing for simple "just run it" triggers.

**Independent Test**: Can be fully tested by sending a POST request with a JSON body to the webhook endpoint and verifying the task receives the payload data.

**Acceptance Scenarios**:

1. **Given** a task triggered by a webhook, **When** the POST request includes a JSON body, **Then** the payload is available to the task.
2. **Given** a task triggered by a webhook, **When** the POST request includes HTTP headers, **Then** relevant headers (e.g., content type, event type) are available to the task.
3. **Given** a task triggered by a webhook with an empty body, **When** the task executes, **Then** the task runs successfully with no payload.

---

### User Story 4 - Configurable HTTP Server Port (Priority: P2)

A user wants to run the webhook server on a specific port to avoid conflicts with other services or to comply with network policies.

**Why this priority**: A sensible default port works for most users, but configurability is needed for environments with port restrictions or multiple anvil instances.

**Independent Test**: Can be fully tested by configuring a custom port, starting the daemon, and verifying the webhook server listens on the configured port.

**Acceptance Scenarios**:

1. **Given** no port configuration, **When** the daemon starts with webhook-triggered tasks, **Then** the HTTP server starts on port 9090.
2. **Given** a port configured as 8080, **When** the daemon starts, **Then** the HTTP server listens on port 8080.
3. **Given** the configured port is already in use, **When** the daemon starts, **Then** an error is reported and the daemon continues running without webhook support.

---

### User Story 5 - Webhook Server Lifecycle Management (Priority: P2)

The HTTP server should only run when there are tasks with webhook triggers. It should start when webhook-triggered tasks are loaded and stop when none remain.

**Why this priority**: Prevents unnecessary resource consumption and open ports when no webhook triggers are configured.

**Independent Test**: Can be fully tested by starting the daemon with no webhook tasks (verify no HTTP server), adding a webhook task (verify server starts), then removing all webhook tasks (verify server stops).

**Acceptance Scenarios**:

1. **Given** no tasks have webhook triggers, **When** the daemon starts, **Then** no HTTP server is started.
2. **Given** tasks with webhook triggers exist, **When** the daemon starts, **Then** the HTTP server starts and routes are registered.
3. **Given** a running daemon with webhook server, **When** all webhook-triggered tasks are removed, **Then** the HTTP server is stopped.
4. **Given** a running daemon with webhook server, **When** the daemon shuts down, **Then** the HTTP server is cleanly stopped.

---

### Edge Cases

- What happens when two tasks configure the same webhook path? The system should reject the duplicate at configuration load time and log an error.
- What happens when the request body exceeds a reasonable size? The system should enforce a maximum body size (e.g., 1 MB) and reject oversized payloads.
- What happens when the webhook endpoint receives requests faster than the task can execute? The system should queue task executions rather than dropping requests.
- What happens when the HMAC secret environment variable is not set? The daemon should log an error at startup and disable that specific webhook trigger.
- What happens when the HTTP server fails to bind to the configured port? The daemon should report the error clearly and continue running without webhook support.
- What happens during a slow task execution when another webhook request arrives for the same task? The new request should be queued per existing task scheduling behavior.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support a `webhook` trigger type in task frontmatter configuration.
- **FR-002**: System MUST run a lightweight HTTP server within the daemon process when webhook-triggered tasks are present.
- **FR-003**: System MUST register a unique HTTP route for each webhook-triggered task based on its configured `path`.
- **FR-004**: System MUST accept only POST requests on webhook endpoints; other HTTP methods MUST receive a 405 response.
- **FR-005**: System MUST support HMAC-based secret verification for webhook requests, with the secret referenced via `env:VARIABLE_NAME` syntax.
- **FR-006**: System MUST reject requests with invalid or missing HMAC signatures when a secret is configured, returning a 401 response.
- **FR-007**: System MUST pass the webhook request payload (body and relevant headers) to the triggered task.
- **FR-008**: System MUST support a configurable HTTP server port with a default of 9090.
- **FR-009**: System MUST only start the HTTP server when at least one task has a webhook trigger configured.
- **FR-010**: System MUST stop the HTTP server when no webhook-triggered tasks remain.
- **FR-011**: System MUST reject duplicate webhook paths across tasks at configuration load time.
- **FR-012**: System MUST enforce a maximum request body size to prevent resource exhaustion.
- **FR-013**: System MUST integrate with the trigger framework established by #363.
- **FR-014**: System MUST log server start/stop, route registration, request receipt, verification results, and errors for observability.
- **FR-015**: System MUST return appropriate HTTP status codes: 200 for accepted requests, 401 for failed authentication, 404 for unknown paths, 405 for wrong methods, 413 for oversized payloads.

### Key Entities

- **WebhookTrigger**: A trigger configuration specifying the endpoint path, optional HMAC secret reference, and any request filters. Extends the trigger framework from #363.
- **WebhookServer**: A lightweight HTTP server managed by the daemon that listens on a configurable port and routes incoming requests to the appropriate webhook triggers.
- **WebhookRequest**: A received HTTP request including the method, path, headers, body, and HMAC signature for verification.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Tasks configured with webhook triggers execute within 2 seconds of receiving a valid HTTP request.
- **SC-002**: HMAC verification correctly rejects unauthorized requests 100% of the time.
- **SC-003**: Users can configure a complete webhook trigger in under 1 minute using the frontmatter syntax.
- **SC-004**: The webhook server adds negligible resource overhead when idle (no active requests).
- **SC-005**: The server handles at least 100 concurrent webhook requests without dropping any.
- **SC-006**: No open ports exist when no webhook triggers are configured.

## Assumptions

- The trigger framework (#363) provides a plugin/registration mechanism for new trigger types that this feature will use.
- HMAC-SHA256 is the signing algorithm, consistent with industry standards (e.g., GitHub webhooks use HMAC-SHA256).
- The `env:VARIABLE_NAME` syntax for secret references is consistent with existing anvil configuration patterns.
- The HTTP server runs within the daemon process; no separate server process is needed.
- The default port of 9090 avoids common conflicts (8080 for dev servers, 9090 is less commonly used).
- The maximum request body size of 1 MB is sufficient for typical webhook payloads.
- Webhook payload is passed to tasks via environment variables or temporary files, consistent with how other trigger types pass context.
