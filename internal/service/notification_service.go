package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"sync"
	"time"

	"github.com/gen2brain/beeep"
	"parental-control/internal/enforcement"
	"parental-control/internal/logging"
	"parental-control/internal/models"
)

// NotificationService provides cross-platform desktop notification capabilities
type NotificationService struct {
	config *NotificationConfig
	logger logging.Logger
	
	// State management
	enabled   bool
	enabledMu sync.RWMutex
	
	// Rate limiting to prevent spam
	rateLimiter *NotificationRateLimiter
	
	// Statistics
	stats   *NotificationStats
	statsMu sync.RWMutex

	// Audit logging (optional)
	auditService enforcement.AuditLogger
}

// NotificationConfig holds configuration for the notification service
type NotificationConfig struct {
	// Enable notifications
	Enabled bool `json:"enabled" yaml:"enabled"`
	
	// App branding
	AppName string `json:"app_name" yaml:"app_name"`
	AppIcon string `json:"app_icon" yaml:"app_icon"`
	
	// Rate limiting
	MaxNotificationsPerMinute int           `json:"max_notifications_per_minute" yaml:"max_notifications_per_minute"`
	CooldownPeriod            time.Duration `json:"cooldown_period" yaml:"cooldown_period"`
	
	// Notification types to enable
	EnableAppBlocking   bool `json:"enable_app_blocking" yaml:"enable_app_blocking"`
	EnableWebBlocking   bool `json:"enable_web_blocking" yaml:"enable_web_blocking"`
	EnableTimeLimit     bool `json:"enable_time_limit" yaml:"enable_time_limit"`
	EnableSystemAlerts  bool `json:"enable_system_alerts" yaml:"enable_system_alerts"`
	
	// Notification behavior
	ShowProcessDetails bool          `json:"show_process_details" yaml:"show_process_details"`
	NotificationTimeout time.Duration `json:"notification_timeout" yaml:"notification_timeout"`
}

// NotificationStats tracks notification statistics
type NotificationStats struct {
	TotalSent           int64     `json:"total_sent"`
	AppBlockingSent     int64     `json:"app_blocking_sent"`
	WebBlockingSent     int64     `json:"web_blocking_sent"`
	TimeLimitSent       int64     `json:"time_limit_sent"`
	SystemAlertsSent    int64     `json:"system_alerts_sent"`
	RateLimited         int64     `json:"rate_limited"`
	Errors              int64     `json:"errors"`
	LastNotificationTime time.Time `json:"last_notification_time"`
	LastError           string    `json:"last_error,omitempty"`
	LastErrorTime       time.Time `json:"last_error_time,omitempty"`
}

// NotificationRateLimiter implements simple rate limiting for notifications
type NotificationRateLimiter struct {
	maxPerMinute    int
	cooldownPeriod  time.Duration
	notifications   []time.Time
	lastCooldown    map[string]time.Time
	mu              sync.Mutex
}

// NotificationType represents different types of notifications
type NotificationType string

const (
	NotificationTypeAppBlocked    NotificationType = "app_blocked"
	NotificationTypeWebBlocked    NotificationType = "web_blocked"
	NotificationTypeTimeLimit     NotificationType = "time_limit"
	NotificationTypeSystemAlert   NotificationType = "system_alert"
)

