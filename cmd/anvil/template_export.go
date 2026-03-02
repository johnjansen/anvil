package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
	"gopkg.in/yaml.v3"
)

// templateExportCmd implements the 'anvil template export' command
func templateExportCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: anvil template export <name> [flags]\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fmt.Fprintf(os.Stderr, "  -o, --output string    Output file path\n")
		fmt.Fprintf(os.Stderr, "      --include-description   Include description in exported template\n")
		os.Exit(1)
	}

	templateName := args[0]
	outputPath := ""
	includeDescription := false

	// Parse flags
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-o", "--output":
			if i+1 >= len(args) {
				log.Fatal("missing value for -o/--output")
			}
			outputPath = args[i+1]
			i++
		case "--include-description":
			includeDescription = true
		default:
			if strings.HasPrefix(args[i], "-") {
				log.Fatalf("unknown flag: %s", args[i])
			}
		}
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("failed to get absolute path: %v", err)
	}

	// Load the template
	template, err := project.LoadTemplate(abs, templateName)
	if err != nil {
		log.Fatalf("failed to load template: %v", err)
	}

	var templateData []byte
	if includeDescription {
		// Include all fields including description
		templateData, err = yaml.Marshal(template.Spec)
		if err != nil {
			log.Fatalf("failed to marshal template: %v", err)
		}
	} else {
		// Exclude description for basic export
		specWithoutDescription := template.Spec
		specWithoutDescription.Description = ""
		templateData, err = yaml.Marshal(specWithoutDescription)
		if err != nil {
			log.Fatalf("failed to marshal template: %v", err)
		}
	}

	// Determine output path
	if outputPath == "" {
		outputPath = templateName + ".yaml"
	}

	// Write to file
	if err := os.WriteFile(outputPath, templateData, 0644); err != nil {
		log.Fatalf("failed to write exported template: %v", err)
	}

	fmt.Printf("Successfully exported template '%s' to %s\n", templateName, outputPath)
}