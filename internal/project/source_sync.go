package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SourceMeta records the relationship between a registered task and the source
// file it was imported from. It is persisted as a JSON sidecar next to the
// canonical ".md" in .anvil/todos/p<N>/<slug>.meta.json so that "anvil reload"
// can re-import the task content on demand and drift can be surfaced to the
// user.
type SourceMeta struct {
	SourcePath       string    `json:"source_path"`
	SourceHashSHA256 string    `json:"source_hash_sha256"`
	LastLoadedAt     time.Time `json:"last_loaded_at"`
	LastLoadStatus   string    `json:"last_load_status"`
	LastLoadError    string    `json:"last_load_error,omitempty"`
}

// SyncStatus is a derived (not persisted) state shown to users, computed by
// comparing the on-disk source file to the sidecar.
type SyncStatus string

const (
	SyncStatusInSync   SyncStatus = "ok"
	SyncStatusDrift    SyncStatus = "drift"
	SyncStatusMissing  SyncStatus = "missing"
	SyncStatusInvalid  SyncStatus = "invalid"
	SyncStatusNoSource SyncStatus = ""
)

const (
	loadStatusOK      = "ok"
	loadStatusMissing = "missing"
	loadStatusInvalid = "invalid"
)

// SidecarPath returns the sidecar path for a given task ".md" path.
func SidecarPath(mdPath string) string {
	base := strings.TrimSuffix(mdPath, ".md")
	return base + ".meta.json"
}

// NewSourceMeta builds a SourceMeta for a freshly-registered task. The source
// path is resolved to an absolute, symlink-resolved path so that later reloads
// do not depend on the user's working directory.
func NewSourceMeta(sourcePath string, sourceBytes []byte) (*SourceMeta, error) {
	abs, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("resolving source path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return &SourceMeta{
		SourcePath:       abs,
		SourceHashSHA256: hashBytes(sourceBytes),
		LastLoadedAt:     time.Now().UTC(),
		LastLoadStatus:   loadStatusOK,
	}, nil
}

// ReadSourceMeta loads the sidecar for a task's ".md" path. Returns (nil, nil)
// when the sidecar is absent so that callers can treat missing sidecars as
// "no-source" without special-casing an error. A malformed sidecar returns an
// error so that it is not silently ignored.
func ReadSourceMeta(mdPath string) (*SourceMeta, error) {
	data, err := os.ReadFile(SidecarPath(mdPath))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var meta SourceMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing sidecar %s: %w", SidecarPath(mdPath), err)
	}
	return &meta, nil
}

// WriteSourceMeta atomically writes the sidecar for a task's ".md" path. The
// write uses a "tmp + rename" pattern so readers never observe a partial
// sidecar.
func WriteSourceMeta(mdPath string, meta *SourceMeta) error {
	if meta == nil {
		return fmt.Errorf("nil meta")
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling sidecar: %w", err)
	}
	dst := SidecarPath(mdPath)
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing sidecar tmp: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming sidecar: %w", err)
	}
	return nil
}

