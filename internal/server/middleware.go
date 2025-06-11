package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"parental-control/internal/logging"
)

// Middleware represents a middleware function
type Middleware func(http.Handler) http.Handler

// MiddlewareChain manages a chain of middleware
type MiddlewareChain struct {
	middlewares []Middleware
}

// NewMiddlewareChain creates a new middleware chain
func NewMiddlewareChain(middlewares ...Middleware) *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: middlewares,
	}
}

// Then adds middleware to the chain
func (mc *MiddlewareChain) Then(handler http.Handler) http.Handler {
	// Apply middleware in reverse order so they execute in the correct order
	for i := len(mc.middlewares) - 1; i >= 0; i-- {
		handler = mc.middlewares[i](handler)
	}
	return handler
}

// ThenFunc adds middleware to the chain and wraps a handler function
func (mc *MiddlewareChain) ThenFunc(handlerFunc http.HandlerFunc) http.Handler {
	return mc.Then(handlerFunc)
}

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := generateRequestID()
			ctx := context.WithValue(r.Context(), "request_id", requestID)
			r = r.WithContext(ctx)

			w.Header().Set("X-Request-ID", requestID)
			next.ServeHTTP(w, r)
		})
	}
}

// LoggingMiddleware logs HTTP requests and responses
func LoggingMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a response writer wrapper to capture status code
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Get request ID from context
			requestID := getRequestID(r.Context())

			// Log request
			logging.Info("HTTP request started",
				logging.String("request_id", requestID),
				logging.String("method", r.Method),
				logging.String("path", r.URL.Path),
				logging.String("remote_addr", r.RemoteAddr),
				logging.String("user_agent", r.UserAgent()),
			)

			// Process request
			next.ServeHTTP(rw, r)

			// Log response
			duration := time.Since(start)
			logging.Info("HTTP request completed",
				logging.String("request_id", requestID),
				logging.String("method", r.Method),
				logging.String("path", r.URL.Path),
				logging.Int("status", rw.statusCode),
				logging.String("duration", duration.String()),
				logging.Int("bytes", rw.bytesWritten),
			)
		})
	}
}

// RecoveryMiddleware recovers from panics and returns a 500 error
func RecoveryMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					requestID := getRequestID(r.Context())

					logging.Error("HTTP request panic recovered",
						logging.String("request_id", requestID),
						logging.String("method", r.Method),
						logging.String("path", r.URL.Path),
						logging.String("error", fmt.Sprintf("%v", err)),
						logging.String("stack", string(debug.Stack())),
					)

					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// CORSMiddleware handles Cross-Origin Resource Sharing
func CORSMiddleware(allowedOrigins []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			if isOriginAllowed(origin, allowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "3600")

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeadersMiddleware adds security headers
func SecurityHeadersMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Content Security Policy
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'unsafe-inline'; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data:; "+
					"font-src 'self'; "+
					"connect-src 'self'")

			// Other security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Only add HSTS for HTTPS
			if r.TLS != nil {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitMiddleware implements rate limiting
func RateLimitMiddleware(requestsPerMinute int) Middleware {
	limiter := newRateLimiter(requestsPerMinute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := getClientIP(r)

			if !limiter.Allow(clientIP) {
				requestID := getRequestID(r.Context())
				logging.Warn("Rate limit exceeded",
					logging.String("request_id", requestID),
					logging.String("client_ip", clientIP),
					logging.String("path", r.URL.Path),
				)

				w.Header().Set("Retry-After", "60")
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TimeoutMiddleware adds request timeout handling
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			r = r.WithContext(ctx)

			done := make(chan struct{})
			go func() {
				defer close(done)
				next.ServeHTTP(w, r)
			}()

			select {
			case <-done:
				// Request completed normally
			case <-ctx.Done():
				// Request timed out
				requestID := getRequestID(r.Context())
				logging.Warn("Request timeout",
					logging.String("request_id", requestID),
					logging.String("method", r.Method),
					logging.String("path", r.URL.Path),
					logging.String("timeout", timeout.String()),
				)

				http.Error(w, "Request timeout", http.StatusRequestTimeout)
			}
		})
	}
}

