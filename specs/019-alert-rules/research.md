# Research: Task Alerting Rules

## Decision 1: Alert Condition Syntax

**Decision**: Use simple prefix-based condition parsing: `cost > N`, `duration > Xm`, `output contains PATTERN`.

**Rationale**: Simple and predictable. Three condition types cover all use cases in the spec.

## Decision 2: Alert Storage

**Decision**: Store fired alerts as JSONL at `.anvil/alerts/<task-id>.jsonl`, append-only.

**Rationale**: Follows activity log pattern. Append-only is safe for concurrent writes.

## Decision 3: Alert Evaluation Location

**Decision**: Evaluate alerts in daemon.go after task completion, alongside existing hooks/webhooks.

**Rationale**: All run data (cost, duration, output) is available at this point.

## Decision 4: Alert Rule Parsing

**Decision**: Add `Alerts []AlertRule` to Todo struct, parsed from frontmatter YAML.

## Decision 5: Webhook Delivery

**Decision**: Reuse existing `webhook.FireURL()` for alert webhooks with alert-specific payload.

## Decision 6: Alert ID Generation

**Decision**: Use first 8 chars of a UUID for alert IDs.

## Decision 7: CLI Command Structure

**Decision**: Add `anvil alerts` as top-level command with `ack` and `history` subcommands.
