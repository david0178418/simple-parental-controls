package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"parental-control/internal/models"
)

// AuditLogRepository implements the models.AuditLogRepository interface
type AuditLogRepository struct {
	db *sql.DB
}

// NewAuditLogRepository creates a new audit log repository
func NewAuditLogRepository(db *sql.DB) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

// Create creates a new audit log entry
func (r *AuditLogRepository) Create(ctx context.Context, log *models.AuditLog) error {
	query := `
		INSERT INTO audit_log (timestamp, event_type, target_type, target_value, action, rule_type, rule_id, details)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query,
		log.Timestamp,
		log.EventType,
		log.TargetType,
		log.TargetValue,
		log.Action,
		log.RuleType,
		log.RuleID,
		log.Details,
	)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get audit log ID: %w", err)
	}

	log.ID = int(id)
	return nil
}

// GetByID retrieves an audit log entry by ID
func (r *AuditLogRepository) GetByID(ctx context.Context, id int) (*models.AuditLog, error) {
	query := `
		SELECT id, timestamp, event_type, target_type, target_value, action, rule_type, rule_id, details, created_at
		FROM audit_log
		WHERE id = ?
	`

	log := &models.AuditLog{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&log.ID,
		&log.Timestamp,
		&log.EventType,
		&log.TargetType,
		&log.TargetValue,
		&log.Action,
		&log.RuleType,
		&log.RuleID,
		&log.Details,
		&log.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("audit log with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get audit log: %w", err)
	}

	return log, nil
}

// GetAll retrieves all audit log entries with pagination
func (r *AuditLogRepository) GetAll(ctx context.Context, limit, offset int) ([]models.AuditLog, error) {
	query := `
		SELECT id, timestamp, event_type, target_type, target_value, action, rule_type, rule_id, details, created_at
		FROM audit_log
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		err := rows.Scan(
			&log.ID,
			&log.Timestamp,
			&log.EventType,
			&log.TargetType,
			&log.TargetValue,
			&log.Action,
			&log.RuleType,
			&log.RuleID,
			&log.Details,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit logs: %w", err)
	}

	return logs, nil
}

// GetByTimeRange retrieves audit log entries within a time range
func (r *AuditLogRepository) GetByTimeRange(ctx context.Context, start, end time.Time, limit, offset int) ([]models.AuditLog, error) {
	query := `
		SELECT id, timestamp, event_type, target_type, target_value, action, rule_type, rule_id, details, created_at
		FROM audit_log
		WHERE timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, start, end, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs by time range: %w", err)
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		err := rows.Scan(
			&log.ID,
			&log.Timestamp,
			&log.EventType,
			&log.TargetType,
			&log.TargetValue,
			&log.Action,
			&log.RuleType,
			&log.RuleID,
			&log.Details,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit logs: %w", err)
	}

	return logs, nil
}

// GetByAction retrieves audit log entries by action type
func (r *AuditLogRepository) GetByAction(ctx context.Context, action models.ActionType, limit, offset int) ([]models.AuditLog, error) {
	query := `
		SELECT id, timestamp, event_type, target_type, target_value, action, rule_type, rule_id, details, created_at
		FROM audit_log
		WHERE action = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, action, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs by action: %w", err)
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		err := rows.Scan(
			&log.ID,
			&log.Timestamp,
			&log.EventType,
			&log.TargetType,
			&log.TargetValue,
			&log.Action,
			&log.RuleType,
			&log.RuleID,
			&log.Details,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit logs: %w", err)
	}

	return logs, nil
}

