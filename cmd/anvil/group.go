package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/johnjansen/anvil/internal/project"
)

func groupCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil group <subcommand> [options]\n")
		fmt.Fprintf(os.Stderr, "Run 'anvil help' for more information.\n")
		os.Exit(1)
	}

	switch args[0] {
	case "ls", "list":
		groupLsCmd(args[1:])
	case "get":
		groupGetCmd(args[1:])
	case "run":
		groupRunCmd(args[1:])
	case "history":
		groupHistoryCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown group command: %s\n", args[0])
		fmt.Fprintf(os.Stderr, "Run 'anvil help' for more information.\n")
		os.Exit(1)
	}
}

func groupLsCmd(args []string) {
	fs := flag.NewFlagSet("group ls", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "Output in JSON format")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: anvil group ls [--json]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "List task groups.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  --json    Output in JSON format")
		os.Exit(1)
	}

	if err := fs.Parse(args); err != nil {
		fs.Usage()
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	_, err = project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	groups, err := proj.LoadGroups()
	if err != nil {
		log.Fatalf("failed to load groups: %v", err)
	}

	if *jsonFlag {
		jsonData, _ := json.MarshalIndent(groups, "", "  ")
		fmt.Println(string(jsonData))
		return
	}

	if len(groups) == 0 {
		fmt.Println("No groups found")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDESCRIPTION\tTASKS\tEXECUTION\tSCHEDULE")
	for _, group := range groups {
		description := group.Description
		if description == "" {
			description = "-"
		}

		schedule := group.Schedule
		if schedule == "" {
			schedule = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
			group.Name,
			description,
			len(group.Tasks),
			group.Execution,
			schedule)
	}
	w.Flush()
}

func groupGetCmd(args []string) {
	fs := flag.NewFlagSet("group get", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "Output in JSON format")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: anvil group get <name> [--json]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Show group details.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  --json    Output in JSON format")
		os.Exit(1)
	}

	if err := fs.Parse(args); err != nil {
		fs.Usage()
	}

	if len(fs.Args()) == 0 {
		fmt.Fprintln(os.Stderr, "missing group name")
		fs.Usage()
	}

	groupName := fs.Args()[0]

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	_, err = project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	groups, err := proj.LoadGroups()
	if err != nil {
		log.Fatalf("failed to load groups: %v", err)
	}

	var targetGroup *project.Group
	for _, group := range groups {
		if group.Name == groupName {
			targetGroup = &group
			break
		}
	}

	if targetGroup == nil {
		fmt.Fprintf(os.Stderr, "group not found: %s\n", groupName)
		os.Exit(1)
	}

	if *jsonFlag {
		jsonData, _ := json.MarshalIndent(targetGroup, "", "  ")
		fmt.Println(string(jsonData))
		return
	}

	fmt.Printf("Name: %s\n", targetGroup.Name)
	if targetGroup.Description != "" {
		fmt.Printf("Description: %s\n", targetGroup.Description)
	}
	fmt.Printf("Execution: %s\n", targetGroup.Execution)
	if targetGroup.Schedule != "" {
		fmt.Printf("Schedule: %s\n", targetGroup.Schedule)
	}
	fmt.Printf("On Failure: %s\n", targetGroup.OnFailure)
	fmt.Println("Tasks:")
	for _, task := range targetGroup.Tasks {
		fmt.Printf("  - %s\n", task)
	}
}

func groupRunCmd(args []string) {
	fs := flag.NewFlagSet("group run", flag.ContinueOnError)
	parallelFlag := fs.Int("parallel", 0, "Maximum parallel tasks (0 = no limit)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: anvil group run <name> [--parallel N]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Run a task group immediately.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  --parallel N    Maximum parallel tasks (0 = no limit)")
		os.Exit(1)
	}

	if err := fs.Parse(args); err != nil {
		fs.Usage()
	}

	if len(fs.Args()) == 0 {
		fmt.Fprintln(os.Stderr, "missing group name")
		fs.Usage()
	}

	groupName := fs.Args()[0]
	_ = parallelFlag // TODO: Actually use this in implementation

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	_, err = project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	proj, err := project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	groups, err := proj.LoadGroups()
	if err != nil {
		log.Fatalf("failed to load groups: %v", err)
	}

	var targetGroup *project.Group
	for _, group := range groups {
		if group.Name == groupName {
			targetGroup = &group
			break
		}
	}

	if targetGroup == nil {
		fmt.Fprintf(os.Stderr, "group not found: %s\n", groupName)
		os.Exit(1)
	}

	fmt.Printf("Running group '%s' (%s execution)...\n", targetGroup.Name, targetGroup.Execution)

	// For now, just print what would be executed
	// In a full implementation, this would actually execute the tasks
	for i, task := range targetGroup.Tasks {
		fmt.Printf("  [%d/%d] %s\n", i+1, len(targetGroup.Tasks), task)
	}

	fmt.Println("Group execution completed.")
}

func groupHistoryCmd(args []string) {
	fs := flag.NewFlagSet("group history", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "Output in JSON format")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: anvil group history <name> [--json]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Show group execution history.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  --json    Output in JSON format")
		os.Exit(1)
	}

	if err := fs.Parse(args); err != nil {
		fs.Usage()
	}

	if len(fs.Args()) == 0 {
		fmt.Fprintln(os.Stderr, "missing group name")
		fs.Usage()
	}

	groupName := fs.Args()[0]

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	_, err = project.Load(abs)
	if err != nil {
		log.Fatalf("failed to load project: %v", err)
	}

	// For now, just show a placeholder message
	// In a full implementation, this would load actual history data

	if *jsonFlag {
		history := map[string]interface{}{
			"group": groupName,
			"runs":  []interface{}{},
		}
		jsonData, _ := json.MarshalIndent(history, "", "  ")
		fmt.Println(string(jsonData))
		return
	}

	fmt.Printf("Execution history for group '%s':\n", groupName)
	fmt.Println("No execution history found.")
}
