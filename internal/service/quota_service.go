package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"parental-control/internal/logging"
	"parental-control/internal/models"
)

// QuotaService provides business logic for managing quota rules and usage tracking
type QuotaService struct {
	repos  *models.RepositoryManager
	logger logging.Logger
}

// NewQuotaService creates a new quota service
func NewQuotaService(repos *models.RepositoryManager, logger logging.Logger) *QuotaService {
	return &QuotaService{
		repos:  repos,
		logger: logger,
	}
}

// CreateQuotaRuleRequest represents a request to create a new quota rule
type CreateQuotaRuleRequest struct {
	ListID       int              `json:"list_id" validate:"required"`
	Name         string           `json:"name" validate:"required,max=255"`
	QuotaType    models.QuotaType `json:"quota_type" validate:"required,oneof=daily weekly monthly"`
	LimitSeconds int              `json:"limit_seconds" validate:"required,min=1"`
	Enabled      bool             `json:"enabled"`
}

// UpdateQuotaRuleRequest represents a request to update an existing quota rule
type UpdateQuotaRuleRequest struct {
	Name         *string           `json:"name,omitempty" validate:"omitempty,max=255"`
	QuotaType    *models.QuotaType `json:"quota_type,omitempty" validate:"omitempty,oneof=daily weekly monthly"`
	LimitSeconds *int              `json:"limit_seconds,omitempty" validate:"omitempty,min=1"`
	Enabled      *bool             `json:"enabled,omitempty"`
}

// QuotaRuleStatus represents the current status of a quota rule
type QuotaRuleStatus struct {
	*models.QuotaRule
	CurrentUsage  *models.QuotaUsage `json:"current_usage"`
	RemainingTime time.Duration      `json:"remaining_time"`
	IsExceeded    bool               `json:"is_exceeded"`
	NextReset     time.Time          `json:"next_reset"`
	WarningLevel  QuotaWarningLevel  `json:"warning_level"`
}

// QuotaWarningLevel represents different warning levels for quota usage
type QuotaWarningLevel string

const (
	WarningLevelNone     QuotaWarningLevel = "none"     // 0-50% used
	WarningLevelLow      QuotaWarningLevel = "low"      // 50-75% used
	WarningLevelMedium   QuotaWarningLevel = "medium"   // 75-90% used
	WarningLevelHigh     QuotaWarningLevel = "high"     // 90-100% used
	WarningLevelExceeded QuotaWarningLevel = "exceeded" // >100% used
)

// UsageTrackingRequest represents a request to track usage
type UsageTrackingRequest struct {
	QuotaRuleID    int           `json:"quota_rule_id" validate:"required"`
	AdditionalTime time.Duration `json:"additional_time" validate:"required"`
}

// UsageSummary provides a summary of quota usage
type UsageSummary struct {
	QuotaRuleID   int               `json:"quota_rule_id"`
	RuleName      string            `json:"rule_name"`
	QuotaType     models.QuotaType  `json:"quota_type"`
	LimitDuration time.Duration     `json:"limit_duration"`
	UsedDuration  time.Duration     `json:"used_duration"`
	RemainingTime time.Duration     `json:"remaining_time"`
	UsagePercent  float64           `json:"usage_percent"`
	IsExceeded    bool              `json:"is_exceeded"`
	NextReset     time.Time         `json:"next_reset"`
	WarningLevel  QuotaWarningLevel `json:"warning_level"`
}

