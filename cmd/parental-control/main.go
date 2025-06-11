package main

import (
	"flag"
	"fmt"
	"os"

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
		port        = flag.Int("port", 8080, "HTTP server port")
		bindLAN     = flag.Bool("bind-lan", true, "Bind to LAN interfaces only")
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

	// Create application configuration
	appConfig := app.DefaultConfig()

	// Override with command line flags
	appConfig.Server.Port = *port
	appConfig.Server.BindToLAN = *bindLAN

	if *configPath != "" {
		// TODO: Load configuration from file when configuration management is implemented
		logging.Info("Using config file", logging.String("path", *configPath))
	}

	// Create and start the application
	application := app.New(appConfig)

	if err := application.Start(); err != nil {
		logging.Fatal("Failed to start application", logging.Err(err))
	}

	// Log the server address for easy access
	if addr := application.GetHTTPAddress(); addr != "" {
		logging.Info("HTTP server available", logging.String("address", fmt.Sprintf("http://%s", addr)))
	}

	// Wait for the application to stop (either via signal or error)
	application.Wait()

	logging.Info("Application stopped.")
}
