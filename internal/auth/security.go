package auth

import (
	"fmt"
	"sync"
	"time"

	"parental-control/internal/logging"
)

// SecurityService handles authentication security features
type SecurityService struct {
	config          AuthConfig
	passwordManager *PasswordManager
	sessionManager  *SessionManager

	// In-memory stores (would be replaced with database in production)
	users          map[string]*User    // username -> user
	sessions       map[string]*Session // session_id -> session (legacy, migrating to SessionManager)
	loginAttempts  []LoginAttempt
	securityEvents []SecurityEvent

	// Rate limiting
	rateLimiter map[string]*rateLimitEntry // IP -> rate limit data

	mu sync.RWMutex
}

// rateLimitEntry tracks rate limiting data for an IP address
type rateLimitEntry struct {
	attempts  int
	resetTime time.Time
}

// NewSecurityService creates a new security service
func NewSecurityService(config AuthConfig) *SecurityService {
	return &SecurityService{
		config:          config,
		passwordManager: NewPasswordManager(config.Password),
		sessionManager:  NewSessionManager(config),
		users:           make(map[string]*User),
		sessions:        make(map[string]*Session),
		loginAttempts:   make([]LoginAttempt, 0),
		securityEvents:  make([]SecurityEvent, 0),
		rateLimiter:     make(map[string]*rateLimitEntry),
	}
}

