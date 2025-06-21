package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"parental-control/internal/logging"
	"parental-control/internal/models"
	"parental-control/internal/service"
)

// ListAPIServer handles list and list entry management endpoints
type ListAPIServer struct {
	repos        *models.RepositoryManager
	listService  *service.ListManagementService
	entryService *service.EntryManagementService
}

// NewListAPIServer creates a new list API server instance
func NewListAPIServer(repos *models.RepositoryManager) *ListAPIServer {
	// Create a default logger for the services
	logger := logging.NewDefault()

	return &ListAPIServer{
		repos:        repos,
		listService:  service.NewListManagementService(repos, logger),
		entryService: service.NewEntryManagementService(repos, logger),
	}
}

// RegisterRoutes registers list management routes with the given server
func (api *ListAPIServer) RegisterRoutes(server *Server) {
	// Middleware chain for API endpoints
	apiMiddleware := NewMiddlewareChain(
		RequestIDMiddleware(),
		LoggingMiddleware(),
		RecoveryMiddleware(),
		SecurityHeadersMiddleware(),
		JSONMiddleware(),
		ContentLengthMiddleware(1024*1024), // 1MB limit
	)

	// Register handlers for specific API endpoints
	server.AddHandler("/api/v1/lists", apiMiddleware.ThenFunc(api.handleAllListEndpoints))
	server.AddHandler("/api/v1/lists/", apiMiddleware.ThenFunc(api.handleAllListEndpoints))
	server.AddHandler("/api/v1/entries/", apiMiddleware.ThenFunc(api.handleAllEntryEndpoints))
}

// handleAllListEndpoints handles requests to /api/v1/lists and all sub-paths
func (api *ListAPIServer) handleAllListEndpoints(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/lists")

	// Handle /api/v1/lists (exact match)
	if path == "" || path == "/" {
		switch r.Method {
		case http.MethodGet:
			api.handleGetLists(w, r)
		case http.MethodPost:
			api.handleCreateList(w, r)
		default:
			WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
		return
	}

	// Remove leading slash for parsing
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid URL path")
		return
	}

	// Parse list ID
	listID, err := strconv.Atoi(parts[0])
	if err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid list ID")
		return
	}

	// Handle /api/v1/lists/{id}/entries
	if len(parts) > 1 && parts[1] == "entries" {
		api.handleListEntries(w, r, listID)
		return
	}

	// Handle /api/v1/lists/{id}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			api.handleGetList(w, r, listID)
		case http.MethodPut:
			api.handleUpdateList(w, r, listID)
		case http.MethodDelete:
			api.handleDeleteList(w, r, listID)
		default:
			WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
		return
	}

	// If we get here, it's an unrecognized path
	WriteErrorResponse(w, http.StatusNotFound, "Endpoint not found")
}

// handleAllEntryEndpoints handles requests to /api/v1/entries/{id}
func (api *ListAPIServer) handleAllEntryEndpoints(w http.ResponseWriter, r *http.Request) {
	// Parse entry ID from URL
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/entries/")

	if path == "" {
		WriteErrorResponse(w, http.StatusBadRequest, "Entry ID required")
		return
	}

	entryID, err := strconv.Atoi(path)
	if err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid entry ID")
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
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleGetLists retrieves all lists
func (api *ListAPIServer) handleGetLists(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check for optional type filter
	var listType *models.ListType
	if typeParam := r.URL.Query().Get("type"); typeParam != "" {
		lt := models.ListType(typeParam)
		if lt != models.ListTypeWhitelist && lt != models.ListTypeBlacklist {
			WriteErrorResponse(w, http.StatusBadRequest, "Invalid list type. Must be 'whitelist' or 'blacklist'")
			return
		}
		listType = &lt
	}

	lists, err := api.listService.GetAllLists(ctx, listType)
	if err != nil {
		WriteErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to retrieve lists: %v", err))
		return
	}

	WriteJSONResponse(w, http.StatusOK, lists)
}

// handleCreateList creates a new list
func (api *ListAPIServer) handleCreateList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req service.CreateListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	list, err := api.listService.CreateList(ctx, req)
	if err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Failed to create list: %v", err))
		return
	}

	WriteJSONResponse(w, http.StatusCreated, list)
}

// handleGetList retrieves a specific list by ID
func (api *ListAPIServer) handleGetList(w http.ResponseWriter, r *http.Request, listID int) {
	ctx := r.Context()

	list, err := api.listService.GetList(ctx, listID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteErrorResponse(w, http.StatusNotFound, fmt.Sprintf("List with ID %d not found", listID))
		} else {
			WriteErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to retrieve list: %v", err))
		}
		return
	}

	WriteJSONResponse(w, http.StatusOK, list)
}

