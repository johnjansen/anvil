# Implementation Plan: Cluster CLI Commands

**Branch**: `012-cluster-cli` | **Date**: 2026-02-27 | **Spec**: [spec.md](spec.md)

## Summary

Add three CLI subcommands (anvil cluster status, anvil cluster health, anvil cluster leave) that communicate with the daemon via the existing Unix socket API. The status command queries the existing /cluster/status endpoint. The health command derives a healthy/degraded/unhealthy assessment from status data. The leave command triggers a new /cluster/leave endpoint that stops cluster participation.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: internal/daemon (socket client), internal/cluster (types), internal/config
**Storage**: N/A (stateless CLI commands querying daemon)
**Testing**: go build ./cmd/anvil/ (compilation check)
**Target Platform**: macOS, Linux
**Project Type**: CLI
**Performance Goals**: Commands respond in <1s
**Constraints**: Must use existing socketClient() pattern for daemon communication
**Scale/Scope**: 3 new subcommands, 1 new daemon endpoint, ~200 lines of code

## Constitution Check

No constitution violations. Feature adds CLI commands following established patterns.

## Project Structure

### Source Code (repository root)

```text
cmd/anvil/main.go           # Add clusterCmd dispatcher + 3 subcommand functions
internal/daemon/daemon.go   # Add /cluster/leave endpoint + SendClusterStatus/Leave functions
```

**Structure Decision**: All changes go in existing files. No new source files needed. The cluster package already exists from issue #299.
