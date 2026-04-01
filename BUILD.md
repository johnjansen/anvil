# Building Anvil

## Prerequisites

- Go 1.24.6 or later
- Git

## Building the Project

### Recommended Approach

Use the provided Makefile to build the project:

```bash
make build
```

This will create the `anvil` binary in the `bin/` directory.

### Manual Build

To build manually, ensure you're in the project root directory (the directory containing `go.mod`):

```bash
# Build the main anvil command
go build ./cmd/anvil

# Or build with version information
go build -ldflags "-X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o bin/anvil ./cmd/anvil
```

### Running Tests

To run all tests:

```bash
go test ./...
```

## Troubleshooting Build Issues

### Error: "directory prefix . does not contain main module or its selected dependencies"

This error occurs when trying to build the project from outside the project directory or in a context where Go cannot properly resolve the module.

**Solution:** Ensure you're running build commands from within the project root directory (the directory containing `go.mod`).

### Error: "go: warning: ignoring go.mod in system temp root"

This warning occurs when Go detects a `go.mod` file in a temporary directory that's interfering with module resolution.

**Solution:** 
1. Make sure you're in the correct project directory
2. If using worktrees, ensure the worktree was created correctly without copying `go.mod` unnecessarily
3. Clear Go's module cache if needed: `go clean -modcache`

## Cross-compilation

To build for different platforms:

```bash
# Build for Linux
GOOS=linux GOARCH=amd64 make build

# Build for Windows
GOOS=windows GOARCH=amd64 make build
```

## Installation

To install the built binary to your system:

```bash
make install
```

This installs the binary to `/usr/local/bin/anvil` by default. You can change the installation prefix:

```bash
PREFIX=/usr/local make install
```