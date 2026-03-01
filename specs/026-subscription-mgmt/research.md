# Research: Subscription Management CLI Commands

**Phase 0**: No unknowns identified.

## Key Findings

- Existing subscriptions stored in `.anvil/subscriptions/<type>/<task-id>.json`
- Subscription types: fs (filesystem), amqp (message queue)
- Need to add Status field (active/paused) to subscription data model
- Daemon communicates via Unix socket (existing pattern)
- JSON output support via `--json` flag (existing pattern)

## No Clarifications Needed

This feature is straightforward - it adds operational commands to manage existing subscription infrastructure. No technical unknowns require resolution.
