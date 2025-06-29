package config

import (
	"parental-control/internal/enforcement"
	"parental-control/internal/service"
)

// ToEnforcementConfig converts config.EnforcementConfig to enforcement.EnforcementConfig
func (cfg EnforcementConfig) ToEnforcementConfig() enforcement.EnforcementConfig {
	return enforcement.EnforcementConfig{
		ProcessPollInterval:    cfg.ProcessPollInterval,
		EnableNetworkFiltering: cfg.EnableNetworkFiltering,
		MaxConcurrentChecks:    cfg.MaxConcurrentChecks,
		CacheTimeout:           cfg.CacheTimeout,
		BlockUnknownProcesses:  cfg.BlockUnknownProcesses,
		LogAllActivity:         cfg.LogAllActivity,
		EnableEmergencyMode:    cfg.EnableEmergencyMode,
		EmergencyWhitelist:     cfg.EmergencyWhitelist,
	}
}

// ToServiceNotificationConfig converts config.NotificationConfig to service.NotificationConfig
func (cfg NotificationConfig) ToServiceNotificationConfig() service.NotificationConfig {
	return service.NotificationConfig{
		Enabled:                   cfg.Enabled,
		AppName:                   cfg.AppName,
		AppIcon:                   cfg.AppIcon,
		MaxNotificationsPerMinute: cfg.MaxNotificationsPerMinute,
		CooldownPeriod:            cfg.CooldownPeriod,
		EnableAppBlocking:         cfg.EnableAppBlocking,
		EnableWebBlocking:         cfg.EnableWebBlocking,
		EnableTimeLimit:           cfg.EnableTimeLimit,
		EnableSystemAlerts:        cfg.EnableSystemAlerts,
		ShowProcessDetails:        cfg.ShowProcessDetails,
		NotificationTimeout:       cfg.NotificationTimeout,
	}
}