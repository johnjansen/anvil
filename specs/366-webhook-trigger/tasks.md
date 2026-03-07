# Tasks: Webhook Trigger for Tasks

**Input**: Design documents from `/specs/366-webhook-trigger/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Note**: This feature extends the existing `WebhookServer` implementation. Most infrastructure already exists — tasks focus on hardening and gap-filling.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Configuration infrastructure for webhook port

- [ ] T001 Add `WebhookPort` field (int, yaml `webhook_port`, default 9090) to Config struct in `internal/config/config.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core webhook server improvements that all user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T002 Update `NewWebhookServer` call in `internal/daemon/daemon.go` to use `cfg.WebhookPort` (or default 9090) instead of hardcoded `:8080`
- [ ] T003 Add `taskPaths` map (taskID -> path) and `running` bool fields to `WebhookServer` struct in `internal/daemon/webhook.go` for lifecycle tracking
- [ ] T004 Implement conditional server startup in `internal/daemon/daemon.go`: only call `webhookServer.Start()` after `startSubscriptions()` if at least one webhook handler was registered (check `len(ws.handlers) > 0`)
- [ ] T005 Add `StopSubscription(taskID string)` method to `WebhookServer` in `internal/daemon/webhook.go` that removes a handler from the `taskPaths` and `handlers` maps, and stops the server if no handlers remain

**Checkpoint**: Foundation ready — webhook server has configurable port, conditional startup, and lifecycle management

---

## Phase 3: User Story 1 — Trigger Task via HTTP Webhook (Priority: P1) MVP

**Goal**: External systems can trigger tasks by sending POST requests to configured webhook endpoints

**Independent Test**: Create a task with webhook trigger, send POST to the endpoint, verify task executes

### Implementation for User Story 1

- [ ] T006 [US1] Add `http.MaxBytesReader` wrapper (1 MB limit) around `r.Body` in the handler function in `internal/daemon/webhook.go` before `io.ReadAll`, and return 413 if body exceeds limit
- [ ] T007 [US1] Add duplicate path detection in `StartSubscription` in `internal/daemon/webhook.go`: check `ws.handlers` map before registering, return error if path already exists, and populate `ws.taskPaths` map with taskID -> path
- [ ] T008 [US1] Update handler in `internal/daemon/webhook.go` to enforce POST-only by default (return 405 for non-POST) when `WebhookMethod` is empty, rather than allowing all methods
- [ ] T009 [US1] Add test cases in `internal/daemon/webhook_test.go`: POST triggers task, GET returns 405, unknown path returns 404, oversized body returns 413, duplicate path registration returns error

**Checkpoint**: User Story 1 complete — tasks can be triggered via HTTP POST with proper error handling

---

## Phase 4: User Story 2 — HMAC Secret Verification (Priority: P1)

**Goal**: Webhook requests are authenticated via HMAC-SHA256 signatures, rejecting unauthorized requests

**Independent Test**: Configure secret, send request with valid signature (accepted), send with invalid signature (rejected 401)

### Implementation for User Story 2

- [ ] T010 [US2] Add startup validation in `StartSubscription` in `internal/daemon/webhook.go`: when `WebhookSecret` starts with `env:`, check that the referenced environment variable is set; log error and skip registration if not
- [ ] T011 [US2] Add test cases in `internal/daemon/webhook_test.go`: valid HMAC signature accepted, invalid signature returns 401, missing signature returns 401, missing env var skips registration with error log, no secret configured allows all requests

**Checkpoint**: User Story 2 complete — HMAC verification works for secured webhook endpoints

---

## Phase 5: User Story 3 — Pass Webhook Payload to Task (Priority: P2)

**Goal**: Tasks receive the webhook payload and request metadata as environment variables

**Independent Test**: Send POST with JSON body and headers, verify task receives `ANVIL_WEBHOOK_PAYLOAD`, `ANVIL_WEBHOOK_CONTENT_TYPE`, `ANVIL_WEBHOOK_EVENT`, and `ANVIL_WEBHOOK_HEADERS`

### Implementation for User Story 3

- [ ] T012 [US3] Extend `triggerWebhookTask` in `internal/daemon/daemon.go` to set additional env vars: `ANVIL_WEBHOOK_CONTENT_TYPE` from `Content-Type` header, `ANVIL_WEBHOOK_EVENT` from `X-GitHub-Event` or `X-Webhook-Event` header, `ANVIL_WEBHOOK_HEADERS` as JSON-encoded map of all request headers
- [ ] T013 [US3] Update the handler in `internal/daemon/webhook.go` to pass the `*http.Request` (or extracted headers) to `triggerWebhookTask` so it has access to headers (update function signature to accept headers map)
- [ ] T014 [US3] Add test case in `internal/daemon/webhook_test.go`: send request with Content-Type and X-GitHub-Event headers, verify env vars are set correctly on the triggered task

