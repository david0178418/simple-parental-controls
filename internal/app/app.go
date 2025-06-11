package app

import (
	"fmt"
	"sync"
	"time"

	"parental-control/internal/logging"
	"parental-control/internal/server"
	"parental-control/internal/service"
)

// Config holds the application configuration
type Config struct {
	Service service.Config
	Server  server.Config
}

// DefaultConfig returns application configuration with sensible defaults
func DefaultConfig() Config {
	return Config{
		Service: service.DefaultConfig(),
		Server:  server.DefaultConfig(),
	}
}

// App coordinates the service and HTTP server
type App struct {
	config     Config
	service    *service.Service
	httpServer *server.Server
	apiServer  *server.SimpleAPIServer
	mu         sync.RWMutex
}

// New creates a new application instance
func New(config Config) *App {
	return &App{
		config: config,
	}
}

// Start initializes and starts all components
func (a *App) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	logging.Info("Starting application")

	// Initialize service
	a.service = service.New(a.config.Service)
	if err := a.service.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Initialize HTTP server
	a.httpServer = server.New(a.config.Server)

	// Initialize API server
	a.apiServer = server.NewSimpleAPIServer()
	a.apiServer.RegisterRoutes(a.httpServer)

	// Start HTTP server
	if err := a.httpServer.Start(); err != nil {
		a.service.Stop()
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	logging.Info("Application started successfully",
		logging.String("http_address", a.httpServer.GetAddress()))

	return nil
}

// Stop gracefully shuts down all components
func (a *App) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	logging.Info("Stopping application")

	var stopErrors []error

	// Stop HTTP server first
	if a.httpServer != nil {
		if err := a.httpServer.Stop(); err != nil {
			logging.Error("Error stopping HTTP server", logging.Err(err))
			stopErrors = append(stopErrors, err)
		}
	}

	// Stop service
	if a.service != nil {
		if err := a.service.Stop(); err != nil {
			logging.Error("Error stopping service", logging.Err(err))
			stopErrors = append(stopErrors, err)
		}
	}

	if len(stopErrors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", stopErrors)
	}

	logging.Info("Application stopped successfully")
	return nil
}

// Wait blocks until the service stops
func (a *App) Wait() {
	if a.service != nil {
		a.service.Wait()
	}
}

// GetStatus returns the application status
func (a *App) GetStatus() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	status := map[string]interface{}{
		"app": map[string]interface{}{
			"running": a.IsRunning(),
		},
	}

	if a.service != nil {
		status["service"] = a.service.GetStatus()
	}

	if a.httpServer != nil {
		status["http_server"] = map[string]interface{}{
			"running": a.httpServer.IsRunning(),
			"address": a.httpServer.GetAddress(),
		}
	}

	return status
}

// IsRunning returns whether the application is running
func (a *App) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	serviceRunning := a.service != nil && a.service.GetState() == service.StateRunning
	serverRunning := a.httpServer != nil && a.httpServer.IsRunning()

	return serviceRunning && serverRunning
}

// IsHealthy performs a health check
func (a *App) IsHealthy() error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.service != nil {
		if err := a.service.IsHealthy(); err != nil {
			return fmt.Errorf("service health check failed: %w", err)
		}
	}

	if a.httpServer != nil && !a.httpServer.IsRunning() {
		return fmt.Errorf("HTTP server is not running")
	}

	return nil
}

// Restart restarts the entire application
func (a *App) Restart() error {
	logging.Info("Restarting application")

	if err := a.Stop(); err != nil {
		return fmt.Errorf("failed to stop application during restart: %w", err)
	}

	// Brief pause before restart
	time.Sleep(1 * time.Second)

	return a.Start()
}

// GetHTTPAddress returns the HTTP server address
func (a *App) GetHTTPAddress() string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.httpServer != nil {
		return a.httpServer.GetAddress()
	}
	return ""
}
