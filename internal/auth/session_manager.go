package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"parental-control/internal/logging"
)

// SessionManager handles advanced session management features
type SessionManager struct {
	config          AuthConfig
	sessions        map[string]*Session
	userSessions    map[int][]string
	sessionMetrics  map[string]*SessionMetrics
	cleanupInterval time.Duration
	stopCleanup     chan bool
	mu              sync.RWMutex
}

// SessionMetrics tracks detailed session analytics
type SessionMetrics struct {
	SessionID    string    `json:"session_id"`
	UserID       int       `json:"user_id"`
	CreatedAt    time.Time `json:"created_at"`
	LastActivity time.Time `json:"last_activity"`
	RequestCount int       `json:"request_count"`
	IPAddresses  []string  `json:"ip_addresses"`
	UserAgents   []string  `json:"user_agents"`
}

// SessionStorage interface for different storage backends
type SessionStorage interface {
	Save(session *Session) error
	Load(sessionID string) (*Session, error)
	Delete(sessionID string) error
	LoadUserSessions(userID int) ([]*Session, error)
	LoadExpiredSessions() ([]*Session, error)
}

// MemorySessionStorage implements in-memory session storage
type MemorySessionStorage struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// SessionRenewalPolicy defines session renewal behavior
type SessionRenewalPolicy struct {
	EnableAutoRenewal   bool          `json:"enable_auto_renewal"`
	RenewalThreshold    time.Duration `json:"renewal_threshold"`    // Renew when session has less than this time left
	MaxRenewalCount     int           `json:"max_renewal_count"`    // Maximum times a session can be renewed
	IdleTimeout         time.Duration `json:"idle_timeout"`         // Session expires after this much inactivity
	AbsoluteTimeout     time.Duration `json:"absolute_timeout"`     // Session expires after this much total time
	RequireReauth       bool          `json:"require_reauth"`       // Require re-authentication for sensitive operations
	SensitiveOperations []string      `json:"sensitive_operations"` // Operations that require re-authentication
}

// SessionAnalytics provides session usage analytics
type SessionAnalytics struct {
	TotalSessions      int              `json:"total_sessions"`
	ActiveSessions     int              `json:"active_sessions"`
	ExpiredSessions    int              `json:"expired_sessions"`
	AverageSessionTime time.Duration    `json:"average_session_time"`
	TopIPAddresses     []IPAddressStats `json:"top_ip_addresses"`
	SessionsByHour     map[int]int      `json:"sessions_by_hour"`
}

// IPAddressStats tracks statistics for IP addresses
type IPAddressStats struct {
	IPAddress    string    `json:"ip_address"`
	SessionCount int       `json:"session_count"`
	LastSeen     time.Time `json:"last_seen"`
}

// UserAgentStats tracks statistics for user agents
type UserAgentStats struct {
	UserAgent    string    `json:"user_agent"`
	SessionCount int       `json:"session_count"`
	LastSeen     time.Time `json:"last_seen"`
}

// NewSessionManager creates a new advanced session manager
func NewSessionManager(config AuthConfig) *SessionManager {
	sm := &SessionManager{
		config:          config,
		sessions:        make(map[string]*Session),
		userSessions:    make(map[int][]string),
		sessionMetrics:  make(map[string]*SessionMetrics),
		cleanupInterval: 15 * time.Minute,
		stopCleanup:     make(chan bool),
	}

	go sm.startCleanupRoutine()
	return sm
}

// CreateSession creates a new session with advanced features
func (sm *SessionManager) CreateSession(userID int, ipAddress, userAgent string, rememberMe bool) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sessionID, err := sm.generateSecureSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	duration := sm.config.SessionTimeout
	if rememberMe {
		duration = sm.config.RememberMeDuration
	}

	now := time.Now()
	session := &Session{
		ID:        sessionID,
		UserID:    userID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		IsActive:  true,
		ExpiresAt: now.Add(duration),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := sm.enforceConcurrentSessionLimits(userID); err != nil {
		return nil, fmt.Errorf("failed to enforce session limits: %w", err)
	}

	sm.sessions[sessionID] = session
	sm.addUserSession(userID, sessionID)

	sm.sessionMetrics[sessionID] = &SessionMetrics{
		SessionID:    sessionID,
		UserID:       userID,
		CreatedAt:    now,
		LastActivity: now,
		RequestCount: 0,
		IPAddresses:  []string{ipAddress},
		UserAgents:   []string{userAgent},
	}

	logging.Info("Session created",
		logging.String("session_id", sessionID),
		logging.Int("user_id", userID),
		logging.String("ip_address", ipAddress))

	return session, nil
}

