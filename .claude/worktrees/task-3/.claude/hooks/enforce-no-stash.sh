#!/bin/bash
# Block git stash in daemon tasks — agents must not touch the main tree's staged/unstaged changes.

input=$(cat)

permission_mode=$(echo "$input" | jq -r '.permission_mode // empty')
command=$(echo "$input" | jq -r '.tool_input.command // empty')

# Only enforce for daemon tasks
if [ "$permission_mode" != "bypassPermissions" ]; then
  echo '{"decision":"allow"}'
  exit 0
fi

# Block git stash
if echo "$command" | grep -qE '^git stash'; then
  cat <<EOF
{"decision":"deny","reason":"WORKTREE ISOLATION: 'git stash' is blocked. You must not touch the main working tree's staged or unstaged changes. Use 'git worktree add .anvil/worktrees/<name> -b <branch> main' to create an isolated workspace — this works even with a dirty main tree."}
EOF
  exit 2
fi

# Block git checkout -- (discarding changes in main tree)
if echo "$command" | grep -qE '^git checkout -- |^git restore '; then
  # Check if we're in a worktree (allow there)
  cwd=$(echo "$input" | jq -r '.cwd // empty')
  if echo "$cwd" | grep -q '/.anvil/worktrees/'; then
    echo '{"decision":"allow"}'
    exit 0
  fi
  cat <<EOF
{"decision":"deny","reason":"WORKTREE ISOLATION: Discarding changes in the main working tree is blocked. Work in your worktree instead."}
EOF
  exit 2
fi

echo '{"decision":"allow"}'
exit 0
