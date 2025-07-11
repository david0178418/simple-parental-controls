package models

import (
	"context"
	"time"
)

// Repository interfaces define the contract for data access

// ConfigRepository handles configuration data access
type ConfigRepository interface {
	Get(ctx context.Context, key string) (*Config, error)
	Set(ctx context.Context, key, value string) error
	GetAll(ctx context.Context) ([]Config, error)
	Delete(ctx context.Context, key string) error
	Update(ctx context.Context, config *Config) error
}

// ListRepository handles list data access
type ListRepository interface {
	Create(ctx context.Context, list *List) error
	GetByID(ctx context.Context, id int) (*List, error)
	GetByName(ctx context.Context, name string) (*List, error)
	GetAll(ctx context.Context) ([]List, error)
	GetByType(ctx context.Context, listType ListType) ([]List, error)
	GetEnabled(ctx context.Context) ([]List, error)
	Update(ctx context.Context, list *List) error
	Delete(ctx context.Context, id int) error
	Count(ctx context.Context) (int, error)
}

// ListEntryRepository handles list entry data access
type ListEntryRepository interface {
	Create(ctx context.Context, entry *ListEntry) error
	GetByID(ctx context.Context, id int) (*ListEntry, error)
	GetByListID(ctx context.Context, listID int) ([]ListEntry, error)
	GetByPattern(ctx context.Context, pattern string, entryType EntryType) ([]ListEntry, error)
	GetEnabled(ctx context.Context) ([]ListEntry, error)
	Update(ctx context.Context, entry *ListEntry) error
	Delete(ctx context.Context, id int) error
	DeleteByListID(ctx context.Context, listID int) error
	Count(ctx context.Context) (int, error)
	CountByListID(ctx context.Context, listID int) (int, error)
}

// TimeRuleRepository handles time rule data access
type TimeRuleRepository interface {
	Create(ctx context.Context, rule *TimeRule) error
	GetByID(ctx context.Context, id int) (*TimeRule, error)
	GetByListID(ctx context.Context, listID int) ([]TimeRule, error)
	GetEnabled(ctx context.Context) ([]TimeRule, error)
	GetActiveRules(ctx context.Context, now time.Time) ([]TimeRule, error)
	Update(ctx context.Context, rule *TimeRule) error
	Delete(ctx context.Context, id int) error
	DeleteByListID(ctx context.Context, listID int) error
	Count(ctx context.Context) (int, error)
}

// QuotaRuleRepository handles quota rule data access
type QuotaRuleRepository interface {
	Create(ctx context.Context, rule *QuotaRule) error
	GetByID(ctx context.Context, id int) (*QuotaRule, error)
	GetByListID(ctx context.Context, listID int) ([]QuotaRule, error)
	GetEnabled(ctx context.Context) ([]QuotaRule, error)
	Update(ctx context.Context, rule *QuotaRule) error
	Delete(ctx context.Context, id int) error
	DeleteByListID(ctx context.Context, listID int) error
	Count(ctx context.Context) (int, error)
}

// QuotaUsageRepository handles quota usage tracking
type QuotaUsageRepository interface {
	Create(ctx context.Context, usage *QuotaUsage) error
	GetByID(ctx context.Context, id int) (*QuotaUsage, error)
	GetByQuotaRuleID(ctx context.Context, quotaRuleID int) ([]QuotaUsage, error)
	GetCurrentUsage(ctx context.Context, quotaRuleID int, now time.Time) (*QuotaUsage, error)
	UpdateUsage(ctx context.Context, quotaRuleID int, additionalSeconds int, now time.Time) error
	GetUsageInPeriod(ctx context.Context, quotaRuleID int, start, end time.Time) (*QuotaUsage, error)
	CleanupExpiredUsage(ctx context.Context, before time.Time) error
	Update(ctx context.Context, usage *QuotaUsage) error
	Delete(ctx context.Context, id int) error
}

// AuditLogRepository handles audit log data access
type AuditLogRepository interface {
	Create(ctx context.Context, log *AuditLog) error
	GetByID(ctx context.Context, id int) (*AuditLog, error)
	GetAll(ctx context.Context, limit, offset int) ([]AuditLog, error)
	GetByTimeRange(ctx context.Context, start, end time.Time, limit, offset int) ([]AuditLog, error)
	GetByAction(ctx context.Context, action ActionType, limit, offset int) ([]AuditLog, error)
	GetByTargetType(ctx context.Context, targetType TargetType, limit, offset int) ([]AuditLog, error)
	GetTodayStats(ctx context.Context) (allows int, blocks int, err error)
	CleanupOldLogs(ctx context.Context, before time.Time) error
	Count(ctx context.Context) (int, error)
	CountByTimeRange(ctx context.Context, start, end time.Time) (int, error)
}

