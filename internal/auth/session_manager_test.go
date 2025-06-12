package auth

import (
	"testing"
	"time"
)

func testSessionConfig() AuthConfig {
	return AuthConfig{
		Password:              testPasswordConfig(),
		SessionTimeout:        time.Hour,
		RememberMeDuration:    24 * time.Hour,
		MaxFailedAttempts:     3,
		LockoutDuration:       15 * time.Minute,
		LoginRateLimit:        5,
		RequireTwoFactor:      false,
		AllowMultipleSessions: true,
		MaxSessions:           3,
	}
}

func TestSessionManager_CreateSession(t *testing.T) {
	config := testSessionConfig()
	sm := NewSessionManager(config)
	defer sm.Stop()

	userID := 1
	ipAddress := "192.168.1.1"
	userAgent := "test-agent"

	session, err := sm.CreateSession(userID, ipAddress, userAgent, false)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if session.ID == "" {
		t.Fatal("Session ID should not be empty")
	}

	if session.UserID != userID {
		t.Fatalf("Expected user ID %d, got %d", userID, session.UserID)
	}

	if !session.IsActive {
		t.Fatal("Session should be active")
	}
}

func TestSessionManager_ValidateSession(t *testing.T) {
	config := testSessionConfig()
	sm := NewSessionManager(config)
	defer sm.Stop()

	userID := 1
	session, err := sm.CreateSession(userID, "192.168.1.1", "test-agent", false)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	validatedSession, err := sm.ValidateSession(session.ID)
	if err != nil {
		t.Fatalf("Failed to validate session: %v", err)
	}

	if validatedSession.ID != session.ID {
		t.Fatalf("Expected session ID %s, got %s", session.ID, validatedSession.ID)
	}

	_, err = sm.ValidateSession("invalid-session-id")
	if err != ErrSessionNotFound {
		t.Fatalf("Expected ErrSessionNotFound, got %v", err)
	}
}

func TestSessionManager_ConcurrentSessionLimits(t *testing.T) {
	config := testSessionConfig()
	config.MaxSessions = 2
	sm := NewSessionManager(config)
	defer sm.Stop()

	userID := 1
	for i := 0; i < 3; i++ {
		_, err := sm.CreateSession(userID, "192.168.1.1", "test-agent", false)
		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i+1, err)
		}
	}

	userSessions, err := sm.GetUserSessions(userID)
	if err != nil {
		t.Fatalf("Failed to get user sessions: %v", err)
	}

	if len(userSessions) != config.MaxSessions {
		t.Fatalf("Expected %d active sessions, got %d", config.MaxSessions, len(userSessions))
	}
}

func TestSessionManager_RevokeSession(t *testing.T) {
	config := testSessionConfig()
	sm := NewSessionManager(config)
	defer sm.Stop()

	session, err := sm.CreateSession(1, "192.168.1.1", "test-agent", false)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	err = sm.RevokeSession(session.ID)
	if err != nil {
		t.Fatalf("Failed to revoke session: %v", err)
	}

	_, err = sm.ValidateSession(session.ID)
	if err == nil {
		t.Fatal("Revoked session should not be valid")
	}
}

func TestSessionManager_Analytics(t *testing.T) {
	config := testSessionConfig()
	sm := NewSessionManager(config)
	defer sm.Stop()

	userIDs := []int{1, 2, 3}
	for _, userID := range userIDs {
		for i := 0; i < 2; i++ {
			_, err := sm.CreateSession(userID, "192.168.1.1", "test-agent", false)
			if err != nil {
				t.Fatalf("Failed to create session for user %d: %v", userID, err)
			}
		}
	}

	analytics := sm.GetSessionAnalytics()
	expectedTotal := len(userIDs) * 2

	if analytics.TotalSessions != expectedTotal {
		t.Fatalf("Expected %d total sessions, got %d", expectedTotal, analytics.TotalSessions)
	}

	if analytics.ActiveSessions != expectedTotal {
		t.Fatalf("Expected %d active sessions, got %d", expectedTotal, analytics.ActiveSessions)
	}
}

func TestSessionManager_CleanupExpiredSessions(t *testing.T) {
	config := testSessionConfig()
	config.SessionTimeout = 100 * time.Millisecond
	sm := NewSessionManager(config)
	defer sm.Stop()

	session, err := sm.CreateSession(1, "192.168.1.1", "test-agent", false)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Don't validate the session before cleanup, as validation removes expired sessions automatically
	// Just call cleanup directly to test the cleanup functionality
	cleaned := sm.CleanupExpiredSessions()
	if cleaned != 1 {
		t.Fatalf("Expected 1 cleaned session, got %d", cleaned)
	}

	// Now verify session is no longer accessible
	_, err = sm.ValidateSession(session.ID)
	if err == nil {
		t.Fatal("Session should be expired and cleaned up")
	}
}
