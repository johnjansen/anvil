package project

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// RiskAnalyzer analyzes task runs to predict failures.
type RiskAnalyzer struct {
	projectPath string
	taskID      string
	thresholds RiskThresholds
}

// NewRiskAnalyzer creates a new RiskAnalyzer for a task.
func NewRiskAnalyzer(projectPath, taskID string, thresholds RiskThresholds) *RiskAnalyzer {
	if thresholds.HighThreshold == 0 {
		thresholds = GetDefaultRiskThresholds()
	}
	return &RiskAnalyzer{
		projectPath: projectPath,
		taskID:      taskID,
		thresholds:  thresholds,
	}
}

// AnalyzeTask performs a full risk analysis on a task.
func (ra *RiskAnalyzer) AnalyzeTask(runs []RunRecord) (*TaskRiskState, error) {
	if len(runs) < ra.thresholds.MinRunsForAnalysis {
		return &TaskRiskState{
			CurrentRisk:     RiskLevelLow,
			RiskScore:       0,
			LastUpdated:     time.Now(),
			HistoricalStats: TaskHistoricalStats{TotalRuns: len(runs)},
			RiskFactors:    []RiskFactor{},
		}, nil
	}

	// Sort runs by time (newest first)
	sortedRuns := make([]RunRecord, len(runs))
	copy(sortedRuns, runs)
	sort.Slice(sortedRuns, func(i, j int) bool {
		return sortedRuns[i].Started.After(sortedRuns[j].Started)
	})

	stats := ra.calculateStats(sortedRuns)
	riskScore := ra.CalculateRiskScore(sortedRuns, stats)
	riskLevel := ra.calculateRiskLevel(riskScore)
	factors := ra.DetectPatterns(sortedRuns)

	return &TaskRiskState{
		CurrentRisk:     riskLevel,
		RiskScore:       riskScore,
		LastUpdated:     time.Now(),
		HistoricalStats: stats,
		RiskFactors:     factors,
	}, nil
}

// calculateStats computes historical statistics from runs.
func (ra *RiskAnalyzer) calculateStats(runs []RunRecord) TaskHistoricalStats {
	total := len(runs)
	successes := 0

	for _, r := range runs {
		if r.Success {
			successes++
		}
	}

	successRate := 0.0
	if total > 0 {
		successRate = float64(successes) / float64(total)
	}

	// Recent failures (last 10 runs)
	recentFailures := 0
	recentCount := minInt(10, total)
	for i := 0; i < recentCount; i++ {
		if !runs[i].Success {
			recentFailures++
		}
	}

	trend := ra.CalculateTrend(runs)

	return TaskHistoricalStats{
		TotalRuns:      total,
		SuccessRate:    successRate,
		RecentFailures: recentFailures,
		TrendDirection: trend,
	}
}

// CalculateRiskScore computes the overall risk score (0.0-1.0).
func (ra *RiskAnalyzer) CalculateRiskScore(runs []RunRecord, stats TaskHistoricalStats) float64 {
	if len(runs) < ra.thresholds.MinRunsForAnalysis {
		return 0
	}

	// Base risk from success rate (inverse - lower success = higher risk)
	successRatePenalty := (1.0 - stats.SuccessRate) * 0.3

	// Recent failure penalty
	recentFailurePenalty := float64(stats.RecentFailures) / 10.0 * 0.3

	// Trend penalty
	trendPenalty := 0.0
	switch stats.TrendDirection {
	case "declining":
		trendPenalty = 0.2
	case "sharply_declining":
		trendPenalty = 0.4
	}

	// Pattern correlation penalty (max from all detected patterns)
	patternPenalty := ra.calculatePatternPenalty(runs)

	riskScore := successRatePenalty + recentFailurePenalty + trendPenalty + patternPenalty
	return math.Min(1.0, math.Max(0.0, riskScore))
}

// calculateRiskLevel determines the risk level from the score.
func (ra *RiskAnalyzer) calculateRiskLevel(score float64) RiskLevel {
	if score >= ra.thresholds.HighThreshold {
		return RiskLevelHigh
	}
	if score >= ra.thresholds.MediumThreshold {
		return RiskLevelMedium
	}
	return RiskLevelLow
}

// DetectPatterns analyzes runs to find correlations with failures.
func (ra *RiskAnalyzer) DetectPatterns(runs []RunRecord) []RiskFactor {
	var factors []RiskFactor

	// Check token correlation
	if factor := ra.detectTokenPattern(runs); factor != nil {
		factors = append(factors, *factor)
	}

	// Check time of day pattern
	if factor := ra.detectTimePattern(runs); factor != nil {
		factors = append(factors, *factor)
	}

	// Check runtime pattern
	if factor := ra.detectRuntimePattern(runs); factor != nil {
		factors = append(factors, *factor)
	}

	// Check error pattern
	if factor := ra.detectErrorPattern(runs); factor != nil {
		factors = append(factors, *factor)
	}

	return factors
}

// detectTokenPattern looks for correlation between high token count and failures.
func (ra *RiskAnalyzer) detectTokenPattern(runs []RunRecord) *RiskFactor {
	var failedWithHighTokens []int
	var succeededWithHighTokens []int

	threshold := 100000 // 100K tokens

	for _, r := range runs {
		tokens := r.InputTokens + r.OutputTokens
		if tokens > threshold {
			if r.Success {
				succeededWithHighTokens = append(succeededWithHighTokens, tokens)
			} else {
				failedWithHighTokens = append(failedWithHighTokens, tokens)
			}
		}
	}

	if len(failedWithHighTokens) > 0 && len(failedWithHighTokens) >= len(succeededWithHighTokens) {
		avgTokens := 0
		for _, t := range failedWithHighTokens {
			avgTokens += t
		}
		avgTokens /= len(failedWithHighTokens)

		score := float64(len(failedWithHighTokens)) / float64(len(failedWithHighTokens)+len(succeededWithHighTokens))
		value := formatTokenCount(avgTokens)

		factor := RiskFactor{
			Type:  "high_tokens",
			Value: "> " + value,
			Score: score * 0.5,
		}
		return &factor
	}

	return nil
}

