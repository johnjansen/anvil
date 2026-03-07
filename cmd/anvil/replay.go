package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
)

func taskReplayCmd(args []string) {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	help := fs.Bool("help", false, "Show help")
	lastSuccess := fs.Bool("last-success", false, "Replay the last successful run")
	rerun := fs.Bool("rerun", false, "Rerun the task with saved inputs")
	diff := fs.Bool("diff", false, "Show diff between two runs")
	pin := fs.Bool("pin", false, "Pin task to last successful run")
	unpin := fs.Bool("unpin", false, "Remove pinned run from task")
	var run1, run2 string
	fs.StringVar(&run1, "run1", "", "First run ID for diff")
	fs.StringVar(&run2, "run2", "", "Second run ID for diff")
	jsonOut := fs.Bool("json", false, "Output in JSON format")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if *help {
		printReplayHelp()
		return
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		fmt.Fprintf(os.Stderr, "Error: task name is required\n")
		printReplayHelp()
		os.Exit(1)
	}
	taskName := remaining[0]

	abs, err := filepath.Abs(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	proj, err := project.Load(abs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading project: %v\n", err)
		os.Exit(1)
	}

	todos, err := proj.LoadTodos()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading tasks: %v\n", err)
		os.Exit(1)
	}

	task := findTodo(todos, taskName)
	if task == nil {
		fmt.Fprintf(os.Stderr, "Error: task '%s' not found\n", taskName)
		os.Exit(1)
	}

	switch {
	case *diff:
		replayDiff(proj, task, run1, run2, *jsonOut)
	case *pin:
		replayPin(proj, task)
	case *unpin:
		replayUnpin(task)
	case *lastSuccess:
		replayLastSuccess(proj, task, *jsonOut)
	case *rerun:
		replayRerun(proj, task)
	case len(remaining) > 1:
		replaySpecificRun(proj, task, remaining[1], *jsonOut)
	default:
		fmt.Fprintf(os.Stderr, "Error: must specify a replay mode (--last-success, --rerun, --diff, --pin, --unpin, or a run ID)\n")
		printReplayHelp()
		os.Exit(1)
	}
}

func printReplayHelp() {
	fmt.Print(`Usage: anvil task replay [options] <task-name> [run-id]

Replay a previous task execution or manage replay settings.

Modes:
  anvil task replay <task> <run-id>           Show output from a specific run
  anvil task replay --last-success <task>      Show output from last successful run
  anvil task replay --rerun <task>             Show saved prompt for re-execution
  anvil task replay --diff <task>              Diff last two runs
  anvil task replay --diff --run1 X --run2 Y <task>  Diff specific runs
  anvil task replay --pin <task>               Pin task to last successful run
  anvil task replay --unpin <task>             Remove pinned run

Options:
  --last-success    Replay the last successful run
  --rerun           Show saved prompt from last successful run
  --diff            Show diff between runs
  --pin             Pin task to deterministic output from last successful run
  --unpin           Remove pinned run from task
  --run1 id         First run ID for diff
  --run2 id         Second run ID for diff
  --json            Output in JSON format
  --help            Show this help
`)
}

// replaySpecificRun displays the output from a specific historical run.
func replaySpecificRun(proj *project.Project, task *project.Todo, runID string, jsonOut bool) {
	records, err := project.ReadAllRunRecords(proj.Path, task.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading run records: %v\n", err)
		os.Exit(1)
	}

	rec := findRunRecord(records, runID)
	if rec == nil {
		fmt.Fprintf(os.Stderr, "Error: run '%s' not found for task '%s'\n", runID, task.Name)
		os.Exit(1)
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(rec)
		return
	}

	printRunSummary(rec)
}

// replayLastSuccess finds and displays the most recent successful run.
func replayLastSuccess(proj *project.Project, task *project.Todo, jsonOut bool) {
	rec, err := findLastSuccessfulRun(proj.Path, task.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(rec)
		return
	}

	fmt.Printf("Last successful run: %s\n", rec.RunID[:min(8, len(rec.RunID))])
	printRunSummary(rec)
}

// replayRerun shows the saved prompt from the last successful run for re-execution.
func replayRerun(proj *project.Project, task *project.Todo) {
	rec, err := findLastSuccessfulRun(proj.Path, task.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	promptData, err := project.ReadSnapshotFile(proj.Path, task.ID, rec.RunID, "prompt.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: no snapshot found for run %s (snapshots require replay: true)\n", rec.RunID[:min(8, len(rec.RunID))])
		os.Exit(1)
	}

	fmt.Printf("Prompt from run %s:\n\n%s\n", rec.RunID[:min(8, len(rec.RunID))], string(promptData))
}