**Checkpoint**: User Story 3 complete — tasks have full access to webhook payload and metadata

---

## Phase 6: User Story 4 — Configurable HTTP Server Port (Priority: P2)

**Goal**: Users can configure the webhook server port via config file

**Independent Test**: Set `webhook_port: 8080` in config, start daemon, verify server listens on 8080

### Implementation for User Story 4

- [ ] T015 [US4] Add test case in `internal/daemon/webhook_test.go`: create WebhookServer with custom port, verify it listens on that port; verify default port is 9090 when not configured

**Checkpoint**: User Story 4 complete — port is configurable (implementation done in T001/T002, this adds testing)

---

## Phase 7: User Story 5 — Server Lifecycle Management (Priority: P2)

**Goal**: HTTP server starts only when webhook tasks exist and stops when none remain

**Independent Test**: Start daemon with no webhook tasks (no server), add webhook task (server starts), remove all webhook tasks (server stops)

### Implementation for User Story 5

- [ ] T016 [US5] Implement dynamic server start in `StartSubscription` in `internal/daemon/webhook.go`: if `!ws.running` and a handler is being registered, call `ws.Start()` and set `ws.running = true`
- [ ] T017 [US5] Implement dynamic server stop in `StopSubscription` in `internal/daemon/webhook.go`: if `len(ws.handlers) == 0` and `ws.running`, call `ws.Stop()` and set `ws.running = false`
- [ ] T018 [US5] Add webhook server shutdown to daemon graceful shutdown in `internal/daemon/daemon.go`: call `ws.Stop(ctx)` during `gracefulShutdown` if `ws.running`
- [ ] T019 [US5] Add test cases in `internal/daemon/webhook_test.go`: verify server not started when no webhook tasks, verify server starts on first subscription, verify server stops when last subscription removed

**Checkpoint**: User Story 5 complete — server lifecycle is fully managed

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Error handling edge cases and logging improvements

- [ ] T020 Add structured log messages for all webhook server events (start, stop, route registration, route removal, request received, auth success/failure, body too large) in `internal/daemon/webhook.go`
- [ ] T021 Handle port-in-use error gracefully in `Start()` in `internal/daemon/webhook.go`: log error clearly, set `ws.running = false`, allow daemon to continue without webhook support
- [ ] T022 Run `go test ./internal/daemon/...` and verify all existing and new tests pass

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (T001)
- **User Stories (Phase 3-7)**: All depend on Phase 2 completion
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Can start after Phase 2 — no dependencies on other stories
- **US2 (P1)**: Can start after Phase 2 — no dependencies on other stories
- **US3 (P2)**: Can start after Phase 2 — no dependencies on other stories
- **US4 (P2)**: Mostly done in Phase 1/2 — just needs testing
- **US5 (P2)**: Depends on T005 from Phase 2 — no dependencies on other stories

### Parallel Opportunities

- US1 and US2 can run in parallel (different concerns: routing vs auth)
- US3 and US4 can run in parallel (different files/concerns)
- T006 and T007 can run in parallel within US1 (different concerns within webhook.go but different sections)

---

## Parallel Example: User Stories 1 & 2

```bash
# These can run in parallel after Phase 2:
Story 1: "T006-T009 — HTTP routing, body limits, duplicate detection"
Story 2: "T010-T011 — HMAC validation, env var checking"
```

---

## Implementation Strategy

### MVP First (User Stories 1 & 2)

1. Complete Phase 1: Config field (T001)
2. Complete Phase 2: Foundational (T002-T005)
3. Complete Phase 3: US1 — Basic webhook triggering (T006-T009)
4. Complete Phase 4: US2 — HMAC authentication (T010-T011)
5. **STOP and VALIDATE**: Test basic webhook trigger with and without HMAC

### Incremental Delivery

1. Setup + Foundational → Configurable, lifecycle-aware server
2. Add US1 + US2 → Secure webhook triggering (MVP!)
3. Add US3 → Rich payload/header passing
4. Add US4 → Port configuration testing
5. Add US5 → Dynamic lifecycle management
6. Polish → Logging, error handling, final test run

---

## Notes

- Existing `webhook.go` has ~180 lines of working code; most tasks are additions/modifications, not rewrites
- The `SubscriptionConfig` in `project.go` already has all needed webhook YAML fields — no changes needed there
- HMAC validation already works but needs startup env var checking (T010)
- `triggerWebhookTask` in `daemon.go` needs signature change to accept headers (T013)
