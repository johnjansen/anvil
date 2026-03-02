package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/daemon"
	"gopkg.in/yaml.v3"
)

type watchFrontmatter struct {
	Path      string    `yaml:"path"`
	WatchedAt time.Time `yaml:"watched_at"`
}

func projectHash(absPath string) string {
	h := sha256.Sum256([]byte(absPath))
	return fmt.Sprintf("%x", h[:4])
}

func loadAllWatched() ([]watchFrontmatter, error) {
	watchedDir := config.WatchedDir()
	dirs, err := os.ReadDir(watchedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var result []watchFrontmatter
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}

		dirPath := filepath.Join(watchedDir, d.Name())
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		// Sort entries, take the latest .md file
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() > entries[j].Name()
		})

		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}

			data, err := os.ReadFile(filepath.Join(dirPath, e.Name()))
			if err != nil {
				break
			}

			content := string(data)
			start := strings.Index(content, "---\n")
			if start == -1 {
				break
			}
			end := strings.Index(content[start+4:], "\n---")
			if end == -1 {
				break
			}

			var fm watchFrontmatter
			if err := yaml.Unmarshal([]byte(content[start+4:start+4+end]), &fm); err != nil {
				break
			}

			result = append(result, fm)
			break
		}
	}

	return result, nil
}

// clusterCmd dispatches cluster subcommands.
func clusterCmd(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: anvil cluster <subcommand>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Subcommands:")
		fmt.Fprintln(os.Stderr, "  status [--json]    Show cluster members and leadership")
		fmt.Fprintln(os.Stderr, "  health [--json]    Show cluster health assessment")
		fmt.Fprintln(os.Stderr, "  leave              Remove this daemon from the cluster")
		os.Exit(1)
	}
	switch args[0] {
	case "status":
		clusterStatusCmd(args[1:])
	case "health":
		clusterHealthCmd(args[1:])
	case "leave":
		clusterLeaveCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown cluster command: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Run 'anvil cluster' for usage.")
		os.Exit(1)
	}
}

// clusterStatusCmd shows cluster status.
func clusterStatusCmd(args []string) {
	jsonFlag := false
	for _, a := range args {
		if a == "--json" {
			jsonFlag = true
		}
	}

	status, err := daemon.SendClusterStatusRequest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot connect to daemon -- is it running?\n")
		os.Exit(1)
	}

	// Check if cluster is disabled
	if enabled, ok := status["enabled"]; ok {
		if b, ok := enabled.(bool); ok && !b {
			if jsonFlag {
				json.NewEncoder(os.Stdout).Encode(status)
			} else {
				fmt.Fprintln(os.Stderr, "Cluster mode is not enabled.")
			}
			os.Exit(1)
		}
	}

	if jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(status)
		return
	}

	// Human-readable output
	fmt.Println("Cluster Status")
	nodeID, _ := status["node_id"].(string)
	role, _ := status["role"].(string)
	term, _ := status["term"].(float64)
	leaderID, _ := status["leader_id"].(string)
	clusterSize, _ := status["cluster_size"].(float64)

	fmt.Printf("  Node:    %s\n", nodeID)
	fmt.Printf("  Role:    %s\n", role)
	fmt.Printf("  Term:    %.0f\n", term)
	fmt.Printf("  Leader:  %s\n", leaderID)
	fmt.Printf("  Members: %.0f\n", clusterSize)
	fmt.Println()

	members, ok := status["members"].([]any)
	if ok && len(members) > 0 {
		fmt.Printf("  %-20s %-12s %s\n", "ID", "ROLE", "LAST SEEN")
		for _, m := range members {
			member, ok := m.(map[string]any)
			if !ok {
				continue
			}
			id, _ := member["id"].(string)
			mRole, _ := member["role"].(string)
			lastSeen, _ := member["last_seen"].(string)

			display := lastSeen
			if t, err := time.Parse(time.RFC3339Nano, lastSeen); err == nil {
				ago := time.Since(t)
				if ago < time.Second {
					display = "now"
				} else {
					display = fmt.Sprintf("%s ago", ago.Truncate(time.Second))
				}
			}
			fmt.Printf("  %-20s %-12s %s\n", id, mRole, display)
		}
	}
}

// clusterHealthCmd shows cluster health assessment.
func clusterHealthCmd(args []string) {
	jsonFlag := false
	for _, a := range args {
		if a == "--json" {
			jsonFlag = true
		}
	}

	status, err := daemon.SendClusterStatusRequest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot connect to daemon -- is it running?\n")
		os.Exit(1)
	}

	// Check if cluster is disabled
	if enabled, ok := status["enabled"]; ok {
		if b, ok := enabled.(bool); ok && !b {
			if jsonFlag {
				json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "disabled"})
			} else {
				fmt.Fprintln(os.Stderr, "Cluster mode is not enabled.")
			}
			os.Exit(1)
		}
	}

	// Derive health from status
	nodeID, _ := status["node_id"].(string)
	role, _ := status["role"].(string)
	leaderID, _ := status["leader_id"].(string)
	clusterSize, _ := status["cluster_size"].(float64)
	members, _ := status["members"].([]any)

	healthStatus := "healthy"
	staleCount := 0
	var staleMembers []string

	// Check for stale members (last seen > 15s ago, ~3x default 5s heartbeat)
	staleThreshold := 15 * time.Second
	for _, m := range members {
		member, ok := m.(map[string]any)
		if !ok {
			continue
		}
		lastSeen, _ := member["last_seen"].(string)
		mID, _ := member["id"].(string)
		if mID == nodeID {
			continue // skip self
		}
		if t, err := time.Parse(time.RFC3339Nano, lastSeen); err == nil {
			if time.Since(t) > staleThreshold {
				staleCount++
				staleMembers = append(staleMembers, fmt.Sprintf("%s (last seen %s ago)", mID, time.Since(t).Truncate(time.Second)))
			}
		}
	}

	if leaderID == "" {
		healthStatus = "unhealthy"
	} else if staleCount > 0 {
		healthStatus = "degraded"
	}

	responsive := int(clusterSize) - staleCount

	if jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(map[string]any{
			"status":       healthStatus,
			"node_id":      nodeID,
			"role":         role,
			"leader_id":    leaderID,
			"cluster_size": int(clusterSize),
			"stale_count":  staleCount,
		})
		return
	}

	fmt.Printf("Cluster Health: %s\n", healthStatus)
	fmt.Printf("  Leader:  %s\n", func() string {
		if leaderID == "" {
			return "(none)"
		}
		return leaderID
	}())
	fmt.Printf("  Members: %d/%d responsive\n", responsive, int(clusterSize))
	for _, s := range staleMembers {
		fmt.Printf("  Stale:   %s\n", s)
	}
}

// clusterLeaveCmd removes this daemon from the cluster.
func clusterLeaveCmd(args []string) {
	result, err := daemon.SendClusterLeaveRequest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot connect to daemon -- is it running?\n")
		os.Exit(1)
	}

	if left, ok := result["left"].(bool); ok && left {
		nodeID, _ := result["node_id"].(string)
		fmt.Printf("Node %s has left the cluster.\n", nodeID)
	} else {
		errMsg, _ := result["error"].(string)
		if errMsg == "" {
			errMsg = "unknown error"
		}
		fmt.Fprintf(os.Stderr, "Cluster mode is not enabled.\n")
		os.Exit(1)
	}
}
