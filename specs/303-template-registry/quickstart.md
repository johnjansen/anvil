# Quickstart: Template Registry

## For Template Users

### Search for templates
```bash
anvil template search "ci"
```

### View template details
```bash
anvil template info anvil-templates/ci-pipeline
```

### Install a template
```bash
anvil template install anvil-templates/ci-pipeline
```

### List installed registry templates
```bash
anvil template list --installed
```

### Use an installed template
```bash
anvil task create --template ci-pipeline my-ci-task
```

## For Template Authors

### 1. Create a GitHub repository

Create a public repository with your template files and an `anvil-template.yaml` manifest:

```yaml
name: my-template
description: "What this template does"
author: your-github-username
version: "1.0.0"
files:
  - my-template.yaml
tags:
  - relevant
  - keywords
```

### 2. Add the anvil-template topic

Go to your repository settings and add the `anvil-template` topic so it appears in search results.

### 3. Publish

Your template is now discoverable via `anvil template search`.
