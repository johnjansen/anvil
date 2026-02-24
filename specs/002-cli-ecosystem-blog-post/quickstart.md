# Quickstart: Validating the CLI Ecosystem Blog Post

**Feature**: 002-cli-ecosystem-blog-post
**Date**: 2026-02-25

## How to validate

### 1. Word count check

```bash
wc -w blog/cli-ecosystem-blog-post.md
# Expected: 1,500-2,500 words
```

### 2. Proprietary name check

```bash
# Verify zero references to specific company or product names (FR-004)
# Add known proprietary names to the grep pattern as needed
grep -iE "(jira|linear|notion|asana|trello|monday|clickup|shortcut|basecamp)" blog/cli-ecosystem-blog-post.md
# Expected: no matches (references should use generic terms like "heavyweight planning tools")
```

### 3. Structural completeness

Manually verify these sections exist in the blog post:

- [ ] Opening hook / problem statement (FR-001)
- [ ] Individual tool description with CLI example: anvil (FR-002)
- [ ] Individual tool description with CLI example: beads (FR-002)
- [ ] Individual tool description with CLI example: speckit (FR-002)
- [ ] Composed workflow walkthrough using all three tools (FR-003)
- [ ] Unix philosophy / composability section (FR-006)
- [ ] Version control section — specs, issues, tasks alongside code (FR-009)
- [ ] Incremental adoption message (FR-007)
- [ ] Call to action with links to repos/docs (FR-010)

### 4. Tone check

Have a developer peer read the post and confirm:

- [ ] Reads as developer-to-developer, not marketing (FR-005)
- [ ] No condescending tone about GUI tools (edge case)
- [ ] CLI examples are realistic and match actual tool interfaces

### 5. Acceptance scenario validation

- [ ] SC-001: Unfamiliar developer can describe each tool's purpose after reading
- [ ] SC-002: Post contains at least one end-to-end workflow example
- [ ] SC-003: Zero proprietary company/product references
- [ ] SC-004: Within 1,500-2,500 word range
- [ ] SC-006: Publishable on any blog platform without modification

### 6. Pre-publication checks

- [ ] Verify beads CLI examples match actual tool interface
- [ ] Verify anvil CLI examples match current version
- [ ] Verify speckit workflow description is accurate
- [ ] All links to repos/docs resolve correctly
- [ ] Markdown renders correctly on target platform
