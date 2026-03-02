# Quickstart Guide: Task Wait Conditions

## Overview

This feature allows you to define sophisticated triggering conditions for your tasks, going beyond simple scheduling to support complex workflows based on multiple criteria.

## Basic Configuration

Add trigger conditions to your task configuration:

```yaml
---
schedule: "0 9 * * *"  # Run at 9 AM daily
trigger:
  # AND conditions - all must be true
  when_file_exists: data/input.json
  when_env_set: CI
---
echo "Processing data..."
```

## AND/OR Logic

Specify complex condition combinations:

```yaml
---
trigger:
  # AND conditions (default)
  schedule: "0 9 * * *"
  when_file_exists: data/input.json

  # OR conditions
  or_conditions:
    - when_file_exists: trigger1.txt
    - when_file_exists: trigger2.txt
---
```

## Polling-Based Triggers

Configure tasks that wait for conditions:

```yaml
---
trigger:
  poll_file: data/input.json
  poll_interval: 30s
  timeout: 1h
---
echo "File arrived, processing now..."
```

## Manual Trigger Evaluation

Test your trigger conditions manually:

```bash
anvil task trigger-check my-task
```

## Common Use Cases

### 1. Wait for File and Environment

```yaml
---
trigger:
  schedule: "0 9 * * *"
  when_file_exists: reports/daily.csv
  when_env_set: PRODUCTION
---
process-report.sh
```

### 2. Poll Until File Appears

```yaml
---
trigger:
  poll_file: uploads/user_data.json
  poll_interval: 10s
  timeout: 30m
---
import-user-data.sh
```

### 3. Multiple Trigger Options

```yaml
---
trigger:
  or_conditions:
    - schedule: "0 9 * * *"  # Daily at 9 AM
    - when_file_exists: trigger.txt  # Or when trigger file appears
---
daily-or-trigger.sh
```