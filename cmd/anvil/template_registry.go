package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
)

// templateInstallCmd implements 'anvil template install <owner/repo>'.
func templateInstallCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: anvil template install <owner/repo> [--force]\n")
		os.Exit(1)
	}

	ref := args[0]
	force := false
	for _, arg := range args[1:] {
		if arg == "--force" {
			force = true
		}
	}

	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		fmt.Fprintf(os.Stderr, "Error: invalid template identifier: %s\n", ref)
		fmt.Fprintf(os.Stderr, "Expected format: owner/repo\n")
		os.Exit(1)
	}
	owner, repo := parts[0], parts[1]

	// Fetch manifest
	manifest, err := project.FetchManifest(owner, repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Check version compatibility
	compatible, msg := project.CheckCompatibility(manifest, version)
	if !compatible {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
		if !force {
			if !promptYN("Install anyway?") {
				fmt.Println("Installation cancelled.")
				os.Exit(1)
			}
		}
	}

	// Determine destination
	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("failed to get absolute path: %v", err)
	}
	destDir := filepath.Join(abs, ".anvil", "templates")

	// Check for existing template
	if !force {
		for _, f := range manifest.Files {
			existing := filepath.Join(destDir, filepath.Base(f))
			if _, err := os.Stat(existing); err == nil {
				fmt.Printf("Template '%s' already exists locally.\n", manifest.Name)
				if !promptYN("Overwrite?") {
					fmt.Println("Installation cancelled.")
					os.Exit(1)
				}
				break
			}
		}
	}

	fmt.Printf("Installing template '%s' from %s/%s (v%s)...\n", manifest.Name, owner, repo, manifest.Version)

	// Download files
	if err := project.DownloadTemplate(owner, repo, manifest, destDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Write registry metadata
	if err := project.WriteRegistryMeta(destDir, manifest.Name, ref, manifest.Version); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write registry metadata: %v\n", err)
	}

	for _, f := range manifest.Files {
		fmt.Printf("  Installed to: .anvil/templates/%s\n", filepath.Base(f))
	}
	fmt.Printf("Successfully installed template '%s'\n", manifest.Name)
}

// templateInfoCmd implements 'anvil template info <owner/repo>'.
func templateInfoCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: anvil template info <owner/repo>\n")
		os.Exit(1)
	}

	ref := args[0]
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		fmt.Fprintf(os.Stderr, "Error: invalid template identifier: %s\n", ref)
		fmt.Fprintf(os.Stderr, "Expected format: owner/repo\n")
		os.Exit(1)
	}
	owner, repo := parts[0], parts[1]

	manifest, err := project.FetchManifest(owner, repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Template: %s\n", manifest.Name)
	fmt.Printf("Author:   %s\n", manifest.Author)
	fmt.Printf("Version:  %s\n", manifest.Version)
	fmt.Printf("Source:    https://github.com/%s/%s\n", owner, repo)
	fmt.Println()
	fmt.Println("Description:")
	fmt.Printf("  %s\n", manifest.Description)
	fmt.Println()
	fmt.Println("Files:")
	for _, f := range manifest.Files {
		fmt.Printf("  - %s\n", f)
	}
	if manifest.MinAnvilVersion != "" {
		fmt.Println()
		fmt.Printf("Requires: anvil v%s+\n", manifest.MinAnvilVersion)
	}
	if len(manifest.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(manifest.Tags, ", "))
	}
}

// templateListInstalledCmd implements 'anvil template ls --installed'.
func templateListInstalledCmd() {
	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("bad path: %v", err)
	}

	metas, err := project.ListInstalledRegistryTemplates(abs)
	if err != nil {
		log.Fatalf("failed to list installed templates: %v", err)
	}

	if len(metas) == 0 {
		fmt.Println("No registry templates installed.")
		fmt.Println("Use 'anvil template search <query>' to find templates.")
		return
	}

	fmt.Println("Installed registry templates:")
	for _, m := range metas {
		// Extract template name from source (owner/repo -> repo)
		name := m.Source
		if parts := strings.SplitN(m.Source, "/", 2); len(parts) == 2 {
			name = parts[1]
		}
		// Parse install date
		installed := m.InstalledAt
		if len(installed) >= 10 {
			installed = installed[:10]
		}
		fmt.Printf("  %-20s %-30s %-10s installed %s\n", name, m.Source, "v"+m.Version, installed)
	}
}

// promptYN asks a yes/no question and returns true for yes.
func promptYN(question string) bool {
	fmt.Printf("%s [y/N]: ", question)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return answer == "y" || answer == "yes"
	}
	return false
}
