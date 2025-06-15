package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"parental-control/internal/models"
)

// RetentionPolicyRepository implements the models.RetentionPolicyRepository interface
type RetentionPolicyRepository struct {
	db *sql.DB
}

// NewRetentionPolicyRepository creates a new retention policy repository
func NewRetentionPolicyRepository(db *sql.DB) *RetentionPolicyRepository {
	return &RetentionPolicyRepository{db: db}
}

// Create creates a new retention policy
func (r *RetentionPolicyRepository) Create(ctx context.Context, policy *models.RetentionPolicy) error {
	// Serialize JSON fields
	timeBasedRule, err := policy.GetTimeBasedRuleJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize time-based rule: %w", err)
	}

	sizeBasedRule, err := policy.GetSizeBasedRuleJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize size-based rule: %w", err)
	}

	countBasedRule, err := policy.GetCountBasedRuleJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize count-based rule: %w", err)
	}

	eventTypeFilter, err := policy.GetEventTypeFilterJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize event type filter: %w", err)
	}

	actionFilter, err := policy.GetActionFilterJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize action filter: %w", err)
	}

	query := `
		INSERT INTO retention_policies (
			name, description, enabled, priority,
			time_based_rule, size_based_rule, count_based_rule,
			event_type_filter, action_filter,
			execution_schedule, last_executed, next_execution
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query,
		policy.Name,
		policy.Description,
		policy.Enabled,
		policy.Priority,
		nullString(timeBasedRule),
		nullString(sizeBasedRule),
		nullString(countBasedRule),
		nullString(eventTypeFilter),
		nullString(actionFilter),
		policy.ExecutionSchedule,
		nullTime(policy.LastExecuted),
		nullTime(policy.NextExecution),
	)
	if err != nil {
		return fmt.Errorf("failed to create retention policy: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get retention policy ID: %w", err)
	}

	policy.ID = int(id)
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	return nil
}

// GetByID retrieves a retention policy by ID
func (r *RetentionPolicyRepository) GetByID(ctx context.Context, id int) (*models.RetentionPolicy, error) {
	query := `
		SELECT id, name, description, enabled, priority,
			   time_based_rule, size_based_rule, count_based_rule,
			   event_type_filter, action_filter,
			   execution_schedule, last_executed, next_execution,
			   created_at, updated_at
		FROM retention_policies
		WHERE id = ?
	`

	policy := &models.RetentionPolicy{}
	var timeBasedRule, sizeBasedRule, countBasedRule sql.NullString
	var eventTypeFilter, actionFilter sql.NullString
	var lastExecuted, nextExecution sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&policy.ID,
		&policy.Name,
		&policy.Description,
		&policy.Enabled,
		&policy.Priority,
		&timeBasedRule,
		&sizeBasedRule,
		&countBasedRule,
		&eventTypeFilter,
		&actionFilter,
		&policy.ExecutionSchedule,
		&lastExecuted,
		&nextExecution,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("retention policy with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get retention policy: %w", err)
	}

	// Deserialize JSON fields
	if err := r.deserializePolicy(policy, timeBasedRule, sizeBasedRule, countBasedRule, eventTypeFilter, actionFilter); err != nil {
		return nil, err
	}

	if lastExecuted.Valid {
		policy.LastExecuted = lastExecuted.Time
	}
	if nextExecution.Valid {
		policy.NextExecution = nextExecution.Time
	}

	return policy, nil
}

// GetAll retrieves all retention policies
func (r *RetentionPolicyRepository) GetAll(ctx context.Context) ([]models.RetentionPolicy, error) {
	query := `
		SELECT id, name, description, enabled, priority,
			   time_based_rule, size_based_rule, count_based_rule,
			   event_type_filter, action_filter,
			   execution_schedule, last_executed, next_execution,
			   created_at, updated_at
		FROM retention_policies
		ORDER BY priority DESC, name ASC
	`

	return r.queryPolicies(ctx, query)
}

// GetEnabled retrieves all enabled retention policies
func (r *RetentionPolicyRepository) GetEnabled(ctx context.Context) ([]models.RetentionPolicy, error) {
	query := `
		SELECT id, name, description, enabled, priority,
			   time_based_rule, size_based_rule, count_based_rule,
			   event_type_filter, action_filter,
			   execution_schedule, last_executed, next_execution,
			   created_at, updated_at
		FROM retention_policies
		WHERE enabled = 1
		ORDER BY priority DESC, name ASC
	`

	return r.queryPolicies(ctx, query)
}

// GetByPriority retrieves retention policies ordered by priority
func (r *RetentionPolicyRepository) GetByPriority(ctx context.Context) ([]models.RetentionPolicy, error) {
	query := `
		SELECT id, name, description, enabled, priority,
			   time_based_rule, size_based_rule, count_based_rule,
			   event_type_filter, action_filter,
			   execution_schedule, last_executed, next_execution,
			   created_at, updated_at
		FROM retention_policies
		ORDER BY priority DESC, created_at ASC
	`

	return r.queryPolicies(ctx, query)
}

// Update updates a retention policy
func (r *RetentionPolicyRepository) Update(ctx context.Context, policy *models.RetentionPolicy) error {
	// Serialize JSON fields
	timeBasedRule, err := policy.GetTimeBasedRuleJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize time-based rule: %w", err)
	}

	sizeBasedRule, err := policy.GetSizeBasedRuleJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize size-based rule: %w", err)
	}

	countBasedRule, err := policy.GetCountBasedRuleJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize count-based rule: %w", err)
	}

	eventTypeFilter, err := policy.GetEventTypeFilterJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize event type filter: %w", err)
	}

	actionFilter, err := policy.GetActionFilterJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize action filter: %w", err)
	}

	query := `
		UPDATE retention_policies SET
			name = ?, description = ?, enabled = ?, priority = ?,
			time_based_rule = ?, size_based_rule = ?, count_based_rule = ?,
			event_type_filter = ?, action_filter = ?,
			execution_schedule = ?, last_executed = ?, next_execution = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		policy.Name,
		policy.Description,
		policy.Enabled,
		policy.Priority,
		nullString(timeBasedRule),
		nullString(sizeBasedRule),
		nullString(countBasedRule),
		nullString(eventTypeFilter),
		nullString(actionFilter),
		policy.ExecutionSchedule,
		nullTime(policy.LastExecuted),
		nullTime(policy.NextExecution),
		policy.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update retention policy: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get update result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("retention policy with ID %d not found", policy.ID)
	}

	policy.UpdatedAt = time.Now()
	return nil
}

