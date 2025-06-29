package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"parental-control/internal/app"
	"parental-control/internal/logging"
)

// Version information - will be injected at build time
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	var (
		showVersion = flag.Bool("version", false, "Show version information")
		configPath  = flag.String("config", "", "Path to configuration file")
		noElevate   = flag.Bool("no-elevate", false, "Skip privilege elevation (for testing)")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Parental Control Service\n")
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		os.Exit(0)
	}

	// Initialize application using startup orchestrator
	startup := app.NewStartupOrchestrator(app.StartupConfig{
		ConfigPath:    *configPath,
		SkipElevation: *noElevate,
		Version:       Version,
	})

	application, appConfig, err := startup.InitializeApplication()
	if err != nil {
		logging.Fatal("Failed to initialize application", logging.Err(err))
	}

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

