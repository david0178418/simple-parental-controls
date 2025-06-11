package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

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
	}
}

// Server represents the embedded HTTP server
type Server struct {
	config     Config
	httpServer *http.Server
	listener   net.Listener
	mux        *http.ServeMux
	mu         sync.RWMutex
	running    bool
	startTime  time.Time
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
		config: config,
		mux:    mux,
	}

	// Register built-in endpoints
	server.registerBuiltinHandlers()

	return server
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server is already running")
	}

	// Create listener with appropriate binding
	listener, err := s.createListener()
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	s.listener = listener
	s.startTime = time.Now()

	// Create HTTP server
	s.httpServer = &http.Server{
		Handler:        s.mux,
		ReadTimeout:    s.config.ReadTimeout,
		WriteTimeout:   s.config.WriteTimeout,
		IdleTimeout:    s.config.IdleTimeout,
		MaxHeaderBytes: s.config.MaxHeaderBytes,
	}

	s.running = true

	// Start server in goroutine
	go func() {
		logging.Info("HTTP server starting",
			logging.String("address", listener.Addr().String()))

		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			logging.Error("HTTP server error", logging.Err(err))
		}
	}()

	logging.Info("HTTP server started successfully",
		logging.String("address", listener.Addr().String()),
		logging.Bool("lan_only", s.config.BindToLAN))

	return nil
}

// Stop gracefully shuts down the HTTP server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.httpServer == nil {
		return nil
	}

	logging.Info("Shutting down HTTP server")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Gracefully shutdown the server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		logging.Error("HTTP server shutdown error", logging.Err(err))
		return err
	}

	s.running = false
	s.httpServer = nil
	s.listener = nil

	logging.Info("HTTP server stopped successfully")
	return nil
}

// IsRunning returns whether the server is currently running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetAddress returns the server's listening address
func (s *Server) GetAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// AddHandler adds a new HTTP handler to the server
func (s *Server) AddHandler(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// AddHandlerFunc adds a new HTTP handler function to the server
func (s *Server) AddHandlerFunc(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
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
	s.mux.HandleFunc("/", s.handleRoot)
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
			},
		},
	}

	if s.running {
		status["uptime"] = time.Since(s.startTime).String()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleRoot serves as a fallback handler for unmatched routes
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	// For now, return a simple response
	// Later this will serve the React app
	if r.URL.Path == "/" {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>Parental Control</title>
</head>
<body>
    <h1>Parental Control Management</h1>
    <p>Web UI will be available here once implemented.</p>
    <ul>
        <li><a href="/health">Health Check</a></li>
        <li><a href="/status">Server Status</a></li>
    </ul>
</body>
</html>`))
		return
	}

	// Return 404 for other paths
	http.NotFound(w, r)
}
