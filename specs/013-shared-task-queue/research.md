# Research: Shared Task Queue Across Cluster Members

## Decision 1: Task Distribution Protocol

**Decision**: Use new cluster message types (task_assign, task_result, worker_report) sent via the existing TCP transport.
**Rationale**: The cluster package already has a reliable TCP transport with JSON messaging, reconnect logic, and peer management. Adding new message types is straightforward -- just add constants to types.go and handlers to node.go.
**Alternatives considered**: HTTP endpoints on each node -- rejected because the TCP transport already handles peer connectivity, and adding HTTP would create a parallel communication channel. Redis/external queue -- rejected as an unnecessary dependency for a task runner.

## Decision 2: Message Payload Extension

**Decision**: Add a Payload field (json.RawMessage) to the existing Message struct in cluster/types.go.
**Rationale**: The current Message struct only has election-specific fields (Term, FromID, Granted). Task distribution needs to carry task data. Using json.RawMessage allows different message types to carry different payloads without a monolithic struct.
**Alternatives considered**: Separate struct per message type -- rejected for complexity; embedding all fields -- rejected for bloat.

## Decision 3: Leader-Driven Distribution

**Decision**: The leader collects due tasks in tick(), then assigns them to specific followers based on worker availability reports. Followers report idle worker counts via heartbeat acks.
**Rationale**: This leverages the existing heartbeat mechanism. Followers already send heartbeat_ack messages -- extending those with worker availability is natural. The leader already has the scheduling logic in tick() and the IsLeader guard.
**Alternatives considered**: Work-stealing (followers pull tasks) -- rejected because it requires a shared state visible to all nodes; Gossip protocol -- over-engineered for this use case.

## Decision 4: Worker Availability Reporting

**Decision**: Extend heartbeat_ack messages to include idle worker count. Leader tracks per-node availability for assignment decisions.
**Rationale**: Heartbeats already flow every 5s. Piggybacking worker counts avoids additional message overhead. The leader already tracks peer last-seen times.
**Alternatives considered**: Separate worker_report message type -- adds complexity without benefit since heartbeat timing is sufficient.

## Decision 5: Node Affinity Configuration

**Decision**: Add a "node" field to task frontmatter in the Todo struct. If set, the leader only assigns to that specific node ID.
**Rationale**: Simple, declarative, per-task configuration that follows the existing frontmatter pattern. Users can set it like any other task option.
**Alternatives considered**: Label-based affinity (node labels + selectors) -- over-engineered for initial version; config-file level affinity -- too coarse-grained.

## Decision 6: Result Reporting

**Decision**: When a follower completes a task, it sends a task_result message to the leader. The leader writes the RunRecord locally. The RunRecord gets a new NodeID field.
**Rationale**: Centralizing results on the leader simplifies querying (all history is on one node). The leader already handles WriteRunRecord. Adding NodeID to RunRecord allows tracking which node executed each run.
**Alternatives considered**: Each node writes its own RunRecord -- makes cross-node querying impossible without a shared filesystem; Distributed log -- unnecessary complexity.

## Decision 7: Serializable Task Representation

**Decision**: Create a TaskAssignment struct that carries the minimal data needed for a follower to execute a task: task name, command/content, environment, timeout, runner config, and project path.
**Rationale**: The full Todo struct contains filesystem paths that are node-specific. We need a serializable subset that any node can execute. Followers need the command to run, not the full project context.
**Alternatives considered**: Serialize the entire Todo -- rejected because it contains non-portable file paths; Only send task ID -- rejected because followers may not have the same project layout.