// replayDiff shows differences between two runs' output summaries.
func replayDiff(proj *project.Project, task *project.Todo, run1, run2 string, jsonOut bool) {
	records, err := project.ReadAllRunRecords(proj.Path, task.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading run records: %v\n", err)
		os.Exit(1)
	}

	if len(records) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no runs found for task '%s'\n", task.Name)
		os.Exit(1)
	}

	var rec1, rec2 *project.RunRecord

	if run1 != "" && run2 != "" {
		rec1 = findRunRecord(records, run1)
		rec2 = findRunRecord(records, run2)
		if rec1 == nil {
			fmt.Fprintf(os.Stderr, "Error: run '%s' not found\n", run1)
			os.Exit(1)
		}
		if rec2 == nil {
			fmt.Fprintf(os.Stderr, "Error: run '%s' not found\n", run2)
			os.Exit(1)
		}
	} else {
		if len(records) < 2 {
			fmt.Fprintf(os.Stderr, "Error: need at least 2 runs to diff (found %d)\n", len(records))
			os.Exit(1)
		}
		rec1 = &records[1] // second most recent (older)
		rec2 = &records[0] // most recent (newer)
	}

	oldLabel := fmt.Sprintf("Run %s (%s)", rec1.RunID[:min(8, len(rec1.RunID))], rec1.Started.Format("2006-01-02 15:04:05"))
	newLabel := fmt.Sprintf("Run %s (%s)", rec2.RunID[:min(8, len(rec2.RunID))], rec2.Started.Format("2006-01-02 15:04:05"))

	diff := project.UnifiedDiff(oldLabel, newLabel, rec1.OutputSummary, rec2.OutputSummary)

	if jsonOut {
		result := map[string]interface{}{
			"run1":      rec1.RunID,
			"run2":      rec2.RunID,
			"identical": diff == "",
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
		return
	}

	if diff == "" {
		fmt.Printf("Outputs are identical between run %s and run %s\n",
			rec1.RunID[:min(8, len(rec1.RunID))], rec2.RunID[:min(8, len(rec2.RunID))])
		return
	}

	fmt.Print(diff)
}

// replayPin pins a task to its last successful run by updating frontmatter.
func replayPin(proj *project.Project, task *project.Todo) {
	rec, err := findLastSuccessfulRun(proj.Path, task.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := updateFrontmatterField(task.Path, "pinned_run", rec.RunID); err != nil {
		fmt.Fprintf(os.Stderr, "Error updating task file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Pinned task '%s' to run %s\n", task.Name, rec.RunID[:min(8, len(rec.RunID))])
}

// replayUnpin removes the pinned_run field from a task's frontmatter.
func replayUnpin(task *project.Todo) {
	if err := removeFrontmatterField(task.Path, "pinned_run"); err != nil {
		fmt.Fprintf(os.Stderr, "Error updating task file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Unpinned task '%s'\n", task.Name)
}

// findLastSuccessfulRun returns the most recent successful run record.
func findLastSuccessfulRun(projectPath, taskID string) (*project.RunRecord, error) {
	records, err := project.ReadAllRunRecords(projectPath, taskID)
	if err != nil {
		return nil, fmt.Errorf("reading run records: %w", err)
	}

	// Records are sorted newest-first
	for i := range records {
		if records[i].Success {
			return &records[i], nil
		}
	}

	return nil, fmt.Errorf("no successful runs found")
}

// findRunRecord finds a run by exact ID or prefix match.
func findRunRecord(records []project.RunRecord, idOrPrefix string) *project.RunRecord {
	var matches []*project.RunRecord
	for i := range records {
		if records[i].RunID == idOrPrefix {
			return &records[i]
		}
		if strings.HasPrefix(records[i].RunID, idOrPrefix) {
			matches = append(matches, &records[i])
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return nil
}

func printRunSummary(rec *project.RunRecord) {
	status := "SUCCESS"
	if !rec.Success {
		status = "FAILED"
	}
	fmt.Printf("Run:     %s\n", rec.RunID)
	fmt.Printf("Status:  %s\n", status)
	fmt.Printf("Started: %s\n", rec.Started.Format("2006-01-02 15:04:05"))
	fmt.Printf("Ended:   %s\n", rec.Finished.Format("2006-01-02 15:04:05"))
	if rec.OutputSummary != "" {
		fmt.Printf("\nOutput:\n%s\n", rec.OutputSummary)
	} else {
		fmt.Println("\nNo output summary available")
	}
}

// updateFrontmatterField adds or updates a field in a task file's YAML frontmatter.
func updateFrontmatterField(taskPath, field, value string) error {
	data, err := os.ReadFile(taskPath)
	if err != nil {
		return fmt.Errorf("reading task file: %w", err)
	}

	content := string(data)
	fieldLine := fmt.Sprintf("%s: %s", field, value)

	if strings.HasPrefix(content, "---\n") {
		parts := strings.SplitN(content[4:], "\n---\n", 2)
		if len(parts) == 2 {
			fm := parts[0]
			body := parts[1]

			if strings.Contains(fm, field+":") {
				lines := strings.Split(fm, "\n")
				for i, line := range lines {
					if strings.HasPrefix(strings.TrimSpace(line), field+":") {
						lines[i] = fieldLine
						break
					}
				}
				fm = strings.Join(lines, "\n")
			} else {
				fm = fm + "\n" + fieldLine
			}
			content = "---\n" + fm + "\n---\n" + body
		} else {
			content = "---\n" + fieldLine + "\n---\n" + content
		}
	} else {
		content = "---\n" + fieldLine + "\n---\n" + content
	}

	return os.WriteFile(taskPath, []byte(content), 0644)
}

// removeFrontmatterField removes a field from a task file's YAML frontmatter.
func removeFrontmatterField(taskPath, field string) error {
	data, err := os.ReadFile(taskPath)
	if err != nil {
		return fmt.Errorf("reading task file: %w", err)
	}

	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		return nil // no frontmatter
	}

	parts := strings.SplitN(content[4:], "\n---\n", 2)
	if len(parts) != 2 {
		return nil
	}

	fm := parts[0]
	body := parts[1]

	lines := strings.Split(fm, "\n")
	var newLines []string
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), field+":") {
			newLines = append(newLines, line)
		}
	}

	if len(newLines) == 0 {
		content = body
	} else {
		content = "---\n" + strings.Join(newLines, "\n") + "\n---\n" + body
	}

	return os.WriteFile(taskPath, []byte(content), 0644)
}
