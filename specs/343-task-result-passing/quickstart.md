# Quickstart: Task Result Passing

## Basic Setup

### 1. Create a producer task

Create `.anvil/todos/fetch-data.md`:

```markdown
---
schedule: "0 9 * * *"
capture_output: true
---
Fetch data from the API and output a summary.
Print the result as: ##anvil:result {"records": 42}
```

### 2. Create a consumer task

Create `.anvil/todos/process-data.md`:

```markdown
---
schedule: "0 9 * * *"
depends_on: [fetch-data]
---
Process the fetched data.
The dependency results are available in ANVIL_DEPENDENCY_RESULTS env var.
```

### 3. View results

```bash
# See the most recent captured result
anvil task results fetch-data

# Preview what a dependent task would receive
anvil task results process-data --preview
```

## Template Variables

Access dependency results directly in task body:

```markdown
---
depends_on: [fetch-data]
---
Process {{ index .DependencyResults "fetch-data" | json "records" }} records.
```

## Cross-Project Dependencies

```markdown
---
depends_on: ["other-project:fetch-data"]
---
Results from other projects work the same way via ANVIL_DEPENDENCY_RESULTS.
```