// detectTimePattern looks for correlation between time of day and failures.
func (ra *RiskAnalyzer) detectTimePattern(runs []RunRecord) *RiskFactor {
	hourFailures := make(map[int]int)
	hourSuccesses := make(map[int]int)

	for _, r := range runs {
		hour := r.Started.Hour()
		if r.Success {
			hourSuccesses[hour]++
		} else {
			hourFailures[hour]++
		}
	}

	var worstHour int
	var worstRatio float64

	for hour, failures := range hourFailures {
		successes := hourSuccesses[hour]
		total := failures + successes
		if total >= 3 { // minimum sample size
			ratio := float64(failures) / float64(total)
			if ratio > worstRatio {
				worstRatio = ratio
				worstHour = hour
			}
		}
	}

	if worstRatio > 0.5 {
		factor := RiskFactor{
			Type:  "time_of_day",
			Value: formatHour(worstHour),
			Score: worstRatio * 0.3,
		}
		return &factor
	}

	return nil
}

// detectRuntimePattern looks for correlation between long runtime and failures.
func (ra *RiskAnalyzer) detectRuntimePattern(runs []RunRecord) *RiskFactor {
	var longRuntimeFailures int
	var longRuntimeSuccesses int

	threshold := 5 * time.Minute

	for _, r := range runs {
		if !r.Started.IsZero() && !r.Finished.IsZero() {
			duration := r.Finished.Sub(r.Started)
			if duration > threshold {
				if r.Success {
					longRuntimeSuccesses++
				} else {
					longRuntimeFailures++
				}
			}
		}
	}

	if longRuntimeFailures > 0 && longRuntimeFailures >= longRuntimeSuccesses {
		score := float64(longRuntimeFailures) / float64(longRuntimeFailures+longRuntimeSuccesses)
		factor := RiskFactor{
			Type:  "runtime",
			Value: "> 5 minutes",
			Score: score * 0.4,
		}
		return &factor
	}

	return nil
}

// detectErrorPattern looks for common error patterns in failures.
func (ra *RiskAnalyzer) detectErrorPattern(runs []RunRecord) *RiskFactor {
	errorCounts := make(map[string]int)

	for _, r := range runs {
		if !r.Success && r.Error != "" {
			errorCat := categorizeError(r.Error)
			errorCounts[errorCat]++
		}
	}

	var worstError string
	var worstCount int

	for error, count := range errorCounts {
		if count > worstCount {
			worstCount = count
			worstError = error
		}
	}

	if worstCount >= 2 {
		score := float64(worstCount) / 10.0 // normalize
		factor := RiskFactor{
			Type:  "error_pattern",
			Value: worstError,
			Score: math.Min(0.5, score),
		}
		return &factor
	}

	return nil
}

// calculatePatternPenalty calculates the max penalty from detected patterns.
func (ra *RiskAnalyzer) calculatePatternPenalty(runs []RunRecord) float64 {
	factors := ra.DetectPatterns(runs)

	var maxPenalty float64
	for _, f := range factors {
		if f.Score > maxPenalty {
			maxPenalty = f.Score
		}
	}

	return maxPenalty * 0.2 // Weight pattern detection at 20%
}

// CalculateTrend determines the success rate trend over time.
func (ra *RiskAnalyzer) CalculateTrend(runs []RunRecord) string {
	if len(runs) < 10 {
		return "stable"
	}

	// Split into old (first half) and recent (second half)
	mid := len(runs) / 2
	oldRuns := runs[mid:]
	recentRuns := runs[:mid]

	oldSuccess := 0
	for _, r := range oldRuns {
		if r.Success {
			oldSuccess++
		}
	}
	oldRate := float64(oldSuccess) / float64(len(oldRuns))

	recentSuccess := 0
	for _, r := range recentRuns {
		if r.Success {
			recentSuccess++
		}
	}
	recentRate := float64(recentSuccess) / float64(len(recentRuns))

	diff := recentRate - oldRate

	if diff > 0.1 {
		return "improving"
	}
	if diff < -0.2 {
		return "sharply_declining"
	}
	if diff < -0.1 {
		return "declining"
	}
	return "stable"
}

// GetPrediction returns a human-readable prediction string.
func (ra *RiskAnalyzer) GetPrediction(riskScore float64) string {
	if riskScore >= 0.7 {
		return "~80% chance of failure in next 5 runs"
	}
	if riskScore >= 0.4 {
		return "~40% chance of failure in next 10 runs"
	}
	return "Low risk - continue monitoring"
}

// Helper functions

func formatTokenCount(tokens int) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%dM", tokens/1000000)
	}
	return fmt.Sprintf("%dK", tokens/1000)
}

func formatHour(hour int) string {
	if hour == 0 {
		return "midnight"
	}
	if hour < 12 {
		return fmt.Sprintf("%dam", hour)
	}
	if hour == 12 {
		return "noon"
	}
	return fmt.Sprintf("%dpm", hour-12)
}

func categorizeError(err string) string {
	switch {
	case containsString(err, "timeout"):
		return "timeout"
	case containsString(err, "rate limit"):
		return "rate_limit"
	case containsString(err, "auth"):
		return "authentication"
	case containsString(err, "connection"):
		return "connection"
	case containsString(err, "memory"):
		return "memory"
	case containsString(err, "disk"):
		return "disk_space"
	default:
		return "other"
	}
}

func containsString(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
