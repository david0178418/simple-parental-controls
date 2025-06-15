package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"parental-control/internal/models"
)

// LogRotationPolicyRepository implements the LogRotationPolicyRepository interface
type LogRotationPolicyRepository struct {
	db *sql.DB
}

// NewLogRotationPolicyRepository creates a new log rotation policy repository
func NewLogRotationPolicyRepository(db *sql.DB) *LogRotationPolicyRepository {
	return &LogRotationPolicyRepository{db: db}
}

// Create creates a new log rotation policy
func (r *LogRotationPolicyRepository) Create(ctx context.Context, policy *models.LogRotationPolicy) error {
	query := `
		INSERT INTO log_rotation_policies (
			name, description, enabled, priority,
			size_based_rotation, time_based_rotation, archival_policy,
			target_log_files, target_log_types, emergency_config,
			execution_schedule, last_executed, next_execution
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// Convert complex fields to JSON
	sizeBasedRotationJSON, _ := policy.GetSizeBasedRotationJSON()
	timeBasedRotationJSON, _ := policy.GetTimeBasedRotationJSON()
	archivalPolicyJSON, _ := policy.GetArchivalPolicyJSON()
	targetLogFilesJSON, _ := policy.GetTargetLogFilesJSON()
	targetLogTypesJSON, _ := policy.GetTargetLogTypesJSON()
	emergencyConfigJSON, _ := policy.GetEmergencyConfigJSON()

	result, err := r.db.ExecContext(ctx, query,
		policy.Name, policy.Description, policy.Enabled, policy.Priority,
		sizeBasedRotationJSON, timeBasedRotationJSON, archivalPolicyJSON,
		targetLogFilesJSON, targetLogTypesJSON, emergencyConfigJSON,
		policy.ExecutionSchedule, policy.LastExecuted, policy.NextExecution,
	)
	if err != nil {
		return fmt.Errorf("failed to create log rotation policy: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert ID: %w", err)
	}

	policy.ID = int(id)
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	return nil
}

// GetByID retrieves a log rotation policy by ID
func (r *LogRotationPolicyRepository) GetByID(ctx context.Context, id int) (*models.LogRotationPolicy, error) {
	query := `
		SELECT id, name, description, enabled, priority,
			   size_based_rotation, time_based_rotation, archival_policy,
			   target_log_files, target_log_types, emergency_config,
			   execution_schedule, last_executed, next_execution,
			   created_at, updated_at
		FROM log_rotation_policies
		WHERE id = ?
	`

	var policy models.LogRotationPolicy
	var sizeBasedRotationJSON, timeBasedRotationJSON, archivalPolicyJSON sql.NullString
	var targetLogFilesJSON, targetLogTypesJSON, emergencyConfigJSON sql.NullString
	var lastExecuted, nextExecution sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&policy.ID, &policy.Name, &policy.Description, &policy.Enabled, &policy.Priority,
		&sizeBasedRotationJSON, &timeBasedRotationJSON, &archivalPolicyJSON,
		&targetLogFilesJSON, &targetLogTypesJSON, &emergencyConfigJSON,
		&policy.ExecutionSchedule, &lastExecuted, &nextExecution,
		&policy.CreatedAt, &policy.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("log rotation policy with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get log rotation policy: %w", err)
	}

	// Parse JSON fields
	if sizeBasedRotationJSON.Valid {
		if err := policy.SetSizeBasedRotationJSON(sizeBasedRotationJSON.String); err != nil {
			return nil, fmt.Errorf("failed to parse size-based rotation: %w", err)
		}
	}

	if timeBasedRotationJSON.Valid {
		if err := policy.SetTimeBasedRotationJSON(timeBasedRotationJSON.String); err != nil {
			return nil, fmt.Errorf("failed to parse time-based rotation: %w", err)
		}
	}

	if archivalPolicyJSON.Valid {
		if err := policy.SetArchivalPolicyJSON(archivalPolicyJSON.String); err != nil {
			return nil, fmt.Errorf("failed to parse archival policy: %w", err)
		}
	}

	if targetLogFilesJSON.Valid {
		if err := policy.SetTargetLogFilesJSON(targetLogFilesJSON.String); err != nil {
			return nil, fmt.Errorf("failed to parse target log files: %w", err)
		}
	}

	if targetLogTypesJSON.Valid {
		if err := policy.SetTargetLogTypesJSON(targetLogTypesJSON.String); err != nil {
			return nil, fmt.Errorf("failed to parse target log types: %w", err)
		}
	}

	if emergencyConfigJSON.Valid {
		if err := policy.SetEmergencyConfigJSON(emergencyConfigJSON.String); err != nil {
			return nil, fmt.Errorf("failed to parse emergency config: %w", err)
		}
	}

	if lastExecuted.Valid {
		policy.LastExecuted = lastExecuted.Time
	}

	if nextExecution.Valid {
		policy.NextExecution = nextExecution.Time
	}

	return &policy, nil
}

// GetAll retrieves all log rotation policies
func (r *LogRotationPolicyRepository) GetAll(ctx context.Context) ([]models.LogRotationPolicy, error) {
	query := `
		SELECT id, name, description, enabled, priority,
			   size_based_rotation, time_based_rotation, archival_policy,
			   target_log_files, target_log_types, emergency_config,
			   execution_schedule, last_executed, next_execution,
			   created_at, updated_at
		FROM log_rotation_policies
		ORDER BY priority DESC, created_at DESC
	`

	return r.scanPolicies(ctx, query)
}

// GetEnabled retrieves all enabled log rotation policies
func (r *LogRotationPolicyRepository) GetEnabled(ctx context.Context) ([]models.LogRotationPolicy, error) {
	query := `
		SELECT id, name, description, enabled, priority,
			   size_based_rotation, time_based_rotation, archival_policy,
			   target_log_files, target_log_types, emergency_config,
			   execution_schedule, last_executed, next_execution,
			   created_at, updated_at
		FROM log_rotation_policies
		WHERE enabled = 1
		ORDER BY priority DESC, created_at DESC
	`

	return r.scanPolicies(ctx, query)
}

// GetByPriority retrieves policies ordered by priority (highest first)
func (r *LogRotationPolicyRepository) GetByPriority(ctx context.Context) ([]models.LogRotationPolicy, error) {
	query := `
		SELECT id, name, description, enabled, priority,
			   size_based_rotation, time_based_rotation, archival_policy,
			   target_log_files, target_log_types, emergency_config,
			   execution_schedule, last_executed, next_execution,
			   created_at, updated_at
		FROM log_rotation_policies
		WHERE enabled = 1
		ORDER BY priority DESC, id ASC
	`

	return r.scanPolicies(ctx, query)
}

// Update updates a log rotation policy
func (r *LogRotationPolicyRepository) Update(ctx context.Context, policy *models.LogRotationPolicy) error {
	query := `
		UPDATE log_rotation_policies SET
			name = ?, description = ?, enabled = ?, priority = ?,
			size_based_rotation = ?, time_based_rotation = ?, archival_policy = ?,
			target_log_files = ?, target_log_types = ?, emergency_config = ?,
			execution_schedule = ?, last_executed = ?, next_execution = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	// Convert complex fields to JSON
	sizeBasedRotationJSON, _ := policy.GetSizeBasedRotationJSON()
	timeBasedRotationJSON, _ := policy.GetTimeBasedRotationJSON()
	archivalPolicyJSON, _ := policy.GetArchivalPolicyJSON()
	targetLogFilesJSON, _ := policy.GetTargetLogFilesJSON()
	targetLogTypesJSON, _ := policy.GetTargetLogTypesJSON()
	emergencyConfigJSON, _ := policy.GetEmergencyConfigJSON()

	result, err := r.db.ExecContext(ctx, query,
		policy.Name, policy.Description, policy.Enabled, policy.Priority,
		sizeBasedRotationJSON, timeBasedRotationJSON, archivalPolicyJSON,
		targetLogFilesJSON, targetLogTypesJSON, emergencyConfigJSON,
		policy.ExecutionSchedule, policy.LastExecuted, policy.NextExecution,
		policy.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update log rotation policy: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("log rotation policy with ID %d not found", policy.ID)
	}

	policy.UpdatedAt = time.Now()
	return nil
}

