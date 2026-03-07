# Research: Template Registry for Shared Templates

**Feature**: 303-template-registry
**Date**: 2026-03-07

## R1: Registry Backend Design

**Decision**: Use GitHub repositories as the registry backend, with GitHub API search for discovery.

**Rationale**: Avoids building and hosting custom registry infrastructure. GitHub provides free hosting, versioning, community contribution workflows (fork/PR), and a search API. Templates are identified by `owner/repo` format, which is familiar to developers. The existing `template_import.go` already handles HTTP downloads, so this is a natural extension.

**Alternatives considered**:
- Custom registry server: Requires hosting, maintenance, authentication. Overkill for initial implementation.
- npm-style registry: Requires custom tooling and a separate publish workflow. Too heavyweight.
- Git submodules: Complex for end users, poor UX for discovery.

## R2: Template Discovery Mechanism

**Decision**: Use GitHub API repository search with topic-based filtering (`anvil-template` topic) and keyword search within results.

**Rationale**: GitHub topics provide a curated discovery mechanism. Repository owners tag their repos with `anvil-template` to opt into the registry. The search API supports query strings with topic filters: `q=<keyword>+topic:anvil-template`. This avoids maintaining a central index.

**Alternatives considered**:
- Central index file in a dedicated repo: Requires manual curation, single point of failure.
- GitHub org-based: Limits contributions to org members.
- Awesome-list style: Manual curation doesn't scale.

## R3: Template Manifest Format

**Decision**: Templates use a `anvil-template.yaml` manifest file at the repository root containing metadata (name, description, author, version, min_anvil_version, files list).

**Rationale**: A manifest file allows templates to declare their contents and compatibility without requiring the CLI to scan the entire repository. YAML is consistent with the existing TemplateSpec format. The manifest is separate from the template spec files themselves.

**Alternatives considered**:
- Use GitHub repo description as metadata: Insufficient for version/compatibility info.
- Embed metadata in each template file: Scattered, hard to aggregate.
- JSON manifest: Inconsistent with existing YAML conventions.

## R4: Template Installation Strategy

**Decision**: Download template files listed in the manifest and place them in `.anvil/templates/` with a `.registry-meta.yaml` sidecar file recording source, version, and install date.

**Rationale**: Sidecar metadata file enables `list --installed` without modifying the template file format. Files go to the standard template directory so existing `LoadTemplate` and `ListTemplates` functions work without changes. The sidecar pattern is non-invasive.

**Alternatives considered**:
- Embed registry metadata in template files: Breaks existing format, complicates parsing.
- Central installed-templates registry file: Single file becomes a merge conflict point.
- Git clone the repo: Too heavy, clutters the project with git metadata.

## R5: Rate Limiting Strategy

**Decision**: Use unauthenticated GitHub API with 60 req/hour limit. Cache search results locally for 1 hour in `~/.anvil/cache/registry/`. If `gh` CLI is available and authenticated, use its token for 5000 req/hour.

**Rationale**: 60 req/hour is sufficient for casual usage. Caching avoids redundant API calls during iterative search sessions. Opportunistic use of `gh` auth token provides higher limits for power users without requiring explicit configuration.

**Alternatives considered**:
- Require GitHub token: Poor onboarding UX.
- No caching: Hits rate limits quickly during iterative search.
- Custom API key: Requires separate auth flow.

## R6: Version Compatibility

**Decision**: Manifest declares `min_anvil_version`. CLI checks against its own version at install time and warns (but does not block) if incompatible.

**Rationale**: Warnings are less disruptive than hard blocks, especially when version mismatches may be minor. Users can proceed with `--force` if they understand the risk.

**Alternatives considered**:
- Hard block on version mismatch: Too restrictive, especially for minor version differences.
- No version checking: Silent failures when templates use unsupported features.