// CreateInitialAdmin creates the initial admin user if no users exist
func (ss *SecurityService) CreateInitialAdmin(username, password, email string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// Check if any users exist
	if len(ss.users) > 0 {
		return fmt.Errorf("users already exist, cannot create initial admin")
	}

	// Create admin user
	now := time.Now()
	passwordHash, err := ss.passwordManager.SetPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	admin := &User{
		ID:                1, // First user gets ID 1
		Username:          username,
		PasswordHash:      passwordHash,
		Email:             email,
		IsActive:          true,
		IsAdmin:           true,
		PasswordChangedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	ss.users[username] = admin

	// Log security event
	ss.logSecurityEvent(&SecurityEvent{
		UserID:      &admin.ID,
		EventType:   "admin_created",
		Description: "Initial admin user created",
		Severity:    SeverityHigh,
		Timestamp:   now,
	})

	logging.Info("Initial admin user created",
		logging.String("username", username),
		logging.String("email", email))

	return nil
}

// Authenticate validates user credentials and returns session info
func (ss *SecurityService) Authenticate(username, password, ipAddress, userAgent string) (*LoginResponse, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// Check rate limiting
	if !ss.checkRateLimit(ipAddress) {
		ss.logSecurityEvent(&SecurityEvent{
			EventType:   EventTypeBruteForce,
			Description: fmt.Sprintf("Rate limit exceeded for IP: %s", ipAddress),
			IPAddress:   ipAddress,
			UserAgent:   userAgent,
			Severity:    SeverityHigh,
			Timestamp:   time.Now(),
		})
		return &LoginResponse{
			Success: false,
			Message: "Too many login attempts. Please try again later.",
		}, nil
	}

	// Find user
	user, exists := ss.users[username]
	if !exists {
		ss.recordLoginAttempt(username, ipAddress, userAgent, false, "user not found")
		return &LoginResponse{
			Success: false,
			Message: "Invalid username or password",
		}, nil
	}

	// Check if account is locked
	if user.IsLocked() {
		ss.recordLoginAttempt(username, ipAddress, userAgent, false, "account locked")
		return &LoginResponse{
			Success: false,
			Message: "Account is temporarily locked. Please try again later.",
		}, nil
	}

	// Check if account is active
	if !user.IsActive {
		ss.recordLoginAttempt(username, ipAddress, userAgent, false, "account inactive")
		return &LoginResponse{
			Success: false,
			Message: "Account is inactive",
		}, nil
	}

	// Verify password
	if err := ss.passwordManager.VerifyPassword(password, user.PasswordHash); err != nil {
		ss.handleFailedLogin(user, ipAddress, userAgent)
		return &LoginResponse{
			Success: false,
			Message: "Invalid username or password",
		}, nil
	}

	// Check if password expired
	if user.PasswordExpired(ss.config.Password.PasswordExpireDays) {
		ss.recordLoginAttempt(username, ipAddress, userAgent, false, "password expired")
		return &LoginResponse{
			Success: false,
			Message: "Password has expired. Please change your password.",
		}, nil
	}

	// Successful login
	return ss.handleSuccessfulLogin(user, ipAddress, userAgent)
}

// ChangePassword changes a user's password
func (ss *SecurityService) ChangePassword(username, currentPassword, newPassword string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	user, exists := ss.users[username]
	if !exists {
		return fmt.Errorf("user not found")
	}

	// Verify current password
	if err := ss.passwordManager.VerifyPassword(currentPassword, user.PasswordHash); err != nil {
		ss.logSecurityEvent(&SecurityEvent{
			UserID:      &user.ID,
			EventType:   EventTypePasswordChange,
			Description: "Failed password change attempt - invalid current password",
			Severity:    SeverityMedium,
			Timestamp:   time.Now(),
		})
		return fmt.Errorf("current password is incorrect")
	}

	// Set new password (includes validation and history check)
	newHash, err := ss.passwordManager.SetPassword(newPassword)
	if err != nil {
		return err
	}

	// Update user
	user.PasswordHash = newHash
	user.PasswordChangedAt = time.Now()
	user.UpdatedAt = time.Now()

	// Log security event
	ss.logSecurityEvent(&SecurityEvent{
		UserID:      &user.ID,
		EventType:   EventTypePasswordChange,
		Description: "Password changed successfully",
		Severity:    SeverityMedium,
		Timestamp:   time.Now(),
	})

	logging.Info("Password changed", logging.String("username", username))

	return nil
}

// CreateSession creates a new session for the user using the enhanced session manager
func (ss *SecurityService) CreateSession(userID int, ipAddress, userAgent string, rememberMe bool) (*Session, error) {
	return ss.sessionManager.CreateSession(userID, ipAddress, userAgent, rememberMe)
}

// ValidateSession validates a session ID and returns the associated user
func (ss *SecurityService) ValidateSession(sessionID string) (*User, error) {
	// Try enhanced session manager first
	session, err := ss.sessionManager.ValidateSession(sessionID)
	if err == nil {
		// Update activity tracking
		ss.sessionManager.UpdateSessionActivity(sessionID, session.IPAddress, session.UserAgent)

		// Find user
		ss.mu.RLock()
		for _, user := range ss.users {
			if user.ID == session.UserID {
				ss.mu.RUnlock()
				return user, nil
			}
		}
		ss.mu.RUnlock()
		return nil, ErrUserNotFound
	}

	// Fallback to legacy session storage
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	session, exists := ss.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	if !session.IsValid() {
		return nil, ErrInvalidSession
	}

	// Find user
	for _, user := range ss.users {
		if user.ID == session.UserID {
			return user, nil
		}
	}

	return nil, ErrUserNotFound
}

// GetSession retrieves a session by ID
func (ss *SecurityService) GetSession(sessionID string) (*Session, error) {
	// Try enhanced session manager first
	sessions, err := ss.sessionManager.GetUserSessions(0) // Get all sessions and filter
	if err == nil {
		for _, session := range sessions {
			if session.ID == sessionID {
				return session, nil
			}
		}
	}

	// Fallback to legacy storage
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	session, exists := ss.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	return session, nil
}

// RevokeSession revokes a specific session
func (ss *SecurityService) RevokeSession(sessionID string) error {
	// Try enhanced session manager first
	err := ss.sessionManager.RevokeSession(sessionID)
	if err == nil {
		ss.logSecurityEvent(&SecurityEvent{
			EventType:   EventTypeSessionRevoked,
			Description: "Session revoked via session manager",
			Severity:    SeverityMedium,
			Timestamp:   time.Now(),
		})
		return nil
	}

	// Fallback to legacy method
	ss.mu.Lock()
	defer ss.mu.Unlock()

	session, exists := ss.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	session.IsActive = false
	session.UpdatedAt = time.Now()

	ss.logSecurityEvent(&SecurityEvent{
		UserID:      &session.UserID,
		EventType:   EventTypeSessionRevoked,
		Description: "Session revoked",
		IPAddress:   session.IPAddress,
		Severity:    SeverityMedium,
		Timestamp:   time.Now(),
	})

	return nil
}

// RevokeUserSessions revokes all sessions for a user
func (ss *SecurityService) RevokeUserSessions(userID int) error {
	return ss.sessionManager.RevokeUserSessions(userID)
}

// GetUserSessions returns all active sessions for a user
func (ss *SecurityService) GetUserSessions(userID int) ([]*Session, error) {
	return ss.sessionManager.GetUserSessions(userID)
}

// RefreshSession extends a session's lifetime
func (ss *SecurityService) RefreshSession(sessionID string, extendBy time.Duration) error {
	return ss.sessionManager.RefreshSession(sessionID, extendBy)
}

// GetSessionAnalytics returns comprehensive session analytics
func (ss *SecurityService) GetSessionAnalytics() *SessionAnalytics {
	return ss.sessionManager.GetSessionAnalytics()
}

// GetSessionMetrics returns detailed metrics for a session
func (ss *SecurityService) GetSessionMetrics(sessionID string) (*SessionMetrics, error) {
	return ss.sessionManager.GetSessionMetrics(sessionID)
}

// CleanupExpiredSessions removes expired sessions and returns count
func (ss *SecurityService) CleanupExpiredSessions() int {
	// Clean up using enhanced session manager
	cleanedFromManager := ss.sessionManager.CleanupExpiredSessions()

	// Also clean up legacy sessions
	ss.mu.Lock()
	defer ss.mu.Unlock()

	now := time.Now()
	cleanedFromLegacy := 0
	for sessionID, session := range ss.sessions {
		if session.IsExpired() {
			ss.logSecurityEvent(&SecurityEvent{
				UserID:      &session.UserID,
				EventType:   EventTypeSessionExpired,
				Description: "Legacy session expired",
				IPAddress:   session.IPAddress,
				Severity:    SeverityLow,
				Timestamp:   now,
			})
			delete(ss.sessions, sessionID)
			cleanedFromLegacy++
		}
	}

	totalCleaned := cleanedFromManager + cleanedFromLegacy
	if totalCleaned > 0 {
		logging.Info("Total sessions cleaned",
			logging.Int("session_manager", cleanedFromManager),
			logging.Int("legacy", cleanedFromLegacy),
			logging.Int("total", totalCleaned))
	}

	return totalCleaned
}

// GetSecurityStats returns security statistics including enhanced session data
func (ss *SecurityService) GetSecurityStats() SecurityStatsResponse {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	stats := SecurityStatsResponse{
		TotalUsers:     len(ss.users),
		ActiveSessions: 0,
		LockedAccounts: 0,
	}

	// Get enhanced session analytics
	sessionAnalytics := ss.sessionManager.GetSessionAnalytics()
	stats.ActiveSessions = sessionAnalytics.ActiveSessions

	// Add legacy active sessions
	for _, session := range ss.sessions {
		if session.IsValid() {
			stats.ActiveSessions++
		}
	}

	// Count locked accounts
	for _, user := range ss.users {
		if user.IsLocked() {
			stats.LockedAccounts++
		}
	}

	// Count recent login attempts (last hour)
	recentTime := time.Now().Add(-time.Hour)
	for _, attempt := range ss.loginAttempts {
		if attempt.Timestamp.After(recentTime) {
			stats.RecentAttempts++
			if !attempt.Success {
				stats.FailedAttempts++
			}
		}
	}

	stats.SecurityEvents = len(ss.securityEvents)

	return stats
}

// Stop gracefully shuts down the security service
func (ss *SecurityService) Stop() {
	if ss.sessionManager != nil {
		ss.sessionManager.Stop()
	}
	logging.Info("Security service stopped")
}

// Helper methods

func (ss *SecurityService) handleSuccessfulLogin(user *User, ipAddress, userAgent string) (*LoginResponse, error) {
	// Reset failed attempts
	user.FailedAttempts = 0
	user.LockedUntil = nil
	user.LastLoginAt = &time.Time{}
	*user.LastLoginAt = time.Now()
	user.UpdatedAt = time.Now()

	// Create session using internal method (mutex already locked)
	session, err := ss.createSessionInternal(user.ID, ipAddress, userAgent, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Record successful login
	ss.recordLoginAttempt(user.Username, ipAddress, userAgent, true, "")

	// Log security event
	ss.logSecurityEvent(&SecurityEvent{
		UserID:      &user.ID,
		EventType:   EventTypeLogin,
		Description: "Successful login",
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Severity:    SeverityLow,
		Timestamp:   time.Now(),
	})

	return &LoginResponse{
		Success:   true,
		Message:   "Login successful",
		SessionID: session.ID,
		ExpiresAt: session.ExpiresAt,
		User: &UserInfo{
			ID:          user.ID,
			Username:    user.Username,
			Email:       user.Email,
			IsAdmin:     user.IsAdmin,
			LastLoginAt: user.LastLoginAt,
			CreatedAt:   user.CreatedAt,
		},
	}, nil
}

func (ss *SecurityService) handleFailedLogin(user *User, ipAddress, userAgent string) {
	user.FailedAttempts++
	user.UpdatedAt = time.Now()

	// Check if account should be locked
	if user.FailedAttempts >= ss.config.MaxFailedAttempts {
		lockUntil := time.Now().Add(ss.config.LockoutDuration)
		user.LockedUntil = &lockUntil

		ss.logSecurityEvent(&SecurityEvent{
			UserID:      &user.ID,
			EventType:   EventTypeAccountLocked,
			Description: fmt.Sprintf("Account locked after %d failed attempts", user.FailedAttempts),
			IPAddress:   ipAddress,
			UserAgent:   userAgent,
			Severity:    SeverityHigh,
			Timestamp:   time.Now(),
		})

		logging.Warn("Account locked due to failed login attempts",
			logging.String("username", user.Username),
			logging.Int("attempts", user.FailedAttempts))
	}

	ss.recordLoginAttempt(user.Username, ipAddress, userAgent, false, "invalid password")
}

func (ss *SecurityService) recordLoginAttempt(username, ipAddress, userAgent string, success bool, failReason string) {
	attempt := LoginAttempt{
		ID:         len(ss.loginAttempts) + 1,
		Username:   username,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Success:    success,
		FailReason: failReason,
		Timestamp:  time.Now(),
	}

	ss.loginAttempts = append(ss.loginAttempts, attempt)

	// Keep only recent attempts (last 1000)
	if len(ss.loginAttempts) > 1000 {
		ss.loginAttempts = ss.loginAttempts[len(ss.loginAttempts)-1000:]
	}
}

func (ss *SecurityService) logSecurityEvent(event *SecurityEvent) {
	event.ID = len(ss.securityEvents) + 1
	ss.securityEvents = append(ss.securityEvents, *event)

	// Keep only recent events (last 1000)
	if len(ss.securityEvents) > 1000 {
		ss.securityEvents = ss.securityEvents[len(ss.securityEvents)-1000:]
	}

	// Log to system logger based on severity
	switch event.Severity {
	case SeverityCritical:
		logging.Error("Security event",
			logging.String("event_type", event.EventType),
			logging.String("description", event.Description),
			logging.String("ip_address", event.IPAddress))
	case SeverityHigh:
		logging.Warn("Security event",
			logging.String("event_type", event.EventType),
			logging.String("description", event.Description),
			logging.String("ip_address", event.IPAddress))
	default:
		logging.Info("Security event",
			logging.String("event_type", event.EventType),
			logging.String("description", event.Description))
	}
}

func (ss *SecurityService) checkRateLimit(ipAddress string) bool {
	now := time.Now()
	entry, exists := ss.rateLimiter[ipAddress]

	if !exists {
		ss.rateLimiter[ipAddress] = &rateLimitEntry{
			attempts:  1,
			resetTime: now.Add(time.Minute),
		}
		return true
	}

	// Reset counter if time window has passed
	if now.After(entry.resetTime) {
		entry.attempts = 1
		entry.resetTime = now.Add(time.Minute)
		return true
	}

	// Check limit
	if entry.attempts >= ss.config.LoginRateLimit {
		return false
	}

	entry.attempts++
	return true
}

func (ss *SecurityService) createSessionInternal(userID int, ipAddress, userAgent string, rememberMe bool) (*Session, error) {
	// Use the enhanced session manager for new sessions
	return ss.sessionManager.CreateSession(userID, ipAddress, userAgent, rememberMe)
}

// Additional session management endpoints for API

// SessionListResponse represents a list of sessions response
type SessionListResponse struct {
	Sessions []SessionInfo `json:"sessions"`
	Total    int           `json:"total"`
}

// SessionInfo represents public session information
type SessionInfo struct {
	ID           string    `json:"id"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	CreatedAt    time.Time `json:"created_at"`
	LastActivity time.Time `json:"last_activity"`
	ExpiresAt    time.Time `json:"expires_at"`
	IsActive     bool      `json:"is_active"`
	IsCurrent    bool      `json:"is_current"`
}

// GetUserSessionsInfo returns formatted session information for a user
func (ss *SecurityService) GetUserSessionsInfo(userID int, currentSessionID string) (*SessionListResponse, error) {
	sessions, err := ss.GetUserSessions(userID)
	if err != nil {
		return nil, err
	}

	var sessionInfos []SessionInfo
	for _, session := range sessions {
		// Get metrics if available
		var lastActivity time.Time
		if metrics, err := ss.GetSessionMetrics(session.ID); err == nil {
			lastActivity = metrics.LastActivity
		} else {
			lastActivity = session.UpdatedAt
		}

		sessionInfo := SessionInfo{
			ID:           session.ID,
			IPAddress:    session.IPAddress,
			UserAgent:    session.UserAgent,
			CreatedAt:    session.CreatedAt,
			LastActivity: lastActivity,
			ExpiresAt:    session.ExpiresAt,
			IsActive:     session.IsActive,
			IsCurrent:    session.ID == currentSessionID,
		}
		sessionInfos = append(sessionInfos, sessionInfo)
	}

	return &SessionListResponse{
		Sessions: sessionInfos,
		Total:    len(sessionInfos),
	}, nil
}
