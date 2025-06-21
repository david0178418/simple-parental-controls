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
		},
		Web:      appConfig.Web,
		Security: appConfig.Security,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := application.Start(ctx); err != nil {
		logging.Fatal("Failed to start application", logging.Err(err))
	}

	// Initialize and start the enforcement engine
	engineConfig := &enforcement.EnforcementConfig{} // Use an empty config to get defaults
	enforcementEngine := enforcement.NewEnforcementEngine(engineConfig, logger, nil)
	if err := enforcementEngine.Start(ctx); err != nil {
		logging.Error("Failed to start enforcement engine, blocking is not active", logging.Err(err))
	} else {
		logging.Info("Enforcement engine started")
		// Load rules into the engine
		loadRulesIntoEngine(ctx, application.GetService(), enforcementEngine)
	}

	// Wait for shutdown signal
	<-ctx.Done()

	logging.Info("Shutting down application...")

	// Create a shutdown context with a timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), appConfig.Service.ShutdownTimeout)
	defer cancel()

	// Stop enforcement engine first
	if enforcementEngine.IsRunning() {
		if err := enforcementEngine.Stop(shutdownCtx); err != nil {
			logging.Error("Error stopping enforcement engine", logging.Err(err))
		} else {
			logging.Info("Enforcement engine stopped")
		}
	}

	// Stop the main application
	if err := application.Stop(shutdownCtx); err != nil {
		logging.Error("Error during application shutdown", logging.Err(err))
	}

	logging.Info("Application stopped.")
}

func loadRulesIntoEngine(ctx context.Context, appService *service.Service, engine *enforcement.EnforcementEngine) {
	if appService == nil {
		logging.Error("Cannot load rules, service is not available")
		return
	}

	repos := appService.GetRepositoryManager()
	if repos == nil {
		logging.Error("Cannot load rules, repository manager is not available")
		return
	}

	lists, err := repos.List.GetAll(ctx)
	if err != nil {
		logging.Error("Failed to get lists for rule loading", logging.Err(err))
		return
	}

	logging.Info("Loading rules into enforcement engine...", logging.Int("list_count", len(lists)))
	ruleCount := 0

	for _, list := range lists {
		if !list.Enabled {
			continue
		}

		entries, err := repos.ListEntry.GetByListID(ctx, list.ID)
		if err != nil {
			logging.Error("Failed to get entries for list",
				logging.Err(err),
				logging.Int("list_id", list.ID))
			continue
		}

		for _, entry := range entries {
			if !entry.Enabled {
				continue
			}

			rule := &enforcement.FilterRule{
				ID:        fmt.Sprintf("entry-%d", entry.ID),
				Name:      fmt.Sprintf("%s - %s", list.Name, entry.Pattern),
				Pattern:   entry.Pattern,
				MatchType: enforcement.MatchType(entry.PatternType),
				Priority:  100, // You might want a more sophisticated priority system
				Enabled:   true,
			}

			// The enforcement engine expects "block" or "allow", but lists are "blacklist" or "whitelist"
			if list.Type == "blacklist" {
				rule.Action = enforcement.ActionBlock
			} else {
				rule.Action = enforcement.ActionAllow
			}

			if err := engine.AddNetworkRule(rule); err != nil {
				logging.Error("Failed to add network rule to engine",
					logging.Err(err),
					logging.String("rule_id", rule.ID))
			} else {
				ruleCount++
			}
		}
	}

	logging.Info("Finished loading rules into enforcement engine", logging.Int("rules_loaded", ruleCount))
}
