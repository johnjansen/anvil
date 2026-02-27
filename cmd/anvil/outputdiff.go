package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/project"
)

// DiffLine represents a single line in a diff hunk.
type DiffLine struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// DiffHunk represents a single hunk in the diff output.
type DiffHunk struct {
	OldStart int        `json:"old_start"`
	OldCount int        `json:"old_count"`
	NewStart int        `json:"new_start"`
	NewCount int        `json:"new_count"`
	Lines    []DiffLine `json:"lines"`
}

// RunMeta holds run metadata for JSON output.
type RunMeta struct {
	RunID   string    `json:"run_id"`
	Started time.Time `json:"started"`
	Success bool      `json:"success"`
}

// OutputDiffResult is the top-level JSON output structure.
type OutputDiffResult struct {
	Run1      RunMeta    `json:"run1"`
	Run2      RunMeta    `json:"run2"`
	Identical bool       `json:"identical"`
	Hunks     []DiffHunk `json:"hunks"`
}

// findRunByPrefix searches for a run whose RunID starts with the given prefix.
func findRunByPrefix(records []project.RunRecord, prefix string) (*project.RunRecord, error) {
	var matches []project.RunRecord
	for _, r := range records {
		if strings.HasPrefix(r.RunID, prefix) {
			matches = append(matches, r)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("run '%s' not found", prefix)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("run prefix '%s' is ambiguous (%d matches)", prefix, len(matches))
	}
	return &matches[0], nil
}

func taskOutputDiffCmd(args []string) {
	// Parse flags
	var run1Flag, run2Flag string
	contextLines := 3
	ignoreWhitespace := false
	jsonOutput := false
	var positional []string

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--run1" && i+1 < len(args):
			i++
			run1Flag = args[i]
		case args[i] == "--run2" && i+1 < len(args):
			i++
			run2Flag = args[i]
		case args[i] == "--context" && i+1 < len(args):
			i++
			fmt.Sscanf(args[i], "%d", &contextLines)
		case args[i] == "--ignore-whitespace":
			ignoreWhitespace = true
		case args[i] == "--json":
			jsonOutput = true
		case !strings.HasPrefix(args[i], "--"):
			positional = append(positional, args[i])
		}
	}

	if len(positional) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task diff <name> [--run1 ID] [--run2 ID] [--context N] [--ignore-whitespace] [--json]\n")
		os.Exit(1)
	}

	taskName := positional[0]

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		log.Fatalf("failed to load todos: %v", err)
	}

	todo := findTodo(todos, taskName)
	if todo == nil {
		fmt.Fprintf(os.Stderr, "Error: task '%s' not found\n", taskName)
		os.Exit(1)
	}

	records, err := project.ReadAllRunRecords(abs, todo.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read run records: %v\n", err)
		os.Exit(1)
	}

	if len(records) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no runs found for task '%s'\n", taskName)
		os.Exit(1)
	}

	// Select runs to compare
	var rec1, rec2 project.RunRecord

	if run1Flag != "" && run2Flag != "" {
		r1, err := findRunByPrefix(records, run1Flag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		r2, err := findRunByPrefix(records, run2Flag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		rec1 = *r1
		rec2 = *r2
	} else if run1Flag != "" {
		r1, err := findRunByPrefix(records, run1Flag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		rec1 = *r1
		rec2 = records[0] // most recent
	} else if run2Flag != "" {
		if len(records) < 2 {
			fmt.Fprintf(os.Stderr, "Error: need at least 2 runs to diff (found %d)\n", len(records))
			os.Exit(1)
		}
		rec1 = records[1] // second most recent
		r2, err := findRunByPrefix(records, run2Flag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		rec2 = *r2
	} else {
		// Default: compare last two runs
		if len(records) < 2 {
			fmt.Fprintf(os.Stderr, "Error: need at least 2 runs to diff (found %d)\n", len(records))
			os.Exit(1)
		}
		rec1 = records[1] // second most recent (older)
		rec2 = records[0] // most recent (newer)
	}

	// Get output summaries
	out1 := rec1.OutputSummary
	out2 := rec2.OutputSummary

	// Apply whitespace normalization if requested
	if ignoreWhitespace {
		out1 = normalizeWhitespace(out1)
		out2 = normalizeWhitespace(out2)
	}

	// Generate labels
	status1 := "SUCCESS"
	if !rec1.Success {
		status1 = "FAILED"
	}
	status2 := "SUCCESS"
	if !rec2.Success {
		status2 = "FAILED"
	}

	oldLabel := fmt.Sprintf("Run %s (%s) %s", rec1.RunID[:8], rec1.Started.Format("2006-01-02 15:04:05"), status1)
	newLabel := fmt.Sprintf("Run %s (%s) %s", rec2.RunID[:8], rec2.Started.Format("2006-01-02 15:04:05"), status2)

	_ = contextLines // UnifiedDiff uses hardcoded 3-line context

	diff := project.UnifiedDiff(oldLabel, newLabel, out1, out2)

	if jsonOutput {
		printOutputDiffJSON(rec1, rec2, diff)
		return
	}

	if diff == "" {
		fmt.Printf("Outputs are identical between run %s and run %s\n", rec1.RunID[:8], rec2.RunID[:8])
		return
	}

	fmt.Print(diff)
}

func normalizeWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.Join(lines, "\n")
}

func parseDiffToHunks(diff string) []DiffHunk {
	if diff == "" {
		return nil
	}

	var hunks []DiffHunk
	lines := strings.Split(diff, "\n")

	var current *DiffHunk
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			if current != nil {
				hunks = append(hunks, *current)
			}
			h := DiffHunk{}
			fmt.Sscanf(line, "@@ -%d,%d +%d,%d @@", &h.OldStart, &h.OldCount, &h.NewStart, &h.NewCount)
			current = &h
			continue
		}
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			continue
		}
		if current == nil {
			continue
		}
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case ' ':
			current.Lines = append(current.Lines, DiffLine{Type: "context", Content: line[1:]})
		case '-':
			current.Lines = append(current.Lines, DiffLine{Type: "removed", Content: line[1:]})
		case '+':
			current.Lines = append(current.Lines, DiffLine{Type: "added", Content: line[1:]})
		}
	}
	if current != nil {
		hunks = append(hunks, *current)
	}
	return hunks
}

func printOutputDiffJSON(rec1, rec2 project.RunRecord, diff string) {
	result := OutputDiffResult{
		Run1: RunMeta{
			RunID:   rec1.RunID,
			Started: rec1.Started,
			Success: rec1.Success,
		},
		Run2: RunMeta{
			RunID:   rec2.RunID,
			Started: rec2.Started,
			Success: rec2.Success,
		},
		Identical: diff == "",
		Hunks:     parseDiffToHunks(diff),
	}
	if result.Hunks == nil {
		result.Hunks = []DiffHunk{}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(result)
}
