package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"parental-control/internal/logging"
	"parental-control/internal/models"
	"parental-control/internal/service"
)

// Context key types to avoid collisions
type contextKey string

const (
	authenticatedKey contextKey = "authenticated"
	userKey          contextKey = "user"
)

// APIServer handles all REST API endpoints for the application
type APIServer struct {
	repos              *models.RepositoryManager
	enforcementService *service.EnforcementService
	authEnabled        bool
	startTime          time.Time
}

// NewAPIServer creates a new API server
func NewAPIServer(repoManager models.RepositoryManager, authEnabled bool) *APIServer {
	return &APIServer{
		repos:       &repoManager,
		authEnabled: authEnabled,
		startTime:   time.Now(),
	}
}

// SetEnforcementService sets the enforcement service for the API server
func (api *APIServer) SetEnforcementService(enforcementService *service.EnforcementService) {
	api.enforcementService = enforcementService
}

// RegisterRoutes registers all API routes with the server
func (api *APIServer) RegisterRoutes(server *Server) {
	// Initialize API servers
	var authMiddleware *AuthMiddleware
	if api.authEnabled {
		authAPIServer := NewAuthAPIServer(api.repos, authMiddleware)
		authAPIServer.RegisterRoutes(server)
	} else {
		// Register a simplified API server if auth is disabled
		simpleAPIServer := NewSimpleAPIServer(api.repos)
		simpleAPIServer.RegisterRoutes(server)
	}

	// Dashboard API is always protected
	dashboardAPIServer := NewDashboardAPIServer(api.repos)
	dashboardAPIServer.RegisterRoutes(server)

	// TLS API is always public
	tlsAPIServer := NewTLSAPIServer(server)
	tlsAPIServer.RegisterRoutes(server)

	// Enforcement API if available
	if api.enforcementService != nil {
		enforcementAPIServer := NewEnforcementAPIServer(api.enforcementService)
		enforcementAPIServer.RegisterRoutes(server)
	}

	// Register dashboard stats and list management endpoints
	server.AddHandlerFunc("/api/v1/dashboard/stats", api.handleDashboardStats)
	server.AddHandlerFunc("/api/v1/lists", api.handleLists)

	// Pattern for list IDs and entries - this needs more sophisticated routing but will work for now
	server.AddHandler("/api/v1/lists/", http.HandlerFunc(api.handleListsWithID))
	server.AddHandler("/api/v1/entries/", http.HandlerFunc(api.handleEntries))
}

// Dashboard and business logic endpoints

// Dashboard endpoints
func (api *APIServer) handleDashboardStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx := r.Context()

	// Get actual stats from repositories
	var stats models.DashboardStats

	if api.repos != nil {
		// Get actual list count
		lists, err := api.repos.List.GetAll(ctx)
		if err != nil {
			logging.Error("Failed to get lists for dashboard stats", logging.Err(err))
		} else {
			stats.TotalLists = len(lists)

			// Count entries across all lists
			for _, list := range lists {
				if list.Enabled {
					stats.ActiveRules++
				}
				entries, err := api.repos.ListEntry.GetByListID(ctx, list.ID)
				if err == nil {
					stats.TotalEntries += len(entries)
				}
			}
		}

		// Get audit stats for today if available
		if allows, blocks, err := api.repos.AuditLog.GetTodayStats(ctx); err == nil {
			stats.TodayAllows = allows
			stats.TodayBlocks = blocks
		} else {
			// Fallback to placeholder values
			stats.TodayBlocks = 15
			stats.TodayAllows = 128
		}

		stats.QuotasNearLimit = 2 // Placeholder - TODO: implement quota near limit logic
	} else {
		// Fallback to mock data
		stats = models.DashboardStats{
			TotalLists:      5,
			TotalEntries:    42,
			ActiveRules:     12,
			TodayBlocks:     15,
			TodayAllows:     128,
			QuotasNearLimit: 2,
		}
	}

	api.writeJSONResponse(w, http.StatusOK, stats)
}