// Delete deletes a log rotation policy
func (r *LogRotationPolicyRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM log_rotation_policies WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete log rotation policy: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("log rotation policy with ID %d not found", id)
	}

	return nil
}

// Count returns the total number of log rotation policies
func (r *LogRotationPolicyRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM log_rotation_policies`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count log rotation policies: %w", err)
	}

	return count, nil
}

// Helper method to scan multiple policies
func (r *LogRotationPolicyRepository) scanPolicies(ctx context.Context, query string, args ...interface{}) ([]models.LogRotationPolicy, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query log rotation policies: %w", err)
	}
	defer rows.Close()

	var policies []models.LogRotationPolicy
	for rows.Next() {
		var policy models.LogRotationPolicy
		var sizeBasedRotationJSON, timeBasedRotationJSON, archivalPolicyJSON sql.NullString
		var targetLogFilesJSON, targetLogTypesJSON, emergencyConfigJSON sql.NullString
		var lastExecuted, nextExecution sql.NullTime

		err := rows.Scan(
			&policy.ID, &policy.Name, &policy.Description, &policy.Enabled, &policy.Priority,
			&sizeBasedRotationJSON, &timeBasedRotationJSON, &archivalPolicyJSON,
			&targetLogFilesJSON, &targetLogTypesJSON, &emergencyConfigJSON,
			&policy.ExecutionSchedule, &lastExecuted, &nextExecution,
			&policy.CreatedAt, &policy.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log rotation policy: %w", err)
		}

		// Parse JSON fields
		if sizeBasedRotationJSON.Valid {
			if err := policy.SetSizeBasedRotationJSON(sizeBasedRotationJSON.String); err != nil {
				return nil, fmt.Errorf("failed to parse size-based rotation: %w", err)
			}
		}

		if timeBasedRotationJSON.Valid {
			if err := policy.SetTimeBasedRotationJSON(timeBasedRotationJSON.String); err != nil {
				return nil, fmt.Errorf("failed to parse time-based rotation: %w", err)
			}
		}

		if archivalPolicyJSON.Valid {
			if err := policy.SetArchivalPolicyJSON(archivalPolicyJSON.String); err != nil {
				return nil, fmt.Errorf("failed to parse archival policy: %w", err)
			}
		}

		if targetLogFilesJSON.Valid {
			if err := policy.SetTargetLogFilesJSON(targetLogFilesJSON.String); err != nil {
				return nil, fmt.Errorf("failed to parse target log files: %w", err)
			}
		}

		if targetLogTypesJSON.Valid {
			if err := policy.SetTargetLogTypesJSON(targetLogTypesJSON.String); err != nil {
				return nil, fmt.Errorf("failed to parse target log types: %w", err)
			}
		}

		if emergencyConfigJSON.Valid {
			if err := policy.SetEmergencyConfigJSON(emergencyConfigJSON.String); err != nil {
				return nil, fmt.Errorf("failed to parse emergency config: %w", err)
			}
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
		return nil, fmt.Errorf("error iterating over log rotation policies: %w", err)
	}

	return policies, nil
}

// LogRotationExecutionRepository implements the LogRotationExecutionRepository interface
type LogRotationExecutionRepository struct {
	db *sql.DB
}

// NewLogRotationExecutionRepository creates a new log rotation execution repository
func NewLogRotationExecutionRepository(db *sql.DB) *LogRotationExecutionRepository {
	return &LogRotationExecutionRepository{db: db}
}

// Create creates a new log rotation execution record
func (r *LogRotationExecutionRepository) Create(ctx context.Context, execution *models.LogRotationExecution) error {
	query := `
		INSERT INTO log_rotation_executions (
			policy_id, execution_time, status, trigger_reason,
			files_rotated, files_archived, files_deleted,
			bytes_compressed, bytes_freed, compression_ratio,
			duration_ms, error_message, details
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	durationMs := execution.Duration.Nanoseconds() / 1000000 // Convert to milliseconds

	result, err := r.db.ExecContext(ctx, query,
		execution.PolicyID, execution.ExecutionTime, string(execution.Status), string(execution.TriggerReason),
		execution.FilesRotated, execution.FilesArchived, execution.FilesDeleted,
		execution.BytesCompressed, execution.BytesFreed, execution.CompressionRatio,
		durationMs, execution.ErrorMessage, execution.Details,
	)
	if err != nil {
		return fmt.Errorf("failed to create log rotation execution: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert ID: %w", err)
	}

	execution.ID = int(id)
	execution.CreatedAt = time.Now()

	return nil
}

// GetByID retrieves a log rotation execution by ID
func (r *LogRotationExecutionRepository) GetByID(ctx context.Context, id int) (*models.LogRotationExecution, error) {
	query := `
		SELECT id, policy_id, execution_time, status, trigger_reason,
			   files_rotated, files_archived, files_deleted,
			   bytes_compressed, bytes_freed, compression_ratio,
			   duration_ms, error_message, details, created_at
		FROM log_rotation_executions
		WHERE id = ?
	`

	var execution models.LogRotationExecution
	var durationMs int64
	var errorMessage sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&execution.ID, &execution.PolicyID, &execution.ExecutionTime,
		&execution.Status, &execution.TriggerReason,
		&execution.FilesRotated, &execution.FilesArchived, &execution.FilesDeleted,
		&execution.BytesCompressed, &execution.BytesFreed, &execution.CompressionRatio,
		&durationMs, &errorMessage, &execution.Details, &execution.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("log rotation execution with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get log rotation execution: %w", err)
	}

	execution.Duration = time.Duration(durationMs) * time.Millisecond
	if errorMessage.Valid {
		execution.ErrorMessage = errorMessage.String
	}

	return &execution, nil
}

