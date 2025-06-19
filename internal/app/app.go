package app

import (
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

// App coordinates the service and HTTP server
type App struct {
	config             Config
	service            *service.Service
	httpServer         *server.Server
	apiServer          *server.SimpleAPIServer
	authAPIServer      *server.AuthAPIServer
	tlsAPIServer       *server.TLSAPIServer
	dashboardAPIServer *server.DashboardAPIServer
	listAPIServer      *server.ListAPIServer
	securityService    *auth.SecurityService
	mu                 sync.RWMutex
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

	// Initialize security service
	authConfig := auth.ConvertSecurityConfig(a.config.Security)
	a.securityService = auth.NewSecurityService(authConfig)

	// Create initial admin if enabled
	if a.config.Security.EnableAuth {
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

	// Initialize API servers
	a.apiServer = server.NewSimpleAPIServer()
	a.apiServer.RegisterRoutes(a.httpServer)

	// Initialize auth API server if authentication is enabled
	if a.config.Security.EnableAuth {
		authServiceAdapter := NewSecurityServiceAdapter(a.securityService)
		a.authAPIServer = server.NewAuthAPIServer(authServiceAdapter)
		a.authAPIServer.RegisterRoutes(a.httpServer)
	}

	// Initialize TLS API server
	a.tlsAPIServer = server.NewTLSAPIServer(a.httpServer)
	a.tlsAPIServer.RegisterRoutes(a.httpServer)

	// Initialize dashboard API server
	a.dashboardAPIServer = server.NewDashboardAPIServer()
	a.dashboardAPIServer.RegisterRoutes(a.httpServer)

	// Initialize list API server
	if a.service != nil {
		// Get repository manager from service
		repos := a.service.GetRepositoryManager()
		a.listAPIServer = server.NewListAPIServer(repos)
		a.listAPIServer.RegisterRoutes(a.httpServer)
	}

	// Setup static file server for web dashboard
	if err := a.setupStaticFileServer(); err != nil {
		a.service.Stop()
		return fmt.Errorf("failed to setup static file server: %w", err)
	}

	// Start HTTP server
	if err := a.httpServer.Start(); err != nil {
		a.service.Stop()
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	logging.Info("Application started successfully",
		logging.String("http_address", a.httpServer.GetAddress()),
		logging.Bool("auth_enabled", a.config.Security.EnableAuth))

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

// GetSecurityService returns the security service (for admin/debugging purposes)
func (a *App) GetSecurityService() *auth.SecurityService {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.securityService
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
