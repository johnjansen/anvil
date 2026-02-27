# Quickstart: Leader Election

## Enable Cluster Mode

### anvil.yaml
```yaml
cluster:
  enabled: true
  name: my-cluster
  listen: ":9091"
  peers:
    - "10.0.0.2:9091"
    - "10.0.0.3:9091"
```

## Start Daemon
```bash
anvil watch  # starts daemon with cluster mode
```

## Check Cluster Status
```bash
# Via daemon API
curl --unix-socket ~/.anvil/daemon.sock http://daemon/cluster/status

# Example output:
# {"node_id":"abc-123","role":"leader","term":3,"cluster_size":3,...}
```

## Three-Node Cluster Example

### Node 1 (10.0.0.1)
```yaml
cluster:
  enabled: true
  listen: ":9091"
  peers: ["10.0.0.2:9091", "10.0.0.3:9091"]
```

### Node 2 (10.0.0.2)
```yaml
cluster:
  enabled: true
  listen: ":9091"
  peers: ["10.0.0.1:9091", "10.0.0.3:9091"]
```

### Node 3 (10.0.0.3)
```yaml
cluster:
  enabled: true
  listen: ":9091"
  peers: ["10.0.0.1:9091", "10.0.0.2:9091"]
```

## Verify Election
```bash
# On any node:
curl --unix-socket ~/.anvil/daemon.sock http://daemon/cluster/status | jq .role
# "leader" on one node, "follower" on others
```
