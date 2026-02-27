# Feature Specification: Cluster CLI Commands

**Feature Branch**: `012-cluster-cli`
**Created**: 2026-02-27
**Status**: Draft
**Input**: User description: "Add cluster CLI commands (status, health, leave)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Cluster Status (Priority: P1)

As an operator running multiple anvil daemons in a cluster, I want to see which daemons are in the cluster, who the leader is, and the current term so I can understand the cluster topology at a glance.

**Why this priority**: This is the most fundamental cluster management need — operators must be able to see the cluster state before they can manage it.

**Independent Test**: Can be fully tested by running `anvil cluster status` against a running daemon and verifying it displays member information. Delivers immediate visibility into cluster state.

**Acceptance Scenarios**:

1. **Given** a daemon is running with cluster mode enabled, **When** the user runs `anvil cluster status`, **Then** the output shows the node ID, role (leader/follower), current term, leader ID, and a list of all cluster members with their roles and last-seen times.
2. **Given** a daemon is running with cluster mode disabled, **When** the user runs `anvil cluster status`, **Then** the output indicates that cluster mode is not enabled.
3. **Given** a daemon is not running, **When** the user runs `anvil cluster status`, **Then** the output shows an error indicating the daemon is not reachable.

---

### User Story 2 - Check Cluster Health (Priority: P2)

As an operator, I want to check the health of the cluster to quickly determine if all members are responsive and if leadership is stable, so I can identify problems before they affect task scheduling.

**Why this priority**: Health checking is essential for monitoring and alerting but builds on the status foundation.

**Independent Test**: Can be tested by running `anvil cluster health` and verifying it returns a clear healthy/unhealthy assessment with details.

**Acceptance Scenarios**:

1. **Given** a cluster with all members responsive and a stable leader, **When** the user runs `anvil cluster health`, **Then** the output shows "healthy" with member count and leader information.
2. **Given** a cluster where some members have not been seen recently, **When** the user runs `anvil cluster health`, **Then** the output shows "degraded" with details about which members are unresponsive.
3. **Given** a cluster with no elected leader, **When** the user runs `anvil cluster health`, **Then** the output shows "unhealthy" indicating no leader is elected.

---

### User Story 3 - Leave Cluster (Priority: P3)

As an operator, I want to gracefully remove a daemon from the cluster so I can perform maintenance or decommission a node without disrupting the cluster.

**Why this priority**: Leaving the cluster is a less frequent operation than viewing status or health, and is needed for maintenance workflows.

**Independent Test**: Can be tested by running `anvil cluster leave` and verifying the daemon stops participating in the cluster.

**Acceptance Scenarios**:

1. **Given** a daemon is participating in a cluster as a follower, **When** the user runs `anvil cluster leave`, **Then** the daemon stops participating in leader election and heartbeats, and the output confirms the node has left the cluster.
2. **Given** a daemon is the cluster leader, **When** the user runs `anvil cluster leave`, **Then** the daemon steps down from leadership before leaving, allowing a new election to occur, and the output confirms the node has left.
3. **Given** cluster mode is not enabled, **When** the user runs `anvil cluster leave`, **Then** the output indicates cluster mode is not active.

---

### Edge Cases

- What happens when the daemon socket is not accessible (daemon not running)?
- What happens when the cluster status endpoint returns an error?
- What happens when a leave request is sent but the daemon cannot communicate with peers?
- How does the output look when there are zero peers (single-node cluster)?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide an `anvil cluster status` command that displays the current node's cluster state including node ID, role, term, leader ID, cluster size, and member list.
- **FR-002**: System MUST provide an `anvil cluster health` command that assesses cluster health based on member responsiveness and leadership stability, returning a clear healthy/degraded/unhealthy status.
- **FR-003**: System MUST provide an `anvil cluster leave` command that gracefully removes the local daemon from the cluster.
- **FR-004**: All cluster commands MUST communicate with the daemon via the existing daemon socket API.
- **FR-005**: All cluster commands MUST support `--json` flag for machine-readable output.
- **FR-006**: System MUST display a clear error message when cluster mode is not enabled or the daemon is not running.

### Key Entities

- **Cluster Status**: Current node's view of the cluster (node ID, role, term, leader, members)
- **Cluster Health**: Derived assessment (healthy/degraded/unhealthy) based on member last-seen times and leader presence
- **Cluster Member**: Individual daemon in the cluster (ID, role, address, last seen)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can view cluster status in under 1 second via a single CLI command.
- **SC-002**: Operators can determine cluster health status (healthy/degraded/unhealthy) in under 1 second.
- **SC-003**: Operators can gracefully remove a daemon from the cluster via a single CLI command.
- **SC-004**: All cluster commands provide both human-readable and machine-readable (JSON) output.

## Assumptions

- The daemon already has a `/cluster/status` HTTP endpoint (from issue #299 leader election implementation).
- The CLI communicates with the daemon via Unix socket at `~/.anvil/daemon.sock`.
- The cluster leave operation is performed locally by stopping the cluster node, not by coordinating removal across all peers.
- Health assessment uses a configurable staleness threshold (default: 3x heartbeat interval) to determine if a member is unresponsive.
