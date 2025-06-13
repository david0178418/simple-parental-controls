package main

import (
	"flag"
	"fmt"
	"os"

	"parental-control/internal/app"
	"parental-control/internal/config"
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
		port        = flag.Int("port", 8080, "HTTP server port")
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
	var appConfig app.Config

	if *configPath != "" {
		// Load configuration from file
		logging.Info("Loading config file", logging.String("path", *configPath))
		fullConfig, err := config.LoadFromFile(*configPath)
		if err != nil {
			logging.Fatal("Failed to load configuration file", logging.Err(err))
		}
		appConfig = app.Config{
			Service:  service.DefaultConfig(),
			Web:      fullConfig.Web,
			Security: fullConfig.Security,
		}
	} else {
		// Load configuration from environment variables
		fullConfig, err := config.LoadFromEnvironment()
		if err != nil {
			logging.Fatal("Failed to load configuration from environment", logging.Err(err))
		}
		appConfig = app.Config{
			Service:  service.DefaultConfig(),
			Web:      fullConfig.Web,
			Security: fullConfig.Security,
		}
	}

	// Override with command line flags
	appConfig.Web.Port = *port

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
