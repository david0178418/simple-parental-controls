package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"parental-control/internal/logging"
	"parental-control/internal/server"
)

// AuthHandlers contains HTTP handlers for authentication endpoints
type AuthHandlers struct {
	securityService *SecurityService
}

// NewAuthHandlers creates new authentication handlers
func NewAuthHandlers(securityService *SecurityService) *AuthHandlers {
	return &AuthHandlers{
		securityService: securityService,
	}
}

// RegisterRoutes registers authentication routes with the server
func (ah *AuthHandlers) RegisterRoutes(srv *server.Server) {
	// Authentication middleware for protected endpoints
	authMiddleware := server.NewMiddlewareChain(
		server.RequestIDMiddleware(),
		server.LoggingMiddleware(),
		server.RecoveryMiddleware(),
		server.SecurityHeadersMiddleware(),
		server.JSONMiddleware(),
		server.ContentLengthMiddleware(1024*1024), // 1MB limit
	)

	// Public endpoints (no authentication required)
	srv.AddHandler("/api/v1/auth/login", authMiddleware.ThenFunc(ah.handleLogin))
	srv.AddHandler("/api/v1/auth/logout", authMiddleware.ThenFunc(ah.handleLogout))
	srv.AddHandler("/api/v1/auth/password/strength", authMiddleware.ThenFunc(ah.handlePasswordStrength))

	// Protected endpoints (require authentication)
	protectedMiddleware := server.NewMiddlewareChain(
		server.RequestIDMiddleware(),
		server.LoggingMiddleware(),
		server.RecoveryMiddleware(),
		server.SecurityHeadersMiddleware(),
		server.JSONMiddleware(),
		server.ContentLengthMiddleware(1024*1024),
		ah.AuthenticationMiddleware(), // Add auth middleware
	)

	srv.AddHandler("/api/v1/auth/me", protectedMiddleware.ThenFunc(ah.handleMe))
	srv.AddHandler("/api/v1/auth/password/change", protectedMiddleware.ThenFunc(ah.handlePasswordChange))
	srv.AddHandler("/api/v1/auth/sessions", protectedMiddleware.ThenFunc(ah.handleSessions))
	srv.AddHandler("/api/v1/auth/sessions/refresh", protectedMiddleware.ThenFunc(ah.handleSessionRefresh))
	srv.AddHandler("/api/v1/auth/sessions/revoke", protectedMiddleware.ThenFunc(ah.handleSessionRevoke))

	// Admin-only endpoints
	adminMiddleware := server.NewMiddlewareChain(
		server.RequestIDMiddleware(),
		server.LoggingMiddleware(),
		server.RecoveryMiddleware(),
		server.SecurityHeadersMiddleware(),
		server.JSONMiddleware(),
		server.ContentLengthMiddleware(1024*1024),
		ah.AuthenticationMiddleware(),
		ah.AdminMiddleware(), // Require admin privileges
	)

	srv.AddHandler("/api/v1/auth/users", adminMiddleware.ThenFunc(ah.handleUsers))
	srv.AddHandler("/api/v1/auth/security/stats", adminMiddleware.ThenFunc(ah.handleSecurityStats))
	srv.AddHandler("/api/v1/auth/sessions/admin", adminMiddleware.ThenFunc(ah.handleAdminSessions))
	srv.AddHandler("/api/v1/auth/sessions/analytics", adminMiddleware.ThenFunc(ah.handleSessionAnalytics))
	srv.AddHandler("/api/v1/auth/setup", authMiddleware.ThenFunc(ah.handleInitialSetup))
}

// handleLogin processes login requests
func (ah *AuthHandlers) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		server.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get client information
	ipAddress := getClientIP(r)
	userAgent := r.UserAgent()

	// Authenticate user
	response, err := ah.securityService.Authenticate(req.Username, req.Password, ipAddress, userAgent)
	if err != nil {
		logging.Error("Authentication error", logging.Err(err))
		server.WriteErrorResponse(w, http.StatusInternalServerError, "Authentication failed")
		return
	}

	// Set session cookie if login successful
	if response.Success {
		http.SetCookie(w, &http.Cookie{
			Name:     "session_id",
			Value:    response.SessionID,
			Path:     "/",
			MaxAge:   int(ah.securityService.config.SessionTimeout.Seconds()),
			HttpOnly: true,
			Secure:   r.TLS != nil, // Only secure over HTTPS
			SameSite: http.SameSiteStrictMode,
		})
	}

	server.WriteJSONResponse(w, http.StatusOK, response)
}

// handleLogout processes logout requests
func (ah *AuthHandlers) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		server.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get session ID from cookie
	cookie, err := r.Cookie("session_id")
	if err != nil {
		server.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Logged out successfully",
		})
		return
	}

	// Revoke session
	if err := ah.securityService.RevokeSession(cookie.Value); err != nil {
		logging.Warn("Failed to revoke session", logging.Err(err))
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})

	server.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Logged out successfully",
	})
}

