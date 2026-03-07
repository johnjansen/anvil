# HTTP Webhook Subscription Example

This example shows how to set up a task that can be triggered via HTTP webhook.

## Task Definition

```yaml
---
# This task runs every hour OR when triggered via webhook
schedule: "@every 1h"
subscription:
  type: webhook
  webhook_path: /webhooks/process-data
  webhook_method: POST
  webhook_secret: env:WEBHOOK_SECRET
env:
  WEBHOOK_SECRET: env:WEBHOOK_SECRET  # Reference the secret in environment
---
#!/bin/bash

echo "Processing data triggered via webhook"
echo "Webhook payload: $ANVIL_WEBHOOK_PAYLOAD"

# Process the payload
if [ -n "$ANVIL_WEBHOOK_PAYLOAD" ]; then
  echo "Received payload, processing..."
  # Your processing logic here
else
  echo "No payload received"
fi
```

## Setting Up the Environment

1. Set the webhook secret as an environment variable:
   ```bash
   export WEBHOOK_SECRET="your-secret-key"
   ```

2. Start the daemon:
   ```bash
   anvil daemon start
   ```

## Triggering the Webhook

Send a POST request to trigger the task:

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -H "X-Signature: sha256=$(echo -n '{"data":"test"}' | openssl dgst -sha256 -hmac your-secret-key | sed 's/.* //')" \
  -d '{"data":"test"}' \
  http://localhost:8080/webhooks/process-data
```

## Webhook Payload

The webhook payload is available in the `ANVIL_WEBHOOK_PAYLOAD` environment variable as a JSON string. Your task script can parse this payload to determine what action to take.

## Security

Webhook requests can be secured using HMAC-SHA256 signatures. If `webhook_secret` is configured, the daemon will validate the signature in the `X-Signature` or `X-Hub-Signature-256` header.