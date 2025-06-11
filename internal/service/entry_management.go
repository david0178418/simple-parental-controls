package service

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"parental-control/internal/logging"
	"parental-control/internal/models"
)

// EntryManagementService provides business logic for managing list entries
type EntryManagementService struct {
	repos  *models.RepositoryManager
	logger logging.Logger
}

// NewEntryManagementService creates a new entry management service
func NewEntryManagementService(repos *models.RepositoryManager, logger logging.Logger) *EntryManagementService {
	return &EntryManagementService{
		repos:  repos,
		logger: logger,
	}
}

// CreateEntryRequest represents a request to create a new list entry
type CreateEntryRequest struct {
	ListID      int                `json:"list_id" validate:"required"`
	EntryType   models.EntryType   `json:"entry_type" validate:"required,oneof=executable url"`
	Pattern     string             `json:"pattern" validate:"required,max=1000"`
	PatternType models.PatternType `json:"pattern_type" validate:"required,oneof=exact wildcard domain"`
	Description string             `json:"description"`
	Enabled     bool               `json:"enabled"`
}

// UpdateEntryRequest represents a request to update an existing entry
type UpdateEntryRequest struct {
	Pattern     *string             `json:"pattern,omitempty" validate:"omitempty,max=1000"`
	PatternType *models.PatternType `json:"pattern_type,omitempty" validate:"omitempty,oneof=exact wildcard domain"`
	Description *string             `json:"description,omitempty"`
	Enabled     *bool               `json:"enabled,omitempty"`
}

// BulkCreateEntriesRequest represents a request to create multiple entries
type BulkCreateEntriesRequest struct {
	ListID  int                  `json:"list_id" validate:"required"`
	Entries []CreateEntryRequest `json:"entries" validate:"required,dive"`
}

// BulkCreateResult represents the result of a bulk create operation
type BulkCreateResult struct {
	SuccessCount int               `json:"success_count"`
	FailureCount int               `json:"failure_count"`
	Errors       []BulkCreateError `json:"errors,omitempty"`
	CreatedIDs   []int             `json:"created_ids"`
}

// BulkCreateError represents an error during bulk creation
type BulkCreateError struct {
	Index   int    `json:"index"`
	Pattern string `json:"pattern"`
	Error   string `json:"error"`
}

// ExportEntriesFormat represents the format for exporting entries
type ExportEntriesFormat string

const (
	ExportFormatJSON ExportEntriesFormat = "json"
	ExportFormatCSV  ExportEntriesFormat = "csv"
	ExportFormatTXT  ExportEntriesFormat = "txt"
)

