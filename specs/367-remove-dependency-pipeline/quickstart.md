# Quickstart: Remove Task Dependency Pipeline

## What This Changes

This feature removes the broken `depends_on` task dependency pipeline. After this change:

- Task files with `depends_on` fields still load — the field is silently ignored
- `anvil task pipeline` command no longer exists
- `anvil task create --depends-on` flag no longer exists
- Task list/dry-run output no longer shows dependency information

## Verification

After implementation, verify with:

1. **Build succeeds**: `go build ./...`
2. **Tests pass**: `go test ./...`
3. **No dependency references in code**: `grep -r "ParseDependency\|ResolveDependencyRunRecord\|depends_on" internal/ cmd/ --include="*.go"` should return nothing
4. **Stale frontmatter works**: Create a task with `depends_on: foo` in frontmatter, run `anvil task list` — task should appear without errors
5. **Pipeline command removed**: `anvil task pipeline` returns unknown command error
