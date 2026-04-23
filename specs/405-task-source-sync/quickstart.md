# Quickstart: Verify Task Source File Sync

A scripted reproduction of the bug from issue #405, run against the `405-task-source-sync` branch after implementation. Mirrors the issue's reproduction exactly so any regression is obvious.

## Setup

```bash
mkdir /tmp/anvil-sync-quickstart && cd /tmp/anvil-sync-quickstart
anvil register .
cat > task.md <<'TASK'
---
schedule: "*/5 * * * *"
allowed-tools: [Read, Bash]
max-concurrent: 2
priority: 1
---
echo ORIGINAL
TASK
anvil add -f task.md
```

## Verification 1 — source path is recorded

```bash
anvil task get task
```

Expected to include:

```
Source:       /tmp/anvil-sync-quickstart/task.md
Last loaded:  <timestamp> (just now)
Sync status:  ok
```

Also verify sidecar exists:

```bash
ls .anvil/todos/p1/*.meta.json
cat .anvil/todos/p1/task.meta.json
```

Expected JSON:

```json
{
  "source_path": "/tmp/anvil-sync-quickstart/task.md",
  "source_hash_sha256": "...",
  "last_loaded_at": "...",
  "last_load_status": "ok"
}
```

## Verification 2 — hyphenated aliases are honored (not stripped)

```bash
anvil task get task | grep -E 'allowed_tools|max_concurrent'
```

Expected: both keys present with values `[Read, Bash]` and `2` respectively. (The on-disk `.md` uses canonical snake_case; `anvil task get` reflects the effective config.)

## Verification 3 — edit + reload picks up changes

```bash
sed -i '' 's/ORIGINAL/UPDATED/' task.md
anvil reload
```

Expected reload output:

```
reloaded config
tasks: <N> checked, 1 reloaded, 0 drift, 0 missing, 0 invalid
```

Then:

```bash
diff task.md .anvil/todos/p1/task.md
```

Expected: empty diff (content matches, modulo canonical frontmatter normalization — `allowed-tools` in source becomes `allowed_tools` in registered copy, but the body and other fields match).

## Verification 4 — edit WITHOUT reload shows drift

```bash
sed -i '' 's/UPDATED/DRIFTED/' task.md
anvil task ls
```

Expected: the row for `task` shows `drift` in the `SYNC` column.

```bash
anvil status
```

Expected: project line shows `(1 drift)`.

```bash
anvil task get task
```

Expected: `Sync status: drift — source file has been edited since last reload`.

## Verification 5 — reload recovers from drift

```bash
anvil reload
```

Expected: `1 reloaded`. `anvil task ls` now shows `ok` for the task.

## Verification 6 — missing source file

```bash
mv task.md task.md.bak
anvil reload
anvil task ls
```

Expected reload output: `0 reloaded, 0 drift, 1 missing`.
Expected `anvil task ls`: row shows `missing`.
The task continues to be scheduled and runs from the last-loaded `.anvil/todos/p1/task.md` content (no regression).

```bash
mv task.md.bak task.md
anvil reload
```

Expected: `1 reloaded`. Status returns to `ok`.

## Verification 7 — invalid frontmatter

```bash
cat > task.md <<'BROKEN'
---
schedule: "*/5 * * * *
---
echo broken
BROKEN
anvil reload
anvil task ls
```

Expected reload output: `0 reloaded, 0 drift, 0 missing, 1 invalid`.
Expected `anvil task ls`: row shows `invalid`.
Expected `anvil reload` exit code: `2`.
The task continues to run from the last-valid cached content.

## Verification 8 — run history is preserved across reloads

```bash
# Assume at least one run has happened before this test
anvil task history task --limit 5 > before.txt
sed -i '' 's/echo/printf/' task.md
anvil reload
anvil task history task --limit 5 > after.txt
diff before.txt after.txt
```

Expected: empty diff. Run history (UUID, RunIDs, timestamps) is unchanged by reload.

## Verification 9 — tasks without source files are untouched

```bash
anvil task new inline-task --schedule "*/10 * * * *" --content "echo inline"
anvil task get inline-task | grep Source
```

Expected: `Source:       (none)`.

```bash
anvil reload
```

Expected: inline-task is counted in `checked` but not touched. `anvil task ls` shows blank in SYNC column for it.

## Cleanup

```bash
cd / && rm -rf /tmp/anvil-sync-quickstart
anvil task rm task
anvil task rm inline-task
```

## Acceptance

All 9 verifications pass ⟹ spec's P1, P2, and P3 acceptance scenarios are satisfied end-to-end.
