# Data Model Changes: Remove Task Dependency Pipeline

**Feature**: 367-remove-dependency-pipeline
**Date**: 2026-03-07

## Types to Delete

### `Dependency` struct (internal/project/dependencies.go)
```
Project  string
Task     string
IsLocal  bool
```
Entire file deleted — includes `Dependency`, `DependencyGraph`, and all functions.

### `depFailInfo` struct (internal/daemon/daemon.go)
```
Reason   string
DepName  string
DepError string
Finished time.Time
ExitCode int
```
Removed along with `checkDependenciesMet()` function.

## Types to Modify

### `Todo` struct (internal/project/project.go)

**Remove fields**:
- `DependsOn []string` — list of dependency strings
- `DependencyPolicy DependencyPolicyConfig` — failure handling policy

### `DependencyPolicyConfig` struct (internal/project/project.go)

**Delete entirely**:
```
OnFailure string  // "skip", "require_all", "require_any"
```

### Frontmatter YAML struct (internal/project/project.go)

**Remove fields**:
- `DependsOn []string` yaml:"depends_on"
- `DependencyPolicy` nested struct with `OnFailure` field

## Run Records

No changes. Existing RunRecord JSON files are left intact. The RunRecord struct itself has no dependency-specific fields.

## Task File Format

The `depends_on` and `dependency_policy` YAML frontmatter fields become unrecognized. Since Go's `yaml.v3` silently ignores unknown fields when unmarshaling into a struct, existing task files with these fields will load without errors.
