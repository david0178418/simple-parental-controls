package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"parental-control/internal/logging"
	"parental-control/internal/service"
)

// PerformanceHandler handles performance monitoring API endpoints
type PerformanceHandler struct {
	performanceMonitor *service.PerformanceMonitor
	logger             logging.Logger
}

// NewPerformanceHandler creates a new performance handler
func NewPerformanceHandler(performanceMonitor *service.PerformanceMonitor, logger logging.Logger) *PerformanceHandler {
	return &PerformanceHandler{
		performanceMonitor: performanceMonitor,
		logger:             logger,
	}
}

// RegisterRoutes registers performance monitoring API routes
func (h *PerformanceHandler) RegisterRoutes(mux *http.ServeMux) {
	// Performance metrics and monitoring
	mux.HandleFunc("/api/v1/performance/metrics", h.handlePerformanceMetrics)
	mux.HandleFunc("/api/v1/performance/report", h.handlePerformanceReport)
	mux.HandleFunc("/api/v1/performance/alerts", h.handlePerformanceAlerts)
	mux.HandleFunc("/api/v1/performance/thresholds", h.handlePerformanceThresholds)
	mux.HandleFunc("/api/v1/performance/thresholds/", h.handlePerformanceThresholdDetail)
	mux.HandleFunc("/api/v1/performance/health", h.handlePerformanceHealth)
}

// handlePerformanceMetrics handles GET /api/v1/performance/metrics
func (h *PerformanceHandler) handlePerformanceMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	metrics := h.performanceMonitor.GetCurrentMetrics()
	if metrics == nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve performance metrics")
		return
	}

	h.writeJSONResponse(w, http.StatusOK, metrics)
}

// handlePerformanceReport handles GET /api/v1/performance/report
func (h *PerformanceHandler) handlePerformanceReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	report := h.performanceMonitor.GetPerformanceReport()
	if report == nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to generate performance report")
		return
	}

	h.writeJSONResponse(w, http.StatusOK, report)
}

// handlePerformanceAlerts handles GET /api/v1/performance/alerts
func (h *PerformanceHandler) handlePerformanceAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	alerts := h.performanceMonitor.GetActiveAlerts()

	response := map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// handlePerformanceThresholds handles GET /api/v1/performance/thresholds and POST /api/v1/performance/thresholds
func (h *PerformanceHandler) handlePerformanceThresholds(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getPerformanceThresholds(w, r)
	case http.MethodPost:
		h.createPerformanceThreshold(w, r)
	default:
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handlePerformanceThresholdDetail handles DELETE /api/v1/performance/thresholds/{name}
func (h *PerformanceHandler) handlePerformanceThresholdDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract threshold name from URL path
	path := r.URL.Path
	if len(path) < len("/api/v1/performance/thresholds/") {
		h.writeErrorResponse(w, http.StatusBadRequest, "Missing threshold name")
		return
	}

	thresholdName := path[len("/api/v1/performance/thresholds/"):]
	if thresholdName == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Missing threshold name")
		return
	}

	h.performanceMonitor.RemoveThreshold(thresholdName)

	response := map[string]string{
		"message": "Threshold removed successfully",
		"name":    thresholdName,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// handlePerformanceHealth handles GET /api/v1/performance/health
func (h *PerformanceHandler) handlePerformanceHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	report := h.performanceMonitor.GetPerformanceReport()
	if report == nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to generate health report")
		return
	}

	healthResponse := map[string]interface{}{
		"health_score":    report.HealthScore,
		"status":          h.getHealthStatus(report.HealthScore),
		"active_alerts":   len(report.ActiveAlerts),
		"recommendations": report.Recommendations,
		"last_updated":    report.GeneratedAt,
	}

	h.writeJSONResponse(w, http.StatusOK, healthResponse)
}

// Individual handler methods

func (h *PerformanceHandler) getPerformanceThresholds(w http.ResponseWriter, r *http.Request) {
	// Since we can't access the internal thresholds map directly,
	// we'll return a message indicating the feature availability
	response := map[string]interface{}{
		"message": "Performance thresholds are managed internally. Use POST to add new thresholds.",
		"example_threshold": map[string]interface{}{
			"name":        "custom_threshold",
			"metric_path": "memory_usage_bytes",
			"threshold":   512000000, // 512MB
			"operator":    "gt",
			"severity":    "warning",
			"description": "Custom memory usage threshold",
		},
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *PerformanceHandler) createPerformanceThreshold(w http.ResponseWriter, r *http.Request) {
	var threshold service.PerformanceThreshold

	if err := json.NewDecoder(r.Body).Decode(&threshold); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON format")
		return
	}

	// Validate threshold
	if err := h.validateThreshold(&threshold); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	h.performanceMonitor.AddThreshold(threshold)

	response := map[string]interface{}{
		"message":   "Threshold added successfully",
		"threshold": threshold,
	}

	h.writeJSONResponse(w, http.StatusCreated, response)
}

func (h *PerformanceHandler) validateThreshold(threshold *service.PerformanceThreshold) error {
	if threshold.Name == "" {
		return fmt.Errorf("threshold name is required")
	}

	if threshold.MetricPath == "" {
		return fmt.Errorf("metric path is required")
	}

	if threshold.Threshold == 0 {
		return fmt.Errorf("threshold value must be non-zero")
	}

	validOperators := map[string]bool{"gt": true, "lt": true, "eq": true}
	if !validOperators[threshold.Operator] {
		return fmt.Errorf("operator must be one of: gt, lt, eq")
	}

	validSeverities := map[string]bool{"info": true, "warning": true, "critical": true}
	if !validSeverities[threshold.Severity] {
		return fmt.Errorf("severity must be one of: info, warning, critical")
	}

	return nil
}

func (h *PerformanceHandler) getHealthStatus(healthScore float64) string {
	if healthScore >= 90 {
		return "excellent"
	} else if healthScore >= 80 {
		return "good"
	} else if healthScore >= 70 {
		return "fair"
	} else if healthScore >= 50 {
		return "poor"
	}
	return "critical"
}

// Utility methods

func (h *PerformanceHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response", logging.Err(err))
	}
}

func (h *PerformanceHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := map[string]string{
		"error":  message,
		"status": strconv.Itoa(statusCode),
	}

	h.writeJSONResponse(w, statusCode, response)
}
