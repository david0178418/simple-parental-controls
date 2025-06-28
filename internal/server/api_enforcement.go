package server

import (
	"encoding/json"
	"net/http"

	"parental-control/internal/logging"
	"parental-control/internal/service"
)

// EnforcementAPIServer handles enforcement-related API endpoints
type EnforcementAPIServer struct {
	enforcementService *service.EnforcementService
}

// NewEnforcementAPIServer creates a new enforcement API server
func NewEnforcementAPIServer(enforcementService *service.EnforcementService) *EnforcementAPIServer {
	return &EnforcementAPIServer{
		enforcementService: enforcementService,
	}
}

// RegisterRoutes registers enforcement API routes
func (api *EnforcementAPIServer) RegisterRoutes(server *Server) {
	if api.enforcementService == nil {
		logging.Warn("Enforcement service not available - skipping enforcement API routes")
		return
	}

	server.AddHandlerFunc("/api/v1/enforcement/refresh", api.handleRefreshRules)
	server.AddHandlerFunc("/api/v1/enforcement/stats", api.handleGetStats)
	server.AddHandlerFunc("/api/v1/enforcement/status", api.handleGetStatus)
}

// handleRefreshRules forces an immediate rule refresh
func (api *EnforcementAPIServer) handleRefreshRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()

	if err := api.enforcementService.RefreshRules(ctx); err != nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, "Failed to refresh rules: "+err.Error())
		return
	}

	api.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Rules refreshed successfully",
	})
}

// handleGetStats returns enforcement statistics
func (api *EnforcementAPIServer) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	stats := api.enforcementService.GetStats()
	api.writeJSONResponse(w, http.StatusOK, stats)
}

// handleGetStatus returns enforcement system status
func (api *EnforcementAPIServer) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	status := map[string]interface{}{
		"running": api.enforcementService.IsRunning(),
		"system":  api.enforcementService.GetSystemInfo(),
	}

	api.writeJSONResponse(w, http.StatusOK, status)
}

// Helper methods
func (api *EnforcementAPIServer) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		logging.Error("Failed to encode JSON response", logging.Err(err))
	}
}

func (api *EnforcementAPIServer) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	api.writeJSONResponse(w, statusCode, map[string]interface{}{
		"error":   true,
		"message": message,
		"status":  statusCode,
	})
}