// Delete deletes a retention policy
func (r *RetentionPolicyRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM retention_policies WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete retention policy: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get delete result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("retention policy with ID %d not found", id)
	}

	return nil
}

// Count returns the total number of retention policies
func (r *RetentionPolicyRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM retention_policies`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count retention policies: %w", err)
	}

	return count, nil
}

// Helper methods

func (r *RetentionPolicyRepository) queryPolicies(ctx context.Context, query string, args ...interface{}) ([]models.RetentionPolicy, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query retention policies: %w", err)
	}
	defer rows.Close()

	var policies []models.RetentionPolicy
	for rows.Next() {
		policy := models.RetentionPolicy{}
		var timeBasedRule, sizeBasedRule, countBasedRule sql.NullString
		var eventTypeFilter, actionFilter sql.NullString
		var lastExecuted, nextExecution sql.NullTime

		err := rows.Scan(
			&policy.ID,
			&policy.Name,
			&policy.Description,
			&policy.Enabled,
			&policy.Priority,
			&timeBasedRule,
			&sizeBasedRule,
			&countBasedRule,
			&eventTypeFilter,
			&actionFilter,
			&policy.ExecutionSchedule,
			&lastExecuted,
			&nextExecution,
			&policy.CreatedAt,
			&policy.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan retention policy: %w", err)
		}

		// Deserialize JSON fields
		if err := r.deserializePolicy(&policy, timeBasedRule, sizeBasedRule, countBasedRule, eventTypeFilter, actionFilter); err != nil {
			return nil, err
		}

		if lastExecuted.Valid {
			policy.LastExecuted = lastExecuted.Time
		}
		if nextExecution.Valid {
			policy.NextExecution = nextExecution.Time
		}

		policies = append(policies, policy)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating retention policies: %w", err)
	}

	return policies, nil
}

func (r *RetentionPolicyRepository) deserializePolicy(policy *models.RetentionPolicy, timeBasedRule, sizeBasedRule, countBasedRule, eventTypeFilter, actionFilter sql.NullString) error {
	if timeBasedRule.Valid {
		if err := policy.SetTimeBasedRuleJSON(timeBasedRule.String); err != nil {
			return fmt.Errorf("failed to deserialize time-based rule: %w", err)
		}
	}

	if sizeBasedRule.Valid {
		if err := policy.SetSizeBasedRuleJSON(sizeBasedRule.String); err != nil {
			return fmt.Errorf("failed to deserialize size-based rule: %w", err)
		}
	}

	if countBasedRule.Valid {
		if err := policy.SetCountBasedRuleJSON(countBasedRule.String); err != nil {
			return fmt.Errorf("failed to deserialize count-based rule: %w", err)
		}
	}

	if eventTypeFilter.Valid {
		if err := policy.SetEventTypeFilterJSON(eventTypeFilter.String); err != nil {
			return fmt.Errorf("failed to deserialize event type filter: %w", err)
		}
	}

	if actionFilter.Valid {
		if err := policy.SetActionFilterJSON(actionFilter.String); err != nil {
			return fmt.Errorf("failed to deserialize action filter: %w", err)
		}
	}

	return nil
}

// Helper functions for null handling
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: t, Valid: true}
}

// RetentionExecutionRepository implements the models.RetentionExecutionRepository interface
type RetentionExecutionRepository struct {
	db *sql.DB
}

// NewRetentionExecutionRepository creates a new retention execution repository
func NewRetentionExecutionRepository(db *sql.DB) *RetentionExecutionRepository {
	return &RetentionExecutionRepository{db: db}
}

// Create creates a new retention policy execution record
func (r *RetentionExecutionRepository) Create(ctx context.Context, execution *models.RetentionPolicyExecution) error {
	query := `
		INSERT INTO retention_policy_executions (
			policy_id, execution_time, status, entries_processed, entries_deleted,
			bytes_freed, duration, error_message, details
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query,
		execution.PolicyID,
		execution.ExecutionTime,
		execution.Status,
		execution.EntriesProcessed,
		execution.EntriesDeleted,
		execution.BytesFreed,
		execution.Duration.Nanoseconds(),
		execution.ErrorMessage,
		execution.Details,
	)
	if err != nil {
		return fmt.Errorf("failed to create retention execution: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get retention execution ID: %w", err)
	}

	execution.ID = int(id)
	execution.CreatedAt = time.Now()

	return nil
}

