package main

import (
	"fmt"
	"os"
)

func taskCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: anvil task <subcommand> [options]\n")
		fmt.Fprintf(os.Stderr, "Run 'anvil help' for more information.\n")
		os.Exit(1)
	}

	switch args[0] {
	case "create":
		taskCreateCmd(args[1:])
	case "ls":
		taskLsCmd(args[1:])
	case "get":
		taskGetCmd(args[1:])
	case "log":
		taskLogCmd(args[1:])
	case "rm":
		taskRmCmd(args[1:])
	case "run":
		taskRunCmd(args[1:])
	case "kill":
		taskKillCmd(args[1:])
	case "history":
		taskHistoryCmd(args[1:])
	case "stop-on-idle":
		taskStopOnIdleCmd(args[1:])
	case "unlock":
		taskUnlockCmd(args[1:])
	case "edit":
		taskEditCmd(args[1:])
	case "pause":
		taskPauseCmd(args[1:])
	case "resume":
		taskResumeCmd(args[1:])
	case "queue":
		taskQueueCmd(args[1:])
	case "timeout":
		taskTimeoutCmd(args[1:])
	case "extend-timeout":
		taskExtendTimeoutCmd(args[1:])
	case "start":
		taskStartCmd(args[1:])
	case "stop":
		taskStopCmd(args[1:])
	case "next":
		taskNextCmd(args[1:])
	case "export":
		taskExportCmd(args[1:])
	case "import":
		taskImportCmd(args[1:])
	case "wait":
		taskWaitCmd(args[1:])
	case "analyze":
		taskAnalyzeCmd(args[1:])
	case "reset-budget":
		taskResetBudgetCmd(args[1:])
	case "results":
		taskResultsCmd(args[1:])
	case "state":
		taskStateCmd(args[1:])
	case "sla":
		taskSlaCmd(args[1:])
	case "alerts":
		taskAlertsCmd(args[1:])
	case "rate-limits":
		taskRateLimitsCmd(args[1:])
	case "health":
		taskHealthCmd(args[1:])
	case "find":
		// "find" is an alias for "ls --match" - inject the pattern as --match flag
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "usage: anvil task find <pattern>\n")
			os.Exit(1)
		}
		taskLsCmd([]string{"--match", args[1]})
	case "backfill":
		taskBackfillCmd(args[1:])
	case "diff":
		taskDiffCmd(args[1:])
	case "restore":
		taskRestoreCmd(args[1:])
	case "rollback":
		taskRollbackCmd(args[1:])
	case "blame":
		taskBlameCmd(args[1:])
	case "dry-run":
		taskDryRunCmd(args[1:])
	case "activity":
	case "cache":
		taskCacheCmd(args[1:])
	case "snapshot":
		taskSnapshotCmd(args[1:])
	case "replay":
		taskReplayCmd(args[1:])
	case "snapshot-diff":
		taskSnapshotDiffCmd(args[1:])
	case "subscription":
		taskSubscriptionCmd(args[1:])
		taskActivityCmd(args[1:])
	case "trigger-check":
		taskTriggerCheckCmd(args[1:])
	case "estimate":
		taskEstimateCmd(args[1:])
	case "forecast":
		taskForecastCmd(args[1:])
	case "group":
		groupCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown task command: %s\n", args[0])
		fmt.Fprintf(os.Stderr, "Run 'anvil help' for more information.\n")
		os.Exit(1)
	}
}
