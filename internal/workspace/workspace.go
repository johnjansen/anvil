package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// WorkspaceType defines the type of workspace isolation.
const (
	TypeProject    = "project"    // default: task runs in project root
	TypeRestricted = "restricted" // task has path-based access restrictions
	TypeTemp       = "temp"       // task runs in an ephemeral temp directory
)

// Config holds workspace isolation settings for a task.
type Config struct {
	Type         string   `yaml:"type"`
	AllowedPaths []string `yaml:"allowed_paths"`
	ReadOnly     []string `yaml:"read_only"`
	BlockedPaths []string `yaml:"blocked_paths"`
	Size         string   `yaml:"size"`
}

// IsZero returns true if no workspace configuration was specified.
func (c Config) IsZero() bool {
	return c.Type == "" && len(c.AllowedPaths) == 0 && len(c.ReadOnly) == 0 &&
		len(c.BlockedPaths) == 0 && c.Size == ""
}

// EffectiveType returns the workspace type, defaulting to "project" if empty.
func (c Config) EffectiveType() string {
	if c.Type == "" {
		return TypeProject
	}
	return c.Type
}

// ValidateConfig validates workspace configuration against the project root.
// It resolves all paths relative to projectRoot, rejects paths that escape
// the project root, resolves symlinks, and validates type-specific field usage.
func ValidateConfig(projectRoot string, cfg *Config) error {
	if cfg == nil || cfg.IsZero() {
		return nil
	}

	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("resolving project root: %w", err)
	}
	absRoot = filepath.Clean(absRoot) + string(filepath.Separator)

	// Validate type
	switch cfg.EffectiveType() {
	case TypeProject, TypeRestricted, TypeTemp:
		// valid
	default:
		return fmt.Errorf("workspace type %q not recognized (must be project, restricted, or temp)", cfg.Type)
	}

	// Validate field applicability
	if cfg.EffectiveType() != TypeRestricted {
		if len(cfg.AllowedPaths) > 0 {
			return fmt.Errorf("allowed_paths only valid for workspace type %q", TypeRestricted)
		}
		if len(cfg.ReadOnly) > 0 {
			return fmt.Errorf("read_only only valid for workspace type %q", TypeRestricted)
		}
	}
	if cfg.EffectiveType() != TypeTemp && cfg.Size != "" {
		return fmt.Errorf("size only valid for workspace type %q", TypeTemp)
	}

	// Validate and resolve paths for restricted type
	if cfg.EffectiveType() == TypeRestricted {
		for i, p := range cfg.AllowedPaths {
			resolved, err := resolvePath(absRoot, p)
			if err != nil {
				return fmt.Errorf("allowed_paths[%d] %q: %w", i, p, err)
			}
			cfg.AllowedPaths[i] = resolved
		}
		for i, p := range cfg.ReadOnly {
			resolved, err := resolvePath(absRoot, p)
			if err != nil {
				return fmt.Errorf("read_only[%d] %q: %w", i, p, err)
			}
			cfg.ReadOnly[i] = resolved
		}
	}

	// Validate blocked_paths (valid for any type)
	for i, p := range cfg.BlockedPaths {
		// Blocked paths can reference paths outside project (e.g. ~/.ssh/)
		resolved := filepath.Clean(expandHome(p))
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(strings.TrimSuffix(absRoot, string(filepath.Separator)), resolved)
		}
		cfg.BlockedPaths[i] = resolved
	}

	// Validate size for temp type
	if cfg.Size != "" {
		if _, err := ParseSize(cfg.Size); err != nil {
			return fmt.Errorf("workspace size %q: %w", cfg.Size, err)
		}
	}

	return nil
}

// resolvePath resolves a relative path against the project root and ensures
// it doesn't escape the project directory.
func resolvePath(absRoot, p string) (string, error) {
	resolved := filepath.Clean(filepath.Join(strings.TrimSuffix(absRoot, string(filepath.Separator)), p))
	if !strings.HasPrefix(resolved+string(filepath.Separator), absRoot) && resolved != strings.TrimSuffix(absRoot, string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes project root")
	}
	// Try to resolve symlinks (non-fatal if path doesn't exist yet)
	if real, err := filepath.EvalSymlinks(resolved); err == nil {
		if !strings.HasPrefix(real+string(filepath.Separator), absRoot) && real != strings.TrimSuffix(absRoot, string(filepath.Separator)) {
			return "", fmt.Errorf("symlink resolves outside project root")
		}
		resolved = real
	}
	return resolved, nil
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[1:])
	}
	return p
}

// EnvVars generates workspace-related environment variables for task execution.
func EnvVars(projectRoot string, cfg Config) map[string]string {
	env := make(map[string]string)

	absRoot, _ := filepath.Abs(projectRoot)

	wsType := cfg.EffectiveType()
	env["ANVIL_WORKSPACE_TYPE"] = wsType
	env["ANVIL_WORKSPACE_ROOT"] = absRoot

	if wsType == TypeRestricted {
		if len(cfg.AllowedPaths) > 0 {
			env["ANVIL_WORKSPACE_ALLOWED"] = strings.Join(cfg.AllowedPaths, ",")
		}
		if len(cfg.ReadOnly) > 0 {
			env["ANVIL_WORKSPACE_READONLY"] = strings.Join(cfg.ReadOnly, ",")
		}
	}

	if len(cfg.BlockedPaths) > 0 {
		env["ANVIL_WORKSPACE_BLOCKED"] = strings.Join(cfg.BlockedPaths, ",")
	}

	return env
}

// ParseSize parses a human-readable size string (e.g., "100mb", "1gb", "500kb")
// into bytes. Supported suffixes: b, kb, mb, gb, tb (case-insensitive).
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty size string")
	}

	multipliers := map[string]int64{
		"b":  1,
		"kb": 1024,
		"mb": 1024 * 1024,
		"gb": 1024 * 1024 * 1024,
		"tb": 1024 * 1024 * 1024 * 1024,
	}

	for suffix, mult := range multipliers {
		if strings.HasSuffix(s, suffix) {
			numStr := strings.TrimSuffix(s, suffix)
			numStr = strings.TrimSpace(numStr)
			n, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid number %q: %w", numStr, err)
			}
			if n < 0 {
				return 0, fmt.Errorf("size must be positive")
			}
			return int64(n * float64(mult)), nil
		}
	}

	// Try plain number as bytes
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unrecognized size format %q", s)
	}
	return n, nil
}

// CreateTempWorkspace creates a temporary directory for task execution.
// Returns the directory path and a cleanup function that removes it.
func CreateTempWorkspace(prefix string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "anvil-workspace-"+prefix+"-")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp workspace: %w", err)
	}
	cleanup := func() {
		os.RemoveAll(dir)
	}
	return dir, cleanup, nil
}

// CheckSize walks a directory and returns the total size in bytes.
// If maxBytes > 0, also returns whether the total exceeds the limit.
func CheckSize(dir string, maxBytes int64) (actualBytes int64, exceeded bool) {
	filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable files
		}
		if !info.IsDir() {
			actualBytes += info.Size()
		}
		return nil
	})
	if maxBytes > 0 {
		exceeded = actualBytes > maxBytes
	}
	return
}
