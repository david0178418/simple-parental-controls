package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"parental-control/internal/models"
)

// ListRepository implements the models.ListRepository interface
type ListRepository struct {
	db *sql.DB
}

// NewListRepository creates a new list repository
func NewListRepository(db *sql.DB) *ListRepository {
	return &ListRepository{db: db}
}

// Create creates a new list
func (r *ListRepository) Create(ctx context.Context, list *models.List) error {
	query := `
		INSERT INTO lists (name, type, description, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	list.CreatedAt = now
	list.UpdatedAt = now

	result, err := r.db.ExecContext(ctx, query,
		list.Name,
		list.Type,
		list.Description,
		list.Enabled,
		list.CreatedAt,
		list.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create list: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get list ID: %w", err)
	}

	list.ID = int(id)
	return nil
}

// GetByID retrieves a list by ID
func (r *ListRepository) GetByID(ctx context.Context, id int) (*models.List, error) {
	query := `
		SELECT id, name, type, description, enabled, created_at, updated_at
		FROM lists
		WHERE id = ?
	`

	list := &models.List{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&list.ID,
		&list.Name,
		&list.Type,
		&list.Description,
		&list.Enabled,
		&list.CreatedAt,
		&list.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("list with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get list: %w", err)
	}

	return list, nil
}

// GetByName retrieves a list by name
func (r *ListRepository) GetByName(ctx context.Context, name string) (*models.List, error) {
	query := `
		SELECT id, name, type, description, enabled, created_at, updated_at
		FROM lists
		WHERE name = ?
	`

	list := &models.List{}
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&list.ID,
		&list.Name,
		&list.Type,
		&list.Description,
		&list.Enabled,
		&list.CreatedAt,
		&list.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("list with name '%s' not found", name)
		}
		return nil, fmt.Errorf("failed to get list: %w", err)
	}

	return list, nil
}

// GetAll retrieves all lists
func (r *ListRepository) GetAll(ctx context.Context) ([]models.List, error) {
	query := `
		SELECT id, name, type, description, enabled, created_at, updated_at
		FROM lists
		ORDER BY name ASC
	`

	return r.queryLists(ctx, query)
}

// GetByType retrieves lists by type
func (r *ListRepository) GetByType(ctx context.Context, listType models.ListType) ([]models.List, error) {
	query := `
		SELECT id, name, type, description, enabled, created_at, updated_at
		FROM lists
		WHERE type = ?
		ORDER BY name ASC
	`

	return r.queryLists(ctx, query, listType)
}

// GetEnabled retrieves all enabled lists
func (r *ListRepository) GetEnabled(ctx context.Context) ([]models.List, error) {
	query := `
		SELECT id, name, type, description, enabled, created_at, updated_at
		FROM lists
		WHERE enabled = 1
		ORDER BY name ASC
	`

	return r.queryLists(ctx, query)
}

// Update updates an existing list
func (r *ListRepository) Update(ctx context.Context, list *models.List) error {
	query := `
		UPDATE lists SET
			name = ?, type = ?, description = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`

	list.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx, query,
		list.Name,
		list.Type,
		list.Description,
		list.Enabled,
		list.UpdatedAt,
		list.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update list: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get update result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("list with ID %d not found", list.ID)
	}

	return nil
}

// Delete deletes a list by ID
func (r *ListRepository) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM lists WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete list: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get delete result: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("list with ID %d not found", id)
	}

	return nil
}

// Count returns the total number of lists
func (r *ListRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM lists`

	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count lists: %w", err)
	}

	return count, nil
}

// Helper method to execute queries that return multiple lists
func (r *ListRepository) queryLists(ctx context.Context, query string, args ...interface{}) ([]models.List, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query lists: %w", err)
	}
	defer rows.Close()

	var lists []models.List
	for rows.Next() {
		var list models.List
		err := rows.Scan(
			&list.ID,
			&list.Name,
			&list.Type,
			&list.Description,
			&list.Enabled,
			&list.CreatedAt,
			&list.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan list: %w", err)
		}
		lists = append(lists, list)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over lists: %w", err)
	}

	return lists, nil
}
