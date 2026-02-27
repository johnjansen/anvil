# CLI Contract: Workspace Isolation

## Frontmatter Schema Extension

The task frontmatter YAML schema is extended with an optional `workspace` block:

```yaml
workspace:
  type: string          # "project" | "restricted" | "temp" (default: "project")
  allowed_paths: [str]  # list of relative directory paths (restricted only)
  read_only: [str]      # list of relative directory paths (restricted only)
  blocked_paths: [str]  # list of relative directory paths (any type)
  size: string          # size limit like "100mb", "1gb" (temp only)
```

## Environment Variables

Tasks receive the following environment variables when workspace is configured:

| Variable | Description | When Set |
|----------|-------------|----------|
| ANVIL_WORKSPACE_TYPE | "project", "restricted", or "temp" | Always (when workspace configured) |
| ANVIL_WORKSPACE_ROOT | Absolute path to the workspace root directory | Always |
| ANVIL_WORKSPACE_ALLOWED | Comma-separated list of allowed paths (absolute) | restricted type |
| ANVIL_WORKSPACE_READONLY | Comma-separated list of read-only paths (absolute) | restricted type |
| ANVIL_WORKSPACE_BLOCKED | Comma-separated list of blocked paths (absolute) | Any type with blocked_paths |

## `anvil task get` Output Extension

The task get command output includes workspace information after existing fields:

```
Workspace:     <type>
  Allowed:     <comma-separated paths>    # only for restricted
  Read-only:   <comma-separated paths>    # only for restricted
  Blocked:     <comma-separated paths>    # if any blocked_paths
  Size limit:  <size>                     # only for temp
```

For default (project) type with no explicit config, output shows:

```
Workspace:     project (default)
```

## Validation Errors

Invalid workspace configurations produce warnings at load time (stderr):

```
anvil: warning: <file>: workspace path "./../../etc" escapes project root (rejected)
anvil: warning: <file>: workspace type "invalid" not recognized (defaulting to "project")
anvil: warning: <file>: allowed_paths only valid for workspace type "restricted" (ignored)
```