// NotificationData contains information for creating a notification
type NotificationData struct {
	Type        NotificationType      `json:"type"`
	Title       string                `json:"title"`
	Message     string                `json:"message"`
	Icon        string                `json:"icon,omitempty"`
	ProcessName string                `json:"process_name,omitempty"`
	ProcessPID  int                   `json:"process_pid,omitempty"`
	URL         string                `json:"url,omitempty"`
	RuleName    string                `json:"rule_name,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// NewNotificationService creates a new notification service
func NewNotificationService(config *NotificationConfig, logger logging.Logger) *NotificationService {
	return NewNotificationServiceWithAudit(config, logger, nil)
}

// NewNotificationServiceWithAudit creates a new notification service with audit logging
func NewNotificationServiceWithAudit(config *NotificationConfig, logger logging.Logger, auditService enforcement.AuditLogger) *NotificationService {
	if config == nil {
		config = DefaultNotificationConfig()
	}
	
	// Set app name for beeep library
	if config.AppName != "" {
		beeep.AppName = config.AppName
	}
	
	rateLimiter := &NotificationRateLimiter{
		maxPerMinute:   config.MaxNotificationsPerMinute,
		cooldownPeriod: config.CooldownPeriod,
		notifications:  make([]time.Time, 0),
		lastCooldown:   make(map[string]time.Time),
	}
	
	return &NotificationService{
		config:       config,
		logger:       logger,
		enabled:      config.Enabled,
		rateLimiter:  rateLimiter,
		stats:        &NotificationStats{},
		auditService: auditService,
	}
}

// DefaultNotificationConfig returns sensible defaults for notification configuration
func DefaultNotificationConfig() *NotificationConfig {
	return &NotificationConfig{
		Enabled:                   true,
		AppName:                   "Parental Control",
		AppIcon:                   "", // Use default system icon
		MaxNotificationsPerMinute: 10,
		CooldownPeriod:            30 * time.Second,
		EnableAppBlocking:         true,
		EnableWebBlocking:         true,
		EnableTimeLimit:           true,
		EnableSystemAlerts:        false,
		ShowProcessDetails:        true,
		NotificationTimeout:       5 * time.Second,
	}
}

// IsEnabled returns whether notifications are currently enabled
func (ns *NotificationService) IsEnabled() bool {
	ns.enabledMu.RLock()
	defer ns.enabledMu.RUnlock()
	return ns.enabled
}

// SetEnabled enables or disables notifications
func (ns *NotificationService) SetEnabled(enabled bool) {
	ns.enabledMu.Lock()
	defer ns.enabledMu.Unlock()
	ns.enabled = enabled
	
	ns.logger.Info("Notification service state changed",
		logging.Bool("enabled", enabled))
}

// NotifyAppBlocked sends a notification when an application is blocked
func (ns *NotificationService) NotifyAppBlocked(ctx context.Context, processName string, pid int, ruleName string) error {
	ns.logger.Info("NotifyAppBlocked called",
		logging.String("process", processName),
		logging.Int("pid", pid),
		logging.String("rule", ruleName),
		logging.Bool("enabled", ns.IsEnabled()),
		logging.Bool("app_blocking_enabled", ns.config.EnableAppBlocking))

	if !ns.IsEnabled() || !ns.config.EnableAppBlocking {
		ns.logger.Info("App blocking notification skipped - disabled")
		return nil
	}
	
	title := "Application Blocked"
	message := fmt.Sprintf("The application '%s' has been blocked by parental controls.", processName)
	
	if ns.config.ShowProcessDetails && pid > 0 {
		message = fmt.Sprintf("The application '%s' (PID: %d) has been blocked by parental controls.", processName, pid)
	}
	
	if ruleName != "" {
		message += fmt.Sprintf(" Rule: %s", ruleName)
	}
	
	data := &NotificationData{
		Type:        NotificationTypeAppBlocked,
		Title:       title,
		Message:     message,
		Icon:        ns.config.AppIcon,
		ProcessName: processName,
		ProcessPID:  pid,
		RuleName:    ruleName,
	}
	
	ns.logger.Info("Calling sendNotification",
		logging.String("title", title),
		logging.String("message", message))
	
	return ns.sendNotification(ctx, data)
}

// NotifyWebBlocked sends a notification when a website is blocked
func (ns *NotificationService) NotifyWebBlocked(ctx context.Context, url string, processName string, ruleName string) error {
	if !ns.IsEnabled() || !ns.config.EnableWebBlocking {
		return nil
	}
	
	title := "Website Blocked"
	message := fmt.Sprintf("Access to '%s' has been blocked by parental controls.", url)
	
	if processName != "" {
		message += fmt.Sprintf(" Application: %s", processName)
	}
	
	if ruleName != "" {
		message += fmt.Sprintf(" Rule: %s", ruleName)
	}
	
	data := &NotificationData{
		Type:        NotificationTypeWebBlocked,
		Title:       title,
		Message:     message,
		Icon:        ns.config.AppIcon,
		ProcessName: processName,
		URL:         url,
		RuleName:    ruleName,
	}
	
	return ns.sendNotification(ctx, data)
}

// NotifyTimeLimit sends a notification about time limits
func (ns *NotificationService) NotifyTimeLimit(ctx context.Context, message string, details map[string]interface{}) error {
	if !ns.IsEnabled() || !ns.config.EnableTimeLimit {
		return nil
	}
	
	title := "Time Limit"
	
	data := &NotificationData{
		Type:    NotificationTypeTimeLimit,
		Title:   title,
		Message: message,
		Icon:    ns.config.AppIcon,
		Details: details,
	}
	
	return ns.sendNotification(ctx, data)
}

// NotifySystemAlert sends a system alert notification
func (ns *NotificationService) NotifySystemAlert(ctx context.Context, title string, message string, details map[string]interface{}) error {
	if !ns.IsEnabled() || !ns.config.EnableSystemAlerts {
		return nil
	}
	
	data := &NotificationData{
		Type:    NotificationTypeSystemAlert,
		Title:   title,
		Message: message,
		Icon:    ns.config.AppIcon,
		Details: details,
	}
	
	return ns.sendNotification(ctx, data)
}

// sendNotification sends a notification to the desktop
func (ns *NotificationService) sendNotification(ctx context.Context, data *NotificationData) error {
	// Check rate limiting
	if !ns.rateLimiter.Allow(string(data.Type)) {
		ns.incrementRateLimited()
		ns.logger.Debug("Notification rate limited",
			logging.String("type", string(data.Type)),
			logging.String("title", data.Title))
		
		// Log rate limiting to audit
		if ns.auditService != nil {
			details := map[string]interface{}{
				"notification_type": string(data.Type),
				"title":             data.Title,
				"process_name":      data.ProcessName,
				"reason":            "rate_limited",
			}
			if err := ns.auditService.LogEnforcementAction(
				ctx,
				models.ActionTypeBlock,
				models.TargetTypeExecutable,
				data.ProcessName,
				"notification_service",
				nil,
				details,
			); err != nil {
				ns.logger.Error("Failed to log notification rate limiting", logging.Err(err))
			}
		}
		
		return nil // Not an error, just rate limited
	}
	
	// Send the notification using beeep
	icon := data.Icon
	if icon == "" {
		icon = ns.config.AppIcon
	}
	
	err := ns.sendNotificationAsUser(data.Title, data.Message, icon)
	if err != nil {
		ns.incrementError(err)
		ns.logger.Error("Failed to send notification",
			logging.Err(err),
			logging.String("type", string(data.Type)),
			logging.String("title", data.Title))
		
		// Log notification failure to audit
		if ns.auditService != nil {
			details := map[string]interface{}{
				"notification_type": string(data.Type),
				"title":             data.Title,
				"message":           data.Message,
				"process_name":      data.ProcessName,
				"error":             err.Error(),
				"reason":            "notification_failed",
			}
			if auditErr := ns.auditService.LogEnforcementAction(
				ctx,
				models.ActionTypeAllow,
				models.TargetTypeExecutable,
				data.ProcessName,
				"notification_service",
				nil,
				details,
			); auditErr != nil {
				ns.logger.Error("Failed to log notification failure", logging.Err(auditErr))
			}
		}
		
		return fmt.Errorf("failed to send notification: %w", err)
	}
	
	// Update statistics
	ns.incrementNotificationSent(data.Type)
	
	// Log successful notification to audit
	if ns.auditService != nil {
		details := map[string]interface{}{
			"notification_type": string(data.Type),
			"title":             data.Title,
			"message":           data.Message,
			"process_name":      data.ProcessName,
			"process_pid":       data.ProcessPID,
			"url":               data.URL,
			"rule_name":         data.RuleName,
		}
		if data.Details != nil {
			for k, v := range data.Details {
				details[k] = v
			}
		}
		
		if err := ns.auditService.LogEnforcementAction(
			ctx,
			models.ActionTypeAllow,
			models.TargetTypeExecutable,
			data.ProcessName,
			"notification_service",
			nil,
			details,
		); err != nil {
			ns.logger.Error("Failed to log notification success", logging.Err(err))
		}
	}
	
	ns.logger.Debug("Notification sent successfully",
		logging.String("type", string(data.Type)),
		logging.String("title", data.Title),
		logging.String("process", data.ProcessName))
	
	return nil
}

// GetStats returns current notification statistics
func (ns *NotificationService) GetStats() *NotificationStats {
	ns.statsMu.RLock()
	defer ns.statsMu.RUnlock()
	
	// Return a copy to prevent race conditions
	stats := *ns.stats
	return &stats
}

// GetConfig returns the current notification configuration
func (ns *NotificationService) GetConfig() *NotificationConfig {
	return ns.config
}

// UpdateConfig updates the notification configuration
func (ns *NotificationService) UpdateConfig(config *NotificationConfig) {
	ns.config = config
	ns.SetEnabled(config.Enabled)
	
	// Update app name for beeep
	if config.AppName != "" {
		beeep.AppName = config.AppName
	}
	
	// Update rate limiter
	ns.rateLimiter.maxPerMinute = config.MaxNotificationsPerMinute
	ns.rateLimiter.cooldownPeriod = config.CooldownPeriod
	
	ns.logger.Info("Notification configuration updated")
}

// Allow checks if a notification of the given type is allowed by rate limiting
func (rl *NotificationRateLimiter) Allow(notificationType string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	
	// Check cooldown period for this specific notification type
	if lastTime, exists := rl.lastCooldown[notificationType]; exists {
		if now.Sub(lastTime) < rl.cooldownPeriod {
			return false
		}
	}
	
	// Clean up old notifications (older than 1 minute)
	cutoff := now.Add(-time.Minute)
	var recent []time.Time
	for _, notifTime := range rl.notifications {
		if notifTime.After(cutoff) {
			recent = append(recent, notifTime)
		}
	}
	rl.notifications = recent
	
	// Check if we're under the rate limit
	if len(rl.notifications) >= rl.maxPerMinute {
		return false
	}
	
	// Allow the notification
	rl.notifications = append(rl.notifications, now)
	rl.lastCooldown[notificationType] = now
	
	return true
}

// incrementNotificationSent increments the appropriate statistics counter
func (ns *NotificationService) incrementNotificationSent(notificationType NotificationType) {
	ns.statsMu.Lock()
	defer ns.statsMu.Unlock()
	
	ns.stats.TotalSent++
	ns.stats.LastNotificationTime = time.Now()
	
	switch notificationType {
	case NotificationTypeAppBlocked:
		ns.stats.AppBlockingSent++
	case NotificationTypeWebBlocked:
		ns.stats.WebBlockingSent++
	case NotificationTypeTimeLimit:
		ns.stats.TimeLimitSent++
	case NotificationTypeSystemAlert:
		ns.stats.SystemAlertsSent++
	}
}

// incrementRateLimited increments the rate limited counter
func (ns *NotificationService) incrementRateLimited() {
	ns.statsMu.Lock()
	defer ns.statsMu.Unlock()
	ns.stats.RateLimited++
}

// incrementError increments the error counter and updates error info
func (ns *NotificationService) incrementError(err error) {
	ns.statsMu.Lock()
	defer ns.statsMu.Unlock()
	
	ns.stats.Errors++
	ns.stats.LastError = err.Error()
	ns.stats.LastErrorTime = time.Now()
}

// sendNotificationAsUser sends notification in user context when running as root
func (ns *NotificationService) sendNotificationAsUser(title, message, icon string) error {
	currentUID := os.Getuid()
	ns.logger.Info("Attempting to send notification",
		logging.String("title", title),
		logging.Int("current_uid", currentUID),
		logging.String("sudo_user", os.Getenv("SUDO_USER")))

	// Skip beeep when running as root since it typically fails and hangs
	if currentUID == 0 {
		ns.logger.Info("Running as root, skipping beeep and using sudo notification")
		return ns.sendNotificationViaSudo(title, message, icon)
	}

	// First try the normal method (works when not running as root)
	ns.logger.Info("Trying beeep notification")
	err := beeep.Notify(title, message, icon)
	if err == nil {
		ns.logger.Info("Notification sent via beeep successfully")
		return nil
	}

	ns.logger.Info("Beeep notification failed", logging.Err(err))
	return err
}

// sendNotificationViaSudo sends notification via sudo to the original user
func (ns *NotificationService) sendNotificationViaSudo(title, message, icon string) error {
	// Get the original user from SUDO_USER environment variable
	sudoUser := os.Getenv("SUDO_USER")
	ns.logger.Info("Attempting sudo notification", logging.String("sudo_user", sudoUser))

	if sudoUser == "" {
		// Try to find the first non-root user logged in
		if u, err := ns.findLoggedInUser(); err == nil {
			sudoUser = u.Username
			ns.logger.Info("Found logged in user", logging.String("user", sudoUser))
		} else {
			ns.logger.Error("Cannot determine original user for notification", logging.Err(err))
			return fmt.Errorf("cannot determine original user for notification")
		}
	}

	// Get user info
	u, err := user.Lookup(sudoUser)
	if err != nil {
		ns.logger.Error("Failed to lookup user", logging.String("user", sudoUser), logging.Err(err))
		return fmt.Errorf("failed to lookup user %s: %w", sudoUser, err)
	}

	ns.logger.Info("User lookup successful",
		logging.String("username", u.Username),
		logging.String("home_dir", u.HomeDir),
		logging.String("uid", u.Uid))

	// Try multiple notification methods
	methods := []struct {
		name string
		cmd  []string
	}{
		{"notify-send", []string{"notify-send", "--app-name=" + ns.config.AppName, "--urgency=normal", title, message}},
		{"zenity", []string{"zenity", "--info", "--title=" + title, "--text=" + message, "--timeout=5"}},
		{"xmessage", []string{"xmessage", "-center", "-timeout", "5", title + ": " + message}},
	}

	for _, method := range methods {
		ns.logger.Info("Trying notification method", logging.String("method", method.name))
		
		// Set a timeout for the notification command
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		
		args := append([]string{"-u", sudoUser}, method.cmd...)
		cmd := exec.CommandContext(timeoutCtx, "sudo", args...)
		
		// Set environment for the user with X11 authorization
		xauthFile := u.HomeDir + "/.Xauthority"
		cmd.Env = []string{
			"HOME=" + u.HomeDir,
			"USER=" + u.Username,
			"DISPLAY=:0",
			"XDG_RUNTIME_DIR=/run/user/" + u.Uid,
			"XAUTHORITY=" + xauthFile,
		}
		
		output, err := cmd.CombinedOutput()
		cancel()
		
		if err == nil {
			ns.logger.Info("Notification sent successfully", 
				logging.String("method", method.name),
				logging.String("output", string(output)))
			return nil
		}
		
		ns.logger.Info("Notification method failed, trying next",
			logging.String("method", method.name),
			logging.Err(err),
			logging.String("output", string(output)))
	}

	// Last resort: log to system and try a simple echo to the user's terminal
	ns.logger.Info("All GUI notification methods failed, trying console notification")
	
	// Try to write to the user's terminal sessions
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	// Try to send a wall message to all terminals
	wallCmd := exec.CommandContext(timeoutCtx, "sudo", "-u", sudoUser, "sh", "-c", 
		fmt.Sprintf("echo '%s: %s' | wall 2>/dev/null || echo '%s: %s' > /dev/console 2>/dev/null || true", 
			title, message, title, message))
	
	output, err := wallCmd.CombinedOutput()
	if err == nil {
		ns.logger.Info("Console notification sent successfully", logging.String("output", string(output)))
		return nil
	}
	
	ns.logger.Info("Console notification also failed", logging.Err(err))
	return fmt.Errorf("all notification methods failed")
}

// findLoggedInUser attempts to find a logged-in user
func (ns *NotificationService) findLoggedInUser() (*user.User, error) {
	// Try to find users with active sessions in /run/user/
	entries, err := os.ReadDir("/run/user")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if uid, err := strconv.Atoi(entry.Name()); err == nil && uid >= 1000 {
				if u, err := user.LookupId(entry.Name()); err == nil {
					return u, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no logged in user found")
}