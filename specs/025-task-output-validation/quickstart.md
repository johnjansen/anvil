# Quickstart: Task Output Validation with Assertions

## Overview

Task output validation allows you to ensure your tasks produce the expected output before considering them successful. With assertions, you can check stdout/stderr content and file properties, catching issues early rather than discovering them when downstream tasks fail.

## Basic Usage

Add an `assert` block to your task's frontmatter:

```yaml
---
schedule: "0 9 * * *"
assert:
  stdout:
    contains: "Success:"
  stderr:
    empty: true
---
#!/bin/bash
echo "Processing complete. Success: Data updated"
```

This task will only be considered successful if its stdout contains "Success:" and its stderr is empty.

## Stdout/Stderr Assertions

Check output stream content with various conditions:

```yaml
assert:
  stdout:
    contains: "Processing complete"
    matches: "\\d+ records processed"
    json_valid: true
    empty: false
  stderr:
    empty: true
```

## File Assertions

Validate files created by your task:

```yaml
assert:
  files:
    - path: "output.json"
      exists: true
      contains: "status: ok"
    - path: "report.csv"
      size_min: 1000
      size_max: 1000000
```

## Soft Assertions

Use soft assertions to warn about potential issues without failing the task:

```yaml
assert:
  soft: true
  stdout:
    contains: "Warning"
```

If the stdout contains "Warning", it will be logged but won't cause the task to fail.

## Error Messages

When assertions fail, you'll see clear error messages:

```
$ anvil task run my-task
...
Assertion failed: stdout does not contain "Success:"
Run failed.
```

## Next Steps

1. Add assertions to your existing tasks to catch output issues early
2. Use file assertions to validate that your tasks create expected files with correct content
3. Implement soft assertions for non-critical validations that shouldn't block task completion