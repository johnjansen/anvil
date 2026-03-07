# Feature Specification: Template Registry for Shared Templates

**Feature Branch**: `303-template-registry`
**Created**: 2026-03-07
**Status**: Draft
**Input**: User description: "Users need a way to discover and install shared templates from a public registry"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Search for Templates (Priority: P1)

A user wants to find shared templates that match a keyword. They run `anvil template search "github"` and get a list of matching templates from the registry, including names, descriptions, and authors. This lets them discover reusable task configurations without browsing the registry manually.

**Why this priority**: Discovery is the entry point for the entire registry experience. Without search, users cannot find templates to install.

**Independent Test**: Can be fully tested by running `anvil template search <keyword>` and verifying that matching results are returned with name, description, and author.

**Acceptance Scenarios**:

1. **Given** the registry contains templates with "github" in their name or description, **When** the user runs `anvil template search "github"`, **Then** a list of matching templates is displayed with name, description, and author.
2. **Given** no templates match the search query, **When** the user runs `anvil template search "nonexistent"`, **Then** a message indicates no results were found.
3. **Given** the registry is unreachable, **When** the user runs `anvil template search "github"`, **Then** a clear error message indicates the registry is unavailable.

---

### User Story 2 - Install a Template (Priority: P1)

A user finds a template they want and runs `anvil template install anvil-templates/ci-pipeline`. The template is downloaded and placed into their project's `.anvil/todos/` directory, ready to use. If a template with the same name already exists locally, the user is warned before overwriting.

**Why this priority**: Installation is the core action that delivers value. Search without install is incomplete.

**Independent Test**: Can be fully tested by running `anvil template install <owner/name>` and verifying the template files appear in the local project.

**Acceptance Scenarios**:

1. **Given** a valid template identifier, **When** the user runs `anvil template install anvil-templates/ci-pipeline`, **Then** the template files are downloaded and placed in `.anvil/todos/`.
2. **Given** a template that already exists locally, **When** the user runs `anvil template install anvil-templates/ci-pipeline`, **Then** the user is prompted to confirm overwrite before proceeding.
3. **Given** an invalid template identifier, **When** the user runs `anvil template install nonexistent/template`, **Then** a clear error message indicates the template was not found.
4. **Given** the user passes `--force`, **When** installing a template that already exists locally, **Then** the template is overwritten without prompting.

---

### User Story 3 - View Template Details Before Installing (Priority: P2)

A user wants to inspect a template before installing it. They run `anvil template info anvil-templates/ci-pipeline` and see the full description, required configuration, author, version, and any dependencies.

**Why this priority**: Allows informed decision-making before installing. Not strictly required for the install flow but significantly improves the user experience.

**Independent Test**: Can be tested by running `anvil template info <owner/name>` and verifying detailed template metadata is displayed.

**Acceptance Scenarios**:

1. **Given** a valid template identifier, **When** the user runs `anvil template info anvil-templates/ci-pipeline`, **Then** the template's full description, author, version, and configuration requirements are displayed.
2. **Given** an invalid template identifier, **When** the user runs `anvil template info nonexistent/template`, **Then** a clear error message indicates the template was not found.

---

### User Story 4 - List Installed Templates (Priority: P3)

A user wants to see which registry templates they have installed in their project. They run `anvil template list --installed` and see the installed templates with their source and version.

**Why this priority**: Useful for maintenance and updates but not required for the core search-and-install workflow.

**Independent Test**: Can be tested by installing a template, then running `anvil template list --installed` and verifying it appears.

**Acceptance Scenarios**:

1. **Given** templates have been installed from the registry, **When** the user runs `anvil template list --installed`, **Then** installed templates are listed with their registry source and version.
2. **Given** no templates have been installed from the registry, **When** the user runs `anvil template list --installed`, **Then** a message indicates no registry templates are installed.

---

### Edge Cases

- What happens when the user has no network connectivity? The system should fail gracefully with a clear offline error message.
- What happens when a template's format is incompatible with the user's anvil version? The system should check compatibility and warn the user before installing.
- What happens when a template contains files that conflict with existing non-template files in the project? The system should warn the user and not overwrite non-template files without explicit confirmation.
- What happens when a template is removed from the registry after being installed locally? The local installation continues to work; the system does not remove locally installed templates.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide an `anvil template search <query>` command that queries the registry and displays matching templates with name, description, and author.
- **FR-002**: System MUST provide an `anvil template install <owner/name>` command that downloads a template from the registry and places it in the local project.
- **FR-003**: System MUST warn the user before overwriting an existing local template during install, unless `--force` is specified.
- **FR-004**: System MUST provide an `anvil template info <owner/name>` command that displays detailed metadata for a registry template.
- **FR-005**: System MUST provide an `anvil template list --installed` command that shows templates installed from the registry.
- **FR-006**: System MUST use GitHub repositories as the registry backend, with templates identified by `<owner>/<repo>` format.
- **FR-007**: System MUST record the source registry identifier and version for each installed template, enabling future update checks.
- **FR-008**: System MUST handle network errors gracefully with clear, actionable error messages.
- **FR-009**: System MUST validate template compatibility with the current anvil version before installing.

### Key Entities

- **RegistryTemplate**: A template available in the registry, with name, description, author, version, and compatibility metadata.
- **InstalledTemplate**: A locally installed template with a reference back to its registry source and the version installed.
- **TemplateManifest**: Metadata file within a template repository that describes the template's contents, configuration requirements, and anvil version compatibility.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can discover relevant templates by keyword search, with results returned in under 5 seconds.
- **SC-002**: Users can install a template from the registry in a single command, with the template ready to use immediately.
- **SC-003**: Users are protected from accidental overwrites of existing local templates during installation.
- **SC-004**: Users can inspect template details before installing to make informed decisions.
- **SC-005**: Users can audit which registry templates are installed in their project.

## Assumptions

- The registry backend is GitHub-based: templates are GitHub repositories following a standard layout (e.g., containing a manifest file and task markdown files). This avoids building custom registry infrastructure.
- Template search uses the GitHub API to search repositories by topic or naming convention (e.g., repos tagged with `anvil-template`).
- Core template import/export (#290) provides the foundational template format that this feature builds upon. This feature assumes that format is established.
- Community contributions happen through standard GitHub workflows (fork, PR, publish repos). No custom submission process is needed.
- Authentication for private registries or rate-limited API access is deferred to a future iteration. Initial implementation uses unauthenticated GitHub API access.
