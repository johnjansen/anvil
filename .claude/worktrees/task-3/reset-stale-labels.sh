#!/bin/bash

# Script to detect and reset stale in-progress labels on GitHub issues

echo "##anvil:status Checking for stale in-progress issues..."

# List all open issues with in-progress label
ISSUES=$(gh issue list --label in-progress --state open --json number,updatedAt --jq '.[] | "\(.number):\(.updatedAt)"')

# If no issues with in-progress label, exit
if [ -z "$ISSUES" ]; then
    echo "##anvil:status No issues with in-progress label found"
    exit 0
fi

# Process each issue
for ISSUE in $ISSUES; do
    ISSUE_NUMBER=$(echo $ISSUE | cut -d':' -f1)
    UPDATED_AT=$(echo $ISSUE | cut -d':' -f2)

    echo "Checking issue #$ISSUE_NUMBER (last updated: $UPDATED_AT)"

    # Check if any anvil task is actively working on this issue
    # We'll look for the issue number in the running task logs
    TASK_WORKING_ON_ISSUE=false

    # Get list of running tasks
    RUNNING_TASKS=$(anvil ps --json 2>/dev/null | jq -r '.[].task' 2>/dev/null)

    if [ -n "$RUNNING_TASKS" ]; then
        # Check each running task's logs for the issue number
        while IFS= read -r task; do
            if anvil task log "$task" 2>/dev/null | grep -q "#$ISSUE_NUMBER"; then
                TASK_WORKING_ON_ISSUE=true
                break
            fi
        done <<< "$RUNNING_TASKS"
    fi

    # Check if the issue has had any activity in the last 2 hours
    RECENT_ACTIVITY=false

    # Convert updated_at to timestamp
    UPDATED_TS=$(date -jf "%Y-%m-%dT%H:%M:%SZ" "$UPDATED_AT" +%s 2>/dev/null)
    CURRENT_TS=$(date +%s)

    if [ -n "$UPDATED_TS" ]; then
        # Calculate difference in seconds
        DIFF=$((CURRENT_TS - UPDATED_TS))
        # If difference is less than 2 hours (7200 seconds), consider it recent
        if [ $DIFF -lt 7200 ]; then
            RECENT_ACTIVITY=true
        fi
    fi

    # If NO anvil task is currently working on that issue AND no recent activity
    if [ "$TASK_WORKING_ON_ISSUE" = false ] && [ "$RECENT_ACTIVITY" = false ]; then
        echo "##anvil:status Reset stale label on issue #$ISSUE_NUMBER"

        # Remove the 'in-progress' label
        gh issue edit $ISSUE_NUMBER --remove-label in-progress 2>/dev/null

        # Add the 'ready' label
        gh issue edit $ISSUE_NUMBER --add-label ready 2>/dev/null
    else
        echo "##anvil:status Issue #$ISSUE_NUMBER is still active (working: $TASK_WORKING_ON_ISSUE, recent activity: $RECENT_ACTIVITY)"
    fi
done

echo "##anvil:status All in-progress issues checked"