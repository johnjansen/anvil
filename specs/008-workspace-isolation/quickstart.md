# Quickstart: Task Workspace Isolation

## Basic Usage

### Restrict a task to specific directories

Create a task file with workspace configuration in the frontmatter:

```markdown
---
schedule: "*/30 * * *"
workspace:
  type: restricted
  allowed_paths:
    - ./data/
    - ./output/
  read_only:
    - ./config/
  blocked_paths:
    - ./.env
---
Process data files and write results to output directory.
```

### Run a task in an isolated temp directory

```markdown
---
workspace:
  type: temp
  size: 100mb
---
Run an experimental analysis. All files are cleaned up after completion.
```

### Default behavior (project-only access)

Tasks without a workspace block default to `type: project`, which restricts access to the project directory. No configuration needed.

### View workspace config

```bash
anvil task get my-task
```

Output includes workspace information:

```
Workspace:     restricted
  Allowed:     ./data/, ./output/
  Read-only:   ./config/
  Blocked:     ./.env
```
