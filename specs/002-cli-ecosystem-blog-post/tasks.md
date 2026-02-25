# Tasks: CLI Ecosystem Blog Post

**Input**: Design documents from `/specs/002-cli-ecosystem-blog-post/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, quickstart.md

**Tests**: Not requested in feature specification. No test tasks included.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the blog directory and initial file structure

- [x] T001 Create `blog/` directory at repository root and empty `blog/cli-ecosystem-blog-post.md` with front matter (title, date, tags placeholder)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Verify tool interfaces so all CLI examples in the blog post are accurate

**⚠️ CRITICAL**: Blog content depends on accurate CLI examples. Verify before writing.

- [x] T002 Verify anvil CLI examples match current interface by running `anvil --help`, `anvil task --help`, and `anvil add --help`; update research.md R1 if any commands have changed
- [x] T003 [P] Locate beads project documentation or repository (check `/Users/johnjansen/Documents/GitHub/beads`, GitHub, or ask the user) and verify CLI interface; update research.md R2 with accurate commands
- [x] T004 [P] Verify speckit workflow commands by reviewing `.specify/` scripts and templates; confirm `/speckit.specify`, `/speckit.plan`, `/speckit.tasks`, `/speckit.implement` are accurate; update research.md R3 if needed

**Checkpoint**: All three tool interfaces verified. Blog writing can begin.

---

## Phase 3: User Story 1 - Developer Discovers the Ecosystem (Priority: P1) 🎯 MVP

**Goal**: The opening hook and philosophy sections. A reader immediately understands the problem (context switching, tool sprawl, flow state disruption) and the proposed alternative (small, composable CLI tools).

**Independent Test**: Have a developer unfamiliar with the tools read only the introduction and philosophy sections. They should articulate the problem and the proposed alternative.

### Implementation for User Story 1

- [x] T005 [US1] Write the opening hook section (~200 words) in `blog/cli-ecosystem-blog-post.md`. Articulate the problem: fragmented workflows, context switching between planning and coding tools, loss of flow state. Use developer-to-developer tone (FR-001, FR-005). Do NOT mention proprietary product names — use generic terms like "heavyweight planning tools" (FR-004).
- [x] T006 [US1] Write the Unix philosophy section (~200 words) in `blog/cli-ecosystem-blog-post.md`. Frame the ecosystem around small, composable, text-based tools that do one thing well (FR-006). Briefly acknowledge GUI tools exist without being dismissive (edge case).

**Checkpoint**: Introduction + philosophy complete. A reader should understand the "why" without knowing any tool names yet.

---

## Phase 4: User Story 2 - Reader Understands Each Tool's Role (Priority: P1)

**Goal**: Three distinct tool description sections, each with a concrete CLI example. The reader can describe each tool's purpose in one sentence after reading.

**Independent Test**: Ask a reader to describe each tool's purpose in one sentence after reading the relevant sections.

### Implementation for User Story 2

- [x] T007 [P] [US2] Write the speckit section (~200 words) in `blog/cli-ecosystem-blog-post.md`. Describe speckit as a specification-driven development workflow. Include CLI example showing the specify → plan → tasks → implement pipeline. Use verified commands from T004. Explain that specs live as markdown in `specs/` directories (FR-002).
- [x] T008 [P] [US2] Write the beads section (~200 words) in `blog/cli-ecosystem-blog-post.md`. Describe beads as a lightweight, git-native issue tracker. Include CLI example showing issue creation, listing, and closing. Use verified commands from T003. Emphasize zero-infrastructure, issues alongside code (FR-002).
- [x] T009 [P] [US2] Write the anvil section (~200 words) in `blog/cli-ecosystem-blog-post.md`. Describe anvil as a scheduled task automation framework. Include CLI example showing `anvil watch`, `anvil add` with cron schedule, `anvil task ls`. Use verified commands from T002. Explain daemon model and plain-English tasks (FR-002).

**Checkpoint**: All three tool sections complete. Each has a distinct identity and a concrete CLI example. Reader should not confuse the tools with each other.

---

## Phase 5: User Story 3 - Reader Sees the Composed Workflow (Priority: P1)

**Goal**: An end-to-end walkthrough showing all three tools used together on a single feature, from idea to completion. This is the thesis of the post.

**Independent Test**: Have a reader trace the workflow and identify which tool handles which phase and how they hand off.

### Implementation for User Story 3

- [x] T010 [US3] Write the composed workflow section (~500 words) in `blog/cli-ecosystem-blog-post.md`. Walk through a single concrete feature (e.g., "add user authentication") showing: (1) speckit generates a spec and plan, (2) beads tracks the issues that emerge, (3) anvil automates recurring tasks during development. Show how data flows between tools and the workflow stays in the terminal (FR-003).
- [x] T011 [US3] Write the "everything lives in git" section (~200 words) in `blog/cli-ecosystem-blog-post.md`. Explain that specs live in `specs/`, issues alongside code, tasks in `.anvil/todos/` — all committed to the repository. Contrast with SaaS tools where planning data is locked in a separate system (FR-009).

**Checkpoint**: The composed workflow and version-control sections complete. A reader can trace a feature from idea to execution using all three tools.

---

## Phase 6: User Story 4 - Reader Adopts Incrementally (Priority: P2)

**Goal**: The reader understands each tool stands alone and can be adopted one at a time. The post ends with a clear call to action.

**Independent Test**: Ask a reader which single tool they'd try first and why.

### Implementation for User Story 4

- [x] T012 [US4] Write the incremental adoption section (~200 words) in `blog/cli-ecosystem-blog-post.md`. Make clear that each tool is independently useful: you can use beads without speckit, anvil without beads, etc. Suggest a starting point based on the reader's biggest pain point (FR-007).
- [x] T013 [US4] Write the call to action section (~100 words) in `blog/cli-ecosystem-blog-post.md`. End with a clear, non-pushy invitation to try one tool. Include links to repos/documentation for all three tools. No marketing language (FR-005, FR-010).

**Checkpoint**: All user story content complete. Full blog post draft exists.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Validate the complete blog post against all requirements and success criteria

- [x] T014 Run word count check on `blog/cli-ecosystem-blog-post.md` — must be 1,500-2,500 words (FR-008, SC-004). If outside range, trim or expand content proportionally across sections.
- [x] T015 [P] Run proprietary name check: search `blog/cli-ecosystem-blog-post.md` for company/product names (Jira, Linear, Notion, Asana, Trello, Monday, ClickUp, Shortcut, Basecamp, etc.). Replace any matches with generic terms (FR-004, SC-003).
- [x] T016 [P] Verify structural completeness against quickstart.md checklist in `specs/002-cli-ecosystem-blog-post/quickstart.md` — confirm all 9 required sections exist.
- [x] T017 Review full blog post for tone consistency: developer-to-developer throughout, no marketing language, no condescension about GUI tools (FR-005).
- [x] T018 Verify all CLI examples in the post match the verified interfaces from Phase 2 (T002-T004). Fix any drift introduced during writing.
- [x] T019 Add or verify blog post title — must be descriptive, SEO-friendly, no proprietary names. Finalize any front matter (tags, description) for the target platform.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all writing tasks
- **User Stories (Phase 3-6)**: All depend on Foundational phase completion (verified CLI interfaces)
  - US1 (Phase 3) should be written first — it sets the opening tone
  - US2 (Phase 4) depends on US1 — tool sections follow the opening narrative
  - US3 (Phase 5) depends on US2 — composed workflow references individual tool sections
  - US4 (Phase 6) depends on US3 — adoption/CTA wraps up the narrative
- **Polish (Phase 7)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) — sets the opening
- **User Story 2 (P1)**: Depends on US1 — tool sections build on the problem statement. T007/T008/T009 can run in parallel with each other.
- **User Story 3 (P1)**: Depends on US2 — workflow section references individual tool descriptions
- **User Story 4 (P2)**: Depends on US3 — adoption section wraps the narrative after the workflow demo

### Within Each User Story

- Sections within a story are sequential (narrative flow) unless marked [P]
- US2 has three parallel tool sections (T007, T008, T009 — different content blocks)

### Parallel Opportunities

- T002, T003, T004 can all run in parallel (verifying different tools)
- T007, T008, T009 can all run in parallel (writing independent tool sections)
- T015, T016 can run in parallel with each other (independent validation checks)

---

## Parallel Example: User Story 2

```bash
# Launch all tool description sections together (different content blocks, same file but different sections):
Task: "Write the speckit section (~200 words) in blog/cli-ecosystem-blog-post.md"
Task: "Write the beads section (~200 words) in blog/cli-ecosystem-blog-post.md"
Task: "Write the anvil section (~200 words) in blog/cli-ecosystem-blog-post.md"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (create blog directory and file)
2. Complete Phase 2: Foundational (verify all CLI interfaces)
3. Complete Phase 3: User Story 1 (opening hook + philosophy)
4. **STOP and VALIDATE**: Does a reader understand the problem and the alternative?
5. Continue if validated

### Incremental Delivery

1. Complete Setup + Foundational → Blog file ready, CLI interfaces verified
2. Add User Story 1 → Opening hook and philosophy → Validate independently
3. Add User Story 2 → Individual tool sections with CLI examples → Validate independently
4. Add User Story 3 → Composed workflow + git section → Validate independently
5. Add User Story 4 → Incremental adoption + CTA → Validate independently
6. Polish → Word count, proprietary check, tone review → Final validation

### Note on Sequential Writing

Unlike code projects, blog posts have a strong narrative arc. While the tool sections (US2) can be written in parallel, the overall story phases should be completed sequentially to maintain coherent voice and flow. Each phase builds on the narrative established by the previous one.

---

## Notes

- [P] tasks = different content blocks, no narrative dependencies
- [Story] label maps task to specific user story for traceability
- Each user story checkpoint should be independently testable per the spec's acceptance scenarios
- Commit after each phase completion
- The single deliverable file is `blog/cli-ecosystem-blog-post.md`
- Action item from research: beads CLI interface needs verification (T003) before writing beads examples