// GetByPolicyID retrieves executions for a specific policy
func (r *LogRotationExecutionRepository) GetByPolicyID(ctx context.Context, policyID int, limit, offset int) ([]models.LogRotationExecution, error) {
	query := `
		SELECT id, policy_id, execution_time, status, trigger_reason,
			   files_rotated, files_archived, files_deleted,
			   bytes_compressed, bytes_freed, compression_ratio,
			   duration_ms, error_message, details, created_at
		FROM log_rotation_executions
		WHERE policy_id = ?
		ORDER BY execution_time DESC
		LIMIT ? OFFSET ?
	`

	return r.scanExecutions(ctx, query, policyID, limit, offset)
}

// GetRecent retrieves recent executions
func (r *LogRotationExecutionRepository) GetRecent(ctx context.Context, limit int) ([]models.LogRotationExecution, error) {
	query := `
		SELECT id, policy_id, execution_time, status, trigger_reason,
			   files_rotated, files_archived, files_deleted,
			   bytes_compressed, bytes_freed, compression_ratio,
			   duration_ms, error_message, details, created_at
		FROM log_rotation_executions
		ORDER BY execution_time DESC
		LIMIT ?
	`

	return r.scanExecutions(ctx, query, limit)
}

// GetByStatus retrieves executions by status
func (r *LogRotationExecutionRepository) GetByStatus(ctx context.Context, status models.ExecutionStatus, limit, offset int) ([]models.LogRotationExecution, error) {
	query := `
		SELECT id, policy_id, execution_time, status, trigger_reason,
			   files_rotated, files_archived, files_deleted,
			   bytes_compressed, bytes_freed, compression_ratio,
			   duration_ms, error_message, details, created_at
		FROM log_rotation_executions
		WHERE status = ?
		ORDER BY execution_time DESC
		LIMIT ? OFFSET ?
	`

	return r.scanExecutions(ctx, query, string(status), limit, offset)
}