// handleMe returns current user information
func (ah *AuthHandlers) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		server.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	user := r.Context().Value("user").(*User)

	userInfo := &UserInfo{
		ID:          user.ID,
		Username:    user.Username,
		Email:       user.Email,
		IsAdmin:     user.IsAdmin,
		LastLoginAt: user.LastLoginAt,
		CreatedAt:   user.CreatedAt,
	}

	server.WriteJSONResponse(w, http.StatusOK, userInfo)
}

// handlePasswordChange processes password change requests
func (ah *AuthHandlers) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		server.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	user := r.Context().Value("user").(*User)

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Change password
	err := ah.securityService.ChangePassword(user.Username, req.CurrentPassword, req.NewPassword)
	if err != nil {
		server.WriteJSONResponse(w, http.StatusBadRequest, ChangePasswordResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	server.WriteJSONResponse(w, http.StatusOK, ChangePasswordResponse{
		Success: true,
		Message: "Password changed successfully",
	})
}

// handlePasswordStrength validates password strength
func (ah *AuthHandlers) handlePasswordStrength(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		server.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate password strength
	hasher := NewPasswordHasher(ah.securityService.config.Password)
	err := hasher.ValidatePasswordStrength(req.Password)

	response := PasswordStrengthResponse{
		Valid:    err == nil,
		Score:    calculatePasswordScore(req.Password),
		Feedback: []string{},
	}

	if err != nil {
		response.Feedback = strings.Split(err.Error(), "; ")
		if len(response.Feedback) > 0 && strings.Contains(response.Feedback[0], "password strength validation failed:") {
			response.Feedback[0] = strings.TrimPrefix(response.Feedback[0], "password strength validation failed: ")
		}
	}

	server.WriteJSONResponse(w, http.StatusOK, response)
}

// handleInitialSetup handles initial admin setup
func (ah *AuthHandlers) handleInitialSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		server.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Create initial admin
	err := ah.securityService.CreateInitialAdmin(req.Username, req.Password, req.Email)
	if err != nil {
		server.WriteJSONResponse(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	server.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Initial admin user created successfully",
	})
}

// handleSessions returns active sessions for the current user
func (ah *AuthHandlers) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ah.handleGetUserSessions(w, r)
	case http.MethodDelete:
		ah.handleRevokeAllUserSessions(w, r)
	default:
		server.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleGetUserSessions returns all sessions for the current user
func (ah *AuthHandlers) handleGetUserSessions(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*User)
	currentSessionID := ah.getCurrentSessionID(r)

	sessionList, err := ah.securityService.GetUserSessionsInfo(user.ID, currentSessionID)
	if err != nil {
		logging.Error("Failed to get user sessions", logging.Err(err))
		server.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve sessions")
		return
	}

	server.WriteJSONResponse(w, http.StatusOK, sessionList)
}

// handleRevokeAllUserSessions revokes all sessions for the current user except current
func (ah *AuthHandlers) handleRevokeAllUserSessions(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(*User)
	currentSessionID := ah.getCurrentSessionID(r)

	// Get all user sessions
	sessions, err := ah.securityService.GetUserSessions(user.ID)
	if err != nil {
		logging.Error("Failed to get user sessions", logging.Err(err))
		server.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve sessions")
		return
	}

	// Revoke all except current session
	revokedCount := 0
	for _, session := range sessions {
		if session.ID != currentSessionID {
			if err := ah.securityService.RevokeSession(session.ID); err != nil {
				logging.Warn("Failed to revoke session",
					logging.String("session_id", session.ID),
					logging.Err(err))
			} else {
				revokedCount++
			}
		}
	}

	server.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"message":       fmt.Sprintf("Revoked %d sessions", revokedCount),
		"revoked_count": revokedCount,
	})
}

// handleSessionRefresh extends the current session's lifetime
func (ah *AuthHandlers) handleSessionRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		server.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	currentSessionID := ah.getCurrentSessionID(r)
	if currentSessionID == "" {
		server.WriteErrorResponse(w, http.StatusBadRequest, "No session found")
		return
	}

	// Extend session by the configured session timeout duration
	extendBy := ah.securityService.config.SessionTimeout
	if err := ah.securityService.RefreshSession(currentSessionID, extendBy); err != nil {
		logging.Error("Failed to refresh session", logging.Err(err))
		server.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to refresh session")
		return
	}

	// Get updated session info
	session, err := ah.securityService.GetSession(currentSessionID)
	if err != nil {
		logging.Error("Failed to get refreshed session", logging.Err(err))
		server.WriteErrorResponse(w, http.StatusInternalServerError, "Session refreshed but failed to get updated info")
		return
	}

	server.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"message":    "Session refreshed successfully",
		"expires_at": session.ExpiresAt,
	})
}

