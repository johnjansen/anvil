# Webhook HTTP API Contract

## Endpoint: Webhook Trigger

**Method**: POST
**Path**: `{configured_webhook_path}` (e.g., `/trigger/deploy`)
**Port**: Configurable via `webhook_port` in config (default: 9090)

### Request

**Headers** (optional):
- `Content-Type`: Any (typically `application/json`)
- `X-Signature`: `sha256={hmac_hex}` — HMAC-SHA256 signature (required if secret configured)
- `X-Hub-Signature-256`: `sha256={hmac_hex}` — Alternative signature header (GitHub format)

**Body**: Any content, max 1 MB

### Responses

| Status | Condition | Body |
|--------|-----------|------|
| 200 OK | Request accepted, task queued | `Task triggered successfully` |
| 401 Unauthorized | Secret configured, signature invalid/missing | `Unauthorized` |
| 404 Not Found | No task registered for this path | `404 page not found` |
| 405 Method Not Allowed | Non-POST request (or method mismatch) | `Method not allowed` |
| 413 Request Entity Too Large | Body exceeds 1 MB | `Request body too large` |

### HMAC Signature Computation

```
signature = HMAC-SHA256(secret, request_body)
header_value = "sha256=" + hex(signature)
```

## Environment Variables Passed to Task

| Variable | Description |
|----------|-------------|
| `ANVIL_WEBHOOK_PAYLOAD` | Full request body as string |
| `ANVIL_WEBHOOK_CONTENT_TYPE` | `Content-Type` header value |
| `ANVIL_WEBHOOK_EVENT` | Value of `X-GitHub-Event` or `X-Webhook-Event` header (if present) |
| `ANVIL_WEBHOOK_HEADERS` | All request headers as JSON object |

## Frontmatter Configuration

```yaml
subscription:
  type: webhook
  webhook_path: /trigger/my-task        # Required: unique HTTP path
  webhook_method: POST                   # Optional: default POST
  webhook_secret: env:MY_SECRET          # Optional: HMAC secret (env: prefix resolves env var)
```

Simplified syntax (equivalent to above with defaults):

```yaml
subscription:
  webhook: /trigger/my-task
```
