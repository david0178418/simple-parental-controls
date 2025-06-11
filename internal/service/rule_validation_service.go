package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"parental-control/internal/logging"
	"parental-control/internal/models"
)

// RuleValidationService provides business logic for rule validation and conflict resolution
type RuleValidationService struct {
	repos        *models.RepositoryManager
	listService  *ListManagementService
	entryService *EntryManagementService
	timeService  *TimeWindowService
	quotaService *QuotaService
	logger       logging.Logger
}

// NewRuleValidationService creates a new rule validation service
func NewRuleValidationService(repos *models.RepositoryManager, logger logging.Logger) *RuleValidationService {
	return &RuleValidationService{
		repos:        repos,
		listService:  NewListManagementService(repos, logger),
		entryService: NewEntryManagementService(repos, logger),
		timeService:  NewTimeWindowService(repos, logger),
		quotaService: NewQuotaService(repos, logger),
		logger:       logger,
	}
}

// ConflictType represents the type of rule conflict
type ConflictType string

const (
	ConflictTypeHard    ConflictType = "hard"    // Rules that directly contradict each other
	ConflictTypeWarning ConflictType = "warning" // Rules that may cause unexpected behavior
)

// ConflictSeverity represents the severity level of a conflict
type ConflictSeverity string

const (
	SeverityLow      ConflictSeverity = "low"
	SeverityMedium   ConflictSeverity = "medium"
	SeverityHigh     ConflictSeverity = "high"
	SeverityCritical ConflictSeverity = "critical"
)

// RuleConflict represents a detected conflict between rules
type RuleConflict struct {
	ID             string           `json:"id"`
	Type           ConflictType     `json:"type"`
	Severity       ConflictSeverity `json:"severity"`
	Title          string           `json:"title"`
	Description    string           `json:"description"`
	AffectedRules  []ConflictedRule `json:"affected_rules"`
	Suggestions    []string         `json:"suggestions"`
	AutoResolvable bool             `json:"auto_resolvable"`
}

// ConflictedRule represents a rule involved in a conflict
type ConflictedRule struct {
	RuleType string `json:"rule_type"` // "list", "time_rule", "quota_rule"
	RuleID   int    `json:"rule_id"`
	RuleName string `json:"rule_name"`
	ListID   int    `json:"list_id"`
	ListName string `json:"list_name"`
}

// ValidationResult represents the result of rule validation
type ValidationResult struct {
	IsValid     bool           `json:"is_valid"`
	Conflicts   []RuleConflict `json:"conflicts"`
	Warnings    []string       `json:"warnings"`
	Suggestions []string       `json:"suggestions"`
}

// RuleImpactAnalysis represents the impact analysis of a rule change
type RuleImpactAnalysis struct {
	AffectedLists      []int          `json:"affected_lists"`
	AffectedEntries    []int          `json:"affected_entries"`
	AffectedTimeRules  []int          `json:"affected_time_rules"`
	AffectedQuotaRules []int          `json:"affected_quota_rules"`
	PotentialConflicts []RuleConflict `json:"potential_conflicts"`
	RecommendedActions []string       `json:"recommended_actions"`
}

// ValidateAllRules validates all rules in the system and returns conflicts
func (s *RuleValidationService) ValidateAllRules(ctx context.Context) (*ValidationResult, error) {
	s.logger.Info("Validating all rules in the system")

	conflicts := make([]RuleConflict, 0)
	warnings := make([]string, 0)
	suggestions := make([]string, 0)

	// Get all lists
	lists, err := s.repos.List.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get lists: %w", err)
	}

	// Validate each list and its rules
	for _, list := range lists {
		listConflicts, listWarnings, listSuggestions := s.validateListRules(ctx, list.ID)
		conflicts = append(conflicts, listConflicts...)
		warnings = append(warnings, listWarnings...)
		suggestions = append(suggestions, listSuggestions...)
	}

	// Check for system-wide conflicts
	systemConflicts := s.detectSystemWideConflicts(ctx, lists)
	conflicts = append(conflicts, systemConflicts...)

	isValid := len(conflicts) == 0 || s.allConflictsAreWarnings(conflicts)

	result := &ValidationResult{
		IsValid:     isValid,
		Conflicts:   conflicts,
		Warnings:    warnings,
		Suggestions: suggestions,
	}

	s.logger.Info("Rule validation completed",
		logging.Int("conflicts", len(conflicts)),
		logging.Int("warnings", len(warnings)),
		logging.Bool("is_valid", isValid))

	return result, nil
}

