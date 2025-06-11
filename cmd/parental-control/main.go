package main

import (
	"flag"
	"fmt"
	"os"

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

	logging.Info("Starting Parental Control Service", logging.String("version", Version))

	// Create service configuration
	serviceConfig := service.DefaultConfig()
	if *configPath != "" {
		// TODO: Load configuration from file when configuration management is implemented
		logging.Info("Using config file", logging.String("path", *configPath))
	}

	// Create and start the service
	svc := service.New(serviceConfig)
	
	if err := svc.Start(); err != nil {
		logging.Fatal("Failed to start service", logging.Err(err))
	}

	// Wait for the service to stop (either via signal or error)
	svc.Wait()
	
	logging.Info("Service stopped.")
} 