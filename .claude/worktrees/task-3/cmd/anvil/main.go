package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
// If not set (e.g. go run), init() falls back to the module version from debug.BuildInfo.
var version = "dev"

func init() {
	if version != "dev" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		fmt.Fprintln(os.Stderr, "'anvil serve' has been renamed. Did you mean 'anvil watch'?")
		serveCmd()
	case "watch":
		watchCmd2(os.Args[2:])
	case "unwatch":
		fmt.Fprintln(os.Stderr, "'anvil unwatch' has been removed. Did you mean 'anvil project rm [path]'?")
		os.Exit(1)
	case "status":
		statusCmd(os.Args[2:])
	case "cleanup":
		cleanupCmd(os.Args[2:])
	case "ps":
		psCmd(os.Args[2:])
	case "init":
		initCmd(os.Args[2:])
	case "register":
		registerCmd(os.Args[2:])
	case "add":
		addCmd(os.Args[2:])
	case "dispatch":
		dispatchCmd(os.Args[2:])
	case "list":
		fmt.Fprintln(os.Stderr, "'anvil list' has been removed. Did you mean 'anvil task ls'?")
		os.Exit(1)
	case "get":
		fmt.Fprintln(os.Stderr, "'anvil get' has been removed. Did you mean 'anvil task get <name>'?")
		os.Exit(1)
	case "delete":
		fmt.Fprintln(os.Stderr, "'anvil delete' has been removed. Did you mean 'anvil task rm <name>'?")
		os.Exit(1)
	case "log":
		fmt.Fprintln(os.Stderr, "'anvil log' has been removed. Did you mean 'anvil task log <name>'?")
		os.Exit(1)
	case "logs":
		logsCmd(os.Args[2:])
	case "daemon":
		daemonCmd(os.Args[2:])
	case "stop-on-idle":
		stopOnIdleCmd()
	case "task":
		taskCmd(os.Args[2:])
	case "group":
		groupCmd(os.Args[2:])
	case "groups":
		groupsCmd(os.Args[2:])
	case "prompt":
		promptCmd(os.Args[2:])
	case "project":
		projectCmd(os.Args[2:])
	case "cluster":
		clusterCmd(os.Args[2:])
	case "template":
		templateCmd(os.Args[2:])
	case "usage":
		usageCmd(os.Args[2:])
	case "update":
		updateCmd(os.Args[2:])
	case "reload":
		reloadCmd(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("anvil %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `anvil - central task dispatcher for LLM projects

Usage:
  anvil <command> [options]

Commands:
  init [--force] [path]    Initialize a project and register it for watching
  register [path]          Register a project for watching (without full init)
  watch [-d|--daemonize]   Start the daemon (press 'd' to detach to background)
  watch --install          Install as system service (auto-start on boot)
  watch --uninstall        Remove the system service
  watch --status           Show system service status
  watch --stop [--graceful] Stop the daemon (--graceful waits for tasks, --force kills)
  add [options] <task>     Add a task to the current project
  dispatch [options] <task>  Add a one-shot task and wait for completion, returning the result
  logs [<name>]            Raw worker output (all tasks if no name given)
  ps [--json] [-w|--watch] Show running tasks (--watch for live dashboard)
  status [--json]          Show watched projects and daemon status
  project <subcommand>     Project management commands
  daemon <subcommand>      Daemon management commands
  prompt <subcommand>    Prompt testing and validation tools
  update [--check]         Update anvil to the latest release
  reload [--graceful]       Reload daemon configuration (--graceful waits for tasks)
  version                  Show version

Add options:
  -p, --priority int          Task priority 0-9 (default 1)
  -s, --schedule string      Cron schedule (e.g., "*/15 * * * *"), "" for one-shot
  -o, --once                 Create a one-shot task (no schedule)
  -n, --dry-run              Validate schedule without creating task
  -f, --file path            Read task content from a file
  -t, --template name        Use a template for task configuration
  -                          Read task content from stdin
      --pre-check string    Shell command to skip task if non-zero exit
      --allowed-tools string  Comma-separated tool allowlist (e.g. "Bash,Read") or scoped (e.g. "Bash(gh:*)")
      --max-concurrent int    Max parallel instances (default 1)
      --skip-permissions     Bypass all tool permission prompts
      --strict               Fail if schedule conflicts with existing tasks
      --no-overlap-check    Skip schedule overlap detection
      --depends-on dep      Task dependency (repeatable; use project:task for cross-project)

Task subcommands:
  create [options] <task>   Create a new task
  ls [-a|--all] [--json] [--label L]  List tasks (--all for all projects, --label to filter)
  find <pattern>            Find tasks by name pattern (alias for ls --match)
  get <name> [--json]       Show task details including run status
  log [-f] <name>           Show execution log (-f to follow)
  history <name> [--json]    Show run history
  rm <name>                 Remove a task (kills if running)
  run <name>                Trigger immediate execution (bypass cron)
  kill <name>               Kill a running task (persistent tasks auto-restart)
  stop <name>               Stop a persistent task permanently (kill + prevent restart)
  start <name>              Start a stopped persistent task (dispatches on next tick)
  stop-on-idle <name>       Finish current run then stop rescheduling task
  unlock <name>             Remove stale lock file to allow retry
  queue [--json]            Show daemon queue status and skip reasons
  pause <name>              Pause a task (sets disabled: true)
  resume <name>             Resume a paused task (sets disabled: false)
  edit <name>               Edit task (schedule, priority, content, labels, or --remove field)
  timeout [name]            Show task timeout progress (--all for all tasks)
  extend-timeout <name> <dur>  Extend a running task's timeout by the given duration
  next [name]              Show next scheduled run time (--all for all projects)
  wait <name> [--timeout D]  Block until a running task completes (exit 0=ok, 1=fail, 2=timeout)
  analyze [-a|--all]         Detect scheduling conflicts and overlapping tasks
  pipeline [--dot|--verbose] [--all]  Visualize task dependency pipelines
  reset-budget <name>        Reset persistent task budget consumption
  state <name>              View, export, import, or clear task state
  dry-run <name> [options]   Validate and preview task config without executing
  activity <name> [options]  Show task activity history (--type, --since, --limit, --export, --json)
  snapshot <name> [--run <id>] [--file <filename>]  View task execution snapshots for debugging
  snapshot-diff <name> --run1 <id1> --run2 <id2>  Compare two task execution snapshots
  sla [--verbose] [--reset] [--json]  Show SLA violations (--verbose for all, --reset to clear)
  export [names...] [-a] [-o file]  Export tasks to JSON for sharing or backup
  import <file> [options]   Import tasks from a JSON export file
  group <subcommand>        Task group management commands

Group subcommands:
  ls [-a|--all] [--json] [--label L]  List groups (--all for all projects, --label to filter)
  get <name> [--json]       Show group details including run status
  run <name>                Run a task group immediately
  history <name> [--json]    Show group execution history

Project subcommands:
  create [path]            Initialize and watch a project in one step
  ls [-a|--all] [--json]   List watched projects
  get [path]               Show project details and running tasks
  rm [path] [--clean]      Unwatch a project (--clean removes .anvil/ too)

Daemon subcommands:
  log [-f] [-n lines] [--level LEVEL] [--match PATTERN] [--since TIME] [--until TIME]
                      View daemon log (filtering options for level, match, since, until)
  config-validate [--show]  Validate config file (--show to display parsed config)

Configuration:
  ~/.anvil/config.yaml   Daemon config
  <project>/.anvil/      Project config and todos
`)
}
