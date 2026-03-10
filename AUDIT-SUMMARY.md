# Audit: Rate limiting — per-provider and global

## Summary

Completed audit of Anvil's rate limiting implementation. Found that while global and task-level rate limiting is fully functional, per-provider rate limiting is not implemented.

## Current Implementation

✅ **Working Features:**
- Global throttling (pause/resume, rate per minute)
- Task-level rate limiting (per hour, per day)
- Concurrency group rate limiting
- CLI commands for all rate limiting functions
- Persistent state storage

❌ **Missing Features:**
- Provider-specific rate limiting
- Multi-provider support
- Provider quota tracking

## Technical Details

The system is designed around Claude (Anthropic) as the primary LLM provider:
- References to `ANTHROPIC_API_KEY` and Claude runners
- No evidence of OpenAI, Google, or Azure provider support
- No provider identification mechanism in rate limiting logic

## Recommendations

To implement per-provider rate limiting:

1. Add provider identification to tasks
2. Implement provider-specific quota tracking
3. Add multi-provider configuration support
4. Extend throttling system for per-provider enforcement

## Status

All existing rate limiting tests pass. The feature requires significant architectural changes to support multiple providers and per-provider quotas.

## Next Steps

Issue #394 should be split into phases:
1. Implement provider identification and tracking
2. Add multi-provider support and per-provider rate limiting