// ValidateListRules validates all rules for a specific list
func (s *RuleValidationService) ValidateListRules(ctx context.Context, listID int) (*ValidationResult, error) {
	s.logger.Info("Validating rules for list", logging.Int("list_id", listID))

	conflicts, warnings, suggestions := s.validateListRules(ctx, listID)

	isValid := len(conflicts) == 0 || s.allConflictsAreWarnings(conflicts)

	return &ValidationResult{
		IsValid:     isValid,
		Conflicts:   conflicts,
		Warnings:    warnings,
		Suggestions: suggestions,
	}, nil
}

// AnalyzeRuleImpact analyzes the impact of a rule change before applying it
func (s *RuleValidationService) AnalyzeRuleImpact(ctx context.Context, ruleType string, ruleID int, operation string) (*RuleImpactAnalysis, error) {
	s.logger.Info("Analyzing rule impact",
		logging.String("rule_type", ruleType),
		logging.Int("rule_id", ruleID),
		logging.String("operation", operation))

	analysis := &RuleImpactAnalysis{
		AffectedLists:      make([]int, 0),
		AffectedEntries:    make([]int, 0),
		AffectedTimeRules:  make([]int, 0),
		AffectedQuotaRules: make([]int, 0),
		PotentialConflicts: make([]RuleConflict, 0),
		RecommendedActions: make([]string, 0),
	}

	switch ruleType {
	case "list":
		return s.analyzeListImpact(ctx, ruleID, operation, analysis)
	case "time_rule":
		return s.analyzeTimeRuleImpact(ctx, ruleID, operation, analysis)
	case "quota_rule":
		return s.analyzeQuotaRuleImpact(ctx, ruleID, operation, analysis)
	case "entry":
		return s.analyzeEntryImpact(ctx, ruleID, operation, analysis)
	default:
		return nil, fmt.Errorf("unsupported rule type: %s", ruleType)
	}
}

// DetectConflictingEntries detects entries that might conflict with each other
func (s *RuleValidationService) DetectConflictingEntries(ctx context.Context, listID int) ([]RuleConflict, error) {
	entries, err := s.repos.ListEntry.GetByListID(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("failed to get entries: %w", err)
	}

	conflicts := make([]RuleConflict, 0)

	// Check for overlapping patterns
	for i, entry1 := range entries {
		for j, entry2 := range entries {
			if i >= j || entry1.EntryType != entry2.EntryType {
				continue
			}

			if s.patternsOverlap(entry1.Pattern, entry1.PatternType, entry2.Pattern, entry2.PatternType) {
				conflict := RuleConflict{
					ID:          fmt.Sprintf("entry_overlap_%d_%d", entry1.ID, entry2.ID),
					Type:        ConflictTypeWarning,
					Severity:    SeverityMedium,
					Title:       "Overlapping Entry Patterns",
					Description: fmt.Sprintf("Entries '%s' and '%s' have overlapping patterns", entry1.Pattern, entry2.Pattern),
					AffectedRules: []ConflictedRule{
						{RuleType: "entry", RuleID: entry1.ID, RuleName: entry1.Pattern},
						{RuleType: "entry", RuleID: entry2.ID, RuleName: entry2.Pattern},
					},
					Suggestions: []string{
						"Consider using more specific patterns",
						"Merge overlapping entries if they serve the same purpose",
					},
					AutoResolvable: false,
				}
				conflicts = append(conflicts, conflict)
			}
		}
	}

	return conflicts, nil
}

// ResolveConflict attempts to automatically resolve a conflict
func (s *RuleValidationService) ResolveConflict(ctx context.Context, conflictID string) error {
	s.logger.Info("Attempting to resolve conflict", logging.String("conflict_id", conflictID))

	// Parse conflict ID to determine resolution strategy
	if strings.HasPrefix(conflictID, "time_overlap_") {
		return s.resolveTimeOverlapConflict(ctx, conflictID)
	} else if strings.HasPrefix(conflictID, "quota_duplicate_") {
		return s.resolveQuotaDuplicateConflict(ctx, conflictID)
	}

	return fmt.Errorf("no automatic resolution available for conflict: %s", conflictID)
}

