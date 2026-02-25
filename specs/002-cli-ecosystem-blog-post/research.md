# Research: CLI Ecosystem Blog Post

**Feature**: 002-cli-ecosystem-blog-post
**Date**: 2026-02-25

## Research Tasks

### R1: Anvil — Accurate CLI interface and capabilities

**Decision**: Describe anvil as a scheduled LLM task automation framework written in Go, using cron-based scheduling with Claude as the executor.

**Rationale**: Based on direct codebase inspection. Anvil is a Go binary (Go 1.24.6) with minimal dependencies (only `gopkg.in/yaml.v3`). Tasks are plain markdown files with YAML frontmatter stored in `.anvil/todos/` directories organized by priority (p0-p9). A single daemon (`anvil watch`) monitors multiple projects.

**Key CLI examples for blog post**:
```bash
anvil watch                              # Start daemon (once per machine)
anvil init                               # Initialize project
anvil add -s "*/30 * * * *" "Triage GitHub issues"
anvil task ls                            # List tasks
anvil task run check-issues              # Trigger immediate execution
anvil task log check-issues              # View execution logs
```

**Alternatives considered**: Could describe anvil as a generic cron replacement, but that undersells it. The LLM integration (Claude executing plain-English tasks) is the differentiator.

---

### R2: Beads — Accurate CLI interface and capabilities

**Decision**: Describe beads as a lightweight, git-native issue tracker. The CLI binary is `bd` (short for beads). Verified locally — beads is installed at `/opt/homebrew/bin/beads` and provides a rich CLI with dependency tracking, epics, sync, and JSONL-based storage.

**Rationale**: Verified against actual `bd --help` output. Beads stores issues in JSONL files alongside code, supports git sync, and provides dependency management. The binary name `bd` is the primary interface. Key features: create/list/close issues, dependency tracking, epic management, search, daemon-based sync, molecule workflows.

**Key CLI examples for blog post** (verified against actual interface):
```bash
bd create "Fix login redirect after OAuth callback"
bd list
bd close bd-42 -r "Fixed in commit abc123"
bd ready                                          # Show unblocked work
bd dep add bd-42 blocks bd-43                     # Track dependencies
```

**Alternatives considered**: Could use the full `beads` name in examples, but `bd` is the actual binary name and what developers will type. The blog should use `bd` for accuracy.

**Action item**: ~~Verify beads CLI interface against actual tool documentation~~ DONE — verified 2026-02-25.

---

### R3: Speckit — Accurate workflow and capabilities

**Decision**: Describe speckit as a specification-driven development workflow that generates structured artifacts (specs, plans, tasks) from feature descriptions, all stored in version control.

**Rationale**: Based on direct inspection of the `.specify/` directory structure. Speckit provides a pipeline: specify → plan → tasks → implement. Each stage generates markdown artifacts in a feature-numbered directory (`specs/###-feature-name/`). It integrates with multiple AI agents (Claude, Gemini, Copilot, Cursor) via auto-generated context files.

**Key CLI examples for blog post**:
```bash
/speckit.specify "Add user authentication"   # Generate spec
/speckit.plan                                 # Create implementation plan
/speckit.tasks                                # Break into actionable tasks
/speckit.implement                            # Execute tasks
```

**Alternatives considered**: Could present speckit as standalone CLI commands rather than slash commands. However, speckit currently operates as AI-agent skills (slash commands within Claude Code), which is its actual interface. The blog post should be honest about this — it's a workflow system that leverages AI agents, not a traditional standalone binary.

---

### R4: Blog post structure and tone

**Decision**: Use a narrative structure that moves from problem → philosophy → individual tools → composed workflow → adoption path → call to action.

**Rationale**: This mirrors the user stories in priority order: P1 (discover ecosystem / understand problem), P1 (understand each tool), P1 (see composed workflow), P2 (adopt incrementally). The spec requires developer-to-developer tone (FR-005) and Unix philosophy framing (FR-006).

**Section outline**:
1. **Opening hook** (~200 words): The problem — context switching, tool sprawl, flow state disruption
2. **Philosophy** (~200 words): Unix philosophy applied to dev workflows — small, composable, text-based
3. **The tools** (~600 words): Each tool with a concrete example (~200 words each)
4. **The composed workflow** (~500 words): End-to-end walkthrough showing all three tools on one feature
5. **Everything lives in git** (~200 words): Version control as the single source of truth
6. **Start with one** (~200 words): Incremental adoption message
7. **Call to action** (~100 words): Links to repos/docs

**Total**: ~2,000 words (within 1,500-2,500 target)

**Alternatives considered**:
- Tutorial-style (rejected: spec says "not a tutorial", FR edge case)
- Comparison-style vs Jira/Linear (rejected: FR-004 prohibits mentioning specific products by name, though we can reference the category)

---

### R5: Version control as unifying theme

**Decision**: Emphasize that all three tools store their data as files in the repository — anvil tasks as `.anvil/todos/*.md`, beads issues alongside code, speckit specs in `specs/`. This is the architectural thesis: your project management system IS your git repo.

**Rationale**: FR-009 requires a section on how specs, issues, and tasks live alongside code. This is also the strongest differentiator from SaaS tools and directly supports the "stay in the terminal" message.

**Alternatives considered**: Could frame version control as just a storage detail. But it's actually the core architectural insight that makes the ecosystem cohesive — all three tools share the same workspace.

---

### R6: Target audience and tone calibration

**Decision**: Write for mid-to-senior developers who already live in the terminal. Acknowledge GUI alternatives exist without being dismissive (edge case from spec). Use concrete examples over abstract claims.

**Rationale**: The spec explicitly states the audience is "mid-to-senior developers comfortable with command-line tools." The tone must be practical and peer-level (FR-005), not marketing copy.

**Alternatives considered**: Could write for a broader audience including junior developers. Rejected because it would dilute the message and require too much CLI basics explanation, pushing past the word count limit.
