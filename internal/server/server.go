package server

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"io/fs"
	"parental-control/internal/logging"
)

// Config holds the HTTP server configuration
type Config struct {
	// Port to bind the server to
	Port int
	// BindToLAN determines if server should bind to LAN interfaces only
	BindToLAN bool
	// AllowedInterfaces specific interfaces to bind to (if empty, auto-detect)
	AllowedInterfaces []string
	// ReadTimeout for incoming requests
	ReadTimeout time.Duration
	// WriteTimeout for outgoing responses
	WriteTimeout time.Duration
	// IdleTimeout for keep-alive connections
	IdleTimeout time.Duration
	// MaxHeaderBytes limits request header size
	MaxHeaderBytes int
	// StaticFileRoot path to static files for the web UI
	StaticFileRoot string
	// EnableCompression for static file serving
	EnableCompression bool
	// TLS configuration
	TLS TLSConfig
}

// DefaultConfig returns server configuration with sensible defaults
func DefaultConfig() Config {
	return Config{
		Port:              8080,
		BindToLAN:         true,
		AllowedInterfaces: []string{},
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
		StaticFileRoot:    "./web/build",
		EnableCompression: true,
		TLS:               DefaultTLSConfig(),
	}
}

// Server represents the embedded HTTP server
type Server struct {
	config      Config
	httpServer  *http.Server
	httpsServer *http.Server
	listener    net.Listener
	tlsListener net.Listener
	mux         *http.ServeMux
	tlsManager  *TLSManager
	mu          sync.RWMutex
	running     bool
	startTime   time.Time
}

// HealthStatus represents the server health information
type HealthStatus struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Uptime    string            `json:"uptime"`
	Version   string            `json:"version"`
	Endpoints map[string]string `json:"endpoints"`
}

// New creates a new HTTP server instance
func New(config Config) *Server {
	mux := http.NewServeMux()

	server := &Server{
		config:     config,
		mux:        mux,
		tlsManager: NewTLSManager(config.TLS),
	}

	// Register built-in endpoints
	server.registerBuiltinHandlers()

	return server
}

// Start starts the HTTP server and optionally HTTPS server
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server is already running")
	}

	s.startTime = time.Now()

	// Start HTTPS server if TLS is enabled
	if s.config.TLS.Enabled {
		if err := s.startHTTPSServer(); err != nil {
			return fmt.Errorf("failed to start HTTPS server: %w", err)
		}
	}

	// Start HTTP server (either standalone or for redirects)
	if err := s.startHTTPServer(); err != nil {
		// If HTTPS failed to start, clean up HTTPS server
		if s.httpsServer != nil {
			s.httpsServer.Close()
		}
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	s.running = true

	if s.config.TLS.Enabled {
		logging.Info("Servers started successfully",
			logging.String("http_address", s.listener.Addr().String()),
			logging.String("https_address", s.tlsListener.Addr().String()),
			logging.Bool("lan_only", s.config.BindToLAN))
	} else {
		logging.Info("HTTP server started successfully",
			logging.String("address", s.listener.Addr().String()),
			logging.Bool("lan_only", s.config.BindToLAN))
	}

	go func() {
		<-ctx.Done()
		s.Stop(context.Background()) // Fallback stop
	}()

	go func() {
		if s.config.TLS.Enabled {
			// Start HTTPS server if TLS is enabled
			if err := s.startHTTPSServer(); err != nil {
				logging.Error("HTTPS server start error", logging.Err(err))
			}
		}
	}()

	return nil
}