// validateListRules validates all rules for a specific list
func (s *RuleValidationService) validateListRules(ctx context.Context, listID int) ([]RuleConflict, []string, []string) {
	conflicts := make([]RuleConflict, 0)
	warnings := make([]string, 0)
	suggestions := make([]string, 0)

	// Validate list entries
	entryConflicts, err := s.DetectConflictingEntries(ctx, listID)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("Failed to validate entries for list %d: %v", listID, err))
	} else {
		conflicts = append(conflicts, entryConflicts...)
	}

	// Validate time rules
	timeConflicts := s.detectTimeRuleConflicts(ctx, listID)
	conflicts = append(conflicts, timeConflicts...)

	// Validate quota rules
	quotaConflicts := s.detectQuotaRuleConflicts(ctx, listID)
	conflicts = append(conflicts, quotaConflicts...)

	// Check for logical inconsistencies
	logicalConflicts := s.detectLogicalInconsistencies(ctx, listID)
	conflicts = append(conflicts, logicalConflicts...)

	// Generate suggestions
	if len(conflicts) > 0 {
		suggestions = append(suggestions, "Review conflicting rules and consider consolidating or clarifying their purposes")
	}

	return conflicts, warnings, suggestions
}

// detectTimeRuleConflicts detects conflicts between time rules
func (s *RuleValidationService) detectTimeRuleConflicts(ctx context.Context, listID int) []RuleConflict {
	rules, err := s.repos.TimeRule.GetByListID(ctx, listID)
	if err != nil {
		return []RuleConflict{}
	}

	conflicts := make([]RuleConflict, 0)

	for i, rule1 := range rules {
		for j, rule2 := range rules {
			if i >= j || !rule1.Enabled || !rule2.Enabled {
				continue
			}

			// Check for opposing rule types with overlapping schedules
			if rule1.RuleType != rule2.RuleType && s.scheduleOverlap(&rule1, &rule2) {
				conflict := RuleConflict{
					ID:          fmt.Sprintf("time_overlap_%d_%d", rule1.ID, rule2.ID),
					Type:        ConflictTypeHard,
					Severity:    SeverityHigh,
					Title:       "Conflicting Time Rules",
					Description: fmt.Sprintf("Time rules '%s' and '%s' have conflicting types and overlapping schedules", rule1.Name, rule2.Name),
					AffectedRules: []ConflictedRule{
						{RuleType: "time_rule", RuleID: rule1.ID, RuleName: rule1.Name, ListID: listID},
						{RuleType: "time_rule", RuleID: rule2.ID, RuleName: rule2.Name, ListID: listID},
					},
					Suggestions: []string{
						"Adjust time ranges to avoid overlap",
						"Change one rule type to match the other",
						"Disable one of the conflicting rules",
					},
					AutoResolvable: false,
				}
				conflicts = append(conflicts, conflict)
			}
		}
	}

	return conflicts
}

// detectQuotaRuleConflicts detects conflicts between quota rules
func (s *RuleValidationService) detectQuotaRuleConflicts(ctx context.Context, listID int) []RuleConflict {
	rules, err := s.repos.QuotaRule.GetByListID(ctx, listID)
	if err != nil {
		return []RuleConflict{}
	}

	conflicts := make([]RuleConflict, 0)

	// Check for duplicate quota types
	typeCount := make(map[models.QuotaType][]models.QuotaRule)
	for _, rule := range rules {
		if rule.Enabled {
			typeCount[rule.QuotaType] = append(typeCount[rule.QuotaType], rule)
		}
	}

	for quotaType, rulesOfType := range typeCount {
		if len(rulesOfType) > 1 {
			affectedRules := make([]ConflictedRule, len(rulesOfType))
			for i, rule := range rulesOfType {
				affectedRules[i] = ConflictedRule{
					RuleType: "quota_rule",
					RuleID:   rule.ID,
					RuleName: rule.Name,
					ListID:   listID,
				}
			}

			conflict := RuleConflict{
				ID:            fmt.Sprintf("quota_duplicate_%s_%d", quotaType, listID),
				Type:          ConflictTypeWarning,
				Severity:      SeverityMedium,
				Title:         "Duplicate Quota Types",
				Description:   fmt.Sprintf("Multiple %s quota rules exist for the same list", quotaType),
				AffectedRules: affectedRules,
				Suggestions: []string{
					"Consolidate duplicate quota rules",
					"Use different quota types for different purposes",
				},
				AutoResolvable: true,
			}
			conflicts = append(conflicts, conflict)
		}
	}

	return conflicts
}

