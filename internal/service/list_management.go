package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"parental-control/internal/logging"
	"parental-control/internal/models"
)

// ListManagementService provides business logic for managing lists
type ListManagementService struct {
	repos  *models.RepositoryManager
	logger logging.Logger
}

// NewListManagementService creates a new list management service
func NewListManagementService(repos *models.RepositoryManager, logger logging.Logger) *ListManagementService {
	return &ListManagementService{
		repos:  repos,
		logger: logger,
	}
}

// CreateListRequest represents a request to create a new list
type CreateListRequest struct {
	Name        string          `json:"name" validate:"required,max=255"`
	Type        models.ListType `json:"type" validate:"required,oneof=whitelist blacklist"`
	Description string          `json:"description"`
	Enabled     bool            `json:"enabled"`
}

// UpdateListRequest represents a request to update an existing list
type UpdateListRequest struct {
	Name        *string          `json:"name,omitempty" validate:"omitempty,max=255"`
	Type        *models.ListType `json:"type,omitempty" validate:"omitempty,oneof=whitelist blacklist"`
	Description *string          `json:"description,omitempty"`
	Enabled     *bool            `json:"enabled,omitempty"`
}

// ListResponse represents a list with its entries
type ListResponse struct {
	*models.List
	EntriesCount int `json:"entries_count"`
}

// CreateList creates a new list with validation
func (s *ListManagementService) CreateList(ctx context.Context, req CreateListRequest) (*models.List, error) {
	s.logger.Info("Creating new list",
		logging.String("name", req.Name),
		logging.String("type", string(req.Type)))

	// Validate the request
	if err := s.validateCreateListRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	list := &models.List{
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Enabled:     req.Enabled,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repos.List.Create(ctx, list); err != nil {
		s.logger.Error("Failed to create list", logging.Err(err))
		return nil, fmt.Errorf("failed to create list: %w", err)
	}

	s.logger.Info("List created successfully",
		logging.Int("id", list.ID),
		logging.String("name", list.Name))

	return list, nil
}

// GetList retrieves a list by ID with its entries
func (s *ListManagementService) GetList(ctx context.Context, id int) (*ListResponse, error) {
	list, err := s.repos.List.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get list: %w", err)
	}

	// Get entries for this list
	entries, err := s.repos.ListEntry.GetByListID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get list entries", logging.Err(err), logging.Int("list_id", id))
		// Don't fail the request, just log the error
		entries = []models.ListEntry{}
	}

	list.Entries = entries

	return &ListResponse{
		List:         list,
		EntriesCount: len(entries),
	}, nil
}

// GetAllLists retrieves all lists with optional filtering
func (s *ListManagementService) GetAllLists(ctx context.Context, listType *models.ListType) ([]ListResponse, error) {
	var lists []models.List
	var err error

	if listType != nil {
		lists, err = s.repos.List.GetByType(ctx, *listType)
	} else {
		lists, err = s.repos.List.GetAll(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get lists: %w", err)
	}

	responses := make([]ListResponse, len(lists))
	for i, list := range lists {
		// Get entry count for each list
		count, err := s.repos.ListEntry.CountByListID(ctx, list.ID)
		if err != nil {
			s.logger.Error("Failed to count entries for list",
				logging.Err(err),
				logging.Int("list_id", list.ID))
			count = 0
		}

		responses[i] = ListResponse{
			List:         &lists[i],
			EntriesCount: count,
		}
	}

	return responses, nil
}

// UpdateList updates an existing list
func (s *ListManagementService) UpdateList(ctx context.Context, id int, req UpdateListRequest) (*models.List, error) {
	s.logger.Info("Updating list", logging.Int("id", id))

	// Get the existing list
	list, err := s.repos.List.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get list: %w", err)
	}

	// Apply updates
	if req.Name != nil {
		if err := s.validateListName(ctx, *req.Name, &id); err != nil {
			return nil, fmt.Errorf("invalid name: %w", err)
		}
		list.Name = *req.Name
	}
	if req.Type != nil {
		list.Type = *req.Type
	}
	if req.Description != nil {
		list.Description = *req.Description
	}
	if req.Enabled != nil {
		list.Enabled = *req.Enabled
	}

	list.UpdatedAt = time.Now()

	if err := s.repos.List.Update(ctx, list); err != nil {
		s.logger.Error("Failed to update list", logging.Err(err))
		return nil, fmt.Errorf("failed to update list: %w", err)
	}

	s.logger.Info("List updated successfully", logging.Int("id", id))
	return list, nil
}

