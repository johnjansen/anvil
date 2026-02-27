# API Contract: Leader Election

## Cluster Status Endpoint

### GET /cluster/status
Returns leadership status for this node.

**Response 200:**
```json
{
  "node_id": "abc-123-def",
  "role": "leader",
  "term": 5,
  "leader_id": "abc-123-def",
  "cluster_size": 3,
  "members": [
    {"id": "abc-123-def", "address": "10.0.0.1:9091", "role": "leader", "last_seen": "2026-02-27T10:00:00Z"},
    {"id": "xyz-456-ghi", "address": "10.0.0.2:9091", "role": "follower", "last_seen": "2026-02-27T10:00:02Z"},
    {"id": "mno-789-pqr", "address": "10.0.0.3:9091", "role": "follower", "last_seen": "2026-02-27T10:00:01Z"}
  ]
}
```

## Protocol Messages (TCP/JSON)

### VoteRequest
```json
{"type": "vote_request", "term": 5, "from_id": "abc-123", "to_id": ""}
```

### VoteResponse
```json
{"type": "vote_response", "term": 5, "from_id": "xyz-456", "to_id": "abc-123", "granted": true}
```

### Heartbeat (leader → followers)
```json
{"type": "heartbeat", "term": 5, "from_id": "abc-123", "to_id": ""}
```

### HeartbeatAck (follower → leader)
```json
{"type": "heartbeat_ack", "term": 5, "from_id": "xyz-456", "to_id": "abc-123"}
```

## Configuration (anvil.yaml)
```yaml
cluster:
  enabled: true
  name: production
  listen: ":9091"           # TCP port for cluster protocol
  peers:                    # static peer list (alternative to discovery)
    - "10.0.0.2:9091"
    - "10.0.0.3:9091"
  heartbeat_interval: 5s    # how often leader sends heartbeats
  election_timeout: 30s     # how long to wait before starting election
```
