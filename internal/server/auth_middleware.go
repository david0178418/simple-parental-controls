package server

import (
	"context"
	"net/http"
	"strings"

	"parental-control/internal/logging"
)

// Context key types to avoid collisions
type authContextKey string

const (
	authUserKey    authContextKey = "user"
	authSessionKey authContextKey = "session"
)

// AuthService interface to avoid circular import
type AuthService interface {
	ValidateSession(sessionID string) (AuthUser, error)
	GetSession(sessionID string) (AuthSession, error)
}

// AuthUser interface to represent authenticated user
type AuthUser interface {
	GetID() int
	GetUsername() string
	GetEmail() string
	HasAdminRole() bool
}

// AuthSession interface to represent user session
type AuthSession interface {
	GetID() string
	GetUserID() int
	IsValid() bool
}

// AuthMiddleware provides authentication middleware for API endpoints
type AuthMiddleware struct {
	authService AuthService
	publicPaths []string
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(authService AuthService) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
		publicPaths: []string{
			"/api/v1/ping",
			"/api/v1/info",
			"/api/v1/auth/login",
			"/api/v1/auth/setup",
			"/api/v1/auth/password/strength",
			"/health",
			"/status",
		},
	}
}

// AddPublicPath adds a path that doesn't require authentication
func (am *AuthMiddleware) AddPublicPath(path string) {
	am.publicPaths = append(am.publicPaths, path)
}

// RequireAuth returns middleware that requires authentication
func (am *AuthMiddleware) RequireAuth() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path is public
			if am.isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Extract session from request
			user, session, err := am.extractAuthFromRequest(r)
			if err != nil {
				requestID := getRequestID(r.Context())
				logging.Warn("Authentication failed",
					logging.String("request_id", requestID),
					logging.String("path", r.URL.Path),
					logging.String("error", err.Error()),
				)

				WriteErrorResponse(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			// Add user and session to context
			ctx := context.WithValue(r.Context(), authUserKey, user)
			ctx = context.WithValue(ctx, authSessionKey, session)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin returns middleware that requires admin privileges
func (am *AuthMiddleware) RequireAdmin() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// First, require authentication
			user, _, err := am.extractAuthFromRequest(r)
			if err != nil {
				requestID := getRequestID(r.Context())
				logging.Warn("Admin authentication failed",
					logging.String("request_id", requestID),
					logging.String("path", r.URL.Path),
					logging.String("error", err.Error()),
				)

				WriteErrorResponse(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			// Check admin privileges
			if !user.HasAdminRole() {
				requestID := getRequestID(r.Context())
				logging.Warn("Admin privilege required",
					logging.String("request_id", requestID),
					logging.String("path", r.URL.Path),
					logging.String("username", user.GetUsername()),
				)

				WriteErrorResponse(w, http.StatusForbidden, "Admin privileges required")
				return
			}

			// Add user to context
			ctx := context.WithValue(r.Context(), authUserKey, user)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// extractAuthFromRequest extracts authentication info from the request
func (am *AuthMiddleware) extractAuthFromRequest(r *http.Request) (AuthUser, AuthSession, error) {
	// Try to get session from cookie first
	sessionID := am.getSessionFromCookie(r)

	// If no cookie, try Authorization header
	if sessionID == "" {
		sessionID = am.getSessionFromHeader(r)
	}

	if sessionID == "" {
		return nil, nil, &AuthError{Message: "session not found"}
	}

	// Validate session
	user, err := am.authService.ValidateSession(sessionID)
	if err != nil {
		return nil, nil, err
	}

	// Get session details
	session, err := am.authService.GetSession(sessionID)
	if err != nil {
		return nil, nil, err
	}

	return user, session, nil
}

// getSessionFromCookie extracts session ID from cookie
func (am *AuthMiddleware) getSessionFromCookie(r *http.Request) string {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return ""
	}
	return cookie.Value
}

// getSessionFromHeader extracts session ID from Authorization header
func (am *AuthMiddleware) getSessionFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	// Support "Bearer <session_id>" format
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	// Support direct session ID
	return authHeader
}

// isPublicPath checks if a path is public (doesn't require authentication)
func (am *AuthMiddleware) isPublicPath(path string) bool {
	for _, publicPath := range am.publicPaths {
		if path == publicPath || strings.HasPrefix(path, publicPath) {
			return true
		}
	}
	return false
}

// GetUserFromContext extracts the authenticated user from request context
func GetUserFromContext(ctx context.Context) (AuthUser, bool) {
	user, ok := ctx.Value(authUserKey).(AuthUser)
	return user, ok
}

// GetSessionFromContext extracts the session from request context
func GetSessionFromContext(ctx context.Context) (AuthSession, bool) {
	session, ok := ctx.Value(authSessionKey).(AuthSession)
	return session, ok
}

// AuthRequiredHandler is a convenience function for handlers that need authentication
func AuthRequiredHandler(handler func(http.ResponseWriter, *http.Request, AuthUser)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r.Context())
		if !ok {
			WriteErrorResponse(w, http.StatusUnauthorized, "User not found in context")
			return
		}
		handler(w, r, user)
	}
}

// AdminRequiredHandler is a convenience function for handlers that need admin privileges
func AdminRequiredHandler(handler func(http.ResponseWriter, *http.Request, AuthUser)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r.Context())
		if !ok {
			WriteErrorResponse(w, http.StatusUnauthorized, "User not found in context")
			return
		}
		if !user.HasAdminRole() {
			WriteErrorResponse(w, http.StatusForbidden, "Admin privileges required")
			return
		}
		handler(w, r, user)
	}
}

// AuthError represents an authentication error
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}
