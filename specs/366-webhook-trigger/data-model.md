# Data Model: Webhook Trigger for Tasks

## Entities

### WebhookServer (existing, extend)

Runtime component managing the HTTP server and webhook route handlers.

**Fields**:
- `server`: HTTP server instance
- `mux`: HTTP request multiplexer
- `daemon`: Reference to parent daemon
- `handlers`: Map of path -> handler function (existing)
- `taskPaths`: Map of taskID -> registered path (NEW - for cleanup and duplicate detection)
- `running`: Boolean indicating if the server is actively listening (NEW)
- `mu`: Read-write mutex for concurrent access

**State transitions**:
- `Idle` -> `Running`: When first webhook subscription is registered
- `Running` -> `Idle`: When last webhook subscription is removed
- `Running` -> `Stopped`: On daemon shutdown

### SubscriptionConfig (existing, no changes needed)

Already has all required webhook fields:
- `Type`: "webhook"
- `WebhookPath`: HTTP path for the endpoint
- `WebhookMethod`: HTTP method (default: POST)
- `WebhookSecret`: HMAC secret (supports `env:VAR` syntax)
- `Webhook`: Simplified config shorthand

### Config (existing, extend)

**New field**:
- `WebhookPort`: Integer, default 9090. Port for the webhook HTTP server.

## Environment Variables Passed to Tasks

| Variable | Description | Example |
|----------|-------------|---------|
| `ANVIL_WEBHOOK_PAYLOAD` | Full request body (existing) | `{"action":"push",...}` |
| `ANVIL_WEBHOOK_CONTENT_TYPE` | Content-Type header (NEW) | `application/json` |
| `ANVIL_WEBHOOK_EVENT` | Event type header if present (NEW) | `push` |
| `ANVIL_WEBHOOK_HEADERS` | All request headers as JSON (NEW) | `{"Content-Type":["application/json"]}` |

## Relationships

- `WebhookServer` 1:1 `Daemon` — one webhook server per daemon instance
- `WebhookServer` 1:N `SubscriptionConfig` — one server handles multiple webhook routes
- Each webhook route maps to exactly one `Todo` task
- Path uniqueness enforced: no two tasks can share the same webhook path