// GetByTimeRange retrieves executions within a time range
func (r *LogRotationExecutionRepository) GetByTimeRange(ctx context.Context, start, end time.Time, limit, offset int) ([]models.LogRotationExecution, error) {
	query := `
		SELECT id, policy_id, execution_time, status, trigger_reason,
			   files_rotated, files_archived, files_deleted,
			   bytes_compressed, bytes_freed, compression_ratio,
			   duration_ms, error_message, details, created_at
		FROM log_rotation_executions
		WHERE execution_time BETWEEN ? AND ?
		ORDER BY execution_time DESC
		LIMIT ? OFFSET ?
	`

	return r.scanExecutions(ctx, query, start, end, limit, offset)
}

// GetByTrigger retrieves executions by trigger type
func (r *LogRotationExecutionRepository) GetByTrigger(ctx context.Context, trigger models.RotationTrigger, limit, offset int) ([]models.LogRotationExecution, error) {
	query := `
		SELECT id, policy_id, execution_time, status, trigger_reason,
			   files_rotated, files_archived, files_deleted,
			   bytes_compressed, bytes_freed, compression_ratio,
			   duration_ms, error_message, details, created_at
		FROM log_rotation_executions
		WHERE trigger_reason = ?
		ORDER BY execution_time DESC
		LIMIT ? OFFSET ?
	`

	return r.scanExecutions(ctx, query, string(trigger), limit, offset)
}

// Update updates a log rotation execution
func (r *LogRotationExecutionRepository) Update(ctx context.Context, execution *models.LogRotationExecution) error {
	query := `
		UPDATE log_rotation_executions SET
			status = ?, files_rotated = ?, files_archived = ?, files_deleted = ?,
			bytes_compressed = ?, bytes_freed = ?, compression_ratio = ?,
			duration_ms = ?, error_message = ?, details = ?
		WHERE id = ?
	`

	durationMs := execution.Duration.Nanoseconds() / 1000000 // Convert to milliseconds

	result, err := r.db.ExecContext(ctx, query,
		string(execution.Status), execution.FilesRotated, execution.FilesArchived, execution.FilesDeleted,
		execution.BytesCompressed, execution.BytesFreed, execution.CompressionRatio,
		durationMs, execution.ErrorMessage, execution.Details,
		execution.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update log rotation execution: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("log rotation execution with ID %d not found", execution.ID)
	}

	return nil
}

