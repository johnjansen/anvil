# Feature Specification: Leader Election for Cluster Coordination

## Overview

When multiple anvil daemons run across different machines in a cluster, there must be a single coordinator to prevent duplicate task execution and manage cluster-wide operations. Without a leader, all daemons independently schedule and execute the same tasks, resulting in wasted resources and conflicting results. This feature adds leader election so that exactly one daemon serves as the cluster coordinator at any time, with automatic failover if the leader becomes unavailable.

## User Stories

### US1: Prevent Duplicate Task Execution (Priority: P1)
**As a** DevOps engineer running anvil across multiple machines,
**I want** a single leader daemon to coordinate task scheduling,
**So that** tasks are not executed redundantly on multiple nodes.

**Acceptance Criteria:**
- Only the elected leader assigns tasks to cluster members
- Non-leader daemons wait for task assignments from the leader
- If two daemons start simultaneously, exactly one becomes leader
- No task is executed more than once during normal cluster operation

### US2: Automatic Leader Failover (Priority: P1)
**As a** system administrator managing a production anvil cluster,
**I want** automatic failover when the leader daemon crashes or becomes unresponsive,
**So that** the cluster continues operating without manual intervention.

**Acceptance Criteria:**
- Remaining daemons detect leader failure within a configurable timeout (default: 30 seconds)
- A new leader is elected automatically without human intervention
- In-flight tasks on the failed leader are reported and can be re-scheduled
- Cluster resumes normal task scheduling after failover completes

### US3: Leadership Visibility (Priority: P2)
**As an** operator monitoring the anvil cluster,
**I want** to see which daemon is the current leader and the election state,
**So that** I can diagnose coordination issues and verify cluster health.

**Acceptance Criteria:**
- Leadership status is visible via daemon API endpoint
- CLI command shows current leader identity, election term, and member count
- Leadership changes are logged with timestamps and reason

## Functional Requirements

### FR1: Leader Election Protocol
The system shall implement a leader election mechanism where:
- Exactly one daemon is elected leader at any time
- Election uses term numbers to prevent stale leaders
- Each daemon has a unique identity (node ID)
- Election completes within 10 seconds under normal conditions

### FR2: Heartbeat Mechanism
The leader shall send periodic heartbeats to all followers:
- Heartbeat interval is configurable (default: 5 seconds)
- If followers do not receive a heartbeat within the election timeout (default: 30 seconds), they initiate a new election
- Heartbeat messages include the current term number and leader identity

### FR3: Leader Responsibilities
The elected leader shall:
- Coordinate task scheduling across cluster members
- Maintain a view of available workers across all nodes
- Reassign tasks if a follower node fails
- Step down if it detects a higher-term leader

### FR4: Follower Behavior
Non-leader daemons shall:
- Accept task assignments from the current leader
- Report their worker availability to the leader
- Trigger an election if the leader heartbeat times out
- Reject requests from stale leaders (lower term number)

### FR5: Split-Brain Prevention
The system shall prevent split-brain scenarios where multiple leaders exist:
- A leader must maintain contact with a majority of nodes to remain leader
- If a leader loses majority contact, it voluntarily steps down
- Term numbers ensure only the latest election winner is recognized

### FR6: Leadership Status API
The system shall expose leadership status via:
- API endpoint returning current role (leader/follower/candidate), term, leader ID, and cluster size
- Log entries for all leadership transitions (elected, stepped down, failover)

## User Scenarios & Testing

### Scenario 1: Initial Cluster Formation
1. Start three anvil daemons with cluster mode enabled
2. Daemons discover each other (via discovery mechanism)
3. An election occurs — one daemon becomes leader
4. Leader begins coordinating task execution
5. Two followers report their worker capacity to the leader
6. Tasks are distributed across all three nodes without duplication

### Scenario 2: Leader Failure and Failover
1. Three-node cluster is running with daemon A as leader
2. Daemon A crashes unexpectedly
3. Daemons B and C detect missing heartbeats after timeout
4. Either B or C initiates an election
5. One of them wins the election and becomes the new leader
6. Task scheduling resumes within the failover window
7. Tasks that were in-flight on daemon A are reported as potentially incomplete

### Scenario 3: Network Partition Recovery
1. Three-node cluster is running
2. Leader loses network to one follower but maintains majority
3. Leader continues operating (still has 2/3 majority)
4. Isolated follower starts an election but cannot win (no majority)
5. Network recovers — isolated node recognizes existing leader and rejoins

### Scenario 4: Monitoring Leadership
1. Operator queries leadership status via API
2. Response shows: role=leader, term=5, members=3, uptime=2h
3. Operator checks another node: role=follower, leader=node-A, term=5
4. Logs show election history with timestamps

## Success Criteria

- Leader election completes within 10 seconds of cluster formation
- Failover to a new leader completes within 30 seconds of leader failure
- No duplicate task execution occurs during normal cluster operation
- Cluster of 3 nodes sustains 1 node failure without service interruption
- Leadership status is always queryable from any cluster member
- Split-brain scenarios are prevented (never more than one active leader per term)

## Assumptions

- Daemons can communicate with each other over the network (TCP)
- A cluster discovery mechanism exists or will exist for daemons to find each other
- Cluster size is typically 3-5 nodes (odd numbers preferred for majority quorum)
- Network partitions are temporary and eventually heal
- Clock drift between nodes is within reasonable bounds (< 1 second)

## Dependencies

- Cluster discovery mechanism (for daemons to find peers) — may be developed in parallel
- Network transport layer between daemons
- Daemon identity system (unique node IDs)

## Out of Scope

- Consensus on data replication (this is leadership only, not replicated state machine)
- Automatic cluster scaling (adding/removing nodes dynamically)
- Cross-datacenter or WAN cluster support
- Encrypted inter-daemon communication (can be added later)
