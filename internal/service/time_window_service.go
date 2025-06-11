package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"parental-control/internal/logging"
	"parental-control/internal/models"
)

// TimeWindowService provides business logic for managing time-based rules
type TimeWindowService struct {
	repos  *models.RepositoryManager
	logger logging.Logger
}

// NewTimeWindowService creates a new time window service
func NewTimeWindowService(repos *models.RepositoryManager, logger logging.Logger) *TimeWindowService {
	return &TimeWindowService{
		repos:  repos,
		logger: logger,
	}
}

// CreateTimeRuleRequest represents a request to create a new time rule
type CreateTimeRuleRequest struct {
	ListID     int             `json:"list_id" validate:"required"`
	Name       string          `json:"name" validate:"required,max=255"`
	RuleType   models.RuleType `json:"rule_type" validate:"required,oneof=allow_during block_during"`
	DaysOfWeek []int           `json:"days_of_week" validate:"required,dive,min=0,max=6"`
	StartTime  string          `json:"start_time" validate:"required"`
	EndTime    string          `json:"end_time" validate:"required"`
	Enabled    bool            `json:"enabled"`
}

// UpdateTimeRuleRequest represents a request to update an existing time rule
type UpdateTimeRuleRequest struct {
	Name       *string          `json:"name,omitempty" validate:"omitempty,max=255"`
	RuleType   *models.RuleType `json:"rule_type,omitempty" validate:"omitempty,oneof=allow_during block_during"`
	DaysOfWeek []int            `json:"days_of_week,omitempty" validate:"omitempty,dive,min=0,max=6"`
	StartTime  *string          `json:"start_time,omitempty" validate:"omitempty"`
	EndTime    *string          `json:"end_time,omitempty" validate:"omitempty"`
	Enabled    *bool            `json:"enabled,omitempty"`
}

// TimeRuleStatus represents the current status of a time rule
type TimeRuleStatus struct {
	*models.TimeRule
	IsActive         bool       `json:"is_active"`
	NextActivation   *time.Time `json:"next_activation,omitempty"`
	NextDeactivation *time.Time `json:"next_deactivation,omitempty"`
}

// SchedulePreview represents a preview of when rules will be active
type SchedulePreview struct {
	RuleID          int          `json:"rule_id"`
	RuleName        string       `json:"rule_name"`
	ActivePeriods   []TimePeriod `json:"active_periods"`
	InactivePeriods []TimePeriod `json:"inactive_periods"`
}

// TimePeriod represents a time period
type TimePeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// CreateTimeRule creates a new time rule with validation
func (s *TimeWindowService) CreateTimeRule(ctx context.Context, req CreateTimeRuleRequest) (*models.TimeRule, error) {
	s.logger.Info("Creating new time rule",
		logging.String("name", req.Name),
		logging.Int("list_id", req.ListID),
		logging.String("rule_type", string(req.RuleType)))

	// Validate the request
	if err := s.validateCreateTimeRuleRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	rule := &models.TimeRule{
		ListID:     req.ListID,
		Name:       req.Name,
		RuleType:   req.RuleType,
		DaysOfWeek: req.DaysOfWeek,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
		Enabled:    req.Enabled,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.repos.TimeRule.Create(ctx, rule); err != nil {
		s.logger.Error("Failed to create time rule", logging.Err(err))
		return nil, fmt.Errorf("failed to create time rule: %w", err)
	}

	s.logger.Info("Time rule created successfully",
		logging.Int("id", rule.ID),
		logging.String("name", rule.Name))

	return rule, nil
}

// GetTimeRule retrieves a time rule by ID
func (s *TimeWindowService) GetTimeRule(ctx context.Context, id int) (*models.TimeRule, error) {
	return s.repos.TimeRule.GetByID(ctx, id)
}

// GetTimeRulesByListID retrieves all time rules for a specific list
func (s *TimeWindowService) GetTimeRulesByListID(ctx context.Context, listID int) ([]models.TimeRule, error) {
	return s.repos.TimeRule.GetByListID(ctx, listID)
}

// GetTimeRuleStatus retrieves a time rule with its current status
func (s *TimeWindowService) GetTimeRuleStatus(ctx context.Context, id int) (*TimeRuleStatus, error) {
	rule, err := s.repos.TimeRule.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get time rule: %w", err)
	}

	now := time.Now()
	isActive := s.IsRuleActiveAt(rule, now)

	nextActivation, nextDeactivation := s.calculateNextStateChanges(rule, now)

	return &TimeRuleStatus{
		TimeRule:         rule,
		IsActive:         isActive,
		NextActivation:   nextActivation,
		NextDeactivation: nextDeactivation,
	}, nil
}

