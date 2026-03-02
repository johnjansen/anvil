# Data Model: Task Circuit Breaker

## Entities

### CircuitBreakerConfig

Configuration for circuit breaker behavior, defined in task frontmatter.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Failures | int | No | Number of consecutive failures before opening circuit (default: 5) |
| Timeout | duration | No | Time to wait before attempting recovery (default: 30m) |
| HalfOpenMax | int | No | Max test requests allowed in half-open state (default: 2) |

### CircuitState

Enum representing the current state of the circuit breaker.

| Value | Description |
|-------|-------------|
| Closed | Normal operation, requests go through |
| Open | Too many failures, requests fail immediately |
| HalfOpen | Testing if service recovered |

### CircuitBreakerRecord

Persisted state for a task's circuit breaker, stored in `.anvil/circuits/<task-id>.json`.

| Field | Type | Description |
|-------|------|-------------|
| TaskID | string | Task identifier |
| State | CircuitState | Current circuit state |
| FailureCount | int | Consecutive failure count |
| LastFailure | timestamp | Time of last failure |
| OpenedAt | timestamp | When circuit opened (null if closed) |
| HalfOpenRequests | int | Test requests made in half-open state |
| LastSuccess | timestamp | Time of last successful run |
| UpdatedAt | timestamp | Last state update |

## State Transitions

```
CLOSED ──(failure)──► OPEN
  │                     │
  │(success)            │(timeout)
  ▼                     ▼
  ┴───────────────── HALF_OPEN
       │                  │
       │(failure)         │(success)
       └──────────────────┘
```

## Storage

- Location: `.anvil/circuits/<task-id>.json`
- Format: JSON
- Created: On first circuit breaker state change
- Updated: On each state transition or failure
- Deleted: When task is deleted (garbage collected)

## Relationships

- **Task → CircuitBreakerRecord**: One-to-one via task ID
- **CircuitBreakerRecord → RunRecord**: Failure count resets on success, increment on failure