// detectLogicalInconsistencies detects logical inconsistencies in rules
func (s *RuleValidationService) detectLogicalInconsistencies(ctx context.Context, listID int) []RuleConflict {
	conflicts := make([]RuleConflict, 0)

	// Get list information
	list, err := s.repos.List.GetByID(ctx, listID)
	if err != nil {
		return conflicts
	}

	// Check for empty lists
	entryCount, err := s.repos.ListEntry.CountByListID(ctx, listID)
	if err == nil && entryCount == 0 && list.Enabled {
		conflict := RuleConflict{
			ID:          fmt.Sprintf("empty_list_%d", listID),
			Type:        ConflictTypeWarning,
			Severity:    SeverityLow,
			Title:       "Empty List",
			Description: fmt.Sprintf("List '%s' is enabled but contains no entries", list.Name),
			AffectedRules: []ConflictedRule{
				{RuleType: "list", RuleID: list.ID, RuleName: list.Name, ListID: listID},
			},
			Suggestions: []string{
				"Add entries to the list",
				"Disable the list if not needed",
			},
			AutoResolvable: false,
		}
		conflicts = append(conflicts, conflict)
	}

	return conflicts
}

// detectSystemWideConflicts detects conflicts across the entire system
func (s *RuleValidationService) detectSystemWideConflicts(ctx context.Context, lists []models.List) []RuleConflict {
	conflicts := make([]RuleConflict, 0)

	// Check for conflicting list types with overlapping entries
	whitelists := make([]models.List, 0)
	blacklists := make([]models.List, 0)

	for _, list := range lists {
		if !list.Enabled {
			continue
		}
		if list.Type == models.ListTypeWhitelist {
			whitelists = append(whitelists, list)
		} else {
			blacklists = append(blacklists, list)
		}
	}

	// This is a simplified check - a full implementation would need to compare actual entries
	if len(whitelists) > 0 && len(blacklists) > 0 {
		conflict := RuleConflict{
			ID:          "system_whitelist_blacklist_conflict",
			Type:        ConflictTypeWarning,
			Severity:    SeverityMedium,
			Title:       "Mixed List Types",
			Description: "Both whitelist and blacklist types are active, which may cause confusion",
			Suggestions: []string{
				"Consider using only one list type for consistency",
				"Clearly document the intended behavior",
			},
			AutoResolvable: false,
		}
		conflicts = append(conflicts, conflict)
	}

	return conflicts
}

// Helper methods

func (s *RuleValidationService) allConflictsAreWarnings(conflicts []RuleConflict) bool {
	for _, conflict := range conflicts {
		if conflict.Type == ConflictTypeHard {
			return false
		}
	}
	return true
}

func (s *RuleValidationService) patternsOverlap(pattern1 string, type1 models.PatternType, pattern2 string, type2 models.PatternType) bool {
	// Simplified overlap detection - a full implementation would need more sophisticated pattern matching
	if type1 == models.PatternTypeExact && type2 == models.PatternTypeExact {
		return pattern1 == pattern2
	}

	// For wildcard patterns, check if one pattern could match the other
	if type1 == models.PatternTypeWildcard || type2 == models.PatternTypeWildcard {
		// This is a simplified check - real implementation would need proper wildcard matching
		return strings.Contains(pattern1, pattern2) || strings.Contains(pattern2, pattern1)
	}

	return false
}

func (s *RuleValidationService) scheduleOverlap(rule1, rule2 *models.TimeRule) bool {
	// Check if rules share any days
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

	// Check if time ranges overlap (simplified)
	return s.timeRangesOverlap(rule1.StartTime, rule1.EndTime, rule2.StartTime, rule2.EndTime)
}

