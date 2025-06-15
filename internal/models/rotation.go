package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// LogRotationPolicy represents a configurable log rotation policy
type LogRotationPolicy struct {
	ID          int    `json:"id" db:"id"`
	Name        string `json:"name" db:"name" validate:"required,max=100"`
	Description string `json:"description" db:"description"`
	Enabled     bool   `json:"enabled" db:"enabled"`
	Priority    int    `json:"priority" db:"priority"`

	// Rotation triggers
	SizeBasedRotation *SizeBasedRotation `json:"size_based_rotation,omitempty" db:"size_based_rotation"`
	TimeBasedRotation *TimeBasedRotation `json:"time_based_rotation,omitempty" db:"time_based_rotation"`

	// Archival settings
	ArchivalPolicy *ArchivalPolicy `json:"archival_policy,omitempty" db:"archival_policy"`

	// Target configuration
	TargetLogFiles []string `json:"target_log_files" db:"target_log_files"` // File patterns to rotate
	TargetLogTypes []string `json:"target_log_types" db:"target_log_types"` // Log types to rotate

	// Emergency settings
	EmergencyConfig *EmergencyCleanupConfig `json:"emergency_config,omitempty" db:"emergency_config"`

	// Execution settings
	ExecutionSchedule string    `json:"execution_schedule" db:"execution_schedule"`
	LastExecuted      time.Time `json:"last_executed" db:"last_executed"`
	NextExecution     time.Time `json:"next_execution" db:"next_execution"`

	// Metadata
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// SizeBasedRotation defines size-based log rotation rules
type SizeBasedRotation struct {
	MaxFileSize       int64   `json:"max_file_size"`      // Maximum file size before rotation (bytes)
	MaxTotalSize      int64   `json:"max_total_size"`     // Maximum total size for all files
	RotationThreshold float64 `json:"rotation_threshold"` // Threshold percentage (0.0-1.0)
}

// TimeBasedRotation defines time-based log rotation rules
type TimeBasedRotation struct {
	RotationInterval time.Duration `json:"rotation_interval"`  // How often to rotate
	RetainDuration   time.Duration `json:"retain_duration"`    // How long to keep files
	ArchiveOlderThan time.Duration `json:"archive_older_than"` // When to archive files
}

// ArchivalPolicy defines archival and compression settings
type ArchivalPolicy struct {
	EnableCompression bool            `json:"enable_compression"`
	CompressionFormat CompressionType `json:"compression_format"`
	ArchiveLocation   string          `json:"archive_location"`
	MaxArchiveSize    int64           `json:"max_archive_size"`
	ArchiveRetention  time.Duration   `json:"archive_retention"`
	CompressionLevel  int             `json:"compression_level"` // 1-9 for gzip
	EncryptArchives   bool            `json:"encrypt_archives"`
}

// EmergencyCleanupConfig defines emergency disk space protection
type EmergencyCleanupConfig struct {
	DiskSpaceThreshold float64           `json:"disk_space_threshold"` // Percentage of disk usage to trigger emergency
	EmergencyActions   []EmergencyAction `json:"emergency_actions"`
	MonitoringInterval time.Duration     `json:"monitoring_interval"`
	AlertThresholds    []AlertThreshold  `json:"alert_thresholds"`
}

// EmergencyAction defines actions to take during emergency cleanup
type EmergencyAction struct {
	Priority    int           `json:"priority"` // Lower number = higher priority
	ActionType  EmergencyType `json:"action_type"`
	TargetFiles []string      `json:"target_files"` // File patterns to target
	MaxDelete   int64         `json:"max_delete"`   // Maximum files/bytes to delete
	Description string        `json:"description"`
}

// AlertThreshold defines alert levels for disk space monitoring
type AlertThreshold struct {
	DiskUsagePercent     float64    `json:"disk_usage_percent"`
	AlertLevel           AlertLevel `json:"alert_level"`
	NotificationChannels []string   `json:"notification_channels"`
}

// LogRotationExecution represents a log rotation execution record
type LogRotationExecution struct {
	ID            int             `json:"id" db:"id"`
	PolicyID      int             `json:"policy_id" db:"policy_id"`
	ExecutionTime time.Time       `json:"execution_time" db:"execution_time"`
	Status        ExecutionStatus `json:"status" db:"status"`
	TriggerReason RotationTrigger `json:"trigger_reason" db:"trigger_reason"`

	// Results
	FilesRotated     int     `json:"files_rotated" db:"files_rotated"`
	FilesArchived    int     `json:"files_archived" db:"files_archived"`
	FilesDeleted     int     `json:"files_deleted" db:"files_deleted"`
	BytesCompressed  int64   `json:"bytes_compressed" db:"bytes_compressed"`
	BytesFreed       int64   `json:"bytes_freed" db:"bytes_freed"`
	CompressionRatio float64 `json:"compression_ratio" db:"compression_ratio"`

	// Performance
	Duration     time.Duration `json:"duration" db:"duration"`
	ErrorMessage string        `json:"error_message,omitempty" db:"error_message"`
	Details      string        `json:"details" db:"details"` // JSON details
	CreatedAt    time.Time     `json:"created_at" db:"created_at"`
}

// DiskSpaceInfo represents current disk space information
type DiskSpaceInfo struct {
	TotalSpace   int64     `json:"total_space"`
	UsedSpace    int64     `json:"used_space"`
	FreeSpace    int64     `json:"free_space"`
	UsagePercent float64   `json:"usage_percent"`
	LogFilesSize int64     `json:"log_files_size"`
	ArchiveSize  int64     `json:"archive_size"`
	LastUpdated  time.Time `json:"last_updated"`
}

// RotationStats represents statistics about rotation operations
type RotationStats struct {
	TotalPolicies           int                          `json:"total_policies"`
	ActivePolicies          int                          `json:"active_policies"`
	LastRotationTime        time.Time                    `json:"last_rotation_time"`
	TotalFilesRotated       int64                        `json:"total_files_rotated"`
	TotalBytesFreed         int64                        `json:"total_bytes_freed"`
	TotalBytesCompressed    int64                        `json:"total_bytes_compressed"`
	AverageCompressionRatio float64                      `json:"average_compression_ratio"`
	DiskSpaceInfo           *DiskSpaceInfo               `json:"disk_space_info"`
	PolicyStats             map[int]*PolicyRotationStats `json:"policy_stats"`
	RecentExecutions        []LogRotationExecution       `json:"recent_executions"`
	EmergencyTriggers       int64                        `json:"emergency_triggers"`
}

// PolicyRotationStats represents statistics for a specific rotation policy
type PolicyRotationStats struct {
	PolicyID                int           `json:"policy_id"`
	PolicyName              string        `json:"policy_name"`
	ExecutionCount          int64         `json:"execution_count"`
	LastExecutionTime       time.Time     `json:"last_execution_time"`
	TotalFilesRotated       int64         `json:"total_files_rotated"`
	TotalBytesFreed         int64         `json:"total_bytes_freed"`
	AverageExecutionTime    time.Duration `json:"average_execution_time"`
	SuccessRate             float64       `json:"success_rate"`
	AverageCompressionRatio float64       `json:"average_compression_ratio"`
}

// Enum types

// CompressionType defines available compression formats
type CompressionType string

const (
	CompressionGzip  CompressionType = "gzip"
	CompressionBzip2 CompressionType = "bzip2"
	CompressionLz4   CompressionType = "lz4"
	CompressionZstd  CompressionType = "zstd"
)

// RotationTrigger defines what triggered a rotation
type RotationTrigger string

const (
	TriggerScheduled RotationTrigger = "scheduled"
	TriggerSize      RotationTrigger = "size_limit"
	TriggerTime      RotationTrigger = "time_limit"
	TriggerEmergency RotationTrigger = "emergency"
	TriggerManual    RotationTrigger = "manual"
)

// EmergencyType defines types of emergency actions
type EmergencyType string

const (
	EmergencyDeleteOldest  EmergencyType = "delete_oldest"
	EmergencyDeleteLargest EmergencyType = "delete_largest"
	EmergencyCompress      EmergencyType = "compress_files"
	EmergencyArchive       EmergencyType = "archive_files"
	EmergencyTruncate      EmergencyType = "truncate_files"
)

// AlertLevel defines severity levels for alerts
type AlertLevel string

const (
	AlertLevelInfo      AlertLevel = "info"
	AlertLevelWarning   AlertLevel = "warning"
	AlertLevelCritical  AlertLevel = "critical"
	AlertLevelEmergency AlertLevel = "emergency"
)

// FileRotationInfo represents information about a rotated file
type FileRotationInfo struct {
	OriginalPath     string    `json:"original_path"`
	RotatedPath      string    `json:"rotated_path"`
	ArchivePath      string    `json:"archive_path,omitempty"`
	OriginalSize     int64     `json:"original_size"`
	CompressedSize   int64     `json:"compressed_size,omitempty"`
	CompressionRatio float64   `json:"compression_ratio,omitempty"`
	RotatedAt        time.Time `json:"rotated_at"`
	Checksum         string    `json:"checksum,omitempty"`
}

// Validation methods

// Validate validates the log rotation policy
func (lrp *LogRotationPolicy) Validate() error {
	if lrp.Name == "" {
		return fmt.Errorf("policy name is required")
	}

	// At least one rotation rule must be specified
	if lrp.SizeBasedRotation == nil && lrp.TimeBasedRotation == nil {
		return fmt.Errorf("at least one rotation rule must be specified")
	}

	// Validate individual rules
	if lrp.SizeBasedRotation != nil {
		if err := lrp.SizeBasedRotation.Validate(); err != nil {
			return fmt.Errorf("size-based rotation validation failed: %w", err)
		}
	}

	if lrp.TimeBasedRotation != nil {
		if err := lrp.TimeBasedRotation.Validate(); err != nil {
			return fmt.Errorf("time-based rotation validation failed: %w", err)
		}
	}

	if lrp.ArchivalPolicy != nil {
		if err := lrp.ArchivalPolicy.Validate(); err != nil {
			return fmt.Errorf("archival policy validation failed: %w", err)
		}
	}

	if lrp.EmergencyConfig != nil {
		if err := lrp.EmergencyConfig.Validate(); err != nil {
			return fmt.Errorf("emergency config validation failed: %w", err)
		}
	}

	return nil
}

// Validate validates size-based rotation rules
func (sbr *SizeBasedRotation) Validate() error {
	if sbr.MaxFileSize <= 0 {
		return fmt.Errorf("max_file_size must be positive")
	}

	if sbr.MaxTotalSize > 0 && sbr.MaxTotalSize < sbr.MaxFileSize {
		return fmt.Errorf("max_total_size must be greater than max_file_size")
	}

	if sbr.RotationThreshold < 0 || sbr.RotationThreshold > 1 {
		return fmt.Errorf("rotation_threshold must be between 0.0 and 1.0")
	}

	return nil
}

// Validate validates time-based rotation rules
func (tbr *TimeBasedRotation) Validate() error {
	if tbr.RotationInterval <= 0 {
		return fmt.Errorf("rotation_interval must be positive")
	}

	if tbr.RetainDuration <= 0 {
		return fmt.Errorf("retain_duration must be positive")
	}

	if tbr.ArchiveOlderThan < 0 {
		return fmt.Errorf("archive_older_than cannot be negative")
	}

	return nil
}

// Validate validates archival policy
func (ap *ArchivalPolicy) Validate() error {
	if ap.ArchiveLocation == "" {
		return fmt.Errorf("archive_location is required")
	}

	if ap.MaxArchiveSize <= 0 {
		return fmt.Errorf("max_archive_size must be positive")
	}

	if ap.ArchiveRetention <= 0 {
		return fmt.Errorf("archive_retention must be positive")
	}

	if ap.CompressionLevel < 1 || ap.CompressionLevel > 9 {
		return fmt.Errorf("compression_level must be between 1 and 9")
	}

	return nil
}

// Validate validates emergency cleanup config
func (ecc *EmergencyCleanupConfig) Validate() error {
	if ecc.DiskSpaceThreshold <= 0 || ecc.DiskSpaceThreshold >= 1 {
		return fmt.Errorf("disk_space_threshold must be between 0.0 and 1.0")
	}

	if len(ecc.EmergencyActions) == 0 {
		return fmt.Errorf("at least one emergency action must be specified")
	}

	if ecc.MonitoringInterval <= 0 {
		return fmt.Errorf("monitoring_interval must be positive")
	}

	return nil
}

// JSON serialization helpers (similar to retention.go)

// SetSizeBasedRotationJSON sets the size-based rotation from JSON
func (lrp *LogRotationPolicy) SetSizeBasedRotationJSON(data string) error {
	if data == "" {
		lrp.SizeBasedRotation = nil
		return nil
	}

	var rotation SizeBasedRotation
	if err := json.Unmarshal([]byte(data), &rotation); err != nil {
		return err
	}

	lrp.SizeBasedRotation = &rotation
	return nil
}

// GetSizeBasedRotationJSON gets the size-based rotation as JSON
func (lrp *LogRotationPolicy) GetSizeBasedRotationJSON() (string, error) {
	if lrp.SizeBasedRotation == nil {
		return "", nil
	}

	data, err := json.Marshal(lrp.SizeBasedRotation)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// SetTimeBasedRotationJSON sets the time-based rotation from JSON
func (lrp *LogRotationPolicy) SetTimeBasedRotationJSON(data string) error {
	if data == "" {
		lrp.TimeBasedRotation = nil
		return nil
	}

	var rotation TimeBasedRotation
	if err := json.Unmarshal([]byte(data), &rotation); err != nil {
		return err
	}

	lrp.TimeBasedRotation = &rotation
	return nil
}

// GetTimeBasedRotationJSON gets the time-based rotation as JSON
func (lrp *LogRotationPolicy) GetTimeBasedRotationJSON() (string, error) {
	if lrp.TimeBasedRotation == nil {
		return "", nil
	}

	data, err := json.Marshal(lrp.TimeBasedRotation)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// SetArchivalPolicyJSON sets the archival policy from JSON
func (lrp *LogRotationPolicy) SetArchivalPolicyJSON(data string) error {
	if data == "" {
		lrp.ArchivalPolicy = nil
		return nil
	}

	var policy ArchivalPolicy
	if err := json.Unmarshal([]byte(data), &policy); err != nil {
		return err
	}

	lrp.ArchivalPolicy = &policy
	return nil
}

// GetArchivalPolicyJSON gets the archival policy as JSON
func (lrp *LogRotationPolicy) GetArchivalPolicyJSON() (string, error) {
	if lrp.ArchivalPolicy == nil {
		return "", nil
	}

	data, err := json.Marshal(lrp.ArchivalPolicy)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// SetEmergencyConfigJSON sets the emergency config from JSON
func (lrp *LogRotationPolicy) SetEmergencyConfigJSON(data string) error {
	if data == "" {
		lrp.EmergencyConfig = nil
		return nil
	}

	var config EmergencyCleanupConfig
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return err
	}

	lrp.EmergencyConfig = &config
	return nil
}

// GetEmergencyConfigJSON gets the emergency config as JSON
func (lrp *LogRotationPolicy) GetEmergencyConfigJSON() (string, error) {
	if lrp.EmergencyConfig == nil {
		return "", nil
	}

	data, err := json.Marshal(lrp.EmergencyConfig)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// SetTargetLogFilesJSON sets the target log files from JSON
func (lrp *LogRotationPolicy) SetTargetLogFilesJSON(data string) error {
	if data == "" {
		lrp.TargetLogFiles = nil
		return nil
	}

	return json.Unmarshal([]byte(data), &lrp.TargetLogFiles)
}

// GetTargetLogFilesJSON gets the target log files as JSON
func (lrp *LogRotationPolicy) GetTargetLogFilesJSON() (string, error) {
	if len(lrp.TargetLogFiles) == 0 {
		return "", nil
	}

	data, err := json.Marshal(lrp.TargetLogFiles)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// SetTargetLogTypesJSON sets the target log types from JSON
func (lrp *LogRotationPolicy) SetTargetLogTypesJSON(data string) error {
	if data == "" {
		lrp.TargetLogTypes = nil
		return nil
	}

	return json.Unmarshal([]byte(data), &lrp.TargetLogTypes)
}

// GetTargetLogTypesJSON gets the target log types as JSON
func (lrp *LogRotationPolicy) GetTargetLogTypesJSON() (string, error) {
	if len(lrp.TargetLogTypes) == 0 {
		return "", nil
	}

	data, err := json.Marshal(lrp.TargetLogTypes)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// SetDetailsMap sets the execution details from a map
func (lre *LogRotationExecution) SetDetailsMap(details map[string]interface{}) error {
	if details == nil {
		lre.Details = ""
		return nil
	}

	data, err := json.Marshal(details)
	if err != nil {
		return err
	}

	lre.Details = string(data)
	return nil
}

// GetDetailsMap gets the execution details as a map
func (lre *LogRotationExecution) GetDetailsMap() (map[string]interface{}, error) {
	if lre.Details == "" {
		return make(map[string]interface{}), nil
	}

	var details map[string]interface{}
	err := json.Unmarshal([]byte(lre.Details), &details)
	return details, err
}
