package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// RetentionPolicy represents a configurable log retention policy
type RetentionPolicy struct {
	ID          int    `json:"id" db:"id"`
	Name        string `json:"name" db:"name" validate:"required,max=100"`
	Description string `json:"description" db:"description"`
	Enabled     bool   `json:"enabled" db:"enabled"`
	Priority    int    `json:"priority" db:"priority"` // Higher number = higher priority

	// Policy rules (at least one must be specified)
	TimeBasedRule  *TimeBasedRetention  `json:"time_based_rule,omitempty" db:"time_based_rule"`
	SizeBasedRule  *SizeBasedRetention  `json:"size_based_rule,omitempty" db:"size_based_rule"`
	CountBasedRule *CountBasedRetention `json:"count_based_rule,omitempty" db:"count_based_rule"`

	// Event filtering
	EventTypeFilter []string `json:"event_type_filter" db:"event_type_filter"` // Empty = all events
	ActionFilter    []string `json:"action_filter" db:"action_filter"`         // Empty = all actions

	// Execution settings
	ExecutionSchedule string    `json:"execution_schedule" db:"execution_schedule"` // Cron expression
	LastExecuted      time.Time `json:"last_executed" db:"last_executed"`
	NextExecution     time.Time `json:"next_execution" db:"next_execution"`

	// Metadata
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TimeBasedRetention defines time-based retention rules
type TimeBasedRetention struct {
	MaxAge      time.Duration `json:"max_age"`      // Maximum age before deletion
	GracePeriod time.Duration `json:"grace_period"` // Additional time before hard deletion

	// Tiered retention (optional)
	TierRules []TimeTierRule `json:"tier_rules,omitempty"`
}

// TimeTierRule defines tiered time-based retention
type TimeTierRule struct {
	AfterDuration time.Duration `json:"after_duration"` // After this duration
	Action        TierAction    `json:"action"`         // What to do
	KeepRatio     float64       `json:"keep_ratio"`     // For sampling actions (0.0-1.0)
}

// SizeBasedRetention defines size-based retention rules
type SizeBasedRetention struct {
	MaxTotalSize int64 `json:"max_total_size"` // Maximum total size in bytes
	MaxFileSize  int64 `json:"max_file_size"`  // Maximum individual file size

	// Cleanup strategy when size exceeded
	CleanupStrategy SizeCleanupStrategy `json:"cleanup_strategy"`
	CleanupRatio    float64             `json:"cleanup_ratio"` // Percentage to clean (0.0-1.0)
}

// CountBasedRetention defines count-based retention rules
type CountBasedRetention struct {
	MaxCount         int64 `json:"max_count"`          // Maximum number of log entries
	CleanupBatchSize int   `json:"cleanup_batch_size"` // Number to delete per cleanup run

	// Cleanup strategy when count exceeded
	CleanupStrategy CountCleanupStrategy `json:"cleanup_strategy"`
}

// TierAction defines what action to take in tiered retention
type TierAction string

const (
	TierActionDelete   TierAction = "delete"   // Delete entries
	TierActionArchive  TierAction = "archive"  // Move to archive storage
	TierActionCompress TierAction = "compress" // Compress entries
	TierActionSample   TierAction = "sample"   // Keep only a sample
)

// SizeCleanupStrategy defines how to clean up when size limits are exceeded
type SizeCleanupStrategy string

const (
	SizeCleanupOldest       SizeCleanupStrategy = "oldest_first"  // Delete oldest entries first
	SizeCleanupLargest      SizeCleanupStrategy = "largest_first" // Delete largest entries first
	SizeCleanupRandom       SizeCleanupStrategy = "random"        // Delete random entries
	SizeCleanupProportional SizeCleanupStrategy = "proportional"  // Delete proportionally by type
)

// CountCleanupStrategy defines how to clean up when count limits are exceeded
type CountCleanupStrategy string

const (
	CountCleanupOldest     CountCleanupStrategy = "oldest_first" // Delete oldest entries first
	CountCleanupRandom     CountCleanupStrategy = "random"       // Delete random entries
	CountCleanupRoundRobin CountCleanupStrategy = "round_robin"  // Delete evenly across event types
)

// RetentionPolicyExecution represents a retention policy execution record
type RetentionPolicyExecution struct {
	ID               int             `json:"id" db:"id"`
	PolicyID         int             `json:"policy_id" db:"policy_id"`
	ExecutionTime    time.Time       `json:"execution_time" db:"execution_time"`
	Status           ExecutionStatus `json:"status" db:"status"`
	EntriesProcessed int64           `json:"entries_processed" db:"entries_processed"`
	EntriesDeleted   int64           `json:"entries_deleted" db:"entries_deleted"`
	BytesFreed       int64           `json:"bytes_freed" db:"bytes_freed"`
	Duration         time.Duration   `json:"duration" db:"duration"`
	ErrorMessage     string          `json:"error_message,omitempty" db:"error_message"`
	Details          string          `json:"details" db:"details"` // JSON details
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
}

// ExecutionStatus represents the status of a retention policy execution
type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusCompleted ExecutionStatus = "completed"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusCancelled ExecutionStatus = "cancelled"
)

