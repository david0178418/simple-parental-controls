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

// RetentionHandler handles retention policy API endpoints
type RetentionHandler struct {
	retentionService *service.RetentionService
	logger           logging.Logger
}

// NewRetentionHandler creates a new retention handler
func NewRetentionHandler(retentionService *service.RetentionService, logger logging.Logger) *RetentionHandler {
	return &RetentionHandler{
		retentionService: retentionService,
		logger:           logger,
	}
}

// RegisterRoutes registers retention policy API routes
func (h *RetentionHandler) RegisterRoutes(mux *http.ServeMux) {
	// Retention policies
	mux.HandleFunc("/api/v1/retention/policies", h.handleRetentionPolicies)
	mux.HandleFunc("/api/v1/retention/policies/", h.handleRetentionPolicyDetail)

	// Policy execution
	mux.HandleFunc("/api/v1/retention/execute", h.handleExecutePolicies)
	mux.HandleFunc("/api/v1/retention/execute/", h.handleExecutePolicy)
	mux.HandleFunc("/api/v1/retention/preview/", h.handlePreviewPolicy)

	// Statistics and monitoring
	mux.HandleFunc("/api/v1/retention/stats", h.handleRetentionStats)
	mux.HandleFunc("/api/v1/retention/executions", h.handleRetentionExecutions)
}

