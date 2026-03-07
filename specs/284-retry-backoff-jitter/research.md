# Research: Advanced Task Retry with Backoff Strategies and Jitter

## R1: YAML Configuration Format for Structured Retry

**Decision**: Support both flat (legacy) and structured YAML formats for retry configuration.

**Rationale**: The existing `retry: N` and `retry_delay: Xm` are simple scalar fields. The new configuration needs additional fields (strategy, jitter, max_total_time). Using a structured `retry` block would break backward compatibility since `retry: 3` (int) vs `retry: {max: 3, ...}` (map) are different YAML types. Instead, add new top-level fields alongside the existing ones.

**Format chosen**:
```yaml
# Legacy (still works, implies strategy: exponential)
retry: 3
retry_delay: 1m

# New fields (additive, not replacing)
retry_strategy: exponential    # exponential | linear | constant
retry_jitter: 0.5             # 0.0-1.0
retry_max_time: 30m           # max total retry duration
```

**Alternatives considered**:
- Nested `retry:` block: Breaks backward compatibility with `retry: N` (int vs map type conflict in YAML)
- Separate `backoff:` block: Adds a disconnected config section; retry fields should stay together
- Prefixed fields (chosen): Simple, flat, backward compatible, consistent with existing patterns like `retry_delay`

## R2: Backoff Calculation Strategies

**Decision**: Implement three strategies with a shared formula pattern.

**Rationale**: These three cover the vast majority of real-world retry patterns.

- **Exponential**: `delay * 2^attempt` (current behavior, default)
- **Linear**: `delay * (attempt + 1)`
- **Constant**: `delay` (no growth)

All strategies apply jitter after computing the base delay. All are capped at 1 hour max.

**Alternatives considered**:
- Fibonacci backoff: Rarely needed, can be added later
- Decorrelated jitter (AWS-style): More complex, marginal benefit for CLI task runner
- Configurable multiplier: Over-engineering for current needs

## R3: Jitter Implementation

**Decision**: Use uniform random jitter applied as +/- percentage of computed delay.

**Rationale**: Simple to understand and configure. A jitter of 0.5 on a 1m delay means the actual delay is uniformly distributed in [30s, 90s]. This prevents thundering herd while being predictable for users.

**Formula**: `actual_delay = computed_delay * (1 + jitter * (2*rand - 1))` where rand is [0, 1).

**Alternatives considered**:
- Full jitter (AWS): `delay = rand(0, computed_delay)` — too aggressive, delays can be near-zero
- Equal jitter: `delay = computed_delay/2 + rand(0, computed_delay/2)` — less intuitive to configure
- Decorrelated jitter: Requires tracking previous delay, adds state complexity

## R4: Max Total Time Implementation

**Decision**: Track elapsed time from first attempt start. Before each retry, check if remaining time budget allows the retry to start. If not, fail immediately.

**Rationale**: Simple wall-clock check. No need to predict whether the next attempt will complete within budget — just check if we should even start another retry.

**Alternatives considered**:
- Check if delay + estimated execution time fits: Too complex, execution time varies
- Hard-kill after max_total_time: Dangerous, could kill mid-execution

## R5: RunRecord and History Display

**Decision**: Add `retry_strategy` and `retry_delays_used` fields to RunRecord. Display in task history when retries occurred.

**Rationale**: The RunRecord already has `attempt`, `max_retries`, and `retry_delay` fields. Adding strategy and actual delays used completes the observability picture.

**New RunRecord fields**:
- `retry_strategy` (string): "exponential", "linear", "constant"
- `retry_delays_used` ([]string): actual delays between each attempt, e.g., ["1m2s", "2m15s"]

**Alternatives considered**:
- Store in separate retry log file: Fragmented, harder to query
- Only log to daemon output: Not queryable via `task history`
