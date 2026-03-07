# Template Manifest Schema: anvil-template.yaml

Template repositories must include an `anvil-template.yaml` file at the repository root.

## Schema

```yaml
# Required fields
name: ci-pipeline                    # Template name (alphanumeric, hyphens)
description: "CI/CD pipeline template"  # Short description
author: anvil-templates              # Author name
version: "1.2.0"                     # Semantic version
files:                               # Template files to install
  - ci-pipeline.yaml
  - deploy-stage.yaml

# Optional fields
min_anvil_version: "0.130.0"         # Minimum anvil version
tags:                                # Additional search keywords
  - ci
  - github-actions
  - deploy
```

## Validation Rules

- `name`: required, must match `^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`
- `description`: required, max 200 characters
- `author`: required
- `version`: required, must be valid semver
- `files`: required, at least one entry, all files must exist in the repository
- `min_anvil_version`: optional, valid semver if provided
- `tags`: optional, max 10 tags, each max 30 characters

## Repository Requirements

- Repository must be public (for unauthenticated access)
- Repository must have the `anvil-template` topic for discovery via search
- `anvil-template.yaml` must be at the repository root
- Template files referenced in `files` must exist at the specified paths
