package server

import (
	"net/http"
	"time"

	"parental-control/internal/logging"
)

// TLSAPIServer handles TLS/certificate-related API endpoints
type TLSAPIServer struct {
	tlsManager *TLSManager
	server     *Server
}

// NewTLSAPIServer creates a new TLS API server instance
func NewTLSAPIServer(server *Server) *TLSAPIServer {
	return &TLSAPIServer{
		tlsManager: server.tlsManager,
		server:     server,
	}
}

// RegisterRoutes registers TLS API routes with the given server
func (api *TLSAPIServer) RegisterRoutes(server *Server) {
	// Base middleware chain for API endpoints
	baseMiddleware := NewMiddlewareChain(
		RequestIDMiddleware(),
		LoggingMiddleware(),
		RecoveryMiddleware(),
		SecurityHeadersMiddleware(),
		JSONMiddleware(),
		ContentLengthMiddleware(1024*1024), // 1MB limit
	)

	// TLS/Certificate endpoints (public for certificate download)
	server.AddHandler("/api/v1/tls/info", baseMiddleware.ThenFunc(api.handleTLSInfo))
	server.AddHandler("/api/v1/tls/certificate", baseMiddleware.ThenFunc(api.handleCertificateDownload))
	server.AddHandler("/api/v1/tls/trust-instructions", baseMiddleware.ThenFunc(api.handleTrustInstructions))
}

// handleTLSInfo returns TLS configuration and certificate information
func (api *TLSAPIServer) handleTLSInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get TLS certificate information
	tlsInfo, err := api.server.GetTLSInfo()
	if err != nil {
		logging.Error("Failed to get TLS info", logging.Err(err))
		WriteErrorResponse(w, http.StatusInternalServerError, "Failed to get TLS information")
		return
	}

	// Add server addresses
	response := map[string]interface{}{
		"tls":           tlsInfo,
		"http_address":  api.server.GetAddress(),
		"https_address": api.server.GetHTTPSAddress(),
		"tls_enabled":   api.server.IsTLSEnabled(),
		"timestamp":     time.Now(),
	}

	WriteJSONResponse(w, http.StatusOK, response)
}

// handleCertificateDownload serves the server certificate for download
func (api *TLSAPIServer) handleCertificateDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !api.server.IsTLSEnabled() {
		WriteErrorResponse(w, http.StatusNotFound, "TLS not enabled")
		return
	}

	// Export certificate
	certPEM, err := api.tlsManager.ExportCertificate()
	if err != nil {
		logging.Error("Failed to export certificate", logging.Err(err))
		WriteErrorResponse(w, http.StatusInternalServerError, "Failed to export certificate")
		return
	}

	// Set appropriate headers for certificate download
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", "attachment; filename=\"server.crt\"")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	w.WriteHeader(http.StatusOK)
	w.Write(certPEM)
}

// handleTrustInstructions returns instructions for trusting the self-signed certificate
func (api *TLSAPIServer) handleTrustInstructions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !api.server.IsTLSEnabled() {
		WriteErrorResponse(w, http.StatusNotFound, "TLS not enabled")
		return
	}

	// Get trust instructions
	instructions := api.tlsManager.GetTrustInstructions()

	// Check Accept header to determine response format
	acceptHeader := r.Header.Get("Accept")
	if acceptHeader == "application/json" || r.URL.Query().Get("format") == "json" {
		// Return JSON response
		response := map[string]interface{}{
			"instructions": instructions,
			"format":       "text",
			"timestamp":    time.Now(),
			"endpoints": map[string]string{
				"certificate": "/api/v1/tls/certificate",
				"info":        "/api/v1/tls/info",
			},
		}
		WriteJSONResponse(w, http.StatusOK, response)
		return
	}

	// Return plain text response
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(instructions))
}

// GetTLSManager returns the TLS manager (for testing)
func (api *TLSAPIServer) GetTLSManager() *TLSManager {
	return api.tlsManager
}
