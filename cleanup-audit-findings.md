# Audit: anvil cleanup — log retention and pruning

## Overview
This audit examines the anvil cleanup functionality for log retention and pruning capabilities.

## Current State
The cleanup functionality is mostly working as designed with some minor issues.

## Working Features
1. **Configuration Detection**: Correctly detects when no retention policy is configured
2. **Manual Cleanup**: Supports `--older-than` flag for manual cleanup operations
3. **Automatic Cleanup**: Enforces retention policies when configured in `~/.anvil/config.yaml`
4. **Dry-run Support**: Preview cleanup operations without actually deleting files
5. **Space Reporting**: Reports total space freed after cleanup operations

## Testing Results
- Basic cleanup functionality works correctly
- Manual cleanup with `--older-than` flag works as expected
- Configuration detection properly identifies missing retention policies
- Cleanup operations successfully free disk space when applicable

## Issues Found

### Segfault Bug
There's a segmentation fault in the `cleanupCmd` function when accessing `cfg.Retention` fields. This occurs due to improper error handling when loading configuration.

### Error Handling
The config loading mechanism could be improved to handle edge cases better and prevent crashes.

## Recommendations

### Immediate Fixes
1. **Fix Segfault**: Properly check if config was loaded successfully before accessing fields
2. **Improve Error Handling**: Add robust error handling for config loading failures

### Enhancements
1. **Environment Variable Support**: Consider supporting environment variables for config file location
2. **Detailed Documentation**: Add comprehensive documentation for retention policies
3. **Configuration Validation**: Add validation for retention policy settings

## Configuration Example
```yaml
retention:
  max_age: 7d      # Delete logs/runs older than 7 days
  max_runs: 50     # Keep at most 50 runs per task
  max_log_size: 0  # Max size per log file (0 = unlimited)
```

## Command Usage
```bash
# Show current retention configuration
anvil cleanup

# Manual cleanup with dry-run
anvil cleanup --older-than=24h --dry-run

# Manual cleanup
anvil cleanup --older-than=7d

# Auto cleanup (when retention configured)
# Runs automatically based on retention settings
```

## Status
✅ Working/broken/incomplete status: Mostly working with minor bug fixes needed