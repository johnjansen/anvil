package daemon

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/johnjansen/anvil/internal/config"
)

// sendNotification sends a desktop notification using platform-native APIs.
// If a custom command is configured, it is used instead.
func sendNotification(cfg config.NotificationsConfig, title, message string) {
	if !cfg.Enabled {
		return
	}

	if cfg.Command != "" {
		// Custom command: replace {title} and {message} placeholders
		cmd := cfg.Command
		cmd = strings.ReplaceAll(cmd, "{title}", title)
		cmd = strings.ReplaceAll(cmd, "{message}", message)
		_ = exec.Command("sh", "-c", cmd).Run()
		return
	}

	switch runtime.GOOS {
	case "darwin":
		// macOS: use osascript for native notification
		script := fmt.Sprintf(`display notification %q with title %q`, message, title)
		_ = exec.Command("osascript", "-e", script).Run()
	case "linux":
		// Linux: use notify-send
		_ = exec.Command("notify-send", title, message).Run()
	case "windows":
		// Windows: use PowerShell toast notification
		ps := fmt.Sprintf(`[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null; `+
			`$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02); `+
			`$textNodes = $template.GetElementsByTagName('text'); `+
			`$textNodes.Item(0).AppendChild($template.CreateTextNode('%s')) | Out-Null; `+
			`$textNodes.Item(1).AppendChild($template.CreateTextNode('%s')) | Out-Null; `+
			`$toast = [Windows.UI.Notifications.ToastNotification]::new($template); `+
			`[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('anvil').Show($toast)`,
			strings.ReplaceAll(title, "'", "''"),
			strings.ReplaceAll(message, "'", "''"))
		_ = exec.Command("powershell", "-Command", ps).Run()
	}
}

// shouldNotifyFailure returns true if a failure notification should be sent for a task.
func shouldNotifyFailure(cfg config.NotificationsConfig, taskNotifyOverride *bool) bool {
	if !cfg.Enabled {
		return false
	}
	if taskNotifyOverride != nil {
		return *taskNotifyOverride
	}
	return cfg.OnFailure
}

// shouldNotifySuccess returns true if a success notification should be sent for a task.
func shouldNotifySuccess(cfg config.NotificationsConfig, taskNotifyOverride *bool) bool {
	if !cfg.Enabled {
		return false
	}
	if taskNotifyOverride != nil {
		return *taskNotifyOverride
	}
	return cfg.OnSuccess
}
