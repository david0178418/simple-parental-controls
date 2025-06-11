package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logging.Info("Received shutdown signal, gracefully shutting down...")
		cancel()
	}()

	// TODO: Initialize and start services
	// This will be implemented in later tasks
	if *configPath != "" {
		logging.Info("Using config file", logging.String("path", *configPath))
	}

	logging.Info("Service started. Press Ctrl+C to shutdown.")
	
	// Wait for shutdown signal
	<-ctx.Done()
	logging.Info("Service stopped.")
} 