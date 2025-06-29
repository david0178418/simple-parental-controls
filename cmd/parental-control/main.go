package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"parental-control/internal/app"
	"parental-control/internal/config"
	"parental-control/internal/enforcement"
	"parental-control/internal/logging"
	"parental-control/internal/service"
)

// Version information - will be injected at build time
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	if os.Geteuid() != 0 {
		fmt.Println("This application must be run as root to manage network settings.")
		fmt.Println("Please try again using 'sudo'.")
		os.Exit(1)
	}

	var (
		showVersion = flag.Bool("version", false, "Show version information")
		configPath  = flag.String("config", "", "Path to configuration file")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Parental Control Service\n")
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		os.Exit(0)
	}

	// Initialize logging
	logger := logging.NewDefault()
	logging.SetGlobalLogger(logger)

	logging.Info("Starting Parental Control Application", logging.String("version", Version))

	// Load configuration
	appConfig, err := config.LoadFromFile(*configPath)
	if err != nil {
		logging.Warn("Could not load config file, using defaults",
			logging.String("path", *configPath),
			logging.Err(err))
		appConfig = config.Default()
	}

	// Create and start the main application
	application := app.New(app.Config{
		Service: service.Config{
			PIDFile:             appConfig.Service.PIDFile,
			ShutdownTimeout:     appConfig.Service.ShutdownTimeout,
			DatabaseConfig:      appConfig.Database,
			HealthCheckInterval: appConfig.Service.HealthCheckInterval,
			EnforcementConfig:   convertToServiceEnforcementConfig(appConfig.Enforcement),
			EnforcementEnabled:  appConfig.Enforcement.Enabled,
			NotificationConfig:  convertToServiceNotificationConfig(appConfig.Notifications),
		},
		Web:      appConfig.Web,
		Security: appConfig.Security,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := application.Start(ctx); err != nil {
		logging.Fatal("Failed to start application", logging.Err(err))
	}

	// Wait for shutdown signal - enforcement is now handled by the service layer
	<-ctx.Done()

	logging.Info("Shutting down application...")

	// Create a shutdown context with a timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), appConfig.Service.ShutdownTimeout)
	defer cancel()

	// Stop the main application (which includes the enforcement service)
	if err := application.Stop(shutdownCtx); err != nil {
		logging.Error("Error during application shutdown", logging.Err(err))
	}

	logging.Info("Application stopped.")
}

// convertToServiceEnforcementConfig converts config.EnforcementConfig to enforcement.EnforcementConfig for service layer
func convertToServiceEnforcementConfig(cfg config.EnforcementConfig) enforcement.EnforcementConfig {
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

// convertToServiceNotificationConfig converts config.NotificationConfig to service.NotificationConfig
func convertToServiceNotificationConfig(cfg config.NotificationConfig) service.NotificationConfig {
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
