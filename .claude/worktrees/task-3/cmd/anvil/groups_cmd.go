package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"github.com/johnjansen/anvil/internal/config"
)

func groupsCmd(args []string) {
	fs := flag.NewFlagSet("groups", flag.ContinueOnError)
	jsonFlag := fs.Bool("json", false, "Output in JSON format")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: anvil groups [--json]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Show concurrency group status.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  --json    Output in JSON format")
		os.Exit(1)
	}

	if err := fs.Parse(args); err != nil {
		fs.Usage()
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if *jsonFlag {
		// For JSON output, we'd need to connect to the daemon to get live data
		// For now, just output the config
		jsonData, _ := json.MarshalIndent(cfg.ConcurrencyGroups, "", "  ")
		fmt.Println(string(jsonData))
		return
	}

	if len(cfg.ConcurrencyGroups) == 0 {
		fmt.Println("No concurrency groups configured")
		return
	}

	// For tabular output, we'd also need to connect to the daemon to get live data
	// For now, just show the configured groups
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "GROUP\tMAX_CONCURRENT\tMIN_AVAILABLE\tRATE_LIMIT\tPRIORITY_BOOST")
	for name, group := range cfg.ConcurrencyGroups {
		rateLimit := "none"
		if group.RateLimit.RequestsPerMinute > 0 {
			rateLimit = fmt.Sprintf("%d req/min", group.RateLimit.RequestsPerMinute)
		} else if group.RateLimit.TokenRateLimit > 0 {
			rateLimit = fmt.Sprintf("%.2f tokens/min", group.RateLimit.TokenRateLimit)
		}

		priorityBoost := "false"
		if group.PriorityBoost {
			priorityBoost = "true"
		}

		fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%s\n",
			name,
			group.MaxConcurrent,
			group.MinAvailable,
			rateLimit,
			priorityBoost)
	}
	w.Flush()
}

// truncate shortens a string to the specified length, collapsing any
// embedded newlines/whitespace runs into single spaces for table display.