// RetentionStats represents statistics about retention operations
type RetentionStats struct {
	TotalPolicies       int                           `json:"total_policies"`
	ActivePolicies      int                           `json:"active_policies"`
	LastExecutionTime   time.Time                     `json:"last_execution_time"`
	TotalEntriesDeleted int64                         `json:"total_entries_deleted"`
	TotalBytesFreed     int64                         `json:"total_bytes_freed"`
	PolicyStats         map[int]*PolicyExecutionStats `json:"policy_stats"`
	RecentExecutions    []RetentionPolicyExecution    `json:"recent_executions"`
}

// PolicyExecutionStats represents statistics for a specific policy
type PolicyExecutionStats struct {
	PolicyID             int           `json:"policy_id"`
	PolicyName           string        `json:"policy_name"`
	ExecutionCount       int64         `json:"execution_count"`
	LastExecutionTime    time.Time     `json:"last_execution_time"`
	TotalEntriesDeleted  int64         `json:"total_entries_deleted"`
	TotalBytesFreed      int64         `json:"total_bytes_freed"`
	AverageExecutionTime time.Duration `json:"average_execution_time"`
	SuccessRate          float64       `json:"success_rate"`
}

// Validation methods

// Validate validates the retention policy
func (rp *RetentionPolicy) Validate() error {
	if rp.Name == "" {
		return fmt.Errorf("policy name is required")
	}

	// At least one retention rule must be specified
	if rp.TimeBasedRule == nil && rp.SizeBasedRule == nil && rp.CountBasedRule == nil {
		return fmt.Errorf("at least one retention rule must be specified")
	}

	// Validate individual rules
	if rp.TimeBasedRule != nil {
		if err := rp.TimeBasedRule.Validate(); err != nil {
			return fmt.Errorf("time-based rule validation failed: %w", err)
		}
	}

	if rp.SizeBasedRule != nil {
		if err := rp.SizeBasedRule.Validate(); err != nil {
			return fmt.Errorf("size-based rule validation failed: %w", err)
		}
	}

	if rp.CountBasedRule != nil {
		if err := rp.CountBasedRule.Validate(); err != nil {
			return fmt.Errorf("count-based rule validation failed: %w", err)
		}
	}

	return nil
}

// Validate validates time-based retention rules
func (tbr *TimeBasedRetention) Validate() error {
	if tbr.MaxAge <= 0 {
		return fmt.Errorf("max_age must be positive")
	}

	if tbr.GracePeriod < 0 {
		return fmt.Errorf("grace_period cannot be negative")
	}

	// Validate tier rules
	for i, tier := range tbr.TierRules {
		if tier.AfterDuration <= 0 {
			return fmt.Errorf("tier rule %d: after_duration must be positive", i)
		}
		if tier.Action == TierActionSample && (tier.KeepRatio < 0 || tier.KeepRatio > 1) {
			return fmt.Errorf("tier rule %d: keep_ratio must be between 0.0 and 1.0", i)
		}
	}

	return nil
}

// Validate validates size-based retention rules
func (sbr *SizeBasedRetention) Validate() error {
	if sbr.MaxTotalSize <= 0 {
		return fmt.Errorf("max_total_size must be positive")
	}

	if sbr.MaxFileSize < 0 {
		return fmt.Errorf("max_file_size cannot be negative")
	}

	if sbr.CleanupRatio < 0 || sbr.CleanupRatio > 1 {
		return fmt.Errorf("cleanup_ratio must be between 0.0 and 1.0")
	}

	return nil
}

