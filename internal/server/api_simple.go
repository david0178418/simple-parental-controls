package server

import (
	"net/http"
	"time"
)

// SimpleAPIServer handles basic RESTful API endpoints without repository dependencies
type SimpleAPIServer struct {
}

// NewSimpleAPIServer creates a new simple API server instance
func NewSimpleAPIServer() *SimpleAPIServer {
	return &SimpleAPIServer{}
}

// RegisterRoutes registers basic API routes with the given server
func (api *SimpleAPIServer) RegisterRoutes(server *Server) {
	// Middleware chain for API endpoints
	apiMiddleware := NewMiddlewareChain(
		RequestIDMiddleware(),
		LoggingMiddleware(),
		RecoveryMiddleware(),
		SecurityHeadersMiddleware(),
		JSONMiddleware(),
		ContentLengthMiddleware(1024*1024), // 1MB limit
	)

	// Basic endpoints for testing
	server.AddHandler("/api/v1/ping", apiMiddleware.ThenFunc(api.handlePing))
	server.AddHandler("/api/v1/info", apiMiddleware.ThenFunc(api.handleInfo))
}

// handlePing returns a simple ping response
func (api *SimpleAPIServer) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"message":   "pong",
		"timestamp": time.Now(),
	})
}

// handleInfo returns basic server information
func (api *SimpleAPIServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	info := map[string]interface{}{
		"name":      "Parental Control API",
		"version":   "1.0.0",
		"timestamp": time.Now(),
		"endpoints": map[string]string{
			"ping":   "/api/v1/ping",
			"info":   "/api/v1/info",
			"health": "/health",
			"status": "/status",
		},
	}

	WriteJSONResponse(w, http.StatusOK, info)
}
