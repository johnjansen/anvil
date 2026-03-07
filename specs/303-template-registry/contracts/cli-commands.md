# CLI Command Contracts: Template Registry

## anvil template search <query>

Search the template registry for matching templates.

**Arguments**: `<query>` - keyword to search for (required)

**Output (success)**:
```
Registry templates matching 'github':
  anvil-templates/github-actions   - GitHub Actions CI/CD pipeline    (v1.2.0, 45 stars)
  user123/github-deploy            - Auto-deploy on GitHub push       (v0.3.1, 12 stars)
```

**Output (no results)**:
```
No registry templates found matching: nonexistent
```

**Output (error - network)**:
```
Error: unable to reach template registry: <error detail>
Hint: check your network connection or try again later
```

**Exit codes**: 0 (success/no results), 1 (error)

---

## anvil template install <owner/repo> [flags]

Install a template from the registry.

**Arguments**: `<owner/repo>` - registry template identifier (required)

**Flags**:
- `--force` - overwrite existing templates without prompting

**Output (success)**:
```
Installing template 'ci-pipeline' from anvil-templates/ci-pipeline (v1.2.0)...
  Downloaded: ci-pipeline.yaml
  Installed to: .anvil/templates/ci-pipeline.yaml
Successfully installed template 'ci-pipeline'
```

**Output (already exists)**:
```
Template 'ci-pipeline' already exists locally.
Overwrite? [y/N]:
```

**Output (error - not found)**:
```
Error: template not found: nonexistent/template
```

**Output (error - version warning)**:
```
Warning: template requires anvil v0.140.0 or later (you have v0.134.0)
Install anyway? [y/N]:
```

**Exit codes**: 0 (success), 1 (error/cancelled)

---

## anvil template info <owner/repo>

Display detailed information about a registry template.

**Arguments**: `<owner/repo>` - registry template identifier (required)

**Output (success)**:
```
Template: ci-pipeline
Author:   anvil-templates
Version:  1.2.0
Source:    https://github.com/anvil-templates/ci-pipeline

Description:
  GitHub Actions CI/CD pipeline template with build, test, and deploy stages.

Files:
  - ci-pipeline.yaml
  - deploy-stage.yaml

Requires: anvil v0.130.0+
Tags: ci, github-actions, deploy
```

**Output (error - not found)**:
```
Error: template not found: nonexistent/template
```

**Exit codes**: 0 (success), 1 (error)

---

## anvil template list --installed

List templates installed from the registry. Extends existing `anvil template ls`.

**Flags**:
- `--installed` - show only registry-installed templates with source info

**Output (with installed templates)**:
```
Installed registry templates:
  ci-pipeline      anvil-templates/ci-pipeline   v1.2.0   installed 2026-03-01
  github-deploy    user123/github-deploy         v0.3.1   installed 2026-03-05
```

**Output (no installed templates)**:
```
No registry templates installed.
Use 'anvil template search <query>' to find templates.
```

**Exit codes**: 0 (success), 1 (error)
