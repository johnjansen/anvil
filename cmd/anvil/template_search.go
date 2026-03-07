package main

import (
	"fmt"
	"os"

	"github.com/johnjansen/anvil/internal/project"
)

// templateSearchCmd implements the 'anvil template search' command.
// Queries the GitHub-based template registry for matching templates.
func templateSearchCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: anvil template search <query>\n")
		os.Exit(1)
	}

	query := args[0]

	results, err := project.SearchRegistry(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: unable to reach template registry: %v\n", err)
		fmt.Fprintf(os.Stderr, "Hint: check your network connection or try again later\n")
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Printf("No registry templates found matching: %s\n", query)
		return
	}

	fmt.Printf("Registry templates matching '%s':\n", query)
	for _, r := range results {
		desc := r.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		name := fmt.Sprintf("%s/%s", r.Owner, r.Repo)
		fmt.Printf("  %-40s - %-50s (%d stars)\n", name, desc, r.Stars)
	}
}
