# Research: Cluster CLI Commands

## Decision 1: CLI-to-Daemon Communication Pattern

**Decision**: Use existing `socketClient()` pattern from `internal/daemon/daemon.go`
**Rationale**: The codebase already has a well-established pattern for CLI-to-daemon communication via Unix socket. Functions like SendPsRequest(), SendStatusRequest(), SendDrainRequest() all use socketClient(). New cluster commands should follow the same pattern.
**Alternatives considered**: Direct TCP connection to cluster port -- rejected because it bypasses the daemon abstraction.

## Decision 2: Command Dispatcher Pattern

**Decision**: Add clusterCmd() dispatcher following the projectCmd()/daemonCmd() pattern in main.go
**Rationale**: main.go already has a consistent pattern for subcommand dispatching with a switch statement.
**Alternatives considered**: Flat commands (anvil cluster-status) -- rejected for consistency with existing grouped commands.

## Decision 3: Daemon Endpoints

**Decision**: Reuse existing /cluster/status endpoint and add /cluster/leave endpoint
**Rationale**: The /cluster/status endpoint was already added in issue #299. Health is derived from status in the CLI. Only /cluster/leave requires a new endpoint.
**Alternatives considered**: Separate /cluster/health endpoint -- rejected because health is a derived view of status data.

## Decision 4: Health Assessment Logic

**Decision**: Derive health from cluster status: healthy (leader exists + all members seen recently), degraded (leader exists but some members stale), unhealthy (no leader)
**Rationale**: Health is a presentation concern that the CLI can compute from the status response.
**Alternatives considered**: Server-side health computation -- rejected to keep the daemon simple.

## Decision 5: Leave Implementation

**Decision**: Add a /cluster/leave POST endpoint that calls clusterNode.Stop() and sets clusterNode to nil
**Rationale**: Leave is a graceful shutdown of cluster participation. The daemon continues running.
**Alternatives considered**: Restart daemon without cluster config -- rejected as too disruptive.
