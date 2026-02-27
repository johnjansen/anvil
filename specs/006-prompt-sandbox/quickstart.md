# Quickstart: Prompt Sandbox

## Test a Task Prompt

```bash
# Run your task's prompt without side effects
anvil prompt sandbox my-task
```

This executes the task's prompt against the LLM and shows the response, token usage, cost estimate, and execution time. No run record is created, no hooks fire.

## Compare Prompt Variations

Create alternative prompt files and compare:

```bash
# Write a variation
echo "Summarize the project status in 3 bullet points" > v1.md
echo "Give a detailed project status report with risks" > v2.md

# Compare the original task prompt with variations
anvil prompt sandbox my-task --compare v1.md v2.md
```

## Watch Mode

Iterate on your prompt with live feedback:

```bash
# Start watching — re-runs on every save
anvil prompt sandbox my-task --watch

# Edit your task file in another terminal
vim .anvil/todos/p1/my-task.md
```

## Machine-Readable Output

```bash
# Get JSON output for scripting
anvil prompt sandbox my-task --json
```