// CreateEntry creates a new list entry with validation
func (s *EntryManagementService) CreateEntry(ctx context.Context, req CreateEntryRequest) (*models.ListEntry, error) {
	s.logger.Info("Creating new entry",
		logging.Int("list_id", req.ListID),
		logging.String("type", string(req.EntryType)),
		logging.String("pattern", req.Pattern))

	// Validate the request
	if err := s.validateCreateEntryRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check for duplicates
	if err := s.checkForDuplicateEntry(ctx, req.ListID, req.Pattern, req.EntryType); err != nil {
		return nil, fmt.Errorf("duplicate check failed: %w", err)
	}

	entry := &models.ListEntry{
		ListID:      req.ListID,
		EntryType:   req.EntryType,
		Pattern:     strings.TrimSpace(req.Pattern),
		PatternType: req.PatternType,
		Description: req.Description,
		Enabled:     req.Enabled,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repos.ListEntry.Create(ctx, entry); err != nil {
		s.logger.Error("Failed to create entry", logging.Err(err))
		return nil, fmt.Errorf("failed to create entry: %w", err)
	}

	s.logger.Info("Entry created successfully",
		logging.Int("id", entry.ID),
		logging.String("pattern", entry.Pattern))

	return entry, nil
}

// GetEntry retrieves an entry by ID
func (s *EntryManagementService) GetEntry(ctx context.Context, id int) (*models.ListEntry, error) {
	return s.repos.ListEntry.GetByID(ctx, id)
}

// GetEntriesByListID retrieves all entries for a specific list
func (s *EntryManagementService) GetEntriesByListID(ctx context.Context, listID int) ([]models.ListEntry, error) {
	return s.repos.ListEntry.GetByListID(ctx, listID)
}

// UpdateEntry updates an existing entry
func (s *EntryManagementService) UpdateEntry(ctx context.Context, id int, req UpdateEntryRequest) (*models.ListEntry, error) {
	s.logger.Info("Updating entry", logging.Int("id", id))

	// Get the existing entry
	entry, err := s.repos.ListEntry.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get entry: %w", err)
	}

	// Apply updates
	if req.Pattern != nil {
		pattern := strings.TrimSpace(*req.Pattern)
		if err := s.validatePattern(pattern, entry.EntryType, entry.PatternType); err != nil {
			return nil, fmt.Errorf("invalid pattern: %w", err)
		}
		// Check for duplicates if pattern changed
		if pattern != entry.Pattern {
			if err := s.checkForDuplicateEntry(ctx, entry.ListID, pattern, entry.EntryType); err != nil {
				return nil, fmt.Errorf("duplicate check failed: %w", err)
			}
		}
		entry.Pattern = pattern
	}
	if req.PatternType != nil {
		entry.PatternType = *req.PatternType
	}
	if req.Description != nil {
		entry.Description = *req.Description
	}
	if req.Enabled != nil {
		entry.Enabled = *req.Enabled
	}

	entry.UpdatedAt = time.Now()

	if err := s.repos.ListEntry.Update(ctx, entry); err != nil {
		s.logger.Error("Failed to update entry", logging.Err(err))
		return nil, fmt.Errorf("failed to update entry: %w", err)
	}

	s.logger.Info("Entry updated successfully", logging.Int("id", id))
	return entry, nil
}

// DeleteEntry deletes an entry
func (s *EntryManagementService) DeleteEntry(ctx context.Context, id int) error {
	s.logger.Info("Deleting entry", logging.Int("id", id))

	// Check if entry exists
	entry, err := s.repos.ListEntry.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get entry: %w", err)
	}

	if err := s.repos.ListEntry.Delete(ctx, id); err != nil {
		s.logger.Error("Failed to delete entry", logging.Err(err))
		return fmt.Errorf("failed to delete entry: %w", err)
	}

	s.logger.Info("Entry deleted successfully",
		logging.Int("id", id),
		logging.String("pattern", entry.Pattern))

	return nil
}

// ToggleEntryEnabled toggles the enabled state of an entry
func (s *EntryManagementService) ToggleEntryEnabled(ctx context.Context, id int) (*models.ListEntry, error) {
	entry, err := s.repos.ListEntry.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get entry: %w", err)
	}

	entry.Enabled = !entry.Enabled
	entry.UpdatedAt = time.Now()

	if err := s.repos.ListEntry.Update(ctx, entry); err != nil {
		return nil, fmt.Errorf("failed to update entry: %w", err)
	}

	s.logger.Info("Entry enabled state toggled",
		logging.Int("id", id),
		logging.Bool("enabled", entry.Enabled))

	return entry, nil
}

// BulkCreateEntries creates multiple entries in a single operation
func (s *EntryManagementService) BulkCreateEntries(ctx context.Context, req BulkCreateEntriesRequest) (*BulkCreateResult, error) {
	s.logger.Info("Bulk creating entries",
		logging.Int("list_id", req.ListID),
		logging.Int("count", len(req.Entries)))

	result := &BulkCreateResult{
		CreatedIDs: make([]int, 0),
		Errors:     make([]BulkCreateError, 0),
	}

	// Verify list exists
	if _, err := s.repos.List.GetByID(ctx, req.ListID); err != nil {
		return nil, fmt.Errorf("invalid list ID: %w", err)
	}

	for i, entryReq := range req.Entries {
		entryReq.ListID = req.ListID // Ensure consistency

		entry, err := s.CreateEntry(ctx, entryReq)
		if err != nil {
			result.FailureCount++
			result.Errors = append(result.Errors, BulkCreateError{
				Index:   i,
				Pattern: entryReq.Pattern,
				Error:   err.Error(),
			})
		} else {
			result.SuccessCount++
			result.CreatedIDs = append(result.CreatedIDs, entry.ID)
		}
	}

	s.logger.Info("Bulk create completed",
		logging.Int("success", result.SuccessCount),
		logging.Int("failures", result.FailureCount))

	return result, nil
}

