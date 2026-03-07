package main

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/johnjansen/anvil/internal/config"
)

func daemonCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: anvil daemon <subcommand>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  log [-f] [-n lines] [--level LEVEL] [--match PATTERN] [--since TIME] [--until TIME]")
		fmt.Fprintln(os.Stderr, "  config-validate           Validate config file")
		os.Exit(1)
	}
	switch args[0] {
	case "log":
		daemonLogCmd(args[1:])
	case "config-validate":
		daemonConfigValidateCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown daemon subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

// logLevelFromLine extracts the log level from a daemon log line.
// Lines contain markers like [warn], [error], [fatal], or are info-level by default.
func logLevelFromLine(line string) string {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "[error]") {
		return "error"
	}
	if strings.Contains(lower, "[fatal]") {
		return "error" // treat fatal as error level
	}
	if strings.Contains(lower, "[warn]") {
		return "warn"
	}
	// Worker and scheduler lines without explicit level markers are info.
	return "info"
}

// logLevelRank returns a numeric rank for level comparison (higher = more severe).
func logLevelRank(level string) int {
	switch strings.ToLower(level) {
	case "debug":
		return 0
	case "info":
		return 1
	case "warn":
		return 2
	case "error":
		return 3
	default:
		return 1
	}
}

// parseLogTimestamp extracts the HH:MM:SS timestamp from the start of a log line
// and returns it as a time.Time on today's date.
func parseLogTimestamp(line string) (time.Time, bool) {
	if len(line) < 8 {
		return time.Time{}, false
	}
	ts := line[:8]
	t, err := time.Parse("15:04:05", ts)
	if err != nil {
		return time.Time{}, false
	}
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local), true
}

// parseTimeArg parses a time argument that is either a Go duration ("1h", "30m")
// or a time-of-day ("10:00", "10:00:00").
func parseTimeArg(s string) (time.Time, error) {
	// Try Go duration first (e.g., "1h", "30m").
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d), nil
	}
	// Try time-of-day HH:MM:SS.
	if t, err := time.Parse("15:04:05", s); err == nil {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local), nil
	}
	// Try time-of-day HH:MM.
	if t, err := time.Parse("15:04", s); err == nil {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local), nil
	}
	return time.Time{}, fmt.Errorf("invalid time/duration: %s (use e.g. \"1h\", \"30m\", or \"10:00\")", s)
}

type logFilter struct {
	minLevel int    // minimum log level rank to show
	match    string // substring to match (case-insensitive)
	since    time.Time
	until    time.Time
}

func (lf *logFilter) matches(line string) bool {
	if line == "" {
		return false
	}
	if lf.minLevel > 0 {
		level := logLevelFromLine(line)
		if logLevelRank(level) < lf.minLevel {
			return false
		}
	}
	if lf.match != "" {
		if !strings.Contains(strings.ToLower(line), strings.ToLower(lf.match)) {
			return false
		}
	}
	if !lf.since.IsZero() || !lf.until.IsZero() {
		ts, ok := parseLogTimestamp(line)
		if !ok {
			return false // skip lines without timestamps when time filtering
		}
		if !lf.since.IsZero() && ts.Before(lf.since) {
			return false
		}
		if !lf.until.IsZero() && ts.After(lf.until) {
			return false
		}
	}
	return true
}

func (lf *logFilter) active() bool {
	return lf.minLevel > 0 || lf.match != "" || !lf.since.IsZero() || !lf.until.IsZero()
}

