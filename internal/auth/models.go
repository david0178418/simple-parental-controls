package auth

import (
	"errors"
	"time"
)

// Common errors
var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidSession     = errors.New("invalid session")
	ErrSessionExpired     = errors.New("session expired")
	ErrSessionNotFound    = errors.New("session not found")
	ErrAccountLocked      = errors.New("account locked")
	ErrRateLimitExceeded  = errors.New("rate limit exceeded")
	ErrPasswordTooWeak    = errors.New("password does not meet requirements")
	ErrPasswordReused     = errors.New("password was recently used")
)

// User represents an authenticated user account
type User struct {
	ID                int        `json:"id" db:"id"`
	Username          string     `json:"username" db:"username"`
	PasswordHash      string     `json:"-" db:"password_hash"` // Never expose in JSON
	Email             string     `json:"email" db:"email"`
	IsActive          bool       `json:"is_active" db:"is_active"`
	IsAdmin           bool       `json:"is_admin" db:"is_admin"`
	LastLoginAt       *time.Time `json:"last_login_at" db:"last_login_at"`
	PasswordChangedAt time.Time  `json:"password_changed_at" db:"password_changed_at"`
	FailedAttempts    int        `json:"failed_attempts" db:"failed_attempts"`
	LockedUntil       *time.Time `json:"locked_until" db:"locked_until"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}

// IsLocked returns true if the account is currently locked
func (u *User) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}

// PasswordExpired returns true if the password has expired
func (u *User) PasswordExpired(expireDays int) bool {
	if expireDays <= 0 {
		return false // No expiration policy
	}
	expireDate := u.PasswordChangedAt.AddDate(0, 0, expireDays)
	return time.Now().After(expireDate)
}

// AuthUser interface implementation for server package compatibility
func (u *User) GetID() int {
	return u.ID
}

func (u *User) GetUsername() string {
	return u.Username
}

func (u *User) GetEmail() string {
	return u.Email
}

func (u *User) HasAdminRole() bool {
	return u.IsAdmin
}

// UserPasswordHistory tracks password history for a user
type UserPasswordHistory struct {
	ID           int       `json:"id" db:"id"`
	UserID       int       `json:"user_id" db:"user_id"`
	PasswordHash string    `json:"-" db:"password_hash"` // Never expose in JSON
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// LoginAttempt tracks authentication attempts for security monitoring
type LoginAttempt struct {
	ID         int       `json:"id" db:"id"`
	Username   string    `json:"username" db:"username"`
	IPAddress  string    `json:"ip_address" db:"ip_address"`
	UserAgent  string    `json:"user_agent" db:"user_agent"`
	Success    bool      `json:"success" db:"success"`
	FailReason string    `json:"fail_reason" db:"fail_reason"`
	Timestamp  time.Time `json:"timestamp" db:"timestamp"`
}

// Session represents an active user session
type Session struct {
	ID        string    `json:"id" db:"id"`
	UserID    int       `json:"user_id" db:"user_id"`
	IPAddress string    `json:"ip_address" db:"ip_address"`
	UserAgent string    `json:"user_agent" db:"user_agent"`
	IsActive  bool      `json:"is_active" db:"is_active"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// IsExpired returns true if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsValid returns true if the session is active and not expired
func (s *Session) IsValid() bool {
	return s.IsActive && !s.IsExpired()
}

// AuthSession interface implementation for server package compatibility
func (s *Session) GetID() string {
	return s.ID
}

func (s *Session) GetUserID() int {
	return s.UserID
}

// SecurityEvent represents a security-related event for auditing
type SecurityEvent struct {
	ID          int       `json:"id" db:"id"`
	UserID      *int      `json:"user_id" db:"user_id"`
	EventType   string    `json:"event_type" db:"event_type"`
	Description string    `json:"description" db:"description"`
	IPAddress   string    `json:"ip_address" db:"ip_address"`
	UserAgent   string    `json:"user_agent" db:"user_agent"`
	Metadata    string    `json:"metadata" db:"metadata"` // JSON string for additional data
	Severity    string    `json:"severity" db:"severity"` // LOW, MEDIUM, HIGH, CRITICAL
	Timestamp   time.Time `json:"timestamp" db:"timestamp"`
}

// SecurityEventType constants for different types of security events
const (
	EventTypeLogin              = "login"
	EventTypeLoginFailed        = "login_failed"
	EventTypeLogout             = "logout"
	EventTypePasswordChange     = "password_change"
	EventTypeAccountLocked      = "account_locked"
	EventTypeAccountUnlocked    = "account_unlocked"
	EventTypePasswordReset      = "password_reset"
	EventTypeSessionExpired     = "session_expired"
	EventTypeSessionRevoked     = "session_revoked"
	EventTypeBruteForce         = "brute_force_detected"
	EventTypeUnauthorizedAccess = "unauthorized_access"
)

// SecurityEventSeverity constants for different severity levels
const (
	SeverityLow      = "LOW"
	SeverityMedium   = "MEDIUM"
	SeverityHigh     = "HIGH"
	SeverityCritical = "CRITICAL"
)

// AuthConfig represents authentication configuration
type AuthConfig struct {
	// Password configuration
	Password PasswordConfig `json:"password" yaml:"password"`

	// Session configuration
	SessionTimeout     time.Duration `json:"session_timeout" yaml:"session_timeout"`
	SessionSecret      string        `json:"-" yaml:"session_secret"` // Never expose in JSON
	RememberMeDuration time.Duration `json:"remember_me_duration" yaml:"remember_me_duration"`

	// Account lockout configuration
	MaxFailedAttempts int           `json:"max_failed_attempts" yaml:"max_failed_attempts"`
	LockoutDuration   time.Duration `json:"lockout_duration" yaml:"lockout_duration"`

	// Rate limiting configuration
	LoginRateLimit int `json:"login_rate_limit" yaml:"login_rate_limit"` // attempts per minute

	// Security configuration
	RequireTwoFactor      bool `json:"require_two_factor" yaml:"require_two_factor"`
	AllowMultipleSessions bool `json:"allow_multiple_sessions" yaml:"allow_multiple_sessions"`
	MaxSessions           int  `json:"max_sessions" yaml:"max_sessions"`
}

// DefaultAuthConfig returns default authentication configuration
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		Password:              DefaultPasswordConfig(),
		SessionTimeout:        24 * time.Hour,
		RememberMeDuration:    30 * 24 * time.Hour, // 30 days
		MaxFailedAttempts:     5,
		LockoutDuration:       15 * time.Minute,
		LoginRateLimit:        10, // 10 attempts per minute
		RequireTwoFactor:      false,
		AllowMultipleSessions: false,
		MaxSessions:           1,
	}
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	RememberMe bool   `json:"remember_me"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	SessionID string    `json:"session_id,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	User      *UserInfo `json:"user,omitempty"`
}

// UserInfo represents public user information (no sensitive data)
type UserInfo struct {
	ID          int        `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	IsAdmin     bool       `json:"is_admin"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ChangePasswordRequest represents a password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

// ChangePasswordResponse represents a password change response
type ChangePasswordResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// PasswordStrengthResponse represents password strength validation response
type PasswordStrengthResponse struct {
	Valid    bool     `json:"valid"`
	Score    int      `json:"score"`    // 0-100 strength score
	Feedback []string `json:"feedback"` // Validation messages
}

// AdminUserRequest represents a request to create/update a user (admin only)
type AdminUserRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password,omitempty"`
	IsAdmin  bool   `json:"is_admin"`
	IsActive bool   `json:"is_active"`
}

// SecurityStatsResponse represents security statistics
type SecurityStatsResponse struct {
	TotalUsers     int `json:"total_users"`
	ActiveSessions int `json:"active_sessions"`
	LockedAccounts int `json:"locked_accounts"`
	RecentAttempts int `json:"recent_attempts"`
	FailedAttempts int `json:"failed_attempts"`
	SecurityEvents int `json:"security_events"`
}
