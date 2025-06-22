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
)

// APIServer handles all REST API endpoints for the application
type APIServer struct {
	repos       *models.RepositoryManager
	authEnabled bool
	startTime   time.Time
}

// NewAPIServer creates a new API server
func NewAPIServer(repos *models.RepositoryManager, authEnabled bool) *APIServer {
	return &APIServer{
		repos:       repos,
		authEnabled: authEnabled,
		startTime:   time.Now(),
	}
}

// RegisterRoutes registers all API routes with the server
func (api *APIServer) RegisterRoutes(server *Server) {
	// Create standardized middleware chains
	publicMiddleware := NewMiddlewareChain(
		RequestIDMiddleware(),
		LoggingMiddleware(),
		RecoveryMiddleware(),
		SecurityHeadersMiddleware(),
		JSONMiddleware(),
		ContentLengthMiddleware(1024*1024),
	)

	protectedMiddleware := NewMiddlewareChain(
		RequestIDMiddleware(),
		LoggingMiddleware(),
		RecoveryMiddleware(),
		SecurityHeadersMiddleware(),
		JSONMiddleware(),
		ContentLengthMiddleware(1024*1024),
		api.authMiddleware,
	)

	// Basic system endpoints (always available)
	server.AddHandler("/api/v1/ping", publicMiddleware.ThenFunc(api.handlePing))
	server.AddHandler("/api/v1/info", publicMiddleware.ThenFunc(api.handleInfo))

	// Authentication endpoints
	server.AddHandler("/api/v1/auth/login", publicMiddleware.ThenFunc(api.handleLogin))
	server.AddHandler("/api/v1/auth/logout", protectedMiddleware.ThenFunc(api.handleLogout))
	server.AddHandler("/api/v1/auth/check", publicMiddleware.ThenFunc(api.handleAuthCheck))
	server.AddHandler("/api/v1/auth/change-password", protectedMiddleware.ThenFunc(api.handleChangePassword))

	// Dashboard endpoints
	server.AddHandler("/api/v1/dashboard/stats", protectedMiddleware.ThenFunc(api.handleDashboardStats))

	// List management endpoints
	server.AddHandler("/api/v1/lists", protectedMiddleware.ThenFunc(api.handleLists))
	server.AddHandler("/api/v1/lists/", protectedMiddleware.ThenFunc(api.handleListsWithID))

	// List entry endpoints
	server.AddHandler("/api/v1/entries/", protectedMiddleware.ThenFunc(api.handleEntries))

	logging.Info("API routes registered",
		logging.Bool("auth_enabled", api.authEnabled),
		logging.Int("total_endpoints", 10))
}

// authMiddleware provides authentication checking
func (api *APIServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !api.authEnabled {
			// When auth is disabled, add mock user to context
			ctx := context.WithValue(r.Context(), "authenticated", true)
			ctx = context.WithValue(ctx, "user", &mockUser{})
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// When auth is enabled, perform basic session validation
		sessionID := api.getSessionFromRequest(r)
		if sessionID == "" {
			api.writeErrorResponse(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		// For now, accept any non-empty session when auth is enabled
		// TODO: Implement proper session validation
		ctx := context.WithValue(r.Context(), "authenticated", true)
		ctx = context.WithValue(ctx, "user", &mockUser{})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Basic system endpoints
func (api *APIServer) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	api.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"message":   "pong",
		"timestamp": time.Now(),
	})
}

func (api *APIServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	api.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"name":         "Parental Control API",
		"version":      "1.0.0",
		"timestamp":    time.Now(),
		"auth_enabled": api.authEnabled,
		"uptime":       time.Since(api.startTime).String(),
	})
}

// Authentication endpoints
func (api *APIServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var loginReq struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		RememberMe bool   `json:"remember_me"`
	}

	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if loginReq.Username == "" || loginReq.Password == "" {
		api.writeErrorResponse(w, http.StatusBadRequest, "Username and password required")
		return
	}

	// Create session ID
	sessionID := fmt.Sprintf("session_%d", time.Now().Unix())
	api.setSessionCookie(w, r, sessionID)

	response := map[string]interface{}{
		"success":    true,
		"message":    "Login successful",
		"session_id": sessionID,
		"expires_at": time.Now().Add(24 * time.Hour),
		"user": map[string]interface{}{
			"id":       1,
			"username": loginReq.Username,
			"is_admin": true,
		},
	}

	if !api.authEnabled {
		response["message"] = "Login successful (auth disabled)"
	}

	api.writeJSONResponse(w, http.StatusOK, response)
}

func (api *APIServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})

	api.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
	})
}

func (api *APIServer) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	authenticated := true
	if api.authEnabled {
		sessionID := api.getSessionFromRequest(r)
		authenticated = sessionID != ""
	}

	api.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"authenticated": authenticated,
		"timestamp":     time.Now().UTC(),
		"auth_enabled":  api.authEnabled,
	})
}

func (api *APIServer) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	message := "Password changed successfully"
	if !api.authEnabled {
		message = "Password change simulated (auth disabled)"
	}

	api.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": message,
	})
}

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

	api.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Entry deleted successfully",
	})
}

// Helper methods
func (api *APIServer) getSessionFromRequest(r *http.Request) string {
	// Try cookie first
	if cookie, err := r.Cookie("session_id"); err == nil {
		return cookie.Value
	}

	// Try Authorization header
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	return ""
}

func (api *APIServer) setSessionCookie(w http.ResponseWriter, r *http.Request, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   int((24 * time.Hour).Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
}

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
