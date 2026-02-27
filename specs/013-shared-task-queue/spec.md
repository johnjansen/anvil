# Feature Specification: Shared Task Queue Across Cluster Members

**Feature Branch**: `013-shared-task-queue`
**Created**: 2026-02-27
**Status**: Draft
**Input**: User description: "Shared task queue across cluster members"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Distributed Task Execution (Priority: P1)

As an operator running a multi-daemon cluster, I want tasks scheduled on any daemon to be distributed across all available daemons so that the cluster's combined worker pool is utilized for parallel execution.

**Why this priority**: This is the core value proposition -- without task distribution, clustering provides no execution benefit. A single leader scheduling tasks across multiple daemons is the fundamental requirement.

**Independent Test**: Schedule a task on the leader daemon. Verify that follower daemons receive and execute the task. Confirm the result is visible from any node.

**Acceptance Scenarios**:

1. **Given** a 3-node cluster with one leader, **When** a task becomes due on the leader, **Then** the leader assigns the task to an available worker across any cluster member based on worker availability.
2. **Given** a follower daemon with idle workers, **When** the leader has a task to distribute, **Then** the follower receives and executes the task.
3. **Given** a task completes on a follower, **When** the result is recorded, **Then** the result is visible from the leader and any other node.

---

### User Story 2 - Node Affinity (Priority: P2)

As an operator, I want to constrain certain tasks to run only on specific daemons so that tasks requiring local resources (files, GPUs, network access) execute on the correct node.

**Why this priority**: Some tasks inherently need to run on specific machines. Without affinity, distributed execution could route tasks to nodes that lack required resources.

**Independent Test**: Configure a task with node affinity for a specific daemon. Verify the task only executes on that daemon, even when other nodes have idle workers.

**Acceptance Scenarios**:

1. **Given** a task configured with affinity for node A, **When** the task becomes due, **Then** the leader assigns it only to node A, regardless of other nodes' availability.
2. **Given** a task with no affinity configured, **When** the task becomes due, **Then** the leader assigns it to any available node.
3. **Given** a task with affinity for node A but node A is offline, **When** the task becomes due, **Then** the task remains queued until node A becomes available.

---

### User Story 3 - Cross-Node Result Access (Priority: P3)

As an operator, I want to view task results and history from any node in the cluster so that I don't need to know which specific daemon executed a task to check its status.

**Why this priority**: Result visibility is important for operations but builds on the distributed execution foundation.

**Independent Test**: Execute a task on a follower node. Query task history from a different node. Verify the result is accessible.

**Acceptance Scenarios**:

1. **Given** a task that completed on node B, **When** the operator queries task history from node A, **Then** the result shows the task ran on node B with full output.
2. **Given** a task in progress on node C, **When** the operator checks running tasks from the leader, **Then** the task appears in the running list with node C identified as the executor.

---

### Edge Cases

- What happens when the leader reassigns after a node goes offline mid-execution?
- What happens when all workers across all nodes are busy?
- What happens when a task is modified on one node while running on another?
- How does the system handle network partitions where some nodes become unreachable?
- What happens when a follower receives a task assignment but has no free workers?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The leader daemon MUST distribute due tasks across all cluster members based on worker availability.
- **FR-002**: Follower daemons MUST accept task assignments from the leader and execute them using their local worker pool.
- **FR-003**: Task results MUST be reported back to the leader for centralized visibility.
- **FR-004**: Tasks MUST support an optional node affinity setting that constrains execution to a specific daemon.
- **FR-005**: Tasks without node affinity MUST be assigned to the node with the most available workers.
- **FR-006**: The system MUST queue tasks when no workers are available across any eligible node.
- **FR-007**: Task execution status (running, completed, failed) MUST be queryable from any node in the cluster.
- **FR-008**: The leader MUST track worker availability across all cluster members.

### Key Entities

- **Task Assignment**: A task dispatched to a specific node for execution (task ID, target node, status)
- **Worker Report**: A node's current worker availability (node ID, total workers, busy workers, idle workers)
- **Task Result**: Execution outcome reported back to the leader (task ID, executing node, exit code, duration)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Tasks are distributed across cluster members within 1 tick interval of becoming due.
- **SC-002**: Cluster utilizes at least 80% of available workers across all nodes when tasks are available.
- **SC-003**: Task results are visible from any node within 5 seconds of completion.
- **SC-004**: Tasks with node affinity execute exclusively on the designated node 100% of the time.

## Assumptions

- The leader election system (from issue #299) is already in place and determines which daemon coordinates task distribution.
- Cluster communication uses the existing TCP transport from the cluster package.
- Only the leader daemon schedules and distributes tasks; followers execute and report back.
- Task definitions (schedules, commands, config) remain stored locally on each node via project configuration; the shared queue distributes execution, not task definitions.
- Node affinity is configured per-task in the task's frontmatter (e.g., "node: <node-id>").
- Network partitions are handled by the existing leader election timeout mechanism; a partitioned follower stops receiving assignments.
