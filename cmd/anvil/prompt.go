package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/project"
)

// promptCmd handles the prompt subcommands: test, analyze, preview
func promptCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil prompt <subcommand> [options]\n")
		fmt.Fprintf(os.Stderr, "Subcommands:\n")
		fmt.Fprintf(os.Stderr, "  test <task>      Test prompt output parsing\n")
		fmt.Fprintf(os.Stderr, "  analyze <task>   Show token count and cost estimates\n")
		fmt.Fprintf(os.Stderr, "  preview <task>   Show expanded prompt with variables\n")
		fmt.Fprintf(os.Stderr, "Run 'anvil help' for more information.\n")
		os.Exit(1)
	}

	switch args[0] {
	case "test":
		promptTestCmd(args[1:])
	case "analyze":
		promptAnalyzeCmd(args[1:])
	case "preview":
		promptPreviewCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown prompt command: %s\n", args[0])
		os.Exit(1)
	}
}

// promptTestCmd runs the task multiple times and reports parse success rate
func promptTestCmd(args []string) {
	format := ""
	iterations := 3
	jsonOutput := false
	var rest []string

	for _, a := range args {
		switch a {
		case "--format":
			// next arg is format
		case "--format=json":
			format = "json"
		case "--format=yaml":
			format = "yaml"
		case "--iterations", "-n":
			// handled below
		case "--json":
			jsonOutput = true
		default:
			if strings.HasPrefix(a, "--iterations=") {
				n, err := strconv.Atoi(strings.TrimPrefix(a, "--iterations="))
				if err == nil && n > 0 {
					iterations = n
				}
			} else if strings.HasPrefix(a, "-n=") {
				n, err := strconv.Atoi(strings.TrimPrefix(a, "-n="))
				if err == nil && n > 0 {
					iterations = n
				}
			} else if a != "-n" && !strings.HasPrefix(a, "--iterations") {
				rest = append(rest, a)
			}
		}
	}

	// Handle --format and -n with separate args
	for i, a := range args {
		if (a == "--format" || a == "-f") && i+1 < len(args) {
			format = args[i+1]
		}
		if (a == "--iterations" || a == "-n") && i+1 < len(args) {
			if n, err := strconv.Atoi(args[i+1]); err == nil && n > 0 {
				iterations = n
			}
		}
	}

	if len(rest) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil prompt test <task> [--format json|yaml] [--iterations N] [--json]\n")
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

	todo := findTodo(todos, rest[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", rest[0])
		os.Exit(1)
	}

	// Load runner config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Run the task multiple times
	fmt.Printf("Running prompt test for '%s' (%d iterations)...\n\n", todo.Name, iterations)

	var successes, failures int
	var outputs []string
	var errors []string

	runnerCmd := getRunnerCommand(todo, cfg)

	for i := 0; i < iterations; i++ {
		fmt.Printf("[%d/%d] Running...\n", i+1, iterations)

		// Run the task
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, "sh", "-c", runnerCmd+" '"+escapeForShell(todo.Content)+"'")
		cmd.Dir = abs

		output, _ := cmd.CombinedOutput()
		outputStr := string(output)
		outputs = append(outputs, outputStr)

		// Try to parse
		if format == "json" {
			var js json.RawMessage
			if json.Unmarshal([]byte(outputStr), &js) == nil {
				successes++
			} else {
				failures++
				errors = append(errors, fmt.Sprintf("iter %d: invalid JSON", i+1))
			}
		} else if format == "yaml" {
			// Simple YAML validation (check for key: value patterns)
			if regexp.MustCompile(`^[\w-]+:\s*`).MatchString(outputStr) {
				successes++
			} else {
				failures++
				errors = append(errors, fmt.Sprintf("iter %d: invalid YAML", i+1))
			}
		} else {
			// No format specified, just check if there's output
			if len(outputStr) > 0 {
				successes++
			} else {
				failures++
				errors = append(errors, fmt.Sprintf("iter %d: no output", i+1))
			}
		}
	}

	// Print results
	successRate := float64(successes) / float64(iterations) * 100

	if jsonOutput {
		type testResult struct {
			Task         string   `json:"task"`
			Iterations   int      `json:"iterations"`
			Successes    int      `json:"successes"`
			Failures     int      `json:"failures"`
			SuccessRate  float64  `json:"success_rate"`
			Format       string   `json:"format"`
			Errors       []string `json:"errors,omitempty"`
		}
		result := testResult{
			Task:        todo.Name,
			Iterations:  iterations,
			Successes:   successes,
			Failures:    failures,
			SuccessRate: successRate,
			Format:      format,
			Errors:      errors,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("Results:\n")
	fmt.Printf("  Successes:  %d\n", successes)
	fmt.Printf("  Failures:  %d\n", failures)
	fmt.Printf("  Success Rate: %.1f%%\n", successRate)

	if len(errors) > 0 && len(errors) <= 5 {
		fmt.Printf("\nErrors:\n")
		for _, e := range errors {
			fmt.Printf("  - %s\n", e)
		}
	}

	if len(outputs) > 0 {
		fmt.Printf("\nSample output (last run):\n")
		sample := outputs[len(outputs)-1]
		lines := strings.Split(sample, "\n")
		for i, line := range lines {
			if i >= 10 {
				fmt.Printf("  ... (%d more lines)\n", len(lines)-10)
				break
			}
			fmt.Printf("  %s\n", line)
		}
	}
}

// promptAnalyzeCmd shows token count and cost estimates
func promptAnalyzeCmd(args []string) {
	jsonOutput := false
	var rest []string
	for _, a := range args {
		if a == "--json" {
			jsonOutput = true
		} else {
			rest = append(rest, a)
		}
	}

	if len(rest) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil prompt analyze <task> [--json]\n")
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

	todo := findTodo(todos, rest[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", rest[0])
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Estimate tokens
	content := strings.TrimSpace(todo.Content)
	charCount := len(content)
	// Rough estimate: ~4 characters per token
	inputTokens := charCount / 4
	// Estimate output tokens (typically 20-50% of input for prompts)
	outputTokens := inputTokens / 4

	// Context limit warning (assume 200k for Claude)
	contextLimit := 200000
	warningThreshold := contextLimit * 80 / 100

	// Calculate costs
	inputRate := cfg.InputTokenRate
	if inputRate <= 0 {
		inputRate = 3.0 // default Sonnet rate
	}
	outputRate := cfg.OutputTokenRate
	if outputRate <= 0 {
		outputRate = 15.0 // default Sonnet rate
	}

	inputCost := float64(inputTokens) / 1_000_000 * inputRate
	outputCost := float64(outputTokens) / 1_000_000 * outputRate
	totalCost := inputCost + outputCost

	if jsonOutput {
		type analyzeResult struct {
			Task            string  `json:"task"`
			CharCount       int     `json:"char_count"`
			InputTokens     int     `json:"input_tokens"`
			OutputTokens    int     `json:"output_tokens"`
			InputCost       float64 `json:"input_cost"`
			OutputCost      float64 `json:"output_cost"`
			TotalCost       float64 `json:"total_cost"`
			ContextLimit    int     `json:"context_limit"`
			ContextPercent  float64 `json:"context_percent"`
			Warning         string  `json:"warning,omitempty"`
		}
		result := analyzeResult{
			Task:           todo.Name,
			CharCount:      charCount,
			InputTokens:    inputTokens,
			OutputTokens:   outputTokens,
			InputCost:      inputCost,
			OutputCost:     outputCost,
			TotalCost:      totalCost,
			ContextLimit:   contextLimit,
			ContextPercent: float64(inputTokens) / float64(contextLimit) * 100,
		}
		if inputTokens > warningThreshold {
			result.Warning = fmt.Sprintf("exceeds 80%% of context limit (%d tokens)", warningThreshold)
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("Prompt Analysis: %s\n\n", todo.Name)
	fmt.Printf("Content:\n")
	fmt.Printf("  Characters:  %d\n", charCount)
	fmt.Printf("  Lines:       %d\n", len(strings.Split(content, "\n")))
	fmt.Printf("  Input Tokens (est): %d\n", inputTokens)
	fmt.Printf("  Output Tokens (est): %d\n", outputTokens)

	fmt.Printf("\nCost Estimate:\n")
	fmt.Printf("  Input:   $%.4f (at $%.2f/1M tokens)\n", inputCost, inputRate)
	fmt.Printf("  Output:  $%.4f (at $%.2f/1M tokens)\n", outputCost, outputRate)
	fmt.Printf("  Total:   $%.4f per run\n", totalCost)

	fmt.Printf("\nContext:\n")
	fmt.Printf("  Limit:   %d tokens\n", contextLimit)
	fmt.Printf("  Used:    %d tokens (%.1f%%)\n", inputTokens, float64(inputTokens)/float64(contextLimit)*100)

	if inputTokens > warningThreshold {
		fmt.Printf("\n  \u26a0 WARNING: Prompt exceeds 80%% of context limit!\n")
		fmt.Printf("  Consider splitting into smaller tasks or reducing prompt size.\n")
	}
}

// promptPreviewCmd shows expanded prompt with variables
func promptPreviewCmd(args []string) {
	showEnv := false
	jsonOutput := false
	var rest []string
	for _, a := range args {
		switch a {
		case "--env":
			showEnv = true
		case "--json":
			jsonOutput = true
		default:
			rest = append(rest, a)
		}
	}

	if len(rest) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil prompt preview <task> [--env] [--full] [--json]\n")
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

	todo := findTodo(todos, rest[0])
	if todo == nil {
		fmt.Fprintf(os.Stderr, "task not found: %s\n", rest[0])
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Get merged env
	mergedEnv := dryRunMergeEnv(cfg.Env, todo.Env)
	resolvedEnv := dryRunResolveEnv(mergedEnv)

	// Find template variables
	templateVars := findTemplateVariables(todo.Content)

	if jsonOutput {
		type previewResult struct {
			Task          string            `json:"task"`
			Content       string            `json:"content"`
			Lines         int               `json:"lines"`
			TemplateVars  []string          `json:"template_vars"`
			EnvVars       map[string]string `json:"env_vars,omitempty"`
			Runner        string            `json:"runner"`
		}
		result := previewResult{
			Task:         todo.Name,
			Content:      todo.Content,
			Lines:        len(strings.Split(todo.Content, "\n")),
			TemplateVars: templateVars,
			Runner:       getRunnerCommand(todo, cfg),
		}
		if showEnv {
			result.EnvVars = resolvedEnv
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("Prompt Preview: %s\n\n", todo.Name)

	if len(templateVars) > 0 {
		fmt.Printf("Template Variables Found:\n")
		for _, v := range templateVars {
			value := ""
			if val, ok := resolvedEnv[v]; ok {
				value = val
				if len(value) > 20 {
					value = value[:20] + "..."
				}
			} else {
				value = "(not set)"
			}
			fmt.Printf("  {{.%s}} = %s\n", v, value)
		}
		fmt.Println()
	}

	fmt.Printf("Prompt Content (%d lines):\n", len(strings.Split(todo.Content, "\n")))
	lines := strings.Split(todo.Content, "\n")
	for i, line := range lines {
		if i >= 30 && len(lines) > 31 {
			fmt.Printf("  ... (%d more lines)\n", len(lines)-30)
			break
		}
		fmt.Printf("  %s\n", line)
	}

	if showEnv {
		fmt.Println("\nEnvironment Variables:")
		for k, v := range resolvedEnv {
			display := v
			if len(v) > 40 {
				display = v[:40] + "..."
			}
			fmt.Printf("  %s=%s\n", k, display)
		}
	}

	fmt.Printf("\nRunner: %s\n", getRunnerCommand(todo, cfg))
}

func getRunnerCommand(todo *project.Todo, cfg *config.Config) string {
	if todo.Runner != "" {
		return todo.Runner
	}
	if len(cfg.Runners) > 0 {
		return cfg.Runners[0]
	}
	if cfg.Runner != "" {
		return cfg.Runner
	}
	return "echo"
}

func findTemplateVariables(content string) []string {
	re := regexp.MustCompile(`\{\{(\.?[\w]+)\}\}`)
	matches := re.FindAllStringSubmatch(content, -1)

	seen := make(map[string]bool)
	var vars []string
	for _, m := range matches {
		if len(m) > 1 {
			v := strings.TrimPrefix(m[1], ".")
			if !seen[v] {
				seen[v] = true
				vars = append(vars, v)
			}
		}
	}
	return vars
}

func escapeForShell(s string) string {
	s = strings.ReplaceAll(s, `'`, `'\\''`)
	return s
}
