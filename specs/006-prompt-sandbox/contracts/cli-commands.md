# CLI Commands: Prompt Sandbox

## `anvil prompt sandbox <task> [flags]`

Run a task's prompt in sandbox mode (no side effects).

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `<task>` | Yes | Task name to sandbox |

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--compare` | `[]string` | - | One or more variation files to compare |
| `--watch` | `bool` | false | Watch task file for changes and re-run |
| `--json` | `bool` | false | Output results in JSON format |

### Examples

```bash
# Basic sandbox run
anvil prompt sandbox my-task

# Compare variations
anvil prompt sandbox my-task --compare v1.md v2.md

# Watch mode
anvil prompt sandbox my-task --watch

# JSON output
anvil prompt sandbox my-task --json
```

### Text Output Format

```
=== Sandbox: my-task ===

[LLM response text here...]

--- Stats ---
Tokens:   1,234 in / 567 out
Cost:     $0.0122
Duration: 12.3s
Runner:   claude (index 0)
```

### JSON Output Format

```json
{
  "label": "default",
  "response": "LLM response text...",
  "input_tokens": 1234,
  "output_tokens": 567,
  "estimated_cost_usd": 0.0122,
  "duration_ms": 12300,
  "runner_index": 0,
  "error": ""
}
```

### Compare Output Format (text)

```
=== Variation 1: default ===

[response...]

--- Stats ---
Tokens:   1,234 in / 567 out
Cost:     $0.0122
Duration: 12.3s

=== Variation 2: v1.md ===

[response...]

--- Stats ---
Tokens:   1,100 in / 490 out
Cost:     $0.0107
Duration: 10.1s

=== Summary ===
VARIATION        TOKENS IN   TOKENS OUT  COST      DURATION
default          1,234       567         $0.0122   12.3s
v1.md            1,100       490         $0.0107   10.1s
```

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Sandbox completed successfully |
| 1 | Error (task not found, runner failed, etc.) |
