package audit

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"
)

// Operation constants for audit log entries.
const (
	OpTaskCreated   = "task.created"
	OpTaskModified  = "task.modified"
	OpTaskDeleted   = "task.deleted"
	OpTaskRun       = "task.run"
	OpTaskCompleted = "task.completed"
	OpTaskPaused    = "task.paused"
	OpTaskResumed   = "task.resumed"
)

const auditLogFile = "audit.jsonl"

// AuditEntry represents a single record in the append-only audit log.
type AuditEntry struct {
	Timestamp   string         `json:"timestamp"`
	Operation   string         `json:"operation"`
	Actor       string         `json:"actor"`
	TaskName    string         `json:"task"`
	ProjectPath string         `json:"project"`
	Details     map[string]any `json:"details,omitempty"`
	PrevHash    string         `json:"prev_hash"`
	Signature   string         `json:"signature"`
}

// auditLogPath returns the path to the audit log file.
func auditLogPath(projectPath string) string {
	return filepath.Join(projectPath, ".anvil", auditLogFile)
}

// currentActor returns the current OS username or "unknown".
func currentActor() string {
	u, err := user.Current()
	if err != nil {
		return "unknown"
	}
	return u.Username
}

// hashEntry computes SHA256 of the JSON bytes of an entry.
func hashEntry(entryJSON []byte) string {
	h := sha256.Sum256(entryJSON)
	return hex.EncodeToString(h[:])
}

// lastEntryJSON reads the last line from the audit log file.
func lastEntryJSON(projectPath string) ([]byte, error) {
	f, err := os.Open(auditLogPath(projectPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var lastLine []byte
	scanner := bufio.NewScanner(f)
	// Increase buffer for large lines
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) > 0 {
			lastLine = make([]byte, len(line))
			copy(lastLine, line)
		}
	}
	return lastLine, scanner.Err()
}

// AppendEntry appends an audit entry to the log with chain hashing and signing.
func AppendEntry(projectPath string, entry AuditEntry) error {
	key, err := LoadOrCreateKey(projectPath)
	if err != nil {
		return fmt.Errorf("loading signing key: %w", err)
	}

	// Compute prev_hash from last entry
	lastJSON, err := lastEntryJSON(projectPath)
	if err != nil {
		return fmt.Errorf("reading last entry: %w", err)
	}
	if lastJSON != nil {
		entry.PrevHash = hashEntry(lastJSON)
	}

	// Serialize without signature for signing
	entry.Signature = ""
	unsigned, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling entry: %w", err)
	}

	// Sign and set signature
	entry.Signature = Sign(key, unsigned)

	// Final serialization with signature
	signed, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling signed entry: %w", err)
	}

	// Ensure .anvil directory exists
	logPath := auditLogPath(projectPath)
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return fmt.Errorf("creating .anvil dir: %w", err)
	}

	// Append to audit log
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening audit log: %w", err)
	}
	defer f.Close()

	signed = append(signed, '\n')
	_, err = f.Write(signed)
	return err
}

// ReadEntries reads all audit entries from the log.
func ReadEntries(projectPath string) ([]AuditEntry, error) {
	f, err := os.Open(auditLogPath(projectPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("opening audit log: %w", err)
	}
	defer f.Close()

	var entries []AuditEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry AuditEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

// VerifyChain walks all entries and verifies chain integrity and signatures.
func VerifyChain(projectPath string) (bool, []string) {
	key, err := LoadOrCreateKey(projectPath)
	if err != nil {
		return false, []string{fmt.Sprintf("failed to load signing key: %v", err)}
	}

	f, err := os.Open(auditLogPath(projectPath))
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil // no log = valid
		}
		return false, []string{fmt.Sprintf("failed to open audit log: %v", err)}
	}
	defer f.Close()

	var errors []string
	var prevJSON []byte
	lineNum := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry AuditEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			errors = append(errors, fmt.Sprintf("entry #%d: malformed JSON: %v", lineNum, err))
			prevJSON = make([]byte, len(line))
			copy(prevJSON, line)
			continue
		}

		// Verify prev_hash
		if prevJSON != nil {
			expectedHash := hashEntry(prevJSON)
			if entry.PrevHash != expectedHash {
				errors = append(errors, fmt.Sprintf("entry #%d: chain broken (expected hash %s, got %s)", lineNum, expectedHash[:12], entry.PrevHash[:min(12, len(entry.PrevHash))]))
			}
		} else if entry.PrevHash != "" {
			errors = append(errors, fmt.Sprintf("entry #%d: first entry should have empty prev_hash", lineNum))
		}

		// Verify signature: re-serialize without signature field
		savedSig := entry.Signature
		entry.Signature = ""
		unsigned, _ := json.Marshal(entry)
		if !Verify(key, unsigned, savedSig) {
			errors = append(errors, fmt.Sprintf("entry #%d: invalid signature", lineNum))
		}

		// Store full line as-is for next hash computation
		prevJSON = make([]byte, len(line))
		copy(prevJSON, line)
	}

	if scanErr := scanner.Err(); scanErr != nil {
		errors = append(errors, fmt.Sprintf("scan error: %v", scanErr))
	}

	return len(errors) == 0, errors
}

// LogOperation is a convenience wrapper that creates an AuditEntry and appends it.
// Errors are logged to stderr but never returned (best-effort).
func LogOperation(projectPath, operation, taskName string, details map[string]any) {
	entry := AuditEntry{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Operation:   operation,
		Actor:       currentActor(),
		TaskName:    taskName,
		ProjectPath: projectPath,
		Details:     details,
	}
	if err := AppendEntry(projectPath, entry); err != nil {
		fmt.Fprintf(os.Stderr, "anvil: audit log error: %v\n", err)
	}
}

// LogOperationWithActor is like LogOperation but allows specifying the actor.
func LogOperationWithActor(projectPath, operation, actor, taskName string, details map[string]any) {
	entry := AuditEntry{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Operation:   operation,
		Actor:       actor,
		TaskName:    taskName,
		ProjectPath: projectPath,
		Details:     details,
	}
	if err := AppendEntry(projectPath, entry); err != nil {
		fmt.Fprintf(os.Stderr, "anvil: audit log error: %v\n", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
