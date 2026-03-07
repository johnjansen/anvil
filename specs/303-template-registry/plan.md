# Implementation Plan: Template Registry for Shared Templates

**Branch**: `303-template-registry` | **Date**: 2026-03-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/303-template-registry/spec.md`

## Summary

Add a public template registry backed by GitHub repositories, enabling users to discover and install shared task templates via `anvil template search`, `anvil template install`, and `anvil template info` commands. Builds on the existing template import/export system (#290) and the `internal/project` Template/TemplateSpec types.

## Technical Context

**Language/Version**: Go 1.24.6
**Primary Dependencies**: `net/http` (GitHub API), `gopkg.in/yaml.v3` (template parsing), existing `internal/project` (Template, TemplateSpec, LoadTemplate, ListTemplates)
**Storage**: Local YAML files in `.anvil/templates/` (project) and `~/.anvil/templates/` (global); registry metadata stored alongside installed templates
**Testing**: Go test (`go test ./...`)
**Target Platform**: macOS, Linux (CLI)
**Project Type**: CLI tool
**Performance Goals**: Registry search results returned in under 5 seconds
**Constraints**: Unauthenticated GitHub API (60 requests/hour rate limit); must degrade gracefully when offline
**Scale/Scope**: Dozens to hundreds of community templates

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Constitution is not yet configured for this project (template placeholders only). No gates to enforce. Proceeding with standard engineering best practices.

## Project Structure

### Documentation (this feature)

```text
specs/303-template-registry/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
cmd/anvil/
├── template.go              # Existing - add "install", "info" subcommands
├── template_search.go       # Existing - extend to query registry
├── template_import.go       # Existing - reuse download logic
├── template_export.go       # Existing - unchanged
└── template_registry.go     # New - registry install, info, list --installed

internal/project/
├── project.go               # Existing - extend Template/TemplateSpec with registry metadata
└── registry.go              # New - registry client (GitHub API search, download, manifest parsing)

internal/project/
└── registry_test.go         # New - unit tests for registry client
```

**Structure Decision**: Follows existing single-project layout. New registry logic goes into `internal/project/registry.go` as a new file to keep concerns separated from the existing template code. CLI commands extend the existing `cmd/anvil/template.go` dispatch.

## Complexity Tracking

No constitution violations to justify.