// handleUpdateList updates an existing list
func (api *ListAPIServer) handleUpdateList(w http.ResponseWriter, r *http.Request, listID int) {
	ctx := r.Context()

	var req service.UpdateListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	list, err := api.listService.UpdateList(ctx, listID, req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteErrorResponse(w, http.StatusNotFound, fmt.Sprintf("List with ID %d not found", listID))
		} else {
			WriteErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Failed to update list: %v", err))
		}
		return
	}

	WriteJSONResponse(w, http.StatusOK, list)
}

// handleDeleteList deletes a list
func (api *ListAPIServer) handleDeleteList(w http.ResponseWriter, r *http.Request, listID int) {
	ctx := r.Context()

	err := api.listService.DeleteList(ctx, listID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteErrorResponse(w, http.StatusNotFound, fmt.Sprintf("List with ID %d not found", listID))
		} else {
			WriteErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete list: %v", err))
		}
		return
	}

	WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("List with ID %d deleted successfully", listID),
	})
}

// handleListEntries handles requests for list entries
func (api *ListAPIServer) handleListEntries(w http.ResponseWriter, r *http.Request, listID int) {
	switch r.Method {
	case http.MethodGet:
		api.handleGetListEntries(w, r, listID)
	case http.MethodPost:
		api.handleCreateListEntry(w, r, listID)
	default:
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleGetListEntries retrieves all entries for a specific list
func (api *ListAPIServer) handleGetListEntries(w http.ResponseWriter, r *http.Request, listID int) {
	ctx := r.Context()

	// First verify the list exists
	_, err := api.listService.GetList(ctx, listID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteErrorResponse(w, http.StatusNotFound, fmt.Sprintf("List with ID %d not found", listID))
		} else {
			WriteErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to check list: %v", err))
		}
		return
	}

	// Get entries for the list
	entries, err := api.entryService.GetEntriesByListID(ctx, listID)
	if err != nil {
		WriteErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to retrieve entries: %v", err))
		return
	}

	WriteJSONResponse(w, http.StatusOK, entries)
}

// handleCreateListEntry creates a new entry for a specific list
func (api *ListAPIServer) handleCreateListEntry(w http.ResponseWriter, r *http.Request, listID int) {
	ctx := r.Context()

	var req service.CreateEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Set the list ID from the URL
	req.ListID = listID

	entry, err := api.entryService.CreateEntry(ctx, req)
	if err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Failed to create entry: %v", err))
		return
	}

	WriteJSONResponse(w, http.StatusCreated, entry)
}

// handleGetEntry retrieves a specific entry by ID
func (api *ListAPIServer) handleGetEntry(w http.ResponseWriter, r *http.Request, entryID int) {
	ctx := r.Context()

	entry, err := api.repos.ListEntry.GetByID(ctx, entryID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteErrorResponse(w, http.StatusNotFound, fmt.Sprintf("Entry with ID %d not found", entryID))
		} else {
			WriteErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to retrieve entry: %v", err))
		}
		return
	}

	WriteJSONResponse(w, http.StatusOK, entry)
}

// handleUpdateEntry updates an existing entry
func (api *ListAPIServer) handleUpdateEntry(w http.ResponseWriter, r *http.Request, entryID int) {
	ctx := r.Context()

	var req service.UpdateEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	entry, err := api.entryService.UpdateEntry(ctx, entryID, req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteErrorResponse(w, http.StatusNotFound, fmt.Sprintf("Entry with ID %d not found", entryID))
		} else {
			WriteErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Failed to update entry: %v", err))
		}
		return
	}

	WriteJSONResponse(w, http.StatusOK, entry)
}

// handleDeleteEntry deletes an entry
func (api *ListAPIServer) handleDeleteEntry(w http.ResponseWriter, r *http.Request, entryID int) {
	ctx := r.Context()

	// First check if entry exists
	_, err := api.repos.ListEntry.GetByID(ctx, entryID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			WriteErrorResponse(w, http.StatusNotFound, fmt.Sprintf("Entry with ID %d not found", entryID))
		} else {
			WriteErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to check entry: %v", err))
		}
		return
	}

	// Delete the entry
	if err := api.repos.ListEntry.Delete(ctx, entryID); err != nil {
		WriteErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete entry: %v", err))
		return
	}

	WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("Entry with ID %d deleted successfully", entryID),
	})
}