// GetByID retrieves a retention execution by ID
func (r *RetentionExecutionRepository) GetByID(ctx context.Context, id int) (*models.RetentionPolicyExecution, error) {
	query := `
		SELECT id, policy_id, execution_time, status, entries_processed, entries_deleted,
			   bytes_freed, duration, error_message, details, created_at
		FROM retention_policy_executions
		WHERE id = ?
	`

	execution := &models.RetentionPolicyExecution{}
	var durationNanos int64

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&execution.ID,
		&execution.PolicyID,
		&execution.ExecutionTime,
		&execution.Status,
		&execution.EntriesProcessed,
		&execution.EntriesDeleted,
		&execution.BytesFreed,
		&durationNanos,
		&execution.ErrorMessage,
		&execution.Details,
		&execution.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("retention execution with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get retention execution: %w", err)
	}

	execution.Duration = time.Duration(durationNanos)
	return execution, nil
}

// GetByPolicyID retrieves executions for a specific policy
func (r *RetentionExecutionRepository) GetByPolicyID(ctx context.Context, policyID int, limit, offset int) ([]models.RetentionPolicyExecution, error) {
	query := `
		SELECT id, policy_id, execution_time, status, entries_processed, entries_deleted,
			   bytes_freed, duration, error_message, details, created_at
		FROM retention_policy_executions
		WHERE policy_id = ?
		ORDER BY execution_time DESC
		LIMIT ? OFFSET ?
	`

	return r.queryExecutions(ctx, query, policyID, limit, offset)
}

// GetRecent retrieves recent executions
func (r *RetentionExecutionRepository) GetRecent(ctx context.Context, limit int) ([]models.RetentionPolicyExecution, error) {
	query := `
		SELECT id, policy_id, execution_time, status, entries_processed, entries_deleted,
			   bytes_freed, duration, error_message, details, created_at
		FROM retention_policy_executions
		ORDER BY execution_time DESC
		LIMIT ?
	`

	return r.queryExecutions(ctx, query, limit)
}

// GetByStatus retrieves executions by status
func (r *RetentionExecutionRepository) GetByStatus(ctx context.Context, status models.ExecutionStatus, limit, offset int) ([]models.RetentionPolicyExecution, error) {
	query := `
		SELECT id, policy_id, execution_time, status, entries_processed, entries_deleted,
			   bytes_freed, duration, error_message, details, created_at
		FROM retention_policy_executions
		WHERE status = ?
		ORDER BY execution_time DESC
		LIMIT ? OFFSET ?
	`

	return r.queryExecutions(ctx, query, status, limit, offset)
}