// Validate validates count-based retention rules
func (cbr *CountBasedRetention) Validate() error {
	if cbr.MaxCount <= 0 {
		return fmt.Errorf("max_count must be positive")
	}

	if cbr.CleanupBatchSize <= 0 {
		return fmt.Errorf("cleanup_batch_size must be positive")
	}

	return nil
}

// Helper methods for JSON serialization

// SetTimeBasedRuleJSON sets the time-based rule from JSON
func (rp *RetentionPolicy) SetTimeBasedRuleJSON(data string) error {
	if data == "" {
		rp.TimeBasedRule = nil
		return nil
	}

	var rule TimeBasedRetention
	if err := json.Unmarshal([]byte(data), &rule); err != nil {
		return err
	}

	rp.TimeBasedRule = &rule
	return nil
}

// GetTimeBasedRuleJSON gets the time-based rule as JSON
func (rp *RetentionPolicy) GetTimeBasedRuleJSON() (string, error) {
	if rp.TimeBasedRule == nil {
		return "", nil
	}

	data, err := json.Marshal(rp.TimeBasedRule)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// SetSizeBasedRuleJSON sets the size-based rule from JSON
func (rp *RetentionPolicy) SetSizeBasedRuleJSON(data string) error {
	if data == "" {
		rp.SizeBasedRule = nil
		return nil
	}

	var rule SizeBasedRetention
	if err := json.Unmarshal([]byte(data), &rule); err != nil {
		return err
	}

	rp.SizeBasedRule = &rule
	return nil
}

// GetSizeBasedRuleJSON gets the size-based rule as JSON
func (rp *RetentionPolicy) GetSizeBasedRuleJSON() (string, error) {
	if rp.SizeBasedRule == nil {
		return "", nil
	}

	data, err := json.Marshal(rp.SizeBasedRule)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// SetCountBasedRuleJSON sets the count-based rule from JSON
func (rp *RetentionPolicy) SetCountBasedRuleJSON(data string) error {
	if data == "" {
		rp.CountBasedRule = nil
		return nil
	}

	var rule CountBasedRetention
	if err := json.Unmarshal([]byte(data), &rule); err != nil {
		return err
	}

	rp.CountBasedRule = &rule
	return nil
}

// GetCountBasedRuleJSON gets the count-based rule as JSON
func (rp *RetentionPolicy) GetCountBasedRuleJSON() (string, error) {
	if rp.CountBasedRule == nil {
		return "", nil
	}

	data, err := json.Marshal(rp.CountBasedRule)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// SetEventTypeFilterJSON sets the event type filter from JSON
func (rp *RetentionPolicy) SetEventTypeFilterJSON(data string) error {
	if data == "" {
		rp.EventTypeFilter = nil
		return nil
	}

	return json.Unmarshal([]byte(data), &rp.EventTypeFilter)
}

// GetEventTypeFilterJSON gets the event type filter as JSON
func (rp *RetentionPolicy) GetEventTypeFilterJSON() (string, error) {
	if len(rp.EventTypeFilter) == 0 {
		return "", nil
	}

	data, err := json.Marshal(rp.EventTypeFilter)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// SetActionFilterJSON sets the action filter from JSON
func (rp *RetentionPolicy) SetActionFilterJSON(data string) error {
	if data == "" {
		rp.ActionFilter = nil
		return nil
	}

	return json.Unmarshal([]byte(data), &rp.ActionFilter)
}

// GetActionFilterJSON gets the action filter as JSON
func (rp *RetentionPolicy) GetActionFilterJSON() (string, error) {
	if len(rp.ActionFilter) == 0 {
		return "", nil
	}

	data, err := json.Marshal(rp.ActionFilter)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// SetDetailsMap sets the execution details from a map
func (rpe *RetentionPolicyExecution) SetDetailsMap(details map[string]interface{}) error {
	if details == nil {
		rpe.Details = ""
		return nil
	}

	data, err := json.Marshal(details)
	if err != nil {
		return err
	}

	rpe.Details = string(data)
	return nil
}

// GetDetailsMap gets the execution details as a map
func (rpe *RetentionPolicyExecution) GetDetailsMap() (map[string]interface{}, error) {
	if rpe.Details == "" {
		return make(map[string]interface{}), nil
	}

	var details map[string]interface{}
	err := json.Unmarshal([]byte(rpe.Details), &details)
	return details, err
}
