package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
)

func templateCmd(args []string) {
	if len(args) == 0 {
		templateUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "ls":
		templateListCmd(args[1:])
	case "get":
		templateGetCmd(args[1:])
	case "search":
		templateSearchCmd(args[1:])
	case "import":
		templateImportCmd(args[1:])
	case "export":
		templateExportCmd(args[1:])
	case "install":
		templateInstallCmd(args[1:])
	case "info":
		templateInfoCmd(args[1:])
	case "-h", "--help":
		templateUsage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown template subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func templateUsage() {
	fmt.Fprintf(os.Stderr, "usage: anvil template <subcommand> [options]\n")
	fmt.Fprintf(os.Stderr, "Subcommands:\n")
	fmt.Fprintf(os.Stderr, "  ls              List available templates\n")
	fmt.Fprintf(os.Stderr, "  get <name>      Show template details\n")
	fmt.Fprintf(os.Stderr, "  search <query>  Search registry templates by keyword\n")
	fmt.Fprintf(os.Stderr, "  import <source> Import template from URL, file, or gist\n")
	fmt.Fprintf(os.Stderr, "  export <name>   Export template to shareable file\n")
	fmt.Fprintf(os.Stderr, "  install <owner/repo>  Install template from registry\n")
	fmt.Fprintf(os.Stderr, "  info <owner/repo>     Show registry template details\n")
	fmt.Fprintf(os.Stderr, "Run 'anvil help' for more information.\n")
}

func templateListCmd(args []string) {
	// Check for --installed flag
	for _, arg := range args {
		if arg == "--installed" {
			templateListInstalledCmd()
			return
		}
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	templates, err := project.ListTemplates(abs)
	if err != nil {
		log.Fatalf("failed to list templates: %v", err)
	}

	if len(templates) == 0 {
		fmt.Println("No templates found")
		fmt.Println("\nCreate templates in:")
		fmt.Println("  .anvil/templates/   (project-specific)")
		fmt.Println("  ~/.anvil/templates/ (global)")
		return
	}

	fmt.Println("Available templates:")
	for _, t := range templates {
		fmt.Printf("  %s\n", t.Name)
	}
}

func templateGetCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil template get <name>\n")
		os.Exit(1)
	}

	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	tmpl, err := project.LoadTemplate(abs, args[0])
	if err != nil {
		log.Fatalf("template not found: %s\n", args[0])
	}

	fmt.Printf("Template: %s\n", tmpl.Name)
	// Display key template fields
	spec := tmpl.Spec
	if spec.Schedule != "" {
		fmt.Printf("Schedule: %s\n", spec.Schedule)
	}
	if spec.Priority > 0 {
		fmt.Printf("Priority: %d\n", spec.Priority)
	}
	if len(spec.AllowedTools) > 0 {
		fmt.Printf("AllowedTools: %s\n", strings.Join(spec.AllowedTools, ", "))
	}
}
