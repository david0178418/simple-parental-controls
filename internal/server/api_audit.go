package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"parental-control/internal/logging"
	"parental-control/internal/models"
	"parental-control/internal/service"
)

// AuditLogHandler handles audit log API endpoints
type AuditLogHandler struct {
	auditService *service.AuditService
	logger       logging.Logger
}

// NewAuditLogHandler creates a new audit log handler
func NewAuditLogHandler(auditService *service.AuditService, logger logging.Logger) *AuditLogHandler {
	return &AuditLogHandler{
		auditService: auditService,
		logger:       logger,
	}
}

// RegisterRoutes registers audit log API routes
func (h *AuditLogHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/audit", h.handleAuditLogs)
	mux.HandleFunc("/api/v1/audit/", h.handleAuditLogDetail)
	mux.HandleFunc("/api/v1/audit/stats", h.handleAuditStats)
	mux.HandleFunc("/api/v1/audit/cleanup", h.handleAuditCleanup)
}

// handleAuditLogs handles GET /api/v1/audit - get audit logs with filtering
func (h *AuditLogHandler) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Parse query parameters for filtering
	filters, err := h.parseAuditFilters(r)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Invalid filters: %v", err))
		return
	}

	// Get audit logs
	logs, totalCount, err := h.auditService.GetAuditLogs(r.Context(), filters)
	if err != nil {
		h.logger.Error("Failed to get audit logs", logging.Err(err))
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve audit logs")
		return
	}

	// Prepare response
	response := map[string]interface{}{
		"logs":        logs,
		"total_count": totalCount,
		"page_size":   filters.Limit,
		"page":        filters.Offset / max(1, filters.Limit),
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// handleAuditLogDetail handles GET /api/v1/audit/{id} - get specific audit log
func (h *AuditLogHandler) handleAuditLogDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract ID from URL path
	path := r.URL.Path
	if len(path) < len("/api/v1/audit/") {
		h.writeErrorResponse(w, http.StatusBadRequest, "Missing audit log ID")
		return
	}

	idStr := path[len("/api/v1/audit/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid audit log ID")
		return
	}

	// TODO: Implement GetByID method in AuditService
	// This would require adding a GetByID method to AuditService
	h.writeErrorResponse(w, http.StatusNotImplemented,
		fmt.Sprintf("Individual audit log retrieval for ID %d not yet implemented", id))
}

// handleAuditStats handles GET /api/v1/audit/stats - get audit statistics
func (h *AuditLogHandler) handleAuditStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	stats := h.auditService.GetStats()
	h.writeJSONResponse(w, http.StatusOK, stats)
}

// handleAuditCleanup handles POST /api/v1/audit/cleanup - manually trigger cleanup
func (h *AuditLogHandler) handleAuditCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	cleanedCount, err := h.auditService.CleanupOldLogs(r.Context())
	if err != nil {
		h.logger.Error("Failed to cleanup audit logs", logging.Err(err))
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to cleanup audit logs")
		return
	}

	response := map[string]interface{}{
		"cleaned_count": cleanedCount,
		"message":       fmt.Sprintf("Successfully cleaned up %d audit log entries", cleanedCount),
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// parseAuditFilters parses query parameters into audit log filters
func (h *AuditLogHandler) parseAuditFilters(r *http.Request) (service.AuditLogFilters, error) {
	filters := service.AuditLogFilters{
		Limit:  25, // Default page size
		Offset: 0,  // Default offset
	}

	query := r.URL.Query()

	// Parse action filter
	if actionStr := query.Get("action"); actionStr != "" {
		action := models.ActionType(actionStr)
		if action != models.ActionTypeAllow && action != models.ActionTypeBlock {
			return filters, fmt.Errorf("invalid action: %s", actionStr)
		}
		filters.Action = &action
	}

	// Parse target type filter
	if targetTypeStr := query.Get("target_type"); targetTypeStr != "" {
		targetType := models.TargetType(targetTypeStr)
		if targetType != models.TargetTypeExecutable && targetType != models.TargetTypeURL {
			return filters, fmt.Errorf("invalid target_type: %s", targetTypeStr)
		}
		filters.TargetType = &targetType
	}

	// Parse event type filter
	if eventType := query.Get("event_type"); eventType != "" {
		filters.EventType = eventType
	}

	// Parse time range filters
	if startTimeStr := query.Get("start_time"); startTimeStr != "" {
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return filters, fmt.Errorf("invalid start_time format: %v", err)
		}
		filters.StartTime = &startTime
	}

	if endTimeStr := query.Get("end_time"); endTimeStr != "" {
		endTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			return filters, fmt.Errorf("invalid end_time format: %v", err)
		}
		filters.EndTime = &endTime
	}

	// Parse search filter
	if search := query.Get("search"); search != "" {
		filters.Search = search
	}

	// Parse pagination parameters
	if limitStr := query.Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 || limit > 1000 {
			return filters, fmt.Errorf("invalid limit: must be between 1 and 1000")
		}
		filters.Limit = limit
	}

	if offsetStr := query.Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			return filters, fmt.Errorf("invalid offset: must be non-negative")
		}
		filters.Offset = offset
	}

	return filters, nil
}

// writeJSONResponse writes a JSON response
func (h *AuditLogHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response", logging.Err(err))
	}
}

// writeErrorResponse writes an error response
func (h *AuditLogHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := map[string]interface{}{
		"error":   true,
		"message": message,
		"status":  statusCode,
	}

	h.writeJSONResponse(w, statusCode, response)
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