// Delete deletes a log rotation execution
func (r *LogRotationExecutionRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM log_rotation_executions WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete log rotation execution: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("log rotation execution with ID %d not found", id)
	}

	return nil
}

// GetStats retrieves rotation statistics
func (r *LogRotationExecutionRepository) GetStats(ctx context.Context) (*models.RotationStats, error) {
	stats := &models.RotationStats{
		PolicyStats: make(map[int]*models.PolicyRotationStats),
	}

	// Get basic stats
	query := `
		SELECT 
			COUNT(*) as total_executions,
			COALESCE(SUM(files_rotated), 0) as total_files_rotated,
			COALESCE(SUM(bytes_freed), 0) as total_bytes_freed,
			COALESCE(SUM(bytes_compressed), 0) as total_bytes_compressed,
			COALESCE(AVG(compression_ratio), 0) as avg_compression_ratio,
			MAX(execution_time) as last_rotation_time
		FROM log_rotation_executions
		WHERE status = 'completed'
	`

	var lastRotationTime sql.NullTime
	err := r.db.QueryRowContext(ctx, query).Scan(
		&stats.TotalFilesRotated, &stats.TotalFilesRotated, &stats.TotalBytesFreed,
		&stats.TotalBytesCompressed, &stats.AverageCompressionRatio, &lastRotationTime,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get rotation stats: %w", err)
	}

	if lastRotationTime.Valid {
		stats.LastRotationTime = lastRotationTime.Time
	}

	// Get policy count
	policyCountQuery := `SELECT COUNT(*) FROM log_rotation_policies`
	err = r.db.QueryRowContext(ctx, policyCountQuery).Scan(&stats.TotalPolicies)
	if err != nil {
		return nil, fmt.Errorf("failed to get policy count: %w", err)
	}

	enabledPolicyCountQuery := `SELECT COUNT(*) FROM log_rotation_policies WHERE enabled = 1`
	err = r.db.QueryRowContext(ctx, enabledPolicyCountQuery).Scan(&stats.ActivePolicies)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled policy count: %w", err)
	}

	// Get recent executions
	recentExecutions, err := r.GetRecent(ctx, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent executions: %w", err)
	}
	stats.RecentExecutions = recentExecutions

	// Get emergency trigger count
	emergencyQuery := `SELECT COUNT(*) FROM log_rotation_executions WHERE trigger_reason = 'emergency'`
	err = r.db.QueryRowContext(ctx, emergencyQuery).Scan(&stats.EmergencyTriggers)
	if err != nil {
		return nil, fmt.Errorf("failed to get emergency trigger count: %w", err)
	}

	return stats, nil
}

// CleanupOldExecutions removes old execution records
func (r *LogRotationExecutionRepository) CleanupOldExecutions(ctx context.Context, before time.Time) error {
	query := `DELETE FROM log_rotation_executions WHERE created_at < ?`

	_, err := r.db.ExecContext(ctx, query, before)
	if err != nil {
		return fmt.Errorf("failed to cleanup old log rotation executions: %w", err)
	}

	return nil
}

// Helper method to scan multiple executions
func (r *LogRotationExecutionRepository) scanExecutions(ctx context.Context, query string, args ...interface{}) ([]models.LogRotationExecution, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query log rotation executions: %w", err)
	}
	defer rows.Close()

	var executions []models.LogRotationExecution
	for rows.Next() {
		var execution models.LogRotationExecution
		var durationMs int64
		var errorMessage sql.NullString

		err := rows.Scan(
			&execution.ID, &execution.PolicyID, &execution.ExecutionTime,
			&execution.Status, &execution.TriggerReason,
			&execution.FilesRotated, &execution.FilesArchived, &execution.FilesDeleted,
			&execution.BytesCompressed, &execution.BytesFreed, &execution.CompressionRatio,
			&durationMs, &errorMessage, &execution.Details, &execution.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log rotation execution: %w", err)
		}

		execution.Duration = time.Duration(durationMs) * time.Millisecond
		if errorMessage.Valid {
			execution.ErrorMessage = errorMessage.String
		}

		executions = append(executions, execution)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over log rotation executions: %w", err)
	}

	return executions, nil
}