// CreateQuotaRule creates a new quota rule with validation
func (s *QuotaService) CreateQuotaRule(ctx context.Context, req CreateQuotaRuleRequest) (*models.QuotaRule, error) {
	s.logger.Info("Creating new quota rule",
		logging.String("name", req.Name),
		logging.Int("list_id", req.ListID),
		logging.String("quota_type", string(req.QuotaType)),
		logging.Int("limit_seconds", req.LimitSeconds))

	// Validate the request
	if err := s.validateCreateQuotaRuleRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	rule := &models.QuotaRule{
		ListID:       req.ListID,
		Name:         req.Name,
		QuotaType:    req.QuotaType,
		LimitSeconds: req.LimitSeconds,
		Enabled:      req.Enabled,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.repos.QuotaRule.Create(ctx, rule); err != nil {
		s.logger.Error("Failed to create quota rule", logging.Err(err))
		return nil, fmt.Errorf("failed to create quota rule: %w", err)
	}

	s.logger.Info("Quota rule created successfully",
		logging.Int("id", rule.ID),
		logging.String("name", rule.Name))

	return rule, nil
}

// GetQuotaRule retrieves a quota rule by ID
func (s *QuotaService) GetQuotaRule(ctx context.Context, id int) (*models.QuotaRule, error) {
	return s.repos.QuotaRule.GetByID(ctx, id)
}

// GetQuotaRulesByListID retrieves all quota rules for a specific list
func (s *QuotaService) GetQuotaRulesByListID(ctx context.Context, listID int) ([]models.QuotaRule, error) {
	return s.repos.QuotaRule.GetByListID(ctx, listID)
}

// GetQuotaRuleStatus retrieves a quota rule with its current status
func (s *QuotaService) GetQuotaRuleStatus(ctx context.Context, id int) (*QuotaRuleStatus, error) {
	rule, err := s.repos.QuotaRule.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get quota rule: %w", err)
	}

	now := time.Now()
	currentUsage, err := s.repos.QuotaUsage.GetCurrentUsage(ctx, id, now)
	if err != nil {
		s.logger.Error("Failed to get current usage", logging.Err(err))
		// Create empty usage if none exists
		currentUsage = &models.QuotaUsage{
			QuotaRuleID: id,
			UsedSeconds: 0,
			PeriodStart: s.getPeriodStart(rule.QuotaType, now),
			PeriodEnd:   s.getPeriodEnd(rule.QuotaType, now),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
	}

	remainingSeconds := rule.LimitSeconds - currentUsage.UsedSeconds
	if remainingSeconds < 0 {
		remainingSeconds = 0
	}

	remainingTime := time.Duration(remainingSeconds) * time.Second
	isExceeded := currentUsage.UsedSeconds >= rule.LimitSeconds
	nextReset := s.getNextReset(rule.QuotaType, now)
	warningLevel := s.calculateWarningLevel(currentUsage.UsedSeconds, rule.LimitSeconds)

	return &QuotaRuleStatus{
		QuotaRule:     rule,
		CurrentUsage:  currentUsage,
		RemainingTime: remainingTime,
		IsExceeded:    isExceeded,
		NextReset:     nextReset,
		WarningLevel:  warningLevel,
	}, nil
}

// UpdateQuotaRule updates an existing quota rule
func (s *QuotaService) UpdateQuotaRule(ctx context.Context, id int, req UpdateQuotaRuleRequest) (*models.QuotaRule, error) {
	s.logger.Info("Updating quota rule", logging.Int("id", id))

	// Get the existing rule
	rule, err := s.repos.QuotaRule.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get quota rule: %w", err)
	}

	// Apply updates
	if req.Name != nil {
		if err := s.validateQuotaRuleName(ctx, *req.Name, rule.ListID, &id); err != nil {
			return nil, fmt.Errorf("invalid name: %w", err)
		}
		rule.Name = *req.Name
	}
	if req.QuotaType != nil {
		rule.QuotaType = *req.QuotaType
	}
	if req.LimitSeconds != nil {
		if *req.LimitSeconds < 1 {
			return nil, fmt.Errorf("limit must be at least 1 second")
		}
		rule.LimitSeconds = *req.LimitSeconds
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}

	rule.UpdatedAt = time.Now()

	if err := s.repos.QuotaRule.Update(ctx, rule); err != nil {
		s.logger.Error("Failed to update quota rule", logging.Err(err))
		return nil, fmt.Errorf("failed to update quota rule: %w", err)
	}

	s.logger.Info("Quota rule updated successfully", logging.Int("id", id))
	return rule, nil
}

// DeleteQuotaRule deletes a quota rule
func (s *QuotaService) DeleteQuotaRule(ctx context.Context, id int) error {
	s.logger.Info("Deleting quota rule", logging.Int("id", id))

	// Check if rule exists
	rule, err := s.repos.QuotaRule.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get quota rule: %w", err)
	}

	if err := s.repos.QuotaRule.Delete(ctx, id); err != nil {
		s.logger.Error("Failed to delete quota rule", logging.Err(err))
		return fmt.Errorf("failed to delete quota rule: %w", err)
	}

	s.logger.Info("Quota rule deleted successfully",
		logging.Int("id", id),
		logging.String("name", rule.Name))

	return nil
}

// TrackUsage tracks usage against a quota rule
func (s *QuotaService) TrackUsage(ctx context.Context, quotaRuleID int, additionalSeconds int) error {
	s.logger.Debug("Tracking usage",
		logging.Int("quota_rule_id", quotaRuleID),
		logging.Int("additional_seconds", additionalSeconds))

	now := time.Now()

	if err := s.repos.QuotaUsage.UpdateUsage(ctx, quotaRuleID, additionalSeconds, now); err != nil {
		s.logger.Error("Failed to track usage",
			logging.Err(err),
			logging.Int("quota_rule_id", quotaRuleID))
		return fmt.Errorf("failed to track usage: %w", err)
	}

	return nil
}

