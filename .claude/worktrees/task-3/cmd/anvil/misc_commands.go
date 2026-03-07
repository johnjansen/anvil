// Package main provides the anvil CLI application.
//
// This file (misc_commands.go) serves as a placeholder documenting where
// miscellaneous command functions are located.
//
// Command functions organized in separate files:
// - initCmd, registerCmd: init_cmd.go
// - serveCmd, watchCmd2: daemon_lifecycle.go
// - statusCmd, reloadCmd, cleanupCmd, psCmd: status.go
// - addCmd: legacy.go
// - dispatchCmd: task_create.go
// - logsCmd: logs.go
// - updateCmd: update.go
// - usageCmd: usage.go
// - templateCmd, templateListCmd, templateGetCmd: template.go
// - groupsCmd: groups_cmd.go
// - promptCmd: prompt.go
//
// The following commands are handled inline in main.go with deprecation messages:
// - watch, unwatch, list, get, delete, log
package main
