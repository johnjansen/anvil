---
disabled: true
schedule: '*/15 * * * *'
priority: 2
max_concurrent: 1
skip_permissions: true
allowed_tools:
    - Bash(gh:*)
    - Bash(jq:*)
    - Bash(recall:*)
    - Bash(/Users/johnjansen/.local/bin/recall:*)
    - Bash(terminal-notifier:*)
    - Bash(cmux:*)
    - Bash(pgrep:*)
    - Bash(date:*)
    - Bash(echo:*)
    - Bash(printf:*)
    - Bash(awk:*)
    - Bash(sed:*)
    - Bash(grep:*)
    - Bash(sort:*)
    - Bash(head:*)
    - Bash(tr:*)
    - Bash(test:*)
    - Bash([:*)
    - Read
    - Glob
    - Grep
on_success: ""
on_failure: ""
timeout: ""
retry: 0
retry_delay: ""
persistent_cooldown: ""
persistent_max_runtime: ""
---
# My PR monitor — discovery (redirect)

Sibling of `my-pr-monitor.md`. **Disabled by default.** Ships alongside the
existing task per the redirect plan
(`dw/ideas/2026-05-29-my-pr-monitor-redirect.md`).

This task is **discover + diff + notify, full stop**. It owns no fixes, no
commits, no replies, no worktrees. It spots state transitions on PRs I
authored and routes structured events to the originating cmux workspace —
the workspace that opened the PR is the one that responds.

> **Hard rule:** Do NOT enable this task while `my-pr-monitor.md` is also
> enabled until plan step 9.6 validates ≥ 5 cycles with no recall keyspace
> conflict. The two tasks have disjoint write surfaces (`comments`/`checks`/
> `notifications` vs `events`), but a human-in-the-loop window is cheap.

## Agent identity

This task does not post under any `[jj.*]` persona. It is silent on GH —
its only outputs are recall writes, cmux paint, and at most one
`terminal-notifier` per ready-to-merge transition. PR replies happen in the
originating workspace under that workspace's persona; JJ-Monitor `[ƒ:kx7]`
is retired from this loop.

## Required tools (must exist on PATH)

`gh`, `jq`, `recall`, `terminal-notifier`, `cmux`. If any is missing, log
one line and exit 0 — next tick will retry.

```bash
# Anvil workers don't inherit a login shell, so PATH is minimal.
export PATH="$HOME/.local/bin:/opt/homebrew/bin:/usr/local/bin:$PATH"

for cmd in gh jq recall terminal-notifier cmux; do
  command -v "$cmd" >/dev/null || { echo "missing: $cmd (PATH=$PATH)"; exit 0; }
done
```

---

## Memory: `recall`

All per-PR state lives in `recall`. Keyspace is shared with the existing
`my-pr-monitor.md` task during the coexistence window; the two tasks write
to disjoint collections.

### Data model

| Recall primitive | Key | Status / attrs | Purpose |
|---|---|---|---|
| `ent pr <repo>#<num>` | attrs: `head_sha`, `state`, `last_seen`, `origin_workspace_ref`, `origin_branch` | The PR + cached origin mapping |
| `child pr <repo>#<num> events <event_id>` | status: `pending` / `delivered` / `acked` / `superseded`; attrs: `kind`, `payload_json`, `created_at`, `delivered_at`, `target_workspace` | Per-event dedup + delivery ledger (this task) |
| `child pr <repo>#<num> comments <comment_id>` | status: `seen` / `superseded` | "have I emitted an event for this comment yet" gate, scoped to current `head_sha` (this task) |
| `child pr <repo>#<num> checks <check_name>` | status: last-seen GH conclusion | Detect transitions, not drive action (this task; shared shape with old task) |
| `child pr <repo>#<num> notifications ready-to-merge` | status: `pending` / `sent` | Idempotency for the one `terminal-notifier` we still fire (shared with old task) |

`<event_id>` is `<kind>:<head_sha>:<discriminator>`:
- `new_review_comment:<sha>:<comment_id>`
- `ci_failure:<sha>:<check_name>`
- `merge_conflict:<sha>`
- `ready_to_merge:<sha>`

The discriminator pins the event to the SHA so head_sha rollover supersedes
everything cleanly.

### head_sha rollover

When `head_sha` advances (we pushed a fix, or the user pushed), older
`events` / `comments` / `checks` are stale relative to the new code.
Supersede them so they don't keep matching as `pending`:

```bash
PREV=$(recall ent get pr "$PR_KEY" 2>/dev/null | jq -r '.Attrs.head_sha // ""')
if [ -n "$PREV" ] && [ "$PREV" != "$HEAD_SHA" ]; then
  recall child supersede pr "$PR_KEY" events
  recall child supersede pr "$PR_KEY" comments
  recall child supersede pr "$PR_KEY" checks
  recall child del pr "$PR_KEY" notifications ready-to-merge 2>/dev/null
fi
recall ent put pr "$PR_KEY" head_sha="$HEAD_SHA" state=open last_seen="$(date -u +%FT%TZ)"
```

### Idempotency rule

This task only ever moves an event from `pending → delivered`. The
originating workspace is the only writer that moves `delivered → acked`. If
the workspace never acks, this task re-paints (cmux pill stays Rose) on the
next cycle but does NOT re-fire `terminal-notifier`.

---

## Cycle

### Phase 1 — discover

```bash
ME=$(gh api user --jq '.login')

# Authoritative discovery: every open non-draft PR I authored.
gh search prs is:open archived:false author:@me \
  --sort updated --order desc \
  --json url,repository,number,title,updatedAt --limit 50 \
  > /tmp/my-pr-monitor-discover.prs.json
```

Build `PR_LIST` of `(repo, number, url)` triples. If empty, exit 0.

### Phase 2 — per-PR loop

**Scope: only emit events for PRs in the `PrimerAI` org.** Skip everything
else with no side effects.

```bash
[ "${REPO%%/*}" = "PrimerAI" ] || continue
```

For each `(REPO, NUM, URL)`:

```bash
PR_KEY="${REPO}#${NUM}"
SNAP=$(gh pr view "$NUM" -R "$REPO" --json \
  headRefOid,headRefName,baseRefName,state,isDraft,mergeable,mergeStateStatus,reviewDecision,statusCheckRollup,labels)
HEAD_SHA=$(echo "$SNAP" | jq -r '.headRefOid')
HEAD_REF=$(echo "$SNAP" | jq -r '.headRefName')
BASE_REF=$(echo "$SNAP" | jq -r '.baseRefName')
STATE=$(echo "$SNAP" | jq -r '.state')
IS_DRAFT=$(echo "$SNAP" | jq -r '.isDraft')
MERGE_STATE=$(echo "$SNAP" | jq -r '.mergeStateStatus')
MERGEABLE=$(echo "$SNAP" | jq -r '.mergeable')
REVIEW_DECISION=$(echo "$SNAP" | jq -r '.reviewDecision')
BEHIND_BY=$(gh api "repos/$REPO/compare/$BASE_REF...$HEAD_SHA" \
            --jq '.behind_by // 0' 2>/dev/null || echo 0)
```

If `STATE != "OPEN"`: `recall ent del pr "$PR_KEY" 2>/dev/null`, continue.

Then run **head_sha rollover** as above.

#### 2a. Resolve origin workspace

Look up the cmux workspace that opened the PR. Order: cached → branch-prefix
→ parking workspace.

```bash
TARGET_WS=$(recall ent get pr "$PR_KEY" 2>/dev/null \
            | jq -r '.Attrs.origin_workspace_ref // ""')

if [ -z "$TARGET_WS" ]; then
  # Branch-prefix lookup: which live workspace's cwd is on this head ref?
  TARGET_WS=$(cmux list --json 2>/dev/null \
              | jq -r --arg ref "$HEAD_REF" '
                  .[] | select(.cwd != null) |
                  select(.git_branch == $ref) | .ref' \
              | head -1)
fi

if [ -z "$TARGET_WS" ]; then
  TARGET_WS="parking:pr-triage"
fi

# Cache resolved mapping for future cycles.
recall ent put pr "$PR_KEY" \
  head_sha="$HEAD_SHA" state=open \
  origin_workspace_ref="$TARGET_WS" origin_branch="$HEAD_REF" \
  last_seen="$(date -u +%FT%TZ)" >/dev/null
```

If exactly one workspace matches in the branch-prefix step, that's the
origin. If `cmux list` returns >1 match, skip the branch-prefix path and
fall through to parking — surface ambiguity in the Phase 5 report.

#### 2b. Detect events (the diff)

For each event kind, compute whether the **current** GH state warrants
emitting a `pending` event at this `head_sha`. Each emit is keyed by
`<kind>:<head_sha>:<discriminator>` so it's idempotent across cycles.

##### new_review_comment

```bash
THREADS=$(gh api graphql -f query='
  query($owner:String!,$repo:String!,$num:Int!){
    repository(owner:$owner,name:$repo){
      pullRequest(number:$num){
        reviewThreads(first:50){nodes{
          id isResolved
          comments(first:20){nodes{
            databaseId author{login} body createdAt
          }}
        }}
      }
    }
  }' -f owner="${REPO%%/*}" -f repo="${REPO##*/}" -F num=$NUM \
  | jq '[.data.repository.pullRequest.reviewThreads.nodes[]
         | select(.isResolved | not)
         | .comments.nodes[0] as $first
         | select($first.author.login != "'$ME'")
         | select($first.author.login != "jj-monitor")
         | {comment_id: $first.databaseId, author: $first.author.login, created_at: $first.createdAt}]')

echo "$THREADS" | jq -c '.[]' | while read -r T; do
  CID=$(echo "$T" | jq -r '.comment_id')
  EID="new_review_comment:${HEAD_SHA}:${CID}"
  emit_event "$PR_KEY" "$EID" new_review_comment \
    "$(echo "$T" | jq -c .)"
done
```

##### ci_failure

A check failing on the PR but NOT failing on `main` of the repo (i.e. not
systemic) emits one event per check name. Checks failing on both are
skipped silently.

```bash
MAIN_FAILED=$(gh api "repos/$REPO/commits/main/check-runs" \
  --jq '[.check_runs[] | select(.conclusion == "failure") | .name]' 2>/dev/null \
  || echo '[]')

echo "$SNAP" | jq -r --argjson main "$MAIN_FAILED" '
    .statusCheckRollup[]? |
    select(.conclusion == "FAILURE") |
    select(.name as $n | $main | index($n) | not) |
    .name' | while read -r CHK; do
  [ -z "$CHK" ] && continue
  EID="ci_failure:${HEAD_SHA}:${CHK}"
  emit_event "$PR_KEY" "$EID" ci_failure \
    "$(jq -nc --arg name "$CHK" '{check_name:$name}')"
done

# Record per-check state for transition tracking.
echo "$SNAP" | jq -r '.statusCheckRollup[]? | "\(.name)\t\(.conclusion // .status // "unknown" | ascii_downcase)"' \
  | while IFS=$'\t' read -r CHK STATUS; do
    case "$STATUS" in
      success|failure|skipped|neutral|cancelled) ST="$STATUS" ;;
      in_progress|queued)                        ST="running" ;;
      *) ST="unknown" ;;
    esac
    recall child put pr "$PR_KEY" checks "$CHK" --status="$ST" >/dev/null
  done
```

##### merge_conflict

```bash
if [ "$MERGE_STATE" = "DIRTY" ] || [ "$MERGE_STATE" = "CONFLICTING" ] \
   || { [ "$BEHIND_BY" -gt 0 ] && [ "$MERGE_STATE" = "BEHIND" ]; }; then
  EID="merge_conflict:${HEAD_SHA}"
  emit_event "$PR_KEY" "$EID" merge_conflict \
    "$(jq -nc --arg ms "$MERGE_STATE" --argjson b "$BEHIND_BY" \
        '{merge_state:$ms, behind_by:$b}')"
fi
```

##### ready_to_merge

All conditions:
1. `STATE == OPEN`, `IS_DRAFT == false`
2. `REVIEW_DECISION == APPROVED`
3. `MERGEABLE == MERGEABLE`
4. No `FAILURE` checks (PENDING is OK — skip event, wait for green)

```bash
FAILED_CHECKS=$(echo "$SNAP" | jq '[.statusCheckRollup[]? | select(.conclusion == "FAILURE")] | length')
PENDING_CHECKS=$(echo "$SNAP" | jq '[.statusCheckRollup[]? | select(.status == "IN_PROGRESS" or .status == "QUEUED")] | length')

if [ "$IS_DRAFT" = "false" ] \
   && [ "$REVIEW_DECISION" = "APPROVED" ] \
   && [ "$MERGEABLE" = "MERGEABLE" ] \
   && [ "$FAILED_CHECKS" = "0" ] \
   && [ "$PENDING_CHECKS" = "0" ]; then
  EID="ready_to_merge:${HEAD_SHA}"
  emit_event "$PR_KEY" "$EID" ready_to_merge \
    "$(jq -nc --arg url "$URL" '{url:$url}')"
fi
```

#### 2c. emit_event helper

```bash
emit_event() {
  local pr_key="$1" eid="$2" kind="$3" payload="$4"
  local existing
  existing=$(recall child get pr "$pr_key" events "$eid" 2>/dev/null \
             | jq -r '.Status // ""')
  case "$existing" in
    pending|delivered|acked|superseded) return 0 ;;
  esac
  recall child put pr "$pr_key" events "$eid" --status=pending \
    kind="$kind" \
    payload_json="$payload" \
    created_at="$(date -u +%FT%TZ)" \
    target_workspace="$TARGET_WS" \
    >/dev/null
}
```

#### 2d. Deliver pending events to the resolved workspace

For every `pending` event on this PR (whether emitted this cycle or a
prior one), deliver to `TARGET_WS` and flip to `delivered`. Three layered
channels — recall is truth, cmux paint is the visible projection,
terminal-notifier fires only for `ready_to_merge`.

```bash
recall child list pr "$PR_KEY" events --status=pending 2>/dev/null \
  | jq -r '.[].ID' | while read -r EID; do
  ROW=$(recall child get pr "$PR_KEY" events "$EID")
  KIND=$(echo "$ROW" | jq -r '.Attrs.kind')

  # 5.2 — cmux paint (idempotent; phase namespace pr-event:* so it doesn't
  # clobber the worker's own phase if any).
  if [ "$TARGET_WS" != "parking:pr-triage" ]; then
    cmux color    "$TARGET_WS" Rose                              >/dev/null 2>&1 || true
    cmux progress "$TARGET_WS" clear                             >/dev/null 2>&1 || true
    cmux status   "$TARGET_WS" "phase=pr-event icon=git-pull-request label=PR#$NUM" \
      >/dev/null 2>&1 || true
  fi

  # 5.3 — terminal-notifier for ready_to_merge only (gated by the existing
  # ready-to-merge notifications key, kept verbatim for cross-task idempotency).
  if [ "$KIND" = "ready_to_merge" ]; then
    SENT=$(recall child get pr "$PR_KEY" notifications ready-to-merge 2>/dev/null \
           | jq -r '.Status // ""')
    if [ "$SENT" != "sent" ]; then
      timeout 30 terminal-notifier \
        -title "PR #$NUM ready to merge" \
        -subtitle "anvil → $TARGET_WS" \
        -message "$REPO — approved, CI green, no conflicts. Click to open." \
        -open "$URL" \
        -sound Glass \
        -group "my-pr-$REPO-$NUM" >/dev/null 2>&1 || true
      recall child put pr "$PR_KEY" notifications ready-to-merge --status=sent \
        >/dev/null 2>&1 || true
    fi
  fi

  recall child put pr "$PR_KEY" events "$EID" --status=delivered \
    delivered_at="$(date -u +%FT%TZ)" >/dev/null
done
```

#### 2e. Re-paint stale events (no re-fire)

For events already `delivered` but not yet `acked`, re-paint the cmux pill
(idempotent — same Rose, same status pill) so a workspace coming out of
sleep / cmux restart still sees the signal. Do NOT re-fire
`terminal-notifier`.

```bash
recall child list pr "$PR_KEY" events --status=delivered 2>/dev/null \
  | jq -r '.[].ID' | while read -r EID; do
  if [ "$TARGET_WS" != "parking:pr-triage" ]; then
    cmux color    "$TARGET_WS" Rose >/dev/null 2>&1 || true
    cmux progress "$TARGET_WS" clear >/dev/null 2>&1 || true
    cmux status   "$TARGET_WS" "phase=pr-event icon=git-pull-request label=PR#$NUM" \
      >/dev/null 2>&1 || true
  fi
done
```

#### 2f. Per-PR end

Update `recall ent pr "$PR_KEY"` with `last_seen=<utc-iso>` so cleanup can
find PRs that GH has closed but we haven't observed yet.

### Phase 3 — cleanup

```bash
recall ent list pr 2>/dev/null | jq -r '.[].ID' | while read -r KEY; do
  R=${KEY%%#*}; N=${KEY##*#}
  S=$(gh pr view "$N" -R "$R" --json state --jq '.state' 2>/dev/null || echo "MISSING")
  if [ "$S" != "OPEN" ]; then
    recall child supersede pr "$KEY" events
    recall ent del pr "$KEY" >/dev/null
  fi
done
```

### Phase 4 — report

≤ 10 lines on stdout:

- PRs scanned: N
- events emitted (by kind): list
- events delivered: list of `(repo#num, kind, target_ws)`
- ambiguous origin mappings: list of `repo#num` (>1 workspace match)
- parked events: list of `repo#num` (no workspace match)
- ready-to-merge notifications fired: list
- errors: list

---

## Hard rules (preserved from sibling task; sharpened for redirect)

- **Never merge any PR.** Ever.
- **No `git` invocations.** This task does not invoke git directly. The
  only repo data it reads comes through `gh` / `gh api`.
- **No worktrees, no `yarn install`, no typecheck.** Those are workspace
  concerns now.
- **No PR comments, no review replies, no `gh pr ready`, no
  `gh api .../update-branch`, no `gh api .../requested_reviewers`,
  no force-push, no API merge.** This task is silent on GitHub.
- **No filesystem state outside recall.** `/tmp/my-pr-monitor-discover.prs.json`
  is ephemeral scratch only.
- **`terminal-notifier` fires only for `ready_to_merge`.** All other event
  kinds visible signal is the cmux pill.
- **Never re-fire `terminal-notifier` for the same `head_sha`.** Gated by
  `recall child get ... notifications ready-to-merge --status=sent`.
- **PrimerAI org filter retained.** Skip non-PrimerAI PRs with zero side
  effects.
- **Anvil never escalates.** No "workspace didn't ack in N cycles, anvil
  takes the fix" path. The escalation surface is the cmux pill staying
  Rose and the Phase 4 report — the human is the escalator.
- **Never touch coordinator-owned cmux fields.** Workspace `name` and
  `description` are owned by `cmux-coordinator`. This task only writes
  `color`, `progress`, and `status` (the pill), and only on the resolved
  `TARGET_WS` — never on the coordinator workspace, never on a sibling
  worker, never on the parking workspace's own pill.
- **Idempotent everywhere.** `recall child put` with the same status is a
  no-op; `cmux color/progress/status` with the same value is a no-op;
  `emit_event` checks existing status before writing.
- **`pending → delivered` is anvil's only transition.** `delivered → acked`
  is the workspace's job. `pending → superseded` happens automatically on
  head_sha rollover.

## Signature

This task does not post comments, so it does not sign anything. The
provenance signature `[ƒ:kx7]` is retired from this loop; replies happen
under the originating workspace's persona.
