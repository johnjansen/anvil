# CLI Contract: Task Source File Sync

This document pins the user-facing CLI surface changed or added by this feature. Any deviation from this contract during implementation requires amending this file.

## `anvil add -f <file>`

**Before**: copies `<file>` into `.anvil/todos/p<N>/<slug>.md` and forgets the source.

**After**: same copy, PLUS writes `.anvil/todos/p<N>/<slug>.meta.json` recording the absolute source path, a sha256 of the source bytes, and a `last_loaded_at` timestamp.

**Help text addition** (emitted by `anvil add --help`):

```
  -f, --file <path>   Register a task from a source file. The source path is
                      recorded; `anvil reload` will re-read the source file
                      and update the registered task. Use `anvil task edit
                      <name> --content-file <path>` to point at a different
                      source file later.
```

**Exit codes**: unchanged (`0` success, `1` on any error).

## `anvil reload`

**Before**: sends `POST /reload` to daemon; daemon reloads `~/.anvil/config.yaml`. Task content is not re-read.

**After**: same signal, but daemon also walks each registered task, and for every task with a `SourceMeta` sidecar whose `source_path` is set, re-imports from that source path per the rules in `data-model.md`.

**New stdout format**:

```
reloaded config
tasks: 12 checked, 3 reloaded, 1 drift, 0 missing, 0 invalid
```

If no tasks were affected:

```
reloaded config
tasks: 12 checked, 0 reloaded
```

If the daemon is an older version that doesn't support task-source reload, CLI prints only `reloaded config` (current behavior) — no error.

**Exit codes**: `0` on successful signal. `2` if daemon reports one or more `invalid` tasks (so CI scripts can detect broken task definitions). Unchanged otherwise.

## `anvil task ls`

**Before**: columns `NAME`, `PRIORITY`, `SCHEDULE`, `STATUS` (roughly — see existing implementation).

**After**: adds a `SYNC` column to text output. Values:

- `ok` — in-sync
- `drift` — source differs from registered copy
- `missing` — source path recorded but file not found
- `invalid` — source exists but frontmatter unparseable
- *(blank)* — no source file registered

**JSON output** (`anvil task ls --json` if that flag exists; otherwise add it as part of this feature): each task object gains:

```json
{
  "sync_status": "drift",
  "source_path": "/Users/alice/project/task.md",
  "last_loaded_at": "2026-04-23T12:34:56Z"
}
```

## `anvil task get <name>`

**Before**: prints task metadata.

**After**: adds three new lines when the task has a sidecar:

```
Source:       /Users/alice/project/task.md
Last loaded:  2026-04-23 12:34:56 UTC (3 hours ago)
Sync status:  drift — source file has been edited since last reload
```

When the task has no sidecar (old task or no-source task):

```
Source:       (none)
```

Error messages (for `missing` / `invalid`) include the stored `last_load_error`.

## `anvil status`

**Before**: prints per-project watched state.

**After**: adds one summary line under each project:

```
  /Users/alice/project  todos=12  (3 drift, 1 missing)
```

The parenthesized clause is omitted when all tasks are in-sync or have no source.

## `anvil task edit <name> --content-file <path>`

**Before**: overwrites the registered copy from the given file.

**After**: same, PLUS updates the sidecar's `source_path` to point at the new file and resets the hash. Future reloads will pull from this new path.

## `anvil task rm <name>`

**Before**: removes `.anvil/todos/p<N>/<slug>.md`.

**After**: additionally removes `.anvil/todos/p<N>/<slug>.meta.json` if present. Silent if the sidecar is already absent.

## Frontmatter Aliases (input only)

Source files MAY use either of the following key forms; both are accepted during registration and reload:

| Canonical (written by anvil) | Alias also accepted on input |
| --- | --- |
| `allowed_tools` | `allowed-tools` |
| `max_concurrent` | `max-concurrent` |

No other aliases are introduced by this feature. The on-disk `.anvil/todos/*.md` always uses the canonical form.

## Backward Compatibility Guarantees

- Tasks registered before this feature (no sidecar present) continue to execute exactly as before. They display as blank in the `SYNC` column and show `Source: (none)` in `anvil task get`.
- `anvil add` without `-f` (inline task text) does not create a sidecar. This is the no-source workflow and is unchanged.
- `.anvil/todos/*.md` file format is unchanged. No migrations required.
