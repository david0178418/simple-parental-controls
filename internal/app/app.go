package app

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"parental-control/internal/auth"
	"parental-control/internal/config"
	"parental-control/internal/logging"
	"parental-control/internal/server"
	"parental-control/internal/service"
)

// Config holds the application configuration
type Config struct {
	Service  service.Config
	Web      config.WebConfig
	Security config.SecurityConfig
}

// DefaultConfig returns application configuration with sensible defaults
func DefaultConfig() Config {
	defaultConfig := config.Default()
	return Config{
		Service:  service.DefaultConfig(),
		Web:      defaultConfig.Web,
		Security: defaultConfig.Security,
	}
}

// convertConfigToServerConfig converts app config to server config format
func convertConfigToServerConfig(webConfig config.WebConfig) server.Config {
	// Convert IP addresses from strings to net.IP
	var ipAddresses []net.IP
	ipAddresses = append(ipAddresses, net.IPv4(127, 0, 0, 1))
	ipAddresses = append(ipAddresses, net.IPv6loopback)

	tlsConfig := server.TLSConfig{
		Enabled:       webConfig.TLSEnabled,
		CertFile:      webConfig.TLSCertFile,
		KeyFile:       webConfig.TLSKeyFile,
		AutoGenerate:  webConfig.TLSAutoGenerate,
		CertDir:       webConfig.TLSCertDir,
		Hostname:      webConfig.TLSHostname,
		IPAddresses:   ipAddresses,
		ValidDuration: 365 * 24 * time.Hour, // 1 year
		MinTLSVersion: 0x0303,               // TLS 1.2
		RedirectHTTP:  webConfig.TLSRedirectHTTP,
		HTTPPort:      webConfig.Port,
	}

	return server.Config{
		Port:              webConfig.Port,
		BindToLAN:         true,
		AllowedInterfaces: []string{},
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
		StaticFileRoot:    webConfig.StaticDir,
		EnableCompression: true,
		TLS:               tlsConfig,
	}
}

// SecurityServiceAdapter adapts auth.SecurityService to implement server.AuthService interface
type SecurityServiceAdapter struct {
	securityService *auth.SecurityService
}

// NewSecurityServiceAdapter creates a new adapter
func NewSecurityServiceAdapter(securityService *auth.SecurityService) *SecurityServiceAdapter {
	return &SecurityServiceAdapter{
		securityService: securityService,
	}
}

