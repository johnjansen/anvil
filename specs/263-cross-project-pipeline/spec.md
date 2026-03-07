# Feature Specification: Cross-Project Pipeline Visualization

**Feature Branch**: `263-cross-project-pipeline`
**Created**: 2026-03-07
**Status**: Draft
**Input**: User description: "UI: Show cross-project deps in pipeline visualization"

## User Scenarios & Testing

### User Story 1 - View Cross-Project Dependencies in Pipeline (Priority: P1)

A user managing multiple anvil projects wants to see how tasks across different projects depend on each other. They run `anvil task pipeline --all` and see a unified view that clearly shows which tasks belong to which projects, and which dependencies cross project boundaries.

**Why this priority**: This is the core value of the feature. Without cross-project visibility, users cannot understand how their multi-project automation pipelines connect.

**Independent Test**: Can be fully tested by creating two watched projects with cross-project dependencies (e.g., `projectB:deploy` depends on `projectA:build`) and running `anvil task pipeline --all` to verify the output shows project groupings and cross-project edges.

**Acceptance Scenarios**:

1. **Given** two watched projects where project-B has a task depending on `project-A:build`, **When** the user runs `anvil task pipeline --all`, **Then** the output shows both projects with clear project boundary headers and the cross-project dependency is visually distinguished from same-project dependencies.
2. **Given** a single project with only local dependencies, **When** the user runs `anvil task pipeline --all`, **Then** the output looks identical to the current behavior (no visual regression).
3. **Given** a cross-project dependency referencing a project that is not watched, **When** the user runs `anvil task pipeline --all`, **Then** a warning is displayed indicating the unresolved cross-project dependency.

---

### User Story 2 - Visual Distinction Between Local and Cross-Project Deps (Priority: P1)

A user viewing the ASCII pipeline output needs to quickly distinguish same-project dependencies from cross-project ones. Cross-project dependencies are labeled with the source project name and use a distinct visual marker so they stand out.

**Why this priority**: Without visual distinction, the unified view would be confusing and defeat the purpose of the feature.

**Independent Test**: Can be tested by verifying that cross-project dependency edges in the ASCII tree include the project name prefix and use a distinguishing marker (e.g., arrow annotation or label).

**Acceptance Scenarios**:

1. **Given** a task with a cross-project dependency `other-project:build`, **When** displayed in the ASCII pipeline, **Then** the dependency is shown with the project name prefix (e.g., `[other-project] build`) to distinguish it from local dependencies.
2. **Given** a task with both local and cross-project dependencies, **When** displayed in the ASCII pipeline, **Then** local dependencies show without a project prefix and cross-project dependencies show with a project prefix.

---

### User Story 3 - Cross-Project Deps in DOT Output (Priority: P2)

A user generating GraphViz DOT output with `anvil task pipeline --dot --all` gets a graph that uses subgraphs to represent project boundaries and visually distinguishes cross-project edges.

**Why this priority**: DOT output is a secondary output format used for documentation and advanced visualization. The ASCII output serves the primary use case.

**Independent Test**: Can be tested by generating DOT output with `--dot --all` on multi-project setups and verifying subgraph clusters and edge styling in the output.

**Acceptance Scenarios**:

1. **Given** multiple watched projects with cross-project dependencies, **When** the user runs `anvil task pipeline --dot --all`, **Then** the DOT output groups tasks into `subgraph cluster_<project>` blocks and cross-project edges are visually distinguishable (e.g., dashed style or different color).

---

### User Story 4 - Verbose Mode with Cross-Project Context (Priority: P3)

A user running `anvil task pipeline --all --verbose` sees schedule and last-run status for tasks across all projects, including cross-project dependency tasks.

**Why this priority**: Verbose mode is an enhancement on top of the core visualization and builds on stories 1-2.

**Independent Test**: Can be tested by running `--all --verbose` and verifying that cross-project tasks show their schedule and run status alongside their project label.

**Acceptance Scenarios**:

1. **Given** cross-project tasks with run history, **When** the user runs `anvil task pipeline --all --verbose`, **Then** each task shows its schedule, last run status, and project label.

---

### Edge Cases

- What happens when a cross-project dependency forms a cycle across projects? The existing cycle detection should catch it and display a warning.
- What happens when there are no cross-project dependencies but `--all` is used? The output should show each project's local pipelines grouped by project.
- What happens when a watched project cannot be loaded (e.g., path no longer exists)? The project should be skipped with a warning, consistent with current behavior.
- What happens when two projects have tasks with the same name? They should be disambiguated by project name in the output.

## Requirements

### Functional Requirements

- **FR-001**: The pipeline command MUST use `ParseDependency` to distinguish local and cross-project dependencies when building the pipeline graph.
- **FR-002**: The ASCII pipeline output MUST display project boundary headers when showing tasks from multiple projects with `--all`.
- **FR-003**: Cross-project dependencies in ASCII output MUST be visually distinguished from local dependencies by including the source project name.
- **FR-004**: The DOT output MUST group tasks into subgraph clusters by project when `--all` is used.
- **FR-005**: Cross-project edges in DOT output MUST be visually distinct from local edges (e.g., dashed line style).
- **FR-006**: Tasks with the same name in different projects MUST be disambiguated in both ASCII and DOT output.
- **FR-007**: The pipeline graph builder MUST resolve cross-project dependency references to actual tasks in watched projects.
- **FR-008**: Unresolvable cross-project dependencies MUST produce a warning message on stderr.
- **FR-009**: The feature MUST be backward compatible -- single-project usage without cross-project deps MUST produce identical output to the current behavior.

### Key Entities

- **Dependency**: Represents a task dependency, either local (same project) or cross-project (prefixed with `project:`). Already exists via `ParseDependency`.
- **Pipeline Task Info**: Extended to include project context (project name, project path) for cross-project awareness.
- **Project Boundary**: A visual grouping in the output that separates tasks belonging to different projects.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Users can identify all cross-project dependencies in a single `anvil task pipeline --all` command without needing to inspect individual projects.
- **SC-002**: Cross-project dependencies are visually distinguishable from local dependencies within 1 second of viewing the output.
- **SC-003**: The pipeline output correctly resolves and displays dependencies across all watched projects without manual configuration.
- **SC-004**: Existing single-project pipeline usage produces identical output (zero regression).
