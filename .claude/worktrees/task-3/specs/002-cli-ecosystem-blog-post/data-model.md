# Data Model: CLI Ecosystem Blog Post

**Feature**: 002-cli-ecosystem-blog-post
**Date**: 2026-02-25

## Entities

This feature is a content deliverable, not a software system. The "entities" here are the conceptual objects the blog post must accurately describe and relate.

### Blog Post

The primary deliverable.

| Field | Type | Constraints |
|-------|------|-------------|
| title | string | Descriptive, SEO-friendly, no proprietary names |
| content | markdown | 1,500-2,500 words |
| format | GFM markdown | Compatible with dev.to, Medium, Hashnode |
| tone | developer-to-developer | Practical, not marketing |
| sections | ordered list | See structure in research.md R4 |

### Anvil (described entity)

| Attribute | Value |
|-----------|-------|
| Role | Recurring task management and automation |
| Language | Go |
| Storage | `.anvil/todos/` markdown files with YAML frontmatter |
| Key concept | Single daemon watches multiple projects; cron-scheduled LLM task execution |
| CLI entry points | `anvil watch`, `anvil add`, `anvil task ls/run/log` |
| Differentiator | Plain-English tasks executed by LLM on a schedule |

### Beads (described entity)

| Attribute | Value |
|-----------|-------|
| Role | Lightweight, git-native issue tracking |
| Storage | Files alongside code in version control |
| Key concept | Issues live in the repo, not in a SaaS tool |
| CLI entry points | `beads add`, `beads list`, `beads close` |
| Differentiator | Zero-infrastructure issue tracking in git |

### Speckit (described entity)

| Attribute | Value |
|-----------|-------|
| Role | Specification-driven development workflow |
| Storage | `specs/###-feature/` directories with markdown artifacts |
| Key concept | Pipeline from idea → spec → plan → tasks → implementation |
| CLI entry points | `/speckit.specify`, `/speckit.plan`, `/speckit.tasks`, `/speckit.implement` |
| Differentiator | Structured development workflow that generates traceable artifacts |

## Relationships

```text
speckit ──creates──→ specification
specification ──breaks into──→ issues (beads)
issues ──become──→ recurring tasks (anvil)
anvil ──automates──→ execution

All three tools:
  └── store data as files in the git repository
  └── operate from the terminal (CLI-first)
  └── are independently useful
  └── compose into a unified workflow
```

## Validation Rules (from requirements)

| Rule | Source | Validation |
|------|--------|------------|
| No proprietary names | FR-004 | Text search for company/product names |
| Word count 1,500-2,500 | FR-008 | `wc -w` on final document |
| CLI example per tool | FR-002 | Manual check: each tool section has code block |
| End-to-end workflow | FR-003 | Manual check: workflow section uses all three tools |
| Call to action with links | FR-010 | Manual check: final section has repo/doc links |
| Developer tone | FR-005 | Peer review: no marketing language |
| Unix philosophy | FR-006 | Manual check: composability/small tools mentioned |
| Incremental adoption | FR-007 | Manual check: standalone value addressed |
| Version control section | FR-009 | Manual check: dedicated section exists |
