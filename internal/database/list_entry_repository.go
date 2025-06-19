package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"parental-control/internal/models"
)

// ListEntryRepository implements the models.ListEntryRepository interface
type ListEntryRepository struct {
	db *sql.DB
}

// NewListEntryRepository creates a new list entry repository
func NewListEntryRepository(db *sql.DB) *ListEntryRepository {
	return &ListEntryRepository{db: db}
}

// Create creates a new list entry
func (r *ListEntryRepository) Create(ctx context.Context, entry *models.ListEntry) error {
	query := `
		INSERT INTO list_entries (list_id, entry_type, pattern, pattern_type, description, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	entry.CreatedAt = now
	entry.UpdatedAt = now

	result, err := r.db.ExecContext(ctx, query,
		entry.ListID,
		entry.EntryType,
		entry.Pattern,
		entry.PatternType,
		entry.Description,
		entry.Enabled,
		entry.CreatedAt,
		entry.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create list entry: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get list entry ID: %w", err)
	}

	entry.ID = int(id)
	return nil
}

// GetByID retrieves a list entry by ID
func (r *ListEntryRepository) GetByID(ctx context.Context, id int) (*models.ListEntry, error) {
	query := `
		SELECT id, list_id, entry_type, pattern, pattern_type, description, enabled, created_at, updated_at
		FROM list_entries
		WHERE id = ?
	`

	entry := &models.ListEntry{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&entry.ID,
		&entry.ListID,
		&entry.EntryType,
		&entry.Pattern,
		&entry.PatternType,
		&entry.Description,
		&entry.Enabled,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("list entry with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get list entry: %w", err)
	}

	return entry, nil
}

// GetByListID retrieves all entries for a specific list
func (r *ListEntryRepository) GetByListID(ctx context.Context, listID int) ([]models.ListEntry, error) {
	query := `
		SELECT id, list_id, entry_type, pattern, pattern_type, description, enabled, created_at, updated_at
		FROM list_entries
		WHERE list_id = ?
		ORDER BY pattern ASC
	`

	return r.queryEntries(ctx, query, listID)
}

// GetByPattern retrieves entries by pattern and type
func (r *ListEntryRepository) GetByPattern(ctx context.Context, pattern string, entryType models.EntryType) ([]models.ListEntry, error) {
	query := `
		SELECT id, list_id, entry_type, pattern, pattern_type, description, enabled, created_at, updated_at
		FROM list_entries
		WHERE pattern = ? AND entry_type = ?
		ORDER BY pattern ASC
	`

	return r.queryEntries(ctx, query, pattern, entryType)
}

// GetEnabled retrieves all enabled list entries
func (r *ListEntryRepository) GetEnabled(ctx context.Context) ([]models.ListEntry, error) {
	query := `
		SELECT id, list_id, entry_type, pattern, pattern_type, description, enabled, created_at, updated_at
		FROM list_entries
		WHERE enabled = 1
		ORDER BY pattern ASC
	`

	return r.queryEntries(ctx, query)
}

// Update updates an existing list entry
func (r *ListEntryRepository) Update(ctx context.Context, entry *models.ListEntry) error {
	query := `
		UPDATE list_entries SET
			pattern = ?, pattern_type = ?, description = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`

	entry.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx, query,
		entry.Pattern,
		entry.PatternType,
		entry.Description,
		entry.Enabled,
		entry.UpdatedAt,
		entry.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update list entry: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get update result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("list entry with ID %d not found", entry.ID)
	}

	return nil
}

// Delete deletes a list entry by ID
func (r *ListEntryRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM list_entries WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete list entry: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get delete result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("list entry with ID %d not found", id)
	}

	return nil
}

// DeleteByListID deletes all entries for a specific list
func (r *ListEntryRepository) DeleteByListID(ctx context.Context, listID int) error {
	query := `DELETE FROM list_entries WHERE list_id = ?`

	_, err := r.db.ExecContext(ctx, query, listID)
	if err != nil {
		return fmt.Errorf("failed to delete list entries for list %d: %w", listID, err)
	}

	return nil
}

// Count returns the total number of list entries
func (r *ListEntryRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM list_entries`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count list entries: %w", err)
	}

	return count, nil
}

// CountByListID returns the number of entries for a specific list
func (r *ListEntryRepository) CountByListID(ctx context.Context, listID int) (int, error) {
	query := `SELECT COUNT(*) FROM list_entries WHERE list_id = ?`

	var count int
	err := r.db.QueryRowContext(ctx, query, listID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count list entries for list %d: %w", listID, err)
	}

	return count, nil
}

// Helper method to execute queries that return multiple entries
func (r *ListEntryRepository) queryEntries(ctx context.Context, query string, args ...interface{}) ([]models.ListEntry, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query list entries: %w", err)
	}
	defer rows.Close()

	var entries []models.ListEntry
	for rows.Next() {
		var entry models.ListEntry
		err := rows.Scan(
			&entry.ID,
			&entry.ListID,
			&entry.EntryType,
			&entry.Pattern,
			&entry.PatternType,
			&entry.Description,
			&entry.Enabled,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan list entry: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over list entries: %w", err)
	}

	return entries, nil
}
