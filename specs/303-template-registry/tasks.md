# Tasks: Template Registry for Shared Templates

**Input**: Design documents from `/specs/303-template-registry/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Not explicitly requested in the feature specification. Test tasks are omitted.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: No new project initialization needed. This feature extends an existing Go CLI project.

- [ ] T001 Create registry cache directory structure helper in `internal/project/registry.go` with `RegistrySearchResult`, `TemplateManifest`, `RegistryMeta`, and `SearchCache` structs per data-model.md

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core registry client infrastructure that all user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T002 Implement GitHub API search client function `SearchRegistry(query string) ([]RegistrySearchResult, error)` in `internal/project/registry.go` that queries GitHub API for repos with `anvil-template` topic matching the query, with response parsing and error handling
- [ ] T003 Implement manifest fetching function `FetchManifest(owner, repo string) (*TemplateManifest, error)` in `internal/project/registry.go` that downloads and parses `anvil-template.yaml` from a GitHub repository's default branch
- [ ] T004 Implement search result caching in `internal/project/registry.go` with `getCachedSearch(query string)` and `cacheSearchResults(query string, results []RegistrySearchResult)` using `~/.anvil/cache/registry/search-<hash>.json` with 1-hour TTL
- [ ] T005 Implement opportunistic GitHub auth token detection in `internal/project/registry.go` — check for `gh` CLI auth token to increase API rate limits from 60 to 5000 req/hour

**Checkpoint**: Registry client ready — user story implementation can begin

---

## Phase 3: User Story 1 - Search for Templates (Priority: P1) MVP

**Goal**: Users can discover templates by keyword search via `anvil template search`

**Independent Test**: Run `anvil template search <keyword>` and verify matching results with name, description, and author are displayed

### Implementation for User Story 1

- [ ] T006 [US1] Rewrite `templateSearchCmd` in `cmd/anvil/template_search.go` to query the registry via `SearchRegistry()` instead of only searching local templates; display results with name, description, author, version, and star count per CLI contract
- [ ] T007 [US1] Add registry error handling in `cmd/anvil/template_search.go` — display clear "registry unavailable" message on network errors with hint to check connectivity
- [ ] T008 [US1] Update `templateCmd` in `cmd/anvil/template.go` to show updated help text reflecting registry search capabilities

**Checkpoint**: `anvil template search <query>` works end-to-end against GitHub registry

---

## Phase 4: User Story 2 - Install a Template (Priority: P1) MVP

**Goal**: Users can install templates from the registry with `anvil template install <owner/repo>`

**Independent Test**: Run `anvil template install <owner/repo>` and verify template files appear in `.anvil/templates/` with registry metadata sidecar

### Implementation for User Story 2

- [ ] T009 [US2] Implement `DownloadTemplate(owner, repo string, manifest *TemplateManifest, destDir string) error` in `internal/project/registry.go` that downloads template files listed in the manifest and saves them to the destination directory
- [ ] T010 [US2] Implement `WriteRegistryMeta(destDir, templateName, source, version string) error` in `internal/project/registry.go` that writes `.registry-meta.yaml` sidecar file alongside installed templates
- [ ] T011 [US2] Implement version compatibility check function `CheckCompatibility(manifest *TemplateManifest, currentVersion string) (bool, string)` in `internal/project/registry.go` that compares `min_anvil_version` against the running anvil version
- [ ] T012 [US2] Add `templateInstallCmd` function in `cmd/anvil/template_registry.go` that orchestrates: parse owner/repo arg, fetch manifest, check compatibility (warn if incompatible), check for existing local template (prompt or --force), download files, write registry meta, display success message
- [ ] T013 [US2] Update `templateCmd` in `cmd/anvil/template.go` to add "install" subcommand routing to `templateInstallCmd`

**Checkpoint**: `anvil template install <owner/repo>` works end-to-end with overwrite protection and version warnings

---

## Phase 5: User Story 3 - View Template Details (Priority: P2)

**Goal**: Users can inspect template metadata before installing via `anvil template info <owner/repo>`

**Independent Test**: Run `anvil template info <owner/repo>` and verify detailed metadata (name, author, version, description, files, tags, compatibility) is displayed

### Implementation for User Story 3

- [ ] T014 [US3] Add `templateInfoCmd` function in `cmd/anvil/template_registry.go` that fetches manifest via `FetchManifest()` and displays formatted template details per CLI contract (name, author, version, source URL, description, files, requirements, tags)
- [ ] T015 [US3] Update `templateCmd` in `cmd/anvil/template.go` to add "info" subcommand routing to `templateInfoCmd`

**Checkpoint**: `anvil template info <owner/repo>` displays detailed template information

---

## Phase 6: User Story 4 - List Installed Templates (Priority: P3)

**Goal**: Users can see which registry templates are installed in their project

**Independent Test**: Install a template, then run `anvil template list --installed` and verify it appears with source and version

### Implementation for User Story 4

- [ ] T016 [US4] Implement `ListInstalledRegistryTemplates(projectPath string) ([]RegistryMeta, error)` in `internal/project/registry.go` that scans `.anvil/templates/` for `.registry-meta.yaml` sidecar files and returns parsed metadata
- [ ] T017 [US4] Add `--installed` flag handling to `templateListCmd` in `cmd/anvil/template.go` that calls `ListInstalledRegistryTemplates()` and displays source, version, and install date per CLI contract
- [ ] T018 [US4] Update help text in `templateCmd` in `cmd/anvil/template.go` to document the `--installed` flag for `ls` subcommand

**Checkpoint**: `anvil template ls --installed` shows registry-installed templates with metadata

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T019 Validate manifest schema in `FetchManifest()` in `internal/project/registry.go` — enforce name regex, description max length, version semver, files non-empty per manifest-schema.md contract
- [ ] T020 Add user-agent header to all GitHub API requests in `internal/project/registry.go` identifying the anvil CLI and version

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (T001) — BLOCKS all user stories
- **US1 Search (Phase 3)**: Depends on Phase 2 (T002, T004)
- **US2 Install (Phase 4)**: Depends on Phase 2 (T003)
- **US3 Info (Phase 5)**: Depends on Phase 2 (T003)
- **US4 List Installed (Phase 6)**: Depends on Phase 2 (T001 structs)
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Depends on Foundational — no dependencies on other stories
- **User Story 2 (P1)**: Depends on Foundational — no dependencies on other stories
- **User Story 3 (P2)**: Depends on Foundational — no dependencies on other stories
- **User Story 4 (P3)**: Depends on Foundational — no dependencies on other stories

### Parallel Opportunities

- T002, T003, T004, T005 can all run in parallel (Phase 2, different functions)
- US1, US2, US3, US4 can all run in parallel after Phase 2 completes (different files/subcommands)
- T014 and T016 can run in parallel (different subcommands, different files)

---

## Parallel Example: Phase 2 (Foundational)

```bash
# All foundational tasks target different functions in the same file but are independent:
Task: "Implement SearchRegistry() in internal/project/registry.go"
Task: "Implement FetchManifest() in internal/project/registry.go"
Task: "Implement search caching in internal/project/registry.go"
Task: "Implement GitHub auth token detection in internal/project/registry.go"
```

## Parallel Example: User Stories after Phase 2

```bash
# All user stories can proceed in parallel:
Task: "US1 - Rewrite templateSearchCmd in cmd/anvil/template_search.go"
Task: "US2 - Add templateInstallCmd in cmd/anvil/template_registry.go"
Task: "US3 - Add templateInfoCmd in cmd/anvil/template_registry.go"
Task: "US4 - Add ListInstalledRegistryTemplates in internal/project/registry.go"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup (T001)
2. Complete Phase 2: Foundational (T002-T005)
3. Complete Phase 3: US1 Search (T006-T008)
4. Complete Phase 4: US2 Install (T009-T013)
5. **STOP and VALIDATE**: Search and install work end-to-end
6. Deploy if ready — users can discover and install templates

### Incremental Delivery

1. Setup + Foundational → Registry client ready
2. Add US1 Search → Users can discover templates (partial MVP)
3. Add US2 Install → Users can install templates (full MVP!)
4. Add US3 Info → Users can inspect before installing
5. Add US4 List → Users can audit installed templates
6. Polish → Manifest validation, user-agent headers

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Existing template code in `cmd/anvil/template*.go` and `internal/project/project.go` is modified, not replaced
