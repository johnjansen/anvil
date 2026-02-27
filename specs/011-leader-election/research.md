# Research: Leader Election for Cluster Coordination

## Decision 1: Election Algorithm
- **Decision**: Simplified Raft-inspired leader election (without log replication)
- **Rationale**: Full Raft is overkill — we only need leader election, not replicated state machine. A simplified protocol with terms, heartbeats, and majority voting is sufficient. Go stdlib provides all needed primitives (net, sync, crypto/rand for jitter).
- **Alternatives**: Bully algorithm (simpler but no split-brain protection), ZooKeeper (external dependency), etcd (external dependency), full Raft (unnecessary complexity).

## Decision 2: Network Transport
- **Decision**: TCP with JSON-encoded messages between daemons
- **Rationale**: Simple, debuggable, no external dependencies. Daemons already use HTTP/JSON for their socket API. TCP provides reliable delivery needed for election protocol.
- **Alternatives**: gRPC (adds protobuf dependency), UDP multicast (unreliable for election), Unix sockets (not cross-machine).

## Decision 3: Node Identity
- **Decision**: Unique node ID generated on first cluster join, persisted to .anvil/node-id
- **Rationale**: Must survive daemon restarts. UUID-based ensures uniqueness across machines. Persisting to disk means a restarted daemon maintains its identity.
- **Alternatives**: Hostname-based (not unique if multiple daemons per host), IP-based (changes with DHCP), random per-start (loses identity on restart).

## Decision 4: Package Structure
- **Decision**: New `internal/cluster/` package with sub-files for election, transport, and types
- **Rationale**: Cluster coordination is a distinct concern from daemon task execution. Separate package allows clean interfaces and testability. Daemon imports cluster package and wires it in.
- **Alternatives**: Add to internal/daemon/ (too much coupling), separate binary (complicates deployment).

## Decision 5: Integration with Daemon
- **Decision**: Daemon starts cluster module if cluster config is enabled. Leader election runs in background goroutines. Daemon queries cluster.IsLeader() to decide whether to schedule tasks.
- **Rationale**: Minimal coupling — daemon only needs to check leadership status. All election logic is encapsulated in the cluster package. Non-cluster mode remains unchanged.
- **Alternatives**: Proxy pattern (daemon wraps cluster), event-driven (cluster emits events to daemon). Both add unnecessary complexity.