// UpdateTimeRule updates an existing time rule
func (s *TimeWindowService) UpdateTimeRule(ctx context.Context, id int, req UpdateTimeRuleRequest) (*models.TimeRule, error) {
	s.logger.Info("Updating time rule", logging.Int("id", id))

	// Get the existing rule
	rule, err := s.repos.TimeRule.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get time rule: %w", err)
	}

	// Apply updates
	if req.Name != nil {
		if err := s.validateTimeRuleName(ctx, *req.Name, rule.ListID, &id); err != nil {
			return nil, fmt.Errorf("invalid name: %w", err)
		}
		rule.Name = *req.Name
	}
	if req.RuleType != nil {
		rule.RuleType = *req.RuleType
	}
	if req.DaysOfWeek != nil {
		if err := s.validateDaysOfWeek(req.DaysOfWeek); err != nil {
			return nil, fmt.Errorf("invalid days of week: %w", err)
		}
		rule.DaysOfWeek = req.DaysOfWeek
	}
	if req.StartTime != nil {
		if err := models.ValidateTimeFormat(*req.StartTime); err != nil {
			return nil, fmt.Errorf("invalid start time: %w", err)
		}
		rule.StartTime = *req.StartTime
	}
	if req.EndTime != nil {
		if err := models.ValidateTimeFormat(*req.EndTime); err != nil {
			return nil, fmt.Errorf("invalid end time: %w", err)
		}
		rule.EndTime = *req.EndTime
	}

	// Validate time range after potential updates
	if req.StartTime != nil || req.EndTime != nil {
		if err := s.validateTimeRange(rule.StartTime, rule.EndTime); err != nil {
			return nil, fmt.Errorf("invalid time range: %w", err)
		}
	}

	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}

	rule.UpdatedAt = time.Now()

	if err := s.repos.TimeRule.Update(ctx, rule); err != nil {
		s.logger.Error("Failed to update time rule", logging.Err(err))
		return nil, fmt.Errorf("failed to update time rule: %w", err)
	}

	s.logger.Info("Time rule updated successfully", logging.Int("id", id))
	return rule, nil
}

// DeleteTimeRule deletes a time rule
func (s *TimeWindowService) DeleteTimeRule(ctx context.Context, id int) error {
	s.logger.Info("Deleting time rule", logging.Int("id", id))

	// Check if rule exists
	rule, err := s.repos.TimeRule.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get time rule: %w", err)
	}

	if err := s.repos.TimeRule.Delete(ctx, id); err != nil {
		s.logger.Error("Failed to delete time rule", logging.Err(err))
		return fmt.Errorf("failed to delete time rule: %w", err)
	}

	s.logger.Info("Time rule deleted successfully",
		logging.Int("id", id),
		logging.String("name", rule.Name))

	return nil
}

// ToggleTimeRuleEnabled toggles the enabled state of a time rule
func (s *TimeWindowService) ToggleTimeRuleEnabled(ctx context.Context, id int) (*models.TimeRule, error) {
	rule, err := s.repos.TimeRule.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get time rule: %w", err)
	}

	rule.Enabled = !rule.Enabled
	rule.UpdatedAt = time.Now()

	if err := s.repos.TimeRule.Update(ctx, rule); err != nil {
		return nil, fmt.Errorf("failed to update time rule: %w", err)
	}

	s.logger.Info("Time rule enabled state toggled",
		logging.Int("id", id),
		logging.Bool("enabled", rule.Enabled))

	return rule, nil
}

// GetActiveRules returns all currently active time rules
func (s *TimeWindowService) GetActiveRules(ctx context.Context) ([]models.TimeRule, error) {
	now := time.Now()
	return s.repos.TimeRule.GetActiveRules(ctx, now)
}

// GetEnabledRules returns all enabled time rules
func (s *TimeWindowService) GetEnabledRules(ctx context.Context) ([]models.TimeRule, error) {
	return s.repos.TimeRule.GetEnabled(ctx)
}

// IsRuleActiveAt checks if a time rule is active at a specific time
func (s *TimeWindowService) IsRuleActiveAt(rule *models.TimeRule, t time.Time) bool {
	if !rule.Enabled {
		return false
	}

	// Check day of week (0 = Sunday, 1 = Monday, etc.)
	dayOfWeek := int(t.Weekday())
	dayMatches := false
	for _, day := range rule.DaysOfWeek {
		if day == dayOfWeek {
			dayMatches = true
			break
		}
	}

	if !dayMatches {
		return false
	}

	// Check time of day
	currentTime := t.Format("15:04")

	// Handle overnight rules (e.g., 22:00 to 06:00)
	if rule.StartTime > rule.EndTime {
		return currentTime >= rule.StartTime || currentTime <= rule.EndTime
	}

	// Normal rules (e.g., 09:00 to 17:00)
	return currentTime >= rule.StartTime && currentTime <= rule.EndTime
}