// SchemaVersionRepository handles schema version tracking
type SchemaVersionRepository interface {
	GetLatestVersion(ctx context.Context) (*SchemaVersion, error)
	GetAll(ctx context.Context) ([]SchemaVersion, error)
	Create(ctx context.Context, version *SchemaVersion) error
}

// DashboardRepository provides aggregated data for dashboard
type DashboardRepository interface {
	GetStats(ctx context.Context) (*DashboardStats, error)
	GetQuotasNearLimit(ctx context.Context, threshold float64) ([]QuotaUsage, error)
}

// RetentionPolicyRepository handles retention policy data access
type RetentionPolicyRepository interface {
	Create(ctx context.Context, policy *RetentionPolicy) error
	GetByID(ctx context.Context, id int) (*RetentionPolicy, error)
	GetAll(ctx context.Context) ([]RetentionPolicy, error)
	GetEnabled(ctx context.Context) ([]RetentionPolicy, error)
	GetByPriority(ctx context.Context) ([]RetentionPolicy, error) // Ordered by priority
	Update(ctx context.Context, policy *RetentionPolicy) error
	Delete(ctx context.Context, id int) error
	Count(ctx context.Context) (int, error)
}

// RetentionExecutionRepository handles retention execution tracking
type RetentionExecutionRepository interface {
	Create(ctx context.Context, execution *RetentionPolicyExecution) error
	GetByID(ctx context.Context, id int) (*RetentionPolicyExecution, error)
	GetByPolicyID(ctx context.Context, policyID int, limit, offset int) ([]RetentionPolicyExecution, error)
	GetRecent(ctx context.Context, limit int) ([]RetentionPolicyExecution, error)
	GetByStatus(ctx context.Context, status ExecutionStatus, limit, offset int) ([]RetentionPolicyExecution, error)
	GetByTimeRange(ctx context.Context, start, end time.Time, limit, offset int) ([]RetentionPolicyExecution, error)
	Update(ctx context.Context, execution *RetentionPolicyExecution) error
	Delete(ctx context.Context, id int) error
	GetStats(ctx context.Context) (*RetentionStats, error)
	CleanupOldExecutions(ctx context.Context, before time.Time) error
}

// LogRotationPolicyRepository handles log rotation policy data access
type LogRotationPolicyRepository interface {
	Create(ctx context.Context, policy *LogRotationPolicy) error
	GetByID(ctx context.Context, id int) (*LogRotationPolicy, error)
	GetAll(ctx context.Context) ([]LogRotationPolicy, error)
	GetEnabled(ctx context.Context) ([]LogRotationPolicy, error)
	GetByPriority(ctx context.Context) ([]LogRotationPolicy, error) // Ordered by priority
	Update(ctx context.Context, policy *LogRotationPolicy) error
	Delete(ctx context.Context, id int) error
	Count(ctx context.Context) (int, error)
}

// LogRotationExecutionRepository handles log rotation execution tracking
type LogRotationExecutionRepository interface {
	Create(ctx context.Context, execution *LogRotationExecution) error
	GetByID(ctx context.Context, id int) (*LogRotationExecution, error)
	GetByPolicyID(ctx context.Context, policyID int, limit, offset int) ([]LogRotationExecution, error)
	GetRecent(ctx context.Context, limit int) ([]LogRotationExecution, error)
	GetByStatus(ctx context.Context, status ExecutionStatus, limit, offset int) ([]LogRotationExecution, error)
	GetByTimeRange(ctx context.Context, start, end time.Time, limit, offset int) ([]LogRotationExecution, error)
	GetByTrigger(ctx context.Context, trigger RotationTrigger, limit, offset int) ([]LogRotationExecution, error)
	Update(ctx context.Context, execution *LogRotationExecution) error
	Delete(ctx context.Context, id int) error
	GetStats(ctx context.Context) (*RotationStats, error)
	CleanupOldExecutions(ctx context.Context, before time.Time) error
}

// RepositoryManager aggregates all repositories
type RepositoryManager struct {
	Config               ConfigRepository
	List                 ListRepository
	ListEntry            ListEntryRepository
	TimeRule             TimeRuleRepository
	QuotaRule            QuotaRuleRepository
	QuotaUsage           QuotaUsageRepository
	AuditLog             AuditLogRepository
	RetentionPolicy      RetentionPolicyRepository
	RetentionExecution   RetentionExecutionRepository
	LogRotationPolicy    LogRotationPolicyRepository
	LogRotationExecution LogRotationExecutionRepository
	SchemaVersion        SchemaVersionRepository
	Dashboard            DashboardRepository
}

// SearchFilters for advanced queries
type SearchFilters struct {
	Enabled   *bool
	ListType  *ListType
	EntryType *EntryType
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Offset    int
	Search    string
}

// DefaultSearchFilters returns filters with sensible defaults
func DefaultSearchFilters() SearchFilters {
	return SearchFilters{
		Limit:  50,
		Offset: 0,
	}
}
