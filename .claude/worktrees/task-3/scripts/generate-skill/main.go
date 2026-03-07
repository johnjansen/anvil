package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	// Read the current main.go file to extract help information
	helpContent, err := extractHelpFromSource()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting help: %v\n", err)
		os.Exit(1)
	}

	// Generate the skill file content
	skillContent := generateSkillFile(helpContent)

	// Write the skill file
	err = writeSkillFile(skillContent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing skill file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully generated skill file at ./tools/skills/anvil/SKILL.md")
}

func extractHelpFromSource() (string, error) {
	// For now, we'll read the printUsage function from main.go
	// In a real implementation, we might want to actually run the CLI with --help
	content, err := os.ReadFile("./cmd/anvil/main.go")
	if err != nil {
		return "", err
	}

	// Find the printUsage function
	lines := strings.Split(string(content), "\n")
	startLine := -1
	endLine := -1

	for i, line := range lines {
		if strings.Contains(line, "func printUsage() {") {
			startLine = i
		}
		if startLine != -1 && strings.Contains(line, "}") && i > startLine {
			// Check if this is the closing brace of printUsage
			indent := strings.Index(line, "}")
			if indent == 0 { // Top-level closing brace
				endLine = i
				break
			}
		}
	}

	if startLine == -1 || endLine == -1 {
		return "", fmt.Errorf("could not find printUsage function")
	}

	return strings.Join(lines[startLine:endLine+1], "\n"), nil
}

func generateSkillFile(helpContent string) string {
	var builder strings.Builder

	// YAML frontmatter
	builder.WriteString("---\n")
	builder.WriteString("name: anvil\n")
	builder.WriteString("description: Manage recurring tasks and todos with the anvil CLI. Use when users ask to add tasks, list todos, delete tasks, check task logs, kill running tasks, initialize a project for anvil, or watch a project. Trigger phrases include \"add a task\", \"add a todo\", \"list todos\", \"show my tasks\", \"delete a task\", \"remove a todo\", \"create a recurring job\", \"check task log\", \"kill task\", \"stop task\", \"initialize anvil\", \"watch this project\", \"anvil init\", \"anvil add\", \"anvil task\", \"anvil project\", \"anvil kill\", \"anvil watch\", \"anvil status\", \"what tasks are running\", \"show running tasks\".\n")
	builder.WriteString("---\n\n")

	// Title
	builder.WriteString("# Anvil CLI\n\n")
	builder.WriteString("Anvil is a task dispatcher for LLM projects. It manages recurring and one-shot todos that the daemon executes on a cron schedule. Use the CLI to manage tasks — the daemon handles execution.\n\n")

	// Parse the help content to extract sections
	scanner := bufio.NewScanner(strings.NewReader(helpContent))

	// Skip the function declaration and opening brace
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "fmt.Fprintf(os.Stderr, `") {
			break
		}
	}

	// Process the help content
	inSection := false
	currentSection := ""
	var sectionLines []string
	finished := false

	for scanner.Scan() && !finished {
		line := scanner.Text()

		// Check for the end of the help string
		if strings.Contains(line, "`") && strings.Contains(line, "); err != nil {") {
			finished = true
			break
		}

		// Remove leading/trailing quotes and formatting
		line = strings.Trim(line, "`")
		line = strings.Trim(line, "\"")
		line = strings.TrimSpace(line)

		// Skip empty lines at the start
		if !inSection && line == "" {
			continue
		}

		inSection = true

		// Check if this is a section header (ends with colon and no spaces)
		if strings.HasSuffix(line, ":") && !strings.Contains(line, " ") && len(line) > 1 {
			// We found a new section
			if currentSection != "" && len(sectionLines) > 0 {
				// Write the previous section
				builder.WriteString(fmt.Sprintf("## %s\n\n", strings.TrimSuffix(currentSection, ":")))
				for _, sectionLine := range sectionLines {
					if sectionLine != "" {
						builder.WriteString(fmt.Sprintf("%s\n", sectionLine))
					}
				}
				builder.WriteString("\n")
			}

			currentSection = line
			sectionLines = []string{}
			continue
		}

		// Special handling for Configuration section
		if currentSection == "Configuration:" && strings.Contains(line, "~/.anvil/config.yaml") {
			// Write the Configuration section and finish
			builder.WriteString(fmt.Sprintf("## %s\n\n", strings.TrimSuffix(currentSection, ":")))
			builder.WriteString("~/.anvil/config.yaml   Daemon config\n")
			builder.WriteString("<project>/.anvil/      Project config and todos\n\n")
			break
		}

		// Add line to current section if it's not empty
		if currentSection != "" && line != "" {
			sectionLines = append(sectionLines, line)
		} else if currentSection == "" && strings.HasPrefix(line, "anvil - ") {
			// Handle the introductory line
			builder.WriteString(fmt.Sprintf("%s\n\n", line))
		}
	}

	// Write the last section if we haven't finished
	if !finished && currentSection != "" && len(sectionLines) > 0 {
		builder.WriteString(fmt.Sprintf("## %s\n\n", strings.TrimSuffix(currentSection, ":")))
		for _, sectionLine := range sectionLines {
			if sectionLine != "" {
				builder.WriteString(fmt.Sprintf("%s\n", sectionLine))
			}
		}
		builder.WriteString("\n")
	}

	// Add specific documentation for features that might be missing
	builder.WriteString(addMissingDocumentation())

	return builder.String()
}