// ValidateSession validates a session and updates activity tracking
func (sm *SessionManager) ValidateSession(sessionID string) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}

	if !session.IsValid() {
		sm.removeSessionInternal(sessionID)
		return nil, ErrSessionExpired
	}

	now := time.Now()
	session.UpdatedAt = now

	if metrics, exists := sm.sessionMetrics[sessionID]; exists {
		metrics.LastActivity = now
		metrics.RequestCount++
	}

	return session, nil
}

// RefreshSession extends a session's lifetime
func (sm *SessionManager) RefreshSession(sessionID string, extendBy time.Duration) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	if !session.IsValid() {
		return ErrSessionExpired
	}

	session.ExpiresAt = session.ExpiresAt.Add(extendBy)
	session.UpdatedAt = time.Now()

	logging.Info("Session refreshed",
		logging.String("session_id", sessionID))

	return nil
}

// RevokeSession revokes a specific session
func (sm *SessionManager) RevokeSession(sessionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.removeSessionInternal(sessionID)
}

// RevokeUserSessions revokes all sessions for a user
func (sm *SessionManager) RevokeUserSessions(userID int) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sessionIDs, exists := sm.userSessions[userID]
	if !exists {
		return nil
	}

	for _, sessionID := range sessionIDs {
		sm.removeSessionInternal(sessionID)
	}

	delete(sm.userSessions, userID)

	logging.Info("All user sessions revoked",
		logging.Int("user_id", userID),
		logging.Int("session_count", len(sessionIDs)))

	return nil
}

