package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
)

// templateSearchCmd implements the 'anvil template search' command
func templateSearchCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: anvil template search <query>\n")
		os.Exit(1)
	}

	query := strings.ToLower(args[0])
	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("failed to get absolute path: %v", err)
	}

	// Load all templates
	templates, err := project.ListTemplates(abs)
	if err != nil {
		log.Fatalf("failed to list templates: %v", err)
	}

	// Search for matching templates
	matches := []project.Template{}
	for _, template := range templates {
		// Load the full template to check description
		fullTemplate, err := project.LoadTemplate(abs, template.Name)
		if err != nil {
			continue // Skip templates that can't be loaded
		}

		// Check if query matches template name or description
		nameMatch := strings.Contains(strings.ToLower(template.Name), query)
		descMatch := strings.Contains(strings.ToLower(fullTemplate.Spec.Description), query)

		if nameMatch || descMatch {
			matches = append(matches, template)
		}
	}

	// Display results
	if len(matches) == 0 {
		fmt.Println("No templates found matching:", query)
		return
	}

	fmt.Printf("Templates matching '%s':\n", query)
	for _, template := range matches {
		// Load full template to get description
		fullTemplate, err := project.LoadTemplate(abs, template.Name)
		if err != nil {
			fmt.Printf("  %s\n", template.Name)
			continue
		}

		fmt.Printf("  %s", template.Name)
		if fullTemplate.Spec.Description != "" {
			// Truncate description for display
			desc := fullTemplate.Spec.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			fmt.Printf(" - %s", desc)
		}
		fmt.Println()
	}
}