// startHTTPSServer starts the HTTPS server
func (s *Server) startHTTPSServer() error {
	// Ensure certificates exist
	if err := s.tlsManager.EnsureCertificates(); err != nil {
		return fmt.Errorf("failed to ensure TLS certificates: %w", err)
	}

	// Get TLS configuration
	tlsConfig, err := s.tlsManager.GetTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to get TLS config: %w", err)
	}

	// Create HTTPS listener
	httpsPort := s.config.Port
	if s.config.TLS.RedirectHTTP {
		httpsPort = 8443 // Use different port for HTTPS when redirecting
	}

	httpsAddr := fmt.Sprintf(":%d", httpsPort)
	if s.config.BindToLAN {
		// For HTTPS, we'll bind to the same interface detection as HTTP
		listener, err := s.createListener()
		if err != nil {
			return fmt.Errorf("failed to create HTTPS listener: %w", err)
		}
		listener.Close() // We just needed the address

		if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
			tcpAddr.Port = httpsPort
			httpsAddr = tcpAddr.String()
		}
	}

	tlsListener, err := tls.Listen("tcp", httpsAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to create TLS listener: %w", err)
	}

	s.tlsListener = tlsListener

	// Create HTTPS server
	s.httpsServer = &http.Server{
		Handler:        s.mux,
		ReadTimeout:    s.config.ReadTimeout,
		WriteTimeout:   s.config.WriteTimeout,
		IdleTimeout:    s.config.IdleTimeout,
		MaxHeaderBytes: s.config.MaxHeaderBytes,
		TLSConfig:      tlsConfig,
	}

	// Start HTTPS server in goroutine
	go func() {
		logging.Info("HTTPS server starting",
			logging.String("address", tlsListener.Addr().String()))

		if err := s.httpsServer.Serve(tlsListener); err != nil && err != http.ErrServerClosed {
			logging.Error("HTTPS server error", logging.Err(err))
		}
	}()

	return nil
}

// startHTTPServer starts the HTTP server
func (s *Server) startHTTPServer() error {
	// Create listener with appropriate binding
	listener, err := s.createListener()
	if err != nil {
		return fmt.Errorf("failed to create HTTP listener: %w", err)
	}

	s.listener = listener

	// Determine handler for HTTP server
	var handler http.Handler = s.mux

	// If TLS is enabled and redirect is configured, use redirect handler
	if s.config.TLS.Enabled && s.config.TLS.RedirectHTTP {
		httpsPort := 8443
		if s.tlsListener != nil {
			if tcpAddr, ok := s.tlsListener.Addr().(*net.TCPAddr); ok {
				httpsPort = tcpAddr.Port
			}
		}
		handler = s.tlsManager.HTTPRedirectHandler(httpsPort)
	}

	// Create HTTP server
	s.httpServer = &http.Server{
		Handler:        handler,
		ReadTimeout:    s.config.ReadTimeout,
		WriteTimeout:   s.config.WriteTimeout,
		IdleTimeout:    s.config.IdleTimeout,
		MaxHeaderBytes: s.config.MaxHeaderBytes,
	}

	// Start HTTP server in goroutine
	go func() {
		logging.Info("HTTP server starting",
			logging.String("address", listener.Addr().String()))

		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			logging.Error("HTTP server error", logging.Err(err))
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP and HTTPS servers
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	logging.Info("Shutting down servers")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var shutdownErrors []error

	// Shutdown HTTPS server if running
	if s.httpsServer != nil {
		if err := s.httpsServer.Shutdown(shutdownCtx); err != nil {
			logging.Error("HTTPS server shutdown error", logging.Err(err))
			shutdownErrors = append(shutdownErrors, err)
		}
		s.httpsServer = nil
		s.tlsListener = nil
	}

	// Shutdown HTTP server if running
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			logging.Error("HTTP server shutdown error", logging.Err(err))
			shutdownErrors = append(shutdownErrors, err)
		}
		s.httpServer = nil
		s.listener = nil
	}

	s.running = false

	if len(shutdownErrors) > 0 {
		return fmt.Errorf("errors during server shutdown: %v", shutdownErrors)
	}

	logging.Info("Servers stopped successfully")
	return nil
}

// IsRunning returns whether the server is currently running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetAddress returns the HTTP server's listening address
func (s *Server) GetAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// GetHTTPSAddress returns the HTTPS server's listening address
func (s *Server) GetHTTPSAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.tlsListener == nil {
		return ""
	}
	return s.tlsListener.Addr().String()
}

// IsTLSEnabled returns whether TLS is enabled
func (s *Server) IsTLSEnabled() bool {
	return s.config.TLS.Enabled
}

// GetTLSInfo returns TLS certificate information
func (s *Server) GetTLSInfo() (map[string]interface{}, error) {
	if s.tlsManager == nil {
		return map[string]interface{}{"enabled": false}, nil
	}
	return s.tlsManager.GetCertificateInfo()
}

