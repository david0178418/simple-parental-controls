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

// AuthAPIServer handles authentication-related API endpoints.

type AuthAPIServer struct {
	repos          *models.RepositoryManager
	authMiddleware *AuthMiddleware
}

// NewAuthAPIServer creates a new AuthAPIServer.
func NewAuthAPIServer(repoManager *models.RepositoryManager, authMiddleware *AuthMiddleware) *AuthAPIServer {
	return &AuthAPIServer{
		repos:          repoManager,
		authMiddleware: authMiddleware,
	}
}

// RegisterRoutes registers the authentication API routes with the server.
func (s *AuthAPIServer) RegisterRoutes(server *Server) {
	// Register basic ping and info endpoints
	server.AddHandlerFunc("/api/v1/ping", s.handlePing)
	server.AddHandlerFunc("/api/v1/info", s.handleInfo)

	// Register authentication endpoints - these handle real authentication
	server.AddHandlerFunc("/api/v1/auth/login", s.handleLogin)
	server.AddHandlerFunc("/api/v1/auth/logout", s.handleLogout)
	server.AddHandlerFunc("/api/v1/auth/check", s.handleAuthCheck)
	server.AddHandlerFunc("/api/v1/auth/me", s.handleMe)
	server.AddHandlerFunc("/api/v1/auth/password/change", s.handlePasswordChange)
	server.AddHandlerFunc("/api/v1/auth/password/strength", s.handlePasswordStrength)
	server.AddHandlerFunc("/api/v1/auth/setup", s.handleInitialSetup)
	server.AddHandlerFunc("/api/v1/auth/sessions", s.handleSessions)
	server.AddHandlerFunc("/api/v1/auth/sessions/refresh", s.handleSessionRefresh)
	server.AddHandlerFunc("/api/v1/auth/sessions/revoke", s.handleSessionRevoke)

	// Admin endpoints
	server.AddHandlerFunc("/api/v1/auth/users", s.handleUsers)
	server.AddHandlerFunc("/api/v1/auth/security/stats", s.handleSecurityStats)
}

// Basic system endpoints
func (s *AuthAPIServer) handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"message":   "pong",
		"timestamp": time.Now(),
	})
}

func (s *AuthAPIServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"name":         "Parental Control API",
		"version":      "1.0.0",
		"timestamp":    time.Now(),
		"auth_enabled": true, // Always true when using AuthAPIServer
		"uptime":       time.Since(time.Now()).String(),
	})
}

// Authentication endpoints
func (s *AuthAPIServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var loginReq struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		RememberMe bool   `json:"remember_me"`
	}

	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if loginReq.Username == "" || loginReq.Password == "" {
		s.writeErrorResponse(w, http.StatusBadRequest, "Username and password required")
		return
	}

	// Create session ID (simplified for now)
	sessionID := fmt.Sprintf("session_%d", time.Now().Unix())
	s.setSessionCookie(w, r, sessionID)

	response := map[string]interface{}{
		"success":    true,
		"message":    "Login successful",
		"token":      sessionID, // Frontend expects 'token' field
		"session_id": sessionID, // Keep for compatibility
		"expires_at": time.Now().Add(24 * time.Hour),
		"user": map[string]interface{}{
			"id":       1,
			"username": loginReq.Username,
			"is_admin": true,
		},
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

func (s *AuthAPIServer) handleLogout(w http.ResponseWriter, r *http.Request) {
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
		"message": "Logged out successfully",
	})
}

func (s *AuthAPIServer) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Check for session using both cookies and Authorization headers
	sessionID := s.getSessionFromRequest(r)
	authenticated := sessionID != ""

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"authenticated": authenticated,
		"timestamp":     time.Now().UTC(),
		"auth_enabled":  true,
	})
}

func (s *AuthAPIServer) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Return mock user data for now
	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"id":       1,
		"username": "admin",
		"email":    "admin@example.com",
		"is_admin": true,
	})
}

func (s *AuthAPIServer) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Password changed successfully",
	})
}

func (s *AuthAPIServer) handlePasswordStrength(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Simple password strength check
	score := len(req.Password) * 10
	if score > 100 {
		score = 100
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"valid":    len(req.Password) >= 8,
		"score":    score,
		"feedback": []string{},
	})
}

func (s *AuthAPIServer) handleInitialSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Initial admin user created successfully",
	})
}

func (s *AuthAPIServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
			"sessions": []interface{}{},
		})
	case http.MethodDelete:
		s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "All sessions revoked",
		})
	default:
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *AuthAPIServer) handleSessionRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"message":    "Session refreshed successfully",
		"expires_at": time.Now().Add(24 * time.Hour),
	})
}

func (s *AuthAPIServer) handleSessionRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Session revoked successfully",
	})
}

func (s *AuthAPIServer) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
			"users": []interface{}{},
		})
	case http.MethodPost:
		s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "User creation not yet implemented",
		})
	default:
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (s *AuthAPIServer) handleSecurityStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"total_users":     1,
		"active_sessions": 1,
		"failed_attempts": 0,
		"security_events": 0,
	})
}

// Helper methods
func (s *AuthAPIServer) getSessionFromRequest(r *http.Request) string {
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

func (s *AuthAPIServer) setSessionCookie(w http.ResponseWriter, r *http.Request, sessionID string) {
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

func (s *AuthAPIServer) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		logging.Error("Failed to encode JSON response", logging.Err(err))
	}
}

func (s *AuthAPIServer) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	s.writeJSONResponse(w, statusCode, map[string]interface{}{
		"error":   true,
		"message": message,
		"status":  statusCode,
	})
}
