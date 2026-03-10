#!/usr/bin/env bash

# token_audit.sh - Script to audit anvil token usage and cost tracking
# Usage: ./token_audit.sh [since_date] [project_path]

set -euo pipefail

SINCE_DATE="${1:-2026-02-28}"
PROJECT_PATH="${2:-$HOME}"

echo "🔍 Auditing anvil token usage since $SINCE_DATE"
echo "📂 Project path: $PROJECT_PATH"
echo

# Check if anvil is installed
if ! command -v anvil &> /dev/null; then
    echo "❌ anvil command not found. Please install anvil first."
    exit 1
fi

# Check if daemon is running for budget information
if anvil status &> /dev/null; then
    echo "✅ anvil daemon is running"
    HAS_DAEMON=1
else
    echo "⚠️  anvil daemon not running - budget information will be unavailable"
    HAS_DAEMON=0
fi

echo
echo "📊 Token Usage Summary:"
echo "======================"

# Run anvil usage command
if anvil usage --since "$SINCE_DATE" &> /tmp/anvil_usage.txt; then
    cat /tmp/anvil_usage.txt
else
    echo "❌ Failed to get usage data"
    echo "💡 Try running: anvil usage --since $SINCE_DATE"
fi

echo
echo "💰 Cost Analysis:"
echo "================="

# Run anvil usage with cost information
if anvil usage --since "$SINCE_DATE" --json &> /tmp/anvil_usage_json.txt; then
    # Parse JSON to extract cost information
    TOTAL_COST=$(jq -r '.total_cost // 0' /tmp/anvil_usage_json.txt)
    TOTAL_RUNS=$(jq -r '.total_runs // 0' /tmp/anvil_usage_json.txt)

    echo "Total runs since $SINCE_DATE: $TOTAL_RUNS"
    echo "Estimated total cost: \$${TOTAL_COST}"

    if [ "$TOTAL_RUNS" -gt 0 ]; then
        AVG_COST=$(echo "scale=4; $TOTAL_COST / $TOTAL_RUNS" | bc -l)
        echo "Average cost per run: \$${AVG_COST}"
    fi
else
    echo "❌ Failed to get cost analysis data"
fi

echo
echo "📈 Top Tasks by Cost:"
echo "====================="

# Get top 5 most expensive tasks
if anvil usage --since "$SINCE_DATE" --json &> /tmp/anvil_top_tasks.txt; then
    jq -r '.tasks | sort_by(.cost) | reverse | .[0:5] | .[] | "\(.task_name // .TaskName): \$\(.cost // .Cost)"' /tmp/anvil_top_tasks.txt 2>/dev/null || echo "No task data available"
fi

if [ "$HAS_DAEMON" -eq 1 ]; then
    echo
    echo "💸 Budget Status:"
    echo "================="

    # Try to get budget information
    if anvil usage --budget &> /tmp/anvil_budget.txt; then
        cat /tmp/anvil_budget.txt
    else
        echo "❌ Failed to get budget information"
    fi
fi

echo
echo "📋 Configuration Check:"
echo "======================"

# Check config for token rates
CONFIG_FILE="$HOME/.anvil/config.yaml"
if [ -f "$CONFIG_FILE" ]; then
    echo "Found config file: $CONFIG_FILE"
    INPUT_RATE=$(grep "input_token_rate" "$CONFIG_FILE" 2>/dev/null | cut -d: -f2 | tr -d ' ' || echo "default")
    OUTPUT_RATE=$(grep "output_token_rate" "$CONFIG_FILE" 2>/dev/null | cut -d: -f2 | tr -d ' ' || echo "default")

    if [ "$INPUT_RATE" = "default" ]; then
        echo "Input token rate: \$3.00/1M tokens (default)"
    else
        echo "Input token rate: \$$INPUT_RATE/1M tokens"
    fi

    if [ "$OUTPUT_RATE" = "default" ]; then
        echo "Output token rate: \$15.00/1M tokens (default)"
    else
        echo "Output token rate: \$$OUTPUT_RATE/1M tokens"
    fi
else
    echo "No config file found at $CONFIG_FILE"
    echo "Using default rates: \$3.00/1M input, \$15.00/1M output"
fi

echo
echo "✅ Audit complete!"

# Cleanup
rm -f /tmp/anvil_usage.txt /tmp/anvil_usage_json.txt /tmp/anvil_top_tasks.txt /tmp/anvil_budget.txt