// IsListActiveAt checks if a list should be active based on its time rules
func (s *TimeWindowService) IsListActiveAt(ctx context.Context, listID int, t time.Time) (bool, error) {
	rules, err := s.repos.TimeRule.GetByListID(ctx, listID)
	if err != nil {
		return false, fmt.Errorf("failed to get time rules: %w", err)
	}

	// If no time rules, the list is active by default
	if len(rules) == 0 {
		return true, nil
	}

	// Process rules by type
	hasAllowRules := false
	hasBlockRules := false
	allowActive := false
	blockActive := false

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		isActive := s.IsRuleActiveAt(&rule, t)

		switch rule.RuleType {
		case models.RuleTypeAllowDuring:
			hasAllowRules = true
			if isActive {
				allowActive = true
			}
		case models.RuleTypeBlockDuring:
			hasBlockRules = true
			if isActive {
				blockActive = true
			}
		}
	}

	// Decision logic:
	// - If there are block rules and any are active, list is inactive
	// - If there are allow rules and none are active, list is inactive
	// - Otherwise, list is active

	if hasBlockRules && blockActive {
		return false, nil
	}

	if hasAllowRules && !allowActive {
		return false, nil
	}

	return true, nil
}

// GetSchedulePreview generates a preview of when rules will be active
func (s *TimeWindowService) GetSchedulePreview(ctx context.Context, listID int, days int) ([]SchedulePreview, error) {
	rules, err := s.repos.TimeRule.GetByListID(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("failed to get time rules: %w", err)
	}

	previews := make([]SchedulePreview, 0, len(rules))
	now := time.Now()
	endTime := now.AddDate(0, 0, days)

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		preview := SchedulePreview{
			RuleID:          rule.ID,
			RuleName:        rule.Name,
			ActivePeriods:   make([]TimePeriod, 0),
			InactivePeriods: make([]TimePeriod, 0),
		}

		// Calculate active periods for the preview window
		current := now
		for current.Before(endTime) {
			if s.IsRuleActiveAt(&rule, current) {
				// Find the end of this active period
				periodEnd := current
				for periodEnd.Before(endTime) && s.IsRuleActiveAt(&rule, periodEnd) {
					periodEnd = periodEnd.Add(15 * time.Minute) // Check every 15 minutes
				}

				preview.ActivePeriods = append(preview.ActivePeriods, TimePeriod{
					Start: current,
					End:   periodEnd,
				})

				current = periodEnd
			} else {
				current = current.Add(15 * time.Minute)
			}
		}

		previews = append(previews, preview)
	}

	return previews, nil
}

// DetectConflicts detects conflicting time rules for a list
func (s *TimeWindowService) DetectConflicts(ctx context.Context, listID int) ([]string, error) {
	rules, err := s.repos.TimeRule.GetByListID(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("failed to get time rules: %w", err)
	}

	var conflicts []string

	// Check for overlapping rules of different types
	for i, rule1 := range rules {
		if !rule1.Enabled {
			continue
		}

		for j, rule2 := range rules {
			if i >= j || !rule2.Enabled {
				continue
			}

			// Check if rules have conflicting types and overlapping schedules
			if rule1.RuleType != rule2.RuleType && s.rulesOverlap(&rule1, &rule2) {
				conflicts = append(conflicts, fmt.Sprintf("Rules '%s' and '%s' have conflicting types and overlapping schedules", rule1.Name, rule2.Name))
			}
		}
	}

	return conflicts, nil
}

// validateCreateTimeRuleRequest validates a create time rule request
func (s *TimeWindowService) validateCreateTimeRuleRequest(ctx context.Context, req CreateTimeRuleRequest) error {
	// Verify list exists
	if _, err := s.repos.List.GetByID(ctx, req.ListID); err != nil {
		return fmt.Errorf("invalid list ID: %w", err)
	}

	// Validate name
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("name is required")
	}

	if err := s.validateTimeRuleName(ctx, req.Name, req.ListID, nil); err != nil {
		return err
	}

	// Validate rule type
	if req.RuleType != models.RuleTypeAllowDuring && req.RuleType != models.RuleTypeBlockDuring {
		return fmt.Errorf("invalid rule type: %s", req.RuleType)
	}

	// Validate days of week
	if err := s.validateDaysOfWeek(req.DaysOfWeek); err != nil {
		return err
	}

	// Validate time format
	if err := models.ValidateTimeFormat(req.StartTime); err != nil {
		return fmt.Errorf("invalid start time: %w", err)
	}

	if err := models.ValidateTimeFormat(req.EndTime); err != nil {
		return fmt.Errorf("invalid end time: %w", err)
	}

	// Validate time range
	return s.validateTimeRange(req.StartTime, req.EndTime)
}

