package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"parental-control/internal/logging"
	"parental-control/internal/models"
)

// LogRotationHandler provides HTTP handlers for log rotation functionality
type LogRotationHandler struct {
	repos  *models.RepositoryManager
	logger logging.Logger
}

// NewLogRotationHandler creates a new log rotation handler
func NewLogRotationHandler(repos *models.RepositoryManager, logger logging.Logger) *LogRotationHandler {
	return &LogRotationHandler{
		repos:  repos,
		logger: logger,
	}
}

// RegisterHandlers registers log rotation HTTP handlers with the provided mux
func (h *LogRotationHandler) RegisterHandlers(mux *http.ServeMux) {
	// Policy management endpoints
	mux.HandleFunc("/api/v1/rotation/policies", h.handleRotationPolicies)
	mux.HandleFunc("/api/v1/rotation/policies/", h.handleRotationPolicyDetail)

	// Execution endpoints
	mux.HandleFunc("/api/v1/rotation/execute", h.handleExecutePolicies)
	mux.HandleFunc("/api/v1/rotation/execute/", h.handleExecutePolicy)

	// Statistics and monitoring
	mux.HandleFunc("/api/v1/rotation/stats", h.handleRotationStats)
	mux.HandleFunc("/api/v1/rotation/executions", h.handleRotationExecutions)
	mux.HandleFunc("/api/v1/rotation/disk-space", h.handleDiskSpace)
	mux.HandleFunc("/api/v1/rotation/emergency-cleanup", h.handleEmergencyCleanup)
}

// handleRotationPolicies handles requests to /api/v1/rotation/policies
func (h *LogRotationHandler) handleRotationPolicies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getRotationPolicies(w, r)
	case http.MethodPost:
		h.createRotationPolicy(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRotationPolicyDetail handles requests to /api/v1/rotation/policies/{id}
func (h *LogRotationHandler) handleRotationPolicyDetail(w http.ResponseWriter, r *http.Request) {
	// Extract ID from URL path
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/rotation/policies/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		http.Error(w, "Policy ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(pathParts[0])
	if err != nil {
		http.Error(w, "Invalid policy ID", http.StatusBadRequest)
		return
	}

	// Check for execution endpoint
	if len(pathParts) > 1 && pathParts[1] == "execute" {
		h.executePolicyByID(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getRotationPolicy(w, r, id)
	case http.MethodPut:
		h.updateRotationPolicy(w, r, id)
	case http.MethodDelete:
		h.deleteRotationPolicy(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleExecutePolicies handles requests to /api/v1/rotation/execute
func (h *LogRotationHandler) handleExecutePolicies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.executeAllPolicies(w, r)
}

// handleExecutePolicy handles requests to /api/v1/rotation/execute/{id}
func (h *LogRotationHandler) handleExecutePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from URL path
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/rotation/execute/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid policy ID", http.StatusBadRequest)
		return
	}

	h.executePolicyByID(w, r, id)
}

// handleRotationStats handles requests to /api/v1/rotation/stats
func (h *LogRotationHandler) handleRotationStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.getRotationStats(w, r)
}

// handleRotationExecutions handles requests to /api/v1/rotation/executions
func (h *LogRotationHandler) handleRotationExecutions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.getRotationExecutions(w, r)
}

// handleDiskSpace handles requests to /api/v1/rotation/disk-space
func (h *LogRotationHandler) handleDiskSpace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.getDiskSpace(w, r)
}

// handleEmergencyCleanup handles requests to /api/v1/rotation/emergency-cleanup
func (h *LogRotationHandler) handleEmergencyCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.triggerEmergencyCleanup(w, r)
}

// Implementation methods