func daemonLogCmd(args []string) {
	follow := false
	numLines := 50
	filter := logFilter{}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f", "--follow":
			follow = true
		case "-n":
			if i+1 >= len(args) {
				log.Fatal("missing value for -n")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				log.Fatalf("invalid line count: %s", args[i])
			}
			numLines = n
		case "--level":
			if i+1 >= len(args) {
				log.Fatal("missing value for --level")
			}
			i++
			// Accept the minimum level; show that level and above.
			level := strings.ToLower(args[i])
			rank := logLevelRank(level)
			if level != "debug" && level != "info" && level != "warn" && level != "error" {
				log.Fatalf("invalid log level: %s (use debug, info, warn, or error)", args[i])
			}
			filter.minLevel = rank
		case "--match":
			if i+1 >= len(args) {
				log.Fatal("missing value for --match")
			}
			i++
			filter.match = args[i]
		case "--since":
			if i+1 >= len(args) {
				log.Fatal("missing value for --since")
			}
			i++
			t, err := parseTimeArg(args[i])
			if err != nil {
				log.Fatalf("--since: %v", err)
			}
			filter.since = t
		case "--until":
			if i+1 >= len(args) {
				log.Fatal("missing value for --until")
			}
			i++
			t, err := parseTimeArg(args[i])
			if err != nil {
				log.Fatalf("--until: %v", err)
			}
			filter.until = t
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `usage: anvil daemon log [-f] [-n lines] [--level LEVEL] [--match PATTERN]
                        [--since TIME] [--until TIME]

View the daemon log file (~/.anvil/daemon.log).

Options:
  -f, --follow     Follow the log (like tail -f)
  -n lines         Show last N lines (default 50)
  --level LEVEL    Minimum log level to show: debug, info, warn, error
                   Shows the specified level and above (e.g. --level warn shows warn+error)
  --match PATTERN  Show only lines containing PATTERN (case-insensitive)
  --since TIME     Show entries after TIME (duration like "1h" or time like "10:00")
  --until TIME     Show entries before TIME (duration like "1h" or time like "10:00")
`)
			os.Exit(0)
		}
	}

	logPath := config.DaemonLogPath()
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "no daemon log found (daemon has not run yet)")
		os.Exit(1)
	}

	// Read and display the last N lines.
	data, err := os.ReadFile(logPath)
	if err != nil {
		log.Fatalf("failed to read daemon log: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	// Remove trailing empty line from Split.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Apply filters, then take the last N matching lines.
	if filter.active() {
		filtered := make([]string, 0, len(lines))
		for _, line := range lines {
			if filter.matches(line) {
				filtered = append(filtered, line)
			}
		}
		lines = filtered
	}

	start := 0
	if len(lines) > numLines {
		start = len(lines) - numLines
	}
	for _, line := range lines[start:] {
		fmt.Println(line)
	}

	if !follow {
		return
	}

	// Follow mode: tail the file for new content.
	f, err := os.Open(logPath)
	if err != nil {
		log.Fatalf("failed to open daemon log: %v", err)
	}
	defer f.Close()

	// Seek to end.
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		log.Fatalf("failed to seek: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	remainder := ""
	buf := make([]byte, 4096)
	for {
		select {
		case <-sigCh:
			return
		default:
		}

		n, readErr := f.Read(buf)
		if n > 0 {
			chunk := remainder + string(buf[:n])
			chunkLines := strings.Split(chunk, "\n")
			// Last element may be incomplete; save it.
			remainder = chunkLines[len(chunkLines)-1]
			for _, line := range chunkLines[:len(chunkLines)-1] {
				if !filter.active() || filter.matches(line) {
					fmt.Println(line)
				}
			}
			continue
		}
		if readErr != nil && readErr != io.EOF {
			return
		}

		// Check if the file was rotated (new file at same path).
		newInfo, statErr := os.Stat(logPath)
		if statErr == nil {
			curInfo, _ := f.Stat()
			if curInfo != nil && !os.SameFile(curInfo, newInfo) {
				// File was rotated — reopen.
				f.Close()
				f, err = os.Open(logPath)
				if err != nil {
					return
				}
			}
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// daemonConfigValidateCmd validates the config file.
func daemonConfigValidateCmd(args []string) {
	showConfig := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--show" {
			showConfig = true
		} else if arg == "-h" || arg == "--help" {
			fmt.Println("Usage: anvil daemon config-validate [options]")
			fmt.Println("")
			fmt.Println("Validate the daemon config file (~/.anvil/config.yaml).")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("  --show    Show parsed config in YAML format")
			fmt.Println("  -h        Show this help")
			return
		}
	}

	// Load config (validates YAML syntax)
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Validate durations
	if cfg.Timeout <= 0 {
		fmt.Fprintf(os.Stderr, "Error: invalid timeout %v (must be > 0)\n", cfg.Timeout)
		os.Exit(1)
	}
	if cfg.TickInterval <= 0 {
		fmt.Fprintf(os.Stderr, "Error: invalid tick_interval %v (must be > 0)\n", cfg.TickInterval)
		os.Exit(1)
	}

	// Validate runners
	if len(cfg.Runners) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no runners configured (must have at least one)\n")
		os.Exit(1)
	}

	// Validate webhooks if present
	if cfg.Webhooks != nil {
		for name, webhook := range cfg.Webhooks {
			if webhook.URL != "" {
				u, err := url.Parse(webhook.URL)
				if err != nil || !u.IsAbs() {
					fmt.Fprintf(os.Stderr, "Error: invalid webhook URL for %s: %s\n", name, webhook.URL)
					os.Exit(1)
				}
			}
		}
	}

	// Show warnings for deprecated fields
	// (currently no deprecated fields in config, but structure is here)

	// If --show, output config
	if showConfig {
		fmt.Println("tick_interval:", cfg.TickInterval)
		fmt.Println("max_workers:", cfg.MaxWorkers)
		fmt.Println("timeout:", cfg.Timeout)
		fmt.Println("runners:")
		for _, r := range cfg.Runners {
			fmt.Println("  -", r)
		}
		if cfg.Hooks.OnSuccess != "" || cfg.Hooks.OnFailure != "" {
			fmt.Println("hooks:")
			fmt.Println("  on_success:", cfg.Hooks.OnSuccess)
			fmt.Println("  on_failure:", cfg.Hooks.OnFailure)
		}
		if cfg.Webhooks != nil {
			fmt.Println("webhooks:")
			for name, wh := range cfg.Webhooks {
				fmt.Printf("  %s:\n", name)
				fmt.Printf("    url: %s\n", wh.URL)
				if len(wh.Events) > 0 {
					fmt.Printf("    events:\n")
					for _, e := range wh.Events {
						fmt.Printf("      - %s\n", e)
					}
				}
			}
		}
		return
	}

	// Config is valid
	fmt.Println("✓ Config is valid")
}
