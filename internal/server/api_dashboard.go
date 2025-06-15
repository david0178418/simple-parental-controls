package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"parental-control/internal/logging"
)

// DashboardAPIServer handles dashboard-specific API endpoints
type DashboardAPIServer struct {
	startTime time.Time
}

// NewDashboardAPIServer creates a new dashboard API server instance
func NewDashboardAPIServer() *DashboardAPIServer {
	return &DashboardAPIServer{
		startTime: time.Now(),
	}
}

// DashboardStats represents the dashboard statistics response
type DashboardStats struct {
	TotalLists      int `json:"total_lists"`
	TotalEntries    int `json:"total_entries"`
	ActiveRules     int `json:"active_rules"`
	TodayBlocks     int `json:"today_blocks"`
	TodayAllows     int `json:"today_allows"`
	QuotasNearLimit int `json:"quotas_near_limit"`
}

// SystemInfo represents system information response
type SystemInfo struct {
	Uptime            string `json:"uptime"`
	Version           string `json:"version"`
	Platform          string `json:"platform"`
	MemoryUsage       string `json:"memory_usage"`
	CPUUsage          string `json:"cpu_usage"`
	ActiveConnections int    `json:"active_connections"`
	GoVersion         string `json:"go_version"`
	StartTime         string `json:"start_time"`
}

// RegisterRoutes registers dashboard API endpoints with the server
// Note: This method signature is designed to be called with an auth handler parameter
// when authentication integration is fully set up. For now, we'll use basic middleware.
func (api *DashboardAPIServer) RegisterRoutes(server *Server) {
	// For now, create protected middleware without authentication
	// TODO: Add authentication when auth handlers are properly integrated
	protectedMiddleware := NewMiddlewareChain(
		RequestIDMiddleware(),
		LoggingMiddleware(),
		RecoveryMiddleware(),
		SecurityHeadersMiddleware(),
		JSONMiddleware(),
		// AuthenticationMiddleware will be added when auth integration is complete
	)

	// Register dashboard endpoints
	server.AddHandler("/api/v1/dashboard/stats",
		protectedMiddleware.ThenFunc(api.handleDashboardStats))
	server.AddHandler("/api/v1/dashboard/system",
		protectedMiddleware.ThenFunc(api.handleSystemInfo))

	logging.Info("Dashboard API routes registered",
		logging.String("stats_endpoint", "/api/v1/dashboard/stats"),
		logging.String("system_endpoint", "/api/v1/dashboard/system"))
}

// handleDashboardStats returns dashboard statistics
func (api *DashboardAPIServer) handleDashboardStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// For now, return mock data as the actual data sources will be implemented in future milestones
	stats := DashboardStats{
		TotalLists:      5,
		TotalEntries:    42,
		ActiveRules:     12,
		TodayBlocks:     15,
		TodayAllows:     128,
		QuotasNearLimit: 2,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(stats); err != nil {
		logging.Error("Failed to encode dashboard stats response", logging.Err(err))
		WriteErrorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	logging.Debug("Dashboard stats request completed",
		logging.String("method", r.Method),
		logging.String("path", r.URL.Path))
}

// handleSystemInfo returns system information
func (api *DashboardAPIServer) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Calculate uptime
	uptime := time.Since(api.startTime)
	uptimeStr := formatDuration(uptime)

	// Get memory statistics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	memUsage := formatBytes(memStats.Alloc)

	// Get system information
	systemInfo := SystemInfo{
		Uptime:            uptimeStr,
		Version:           "1.0.0", // TODO: Get from build info
		Platform:          runtime.GOOS + "/" + runtime.GOARCH,
		MemoryUsage:       memUsage,
		CPUUsage:          "N/A", // TODO: Implement CPU usage tracking
		ActiveConnections: 0,     // TODO: Implement connection tracking
		GoVersion:         runtime.Version(),
		StartTime:         api.startTime.Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(systemInfo); err != nil {
		logging.Error("Failed to encode system info response", logging.Err(err))
		WriteErrorResponse(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	logging.Debug("System info request completed",
		logging.String("method", r.Method),
		logging.String("path", r.URL.Path),
		logging.String("uptime", uptimeStr))
}

// formatDuration formats a duration in a human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Truncate(time.Second).String()
	}
	if d < time.Hour {
		return d.Truncate(time.Minute).String()
	}
	if d < 24*time.Hour {
		return d.Truncate(time.Minute).String()
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dd%dh%dm", days, hours, minutes)
}

// formatBytes formats bytes in a human-readable format
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