// JSONMiddleware handles JSON request/response processing
func JSONMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// For POST, PUT, PATCH requests, validate JSON content type
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				contentType := r.Header.Get("Content-Type")
				if contentType != "" && !strings.HasPrefix(contentType, "application/json") {
					http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
					return
				}
			}

			// Set JSON response header for API endpoints
			if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "application/json")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ContentLengthMiddleware validates request content length
func ContentLengthMiddleware(maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				http.Error(w, "Request entity too large", http.StatusRequestEntityTooLarge)
				return
			}

			// Limit the request body reader
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

			next.ServeHTTP(w, r)
		})
	}
}

// IPWhitelistMiddleware restricts access to allowed IP ranges
func IPWhitelistMiddleware(allowedRanges []string) Middleware {
	var allowedNets []*net.IPNet

	for _, rangeStr := range allowedRanges {
		_, ipNet, err := net.ParseCIDR(rangeStr)
		if err != nil {
			// Try parsing as single IP
			ip := net.ParseIP(rangeStr)
			if ip != nil {
				if ip.To4() != nil {
					_, ipNet, _ = net.ParseCIDR(rangeStr + "/32")
				} else {
					_, ipNet, _ = net.ParseCIDR(rangeStr + "/128")
				}
			}
		}
		if ipNet != nil {
			allowedNets = append(allowedNets, ipNet)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := net.ParseIP(getClientIP(r))
			if clientIP == nil {
				http.Error(w, "Invalid client IP", http.StatusBadRequest)
				return
			}

			allowed := false
			for _, allowedNet := range allowedNets {
				if allowedNet.Contains(clientIP) {
					allowed = true
					break
				}
			}

			if !allowed {
				requestID := getRequestID(r.Context())
				logging.Warn("IP not in whitelist",
					logging.String("request_id", requestID),
					logging.String("client_ip", clientIP.String()),
					logging.String("path", r.URL.Path),
				)

				http.Error(w, "Access denied", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Helper types and functions

// responseWriter wraps http.ResponseWriter to capture response data
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	rw.bytesWritten += len(data)
	return rw.ResponseWriter.Write(data)
}

// Rate limiter implementation
type rateLimiter struct {
	requests map[string]*clientRequests
	mu       sync.RWMutex
	limit    int
}

type clientRequests struct {
	count     int
	resetTime time.Time
}

func newRateLimiter(requestsPerMinute int) *rateLimiter {
	rl := &rateLimiter{
		requests: make(map[string]*clientRequests),
		limit:    requestsPerMinute,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

func (rl *rateLimiter) Allow(clientIP string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	client, exists := rl.requests[clientIP]

	if !exists || now.After(client.resetTime) {
		rl.requests[clientIP] = &clientRequests{
			count:     1,
			resetTime: now.Add(time.Minute),
		}
		return true
	}

	if client.count >= rl.limit {
		return false
	}

	client.count++
	return true
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()

		for ip, client := range rl.requests {
			if now.After(client.resetTime) {
				delete(rl.requests, ip)
			}
		}

		rl.mu.Unlock()
	}
}

// Utility functions

func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func getRequestID(ctx context.Context) string {
	if id, ok := ctx.Value("request_id").(string); ok {
		return id
	}
	return "unknown"
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isOriginAllowed(origin string, allowedOrigins []string) bool {
	if len(allowedOrigins) == 0 {
		return false
	}

	for _, allowed := range allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}

	return false
}

// ErrorResponse represents a standard API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}

// WriteErrorResponse writes a standardized error response
func WriteErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
		Code:    statusCode,
	}

	json.NewEncoder(w).Encode(response)
}

// WriteJSONResponse writes a JSON response
func WriteJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	return json.NewEncoder(w).Encode(data)
}
