# Contract: Task Frontmatter with Assertions

## Overview

Tasks can include an `assert` block in their frontmatter to validate output before considering the task successful.

## Syntax

```yaml
---
# ... other frontmatter fields ...
assert:
  # Optional: treat assertion failures as warnings rather than errors
  soft: true|false  # default: false

  # Stdout assertions
  stdout:
    contains: "string"     # stdout must contain this string
    matches: "regex"       # stdout must match this regex pattern
    json_valid: true|false # stdout must be valid JSON
    empty: true|false      # stdout must be empty

  # Stderr assertions
  stderr:
    contains: "string"     # stderr must contain this string
    matches: "regex"       # stderr must match this regex pattern
    empty: true|false      # stderr must be empty

  # File assertions
  files:
    - path: "file/path"    # path to file to check
      exists: true|false   # file must exist/not exist
      contains: "string"   # file must contain this string
      size_min: number     # minimum file size in bytes
      size_max: number     # maximum file size in bytes
---
```

## Examples

### Basic stdout validation

```yaml
---
schedule: "0 9 * * *"
assert:
  stdout:
    contains: "Success:"
---
```

### Comprehensive validation

```yaml
---
schedule: "0 9 * * *"
assert:
  stdout:
    contains: "Processing complete"
    matches: "\\d+ records processed"
  stderr:
    empty: true
  files:
    - path: "output.json"
      exists: true
      contains: "status: ok"
    - path: "report.csv"
      size_min: 1000
---
```

### Soft assertions

```yaml
---
schedule: "0 9 * * *"
assert:
  soft: true
  stdout:
    contains: "Warning:"
---
```

## Field Details

### soft
- **Type**: Boolean
- **Default**: false
- **Description**: When true, assertion failures are logged as warnings but don't cause task failure

### stdout.contains / stderr.contains
- **Type**: String
- **Description**: The output stream must contain this exact string

### stdout.matches / stderr.matches
- **Type**: String (valid Go regex)
- **Description**: The output stream must match this regular expression pattern

### stdout.json_valid / stderr.json_valid
- **Type**: Boolean
- **Description**: When true, the output stream must be valid JSON

### stdout.empty / stderr.empty
- **Type**: Boolean
- **Description**: When true, the output stream must be empty

### files.path
- **Type**: String
- **Description**: Path to the file to check (relative to project directory)

### files.exists
- **Type**: Boolean
- **Description**: When true, the file must exist; when false, the file must not exist

### files.contains
- **Type**: String
- **Description**: The file must contain this exact string

### files.size_min / files.size_max
- **Type**: Integer (bytes)
- **Description**: File size must be within this range (inclusive)