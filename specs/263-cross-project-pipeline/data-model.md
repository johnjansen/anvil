# Data Model: Cross-Project Pipeline Visualization

## Entities

### pipelineTaskInfo (extended)

Existing struct in `cmd/anvil/task_pipeline.go`. Extended with project context.

| Field       | Type     | Description                                                |
|-------------|----------|------------------------------------------------------------|
| name        | string   | Task name without .md extension                            |
| schedule    | string   | Cron schedule expression                                   |
| dependsOn   | []string | Raw dependency strings (may include `project:task` format) |
| projPath    | string   | Filesystem path to the project (existing)                  |
| taskID      | string   | Task UUID (existing)                                       |
| projectName | string   | **NEW** - Human-readable project name for display          |

### Qualified Task Key

When `--all` mode is active, tasks are keyed in the graph as `projectName:taskName` to avoid collisions between same-named tasks in different projects.

| Format              | When Used         | Example              |
|---------------------|-------------------|----------------------|
| `taskName`          | Single project    | `build`              |
| `projectName:task`  | Multi-project     | `project-alpha:build`|

### Cross-Project Edge

Not a separate struct — represented by the relationship between tasks in different projects within the existing adjacency map. Identified by comparing `projectName` of the source and target tasks.

## Relationships

```
Project 1:N Task
Task N:M Task (via dependsOn, may cross project boundaries)
```

## State Transitions

No state transitions — this is a read-only visualization feature.