// handleSessionRevoke revokes a specific session
func (ah *AuthHandlers) handleSessionRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		server.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user := r.Context().Value("user").(*User)
	currentSessionID := ah.getCurrentSessionID(r)

	// Verify the session belongs to the current user
	session, err := ah.securityService.GetSession(req.SessionID)
	if err != nil {
		server.WriteErrorResponse(w, http.StatusNotFound, "Session not found")
		return
	}

	if session.UserID != user.ID {
		server.WriteErrorResponse(w, http.StatusForbidden, "Session does not belong to you")
		return
	}

	// Don't allow revoking current session
	if req.SessionID == currentSessionID {
		server.WriteErrorResponse(w, http.StatusBadRequest, "Cannot revoke current session")
		return
	}

	// Revoke the session
	if err := ah.securityService.RevokeSession(req.SessionID); err != nil {
		logging.Error("Failed to revoke session", logging.Err(err))
		server.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to revoke session")
		return
	}

	server.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Session revoked successfully",
	})
}

// handleAdminSessions handles admin session management (admin only)
func (ah *AuthHandlers) handleAdminSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ah.handleGetAllSessions(w, r)
	case http.MethodDelete:
		ah.handleAdminRevokeSession(w, r)
	default:
		server.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleGetAllSessions returns all active sessions (admin only)
func (ah *AuthHandlers) handleGetAllSessions(w http.ResponseWriter, r *http.Request) {
	// Get session analytics which includes session counts
	analytics := ah.securityService.GetSessionAnalytics()

	// For now, return basic analytics data
	// In a full implementation, this would return detailed session list
	server.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"total_sessions":   analytics.TotalSessions,
		"active_sessions":  analytics.ActiveSessions,
		"expired_sessions": analytics.ExpiredSessions,
		"message":          "Detailed session listing not implemented yet",
	})
}

// handleAdminRevokeSession allows admin to revoke any session
func (ah *AuthHandlers) handleAdminRevokeSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := ah.securityService.RevokeSession(req.SessionID); err != nil {
		logging.Error("Admin failed to revoke session", logging.Err(err))
		server.WriteErrorResponse(w, http.StatusInternalServerError, "Failed to revoke session")
		return
	}

	server.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Session revoked successfully",
	})
}

// handleSessionAnalytics returns detailed session analytics (admin only)
func (ah *AuthHandlers) handleSessionAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		server.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	analytics := ah.securityService.GetSessionAnalytics()
	server.WriteJSONResponse(w, http.StatusOK, analytics)
}

// handleUsers handles user management (admin only)
func (ah *AuthHandlers) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ah.handleGetUsers(w, r)
	case http.MethodPost:
		ah.handleCreateUser(w, r)
	default:
		server.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleGetUsers returns list of users (admin only)
func (ah *AuthHandlers) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	// Return placeholder for now
	server.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"users": []interface{}{},
	})
}

// handleCreateUser creates a new user (admin only)
func (ah *AuthHandlers) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req AdminUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		server.WriteErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Placeholder - would implement user creation
	server.WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "User creation not yet implemented",
	})
}

// handleSecurityStats returns security statistics (admin only)
func (ah *AuthHandlers) handleSecurityStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		server.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	stats := ah.securityService.GetSecurityStats()
	server.WriteJSONResponse(w, http.StatusOK, stats)
}

// AuthenticationMiddleware validates session and adds user to context
func (ah *AuthHandlers) AuthenticationMiddleware() server.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get session ID from cookie
			cookie, err := r.Cookie("session_id")
			if err != nil {
				server.WriteErrorResponse(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			// Validate session
			user, err := ah.securityService.ValidateSession(cookie.Value)
			if err != nil {
				server.WriteErrorResponse(w, http.StatusUnauthorized, "Invalid or expired session")
				return
			}

			// Add user to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, "user", user)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// AdminMiddleware ensures user has admin privileges
func (ah *AuthHandlers) AdminMiddleware() server.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := r.Context().Value("user").(*User)

			if !user.IsAdmin {
				server.WriteErrorResponse(w, http.StatusForbidden, "Admin privileges required")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Helper functions

// getClientIP extracts the real client IP address
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Use remote address
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	return ip
}

// calculatePasswordScore calculates a simple password strength score (0-100)
func calculatePasswordScore(password string) int {
	score := 0
	length := len(password)

	// Length score (max 40 points)
	if length >= 12 {
		score += 40
	} else if length >= 8 {
		score += 25
	} else if length >= 6 {
		score += 10
	}

	// Character variety (max 60 points)
	hasUpper := strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	hasLower := strings.ContainsAny(password, "abcdefghijklmnopqrstuvwxyz")
	hasNumbers := strings.ContainsAny(password, "0123456789")
	hasSpecial := strings.ContainsAny(password, "!@#$%^&*()_+-=[]{}|;':\",./<>?")

	if hasUpper {
		score += 15
	}
	if hasLower {
		score += 15
	}
	if hasNumbers {
		score += 15
	}
	if hasSpecial {
		score += 15
	}

	// Bonus for long passwords
	if length >= 16 {
		score += 10
	}

	if score > 100 {
		score = 100
	}

	return score
}

// Helper method to get current session ID
func (ah *AuthHandlers) getCurrentSessionID(r *http.Request) string {
	// Try cookie first
	if cookie, err := r.Cookie("session_id"); err == nil {
		return cookie.Value
	}

	// Try Authorization header
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	return ""
}
