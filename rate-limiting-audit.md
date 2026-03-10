# Rate Limiting Audit for Anvil

## Overview

This document summarizes the current rate limiting implementation in Anvil and identifies gaps related to per-provider rate limiting as mentioned in issue #394.

## Current Rate Limiting Implementation

### 1. Global Throttling (throttle.go/throttle_test.go)
- **Global pause/resume**: Ability to pause all task execution or specific label-based task groups
- **Rate per minute**: Global rate limiting measured in tasks per minute (e.g., 5/m)
- **Persistence**: State is stored in `~/.anvil/throttle.json` and survives daemon restarts
- **CLI commands**:
  - `anvil pause [--label <label>]` - Pause globally or by label
  - `anvil resume [--label <label>]` - Resume globally or by label
  - `anvil throttle --rate N/m | --off` - Set global rate limit or disable it

### 2. Task-Level Rate Limiting (project/project.go)
- **Per-task configuration**: Each task can define:
  - `MaxPerHour`: maximum executions per hour (0 = unlimited)
  - `MaxPerDay`: maximum executions per day (0 = unlimited)
- **Tracking**: Execution counts are tracked and persisted per task
- **CLI command**: `anvil task rate-limits` to view current usage

### 3. Concurrency Group Rate Limiting (config/config.go)
- **Group-level configuration**: Concurrency groups can define:
  - `RequestsPerMinute`: max API requests per minute (0 = no limit)
  - `TokenRateLimit`: max tokens per minute (0 = no limit)

### 4. API Call Rate Limiting (daemon.go)
- **Semaphore-based limiting**: A semaphore (`rateLimitSemaphore`) can limit concurrent API calls
- **Not currently configured**: The semaphore is not initialized based on any configuration

### 5. Cost-Based Rate Limiting (config/config.go)
- **Token rate configuration**: Configurable input/output token rates for cost estimation
- **Token rate limiting**: Groups can define token rate limits (tokens per minute)

## Missing Provider-Specific Rate Limiting

After auditing the codebase, there is no implementation of provider-specific rate limiting. The current system:

1. **Is Claude/Anthropic-focused**: The system is designed primarily for Claude (Anthropic) with references to `ANTHROPIC_API_KEY`
2. **Has no multi-provider support**: No evidence of support for OpenAI, Google Gemini, or other providers
3. **Has no provider identification**: No mechanism to identify which API provider a task uses
4. **Has no per-provider quota tracking**: No tracking or enforcement of quotas per API provider

## Recommendations

To implement per-provider rate limiting, the following changes would be needed:

1. **Configuration**: Add provider-specific rate limit configurations
2. **Provider identification**: Mechanism to identify which provider each task uses
3. **Tracking**: Implement tracking mechanisms for each provider's usage
4. **Enforcement**: Add logic to check provider quotas before dispatching tasks
5. **Integration**: Connect with existing throttling mechanisms

## Status

- ✅ Global rate limiting: Working
- ✅ Task-level rate limiting: Working
- ✅ Concurrency group rate limiting: Configurable but not enforced
- ✅ Cost-based rate limiting: Configurable but not enforced
- ❌ Per-provider rate limiting: Not implemented
- ❌ Multi-provider support: Not implemented
- ❌ Provider-specific quota tracking: Not implemented

## Next Steps

Based on issue #394 requirements, we need to:
1. Design and implement provider-specific rate limiting
2. Add configuration options for per-provider quotas
3. Integrate with existing throttling mechanisms
4. Consider supporting multiple API providers beyond Claude/Anthropic