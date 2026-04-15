# Build Issues #402 and #404 Resolution

## Problem Description
Issue #402 indicated that `go build ./...` was failing with the error:
```
go: warning: ignoring go.mod in system temp root /Users/johnjansen/Documents/GitHub/johnjansen/anvil
pattern ./...: directory prefix . does not contain main module or its selected dependencies
```

Issue #404 reported the same error occurring on the main branch, blocking all implementation work.

## Investigation Findings
Upon investigation, we found that:

1. The build commands (`go build ./...` and `go build ./cmd/anvil`) completed successfully with no errors in the main directory
2. All tests pass when running `go test ./...`
3. The go.mod file is correctly configured with the module name `github.com/johnjansen/anvil`
4. The issue occurs specifically in worktrees when the go.work file is not properly configured

## Root Cause Analysis
The error occurs when:
1. Working in a git worktree without a properly configured go.work file
2. The go.work file exists but doesn't include the current module with `use .`

## Solution
To resolve this issue in worktrees:

1. Ensure the go.work file includes the current module:
   ```
   go work use .
   ```

2. Or create a new go.work file if none exists:
   ```
   go work init
   go work use .
   ```

## Possible Causes
This type of error typically occurs when:

1. The Go toolchain is confused about the current directory context
2. There are conflicting Go installations or environment variables
3. Temporary files or cached modules are causing issues
4. Worktrees don't have properly configured go.work files
5. The issue was transient and has resolved itself

## Troubleshooting Steps
If this issue reoccurs, try these steps:

1. Clear Go module cache:
   ```
   go clean -modcache
   ```

2. Check Go environment:
   ```
   go env
   ```

3. Verify GOPATH and GOMOD settings:
   ```
   go env GOPATH
   go env GOMOD
   ```

4. Try building from a clean state:
   ```
   git clean -xfd
   go mod tidy
   go build ./...
   ```

5. Check for conflicting Go installations:
   ```
   which go
   go version
   ```

6. For worktree issues, ensure go.work is properly configured:
   ```
   go work init  # if no go.work exists
   go work use . # add current module to workspace
   ```

## Resolution Status
This issue appears to be resolved as of commit e5248b7. Both builds and tests are passing successfully. The fix involves ensuring that worktrees have properly configured go.work files.

## Additional Notes
Commit ea1aa67 ("fix(#401): add build documentation and troubleshooting guide") suggests that recent work has already been done to address build-related issues and improve documentation. This fix extends that work by specifically addressing worktree-related build issues.