// CheckQuotaExceeded checks if a quota rule is exceeded
func (s *QuotaService) CheckQuotaExceeded(ctx context.Context, quotaRuleID int) (bool, *QuotaRuleStatus, error) {
	status, err := s.GetQuotaRuleStatus(ctx, quotaRuleID)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get quota status: %w", err)
	}

	return status.IsExceeded, status, nil
}

// GetUsageSummary returns a usage summary for all quota rules in a list
func (s *QuotaService) GetUsageSummary(ctx context.Context, listID int) ([]UsageSummary, error) {
	rules, err := s.repos.QuotaRule.GetByListID(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("failed to get quota rules: %w", err)
	}

	summaries := make([]UsageSummary, 0, len(rules))
	now := time.Now()

	for _, rule := range rules {
		usage, err := s.repos.QuotaUsage.GetCurrentUsage(ctx, rule.ID, now)
		if err != nil {
			s.logger.Error("Failed to get usage for rule", logging.Err(err), logging.Int("rule_id", rule.ID))
			// Create empty usage if none exists
			usage = &models.QuotaUsage{
				QuotaRuleID: rule.ID,
				UsedSeconds: 0,
				PeriodStart: s.getPeriodStart(rule.QuotaType, now),
				PeriodEnd:   s.getPeriodEnd(rule.QuotaType, now),
			}
		}

		limitDuration := rule.GetLimitDuration()
		usedDuration := usage.GetUsedDuration()
		remainingSeconds := rule.LimitSeconds - usage.UsedSeconds
		if remainingSeconds < 0 {
			remainingSeconds = 0
		}
		remainingTime := time.Duration(remainingSeconds) * time.Second

		usagePercent := float64(usage.UsedSeconds) / float64(rule.LimitSeconds) * 100
		if usagePercent > 100 {
			usagePercent = 100
		}

		summaries = append(summaries, UsageSummary{
			QuotaRuleID:   rule.ID,
			RuleName:      rule.Name,
			QuotaType:     rule.QuotaType,
			LimitDuration: limitDuration,
			UsedDuration:  usedDuration,
			RemainingTime: remainingTime,
			UsagePercent:  usagePercent,
			IsExceeded:    usage.UsedSeconds >= rule.LimitSeconds,
			NextReset:     s.getNextReset(rule.QuotaType, now),
			WarningLevel:  s.calculateWarningLevel(usage.UsedSeconds, rule.LimitSeconds),
		})
	}

	return summaries, nil
}

// GetQuotasNearLimit returns quota rules that are near their limits
func (s *QuotaService) GetQuotasNearLimit(ctx context.Context, threshold float64) ([]UsageSummary, error) {
	// Get all enabled quota rules
	rules, err := s.repos.QuotaRule.GetEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled quota rules: %w", err)
	}

	nearLimit := make([]UsageSummary, 0)
	now := time.Now()

	for _, rule := range rules {
		usage, err := s.repos.QuotaUsage.GetCurrentUsage(ctx, rule.ID, now)
		if err != nil {
			continue // Skip if we can't get usage data
		}

		usagePercent := float64(usage.UsedSeconds) / float64(rule.LimitSeconds) * 100

		if usagePercent >= threshold {
			limitDuration := rule.GetLimitDuration()
			usedDuration := usage.GetUsedDuration()
			remainingSeconds := rule.LimitSeconds - usage.UsedSeconds
			if remainingSeconds < 0 {
				remainingSeconds = 0
			}
			remainingTime := time.Duration(remainingSeconds) * time.Second

			nearLimit = append(nearLimit, UsageSummary{
				QuotaRuleID:   rule.ID,
				RuleName:      rule.Name,
				QuotaType:     rule.QuotaType,
				LimitDuration: limitDuration,
				UsedDuration:  usedDuration,
				RemainingTime: remainingTime,
				UsagePercent:  usagePercent,
				IsExceeded:    usage.UsedSeconds >= rule.LimitSeconds,
				NextReset:     s.getNextReset(rule.QuotaType, now),
				WarningLevel:  s.calculateWarningLevel(usage.UsedSeconds, rule.LimitSeconds),
			})
		}
	}

	return nearLimit, nil
}

