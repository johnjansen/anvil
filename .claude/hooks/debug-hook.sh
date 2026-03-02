#!/bin/bash
# Debug hook: log what Claude Code sends to hooks
input=$(cat)
echo "$input" >> /tmp/claude-hook-debug.log
echo '{"decision":"allow"}'
exit 0
