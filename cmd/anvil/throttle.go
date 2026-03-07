package main

import (
	"fmt"
	"os"

	"github.com/johnjansen/anvil/internal/daemon"
)

func pauseCmd(args []string) {
	label := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `usage: anvil pause [--label <label>]

Pause task execution. Without --label, pauses all tasks globally.
With --label, pauses only tasks with the specified label.

Running tasks are not affected — only new dispatching is paused.

Options:
  --label <label>   Pause only tasks with this label
`)
			return
		case "--label":
			if i+1 < len(args) {
				label = args[i+1]
				i++
			} else {
				fmt.Fprintln(os.Stderr, "error: --label requires a value")
				os.Exit(1)
			}
		}
	}

	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		os.Exit(1)
	}

	if err := daemon.SendThrottlePauseRequest(label); err != nil {
		fmt.Fprintf(os.Stderr, "failed to pause: %v\n", err)
		os.Exit(1)
	}

	if label != "" {
		fmt.Printf("paused tasks with label %q\n", label)
	} else {
		fmt.Println("all task execution paused")
	}
}

func resumeCmd(args []string) {
	label := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `usage: anvil resume [--label <label>]

Resume task execution. Without --label, resumes global pause.
With --label, resumes only tasks with the specified label.

Options:
  --label <label>   Resume only tasks with this label
`)
			return
		case "--label":
			if i+1 < len(args) {
				label = args[i+1]
				i++
			} else {
				fmt.Fprintln(os.Stderr, "error: --label requires a value")
				os.Exit(1)
			}
		}
	}

	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		os.Exit(1)
	}

	if err := daemon.SendThrottleResumeRequest(label); err != nil {
		fmt.Fprintf(os.Stderr, "failed to resume: %v\n", err)
		os.Exit(1)
	}

	if label != "" {
		fmt.Printf("resumed tasks with label %q\n", label)
	} else {
		fmt.Println("task execution resumed")
	}
}

func throttleCmd(args []string) {
	rate := ""
	off := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			fmt.Fprintf(os.Stderr, `usage: anvil throttle --rate N/m | --off

Control the task execution rate.

Options:
  --rate N/m   Limit to N tasks per minute (e.g. 1/m, 5/m)
  --off        Disable throttle rate limiting
`)
			return
		case "--off":
			off = true
		case "--rate":
			if i+1 < len(args) {
				rate = args[i+1]
				i++
			} else {
				fmt.Fprintln(os.Stderr, "error: --rate requires a value (e.g. 1/m)")
				os.Exit(1)
			}
		}
	}

	if !off && rate == "" {
		// Show current throttle state
		if !daemon.IsDaemonRunning() {
			fmt.Println("daemon not running")
			os.Exit(1)
		}
		state, err := daemon.SendThrottleStateRequest()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to get throttle state: %v\n", err)
			os.Exit(1)
		}
		if state.RatePerMin > 0 {
			fmt.Printf("throttle rate: %d/m\n", state.RatePerMin)
		} else {
			fmt.Println("throttle rate: unlimited")
		}
		return
	}

	if !daemon.IsDaemonRunning() {
		fmt.Println("daemon not running")
		os.Exit(1)
	}

	ratePerMin := 0
	if !off {
		var err error
		ratePerMin, err = daemon.ParseRate(rate)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}

	if err := daemon.SendThrottleRateRequest(ratePerMin); err != nil {
		fmt.Fprintf(os.Stderr, "failed to set throttle rate: %v\n", err)
		os.Exit(1)
	}

	if ratePerMin > 0 {
		fmt.Printf("throttle rate set to %d/m\n", ratePerMin)
	} else {
		fmt.Println("throttle rate disabled")
	}
}