// GetUserSessions returns all active sessions for a user
func (sm *SessionManager) GetUserSessions(userID int) ([]*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessionIDs, exists := sm.userSessions[userID]
	if !exists {
		return []*Session{}, nil
	}

	var sessions []*Session
	for _, sessionID := range sessionIDs {
		if session, exists := sm.sessions[sessionID]; exists && session.IsValid() {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// UpdateSessionActivity updates session activity with new IP/User-Agent
func (sm *SessionManager) UpdateSessionActivity(sessionID, ipAddress, userAgent string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	if session.IPAddress != ipAddress {
		session.IPAddress = ipAddress
		session.UpdatedAt = time.Now()
	}

	if session.UserAgent != userAgent {
		session.UserAgent = userAgent
		session.UpdatedAt = time.Now()
	}

	if metrics, exists := sm.sessionMetrics[sessionID]; exists {
		metrics.LastActivity = time.Now()

		if !contains(metrics.IPAddresses, ipAddress) {
			metrics.IPAddresses = append(metrics.IPAddresses, ipAddress)
		}

		if !contains(metrics.UserAgents, userAgent) {
			metrics.UserAgents = append(metrics.UserAgents, userAgent)
		}
	}

	return nil
}

// GetSessionAnalytics returns comprehensive session analytics
func (sm *SessionManager) GetSessionAnalytics() *SessionAnalytics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	analytics := &SessionAnalytics{
		SessionsByHour: make(map[int]int),
	}

	ipMap := make(map[string]int)
	var totalDuration time.Duration
	var validSessionCount int

	for _, session := range sm.sessions {
		analytics.TotalSessions++

		if session.IsValid() {
			analytics.ActiveSessions++
			validSessionCount++

			duration := time.Since(session.CreatedAt)
			totalDuration += duration

			ipMap[session.IPAddress]++

			hour := session.CreatedAt.Hour()
			analytics.SessionsByHour[hour]++
		} else {
			analytics.ExpiredSessions++
		}
	}

	if validSessionCount > 0 {
		analytics.AverageSessionTime = totalDuration / time.Duration(validSessionCount)
	}

	for ip, count := range ipMap {
		analytics.TopIPAddresses = append(analytics.TopIPAddresses, IPAddressStats{
			IPAddress:    ip,
			SessionCount: count,
			LastSeen:     time.Now(),
		})
	}

	sort.Slice(analytics.TopIPAddresses, func(i, j int) bool {
		return analytics.TopIPAddresses[i].SessionCount > analytics.TopIPAddresses[j].SessionCount
	})

	if len(analytics.TopIPAddresses) > 10 {
		analytics.TopIPAddresses = analytics.TopIPAddresses[:10]
	}

	return analytics
}

// CleanupExpiredSessions removes expired sessions and metrics
func (sm *SessionManager) CleanupExpiredSessions() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var cleanedCount int

	for sessionID, session := range sm.sessions {
		if session.IsExpired() {
			sm.removeSessionInternal(sessionID)
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		logging.Info("Session cleanup completed",
			logging.Int("cleaned_sessions", cleanedCount))
	}

	return cleanedCount
}

// GetSessionMetrics returns detailed metrics for a session
func (sm *SessionManager) GetSessionMetrics(sessionID string) (*SessionMetrics, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	metrics, exists := sm.sessionMetrics[sessionID]
	if !exists {
		return nil, fmt.Errorf("metrics not found for session %s", sessionID)
	}

	return metrics, nil
}

// ExportSessions exports all session data for backup/analysis
func (sm *SessionManager) ExportSessions() ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	exportData := struct {
		Sessions   map[string]*Session        `json:"sessions"`
		Metrics    map[string]*SessionMetrics `json:"metrics"`
		ExportedAt time.Time                  `json:"exported_at"`
	}{
		Sessions:   sm.sessions,
		Metrics:    sm.sessionMetrics,
		ExportedAt: time.Now(),
	}

	return json.Marshal(exportData)
}

// Stop gracefully stops the session manager
func (sm *SessionManager) Stop() {
	close(sm.stopCleanup)
	logging.Info("Session manager stopped")
}

// Private helper methods

func (sm *SessionManager) generateSecureSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (sm *SessionManager) enforceConcurrentSessionLimits(userID int) error {
	if sm.config.AllowMultipleSessions {
		sessionIDs := sm.userSessions[userID]
		if len(sessionIDs) >= sm.config.MaxSessions {
			if len(sessionIDs) > 0 {
				oldestSessionID := sm.findOldestSession(sessionIDs)
				sm.removeSessionInternal(oldestSessionID)
			}
		}
	} else {
		if sessionIDs, exists := sm.userSessions[userID]; exists {
			for _, sessionID := range sessionIDs {
				sm.removeSessionInternal(sessionID)
			}
		}
	}

	return nil
}

func (sm *SessionManager) findOldestSession(sessionIDs []string) string {
	if len(sessionIDs) == 0 {
		return ""
	}

	oldestID := sessionIDs[0]
	oldestTime := time.Now()

	for _, sessionID := range sessionIDs {
		if session, exists := sm.sessions[sessionID]; exists {
			if session.CreatedAt.Before(oldestTime) {
				oldestTime = session.CreatedAt
				oldestID = sessionID
			}
		}
	}

	return oldestID
}

func (sm *SessionManager) addUserSession(userID int, sessionID string) {
	if _, exists := sm.userSessions[userID]; !exists {
		sm.userSessions[userID] = make([]string, 0)
	}
	sm.userSessions[userID] = append(sm.userSessions[userID], sessionID)
}

func (sm *SessionManager) removeSessionInternal(sessionID string) error {
	session, exists := sm.sessions[sessionID]
	if !exists {
		return ErrSessionNotFound
	}

	delete(sm.sessions, sessionID)

	if sessionIDs, exists := sm.userSessions[session.UserID]; exists {
		newSessionIDs := make([]string, 0, len(sessionIDs)-1)
		for _, id := range sessionIDs {
			if id != sessionID {
				newSessionIDs = append(newSessionIDs, id)
			}
		}
		if len(newSessionIDs) == 0 {
			delete(sm.userSessions, session.UserID)
		} else {
			sm.userSessions[session.UserID] = newSessionIDs
		}
	}

	delete(sm.sessionMetrics, sessionID)

	logging.Debug("Session removed",
		logging.String("session_id", sessionID),
		logging.Int("user_id", session.UserID))

	return nil
}

func (sm *SessionManager) startCleanupRoutine() {
	ticker := time.NewTicker(sm.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.CleanupExpiredSessions()
		case <-sm.stopCleanup:
			return
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Memory storage implementation
func NewMemorySessionStorage() *MemorySessionStorage {
	return &MemorySessionStorage{
		sessions: make(map[string]*Session),
	}
}

func (mss *MemorySessionStorage) Save(session *Session) error {
	mss.mu.Lock()
	defer mss.mu.Unlock()
	mss.sessions[session.ID] = session
	return nil
}

func (mss *MemorySessionStorage) Load(sessionID string) (*Session, error) {
	mss.mu.RLock()
	defer mss.mu.RUnlock()

	session, exists := mss.sessions[sessionID]
	if !exists {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

func (mss *MemorySessionStorage) Delete(sessionID string) error {
	mss.mu.Lock()
	defer mss.mu.Unlock()
	delete(mss.sessions, sessionID)
	return nil
}

func (mss *MemorySessionStorage) LoadUserSessions(userID int) ([]*Session, error) {
	mss.mu.RLock()
	defer mss.mu.RUnlock()

	var sessions []*Session
	for _, session := range mss.sessions {
		if session.UserID == userID {
			sessions = append(sessions, session)
		}
	}
	return sessions, nil
}

func (mss *MemorySessionStorage) LoadExpiredSessions() ([]*Session, error) {
	mss.mu.RLock()
	defer mss.mu.RUnlock()

	var expiredSessions []*Session
	for _, session := range mss.sessions {
		if session.IsExpired() {
			expiredSessions = append(expiredSessions, session)
		}
	}
	return expiredSessions, nil
}