// AddHandler adds a new HTTP handler to the server
func (s *Server) AddHandler(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// AddHandlerFunc adds a new HTTP handler function to the server
func (s *Server) AddHandlerFunc(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
}

// SetupStaticFileServer configures and registers the static file server
func (s *Server) SetupStaticFileServer(fileSystem fs.FS, authMiddleware *AuthMiddleware) error {
	if s.config.StaticFileRoot == "" {
		return fmt.Errorf("static file root not configured")
	}

	// Create static file server
	staticServer := NewStaticFileServer(s.config, fileSystem)

	// By default, the static server is unprotected
	var protectedStaticServer http.Handler = staticServer

	// If auth is enabled, protect the static file server
	if authMiddleware != nil {
		// Protect all routes except for the root (which should be public)
		// The static file server will handle redirecting to /login
		authMiddleware.AddPublicPath("/")
		protectedStaticServer = authMiddleware.RequireAuth()(staticServer)
	}

	// Register the static file server for all unmatched routes
	s.mux.Handle("/", protectedStaticServer)

	logging.Info("Static file server configured",
		logging.String("static_root", s.config.StaticFileRoot),
		logging.Bool("compression_enabled", s.config.EnableCompression),
		logging.Bool("auth_enabled", authMiddleware != nil))

	return nil
}

// createListener creates the appropriate network listener based on configuration
func (s *Server) createListener() (net.Listener, error) {
	if s.config.BindToLAN {
		return s.createLANListener()
	}

	// Bind to all interfaces
	addr := fmt.Sprintf(":%d", s.config.Port)
	return net.Listen("tcp", addr)
}

// createLANListener creates a listener that only binds to LAN interfaces
func (s *Server) createLANListener() (net.Listener, error) {
	// Get LAN interfaces
	interfaces, err := s.getLANInterfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get LAN interfaces: %w", err)
	}

	if len(interfaces) == 0 {
		// Fallback to localhost if no LAN interfaces found
		logging.Warn("No LAN interfaces found, binding to localhost only")
		addr := fmt.Sprintf("127.0.0.1:%d", s.config.Port)
		return net.Listen("tcp", addr)
	}

	// Try to bind to the first available LAN interface
	for _, iface := range interfaces {
		addr := fmt.Sprintf("%s:%d", iface, s.config.Port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			logging.Warn("Failed to bind to interface",
				logging.String("interface", iface),
				logging.Err(err))
			continue
		}

		logging.Info("Successfully bound to LAN interface",
			logging.String("interface", iface),
			logging.String("address", addr))
		return listener, nil
	}

	return nil, fmt.Errorf("failed to bind to any LAN interface")
}

// getLANInterfaces returns a list of LAN IP addresses
func (s *Server) getLANInterfaces() ([]string, error) {
	var lanIPs []string

	// If specific interfaces are configured, use those
	if len(s.config.AllowedInterfaces) > 0 {
		return s.config.AllowedInterfaces, nil
	}

	// Auto-detect LAN interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipNet.IP
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}

			// Check if it's a private IP address (LAN)
			if s.isPrivateIP(ip) {
				lanIPs = append(lanIPs, ip.String())
			}
		}
	}

	return lanIPs, nil
}

// isPrivateIP checks if an IP address is in private ranges
func (s *Server) isPrivateIP(ip net.IP) bool {
	// Private IP ranges:
	// 10.0.0.0/8
	// 172.16.0.0/12
	// 192.168.0.0/16

	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 10 ||
			(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) ||
			(ip4[0] == 192 && ip4[1] == 168)
	}

	// IPv6 private ranges (ULA)
	if ip6 := ip.To16(); ip6 != nil {
		return ip6[0] == 0xfc || ip6[0] == 0xfd
	}

	return false
}

// registerBuiltinHandlers registers the server's built-in endpoints
func (s *Server) registerBuiltinHandlers() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/status", s.handleStatus)
	// Note: Static file server will be registered separately during server initialization
}

// handleHealth returns server health information
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := HealthStatus{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0", // TODO: Get from build info
		Endpoints: map[string]string{
			"health": "/health",
			"status": "/status",
			"api":    "/api/v1",
		},
	}

	if s.running {
		status.Uptime = time.Since(s.startTime).String()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleStatus returns detailed server status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	status := map[string]interface{}{
		"server": map[string]interface{}{
			"running":    s.running,
			"start_time": s.startTime,
			"address":    s.GetAddress(),
			"config": map[string]interface{}{
				"port":               s.config.Port,
				"bind_to_lan":        s.config.BindToLAN,
				"read_timeout":       s.config.ReadTimeout.String(),
				"write_timeout":      s.config.WriteTimeout.String(),
				"idle_timeout":       s.config.IdleTimeout.String(),
				"enable_compression": s.config.EnableCompression,
				"static_file_root":   s.config.StaticFileRoot,
			},
		},
	}

	if s.running {
		status["uptime"] = time.Since(s.startTime).String()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