// List management endpoints
func (api *APIServer) handleLists(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		api.handleGetLists(w, r)
	case http.MethodPost:
		api.handleCreateList(w, r)
	default:
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (api *APIServer) handleListsWithID(w http.ResponseWriter, r *http.Request) {
	// Parse list ID from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/lists/")
	if path == "" {
		api.writeErrorResponse(w, http.StatusBadRequest, "List ID required")
		return
	}

	parts := strings.Split(path, "/")
	listIDStr := parts[0]

	listID, err := strconv.Atoi(listIDStr)
	if err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "Invalid list ID")
		return
	}

	// Handle /api/v1/lists/{id}/entries
	if len(parts) > 1 && parts[1] == "entries" {
		api.handleListEntries(w, r, listID)
		return
	}

	// Handle /api/v1/lists/{id}
	switch r.Method {
	case http.MethodGet:
		api.handleGetList(w, r, listID)
	case http.MethodPut:
		api.handleUpdateList(w, r, listID)
	case http.MethodDelete:
		api.handleDeleteList(w, r, listID)
	default:
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (api *APIServer) handleGetLists(w http.ResponseWriter, r *http.Request) {
	if api.repos == nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, "Repository not available")
		return
	}

	ctx := r.Context()

	// Check for optional type filter
	var lists []models.List
	var err error

	if typeParam := r.URL.Query().Get("type"); typeParam != "" {
		listType := models.ListType(typeParam)
		if listType != models.ListTypeWhitelist && listType != models.ListTypeBlacklist {
			api.writeErrorResponse(w, http.StatusBadRequest, "Invalid list type. Must be 'whitelist' or 'blacklist'")
			return
		}
		lists, err = api.repos.List.GetByType(ctx, listType)
	} else {
		lists, err = api.repos.List.GetAll(ctx)
	}

	if err != nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to retrieve lists: %v", err))
		return
	}

	// Return lists in expected format
	api.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"lists": lists,
	})
}

func (api *APIServer) handleGetList(w http.ResponseWriter, r *http.Request, listID int) {
	if api.repos == nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, "Repository not available")
		return
	}

	ctx := r.Context()
	list, err := api.repos.List.GetByID(ctx, listID)
	if err != nil {
		api.writeErrorResponse(w, http.StatusNotFound, "List not found")
		return
	}

	// Get entries for this list
	entries, err := api.repos.ListEntry.GetByListID(ctx, listID)
	if err != nil {
		logging.Error("Failed to get entries for list", logging.Err(err), logging.Int("list_id", listID))
	} else {
		list.Entries = entries
	}

	api.writeJSONResponse(w, http.StatusOK, list)
}

func (api *APIServer) handleCreateList(w http.ResponseWriter, r *http.Request) {
	if api.repos == nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, "Repository not available")
		return
	}

	var req struct {
		Name        string          `json:"name"`
		Type        models.ListType `json:"type"`
		Description string          `json:"description"`
		Enabled     bool            `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		api.writeErrorResponse(w, http.StatusBadRequest, "List name is required")
		return
	}

	if req.Type != models.ListTypeWhitelist && req.Type != models.ListTypeBlacklist {
		api.writeErrorResponse(w, http.StatusBadRequest, "List type must be 'whitelist' or 'blacklist'")
		return
	}

	list := &models.List{
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Enabled:     req.Enabled,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctx := r.Context()
	if err := api.repos.List.Create(ctx, list); err != nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create list: %v", err))
		return
	}

	// Trigger rule refresh after list creation
	api.refreshRulesAsync(ctx)

	api.writeJSONResponse(w, http.StatusCreated, list)
}

func (api *APIServer) handleUpdateList(w http.ResponseWriter, r *http.Request, listID int) {
	if api.repos == nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, "Repository not available")
		return
	}

	ctx := r.Context()

	// Check if list exists
	existingList, err := api.repos.List.GetByID(ctx, listID)
	if err != nil {
		api.writeErrorResponse(w, http.StatusNotFound, "List not found")
		return
	}

	var req struct {
		Name        string          `json:"name"`
		Type        models.ListType `json:"type"`
		Description string          `json:"description"`
		Enabled     bool            `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Update fields
	existingList.Name = req.Name
	existingList.Type = req.Type
	existingList.Description = req.Description
	existingList.Enabled = req.Enabled
	existingList.UpdatedAt = time.Now()

	if err := api.repos.List.Update(ctx, existingList); err != nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update list: %v", err))
		return
	}

	// Trigger rule refresh after list update
	api.refreshRulesAsync(ctx)

	api.writeJSONResponse(w, http.StatusOK, existingList)
}

func (api *APIServer) handleDeleteList(w http.ResponseWriter, r *http.Request, listID int) {
	if api.repos == nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, "Repository not available")
		return
	}

	ctx := r.Context()
	if err := api.repos.List.Delete(ctx, listID); err != nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete list: %v", err))
		return
	}

	// Trigger rule refresh after list deletion
	api.refreshRulesAsync(ctx)

	api.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "List deleted successfully",
	})
}

func (api *APIServer) handleListEntries(w http.ResponseWriter, r *http.Request, listID int) {
	switch r.Method {
	case http.MethodGet:
		api.handleGetListEntries(w, r, listID)
	case http.MethodPost:
		api.handleCreateListEntry(w, r, listID)
	default:
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (api *APIServer) handleGetListEntries(w http.ResponseWriter, r *http.Request, listID int) {
	if api.repos == nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, "Repository not available")
		return
	}

	ctx := r.Context()
	entries, err := api.repos.ListEntry.GetByListID(ctx, listID)
	if err != nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to retrieve entries: %v", err))
		return
	}

	api.writeJSONResponse(w, http.StatusOK, entries)
}

