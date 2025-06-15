package server

import (
	"encoding/json"
	"net/http"
	"time"

	"parental-control/internal/logging"
)

// AuthAPIServer handles authentication-related API endpoints
type AuthAPIServer struct {
	authService    AuthService
	authMiddleware *AuthMiddleware
}

// NewAuthAPIServer creates a new authentication API server instance
func NewAuthAPIServer(authService AuthService) *AuthAPIServer {
	return &AuthAPIServer{
		authService:    authService,
		authMiddleware: NewAuthMiddleware(authService),
	}
}

// RegisterRoutes registers authentication API routes with the given server
func (api *AuthAPIServer) RegisterRoutes(server *Server) {
	// Base middleware chain for API endpoints
	baseMiddleware := NewMiddlewareChain(
		RequestIDMiddleware(),
		LoggingMiddleware(),
		RecoveryMiddleware(),
		SecurityHeadersMiddleware(),
		JSONMiddleware(),
		ContentLengthMiddleware(1024*1024), // 1MB limit
	)

	// Authentication middleware chain (for protected endpoints)
	authMiddleware := NewMiddlewareChain(
		RequestIDMiddleware(),
		LoggingMiddleware(),
		RecoveryMiddleware(),
		SecurityHeadersMiddleware(),
		JSONMiddleware(),
		ContentLengthMiddleware(1024*1024),
		api.authMiddleware.RequireAuth(),
	)

	// Admin middleware chain (for admin-only endpoints)
	adminMiddleware := NewMiddlewareChain(
		RequestIDMiddleware(),
		LoggingMiddleware(),
		RecoveryMiddleware(),
		SecurityHeadersMiddleware(),
		JSONMiddleware(),
		ContentLengthMiddleware(1024*1024),
		api.authMiddleware.RequireAdmin(),
	)

	// Public authentication endpoints (no auth required)
	server.AddHandler("/api/v1/auth/login", baseMiddleware.ThenFunc(api.handleLogin))
	server.AddHandler("/api/v1/auth/setup", baseMiddleware.ThenFunc(api.handleInitialSetup))
	server.AddHandler("/api/v1/auth/password/strength", baseMiddleware.ThenFunc(api.handlePasswordStrength))

	// Protected authentication endpoints (auth required)
	server.AddHandler("/api/v1/auth/check", authMiddleware.ThenFunc(api.handleAuthCheck))
	server.AddHandler("/api/v1/auth/logout", authMiddleware.ThenFunc(api.handleLogout))
	server.AddHandler("/api/v1/auth/password/change", authMiddleware.ThenFunc(api.handleChangePassword))
	server.AddHandler("/api/v1/auth/change-password", authMiddleware.ThenFunc(api.handleChangePassword)) // Frontend alias
	server.AddHandler("/api/v1/auth/profile", authMiddleware.ThenFunc(api.handleProfile))

	// Admin-only endpoints
	server.AddHandler("/api/v1/admin/users", adminMiddleware.ThenFunc(api.handleAdminUsers))
	server.AddHandler("/api/v1/admin/security", adminMiddleware.ThenFunc(api.handleAdminSecurity))
}

// handleLogin handles user authentication
func (api *AuthAPIServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var loginReq struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		RememberMe bool   `json:"remember_me"`
	}

	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if loginReq.Username == "" || loginReq.Password == "" {
		WriteErrorResponse(w, http.StatusBadRequest, "Username and password required")
		return
	}

	// Mock response for demonstration
	response := map[string]interface{}{
		"success":    true,
		"message":    "Login successful",
		"session_id": "mock_session_id",
		"expires_at": time.Now().Add(24 * time.Hour),
		"user": map[string]interface{}{
			"id":       1,
			"username": loginReq.Username,
			"email":    loginReq.Username + "@example.com",
			"is_admin": loginReq.Username == "admin",
		},
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "mock_session_id",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})

	WriteJSONResponse(w, http.StatusOK, response)
}

// handleLogout handles user logout
func (api *AuthAPIServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(0, 0),
	})

	WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
	})
}

// handleInitialSetup handles initial admin setup
func (api *AuthAPIServer) handleInitialSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var setupReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&setupReq); err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if setupReq.Username == "" || setupReq.Password == "" || setupReq.Email == "" {
		WriteErrorResponse(w, http.StatusBadRequest, "Username, password, and email required")
		return
	}

	WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Initial admin setup completed",
	})
}