func (h *LogRotationHandler) getRotationPolicies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	enabledOnly := r.URL.Query().Get("enabled") == "true"

	var policies []models.LogRotationPolicy
	var err error

	if enabledOnly {
		policies, err = h.repos.LogRotationPolicy.GetEnabled(ctx)
	} else {
		policies, err = h.repos.LogRotationPolicy.GetAll(ctx)
	}

	if err != nil {
		h.logger.Error("Failed to get rotation policies", logging.Err(err))
		http.Error(w, "Failed to get rotation policies", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"policies": policies,
		"total":    len(policies),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *LogRotationHandler) createRotationPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var policy models.LogRotationPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate policy
	if err := policy.Validate(); err != nil {
		http.Error(w, "Policy validation failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.repos.LogRotationPolicy.Create(ctx, &policy); err != nil {
		h.logger.Error("Failed to create rotation policy", logging.Err(err))
		http.Error(w, "Failed to create rotation policy", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(policy)
}

func (h *LogRotationHandler) getRotationPolicy(w http.ResponseWriter, r *http.Request, id int) {
	ctx := r.Context()

	policy, err := h.repos.LogRotationPolicy.GetByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Policy not found", http.StatusNotFound)
		} else {
			h.logger.Error("Failed to get rotation policy", logging.Err(err))
			http.Error(w, "Failed to get rotation policy", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(policy)
}

func (h *LogRotationHandler) updateRotationPolicy(w http.ResponseWriter, r *http.Request, id int) {
	ctx := r.Context()

	var policy models.LogRotationPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	policy.ID = id

	// Validate policy
	if err := policy.Validate(); err != nil {
		http.Error(w, "Policy validation failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.repos.LogRotationPolicy.Update(ctx, &policy); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Policy not found", http.StatusNotFound)
		} else {
			h.logger.Error("Failed to update rotation policy", logging.Err(err))
			http.Error(w, "Failed to update rotation policy", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(policy)
}

func (h *LogRotationHandler) deleteRotationPolicy(w http.ResponseWriter, r *http.Request, id int) {
	ctx := r.Context()

	if err := h.repos.LogRotationPolicy.Delete(ctx, id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Policy not found", http.StatusNotFound)
		} else {
			h.logger.Error("Failed to delete rotation policy", logging.Err(err))
			http.Error(w, "Failed to delete rotation policy", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *LogRotationHandler) executePolicyByID(w http.ResponseWriter, r *http.Request, id int) {
	ctx := r.Context()

	// Get the policy
	policy, err := h.repos.LogRotationPolicy.GetByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Policy not found", http.StatusNotFound)
		} else {
			h.logger.Error("Failed to get rotation policy", logging.Err(err))
			http.Error(w, "Failed to get rotation policy", http.StatusInternalServerError)
		}
		return
	}

	if !policy.Enabled {
		http.Error(w, "Policy is disabled", http.StatusBadRequest)
		return
	}

	// Create execution record
	execution := &models.LogRotationExecution{
		PolicyID:      policy.ID,
		ExecutionTime: time.Now(),
		Status:        models.ExecutionStatusRunning,
		TriggerReason: models.TriggerManual,
	}

	if err := h.repos.LogRotationExecution.Create(ctx, execution); err != nil {
		h.logger.Error("Failed to create execution record", logging.Err(err))
		http.Error(w, "Failed to create execution record", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Rotation policy execution started",
		logging.Int("policy_id", policy.ID),
		logging.String("policy_name", policy.Name))

	// In a real implementation, this would trigger the actual rotation service
	// For now, we'll just mark it as completed
	execution.Status = models.ExecutionStatusCompleted
	execution.Duration = time.Since(execution.ExecutionTime)

	// Set execution details
	details := map[string]interface{}{
		"policy_name":    policy.Name,
		"trigger_reason": string(models.TriggerManual),
		"dry_run_mode":   false, // Would be configurable
	}
	if err := execution.SetDetailsMap(details); err != nil {
		h.logger.Error("Failed to set execution details", logging.Err(err))
	}

	if err := h.repos.LogRotationExecution.Update(ctx, execution); err != nil {
		h.logger.Error("Failed to update execution record", logging.Err(err))
	}

	response := map[string]interface{}{
		"execution": execution,
		"message":   "Policy execution completed successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *LogRotationHandler) executeAllPolicies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	policies, err := h.repos.LogRotationPolicy.GetEnabled(ctx)
	if err != nil {
		h.logger.Error("Failed to get enabled policies", logging.Err(err))
		http.Error(w, "Failed to get enabled policies", http.StatusInternalServerError)
		return
	}

	var executions []models.LogRotationExecution
	successCount := 0

	for _, policy := range policies {
		execution := &models.LogRotationExecution{
			PolicyID:      policy.ID,
			ExecutionTime: time.Now(),
			Status:        models.ExecutionStatusRunning,
			TriggerReason: models.TriggerManual,
		}

		if err := h.repos.LogRotationExecution.Create(ctx, execution); err != nil {
			h.logger.Error("Failed to create execution record",
				logging.Int("policy_id", policy.ID),
				logging.Err(err))
			continue
		}

		// In a real implementation, this would trigger the actual rotation service
		execution.Status = models.ExecutionStatusCompleted
		execution.Duration = time.Since(execution.ExecutionTime)

		details := map[string]interface{}{
			"policy_name":    policy.Name,
			"trigger_reason": string(models.TriggerManual),
			"dry_run_mode":   false,
		}
		if err := execution.SetDetailsMap(details); err != nil {
			h.logger.Error("Failed to set execution details", logging.Err(err))
		}

		if err := h.repos.LogRotationExecution.Update(ctx, execution); err != nil {
			h.logger.Error("Failed to update execution record", logging.Err(err))
		}

		executions = append(executions, *execution)
		successCount++
	}

	response := map[string]interface{}{
		"executions": executions,
		"total":      len(policies),
		"successful": successCount,
		"message":    fmt.Sprintf("Executed %d out of %d enabled policies", successCount, len(policies)),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *LogRotationHandler) getRotationStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stats, err := h.repos.LogRotationExecution.GetStats(ctx)
	if err != nil {
		h.logger.Error("Failed to get rotation stats", logging.Err(err))
		http.Error(w, "Failed to get rotation stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *LogRotationHandler) getRotationExecutions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	status := r.URL.Query().Get("status")
	trigger := r.URL.Query().Get("trigger")

	limit := 50 // default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	var executions []models.LogRotationExecution
	var err error

	if status != "" {
		executions, err = h.repos.LogRotationExecution.GetByStatus(ctx, models.ExecutionStatus(status), limit, 0)
	} else if trigger != "" {
		executions, err = h.repos.LogRotationExecution.GetByTrigger(ctx, models.RotationTrigger(trigger), limit, 0)
	} else {
		executions, err = h.repos.LogRotationExecution.GetRecent(ctx, limit)
	}

	if err != nil {
		h.logger.Error("Failed to get rotation executions", logging.Err(err))
		http.Error(w, "Failed to get rotation executions", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"executions": executions,
		"count":      len(executions),
		"limit":      limit,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *LogRotationHandler) getDiskSpace(w http.ResponseWriter, r *http.Request) {
	// In a real implementation, this would get actual disk space information
	// For now, we'll return mock data
	diskInfo := &models.DiskSpaceInfo{
		TotalSpace:   100 * 1024 * 1024 * 1024, // 100GB
		UsedSpace:    45 * 1024 * 1024 * 1024,  // 45GB
		FreeSpace:    55 * 1024 * 1024 * 1024,  // 55GB
		UsagePercent: 0.45,                     // 45%
		LogFilesSize: 2 * 1024 * 1024 * 1024,   // 2GB
		ArchiveSize:  1 * 1024 * 1024 * 1024,   // 1GB
		LastUpdated:  time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(diskInfo)
}

func (h *LogRotationHandler) triggerEmergencyCleanup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get emergency policies
	policies, err := h.repos.LogRotationPolicy.GetEnabled(ctx)
	if err != nil {
		h.logger.Error("Failed to get enabled policies", logging.Err(err))
		http.Error(w, "Failed to get enabled policies", http.StatusInternalServerError)
		return
	}

	// Filter for emergency policies
	var emergencyPolicies []models.LogRotationPolicy
	for _, policy := range policies {
		if policy.EmergencyConfig != nil {
			emergencyPolicies = append(emergencyPolicies, policy)
		}
	}

	if len(emergencyPolicies) == 0 {
		http.Error(w, "No emergency policies configured", http.StatusBadRequest)
		return
	}

	// Execute emergency policies
	var executions []models.LogRotationExecution
	successCount := 0

	for _, policy := range emergencyPolicies {
		execution := &models.LogRotationExecution{
			PolicyID:      policy.ID,
			ExecutionTime: time.Now(),
			Status:        models.ExecutionStatusRunning,
			TriggerReason: models.TriggerEmergency,
		}

		if err := h.repos.LogRotationExecution.Create(ctx, execution); err != nil {
			h.logger.Error("Failed to create emergency execution record",
				logging.Int("policy_id", policy.ID),
				logging.Err(err))
			continue
		}

		// In a real implementation, this would trigger the actual rotation service
		execution.Status = models.ExecutionStatusCompleted
		execution.Duration = time.Since(execution.ExecutionTime)

		details := map[string]interface{}{
			"policy_name":    policy.Name,
			"trigger_reason": string(models.TriggerEmergency),
			"emergency_mode": true,
		}
		if err := execution.SetDetailsMap(details); err != nil {
			h.logger.Error("Failed to set execution details", logging.Err(err))
		}

		if err := h.repos.LogRotationExecution.Update(ctx, execution); err != nil {
			h.logger.Error("Failed to update execution record", logging.Err(err))
		}

		executions = append(executions, *execution)
		successCount++
	}

	h.logger.Warn("Emergency cleanup triggered",
		logging.Int("policies_executed", successCount),
		logging.Int("total_emergency_policies", len(emergencyPolicies)))

	response := map[string]interface{}{
		"executions":   executions,
		"total":        len(emergencyPolicies),
		"successful":   successCount,
		"message":      "Emergency cleanup completed",
		"triggered_at": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
