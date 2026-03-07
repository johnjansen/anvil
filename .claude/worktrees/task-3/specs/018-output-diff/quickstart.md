# Quickstart: Task Output Diffing

## Compare last two runs

```bash
anvil task diff my-task
```

## Compare specific runs

```bash
anvil task diff my-task --run1 abc123 --run2 def456
```

## Ignore whitespace changes

```bash
anvil task diff my-task --ignore-whitespace
```

## Get JSON output

```bash
anvil task diff my-task --json
```

## Adjust context lines

```bash
anvil task diff my-task --context 5
```