func addMissingDocumentation() string {
	var builder strings.Builder

	// Add documentation for scoped allowed_tools syntax
	builder.WriteString("## Advanced allowed_tools Syntax\n\n")
	builder.WriteString("The `allowed_tools` feature supports scoped permissions for fine-grained control:\n\n")
	builder.WriteString("```yaml\n")
	builder.WriteString("---\n")
	builder.WriteString("id: \"some-uuid\"\n")
	builder.WriteString("schedule: \"*/5 * * * *\"\n")
	builder.WriteString("allowed_tools:\n")
	builder.WriteString("  - Bash(gh:*)      # only gh subcommands\n")
	builder.WriteString("  - Read(.claude/commands/*)  # only read files in .claude/commands/\n")
	builder.WriteString("  - Write(/tmp/*)   # only write files in /tmp/\n")
	builder.WriteString("---\n")
	builder.WriteString("```\n\n")
	builder.WriteString("Scoped syntax allows you to restrict tools to specific command prefixes or file paths, providing least-privilege access control.\n\n")

	// Add documentation for YAML list format in frontmatter
	builder.WriteString("## Frontmatter Configuration\n\n")
	builder.WriteString("Tasks can be configured with various options in YAML frontmatter:\n\n")
	builder.WriteString("```yaml\n")
	builder.WriteString("---\n")
	builder.WriteString("id: \"some-uuid\"\n")
	builder.WriteString("schedule: \"*/30 * * * *\"\n")
	builder.WriteString("priority: 1\n")
	builder.WriteString("pre_check: \"gh issue list --state open --label untriaged | grep -q .\"\n")
	builder.WriteString("allowed_tools:\n")
	builder.WriteString("  - Bash\n")
	builder.WriteString("  - Read\n")
	builder.WriteString("  - Write\n")
	builder.WriteString("max_concurrent: 2\n")
	builder.WriteString("skip_permissions: false\n")
	builder.WriteString("disabled: false\n")
	builder.WriteString("timeout: 15m\n")
	builder.WriteString("retry: 3\n")
	builder.WriteString("retry_delay: 2m\n")
	builder.WriteString("persistent_cooldown: 5s\n")
	builder.WriteString("persistent_max_runtime: 30m\n")
	builder.WriteString("on_success: \"echo 'done' >> /tmp/anvil.log\"\n")
	builder.WriteString("on_failure: \"curl -X POST https://slack.example.com/webhook -d '{\\\"text\\\":\\\"Task failed\\\"}'\"\n")
	builder.WriteString("---\n")
	builder.WriteString("```\n\n")

	return builder.String()
}

func writeSkillFile(content string) error {
	file, err := os.Create("./tools/skills/anvil/SKILL.md")
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}