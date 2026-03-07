# Audit: Hot reload — config changes without restart

## Issue #391

### Scope
- Config changes without restart

### Status: WORKING

### Findings

The hot reload feature is **working** as designed.

**Tested:**
- `anvil reload` command - triggers config reload
- SIGHUP signal handling - reloads config on signal
- max_workers reload - applies to new tasks
- timeout reload - applies to new tasks
- runners reload - applies to new tasks
- tick_interval reload - applies to next tick cycle
- webhooks config reload - updates webhook sender config

### Not Reloadable (by design)

The following config fields are intentionally NOT reloaded:
- `env` - global environment variables (would require restarting running tasks)
- `graceful_shutdown_timeout` - only applies on daemon stop
- `quiet_hours` - time-based restrictions (evaluated at task dispatch)
- `sla` - SLA tracking settings
- `notifications` - notification settings
- `health` - health check configuration
- `cluster` - cluster configuration (requires daemon restart)
- `alerts` - alert rules
- `circuit_breaker` - circuit breaker settings
- `priority_aging` - priority aging settings
- `concurrency_groups` - concurrency group settings
- `webhook_port` - HTTP server port (requires daemon restart)
- `auto_update` - auto-update setting (checked on daemon start)
- `input_token_rate` / `output_token_rate` - token cost rates (static)
- `retention` - retention settings (only affects new runs)

This is acceptable since these settings either:
1. Require daemon restart to take effect (ports, cluster config)
2. Would require interrupting running tasks (env vars)
3. Are evaluated at specific times (quiet hours, SLA)

### Conclusion

The hot reload feature is functional for the core use case: changing task execution parameters (workers, timeout, runners, tick interval, webhooks) without interrupting running tasks.
