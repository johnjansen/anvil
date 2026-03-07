package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/daemon"
	"github.com/johnjansen/anvil/internal/project"
)

// Use blank imports to ensure packages are available for command functions
// that will be moved to this file in subsequent tasks
var (
	_ = config.Load
	_ = daemon.New
	_ = project.Load
	_ = fmt.Sprintf
	_ = os.Exit
	_ = strings.TrimSpace
	_ = time.Now
)
