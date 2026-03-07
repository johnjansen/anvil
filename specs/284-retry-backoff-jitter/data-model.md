# Data Model: Advanced Task Retry with Backoff Strategies and Jitter

## Modified Entities

### Todo (internal/project)

Extended with new retry configuration fields:

| Field | Type | Default | Description |
| ----- | ---- | ------- | ----------- |
| Retry | int | 0 | Max retry attempts (existing) |
| RetryDelay | time.Duration | 1m | Base delay between retries (existing) |
| RetryStrategy | string | "exponential" | Backoff strategy: "exponential", "linear", "constant" |
| RetryJitter | float64 | 0.0 | Jitter percentage (0.0-1.0), clamped on parse |
| RetryMaxTime | time.Duration | 0 | Max total retry wall-clock time (0 = unlimited) |

**Validation rules**:
- RetryStrategy must be one of: "exponential", "linear", "constant". Invalid values default to "exponential" with a warning.
- RetryJitter is clamped to [0.0, 1.0]. Out-of-range values are clamped with a warning log.
- RetryMaxTime of 0 means no time limit (all retries attempted).

### RunRecord (internal/project)

Extended with retry observability fields:

| Field | Type | JSON Key | Description |
| ----- | ---- | -------- | ----------- |
| Attempt | int | attempt | Final attempt number, 1-based (existing) |
| MaxRetries | int | max_retries | Configured max retries (existing) |
| RetryDelay | string | retry_delay | Base delay string (existing) |
| RetryStrategy | string | retry_strategy | Strategy used: "exponential", "linear", "constant" |
| RetryDelaysUsed | []string | retry_delays_used | Actual delays between attempts |

### YAML Frontmatter (task files)

New fields added to the frontmatter struct:

| Field | YAML Key | Type | Description |
| ----- | -------- | ---- | ----------- |
| RetryStrategy | retry_strategy | string | Backoff strategy name |
| RetryJitter | retry_jitter | float64 | Jitter percentage |
| RetryMaxTime | retry_max_time | string | Max total time as duration string |

### TaskDefaults (project config)

Same three new fields added to project-level defaults, following the existing pattern where task frontmatter overrides project defaults.

| Field | YAML Key | Type | Description |
| ----- | -------- | ---- | ----------- |
| RetryStrategy | retry_strategy | string | Default backoff strategy |
| RetryJitter | retry_jitter | float64 | Default jitter percentage |
| RetryMaxTime | retry_max_time | string | Default max total time |

## Backoff Calculation

Given `attempt` (0-based), `base_delay`, and `strategy`:

```
exponential: delay = base_delay * 2^attempt
linear:      delay = base_delay * (attempt + 1)
constant:    delay = base_delay
```

After strategy calculation:
```
capped_delay = min(delay, 1_hour)
jittered_delay = capped_delay * (1 + jitter * (2*rand() - 1))
final_delay = max(jittered_delay, 1_second)  // floor to prevent near-zero delays
```

## State Transitions

```
Task Fails → Check retries remaining
  → No retries left → Final failure
  → Retries remain → Check max_total_time
    → Time exceeded → Final failure (log: "retry time budget exhausted")
    → Time available → Calculate delay (strategy + jitter) → Wait → Retry
```
