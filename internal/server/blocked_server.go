package server

import (
	"context"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"sync"
	"time"

	"parental-control/internal/logging"
)

// BlockedServerConfig holds configuration for the blocked page server
type BlockedServerConfig struct {
	// Port to bind the blocked page server to
	Port int
	// Host to bind to (usually localhost)
	Host string
	// ReadTimeout for incoming requests
	ReadTimeout time.Duration
	// WriteTimeout for outgoing responses
	WriteTimeout time.Duration
	// IdleTimeout for keep-alive connections
	IdleTimeout time.Duration
	// MaxHeaderBytes limits request header size
	MaxHeaderBytes int
	// CustomMessage optional custom message for blocked pages
	CustomMessage string
	// EnableLogging whether to log blocked page access attempts
	EnableLogging bool
}

// DefaultBlockedServerConfig returns blocked server configuration with sensible defaults
func DefaultBlockedServerConfig() BlockedServerConfig {
	return BlockedServerConfig{
		Port:           8081,
		Host:           "127.0.0.1",
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxHeaderBytes: 1 << 16, // 64 KB
		CustomMessage:  "",
		EnableLogging:  true,
	}
}

// BlockedServer serves blocked page content for filtered domains
type BlockedServer struct {
	config     BlockedServerConfig
	httpServer *http.Server
	listener   net.Listener
	mux        *http.ServeMux
	template   *template.Template
	mu         sync.RWMutex
	running    bool
	startTime  time.Time
}

// BlockedPageData contains data passed to the blocked page template
type BlockedPageData struct {
	Domain        string
	URL           string
	Timestamp     time.Time
	CustomMessage string
	Reason        string
	RequestID     string
}

// NewBlockedServer creates a new blocked page server instance
func NewBlockedServer(config BlockedServerConfig) *BlockedServer {
	mux := http.NewServeMux()
	
	server := &BlockedServer{
		config: config,
		mux:    mux,
	}

	// Initialize template
	server.initTemplate()
	
	// Register handlers
	server.registerHandlers()

	return server
}

// Start starts the blocked page server
func (bs *BlockedServer) Start(ctx context.Context) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if bs.running {
		return fmt.Errorf("blocked server is already running")
	}

	bs.startTime = time.Now()

	// Create listener
	addr := fmt.Sprintf("%s:%d", bs.config.Host, bs.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to create blocked server listener: %w", err)
	}

	bs.listener = listener

	// Create HTTP server
	bs.httpServer = &http.Server{
		Handler:        bs.mux,
		ReadTimeout:    bs.config.ReadTimeout,
		WriteTimeout:   bs.config.WriteTimeout,
		IdleTimeout:    bs.config.IdleTimeout,
		MaxHeaderBytes: bs.config.MaxHeaderBytes,
	}

	// Start server in goroutine
	go func() {
		logging.Info("Blocked page server starting",
			logging.String("address", listener.Addr().String()))

		if err := bs.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			logging.Error("Blocked page server error", logging.Err(err))
		}
	}()

	bs.running = true

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		bs.Stop(context.Background())
	}()

	logging.Info("Blocked page server started successfully",
		logging.String("address", listener.Addr().String()))

	return nil
}

// Stop gracefully shuts down the blocked page server
func (bs *BlockedServer) Stop(ctx context.Context) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if !bs.running {
		return nil
	}

	logging.Info("Shutting down blocked page server")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Shutdown server
	if bs.httpServer != nil {
		if err := bs.httpServer.Shutdown(shutdownCtx); err != nil {
			logging.Error("Blocked page server shutdown error", logging.Err(err))
			return err
		}
		bs.httpServer = nil
		bs.listener = nil
	}

	bs.running = false

	logging.Info("Blocked page server stopped successfully")
	return nil
}

// IsRunning returns whether the blocked server is currently running
func (bs *BlockedServer) IsRunning() bool {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.running
}