// ResetQuotaUsage manually resets usage for a quota rule
func (s *QuotaService) ResetQuotaUsage(ctx context.Context, quotaRuleID int) error {
	s.logger.Info("Manually resetting quota usage", logging.Int("quota_rule_id", quotaRuleID))

	// Verify quota rule exists
	if _, err := s.repos.QuotaRule.GetByID(ctx, quotaRuleID); err != nil {
		return fmt.Errorf("failed to get quota rule: %w", err)
	}

	now := time.Now()

	// This would require implementing a reset method in the repository
	// For now, we'll use UpdateUsage with negative value to reset
	currentUsage, err := s.repos.QuotaUsage.GetCurrentUsage(ctx, quotaRuleID, now)
	if err == nil && currentUsage != nil {
		// Reset to zero by subtracting current usage
		if err := s.repos.QuotaUsage.UpdateUsage(ctx, quotaRuleID, -currentUsage.UsedSeconds, now); err != nil {
			return fmt.Errorf("failed to reset quota usage: %w", err)
		}
	}

	s.logger.Info("Quota usage reset successfully", logging.Int("quota_rule_id", quotaRuleID))
	return nil
}

// validateCreateQuotaRuleRequest validates a create quota rule request
func (s *QuotaService) validateCreateQuotaRuleRequest(ctx context.Context, req CreateQuotaRuleRequest) error {
	// Verify list exists
	if _, err := s.repos.List.GetByID(ctx, req.ListID); err != nil {
		return fmt.Errorf("invalid list ID: %w", err)
	}

	// Validate name
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("name is required")
	}

	if err := s.validateQuotaRuleName(ctx, req.Name, req.ListID, nil); err != nil {
		return err
	}

	// Validate quota type
	if req.QuotaType != models.QuotaTypeDaily &&
		req.QuotaType != models.QuotaTypeWeekly &&
		req.QuotaType != models.QuotaTypeMonthly {
		return fmt.Errorf("invalid quota type: %s", req.QuotaType)
	}

	// Validate limit
	if req.LimitSeconds < 1 {
		return fmt.Errorf("limit must be at least 1 second")
	}

	return nil
}

// validateQuotaRuleName checks if a quota rule name is unique within a list
func (s *QuotaService) validateQuotaRuleName(ctx context.Context, name string, listID int, excludeID *int) error {
	rules, err := s.repos.QuotaRule.GetByListID(ctx, listID)
	if err != nil {
		return fmt.Errorf("failed to check existing rules: %w", err)
	}

	for _, rule := range rules {
		if excludeID != nil && rule.ID == *excludeID {
			continue
		}
		if rule.Name == name {
			return fmt.Errorf("quota rule name '%s' already exists in this list", name)
		}
	}

	return nil
}

// getPeriodStart returns the start of the current period for a quota type
func (s *QuotaService) getPeriodStart(quotaType models.QuotaType, t time.Time) time.Time {
	switch quotaType {
	case models.QuotaTypeDaily:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case models.QuotaTypeWeekly:
		// Start of week (Sunday)
		days := int(t.Weekday())
		return time.Date(t.Year(), t.Month(), t.Day()-days, 0, 0, 0, 0, t.Location())
	case models.QuotaTypeMonthly:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	default:
		return t
	}
}

// getPeriodEnd returns the end of the current period for a quota type
func (s *QuotaService) getPeriodEnd(quotaType models.QuotaType, t time.Time) time.Time {
	switch quotaType {
	case models.QuotaTypeDaily:
		return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, t.Location())
	case models.QuotaTypeWeekly:
		// End of week (Saturday)
		days := 6 - int(t.Weekday())
		return time.Date(t.Year(), t.Month(), t.Day()+days, 23, 59, 59, 999999999, t.Location())
	case models.QuotaTypeMonthly:
		nextMonth := t.AddDate(0, 1, 0)
		return time.Date(nextMonth.Year(), nextMonth.Month(), 1, 0, 0, 0, 0, t.Location()).Add(-time.Nanosecond)
	default:
		return t
	}
}

// getNextReset returns when the quota will next reset
func (s *QuotaService) getNextReset(quotaType models.QuotaType, t time.Time) time.Time {
	switch quotaType {
	case models.QuotaTypeDaily:
		return time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
	case models.QuotaTypeWeekly:
		// Next Sunday
		days := 7 - int(t.Weekday())
		return time.Date(t.Year(), t.Month(), t.Day()+days, 0, 0, 0, 0, t.Location())
	case models.QuotaTypeMonthly:
		return time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
	default:
		return t
	}
}

// calculateWarningLevel calculates the warning level based on usage percentage
func (s *QuotaService) calculateWarningLevel(usedSeconds, limitSeconds int) QuotaWarningLevel {
	if limitSeconds == 0 {
		return WarningLevelNone
	}

	percentage := float64(usedSeconds) / float64(limitSeconds) * 100

	switch {
	case percentage > 100:
		return WarningLevelExceeded
	case percentage >= 90:
		return WarningLevelHigh
	case percentage >= 75:
		return WarningLevelMedium
	case percentage >= 50:
		return WarningLevelLow
	default:
		return WarningLevelNone
	}
}
