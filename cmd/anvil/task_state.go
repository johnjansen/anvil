package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
)

func taskStateCmd(args []string) {
	// Handle subcommands: get, export, import, clear
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task state <name> [subcommand]\n")
		fmt.Fprintf(os.Stderr, "Subcommands:\n")
		fmt.Fprintf(os.Stderr, "  (none)       Show current state\n")
		fmt.Fprintf(os.Stderr, "  --export FILE  Export state to file\n")
		fmt.Fprintf(os.Stderr, "  --import FILE  Import state from file\n")
		fmt.Fprintf(os.Stderr, "  --clear        Clear state\n")
		os.Exit(1)
	}

	// Check for flags first
	var exportFile, importFile string
	var clearState bool
	var taskName string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--export":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --export requires a file argument\n")
				os.Exit(1)
			}
			exportFile = args[i+1]
			i++
		case "--import":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "error: --import requires a file argument\n")
				os.Exit(1)
			}
			importFile = args[i+1]
			i++
		case "--clear":
			clearState = true
		default:
			if taskName == "" {
				taskName = args[i]
			}
		}
	}

	if taskName == "" {
		fmt.Fprintf(os.Stderr, "error: task name required\n")
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

	todo := findTodo(todos, taskName)
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", taskName)
		os.Exit(1)
	}

	// Check if task has state configured
	if todo.State == nil {
		fmt.Fprintf(os.Stderr, "task %s does not have state configured\n", taskName)
		fmt.Fprintf(os.Stderr, "Add 'state' configuration to task frontmatter:\n")
		fmt.Fprintf(os.Stderr, "  state:\n")
		fmt.Fprintf(os.Stderr, "    bucket: \"my-bucket\"\n")
		fmt.Fprintf(os.Stderr, "    key: \"{{ .TaskID }}\"\n")
		os.Exit(1)
	}

	// Resolve the key template
	stateKey := strings.ReplaceAll(todo.State.Key, "{{ .TaskID }}", todo.ID)

	if importFile != "" {
		// Import state from file
		data, err := os.ReadFile(importFile)
		if err != nil {
			log.Fatalf("failed to read import file: %v", err)
		}
		var state map[string]interface{}
		if err := json.Unmarshal(data, &state); err != nil {
			log.Fatalf("failed to parse import file: %v", err)
		}
		if err := project.WriteTaskState(abs, todo.State.Bucket, stateKey, state); err != nil {
			log.Fatalf("failed to write state: %v", err)
		}
		fmt.Printf("State imported from %s\n", importFile)
		return
	}

	if clearState {
		if err := project.DeleteTaskState(abs, todo.State.Bucket, stateKey); err != nil {
			log.Fatalf("failed to clear state: %v", err)
		}
		fmt.Printf("State cleared for %s\n", taskName)
		return
	}

	if exportFile != "" {
		// Export state to file
		state, err := project.ReadTaskState(abs, todo.State.Bucket, stateKey)
		if err != nil {
			log.Fatalf("failed to read state: %v", err)
		}
		if state == nil {
			state = map[string]interface{}{}
		}
		data, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal state: %v", err)
		}
		if err := os.WriteFile(exportFile, data, 0644); err != nil {
			log.Fatalf("failed to write export file: %v", err)
		}
		fmt.Printf("State exported to %s\n", exportFile)
		return
	}

	// Default: show current state
	state, err := project.ReadTaskState(abs, todo.State.Bucket, stateKey)
	if err != nil {
		log.Fatalf("failed to read state: %v", err)
	}
	if state == nil {
		fmt.Printf("No state found for %s (bucket: %s, key: %s)\n", taskName, todo.State.Bucket, stateKey)
		return
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal state: %v", err)
	}
	fmt.Print(string(data))
}

func taskLogCmd(args []string) {
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
		fmt.Fprintf(os.Stderr, "usage: anvil task log [-f] <name|uuid>\n")
		os.Exit(1)
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

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

	// Try to resolve session ID: first from run records, then from the running daemon.
	var sessionID string

	if id != "" {
		sessionID, _ = project.LatestSessionID(abs, id)
	}

	// If we couldn't resolve a session ID from disk, check if the task is
	// currently running — the daemon tracks the session ID in memory.
	if sessionID == "" && todoName != "" && daemon.IsDaemonRunning() {
		if tasks, psErr := daemon.SendPsRequest(); psErr == nil {
			fullKey := fmt.Sprintf("%s/%s", abs, todoName)
			for _, t := range tasks {
				if t.Name == fullKey && t.SessionID != "" {
					sessionID = t.SessionID
					break
				}
			}
		}
	}

	if sessionID == "" {
		if id == "" {
			fmt.Fprintf(os.Stderr, "task has no ID in frontmatter and is not currently running\n")
		} else {
			fmt.Fprintf(os.Stderr, "no session log found for task\n")
		}
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

func taskRmCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task rm <name>\n")
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
		fmt.Fprintf(os.Stderr, "task not found: %s\n", args[0])
		os.Exit(1)
	}

	// Kill if running
	if daemon.IsDaemonRunning() {
		if err := daemon.SendKillRequest(todo.ID); err == nil {
			fmt.Printf("killed running task %s\n", todo.Name)
		}
	}

	if err := project.RemoveTodo(*todo); err != nil {
		log.Fatalf("failed to remove todo: %v", err)
	}

	fmt.Printf("removed p%d/%s\n", todo.Priority, todo.Name)
}
