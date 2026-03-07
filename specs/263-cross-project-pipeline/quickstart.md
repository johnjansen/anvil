# Quickstart: Cross-Project Pipeline Visualization

## Prerequisites

- Two or more anvil projects watched via `anvil watch`
- At least one cross-project dependency configured (e.g., `depends_on: other-project:task-name`)

## Setup Test Environment

```bash
# Project A
mkdir -p /tmp/proj-a/.anvil/todos/p1
cat > /tmp/proj-a/.anvil/todos/p1/build.md << 'EOF'
---
schedule: "*/5 * * * *"
---
Build step
EOF

cat > /tmp/proj-a/.anvil/todos/p1/deploy.md << 'EOF'
---
schedule: "0 9 * * *"
depends_on:
  - build
---
Deploy step
EOF

# Project B with cross-project dep
mkdir -p /tmp/proj-b/.anvil/todos/p1
cat > /tmp/proj-b/.anvil/todos/p1/notify.md << 'EOF'
---
depends_on:
  - proj-a:deploy
---
Notify after deploy
EOF

# Watch both projects
cd /tmp/proj-a && anvil watch
cd /tmp/proj-b && anvil watch
```

## Usage

```bash
# View cross-project pipeline (ASCII)
anvil task pipeline --all

# View with verbose info (schedules + status)
anvil task pipeline --all --verbose

# Generate DOT graph
anvil task pipeline --dot --all > pipeline.dot
dot -Tpng pipeline.dot -o pipeline.png
```

## Expected Output

```
=== proj-a ===
build
└── deploy

=== proj-b ===
[proj-a] deploy
└── notify
```