// validateTimeRuleName checks if a time rule name is unique within a list
func (s *TimeWindowService) validateTimeRuleName(ctx context.Context, name string, listID int, excludeID *int) error {
	rules, err := s.repos.TimeRule.GetByListID(ctx, listID)
	if err != nil {
		return fmt.Errorf("failed to check existing rules: %w", err)
	}

	for _, rule := range rules {
		if excludeID != nil && rule.ID == *excludeID {
			continue
		}
		if rule.Name == name {
			return fmt.Errorf("time rule name '%s' already exists in this list", name)
		}
	}

	return nil
}

// validateDaysOfWeek validates the days of week array
func (s *TimeWindowService) validateDaysOfWeek(days []int) error {
	if len(days) == 0 {
		return fmt.Errorf("at least one day of week must be specified")
	}

	daysSeen := make(map[int]bool)
	for _, day := range days {
		if day < 0 || day > 6 {
			return fmt.Errorf("invalid day of week: %d (must be 0-6)", day)
		}
		if daysSeen[day] {
			return fmt.Errorf("duplicate day of week: %d", day)
		}
		daysSeen[day] = true
	}

	return nil
}

// validateTimeRange validates that times are properly formatted and make sense
func (s *TimeWindowService) validateTimeRange(startTime, endTime string) error {
	// Times can be equal (for single-time rules) but start can also be after end (overnight rules)
	// So we don't need strict validation here, just format validation which is done elsewhere
	return nil
}

// rulesOverlap checks if two time rules have overlapping schedules
func (s *TimeWindowService) rulesOverlap(rule1, rule2 *models.TimeRule) bool {
	// Check if they share any days
	dayOverlap := false
	for _, day1 := range rule1.DaysOfWeek {
		for _, day2 := range rule2.DaysOfWeek {
			if day1 == day2 {
				dayOverlap = true
				break
			}
		}
		if dayOverlap {
			break
		}
	}

	if !dayOverlap {
		return false
	}

	// Check if time ranges overlap
	return s.timeRangesOverlap(rule1.StartTime, rule1.EndTime, rule2.StartTime, rule2.EndTime)
}

// timeRangesOverlap checks if two time ranges overlap
func (s *TimeWindowService) timeRangesOverlap(start1, end1, start2, end2 string) bool {
	// Parse times
	s1, _ := time.Parse("15:04", start1)
	e1, _ := time.Parse("15:04", end1)
	s2, _ := time.Parse("15:04", start2)
	e2, _ := time.Parse("15:04", end2)

	// Handle overnight ranges
	if start1 > end1 {
		e1 = e1.Add(24 * time.Hour)
	}
	if start2 > end2 {
		e2 = e2.Add(24 * time.Hour)
	}

	// Check overlap
	return s1.Before(e2) && s2.Before(e1)
}

// calculateNextStateChanges calculates when a rule will next activate/deactivate
func (s *TimeWindowService) calculateNextStateChanges(rule *models.TimeRule, from time.Time) (*time.Time, *time.Time) {
	// This is a simplified version - a full implementation would need to handle
	// complex cases like overnight rules, timezone changes, etc.

	var nextActivation, nextDeactivation *time.Time

	// Look ahead up to 7 days
	for i := 0; i < 7; i++ {
		checkTime := from.AddDate(0, 0, i)
		dayOfWeek := int(checkTime.Weekday())

		// Check if this day is in the rule's days
		dayMatches := false
		for _, day := range rule.DaysOfWeek {
			if day == dayOfWeek {
				dayMatches = true
				break
			}
		}

		if !dayMatches {
			continue
		}

		// Calculate activation time for this day
		startTime, _ := time.Parse("15:04", rule.StartTime)
		endTime, _ := time.Parse("15:04", rule.EndTime)

		activationTime := time.Date(checkTime.Year(), checkTime.Month(), checkTime.Day(),
			startTime.Hour(), startTime.Minute(), 0, 0, checkTime.Location())
		deactivationTime := time.Date(checkTime.Year(), checkTime.Month(), checkTime.Day(),
			endTime.Hour(), endTime.Minute(), 0, 0, checkTime.Location())

		// Adjust for overnight rules
		if rule.StartTime > rule.EndTime {
			deactivationTime = deactivationTime.AddDate(0, 0, 1)
		}

		if nextActivation == nil && activationTime.After(from) {
			nextActivation = &activationTime
		}
		if nextDeactivation == nil && deactivationTime.After(from) {
			nextDeactivation = &deactivationTime
		}

		if nextActivation != nil && nextDeactivation != nil {
			break
		}
	}

	return nextActivation, nextDeactivation
}
