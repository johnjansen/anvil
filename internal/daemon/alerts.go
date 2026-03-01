package daemon

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/johnjansen/anvil/internal/config"
	"github.com/johnjansen/anvil/internal/project"
)

// AlertRecord represents an alert that has fired.
type AlertRecord struct {
	ID             string    `json:"id"`
	TaskID         string    `json:"task_id"`
	RuleName       string    `json:"rule_name"`
	Condition      project.AlertCondition `json:"condition"`
	Message        string    `json:"message"`
	Severity       string    `json:"severity"`
	FiredAt        time.Time `json:"fired_at"`
	Acknowledged   bool      `json:"acknowledged"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
}

// alertResult holds the outcome of evaluating alert rules for a task run.
type alertResult struct {
	AlertsFired []AlertRecord
}

// getEffectiveAlerts returns the effective alert config for a task.
// Per-task alerts override global defaults.
func getEffectiveAlerts(todo project.Todo, globalCfg config.AlertGlobalConfig) project.AlertConfig {
	if len(todo.Alerts.Rules) > 0 {
		return todo.Alerts
	}
	// Could add global default support here in the future
	return project.AlertConfig{}
}

// getAlertAction returns the action for a specific rule name.
func getAlertAction(todo project.Todo, ruleName string) project.AlertAction {
	for _, rule := range todo.Alerts.Rules {
		if rule.Name == ruleName {
			return rule.Action
		}
	}
	return project.AlertAction{}
}

// checkAlerts evaluates all alert rules for a completed task run.
// Returns alerts that fired based on the run's metrics.
func checkAlerts(todo project.Todo, runRecord project.RunRecord, globalCfg config.AlertGlobalConfig) alertResult {
	// Check if alerts are globally disabled
	if !globalCfg.Enabled {
		return alertResult{}
	}

	alertConfig := getEffectiveAlerts(todo, globalCfg)
	if len(alertConfig.Rules) == 0 {
		return alertResult{}
	}

	var fired []AlertRecord

	for _, rule := range alertConfig.Rules {
		if evaluateCondition(rule.Condition, runRecord) {
			alert := AlertRecord{
				ID:       generateUUID(),
				TaskID:   todo.Name,
				RuleName: rule.Name,
				Condition: rule.Condition,
				Message:  rule.Message,
				Severity: rule.Severity,
				FiredAt:  time.Now(),
			}
			fired = append(fired, alert)
		}
	}

	return alertResult{AlertsFired: fired}
}

// evaluateCondition checks if an alert condition is met based on the run record.
func evaluateCondition(condition project.AlertCondition, runRecord project.RunRecord) bool {
	switch condition.Type {
	case "cost":
		return evaluateCostCondition(condition.Threshold, runRecord.EstimatedCostUSD)
	case "duration":
		return evaluateDurationCondition(condition.Threshold, runRecord)
	case "output":
		return evaluateOutputCondition(condition.Pattern, runRecord)
	default:
		return false
	}
}

// evaluateCostCondition checks if cost exceeds threshold.
func evaluateCostCondition(thresholdStr string, cost float64) bool {
	var threshold float64
	if _, err := fmt.Sscanf(thresholdStr, "%f", &threshold); err != nil {
		return false
	}
	return cost > threshold
}

// evaluateDurationCondition checks if duration exceeds threshold.
func evaluateDurationCondition(thresholdStr string, runRecord project.RunRecord) bool {
	threshold, err := time.ParseDuration(thresholdStr)
	if err != nil {
		return false
	}
	duration := runRecord.Finished.Sub(runRecord.Started)
	return duration > threshold
}

// evaluateOutputCondition checks if output matches the pattern.
func evaluateOutputCondition(pattern string, runRecord project.RunRecord) bool {
	if pattern == "" {
		return false
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(runRecord.OutputSummary)
}

// generateUUID generates a simple UUID-like string.
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// AlertStorage handles persisting and loading alert records.
type AlertStorage struct {
	BasePath string
}

// NewAlertStorage creates a new AlertStorage.
func NewAlertStorage(basePath string) *AlertStorage {
	return &AlertStorage{BasePath: basePath}
}

// TaskAlertsFile represents the JSON file storing alerts for a task.
type TaskAlertsFile struct {
	TaskID  string        `json:"task_id"`
	Alerts  []AlertRecord `json:"alerts"`
}

// SaveAlert saves an alert record to storage.
func (s *AlertStorage) SaveAlert(taskID string, alert AlertRecord) error {
	taskDir := filepath.Join(s.BasePath, taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return fmt.Errorf("creating alert directory: %w", err)
	}

	alertsFile := filepath.Join(taskDir, "alerts.json")
	var alerts TaskAlertsFile

	// Load existing alerts if file exists
	if data, err := os.ReadFile(alertsFile); err == nil {
		if err := json.Unmarshal(data, &alerts); err != nil {
			// If we can't parse, start fresh
			alerts = TaskAlertsFile{TaskID: taskID}
		}
	} else {
		alerts = TaskAlertsFile{TaskID: taskID}
	}

	alerts.Alerts = append(alerts.Alerts, alert)

	data, err := json.MarshalIndent(alerts, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling alerts: %w", err)
	}

	if err := os.WriteFile(alertsFile, data, 0644); err != nil {
		return fmt.Errorf("writing alerts file: %w", err)
	}

	return nil
}

// LoadAlerts loads all alerts for a task.
func (s *AlertStorage) LoadAlerts(taskID string) ([]AlertRecord, error) {
	alertsFile := filepath.Join(s.BasePath, taskID, "alerts.json")

	data, err := os.ReadFile(alertsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []AlertRecord{}, nil
		}
		return nil, fmt.Errorf("reading alerts file: %w", err)
	}

	var alerts TaskAlertsFile
	if err := json.Unmarshal(data, &alerts); err != nil {
		return nil, fmt.Errorf("parsing alerts file: %w", err)
	}

	return alerts.Alerts, nil
}

// LoadActiveAlerts loads unacknowledged alerts for a task.
func (s *AlertStorage) LoadActiveAlerts(taskID string) ([]AlertRecord, error) {
	all, err := s.LoadAlerts(taskID)
	if err != nil {
		return nil, err
	}

	var active []AlertRecord
	for _, alert := range all {
		if !alert.Acknowledged {
			active = append(active, alert)
		}
	}
	return active, nil
}

// AcknowledgeAlert marks an alert as acknowledged.
func (s *AlertStorage) AcknowledgeAlert(taskID, alertID string) error {
	alertsFile := filepath.Join(s.BasePath, taskID, "alerts.json")

	data, err := os.ReadFile(alertsFile)
	if err != nil {
		return fmt.Errorf("reading alerts file: %w", err)
	}

	var alerts TaskAlertsFile
	if err := json.Unmarshal(data, &alerts); err != nil {
		return fmt.Errorf("parsing alerts file: %w", err)
	}

	now := time.Now()
	found := false
	for i := range alerts.Alerts {
		if alerts.Alerts[i].ID == alertID {
			alerts.Alerts[i].Acknowledged = true
			alerts.Alerts[i].AcknowledgedAt = &now
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	output, err := json.MarshalIndent(alerts, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling alerts: %w", err)
	}

	if err := os.WriteFile(alertsFile, output, 0644); err != nil {
		return fmt.Errorf("writing alerts file: %w", err)
	}

	return nil
}

// LoadAllAlerts loads all alerts across all tasks.
func (s *AlertStorage) LoadAllAlerts() (map[string][]AlertRecord, error) {
	result := make(map[string][]AlertRecord)

	entries, err := os.ReadDir(s.BasePath)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("reading alerts directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		taskID := entry.Name()
		alerts, err := s.LoadAlerts(taskID)
		if err != nil {
			continue // Skip tasks with read errors
		}
		if len(alerts) > 0 {
			result[taskID] = alerts
		}
	}

	return result, nil
}

// ExecuteAlertAction performs the action configured for an alert.
func ExecuteAlertAction(alert AlertRecord, action project.AlertAction, globalWebhook string) {
	webhookURL := action.Webhook
	if webhookURL == "" {
		webhookURL = globalWebhook
	}

	// Execute webhook if configured
	if webhookURL != "" {
		payload := formatAlertPayload(alert)
		deliverWebhook(webhookURL, payload, action.Retry)
	}

	// Execute notify action (placeholder - would integrate with notification system)
	for _, recipient := range action.Notify {
		fmt.Printf("[ALERT] Notifying %s: %s\n", recipient, alert.Message)
	}
}

// formatAlertPayload creates the JSON payload for a webhook.
func formatAlertPayload(alert AlertRecord) string {
	var sb strings.Builder
	sb.WriteString(`{"alert_id":"`)
	sb.WriteString(alert.ID)
	sb.WriteString(`","task_id":"`)
	sb.WriteString(alert.TaskID)
	sb.WriteString(`","rule_name":"`)
	sb.WriteString(alert.RuleName)
	sb.WriteString(`","message":"`)
	sb.WriteString(alert.Message)
	sb.WriteString(`","severity":"`)
	sb.WriteString(alert.Severity)
	sb.WriteString(`","fired_at":"`)
	sb.WriteString(alert.FiredAt.Format(time.RFC3339))
	sb.WriteString(`"}`)
	return sb.String()
}

// deliverWebhook sends the alert payload to a webhook URL with retries.
func deliverWebhook(url, payload string, retries int) {
	// Placeholder for actual HTTP implementation
	// In production, this would use net/http to POST the payload
	fmt.Printf("[ALERT] Webhook would be sent to %s: %s\n", url, payload)
}
