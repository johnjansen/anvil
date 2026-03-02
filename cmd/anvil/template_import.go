package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/johnjansen/anvil/internal/project"
	"gopkg.in/yaml.v3"
)

// templateImportCmd implements the 'anvil template import' command
func templateImportCmd(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: anvil template import <source>\n")
		fmt.Fprintf(os.Stderr, "  source can be a URL, local file path, or gist:ID\n")
		os.Exit(1)
	}

	source := args[0]
	abs, err := filepath.Abs(".")
	if err != nil {
		log.Fatalf("failed to get absolute path: %v", err)
	}

	var data []byte

	// Handle different source types
	switch {
	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		// Import from URL
		data, err = importFromURL(source)
		if err != nil {
			log.Fatalf("failed to import from URL: %v", err)
		}
	case strings.HasPrefix(source, "gist:"):
		// Import from GitHub gist
		gistID := strings.TrimPrefix(source, "gist:")
		data, err = importFromGist(gistID)
		if err != nil {
			log.Fatalf("failed to import from gist: %v", err)
		}
	default:
		// Import from local file
		data, err = os.ReadFile(source)
		if err != nil {
			log.Fatalf("failed to read file: %v", err)
		}
	}

	// Parse the template to validate it
	var templateSpec project.TemplateSpec
	if err := yaml.Unmarshal(data, &templateSpec); err != nil {
		log.Fatalf("failed to parse template: %v", err)
	}

	// Derive template name from source
	templateName := "imported-template"
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		// Extract filename from URL
		parts := strings.Split(source, "/")
		if len(parts) > 0 {
			templateName = strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(parts[len(parts)-1]))
		}
	} else if strings.HasPrefix(source, "gist:") {
		templateName = "imported-template"
	} else {
		// Local file
		templateName = strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	}

	// Save the template to project templates directory
	projectTemplateDir := filepath.Join(abs, ".anvil", "templates")
	if err := os.MkdirAll(projectTemplateDir, 0755); err != nil {
		log.Fatalf("failed to create templates directory: %v", err)
	}

	templatePath := filepath.Join(projectTemplateDir, templateName+".yaml")
	if err := os.WriteFile(templatePath, data, 0644); err != nil {
		log.Fatalf("failed to save template: %v", err)
	}

	fmt.Printf("Successfully imported template '%s' from %s\n", templateName, source)
}

// importFromURL downloads template content from a URL
func importFromURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download from URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

// importFromGist downloads template content from a GitHub gist
func importFromGist(gistID string) ([]byte, error) {
	// For simplicity, we'll construct the raw URL
	// In a real implementation, we might want to use the GitHub API to get the actual file list
	url := fmt.Sprintf("https://gist.githubusercontent.com/raw/%s", gistID)
	return importFromURL(url)
}