---
name: anvil-feedback
description: Request anvil features or report anvil bugs directly from your Claude Code session. Use when users say things like "request an anvil feature", "log an anvil issue", "report an anvil bug", "anvil feature request", "anvil bug report", "suggest anvil improvement", or "I found an anvil bug".
---

# Anvil Feedback

Use this skill to create GitHub issues on the `johnjansen/anvil` repository without leaving your workflow.

## Feature Request

When the user wants to request a feature or suggest an improvement:

1. If not already provided, ask for:
   - **Title**: A short, descriptive title for the feature
   - **Description**: What they want and why it would be useful

2. Create the issue:

```bash
gh issue create \
  --repo johnjansen/anvil \
  --title "<title>" \
  --label "enhancement" \
  --body "<description>"
```

3. Return the issue URL to the user.

## Bug Report

When the user wants to report a bug:

1. If not already provided, ask for:
   - **Title**: A short description of the bug
   - **Description**: What happened, what was expected, and steps to reproduce (if known)

2. Create the issue:

```bash
gh issue create \
  --repo johnjansen/anvil \
  --title "<title>" \
  --label "bug" \
  --body "<description>"
```

3. Return the issue URL to the user.

## Notes

- Use `--label "enhancement"` for feature requests, improvements, or suggestions
- Use `--label "bug"` for unexpected behavior, errors, or broken functionality
- If the user provides a title and description inline (e.g. "report a bug: anvil ps crashes when daemon isn't running"), use them directly without asking follow-up questions
- Always confirm the issue URL after creation so the user can track it
