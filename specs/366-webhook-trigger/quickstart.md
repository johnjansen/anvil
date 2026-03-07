# Quickstart: Webhook Trigger for Tasks

## Basic Webhook Task

Create a task file `.anvil/todos/deploy.md`:

```markdown
---
subscription:
  type: webhook
  webhook_path: /trigger/deploy
---
#!/bin/bash
echo "Deploy triggered!"
echo "Payload: $ANVIL_WEBHOOK_PAYLOAD"
```

Start the daemon and trigger with:

```bash
curl -X POST http://localhost:9090/trigger/deploy -d '{"ref":"main"}'
```

## Webhook with HMAC Secret

```markdown
---
subscription:
  type: webhook
  webhook_path: /trigger/secure-deploy
  webhook_secret: env:DEPLOY_SECRET
---
#!/bin/bash
echo "Verified deploy: $ANVIL_WEBHOOK_PAYLOAD"
```

Trigger with signature:

```bash
SECRET="my-secret"
PAYLOAD='{"ref":"main"}'
SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$SECRET" | cut -d' ' -f2)
curl -X POST http://localhost:9090/trigger/secure-deploy \
  -H "X-Signature: sha256=$SIGNATURE" \
  -d "$PAYLOAD"
```

## Custom Port

Set in `~/.anvil/config.yaml`:

```yaml
webhook_port: 8080
```

## Simplified Syntax

```markdown
---
subscription:
  webhook: /trigger/my-task
---
#!/bin/bash
echo "Triggered via simplified syntax"
```