func (api *APIServer) handleCreateListEntry(w http.ResponseWriter, r *http.Request, listID int) {
	if api.repos == nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, "Repository not available")
		return
	}

	var req struct {
		EntryType   models.EntryType   `json:"entry_type"`
		Pattern     string             `json:"pattern"`
		PatternType models.PatternType `json:"pattern_type"`
		Description string             `json:"description"`
		Enabled     bool               `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Pattern == "" {
		api.writeErrorResponse(w, http.StatusBadRequest, "Pattern is required")
		return
	}

	entry := &models.ListEntry{
		ListID:      listID,
		EntryType:   req.EntryType,
		Pattern:     req.Pattern,
		PatternType: req.PatternType,
		Description: req.Description,
		Enabled:     req.Enabled,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	ctx := r.Context()
	if err := api.repos.ListEntry.Create(ctx, entry); err != nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create entry: %v", err))
		return
	}

	// Trigger rule refresh after entry creation
	api.refreshRulesAsync(ctx)

	api.writeJSONResponse(w, http.StatusCreated, entry)
}

// Entry management endpoints
func (api *APIServer) handleEntries(w http.ResponseWriter, r *http.Request) {
	// Parse entry ID from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/entries/")
	if path == "" {
		api.writeErrorResponse(w, http.StatusBadRequest, "Entry ID required")
		return
	}

	entryID, err := strconv.Atoi(path)
	if err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "Invalid entry ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		api.handleGetEntry(w, r, entryID)
	case http.MethodPut:
		api.handleUpdateEntry(w, r, entryID)
	case http.MethodDelete:
		api.handleDeleteEntry(w, r, entryID)
	default:
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (api *APIServer) handleGetEntry(w http.ResponseWriter, r *http.Request, entryID int) {
	if api.repos == nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, "Repository not available")
		return
	}

	ctx := r.Context()
	entry, err := api.repos.ListEntry.GetByID(ctx, entryID)
	if err != nil {
		api.writeErrorResponse(w, http.StatusNotFound, "Entry not found")
		return
	}

	api.writeJSONResponse(w, http.StatusOK, entry)
}

func (api *APIServer) handleUpdateEntry(w http.ResponseWriter, r *http.Request, entryID int) {
	if api.repos == nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, "Repository not available")
		return
	}

	ctx := r.Context()

	existingEntry, err := api.repos.ListEntry.GetByID(ctx, entryID)
	if err != nil {
		api.writeErrorResponse(w, http.StatusNotFound, "Entry not found")
		return
	}

	var req struct {
		EntryType   models.EntryType   `json:"entry_type"`
		Pattern     string             `json:"pattern"`
		PatternType models.PatternType `json:"pattern_type"`
		Description string             `json:"description"`
		Enabled     bool               `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	existingEntry.EntryType = req.EntryType
	existingEntry.Pattern = req.Pattern
	existingEntry.PatternType = req.PatternType
	existingEntry.Description = req.Description
	existingEntry.Enabled = req.Enabled
	existingEntry.UpdatedAt = time.Now()

	if err := api.repos.ListEntry.Update(ctx, existingEntry); err != nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update entry: %v", err))
		return
	}

	// Trigger rule refresh after entry update
	api.refreshRulesAsync(ctx)

	api.writeJSONResponse(w, http.StatusOK, existingEntry)
}

func (api *APIServer) handleDeleteEntry(w http.ResponseWriter, r *http.Request, entryID int) {
	if api.repos == nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, "Repository not available")
		return
	}

	ctx := r.Context()
	if err := api.repos.ListEntry.Delete(ctx, entryID); err != nil {
		api.writeErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete entry: %v", err))
		return
	}

	// Trigger rule refresh after entry deletion
	api.refreshRulesAsync(ctx)

	api.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Entry deleted successfully",
	})
}

// Helper methods

func (api *APIServer) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		logging.Error("Failed to encode JSON response", logging.Err(err))
	}
}

func (api *APIServer) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	api.writeJSONResponse(w, statusCode, map[string]interface{}{
		"error":   true,
		"message": message,
		"status":  statusCode,
	})
}

// mockUser provides a simple user implementation when auth is disabled
type mockUser struct{}

func (m *mockUser) GetID() int          { return 1 }
func (m *mockUser) GetUsername() string { return "admin" }
func (m *mockUser) GetEmail() string    { return "admin@example.com" }
func (m *mockUser) HasAdminRole() bool  { return true }

// refreshRulesAsync triggers an asynchronous rule refresh
func (api *APIServer) refreshRulesAsync(ctx context.Context) {
	if api.enforcementService != nil {
		go func() {
			// Use background context to avoid cancellation when request completes
			backgroundCtx := context.Background()
			if err := api.enforcementService.RefreshRules(backgroundCtx); err != nil {
				logging.Error("Failed to refresh rules after API change", logging.Err(err))
			} else {
				logging.Debug("Rules refreshed after API change")
			}
		}()
	} else {
		logging.Warn("Cannot refresh rules - enforcement service is nil")
	}
}