func (s *RuleValidationService) timeRangesOverlap(start1, end1, start2, end2 string) bool {
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

// Impact analysis methods (simplified implementations)

func (s *RuleValidationService) analyzeListImpact(ctx context.Context, listID int, operation string, analysis *RuleImpactAnalysis) (*RuleImpactAnalysis, error) {
	analysis.AffectedLists = append(analysis.AffectedLists, listID)

	// Get affected entries, time rules, and quota rules
	entries, _ := s.repos.ListEntry.GetByListID(ctx, listID)
	for _, entry := range entries {
		analysis.AffectedEntries = append(analysis.AffectedEntries, entry.ID)
	}

	timeRules, _ := s.repos.TimeRule.GetByListID(ctx, listID)
	for _, rule := range timeRules {
		analysis.AffectedTimeRules = append(analysis.AffectedTimeRules, rule.ID)
	}

	quotaRules, _ := s.repos.QuotaRule.GetByListID(ctx, listID)
	for _, rule := range quotaRules {
		analysis.AffectedQuotaRules = append(analysis.AffectedQuotaRules, rule.ID)
	}

	if operation == "delete" {
		analysis.RecommendedActions = append(analysis.RecommendedActions,
			fmt.Sprintf("Backup list data before deletion - %d entries, %d time rules, %d quota rules will be deleted",
				len(entries), len(timeRules), len(quotaRules)))
	}

	return analysis, nil
}

func (s *RuleValidationService) analyzeTimeRuleImpact(ctx context.Context, ruleID int, operation string, analysis *RuleImpactAnalysis) (*RuleImpactAnalysis, error) {
	rule, err := s.repos.TimeRule.GetByID(ctx, ruleID)
	if err != nil {
		return analysis, err
	}

	analysis.AffectedTimeRules = append(analysis.AffectedTimeRules, ruleID)
	analysis.AffectedLists = append(analysis.AffectedLists, rule.ListID)

	// Check for potential conflicts with other time rules
	otherRules, _ := s.repos.TimeRule.GetByListID(ctx, rule.ListID)
	for _, otherRule := range otherRules {
		if otherRule.ID != ruleID && s.scheduleOverlap(rule, &otherRule) {
			analysis.RecommendedActions = append(analysis.RecommendedActions,
				fmt.Sprintf("Review potential conflict with time rule '%s'", otherRule.Name))
		}
	}

	return analysis, nil
}

func (s *RuleValidationService) analyzeQuotaRuleImpact(ctx context.Context, ruleID int, operation string, analysis *RuleImpactAnalysis) (*RuleImpactAnalysis, error) {
	rule, err := s.repos.QuotaRule.GetByID(ctx, ruleID)
	if err != nil {
		return analysis, err
	}

	analysis.AffectedQuotaRules = append(analysis.AffectedQuotaRules, ruleID)
	analysis.AffectedLists = append(analysis.AffectedLists, rule.ListID)

	if operation == "delete" {
		analysis.RecommendedActions = append(analysis.RecommendedActions,
			"Consider backing up usage data before deleting quota rule")
	}

	return analysis, nil
}

func (s *RuleValidationService) analyzeEntryImpact(ctx context.Context, entryID int, operation string, analysis *RuleImpactAnalysis) (*RuleImpactAnalysis, error) {
	entry, err := s.repos.ListEntry.GetByID(ctx, entryID)
	if err != nil {
		return analysis, err
	}

	analysis.AffectedEntries = append(analysis.AffectedEntries, entryID)
	analysis.AffectedLists = append(analysis.AffectedLists, entry.ListID)

	return analysis, nil
}

// Conflict resolution methods (simplified)

func (s *RuleValidationService) resolveTimeOverlapConflict(ctx context.Context, conflictID string) error {
	// This would implement automatic resolution logic for time overlaps
	// For now, just return not implemented
	return fmt.Errorf("automatic resolution for time overlap conflicts not yet implemented")
}

func (s *RuleValidationService) resolveQuotaDuplicateConflict(ctx context.Context, conflictID string) error {
	// This would implement automatic resolution logic for quota duplicates
	// For now, just return not implemented
	return fmt.Errorf("automatic resolution for quota duplicate conflicts not yet implemented")
}
