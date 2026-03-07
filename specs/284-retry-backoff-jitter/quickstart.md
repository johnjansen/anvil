# Quickstart: Advanced Task Retry with Backoff Strategies and Jitter

## Basic Usage

### Configure a task with exponential backoff (default, same as current behavior)

```yaml
---
schedule: "*/30 * * * *"
retry: 3
retry_delay: 1m
---
Do the thing...
```

Retries after: ~1m, ~2m, ~4m

### Configure linear backoff

```yaml
---
schedule: "*/30 * * * *"
retry: 3
retry_delay: 1m
retry_strategy: linear
---
Do the thing...
```

Retries after: ~1m, ~2m, ~3m

### Configure constant delay

```yaml
---
schedule: "*/30 * * * *"
retry: 3
retry_delay: 2m
retry_strategy: constant
---
Do the thing...
```

Retries after: ~2m, ~2m, ~2m

### Add jitter to prevent thundering herd

```yaml
---
schedule: "*/30 * * * *"
retry: 5
retry_delay: 1m
retry_strategy: exponential
retry_jitter: 0.5
---
Do the thing...
```

Retries after: ~1m(+/-50%), ~2m(+/-50%), ~4m(+/-50%), ...

### Limit total retry time

```yaml
---
schedule: "0 * * * *"
retry: 10
retry_delay: 2m
retry_strategy: exponential
retry_max_time: 30m
---
Do the thing...
```

Retries stop after 30 minutes regardless of remaining attempts.

### Set project-level defaults

In `.anvil/config.yaml`:

```yaml
defaults:
  retry: 3
  retry_delay: 30s
  retry_strategy: exponential
  retry_jitter: 0.25
```

Individual tasks override these defaults via their frontmatter.

## Viewing Retry History

```bash
$ anvil task history my-task
```

Shows retry strategy, attempt count, and actual delays used for each run.
