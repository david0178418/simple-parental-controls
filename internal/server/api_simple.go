package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"parental-control/internal/logging"
	"parental-control/internal/models"
)

// SimpleAPIServer handles simple API endpoints.

type SimpleAPIServer struct {
	repos *models.RepositoryManager
}

// NewSimpleAPIServer creates a new SimpleAPIServer.
func NewSimpleAPIServer(repoManager *models.RepositoryManager) *SimpleAPIServer {
	return &SimpleAPIServer{
		repos: repoManager,
	}
}

// RegisterRoutes registers the simple API routes with the server.
func (s *SimpleAPIServer) RegisterRoutes(server *Server) {
	// Register basic ping and info endpoints
	server.AddHandlerFunc("/api/v1/ping", s.handlePing)
	server.AddHandlerFunc("/api/v1/info", s.handleInfo)

	// Register authentication endpoints - these return mock responses when auth is disabled
	server.AddHandlerFunc("/api/v1/auth/login", s.handleLogin)
	server.AddHandlerFunc("/api/v1/auth/logout", s.handleLogout)
	server.AddHandlerFunc("/api/v1/auth/check", s.handleAuthCheck)
	server.AddHandlerFunc("/api/v1/auth/me", s.handleMe)
	server.AddHandlerFunc("/api/v1/auth/password/change", s.handlePasswordChange)
	server.AddHandlerFunc("/api/v1/auth/password/strength", s.handlePasswordStrength)
	server.AddHandlerFunc("/api/v1/auth/setup", s.handleInitialSetup)
	server.AddHandlerFunc("/api/v1/auth/sessions", s.handleSessions)
}

// Basic system endpoints
func (s *SimpleAPIServer) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"message":   "pong",
		"timestamp": time.Now(),
	})
}

func (s *SimpleAPIServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"name":         "Parental Control API",
		"version":      "1.0.0",
		"timestamp":    time.Now(),
		"auth_enabled": false, // Always false when using SimpleAPIServer
		"uptime":       time.Since(time.Now()).String(),
	})
}

// Mock authentication endpoints (auth disabled)
func (s *SimpleAPIServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Always return success when auth is disabled
	sessionToken := "mock_session_" + fmt.Sprintf("%d", time.Now().Unix())

	// Set session cookie for compatibility
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionToken,
		Path:     "/",
		MaxAge:   int((24 * time.Hour).Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"message":    "Login successful (auth disabled)",
		"token":      sessionToken, // Frontend expects 'token' field
		"session_id": sessionToken, // Keep for compatibility
		"expires_at": time.Now().Add(24 * time.Hour),
		"user": map[string]interface{}{
			"id":       1,
			"username": "admin",
			"is_admin": true,
		},
	})
}

func (s *SimpleAPIServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
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

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Logged out successfully (auth disabled)",
	})
}

func (s *SimpleAPIServer) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// When auth is disabled, check if user has a valid token/session
	// This allows the frontend to distinguish between "logged in" and "not logged in"
	authenticated := s.hasValidToken(r)

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"authenticated": authenticated,
		"timestamp":     time.Now().UTC(),
		"auth_enabled":  false,
	})
}

func (s *SimpleAPIServer) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Return mock user data
	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"id":       1,
		"username": "admin",
		"email":    "admin@example.com",
		"is_admin": true,
	})
}

func (s *SimpleAPIServer) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Password change simulated (auth disabled)",
	})
}

func (s *SimpleAPIServer) handlePasswordStrength(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Always return valid for simplicity when auth is disabled
	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"valid":    true,
		"score":    100,
		"feedback": []string{},
	})
}

func (s *SimpleAPIServer) handleInitialSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Initial setup simulated (auth disabled)",
	})
}

func (s *SimpleAPIServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
			"sessions": []interface{}{},
		})
	case http.MethodDelete:
		s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "All sessions revoked (auth disabled)",
		})
	default:
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// Helper methods
func (s *SimpleAPIServer) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		logging.Error("Failed to encode JSON response", logging.Err(err))
	}
}

func (s *SimpleAPIServer) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	s.writeJSONResponse(w, statusCode, map[string]interface{}{
		"error":   true,
		"message": message,
		"status":  statusCode,
	})
}

// hasValidToken checks if the request has a valid token/session (cookies or Authorization header)
func (s *SimpleAPIServer) hasValidToken(r *http.Request) bool {
	// Check session cookie
	if cookie, err := r.Cookie("session_id"); err == nil && cookie.Value != "" {
		return strings.HasPrefix(cookie.Value, "mock_session_") || strings.HasPrefix(cookie.Value, "session_")
	}

	// Check Authorization header
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		return strings.HasPrefix(token, "mock_session_") || strings.HasPrefix(token, "session_")
	}

	return false
}
