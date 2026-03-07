# CLI Output Contract: anvil task queue

## Table Output (default)

### Without --all (cross-project deps shown as metadata)

```
TASK                                     PRIORITY   STATUS     SKIP REASON                    CASCADE    CROSS-DEPS
--------------------------------------------------------------------------------------------------------
deploy-app.md                            5          pending    dependency failed: other:build  -          other:build(failed)
run-tests.md                             3          running    -                               -          -
sync-data.md                             4          skipped    dependency failed: ext:etl      -          ext:etl(success) ext:validate(failed)
```

The CROSS-DEPS column shows a compact summary: `project:task(status)` for each cross-project dependency. Local dependencies are not shown in this column.

### With --all (cross-project deps as separate entries)

```
TASK                                     PRIORITY   STATUS     SKIP REASON                    CASCADE    CROSS-DEPS
--------------------------------------------------------------------------------------------------------
[other] build.md                         -          success    -                               -          -
[ext] etl.md                             -          success    -                               -          -
[ext] validate.md                        -          failed     -                               -          -
deploy-app.md                            5          pending    dependency failed: other:build  -          other:build(failed)
run-tests.md                             3          running    -                               -          -
sync-data.md                             4          skipped    dependency failed: ext:etl      -          ext:etl(success) ext:validate(failed)
```

Cross-project entries are prefixed with `[project-name]` and show their last run status. Priority shows `-` since they belong to another project.

## JSON Output (--json)

```json
[
  {
    "project": "/path/to/project",
    "name": "deploy-app.md",
    "priority": 5,
    "status": "pending",
    "skip_reason": "dependency failed: other:build",
    "cross_deps": [
      {
        "project": "other",
        "task": "build.md",
        "status": "failed",
        "blocking": true,
        "last_run": ""
      }
    ]
  },
  {
    "project": "/path/to/project",
    "name": "sync-data.md",
    "priority": 4,
    "status": "skipped",
    "skip_reason": "dependency failed: ext:etl",
    "cross_deps": [
      {
        "project": "ext",
        "task": "etl.md",
        "status": "success",
        "blocking": false,
        "last_run": "2026-03-07T10:00:00Z"
      },
      {
        "project": "ext",
        "task": "validate.md",
        "status": "failed",
        "blocking": true,
        "last_run": "2026-03-07T09:30:00Z"
      }
    ]
  }
]
```

## JSON Output with --all (--json --all)

Same as above, but with additional entries for cross-project tasks:

```json
[
  {
    "project": "other",
    "name": "build.md",
    "priority": 0,
    "status": "failed",
    "is_cross_project": true
  },
  {
    "project": "ext",
    "name": "etl.md",
    "priority": 0,
    "status": "success",
    "is_cross_project": true
  },
  ...local tasks with cross_deps as above...
]
```

## Error Cases

| Scenario | CROSS-DEPS column | JSON status |
|----------|-------------------|-------------|
| Project not in watched | `other:build(unknown project)` | `"unknown project"` |
| Task not found in project | `other:build(task not found)` | `"task not found"` |
| Task never run | `other:build(no runs)` | `"no runs"` |
