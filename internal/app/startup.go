package app

import (
	"context"
	"fmt"
	"time"

	"parental-control/internal/config"
	"parental-control/internal/logging"
	"parental-control/internal/privilege"
	"parental-control/internal/service"
)

// StartupConfig holds options for application startup
type StartupConfig struct {
	ConfigPath    string
	SkipElevation bool
	Version       string
}

// StartupOrchestrator handles the complex application startup sequence
type StartupOrchestrator struct {
	config StartupConfig
	logger *logging.ConcreteLogger
}

// NewStartupOrchestrator creates a new startup orchestrator
func NewStartupOrchestrator(config StartupConfig) *StartupOrchestrator {
	return &StartupOrchestrator{
		config: config,
		logger: logging.NewDefault(),
	}
}

// InitializeApplication handles the complete application initialization
func (so *StartupOrchestrator) InitializeApplication() (*App, *config.Config, error) {
	// Set up logging
	logging.SetGlobalLogger(so.logger)
	so.logger.Info("Starting Parental Control Application", logging.String("version", so.config.Version))

	// Load configuration
	appConfig, err := so.loadConfiguration()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Handle privilege elevation
	if err := so.ensurePrivileges(appConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to obtain required privileges: %w", err)
	}

	// Create application
	application := New(Config{
		Service: service.Config{
			PIDFile:             appConfig.Service.PIDFile,
			ShutdownTimeout:     appConfig.Service.ShutdownTimeout,
			DatabaseConfig:      appConfig.Database,
			HealthCheckInterval: appConfig.Service.HealthCheckInterval,
			EnforcementConfig:   appConfig.Enforcement.ToEnforcementConfig(),
			EnforcementEnabled:  appConfig.Enforcement.Enabled,
			NotificationConfig:  appConfig.Notifications.ToServiceNotificationConfig(),
		},
		Web:      appConfig.Web,
		Security: appConfig.Security,
	})

	return application, appConfig, nil
}

// loadConfiguration loads and validates the application configuration
func (so *StartupOrchestrator) loadConfiguration() (*config.Config, error) {
	appConfig, err := config.LoadFromFile(so.config.ConfigPath)
	if err != nil {
		so.logger.Warn("Could not load config file, using defaults",
			logging.String("path", so.config.ConfigPath),
			logging.Err(err))
		appConfig = config.Default()
	}

	return appConfig, nil
}

// ensurePrivileges handles privilege elevation if needed
func (so *StartupOrchestrator) ensurePrivileges(appConfig *config.Config) error {
	if so.config.SkipElevation || appConfig.Privilege.SkipElevationCheck {
		so.logger.Debug("Skipping privilege elevation")
		return nil
	}

	privConfig := &privilege.Config{
		TimeoutSeconds:     appConfig.Privilege.TimeoutSeconds,
		AllowFallback:      appConfig.Privilege.AllowFallback,
		PreferredElevator:  appConfig.Privilege.PreferredElevator,
		RestartOnElevation: appConfig.Privilege.RestartOnElevation,
	}
	
	// Set elevation method
	switch appConfig.Privilege.ElevationMethod {
	case "uac":
		privConfig.Method = privilege.ElevationMethodUAC
	case "sudo":
		privConfig.Method = privilege.ElevationMethodSudo
	case "pkexec":
		privConfig.Method = privilege.ElevationMethodPkexec
	default:
		privConfig.Method = privilege.ElevationMethodAuto
	}
	
	privManager := privilege.NewManager(privConfig)

	if privManager.IsElevated() {
		so.logger.Info("Application is running with elevated privileges")
		return nil
	}

	if !privManager.CanElevate() {
		return fmt.Errorf("privilege elevation is not available on this system")
	}

	so.logger.Info("Application requires elevated privileges for system enforcement")
	so.logger.Info("Requesting privilege elevation...")

	timeout := time.Duration(appConfig.Privilege.TimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := privManager.RequestElevation(ctx, "Parental Control Application requires administrator privileges to manage network settings and process monitoring")
	if err != nil {
		switch err {
		case privilege.ErrElevationDenied:
			return fmt.Errorf("privilege elevation was denied by user - application cannot function without administrator privileges")
		case privilege.ErrElevationTimeout:
			return fmt.Errorf("privilege elevation request timed out - try increasing the timeout in configuration")
		case privilege.ErrNotSupported:
			return fmt.Errorf("privilege elevation is not supported on this platform")
		default:
			return fmt.Errorf("privilege elevation failed: %w", err)
		}
	}

	return nil
}