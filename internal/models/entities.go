package models

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

// Config represents a configuration key-value pair
type Config struct {
	ID          int       `json:"id" db:"id"`
	Key         string    `json:"key" db:"key" validate:"required,max=255"`
	Value       string    `json:"value" db:"value" validate:"required"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// ListType represents the type of list (whitelist or blacklist)
type ListType string

const (
	ListTypeWhitelist ListType = "whitelist"
	ListTypeBlacklist ListType = "blacklist"
)

// List represents a whitelist or blacklist containing entries
type List struct {
	ID          int         `json:"id" db:"id"`
	Name        string      `json:"name" db:"name" validate:"required,max=255"`
	Type        ListType    `json:"type" db:"type" validate:"required,oneof=whitelist blacklist"`
	Description string      `json:"description" db:"description"`
	Enabled     bool        `json:"enabled" db:"enabled"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`
	Entries     []ListEntry `json:"entries,omitempty" db:"-"`
}

// EntryType represents the type of list entry (executable or URL)
type EntryType string

const (
	EntryTypeExecutable EntryType = "executable"
	EntryTypeURL        EntryType = "url"
)

// PatternType represents how to match the pattern
type PatternType string

const (
	PatternTypeExact    PatternType = "exact"
	PatternTypeWildcard PatternType = "wildcard"
	PatternTypeDomain   PatternType = "domain"
)

// ListEntry represents an entry in a list (executable or URL)
type ListEntry struct {
	ID          int         `json:"id" db:"id"`
	ListID      int         `json:"list_id" db:"list_id" validate:"required"`
	EntryType   EntryType   `json:"entry_type" db:"entry_type" validate:"required,oneof=executable url"`
	Pattern     string      `json:"pattern" db:"pattern" validate:"required,max=1000"`
	PatternType PatternType `json:"pattern_type" db:"pattern_type" validate:"required,oneof=exact wildcard domain"`
	Description string      `json:"description" db:"description"`
	Enabled     bool        `json:"enabled" db:"enabled"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`
}

// RuleType represents the type of time rule
type RuleType string

const (
	RuleTypeAllowDuring RuleType = "allow_during"
	RuleTypeBlockDuring RuleType = "block_during"
)

// TimeRule represents a time-based rule for when lists are active
type TimeRule struct {
	ID         int       `json:"id" db:"id"`
	ListID     int       `json:"list_id" db:"list_id" validate:"required"`
	Name       string    `json:"name" db:"name" validate:"required,max=255"`
	RuleType   RuleType  `json:"rule_type" db:"rule_type" validate:"required,oneof=allow_during block_during"`
	DaysOfWeek []int     `json:"days_of_week" db:"days_of_week" validate:"required,dive,min=0,max=6"`
	StartTime  string    `json:"start_time" db:"start_time" validate:"required"`
	EndTime    string    `json:"end_time" db:"end_time" validate:"required"`
	Enabled    bool      `json:"enabled" db:"enabled"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// MarshalDaysOfWeek converts the days of week slice to JSON for database storage
func (tr *TimeRule) MarshalDaysOfWeek() (string, error) {
	data, err := json.Marshal(tr.DaysOfWeek)
	return string(data), err
}

// UnmarshalDaysOfWeek converts the JSON string from database to days of week slice
func (tr *TimeRule) UnmarshalDaysOfWeek(data string) error {
	return json.Unmarshal([]byte(data), &tr.DaysOfWeek)
}

// ValidateTimeFormat validates that the time string is in HH:MM format
func ValidateTimeFormat(timeStr string) error {
	// Stricter validation to ensure HH:MM format
	re := regexp.MustCompile(`^([01]\d|2[0-3]):([0-5]\d)$`)
	if !re.MatchString(timeStr) {
		return fmt.Errorf("invalid time format, expected HH:MM")
	}

	// time.Parse is still useful for a final check on valid hours/minutes
	_, err := time.Parse("15:04", timeStr)
	if err != nil {
		return fmt.Errorf("invalid time value: %w", err)
	}
	return nil
}

// QuotaType represents the type of quota (daily, weekly, monthly)
type QuotaType string

const (
	QuotaTypeDaily   QuotaType = "daily"
	QuotaTypeWeekly  QuotaType = "weekly"
	QuotaTypeMonthly QuotaType = "monthly"
)

// QuotaRule represents a duration-based limit rule
type QuotaRule struct {
	ID           int       `json:"id" db:"id"`
	ListID       int       `json:"list_id" db:"list_id" validate:"required"`
	Name         string    `json:"name" db:"name" validate:"required,max=255"`
	QuotaType    QuotaType `json:"quota_type" db:"quota_type" validate:"required,oneof=daily weekly monthly"`
	LimitSeconds int       `json:"limit_seconds" db:"limit_seconds" validate:"required,min=1"`
	Enabled      bool      `json:"enabled" db:"enabled"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// GetLimitDuration returns the limit as a time.Duration
func (qr *QuotaRule) GetLimitDuration() time.Duration {
	return time.Duration(qr.LimitSeconds) * time.Second
}

// QuotaUsage tracks usage against quota rules
type QuotaUsage struct {
	ID          int       `json:"id" db:"id"`
	QuotaRuleID int       `json:"quota_rule_id" db:"quota_rule_id" validate:"required"`
	PeriodStart time.Time `json:"period_start" db:"period_start" validate:"required"`
	PeriodEnd   time.Time `json:"period_end" db:"period_end" validate:"required"`
	UsedSeconds int       `json:"used_seconds" db:"used_seconds"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// GetUsedDuration returns the used time as a time.Duration
func (qu *QuotaUsage) GetUsedDuration() time.Duration {
	return time.Duration(qu.UsedSeconds) * time.Second
}

// RemainingSeconds returns the remaining seconds in the quota
func (qu *QuotaUsage) RemainingSeconds(limitSeconds int) int {
	remaining := limitSeconds - qu.UsedSeconds
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ActionType represents the action taken (allow or block)
type ActionType string

const (
	ActionTypeAllow ActionType = "allow"
	ActionTypeBlock ActionType = "block"
)

// TargetType represents the type of target (executable or URL)
type TargetType string

const (
	TargetTypeExecutable TargetType = "executable"
	TargetTypeURL        TargetType = "url"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID          int        `json:"id" db:"id"`
	Timestamp   time.Time  `json:"timestamp" db:"timestamp"`
	EventType   string     `json:"event_type" db:"event_type" validate:"required,max=100"`
	TargetType  TargetType `json:"target_type" db:"target_type" validate:"required,oneof=executable url"`
	TargetValue string     `json:"target_value" db:"target_value" validate:"required,max=1000"`
	Action      ActionType `json:"action" db:"action" validate:"required,oneof=allow block"`
	RuleType    string     `json:"rule_type" db:"rule_type"`
	RuleID      *int       `json:"rule_id" db:"rule_id"`
	Details     string     `json:"details" db:"details"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
}

// GetDetailsMap parses the details JSON into a map
func (al *AuditLog) GetDetailsMap() (map[string]interface{}, error) {
	if al.Details == "" {
		return make(map[string]interface{}), nil
	}

	var details map[string]interface{}
	err := json.Unmarshal([]byte(al.Details), &details)
	return details, err
}

// SetDetailsMap converts a map to JSON and stores it in Details
func (al *AuditLog) SetDetailsMap(details map[string]interface{}) error {
	if details == nil {
		al.Details = ""
		return nil
	}

	data, err := json.Marshal(details)
	if err != nil {
		return err
	}

	al.Details = string(data)
	return nil
}

// SchemaVersion represents a database schema version
type SchemaVersion struct {
	Version     int       `json:"version" db:"version"`
	AppliedAt   time.Time `json:"applied_at" db:"applied_at"`
	Description string    `json:"description" db:"description"`
}

// ValidationError represents a validation error for a specific field
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error implements the error interface
func (ve ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", ve.Field, ve.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

// Error implements the error interface
func (ves ValidationErrors) Error() string {
	if len(ves) == 0 {
		return ""
	}

	if len(ves) == 1 {
		return ves[0].Error()
	}

	return fmt.Sprintf("multiple validation errors: %d errors", len(ves))
}

// HasErrors returns true if there are validation errors
func (ves ValidationErrors) HasErrors() bool {
	return len(ves) > 0
}

// Add adds a validation error
func (ves *ValidationErrors) Add(field, message string) {
	*ves = append(*ves, ValidationError{Field: field, Message: message})
}

// Summary statistics for the dashboard
type DashboardStats struct {
	TotalLists      int `json:"total_lists"`
	TotalEntries    int `json:"total_entries"`
	ActiveRules     int `json:"active_rules"`
	TodayBlocks     int `json:"today_blocks"`
	TodayAllows     int `json:"today_allows"`
	QuotasNearLimit int `json:"quotas_near_limit"`
}
