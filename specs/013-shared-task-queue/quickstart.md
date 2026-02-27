# Quickstart: Shared Task Queue

## Prerequisites

- anvil cluster with leader election enabled (cluster.enabled: true)
- Multiple daemons configured as peers

## Usage

### Basic distributed execution (no config changes needed)
When cluster mode is enabled and a node becomes leader, it automatically distributes tasks to followers based on worker availability. No configuration changes needed beyond cluster setup.

### Node affinity
Add "node: <node-id>" to a task's frontmatter to constrain it to a specific node:
```yaml
---
schedule: "*/5 * * * *"
node: abc123-def4-5678
---
Check local GPU status
```

### Verify distribution
```bash
# On leader: see which tasks are running on which nodes
anvil cluster status

# Check run records for node information
anvil task history <taskname>
```
