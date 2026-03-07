#!/bin/bash

# Task health monitor - Detect and reset stale in-progress labels on GitHub issues

echo "##anvil:status Checking for stale in-progress issues..."

# Get all open issues with in-progress label
issues=$(gh issue list --label in-progress --state open --json number,title,updatedAt)

# Check if there are any issues
if [[ "$issues" == "[]" ]]; then
    echo "##anvil:status No in-progress issues found"
    exit 0
fi

# Parse the issues and check each one
echo "$issues" | jq -c '.[]' | while read -r issue; do
    number=$(echo "$issue" | jq -r '.number')
    title=$(echo "$issue" | jq -r '.title')
    updated_at=$(echo "$issue" | jq -r '.updatedAt')

    echo "Checking issue #$number: $title (last updated: $updated_at)"

    # Check if any anvil task is actively working on this issue
    anvil_task_active=false

    # Try to find anvil tasks working on this issue
    anvil_tasks=$(anvil ps 2>/dev/null)
    if [[ $? -eq 0 ]] && [[ -n "$anvil_tasks" ]]; then
        # Check if any task references this issue number in its logs
        while read -r line; do
            if [[ "$line" =~ "#$number" ]]; then
                anvil_task_active=true
                break
            fi
        done < <(echo "$anvil_tasks")
    fi

    # Calculate time difference in hours (macOS compatible)
    current_time=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    updated_epoch=$(date -jf "%Y-%m-%dT%H:%M:%SZ" "$updated_at" +%s 2>/dev/null)

    # Fallback for GNU date if macOS date fails
    if [[ -z "$updated_epoch" ]]; then
        updated_epoch=$(date -d "$updated_at" +%s 2>/dev/null)
    fi

    current_epoch=$(date -jf "%Y-%m-%dT%H:%M:%SZ" "$current_time" +%s 2>/dev/null)

    # Fallback for GNU date if macOS date fails
    if [[ -z "$current_epoch" ]]; then
        current_epoch=$(date -d "$current_time" +%s 2>/dev/null)
    fi

    # If we couldn't get epoch times, default to treating the issue as active
    if [[ -z "$updated_epoch" ]] || [[ -z "$current_epoch" ]]; then
        echo "Issue #$number is still active (could not calculate time difference)"
        continue
    fi

    time_diff_seconds=$((current_epoch - updated_epoch))
    time_diff_hours=$((time_diff_seconds / 3600))

    # If no anvil task is working on this issue AND no recent activity in last 2 hours
    if [[ "$anvil_task_active" == false ]] && [[ $time_diff_hours -ge 2 ]]; then
        echo "##anvil:status Reset stale label on issue #$number"

        # Remove in-progress label and add ready label
        gh issue edit "$number" --remove-label "in-progress" --add-label "ready" 2>/dev/null

        if [[ $? -eq 0 ]]; then
            echo "Successfully reset labels on issue #$number"
        else
            echo "Failed to reset labels on issue #$number"
        fi
    else
        echo "Issue #$number is still active (updated ${time_diff_hours} hours ago)"
    fi
done

echo "##anvil:status All in-progress issues checked"