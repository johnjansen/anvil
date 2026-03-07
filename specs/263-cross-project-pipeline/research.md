# Research: Cross-Project Pipeline Visualization

## R1: Task Key Disambiguation Strategy

**Decision**: Use `projectName:taskName` as the internal key for tasks in the pipeline graph when `--all` is used. For single-project mode, continue using plain `taskName`.

**Rationale**: The `ParseDependency` function already uses `project:task` format for cross-project deps. Using the same format as internal graph keys ensures consistent lookup. The project name comes from the watched directory entry name (the directory name under `~/.anvil/watched/`). For the current project (not watched), derive the project name from the directory basename.

**Alternatives considered**:
- Full path as key: Too verbose, leaks filesystem details into output
- UUID-based keys: Tasks already have UUIDs but they're not user-facing; poor readability

## R2: ASCII Visual Distinction Approach

**Decision**: Use project header sections and bracket-prefixed labels for cross-project references.

Format for `--all` ASCII output:
```
=== project-alpha ===
build
├── test
│   └── deploy
│       └── [project-beta] notify    ← cross-project dep shown with bracket prefix

=== project-beta ===
notify [depends on project-alpha:deploy]
└── cleanup
```

**Rationale**: Project headers (`=== name ===`) provide clear visual boundaries. Bracket-prefixed task names (`[project] task`) make cross-project edges immediately identifiable without disrupting the tree structure.

**Alternatives considered**:
- Color-coded output: Not all terminals support color; adds complexity
- Indentation-only grouping: Too subtle to distinguish projects
- Separate trees per project with no cross-references: Loses the main value of showing cross-project flow

## R3: DOT Subgraph Clustering

**Decision**: Use GraphViz `subgraph cluster_<project>` with labeled borders. Cross-project edges use `style=dashed` to distinguish from local edges.

**Rationale**: GraphViz natively supports subgraph clustering with `cluster_` prefix naming. Dashed edges are a well-established convention for "external" relationships in graph visualizations.

**Alternatives considered**:
- Different node shapes per project: Harder to read with many projects
- Edge color only: Less accessible, may not render in black-and-white

## R4: Integration with Existing buildPipelineGraph

**Decision**: Extend `pipelineTaskInfo` to include a `projectName` field. Modify `buildPipelineGraph` to:
1. Assign project names to each task based on which project loaded it
2. Use `ParseDependency` when processing `dependsOn` to detect cross-project refs
3. Use qualified keys (`project:task`) in the tasks map when multi-project mode is active

**Rationale**: Minimal change to existing code. The `pipelineTaskInfo` struct already has `projPath` — adding `projectName` is a natural extension. Using `ParseDependency` aligns with the existing dependency infrastructure from #259.

**Alternatives considered**:
- Separate graph per project merged at render time: More complex, harder to detect cross-project cycles
- New dedicated struct: Unnecessary when extending existing struct works

## R5: Backward Compatibility

**Decision**: Single-project mode (no `--all`) remains completely unchanged. Multi-project mode without cross-project deps shows project headers but otherwise identical tree structure within each project.

**Rationale**: Zero-regression requirement from FR-009. The project header grouping only activates when multiple projects are loaded.

**Alternatives considered**: None — backward compatibility is non-negotiable.
