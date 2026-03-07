# Data Model: Template Registry for Shared Templates

**Feature**: 303-template-registry
**Date**: 2026-03-07

## Entities

### TemplateManifest

Metadata file (`anvil-template.yaml`) at the root of a registry template repository.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | yes | Human-readable template name |
| description | string | yes | Short description of what the template does |
| author | string | yes | Template author name |
| version | string | yes | Semantic version (e.g., "1.0.0") |
| min_anvil_version | string | no | Minimum anvil version required |
| files | []string | yes | List of template files relative to repo root (e.g., ["deploy.yaml", "ci.yaml"]) |
| tags | []string | no | Additional search keywords |

### RegistryMeta

Sidecar file (`.registry-meta.yaml`) stored alongside installed templates in `.anvil/templates/`.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| source | string | yes | Registry identifier (e.g., "anvil-templates/ci-pipeline") |
| version | string | yes | Version that was installed |
| installed_at | string (RFC3339) | yes | Installation timestamp |
| manifest_url | string | yes | URL of the manifest for update checks |

### RegistrySearchResult

In-memory struct representing a search result from the GitHub API.

| Field | Type | Description |
|-------|------|-------------|
| owner | string | GitHub repository owner |
| repo | string | GitHub repository name |
| description | string | Repository description |
| stars | int | Star count (for ranking) |
| url | string | Repository URL |

### SearchCache

Local cache file at `~/.anvil/cache/registry/search-<hash>.json`.

| Field | Type | Description |
|-------|------|-------------|
| query | string | Original search query |
| results | []RegistrySearchResult | Cached results |
| cached_at | string (RFC3339) | When the cache was written |
| ttl | duration | Cache validity (1 hour) |

## Relationships

```
GitHub Repository (remote)
  └── anvil-template.yaml (TemplateManifest)
  └── *.yaml (template files)

.anvil/templates/ (local)
  ├── <name>.yaml (TemplateSpec - existing)
  └── <name>.registry-meta.yaml (RegistryMeta - new)

~/.anvil/cache/registry/ (local cache)
  └── search-<hash>.json (SearchCache)
```

## State Transitions

Template lifecycle:
1. **Remote** - Template exists only in GitHub registry
2. **Discovered** - User has found it via search
3. **Installed** - Downloaded to local `.anvil/templates/` with registry metadata
4. **Outdated** - Local version differs from remote (future: update command)
