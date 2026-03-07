# Quickstart: Fix Raw Mode Line Break Output

## What Changed

Two files modified to fix daemon output line breaks in raw terminal mode:

1. **`internal/daemon/logger.go`**: Added `rawMode` flag to `daemonLogger`. When active, `println()` writes `\r\n` instead of bare `\n`.
2. **`cmd/anvil/main.go`**: Calls `daemon.SetRawMode(true/false)` when entering/leaving raw terminal mode in `serveCmd()`.

## How to Test

```bash
# Build and run in foreground mode
go build -o anvil ./cmd/anvil
./anvil watch /path/to/project

# Verify: all log lines start at column 0 with no horizontal drift
# Press 'd' to hot-daemonize (tests raw→cooked transition)
```

## Verification Checklist

- [ ] Foreground mode: log lines start at column 0
- [ ] Background (daemonized) mode: log file has no `\r` characters
- [ ] Hot-daemonize transition: output correct before and after detach
- [ ] `go test ./internal/daemon/...` passes