// GetByTargetType retrieves audit log entries by target type
func (r *AuditLogRepository) GetByTargetType(ctx context.Context, targetType models.TargetType, limit, offset int) ([]models.AuditLog, error) {
	query := `
		SELECT id, timestamp, event_type, target_type, target_value, action, rule_type, rule_id, details, created_at
		FROM audit_log
		WHERE target_type = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, targetType, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs by target type: %w", err)
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		err := rows.Scan(
			&log.ID,
			&log.Timestamp,
			&log.EventType,
			&log.TargetType,
			&log.TargetValue,
			&log.Action,
			&log.RuleType,
			&log.RuleID,
			&log.Details,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit logs: %w", err)
	}

	return logs, nil
}

// GetTodayStats returns today's allow/block statistics
func (r *AuditLogRepository) GetTodayStats(ctx context.Context) (allows int, blocks int, err error) {
	today := time.Now().Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	query := `
		SELECT 
			SUM(CASE WHEN action = 'allow' THEN 1 ELSE 0 END) as allows,
			SUM(CASE WHEN action = 'block' THEN 1 ELSE 0 END) as blocks
		FROM audit_log
		WHERE timestamp >= ? AND timestamp < ?
	`

	err = r.db.QueryRowContext(ctx, query, today, tomorrow).Scan(&allows, &blocks)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get today's stats: %w", err)
	}

	return allows, blocks, nil
}

// CleanupOldLogs removes audit logs older than the specified time
func (r *AuditLogRepository) CleanupOldLogs(ctx context.Context, before time.Time) error {
	query := `DELETE FROM audit_log WHERE timestamp < ?`

	result, err := r.db.ExecContext(ctx, query, before)
	if err != nil {
		return fmt.Errorf("failed to cleanup old logs: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get cleanup result: %w", err)
	}

	// Log the cleanup result for monitoring
	if rowsAffected > 0 {
		// This would normally be logged, but we'll keep it simple for now
		_ = rowsAffected
	}

	return nil
}

// Count returns the total number of audit log entries
func (r *AuditLogRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM audit_log`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	return count, nil
}

// CountByTimeRange returns the number of audit log entries within a time range
func (r *AuditLogRepository) CountByTimeRange(ctx context.Context, start, end time.Time) (int, error) {
	query := `SELECT COUNT(*) FROM audit_log WHERE timestamp >= ? AND timestamp <= ?`

	var count int
	err := r.db.QueryRowContext(ctx, query, start, end).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit logs by time range: %w", err)
	}

	return count, nil
}

// GetByFilters retrieves audit log entries with advanced filtering
func (r *AuditLogRepository) GetByFilters(ctx context.Context, filters AuditLogFilters) ([]models.AuditLog, error) {
	var conditions []string
	var args []interface{}

	baseQuery := `
		SELECT id, timestamp, event_type, target_type, target_value, action, rule_type, rule_id, details, created_at
		FROM audit_log
	`

	// Build WHERE conditions
	if filters.Action != nil {
		conditions = append(conditions, "action = ?")
		args = append(args, *filters.Action)
	}

	if filters.TargetType != nil {
		conditions = append(conditions, "target_type = ?")
		args = append(args, *filters.TargetType)
	}

	if filters.EventType != "" {
		conditions = append(conditions, "event_type = ?")
		args = append(args, filters.EventType)
	}

	if filters.StartTime != nil {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, *filters.StartTime)
	}

	if filters.EndTime != nil {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, *filters.EndTime)
	}

	if filters.Search != "" {
		conditions = append(conditions, "(target_value LIKE ? OR details LIKE ?)")
		searchPattern := "%" + filters.Search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	// Combine conditions
	if len(conditions) > 0 {
		baseQuery += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Add ordering and pagination
	baseQuery += " ORDER BY timestamp DESC"
	if filters.Limit > 0 {
		baseQuery += " LIMIT ?"
		args = append(args, filters.Limit)

		if filters.Offset > 0 {
			baseQuery += " OFFSET ?"
			args = append(args, filters.Offset)
		}
	}

	rows, err := r.db.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs with filters: %w", err)
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		err := rows.Scan(
			&log.ID,
			&log.Timestamp,
			&log.EventType,
			&log.TargetType,
			&log.TargetValue,
			&log.Action,
			&log.RuleType,
			&log.RuleID,
			&log.Details,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating audit logs: %w", err)
	}

	return logs, nil
}

// AuditLogFilters represents filtering options for audit log queries
type AuditLogFilters struct {
	Action     *models.ActionType
	TargetType *models.TargetType
	EventType  string
	StartTime  *time.Time
	EndTime    *time.Time
	Search     string
	Limit      int
	Offset     int
}