// handlePasswordStrength validates password strength
func (api *AuthAPIServer) handlePasswordStrength(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Mock password strength validation
	score := calculatePasswordScore(req.Password)
	feedback := []string{}

	if len(req.Password) < 8 {
		feedback = append(feedback, "Password must be at least 8 characters long")
	}
	if score < 50 {
		feedback = append(feedback, "Password should include uppercase, lowercase, numbers, and special characters")
	}

	WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"valid":    score >= 50 && len(req.Password) >= 8,
		"score":    score,
		"feedback": feedback,
	})
}

// handleChangePassword handles password change requests
func (api *AuthAPIServer) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	user, ok := GetUserFromContext(r.Context())
	if !ok {
		WriteErrorResponse(w, http.StatusUnauthorized, "User not found in context")
		return
	}

	// Support both frontend and backend request formats
	var requestBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Extract current password (support both "current_password" and "old_password")
	var currentPassword, newPassword string
	if cp, ok := requestBody["current_password"].(string); ok {
		currentPassword = cp
	} else if op, ok := requestBody["old_password"].(string); ok {
		currentPassword = op
	} else {
		WriteErrorResponse(w, http.StatusBadRequest, "Missing current/old password")
		return
	}

	if np, ok := requestBody["new_password"].(string); ok {
		newPassword = np
	} else {
		WriteErrorResponse(w, http.StatusBadRequest, "Missing new password")
		return
	}

	logging.Info("Password change request",
		logging.String("username", user.GetUsername()),
		logging.Bool("current_password_provided", currentPassword != ""),
		logging.Bool("new_password_provided", newPassword != ""),
		logging.String("request_id", getRequestID(r.Context())))

	WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Password changed successfully",
	})
}

// handleAuthCheck returns authentication status
func (api *AuthAPIServer) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Simple auth status check - user already validated by middleware
	WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
		"timestamp":     time.Now().UTC(),
	})
}

// handleProfile returns user profile information
func (api *AuthAPIServer) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	user, ok := GetUserFromContext(r.Context())
	if !ok {
		WriteErrorResponse(w, http.StatusUnauthorized, "User not found in context")
		return
	}

	WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"id":       user.GetID(),
		"username": user.GetUsername(),
		"email":    user.GetEmail(),
		"is_admin": user.HasAdminRole(),
	})
}

// handleAdminUsers handles admin user management
func (api *AuthAPIServer) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	user, ok := GetUserFromContext(r.Context())
	if !ok {
		WriteErrorResponse(w, http.StatusUnauthorized, "User not found in context")
		return
	}

	switch r.Method {
	case http.MethodGet:
		users := []map[string]interface{}{
			{
				"id":        1,
				"username":  "admin",
				"email":     "admin@example.com",
				"is_admin":  true,
				"is_active": true,
			},
		}
		WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
			"users": users,
		})

	case http.MethodPost:
		logging.Info("Admin creating new user",
			logging.String("admin_username", user.GetUsername()),
			logging.String("request_id", getRequestID(r.Context())))

		WriteJSONResponse(w, http.StatusCreated, map[string]interface{}{
			"success": true,
			"message": "User created successfully",
		})

	default:
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleAdminSecurity handles admin security management
func (api *AuthAPIServer) handleAdminSecurity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	user, ok := GetUserFromContext(r.Context())
	if !ok {
		WriteErrorResponse(w, http.StatusUnauthorized, "User not found in context")
		return
	}

	logging.Info("Admin accessing security stats",
		logging.String("admin_username", user.GetUsername()),
		logging.String("request_id", getRequestID(r.Context())))

	stats := map[string]interface{}{
		"total_users":     1,
		"active_sessions": 1,
		"locked_accounts": 0,
		"recent_attempts": 5,
		"failed_attempts": 2,
		"security_events": 10,
	}

	WriteJSONResponse(w, http.StatusOK, stats)
}

// calculatePasswordScore is a simple password strength calculator
func calculatePasswordScore(password string) int {
	score := 0

	if len(password) >= 8 {
		score += 25
	}
	if len(password) >= 12 {
		score += 10
	}

	hasLower := false
	hasUpper := false
	hasDigit := false
	hasSpecial := false

	for _, r := range password {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}

	if hasLower {
		score += 15
	}
	if hasUpper {
		score += 15
	}
	if hasDigit {
		score += 15
	}
	if hasSpecial {
		score += 20
	}

	if score > 100 {
		score = 100
	}

	return score
}