// GetAddress returns the blocked server's listening address
func (bs *BlockedServer) GetAddress() string {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	if bs.listener == nil {
		return ""
	}
	return bs.listener.Addr().String()
}

// registerHandlers registers HTTP handlers for the blocked page server
func (bs *BlockedServer) registerHandlers() {
	// Handle all requests as blocked content
	bs.mux.HandleFunc("/", bs.handleBlockedPage)
	bs.mux.HandleFunc("/favicon.ico", bs.handleFavicon)
	bs.mux.HandleFunc("/health", bs.handleHealth)
}

// handleBlockedPage serves the blocked page content
func (bs *BlockedServer) handleBlockedPage(w http.ResponseWriter, r *http.Request) {
	// Log the blocked access attempt if enabled
	if bs.config.EnableLogging {
		logging.Info("Blocked page access attempt",
			logging.String("domain", r.Host),
			logging.String("url", r.URL.String()),
			logging.String("user_agent", r.UserAgent()),
			logging.String("remote_addr", r.RemoteAddr))
	}

	// Extract domain from request
	domain := r.Host
	if domain == "" {
		domain = "unknown"
	}

	// Create page data
	pageData := BlockedPageData{
		Domain:        domain,
		URL:           r.URL.String(),
		Timestamp:     time.Now(),
		CustomMessage: bs.config.CustomMessage,
		Reason:        "This website has been blocked by parental controls",
		RequestID:     fmt.Sprintf("%d", time.Now().UnixNano()),
	}

	// Set headers to prevent caching
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Execute template
	if err := bs.template.Execute(w, pageData); err != nil {
		logging.Error("Failed to execute blocked page template", logging.Err(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// handleFavicon serves a minimal favicon to prevent 404 errors
func (bs *BlockedServer) handleFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 1 day
	// Serve empty favicon
	w.WriteHeader(http.StatusOK)
}

// handleHealth returns blocked server health information
func (bs *BlockedServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","service":"blocked-page-server"}`))
}

// initTemplate initializes the blocked page HTML template
func (bs *BlockedServer) initTemplate() {
	const blockedPageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Access Blocked</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            margin: 0;
            padding: 0;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container {
            background: white;
            border-radius: 12px;
            box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
            padding: 2rem;
            max-width: 500px;
            width: 90%;
            text-align: center;
            animation: slideIn 0.5s ease-out;
        }
        @keyframes slideIn {
            from {
                opacity: 0;
                transform: translateY(-20px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }
        .icon {
            font-size: 4rem;
            color: #e74c3c;
            margin-bottom: 1rem;
        }
        h1 {
            color: #2c3e50;
            margin-bottom: 1rem;
            font-size: 1.8rem;
        }
        .domain {
            background: #f8f9fa;
            padding: 0.5rem 1rem;
            border-radius: 6px;
            font-family: monospace;
            margin: 1rem 0;
            word-break: break-all;
            color: #495057;
        }
        .reason {
            color: #6c757d;
            margin: 1.5rem 0;
            line-height: 1.6;
        }
        .custom-message {
            background: #e3f2fd;
            padding: 1rem;
            border-radius: 6px;
            margin: 1rem 0;
            color: #1565c0;
            border-left: 3px solid #2196f3;
        }
        .details {
            margin-top: 2rem;
            padding-top: 1rem;
            border-top: 1px solid #dee2e6;
            font-size: 0.9rem;
            color: #6c757d;
        }
        .refresh-notice {
            background: #fff3cd;
            padding: 1rem;
            border-radius: 6px;
            margin: 1rem 0;
            color: #856404;
            border-left: 3px solid #ffc107;
            font-size: 0.9rem;
        }
        .technical-info {
            background: #f8f9fa;
            padding: 1rem;
            border-radius: 6px;
            margin: 1rem 0;
            color: #6c757d;
            font-size: 0.8rem;
            border: 1px solid #dee2e6;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">🚫</div>
        <h1>Access Blocked</h1>
        <div class="domain">{{.Domain}}</div>
        <div class="reason">{{.Reason}}</div>
        {{if .CustomMessage}}
        <div class="custom-message">{{.CustomMessage}}</div>
        {{end}}
        <div class="refresh-notice">
            <strong>Note:</strong> Refreshing this page or clearing your browser cache will not bypass this block.
        </div>
        <div class="technical-info">
            <strong>Technical Info:</strong> This domain has been redirected to a local blocked page server (127.0.0.1:80) by the parental control system's DNS filtering.
        </div>
        <div class="details">
            <div>Time: {{.Timestamp.Format "2006-01-02 15:04:05"}}</div>
            <div>Request ID: {{.RequestID}}</div>
            <div>Requested URL: {{.URL}}</div>
        </div>
    </div>

    <script>
    // Prevent cache bypass attempts
    (function() {
        // Override common cache bypass methods
        const originalReload = location.reload;
        const originalReplace = location.replace;
        const originalAssign = location.assign;
        
        // Add timestamp to prevent cache bypass on reload
        location.reload = function(forcedReload) {
            const timestamp = Date.now();
            const separator = location.search ? '&' : '?';
            const newUrl = location.pathname + location.search + separator + '_blocked_ts=' + timestamp + location.hash;
            location.replace(newUrl);
        };
        
        // Handle manual URL manipulation attempts
        location.replace = function(url) {
            if (url && !url.includes('_blocked_ts=')) {
                const timestamp = Date.now();
                const separator = url.includes('?') ? '&' : '?';
                url = url + separator + '_blocked_ts=' + timestamp;
            }
            originalReplace.call(location, url);
        };
        
        location.assign = function(url) {
            if (url && !url.includes('_blocked_ts=')) {
                const timestamp = Date.now();
                const separator = url.includes('?') ? '&' : '?';
                url = url + separator + '_blocked_ts=' + timestamp;
            }
            originalAssign.call(location, url);
        };
        
        // Detect and handle common cache bypass key combinations
        document.addEventListener('keydown', function(e) {
            // Ctrl+F5, Shift+F5, Ctrl+Shift+R (hard refresh)
            if ((e.ctrlKey && e.key === 'F5') || 
                (e.shiftKey && e.key === 'F5') || 
                (e.ctrlKey && e.shiftKey && e.key === 'R')) {
                e.preventDefault();
                location.reload();
                return false;
            }
        });
        
        // Handle page visibility changes (detect when user comes back from cache clearing)
        document.addEventListener('visibilitychange', function() {
            if (!document.hidden) {
                // Add timestamp to current URL if not already present
                if (!location.search.includes('_blocked_ts=')) {
                    const timestamp = Date.now();
                    const separator = location.search ? '&' : '?';
                    const newUrl = location.pathname + location.search + separator + '_blocked_ts=' + timestamp + location.hash;
                    history.replaceState(null, '', newUrl);
                }
            }
        });
        
        // Prevent right-click context menu to avoid "Reload" option
        document.addEventListener('contextmenu', function(e) {
            e.preventDefault();
            return false;
        });
        
        // Add cache-busting parameter to current URL if not present
        if (!location.search.includes('_blocked_ts=')) {
            const timestamp = Date.now();
            const separator = location.search ? '&' : '?';
            const newUrl = location.pathname + location.search + separator + '_blocked_ts=' + timestamp + location.hash;
            history.replaceState(null, '', newUrl);
        }
    })();
    </script>
</body>
</html>`

	tmpl, err := template.New("blocked_page").Parse(blockedPageTemplate)
	if err != nil {
		logging.Error("Failed to parse blocked page template", logging.Err(err))
		// Use a simple fallback template
		tmpl = template.Must(template.New("blocked_page_fallback").Parse(`
<!DOCTYPE html>
<html><head><title>Access Blocked</title></head>
<body><h1>Access Blocked</h1><p>{{.Domain}} has been blocked.</p></body>
</html>`))
	}

	bs.template = tmpl
}