// GetByTimeRange retrieves executions within a time range
func (r *RetentionExecutionRepository) GetByTimeRange(ctx context.Context, start, end time.Time, limit, offset int) ([]models.RetentionPolicyExecution, error) {
	query := `
		SELECT id, policy_id, execution_time, status, entries_processed, entries_deleted,
			   bytes_freed, duration, error_message, details, created_at
		FROM retention_policy_executions
		WHERE execution_time >= ? AND execution_time <= ?
		ORDER BY execution_time DESC
		LIMIT ? OFFSET ?
	`

	return r.queryExecutions(ctx, query, start, end, limit, offset)
}

// Update updates a retention execution
func (r *RetentionExecutionRepository) Update(ctx context.Context, execution *models.RetentionPolicyExecution) error {
	query := `
		UPDATE retention_policy_executions SET
			status = ?, entries_processed = ?, entries_deleted = ?,
			bytes_freed = ?, duration = ?, error_message = ?, details = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		execution.Status,
		execution.EntriesProcessed,
		execution.EntriesDeleted,
		execution.BytesFreed,
		execution.Duration.Nanoseconds(),
		execution.ErrorMessage,
		execution.Details,
		execution.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update retention execution: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get update result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("retention execution with ID %d not found", execution.ID)
	}

	return nil
}

// Delete deletes a retention execution
func (r *RetentionExecutionRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM retention_policy_executions WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete retention execution: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get delete result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("retention execution with ID %d not found", id)
	}

	return nil
}

// GetStats retrieves retention statistics
func (r *RetentionExecutionRepository) GetStats(ctx context.Context) (*models.RetentionStats, error) {
	// This is a simplified implementation - in practice, you might want more complex aggregations
	stats := &models.RetentionStats{
		PolicyStats: make(map[int]*models.PolicyExecutionStats),
	}

	// Get basic counts
	err := r.db.QueryRowContext(ctx, `
		SELECT 
			COUNT(DISTINCT rp.id) as total_policies,
			COUNT(DISTINCT CASE WHEN rp.enabled = 1 THEN rp.id END) as active_policies
		FROM retention_policies rp
	`).Scan(&stats.TotalPolicies, &stats.ActivePolicies)
	if err != nil {
		return nil, fmt.Errorf("failed to get policy counts: %w", err)
	}

	// Get execution stats
	err = r.db.QueryRowContext(ctx, `
		SELECT 
			COALESCE(MAX(execution_time), '1970-01-01') as last_execution,
			COALESCE(SUM(entries_deleted), 0) as total_deleted,
			COALESCE(SUM(bytes_freed), 0) as total_freed
		FROM retention_policy_executions
		WHERE status = 'completed'
	`).Scan(&stats.LastExecutionTime, &stats.TotalEntriesDeleted, &stats.TotalBytesFreed)
	if err != nil {
		return nil, fmt.Errorf("failed to get execution stats: %w", err)
	}

	// Get recent executions
	recentExecutions, err := r.GetRecent(ctx, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent executions: %w", err)
	}
	stats.RecentExecutions = recentExecutions

	return stats, nil
}

// CleanupOldExecutions removes old execution records
func (r *RetentionExecutionRepository) CleanupOldExecutions(ctx context.Context, before time.Time) error {
	query := `DELETE FROM retention_policy_executions WHERE created_at < ?`

	_, err := r.db.ExecContext(ctx, query, before)
	if err != nil {
		return fmt.Errorf("failed to cleanup old executions: %w", err)
	}

	return nil
}

// Helper method for querying executions
func (r *RetentionExecutionRepository) queryExecutions(ctx context.Context, query string, args ...interface{}) ([]models.RetentionPolicyExecution, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query retention executions: %w", err)
	}
	defer rows.Close()

	var executions []models.RetentionPolicyExecution
	for rows.Next() {
		execution := models.RetentionPolicyExecution{}
		var durationNanos int64

		err := rows.Scan(
			&execution.ID,
			&execution.PolicyID,
			&execution.ExecutionTime,
			&execution.Status,
			&execution.EntriesProcessed,
			&execution.EntriesDeleted,
			&execution.BytesFreed,
			&durationNanos,
			&execution.ErrorMessage,
			&execution.Details,
			&execution.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan retention execution: %w", err)
		}

		execution.Duration = time.Duration(durationNanos)
		executions = append(executions, execution)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating retention executions: %w", err)
	}

	return executions, nil
}
