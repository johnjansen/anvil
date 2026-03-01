#!/bin/bash
# Enforce worktree isolation: block Write/Edit to files outside .anvil/worktrees/
# Only applies when running in bypass-permissions mode (daemon tasks).
# Interactive sessions are not affected.

input=$(cat)

permission_mode=$(echo "$input" | jq -r '.permission_mode // empty')
file_path=$(echo "$input" | jq -r '.tool_input.file_path // empty')

# Only enforce for daemon tasks (bypass permissions mode)
if [ "$permission_mode" != "bypassPermissions" ]; then
  echo '{"decision":"allow"}'
  exit 0
fi

# If no file path, allow (shouldn't happen for Write/Edit)
if [ -z "$file_path" ]; then
  echo '{"decision":"allow"}'
  exit 0
fi

# Allow writes inside worktrees
if echo "$file_path" | grep -q '/.anvil/worktrees/'; then
  echo '{"decision":"allow"}'
  exit 0
fi

# Allow writes to .anvil/todos/ (task self-modification if ever needed)
if echo "$file_path" | grep -q '/.anvil/todos/'; then
  echo '{"decision":"allow"}'
  exit 0
fi

# Block everything else
cat <<EOF
{"decision":"deny","reason":"WORKTREE ISOLATION: You can only write files inside your worktree (.anvil/worktrees/). You are trying to write to '$file_path' which is in the main working tree. Create a worktree first with 'git worktree add .anvil/worktrees/<name> -b <branch> main' and work there."}
EOF
exit 2
