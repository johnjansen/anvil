package project

import (
	"fmt"
	"strings"
)

// UnifiedDiff produces a unified diff between two texts.
func UnifiedDiff(oldLabel, newLabel, oldText, newText string) string {
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")

	// Simple line-by-line diff using longest common subsequence
	lcs := lcsLines(oldLines, newLines)

	var hunks []hunk
	var current *hunk

	oi, ni, li := 0, 0, 0
	for oi < len(oldLines) || ni < len(newLines) {
		if li < len(lcs) && oi < len(oldLines) && ni < len(newLines) && oldLines[oi] == lcs[li] && newLines[ni] == lcs[li] {
			// Common line
			if current != nil {
				current.lines = append(current.lines, " "+oldLines[oi])
			}
			oi++
			ni++
			li++
		} else if li < len(lcs) && oi < len(oldLines) && oldLines[oi] != lcs[li] {
			// Removed line
			if current == nil {
				current = &hunk{oldStart: oi + 1, newStart: ni + 1}
				// Add up to 3 context lines before
				start := oi - 3
				if start < 0 {
					start = 0
				}
				for c := start; c < oi; c++ {
					current.lines = append(current.lines, " "+oldLines[c])
					current.oldStart = c + 1
					current.newStart = ni - (oi - c) + 1
				}
			}
			current.lines = append(current.lines, "-"+oldLines[oi])
			oi++
		} else if li < len(lcs) && ni < len(newLines) && newLines[ni] != lcs[li] {
			// Added line
			if current == nil {
				current = &hunk{oldStart: oi + 1, newStart: ni + 1}
				start := ni - 3
				if start < 0 {
					start = 0
				}
				for c := start; c < ni; c++ {
					current.lines = append(current.lines, " "+newLines[c])
					current.newStart = c + 1
					current.oldStart = oi - (ni - c) + 1
				}
			}
			current.lines = append(current.lines, "+"+newLines[ni])
			ni++
		} else if oi < len(oldLines) {
			if current == nil {
				current = &hunk{oldStart: oi + 1, newStart: ni + 1}
			}
			current.lines = append(current.lines, "-"+oldLines[oi])
			oi++
		} else if ni < len(newLines) {
			if current == nil {
				current = &hunk{oldStart: oi + 1, newStart: ni + 1}
			}
			current.lines = append(current.lines, "+"+newLines[ni])
			ni++
		}

		// Flush hunk if we've had 3+ context lines after changes
		if current != nil {
			contextCount := 0
			for j := len(current.lines) - 1; j >= 0; j-- {
				if current.lines[j][0] == ' ' {
					contextCount++
				} else {
					break
				}
			}
			if contextCount >= 3 && (oi >= len(oldLines) && ni >= len(newLines) || (li < len(lcs))) {
				hunks = append(hunks, *current)
				current = nil
			}
		}
	}
	if current != nil {
		hunks = append(hunks, *current)
	}

	if len(hunks) == 0 {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n", oldLabel)
	fmt.Fprintf(&sb, "+++ %s\n", newLabel)
	for _, h := range hunks {
		oldCount := 0
		newCount := 0
		for _, l := range h.lines {
			switch l[0] {
			case ' ':
				oldCount++
				newCount++
			case '-':
				oldCount++
			case '+':
				newCount++
			}
		}
		fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", h.oldStart, oldCount, h.newStart, newCount)
		for _, l := range h.lines {
			sb.WriteString(l)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

type hunk struct {
	oldStart int
	newStart int
	lines    []string // each prefixed with ' ', '+', or '-'
}

// lcsLines computes the longest common subsequence of two line slices.
func lcsLines(a, b []string) []string {
	m, n := len(a), len(b)
	// Build LCS table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	// Backtrack to find LCS
	result := make([]string, dp[m][n])
	i, j, k := m, n, dp[m][n]-1
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			result[k] = a[i-1]
			i--
			j--
			k--
		} else if dp[i-1][j] >= dp[i][j-1] {
			i--
		} else {
			j--
		}
	}
	return result
}