// DeleteList deletes a list and all its entries
func (s *ListManagementService) DeleteList(ctx context.Context, id int) error {
	s.logger.Info("Deleting list", logging.Int("id", id))

	// Check if list exists
	list, err := s.repos.List.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get list: %w", err)
	}

	// Delete all entries first
	if err := s.repos.ListEntry.DeleteByListID(ctx, id); err != nil {
		s.logger.Error("Failed to delete list entries", logging.Err(err))
		return fmt.Errorf("failed to delete list entries: %w", err)
	}

	// Delete associated time rules
	if err := s.repos.TimeRule.DeleteByListID(ctx, id); err != nil {
		s.logger.Error("Failed to delete time rules", logging.Err(err))
		return fmt.Errorf("failed to delete time rules: %w", err)
	}

	// Delete associated quota rules
	if err := s.repos.QuotaRule.DeleteByListID(ctx, id); err != nil {
		s.logger.Error("Failed to delete quota rules", logging.Err(err))
		return fmt.Errorf("failed to delete quota rules: %w", err)
	}

	// Finally delete the list itself
	if err := s.repos.List.Delete(ctx, id); err != nil {
		s.logger.Error("Failed to delete list", logging.Err(err))
		return fmt.Errorf("failed to delete list: %w", err)
	}

	s.logger.Info("List deleted successfully",
		logging.Int("id", id),
		logging.String("name", list.Name))

	return nil
}

// ToggleListEnabled toggles the enabled state of a list
func (s *ListManagementService) ToggleListEnabled(ctx context.Context, id int) (*models.List, error) {
	list, err := s.repos.List.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get list: %w", err)
	}

	list.Enabled = !list.Enabled
	list.UpdatedAt = time.Now()

	if err := s.repos.List.Update(ctx, list); err != nil {
		return nil, fmt.Errorf("failed to update list: %w", err)
	}

	s.logger.Info("List enabled state toggled",
		logging.Int("id", id),
		logging.Bool("enabled", list.Enabled))

	return list, nil
}

// GetEnabledLists returns all enabled lists
func (s *ListManagementService) GetEnabledLists(ctx context.Context) ([]models.List, error) {
	return s.repos.List.GetEnabled(ctx)
}

// DuplicateList creates a copy of an existing list
func (s *ListManagementService) DuplicateList(ctx context.Context, id int, newName string) (*models.List, error) {
	s.logger.Info("Duplicating list",
		logging.Int("source_id", id),
		logging.String("new_name", newName))

	// Get the source list with entries
	sourceList, err := s.repos.List.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get source list: %w", err)
	}

	entries, err := s.repos.ListEntry.GetByListID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get source list entries: %w", err)
	}

	// Validate new name
	if err := s.validateListName(ctx, newName, nil); err != nil {
		return nil, fmt.Errorf("invalid new name: %w", err)
	}

	// Create new list
	newList := &models.List{
		Name:        newName,
		Type:        sourceList.Type,
		Description: fmt.Sprintf("Copy of %s", sourceList.Name),
		Enabled:     false, // New lists start disabled
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repos.List.Create(ctx, newList); err != nil {
		return nil, fmt.Errorf("failed to create new list: %w", err)
	}

	// Copy entries
	for _, entry := range entries {
		newEntry := &models.ListEntry{
			ListID:      newList.ID,
			EntryType:   entry.EntryType,
			Pattern:     entry.Pattern,
			PatternType: entry.PatternType,
			Description: entry.Description,
			Enabled:     entry.Enabled,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := s.repos.ListEntry.Create(ctx, newEntry); err != nil {
			s.logger.Error("Failed to copy entry", logging.Err(err))
			// Continue with other entries, don't fail the entire operation
		}
	}

	s.logger.Info("List duplicated successfully",
		logging.Int("source_id", id),
		logging.Int("new_id", newList.ID))

	return newList, nil
}

// validateCreateListRequest validates a create list request
func (s *ListManagementService) validateCreateListRequest(ctx context.Context, req CreateListRequest) error {
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("name is required")
	}

	if req.Type != models.ListTypeWhitelist && req.Type != models.ListTypeBlacklist {
		return fmt.Errorf("invalid list type: %s", req.Type)
	}

	return s.validateListName(ctx, req.Name, nil)
}

// validateListName checks if a list name is unique
func (s *ListManagementService) validateListName(ctx context.Context, name string, excludeID *int) error {
	existing, err := s.repos.List.GetByName(ctx, name)
	if err != nil {
		// If error is "not found", that's actually good for validation
		return nil
	}

	// Check if we're excluding a specific ID (for updates)
	if excludeID != nil && existing.ID == *excludeID {
		return nil
	}

	return fmt.Errorf("list name '%s' already exists", name)
}
