# Data Model: Task Workspace Isolation

## Entities

### WorkspaceConfig

Represents the workspace isolation settings for a task.

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| Type | string | Workspace type: "project", "restricted", "temp" | "project" |
| AllowedPaths | []string | Directories with read/write access (restricted type) | nil |
| ReadOnly | []string | Directories with read-only access (restricted type) | nil |
| BlockedPaths | []string | Directories that are always blocked | nil |
| Size | string | Max temp workspace size, e.g. "100mb" (temp type) | "" (unlimited) |

### YAML Frontmatter Schema

```yaml
workspace:
  type: restricted|temp|project    # default: project
  allowed_paths:                   # only for type: restricted
    - ./data/
    - ./output/
  read_only:                       # only for type: restricted
    - ./config/
  blocked_paths:                   # for any type
    - ~/.ssh/
    - ~/.anvil/
  size: 100mb                      # only for type: temp
```

### Todo Struct Extension

The existing `Todo` struct in `internal/project/project.go` is extended with:

| Field | Type | Description |
|-------|------|-------------|
| Workspace | WorkspaceConfig | Workspace isolation settings (zero value = project type) |

### Validation Rules

1. `Type` must be one of: "project", "restricted", "temp" (empty defaults to "project")
2. `AllowedPaths` and `ReadOnly` are only valid when Type is "restricted"
3. `Size` is only valid when Type is "temp"
4. `BlockedPaths` is valid for any type
5. All paths must resolve within the project root (for restricted type)
6. Paths containing `..` that escape the project root are rejected at parse time
7. Symlinks that resolve outside allowed paths are rejected at parse time

### State Transitions

None — WorkspaceConfig is immutable per task load. Changes require editing the task file and reloading.

## Relationships

- `Todo` has-one `WorkspaceConfig` (embedded struct)
- `WorkspaceConfig` is parsed from YAML frontmatter alongside existing Todo fields
- `daemon.runTask()` reads `Todo.Workspace` to configure execution environment
- `runner.Run()` receives the resolved working directory (may be temp dir for temp type)