// ValidateSession validates a session and returns the user
func (a *SecurityServiceAdapter) ValidateSession(sessionID string) (server.AuthUser, error) {
	user, err := a.securityService.ValidateSession(sessionID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetSession retrieves a session by ID
func (a *SecurityServiceAdapter) GetSession(sessionID string) (server.AuthSession, error) {
	session, err := a.securityService.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// App represents the main application
type App struct {
	mu              sync.Mutex
	config          Config
	service         *service.Service
	securityService *auth.SecurityService
	httpServer      *server.Server
}

// New creates a new application instance
func New(config Config) *App {
	return &App{
		config: config,
	}
}

// Start initializes and starts all components of the application
func (a *App) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	logging.Info("Starting application")

	// Initialize security service only if auth is enabled
	if a.config.Security.EnableAuth {
		authConfig := auth.ConvertSecurityConfig(a.config.Security)
		a.securityService = auth.NewSecurityService(authConfig)

		// Create initial admin if enabled
		if err := a.securityService.CreateInitialAdmin("admin", a.config.Security.AdminPassword, "admin@example.com"); err != nil {
			logging.Warn("Failed to create initial admin", logging.Err(err))
		}
	}

	// Initialize service
	a.service = service.New(a.config.Service)
	if err := a.service.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Initialize HTTP server
	serverConfig := convertConfigToServerConfig(a.config.Web)
	a.httpServer = server.New(serverConfig)

	// Initialize API server
	repos := a.service.GetRepositoryManager()
	apiServer := server.NewAPIServer(repos, a.config.Security.EnableAuth)
	apiServer.RegisterRoutes(a.httpServer)

	// Setup static file server for web dashboard
	if err := a.setupStaticFileServer(); err != nil {
		a.service.Stop(ctx)
		return fmt.Errorf("failed to setup static file server: %w", err)
	}

	// Start HTTP server
	if err := a.httpServer.Start(ctx); err != nil {
		a.service.Stop(ctx)
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	logging.Info("Application started successfully")
	return nil
}

// Stop gracefully shuts down all components
func (a *App) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	logging.Info("Stopping application")

	var stopErrors []error

	// Stop HTTP server first
	if a.httpServer != nil {
		if err := a.httpServer.Stop(ctx); err != nil {
			logging.Error("Error stopping HTTP server", logging.Err(err))
			stopErrors = append(stopErrors, err)
		}
	}

	// Stop service
	if a.service != nil {
		if err := a.service.Stop(ctx); err != nil {
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
	a.mu.Lock()
	defer a.mu.Unlock()

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
			"running":       a.httpServer.IsRunning(),
			"address":       a.httpServer.GetAddress(),
			"https_address": a.httpServer.GetHTTPSAddress(),
			"tls_enabled":   a.httpServer.IsTLSEnabled(),
		}

		// Add TLS certificate info if available
		if tlsInfo, err := a.httpServer.GetTLSInfo(); err == nil {
			status["tls"] = tlsInfo
		}
	}

	if a.securityService != nil {
		status["auth"] = map[string]interface{}{
			"enabled": a.config.Security.EnableAuth,
			"stats":   a.securityService.GetSecurityStats(),
		}
	}

	return status
}

// IsRunning returns whether the application is running
func (a *App) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	serviceRunning := a.service != nil && a.service.GetState() == service.StateRunning
	serverRunning := a.httpServer != nil && a.httpServer.IsRunning()

	return serviceRunning && serverRunning
}

// IsHealthy performs a health check
func (a *App) IsHealthy() error {
	a.mu.Lock()
	defer a.mu.Unlock()

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
func (a *App) Restart(ctx context.Context) error {
	logging.Info("Restarting application")

	if err := a.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop application during restart: %w", err)
	}

	// Brief pause before restart
	time.Sleep(1 * time.Second)

	return a.Start(ctx)
}

// GetHTTPAddress returns the HTTP server address
func (a *App) GetHTTPAddress() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.httpServer != nil {
		return a.httpServer.GetAddress()
	}
	return ""
}

// GetSecurityService returns the security service (for admin/debugging purposes)
func (a *App) GetSecurityService() *auth.SecurityService {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.securityService
}

// GetService returns the underlying service instance
func (a *App) GetService() *service.Service {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.service
}

// setupStaticFileServer sets up the static file server for the web dashboard
func (a *App) setupStaticFileServer() error {
	staticRoot := a.config.Web.StaticDir
	if staticRoot == "" {
		staticRoot = "./web/build"
	}

	// Check if static directory exists
	if _, err := os.Stat(staticRoot); os.IsNotExist(err) {
		logging.Warn("Static file directory does not exist",
			logging.String("static_root", staticRoot))
		// Create a simple filesystem that will return 404 for all files
		return a.httpServer.SetupStaticFileServer(os.DirFS("."))
	} else if err != nil {
		return fmt.Errorf("failed to check static directory: %w", err)
	}

	// Create filesystem from the static directory
	fileSystem := os.DirFS(staticRoot)

	// Setup the static file server
	if err := a.httpServer.SetupStaticFileServer(fileSystem); err != nil {
		return fmt.Errorf("failed to configure static file server: %w", err)
	}

	logging.Info("Static file server setup complete",
		logging.String("static_root", staticRoot),
		logging.Bool("web_enabled", a.config.Web.Enabled))

	return nil
}