// handleRetentionPolicies handles GET /api/v1/retention/policies and POST /api/v1/retention/policies
func (h *RetentionHandler) handleRetentionPolicies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getRetentionPolicies(w, r)
	case http.MethodPost:
		h.createRetentionPolicy(w, r)
	default:
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleRetentionPolicyDetail handles GET/PUT/DELETE /api/v1/retention/policies/{id}
func (h *RetentionHandler) handleRetentionPolicyDetail(w http.ResponseWriter, r *http.Request) {
	// Extract ID from URL path
	path := r.URL.Path
	if len(path) < len("/api/v1/retention/policies/") {
		h.writeErrorResponse(w, http.StatusBadRequest, "Missing policy ID")
		return
	}

	idStr := path[len("/api/v1/retention/policies/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid policy ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getRetentionPolicy(w, r, id)
	case http.MethodPut:
		h.updateRetentionPolicy(w, r, id)
	case http.MethodDelete:
		h.deleteRetentionPolicy(w, r, id)
	default:
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleExecutePolicies handles POST /api/v1/retention/execute - execute all policies
func (h *RetentionHandler) handleExecutePolicies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	executions, err := h.retentionService.ExecuteAllPolicies(r.Context())
	if err != nil {
		h.logger.Error("Failed to execute all retention policies", logging.Err(err))
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to execute retention policies")
		return
	}

	response := map[string]interface{}{
		"message":    "Retention policies executed successfully",
		"executions": executions,
		"count":      len(executions),
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// handleExecutePolicy handles POST /api/v1/retention/execute/{id} - execute specific policy
func (h *RetentionHandler) handleExecutePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract ID from URL path
	path := r.URL.Path
	if len(path) < len("/api/v1/retention/execute/") {
		h.writeErrorResponse(w, http.StatusBadRequest, "Missing policy ID")
		return
	}

	idStr := path[len("/api/v1/retention/execute/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid policy ID")
		return
	}

	execution, err := h.retentionService.ExecutePolicy(r.Context(), id)
	if err != nil {
		h.logger.Error("Failed to execute retention policy",
			logging.Int("policy_id", id),
			logging.Err(err))
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to execute retention policy")
		return
	}

	response := map[string]interface{}{
		"message":   "Retention policy executed successfully",
		"execution": execution,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// handlePreviewPolicy handles GET /api/v1/retention/preview/{id} - preview policy execution
func (h *RetentionHandler) handlePreviewPolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract ID from URL path
	path := r.URL.Path
	if len(path) < len("/api/v1/retention/preview/") {
		h.writeErrorResponse(w, http.StatusBadRequest, "Missing policy ID")
		return
	}

	idStr := path[len("/api/v1/retention/preview/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid policy ID")
		return
	}

	preview, err := h.retentionService.PreviewPolicyExecution(r.Context(), id)
	if err != nil {
		h.logger.Error("Failed to preview retention policy",
			logging.Int("policy_id", id),
			logging.Err(err))
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to preview retention policy")
		return
	}

	h.writeJSONResponse(w, http.StatusOK, preview)
}

// handleRetentionStats handles GET /api/v1/retention/stats
func (h *RetentionHandler) handleRetentionStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	stats := h.retentionService.GetStats()
	h.writeJSONResponse(w, http.StatusOK, stats)
}

// handleRetentionExecutions handles GET /api/v1/retention/executions
func (h *RetentionHandler) handleRetentionExecutions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// This would require implementing GetRecentExecutions in the service
	// For now, return a placeholder response
	response := map[string]interface{}{
		"message":    "Retention executions endpoint not yet fully implemented",
		"executions": []interface{}{},
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// Individual handler methods

func (h *RetentionHandler) getRetentionPolicies(w http.ResponseWriter, r *http.Request) {
	// This would require implementing GetAllPolicies in the service
	// For now, return a placeholder response
	response := map[string]interface{}{
		"message":  "Get retention policies endpoint not yet fully implemented",
		"policies": []interface{}{},
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *RetentionHandler) createRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	var policyRequest CreateRetentionPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&policyRequest); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate the request
	if err := policyRequest.Validate(); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Validation failed: %v", err))
		return
	}

	// Convert request to policy model
	policy := policyRequest.ToModel()

	// This would require implementing CreatePolicy in the service
	// For now, return a placeholder response
	response := map[string]interface{}{
		"message": "Create retention policy endpoint not yet fully implemented",
		"policy":  policy,
	}

	h.writeJSONResponse(w, http.StatusCreated, response)
}

func (h *RetentionHandler) getRetentionPolicy(w http.ResponseWriter, r *http.Request, id int) {
	// This would require implementing GetPolicy in the service
	// For now, return a placeholder response
	response := map[string]interface{}{
		"message":   "Get retention policy endpoint not yet fully implemented",
		"policy_id": id,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *RetentionHandler) updateRetentionPolicy(w http.ResponseWriter, r *http.Request, id int) {
	var policyRequest UpdateRetentionPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&policyRequest); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// This would require implementing UpdatePolicy in the service
	// For now, return a placeholder response
	response := map[string]interface{}{
		"message":   "Update retention policy endpoint not yet fully implemented",
		"policy_id": id,
		"request":   policyRequest,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *RetentionHandler) deleteRetentionPolicy(w http.ResponseWriter, r *http.Request, id int) {
	// This would require implementing DeletePolicy in the service
	// For now, return a placeholder response
	response := map[string]interface{}{
		"message":   "Delete retention policy endpoint not yet fully implemented",
		"policy_id": id,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// Request/Response models

// CreateRetentionPolicyRequest represents a request to create a retention policy
type CreateRetentionPolicyRequest struct {
	Name        string `json:"name" validate:"required,max=100"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"`

	// Policy rules
	TimeBasedRule  *TimeBasedRetentionRequest  `json:"time_based_rule,omitempty"`
	SizeBasedRule  *SizeBasedRetentionRequest  `json:"size_based_rule,omitempty"`
	CountBasedRule *CountBasedRetentionRequest `json:"count_based_rule,omitempty"`

	// Event filtering
	EventTypeFilter []string `json:"event_type_filter"`
	ActionFilter    []string `json:"action_filter"`

	// Execution settings
	ExecutionSchedule string `json:"execution_schedule"`
}

// TimeBasedRetentionRequest represents a time-based retention rule request
type TimeBasedRetentionRequest struct {
	MaxAgeDays      int                   `json:"max_age_days" validate:"min=1"`
	GracePeriodDays int                   `json:"grace_period_days" validate:"min=0"`
	TierRules       []TimeTierRuleRequest `json:"tier_rules,omitempty"`
}

// TimeTierRuleRequest represents a time tier rule request
type TimeTierRuleRequest struct {
	AfterDays int     `json:"after_days" validate:"min=1"`
	Action    string  `json:"action" validate:"required,oneof=delete archive compress sample"`
	KeepRatio float64 `json:"keep_ratio,omitempty" validate:"min=0,max=1"`
}

// SizeBasedRetentionRequest represents a size-based retention rule request
type SizeBasedRetentionRequest struct {
	MaxTotalSizeMB  int64   `json:"max_total_size_mb" validate:"min=1"`
	MaxFileSizeMB   int64   `json:"max_file_size_mb,omitempty" validate:"min=0"`
	CleanupStrategy string  `json:"cleanup_strategy" validate:"required,oneof=oldest_first largest_first random proportional"`
	CleanupRatio    float64 `json:"cleanup_ratio" validate:"min=0,max=1"`
}

// CountBasedRetentionRequest represents a count-based retention rule request
type CountBasedRetentionRequest struct {
	MaxCount         int64  `json:"max_count" validate:"min=1"`
	CleanupBatchSize int    `json:"cleanup_batch_size" validate:"min=1"`
	CleanupStrategy  string `json:"cleanup_strategy" validate:"required,oneof=oldest_first random round_robin"`
}

// UpdateRetentionPolicyRequest represents a request to update a retention policy
type UpdateRetentionPolicyRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
	Priority    *int    `json:"priority,omitempty"`

	// Policy rules
	TimeBasedRule  *TimeBasedRetentionRequest  `json:"time_based_rule,omitempty"`
	SizeBasedRule  *SizeBasedRetentionRequest  `json:"size_based_rule,omitempty"`
	CountBasedRule *CountBasedRetentionRequest `json:"count_based_rule,omitempty"`

	// Event filtering
	EventTypeFilter *[]string `json:"event_type_filter,omitempty"`
	ActionFilter    *[]string `json:"action_filter,omitempty"`

	// Execution settings
	ExecutionSchedule *string `json:"execution_schedule,omitempty"`
}

// Validation methods

// Validate validates the create retention policy request
func (req *CreateRetentionPolicyRequest) Validate() error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}

	// At least one retention rule must be specified
	if req.TimeBasedRule == nil && req.SizeBasedRule == nil && req.CountBasedRule == nil {
		return fmt.Errorf("at least one retention rule must be specified")
	}

	// Validate individual rules
	if req.TimeBasedRule != nil {
		if err := req.TimeBasedRule.Validate(); err != nil {
			return fmt.Errorf("time-based rule validation failed: %w", err)
		}
	}

	if req.SizeBasedRule != nil {
		if err := req.SizeBasedRule.Validate(); err != nil {
			return fmt.Errorf("size-based rule validation failed: %w", err)
		}
	}

	if req.CountBasedRule != nil {
		if err := req.CountBasedRule.Validate(); err != nil {
			return fmt.Errorf("count-based rule validation failed: %w", err)
		}
	}

	return nil
}

// Validate validates the time-based retention request
func (req *TimeBasedRetentionRequest) Validate() error {
	if req.MaxAgeDays <= 0 {
		return fmt.Errorf("max_age_days must be positive")
	}

	if req.GracePeriodDays < 0 {
		return fmt.Errorf("grace_period_days cannot be negative")
	}

	for i, tier := range req.TierRules {
		if tier.AfterDays <= 0 {
			return fmt.Errorf("tier rule %d: after_days must be positive", i)
		}
		if tier.Action == "sample" && (tier.KeepRatio < 0 || tier.KeepRatio > 1) {
			return fmt.Errorf("tier rule %d: keep_ratio must be between 0.0 and 1.0", i)
		}
	}

	return nil
}

// Validate validates the size-based retention request
func (req *SizeBasedRetentionRequest) Validate() error {
	if req.MaxTotalSizeMB <= 0 {
		return fmt.Errorf("max_total_size_mb must be positive")
	}

	if req.MaxFileSizeMB < 0 {
		return fmt.Errorf("max_file_size_mb cannot be negative")
	}

	if req.CleanupRatio < 0 || req.CleanupRatio > 1 {
		return fmt.Errorf("cleanup_ratio must be between 0.0 and 1.0")
	}

	return nil
}

// Validate validates the count-based retention request
func (req *CountBasedRetentionRequest) Validate() error {
	if req.MaxCount <= 0 {
		return fmt.Errorf("max_count must be positive")
	}

	if req.CleanupBatchSize <= 0 {
		return fmt.Errorf("cleanup_batch_size must be positive")
	}

	return nil
}

// ToModel converts the create request to a retention policy model
func (req *CreateRetentionPolicyRequest) ToModel() *models.RetentionPolicy {
	policy := &models.RetentionPolicy{
		Name:              req.Name,
		Description:       req.Description,
		Enabled:           req.Enabled,
		Priority:          req.Priority,
		EventTypeFilter:   req.EventTypeFilter,
		ActionFilter:      req.ActionFilter,
		ExecutionSchedule: req.ExecutionSchedule,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// Convert time-based rule
	if req.TimeBasedRule != nil {
		policy.TimeBasedRule = &models.TimeBasedRetention{
			MaxAge:      time.Duration(req.TimeBasedRule.MaxAgeDays) * 24 * time.Hour,
			GracePeriod: time.Duration(req.TimeBasedRule.GracePeriodDays) * 24 * time.Hour,
		}

		for _, tierReq := range req.TimeBasedRule.TierRules {
			tier := models.TimeTierRule{
				AfterDuration: time.Duration(tierReq.AfterDays) * 24 * time.Hour,
				Action:        models.TierAction(tierReq.Action),
				KeepRatio:     tierReq.KeepRatio,
			}
			policy.TimeBasedRule.TierRules = append(policy.TimeBasedRule.TierRules, tier)
		}
	}

	// Convert size-based rule
	if req.SizeBasedRule != nil {
		policy.SizeBasedRule = &models.SizeBasedRetention{
			MaxTotalSize:    req.SizeBasedRule.MaxTotalSizeMB * 1024 * 1024, // Convert MB to bytes
			MaxFileSize:     req.SizeBasedRule.MaxFileSizeMB * 1024 * 1024,  // Convert MB to bytes
			CleanupStrategy: models.SizeCleanupStrategy(req.SizeBasedRule.CleanupStrategy),
			CleanupRatio:    req.SizeBasedRule.CleanupRatio,
		}
	}

	// Convert count-based rule
	if req.CountBasedRule != nil {
		policy.CountBasedRule = &models.CountBasedRetention{
			MaxCount:         req.CountBasedRule.MaxCount,
			CleanupBatchSize: req.CountBasedRule.CleanupBatchSize,
			CleanupStrategy:  models.CountCleanupStrategy(req.CountBasedRule.CleanupStrategy),
		}
	}

	return policy
}

// Helper methods

func (h *RetentionHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response", logging.Err(err))
	}
}

func (h *RetentionHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := map[string]interface{}{
		"error":   true,
		"message": message,
		"status":  statusCode,
	}

	h.writeJSONResponse(w, statusCode, response)
}