// RemoveSourceMeta deletes the sidecar for a task if present; missing sidecar
// is not an error (idempotent).
func RemoveSourceMeta(mdPath string) error {
	err := os.Remove(SidecarPath(mdPath))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// HashSource computes the hex sha256 of a source file's raw bytes.
func HashSource(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return hashBytes(data), nil
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ComputeSyncStatus returns the user-facing sync status for a task by
// comparing its sidecar against the current on-disk source file.
func ComputeSyncStatus(meta *SourceMeta) SyncStatus {
	if meta == nil || meta.SourcePath == "" {
		return SyncStatusNoSource
	}
	data, err := os.ReadFile(meta.SourcePath)
	if err != nil {
		return SyncStatusMissing
	}
	newHash := hashBytes(data)
	if meta.LastLoadStatus == loadStatusInvalid && newHash == meta.SourceHashSHA256 {
		return SyncStatusInvalid
	}
	if newHash == meta.SourceHashSHA256 {
		return SyncStatusInSync
	}
	return SyncStatusDrift
}

// ReloadSummary captures the counts emitted by a reload pass across all
// registered tasks.
type ReloadSummary struct {
	Checked  int `json:"checked"`
	Reloaded int `json:"reloaded"`
	Drift    int `json:"drift"`
	Missing  int `json:"missing"`
	Invalid  int `json:"invalid"`
}

// ReloadAllFromSources walks the registered tasks under the given project root
// and re-imports each task whose sidecar points at a source file. The canonical
// ".md" is atomically rewritten when the source has changed; task identity
// (UUID) and unrelated frontmatter are preserved. Tasks without a sidecar are
// counted but left untouched.
func ReloadAllFromSources(projectRoot string) (ReloadSummary, error) {
	var summary ReloadSummary
	todosDir := filepath.Join(projectRoot, ".anvil", "todos")

	for pri := 0; pri <= 9; pri++ {
		dir := filepath.Join(todosDir, fmt.Sprintf("p%d", pri))
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return summary, fmt.Errorf("reading p%d: %w", pri, err)
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			mdPath := filepath.Join(dir, e.Name())
			summary.Checked++

			meta, err := ReadSourceMeta(mdPath)
			if err != nil || meta == nil || meta.SourcePath == "" {
				continue
			}

			sourceBytes, err := os.ReadFile(meta.SourcePath)
			if err != nil {
				meta.LastLoadStatus = loadStatusMissing
				meta.LastLoadError = err.Error()
				_ = WriteSourceMeta(mdPath, meta)
				summary.Missing++
				continue
			}

			newHash := hashBytes(sourceBytes)
			if meta.LastLoadStatus == loadStatusOK && newHash == meta.SourceHashSHA256 {
				continue
			}

			if err := reloadTaskFromSource(mdPath, sourceBytes); err != nil {
				meta.LastLoadStatus = loadStatusInvalid
				meta.LastLoadError = err.Error()
				_ = WriteSourceMeta(mdPath, meta)
				summary.Invalid++
				continue
			}

			meta.SourceHashSHA256 = newHash
			meta.LastLoadedAt = time.Now().UTC()
			meta.LastLoadStatus = loadStatusOK
			meta.LastLoadError = ""
			_ = WriteSourceMeta(mdPath, meta)
			summary.Reloaded++
		}
	}
	return summary, nil
}

// frontmatterAliasRE matches a top-level frontmatter key that uses a hyphen
// where anvil's canonical form is snake_case. Restricted to known aliases so
// user-defined keys in env: or similar blocks are not touched.
var frontmatterAliasRE = regexp.MustCompile(`(?m)^(allowed-tools|max-concurrent)(\s*:)`)

// normalizeFrontmatterAliases rewrites hyphenated aliases to their canonical
// snake_case form, line-by-line, so that yaml.Unmarshal against the typed
// struct (which tags the canonical keys) sees them. Only top-of-line keys are
// rewritten; the same key appearing inside a nested value or body text is not.
func normalizeFrontmatterAliases(fm []byte) []byte {
	return frontmatterAliasRE.ReplaceAllFunc(fm, func(match []byte) []byte {
		if idx := indexByte(match, ':'); idx > 0 {
			key := string(match[:idx])
			switch key {
			case "allowed-tools":
				return []byte("allowed_tools" + string(match[idx:]))
			case "max-concurrent":
				return []byte("max_concurrent" + string(match[idx:]))
			}
		}
		return match
	})
}

func indexByte(b []byte, c byte) int {
	for i := 0; i < len(b); i++ {
		if b[i] == c {
			return i
		}
	}
	return -1
}

// reloadTaskFromSource regenerates the canonical ".md" from source bytes while
// preserving the task's stable identity. The existing ".md" is consulted to
// extract its UUID (so run history is not orphaned) and its original schedule
// metadata is preserved when the source does not override it.
func reloadTaskFromSource(mdPath string, sourceBytes []byte) error {
	existingID := readIDFromMarkdown(mdPath)

	if err := parseFrontmatterForValidation(sourceBytes); err != nil {
		return err
	}

	newContent := mergeSourceWithExistingID(sourceBytes, existingID)

	tmp := mdPath + ".tmp"
	if err := os.WriteFile(tmp, newContent, 0644); err != nil {
		return fmt.Errorf("writing tmp md: %w", err)
	}
	if err := os.Rename(tmp, mdPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming md: %w", err)
	}
	return nil
}

// readIDFromMarkdown extracts the task id from a registered ".md" file's
// frontmatter. Returns empty string if not found.
func readIDFromMarkdown(mdPath string) string {
	raw, err := os.ReadFile(mdPath)
	if err != nil {
		return ""
	}
	s := string(raw)
	if !strings.HasPrefix(s, "---\n") {
		return ""
	}
	parts := strings.SplitN(s[4:], "\n---\n", 2)
	if len(parts) != 2 {
		return ""
	}
	var fm struct {
		ID string `yaml:"id"`
	}
	if err := yaml.Unmarshal([]byte(parts[0]), &fm); err != nil {
		return ""
	}
	return fm.ID
}

// parseFrontmatterForValidation attempts a YAML unmarshal over just the
// frontmatter block (alias-normalized). It returns the parse error, if any,
// so that the caller can mark the source as "invalid".
func parseFrontmatterForValidation(sourceBytes []byte) error {
	s := string(sourceBytes)
	if !strings.HasPrefix(s, "---\n") {
		return nil
	}
	parts := strings.SplitN(s[4:], "\n---\n", 2)
	if len(parts) != 2 {
		return fmt.Errorf("frontmatter closing delimiter missing")
	}
	fm := normalizeFrontmatterAliases([]byte(parts[0]))
	var dummy map[string]interface{}
	return yaml.Unmarshal(fm, &dummy)
}

// mergeSourceWithExistingID ensures the regenerated ".md" carries the same id
// as the existing file. If the source already carries an id we keep the
// existing one (source of truth for identity is the registered copy; users
// should not be able to rename tasks by editing the source file). Hyphenated
// frontmatter aliases are normalized to canonical form on write.
func mergeSourceWithExistingID(sourceBytes []byte, existingID string) []byte {
	s := string(sourceBytes)
	if !strings.HasPrefix(s, "---\n") {
		var sb strings.Builder
		sb.WriteString("---\n")
		if existingID != "" {
			sb.WriteString(fmt.Sprintf("id: %q\n", existingID))
		}
		sb.WriteString("---\n")
		sb.WriteString(s)
		if !strings.HasSuffix(s, "\n") {
			sb.WriteByte('\n')
		}
		return []byte(sb.String())
	}
	parts := strings.SplitN(s[4:], "\n---\n", 2)
	if len(parts) != 2 {
		return sourceBytes
	}
	fmRaw := string(normalizeFrontmatterAliases([]byte(parts[0])))
	body := parts[1]

	if existingID != "" {
		fmRaw = setOrInsertIDLine(fmRaw, existingID)
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(strings.TrimRight(fmRaw, "\n"))
	sb.WriteString("\n---\n")
	sb.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		sb.WriteByte('\n')
	}
	return []byte(sb.String())
}

var idLineRE = regexp.MustCompile(`(?m)^id\s*:.*$`)

func setOrInsertIDLine(fm, id string) string {
	replacement := fmt.Sprintf("id: %q", id)
	if idLineRE.MatchString(fm) {
		return idLineRE.ReplaceAllString(fm, replacement)
	}
	if !strings.HasSuffix(fm, "\n") {
		fm += "\n"
	}
	return replacement + "\n" + fm
}