// ImportEntries imports entries from various formats
func (s *EntryManagementService) ImportEntries(ctx context.Context, listID int, data []byte, format ExportEntriesFormat) (*BulkCreateResult, error) {
	s.logger.Info("Importing entries",
		logging.Int("list_id", listID),
		logging.String("format", string(format)))

	var entries []CreateEntryRequest
	var err error

	switch format {
	case ExportFormatTXT:
		entries, err = s.parseTXTImport(data, listID)
	default:
		return nil, fmt.Errorf("unsupported import format: %s", format)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse import data: %w", err)
	}

	return s.BulkCreateEntries(ctx, BulkCreateEntriesRequest{
		ListID:  listID,
		Entries: entries,
	})
}

// ExportEntries exports entries in the specified format
func (s *EntryManagementService) ExportEntries(ctx context.Context, listID int, format ExportEntriesFormat) ([]byte, error) {
	s.logger.Info("Exporting entries",
		logging.Int("list_id", listID),
		logging.String("format", string(format)))

	entries, err := s.repos.ListEntry.GetByListID(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("failed to get entries: %w", err)
	}

	switch format {
	case ExportFormatTXT:
		return s.exportAsTXT(entries)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// SearchEntries searches for entries matching a pattern
func (s *EntryManagementService) SearchEntries(ctx context.Context, listID int, searchTerm string, entryType *models.EntryType) ([]models.ListEntry, error) {
	allEntries, err := s.repos.ListEntry.GetByListID(ctx, listID)
	if err != nil {
		return nil, fmt.Errorf("failed to get entries: %w", err)
	}

	searchTerm = strings.ToLower(strings.TrimSpace(searchTerm))
	var filtered []models.ListEntry

	for _, entry := range allEntries {
		// Filter by entry type if specified
		if entryType != nil && entry.EntryType != *entryType {
			continue
		}

		// Search in pattern and description
		if searchTerm == "" ||
			strings.Contains(strings.ToLower(entry.Pattern), searchTerm) ||
			strings.Contains(strings.ToLower(entry.Description), searchTerm) {
			filtered = append(filtered, entry)
		}
	}

	return filtered, nil
}

// validateCreateEntryRequest validates a create entry request
func (s *EntryManagementService) validateCreateEntryRequest(ctx context.Context, req CreateEntryRequest) error {
	// Verify list exists
	if _, err := s.repos.List.GetByID(ctx, req.ListID); err != nil {
		return fmt.Errorf("invalid list ID: %w", err)
	}

	// Validate entry type
	if req.EntryType != models.EntryTypeExecutable && req.EntryType != models.EntryTypeURL {
		return fmt.Errorf("invalid entry type: %s", req.EntryType)
	}

	// Validate pattern type
	if req.PatternType != models.PatternTypeExact &&
		req.PatternType != models.PatternTypeWildcard &&
		req.PatternType != models.PatternTypeDomain {
		return fmt.Errorf("invalid pattern type: %s", req.PatternType)
	}

	// Validate pattern
	pattern := strings.TrimSpace(req.Pattern)
	if pattern == "" {
		return fmt.Errorf("pattern is required")
	}

	return s.validatePattern(pattern, req.EntryType, req.PatternType)
}

// validatePattern validates a pattern based on its type and entry type
func (s *EntryManagementService) validatePattern(pattern string, entryType models.EntryType, patternType models.PatternType) error {
	switch entryType {
	case models.EntryTypeExecutable:
		return s.validateExecutablePattern(pattern, patternType)
	case models.EntryTypeURL:
		return s.validateURLPattern(pattern, patternType)
	default:
		return fmt.Errorf("unsupported entry type: %s", entryType)
	}
}

// validateExecutablePattern validates executable patterns
func (s *EntryManagementService) validateExecutablePattern(pattern string, patternType models.PatternType) error {
	switch patternType {
	case models.PatternTypeExact:
		// Must be a valid file path or executable name
		if strings.Contains(pattern, "/") {
			// Absolute or relative path
			if !filepath.IsAbs(pattern) && !strings.HasPrefix(pattern, "./") {
				pattern = "./" + pattern
			}
		}
		// Executable name validation - allow alphanumeric, dots, dashes, underscores
		if matched, _ := regexp.MatchString(`^[a-zA-Z0-9._-]+$`, filepath.Base(pattern)); !matched {
			return fmt.Errorf("invalid executable name pattern")
		}
	case models.PatternTypeWildcard:
		// Basic wildcard validation - allow *, ?, alphanumeric, and path separators
		if matched, _ := regexp.MatchString(`^[a-zA-Z0-9.*?/_-]+$`, pattern); !matched {
			return fmt.Errorf("invalid wildcard pattern")
		}
	case models.PatternTypeDomain:
		return fmt.Errorf("domain pattern type not supported for executables")
	}
	return nil
}

// validateURLPattern validates URL patterns
func (s *EntryManagementService) validateURLPattern(pattern string, patternType models.PatternType) error {
	switch patternType {
	case models.PatternTypeExact:
		// Must be a valid URL
		if _, err := url.Parse(pattern); err != nil {
			return fmt.Errorf("invalid URL pattern: %w", err)
		}
	case models.PatternTypeWildcard:
		// Allow wildcards in URLs
		// Remove wildcards for basic URL structure validation
		testPattern := strings.ReplaceAll(strings.ReplaceAll(pattern, "*", "example"), "?", "a")
		if _, err := url.Parse(testPattern); err != nil {
			return fmt.Errorf("invalid URL wildcard pattern: %w", err)
		}
	case models.PatternTypeDomain:
		// Must be a valid domain name
		if matched, _ := regexp.MatchString(`^[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, pattern); !matched {
			return fmt.Errorf("invalid domain pattern")
		}
	}
	return nil
}

// checkForDuplicateEntry checks if an entry with the same pattern already exists
func (s *EntryManagementService) checkForDuplicateEntry(ctx context.Context, listID int, pattern string, entryType models.EntryType) error {
	existing, err := s.repos.ListEntry.GetByPattern(ctx, pattern, entryType)
	if err != nil {
		// If no existing entry found, that's good
		return nil
	}

	// Check if any existing entry belongs to the same list
	for _, entry := range existing {
		if entry.ListID == listID {
			return fmt.Errorf("entry with pattern '%s' already exists in this list", pattern)
		}
	}

	return nil
}

// parseTXTImport parses simple text import - one pattern per line
func (s *EntryManagementService) parseTXTImport(data []byte, listID int) ([]CreateEntryRequest, error) {
	lines := strings.Split(string(data), "\n")
	entries := make([]CreateEntryRequest, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and comments
		}

		// Try to determine if it's a URL or executable
		entryType := models.EntryTypeExecutable
		patternType := models.PatternTypeExact

		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") || strings.Contains(line, ".") {
			entryType = models.EntryTypeURL
			if strings.Contains(line, "*") || strings.Contains(line, "?") {
				patternType = models.PatternTypeWildcard
			}
		}

		entries = append(entries, CreateEntryRequest{
			ListID:      listID,
			EntryType:   entryType,
			Pattern:     line,
			PatternType: patternType,
			Enabled:     true,
		})
	}

	return entries, nil
}

// exportAsTXT exports entries as simple text format
func (s *EntryManagementService) exportAsTXT(entries []models.ListEntry) ([]byte, error) {
	var lines []string
	for _, entry := range entries {
		if entry.Enabled {
			lines = append(lines, entry.Pattern)
		}
	}
	return []byte(strings.Join(lines, "\n")), nil
}
