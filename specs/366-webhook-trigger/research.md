# Research: Webhook Trigger for Tasks

## Decision: Existing Implementation vs. Rewrite

**Decision**: Extend and harden the existing `WebhookServer` in `internal/daemon/webhook.go`

**Rationale**: The webhook trigger is already implemented with core functionality (route registration, HMAC validation, payload passing). The remaining work is hardening and gap-filling rather than a greenfield implementation.

**Alternatives considered**:
- Full rewrite: Unnecessary since the existing code follows established subscription patterns and is well-structured
- Third-party HTTP framework (chi, gin): Overkill for simple route registration; `net/http` ServeMux is sufficient

## Decision: HTTP Server Port Configuration

**Decision**: Add `WebhookPort` field to `config.Config`, defaulting to `9090`. Read from `~/.anvil/config.yaml` as `webhook_port`.

**Rationale**: The issue specifies 9090 as the default. The existing hardcoded `:8080` conflicts with common development server ports. Config follows existing patterns in `internal/config/`.

**Alternatives considered**:
- Per-task port: Too complex, one server with multiple routes is simpler
- Environment variable only: Less discoverable than config file; use both (config takes precedence, env as fallback)

## Decision: Conditional Server Startup

**Decision**: Only start the HTTP server when at least one task has a `webhook` subscription. Defer `Start()` until `startSubscriptions()` detects webhook tasks.

**Rationale**: Avoids opening a port when no webhook tasks are configured, reducing attack surface and resource usage. Matches spec requirement FR-009.

**Alternatives considered**:
- Always start: Wastes resources and opens unnecessary ports
- Lazy start on first subscription: Could cause timing issues; better to scan all tasks first

## Decision: Request Body Size Limit

**Decision**: Limit request body to 1 MB using `http.MaxBytesReader`.

**Rationale**: 1 MB covers typical webhook payloads (GitHub webhooks are typically under 100 KB). Prevents memory exhaustion from malicious oversized requests.

**Alternatives considered**:
- Configurable limit: Over-engineering for this use case
- No limit: Security risk

## Decision: Duplicate Path Detection

**Decision**: Check `ws.handlers` map before registering a new path. Return error if duplicate detected during `StartSubscription`.

**Rationale**: Silent path overwrites could cause confusing behavior. Fail-fast approach makes misconfiguration visible.

## Decision: Webhook Headers Passed to Tasks

**Decision**: Pass `Content-Type` and common webhook event headers (`X-GitHub-Event`, `X-Webhook-Event`) as `ANVIL_WEBHOOK_CONTENT_TYPE` and `ANVIL_WEBHOOK_EVENT` environment variables. Pass all request headers as JSON in `ANVIL_WEBHOOK_HEADERS`.

**Rationale**: Tasks need to know what kind of event triggered them to process the payload correctly. Full headers available in JSON for advanced use cases.

## Decision: Server Lifecycle with Dynamic Tasks

**Decision**: Add `StopSubscription` method. When all webhook handlers are removed, stop the HTTP server. When a new webhook handler is added and the server isn't running, start it.

**Rationale**: Matches the spec requirement for clean lifecycle management (FR-009, FR-010).
