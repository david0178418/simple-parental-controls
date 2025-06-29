package service

import (
	"context"
	"fmt"
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
	if !ns.IsEnabled() || !ns.config.EnableAppBlocking {
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
	
	err := beeep.Notify(data.Title, data.Message, icon)
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