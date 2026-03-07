package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
)

func addCmd(args []string) {
	// Handle -h/--help before creating task
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			fmt.Fprintf(os.Stderr, `usage: anvil add [-p priority] [-s schedule | --once] [--pre-check cmd] [--allowed-tools tools] [--max-concurrent n] [--skip-permissions] [-f file | -] <task text>

Add a new task to the project.

Options:
  -p, --priority n        Priority 0-9 (default: 1)
  -s, --schedule cron     Cron schedule (e.g., "*/15 * * * *")
  -t, --template name    Use a template for task configuration
  -o, --once              Create a one-shot task (no schedule)
  -n, --dry-run          Show impact analysis (conflicts, load) without creating task
  --json                 Output impact analysis as JSON (with --dry-run)
  --pre-check cmd        Command to run before task execution
  --allowed-tools tools  Comma-separated list of allowed tools (e.g. "Bash,Read" or scoped "Bash(gh:*)", "Read(.claude/commands/*)")
  --max-concurrent n     Max concurrent runs (default: 1)
  --skip-permissions     Skip permission checks
  --strict               Fail if schedule conflicts with existing tasks
  --no-overlap-check     Skip schedule overlap detection
  --depends-on dep       Task dependency (repeatable; use project:task for cross-project)
  -f, --file path        Read task content from a file
  -                      Read task content from stdin

Frontmatter in file/stdin input is merged with CLI flags (CLI flags take precedence).

Examples:
  anvil add "Review pull requests"
  anvil add --once "Migrate the database schema"
  anvil add -p 2 -s "0 9 * * *" "Daily standup notes"
  anvil add --pre-check "git diff --quiet" "Sync documentation"
  anvil add --depends-on setup-db "Run migrations"
  anvil add --depends-on other-project:build-step "Deploy after build"
  anvil add -s "*/30 * * * *" --file triage-prompt.md
  cat prompt.md | anvil add -s "*/30 * * * *" -
  anvil add -s "*/30 * * * *" <<'EOF'
  Check GitHub for new untriaged issues...
  EOF
`)
			os.Exit(0)
		}
	}
	taskCreateCmd(args)
}

func listCmd() {
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

	if len(todos) == 0 {
		fmt.Println("no todos")
		return
	}

	for _, t := range todos {
		preview := strings.TrimSpace(t.Content)
		preview = strings.Join(strings.Fields(preview), " ")
		if len(preview) > 60 {
			preview = preview[:60] + "..."
		}
		fmt.Printf("p%d  %-14s  %-40s  %s\n", t.Priority, t.Schedule, t.Name, preview)
	}
}

func getCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil get <name>\n")
		os.Exit(1)
	}

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

	todo := findTodo(todos, args[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "todo not found: %s\n", args[0])
		os.Exit(1)
	}

	fmt.Printf("File:     p%d/%s\n", todo.Priority, todo.Name)
	fmt.Printf("ID:       %s\n", todo.ID)
	fmt.Printf("Schedule: %s\n", todo.Schedule)
	fmt.Printf("Priority: %d\n", todo.Priority)
	if todo.ID != "" {
		sessionPath := project.SessionPath(abs, todo.ID)
		if _, err := os.Stat(sessionPath); err == nil {
			fmt.Printf("Session:  %s\n", sessionPath)
		}
	}
	fmt.Printf("\n%s", todo.Content)
}

func deleteCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil delete <name>\n")
		os.Exit(1)
	}

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

	todo := findTodo(todos, args[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "todo not found: %s\n", args[0])
		os.Exit(1)
	}

	if err := project.RemoveTodo(*todo); err != nil {
		log.Fatalf("failed to delete todo: %v", err)
	}

	fmt.Printf("deleted p%d/%s\n", todo.Priority, todo.Name)
}

func logCmd(args []string) {
	follow := false
	var rest []string
	for _, a := range args {
		switch a {
		case "-f", "--follow":
			follow = true
		default:
			rest = append(rest, a)
		}
	}

	if len(rest) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil log [-f] <name|uuid>\n")
		os.Exit(1)
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	// Try as todo name first, fall back to treating arg as a UUID directly
	id := rest[0]
	var todoName string
	proj, err := project.Load(abs)
	if err == nil {
		todos, err := proj.LoadTodos()
		if err == nil {
			if todo := findTodo(todos, rest[0]); todo != nil {
				id = todo.ID
				todoName = todo.Name
			}
		}
	}

	if id == "" {
		fmt.Fprintf(os.Stderr, "todo has no session ID\n")
		os.Exit(1)
	}

	// Resolve the session ID from the latest run record (not the task ID)
	sessionID, err := project.LatestSessionID(abs, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "no session log found (looked at %s): %v\n", project.SessionPath(abs, id), err)
		os.Exit(1)
	}

	sessionPath := project.SessionPathBySessionID(abs, sessionID)

	if follow {
		followLog(sessionPath, abs, todoName)
		return
	}

	data, err := os.ReadFile(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "no session log found (looked at %s)\n", sessionPath)
			os.Exit(1)
		}
		log.Fatalf("failed to read session log: %v", err)
	}

	fmt.Print(string(data))
}
