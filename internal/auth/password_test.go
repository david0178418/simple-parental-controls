package auth

import (
	"testing"
)

// testPasswordConfig returns a password config optimized for testing (faster bcrypt)
func testPasswordConfig() PasswordConfig {
	config := DefaultPasswordConfig()
	config.BcryptCost = 4 // Use minimal cost for testing
	return config
}

// testAuthConfig returns an auth config optimized for testing
func testAuthConfig() AuthConfig {
	config := DefaultAuthConfig()
	config.Password.BcryptCost = 4 // Use minimal cost for testing
	return config
}

func TestPasswordHasher_HashPassword(t *testing.T) {
	config := testPasswordConfig()
	hasher := NewPasswordHasher(config)

	password := "TestPassword123!"
	hash, err := hasher.HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if hash == "" {
		t.Fatal("Hash should not be empty")
	}

	if hash == password {
		t.Fatal("Hash should not equal original password")
	}
}

func TestPasswordHasher_VerifyPassword(t *testing.T) {
	config := testPasswordConfig()
	hasher := NewPasswordHasher(config)

	password := "TestPassword123!"
	hash, err := hasher.HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	// Test correct password
	err = hasher.VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("Failed to verify correct password: %v", err)
	}

	// Test incorrect password
	err = hasher.VerifyPassword("WrongPassword", hash)
	if err == nil {
		t.Fatal("Should have failed to verify incorrect password")
	}
}

func TestPasswordHasher_ValidatePasswordStrength(t *testing.T) {
	config := testPasswordConfig()
	hasher := NewPasswordHasher(config)

	tests := []struct {
		password string
		valid    bool
		name     string
	}{
		{"TestPassword123!", true, "valid strong password"},
		{"testpassword123!", false, "missing uppercase"},
		{"TESTPASSWORD123!", false, "missing lowercase"},
		{"TestPassword!", false, "missing numbers"},
		{"TestPassword123", true, "valid without special chars (not required by default)"},
		{"Test12!", false, "too short"},
		{"", false, "empty password"},
		{"password", false, "common weak password"},
		{"123456", false, "common weak password"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := hasher.ValidatePasswordStrength(test.password)
			if test.valid && err != nil {
				t.Errorf("Expected password '%s' to be valid, but got error: %v", test.password, err)
			}
			if !test.valid && err == nil {
				t.Errorf("Expected password '%s' to be invalid, but it was accepted", test.password)
			}
		})
	}
}

func TestPasswordHasher_GenerateSecurePassword(t *testing.T) {
	config := testPasswordConfig()
	hasher := NewPasswordHasher(config)

	password, err := hasher.GenerateSecurePassword(12)
	if err != nil {
		t.Fatalf("Failed to generate secure password: %v", err)
	}

	if len(password) < 12 {
		t.Fatalf("Generated password is too short: %d characters", len(password))
	}

	// Validate that the generated password meets our strength requirements
	err = hasher.ValidatePasswordStrength(password)
	if err != nil {
		t.Fatalf("Generated password doesn't meet strength requirements: %v", err)
	}

	// Generate another password and ensure they're different
	password2, err := hasher.GenerateSecurePassword(12)
	if err != nil {
		t.Fatalf("Failed to generate second secure password: %v", err)
	}

	if password == password2 {
		t.Fatal("Generated passwords should be different")
	}
}

func TestPasswordManager_SetPassword(t *testing.T) {
	config := testPasswordConfig()
	manager := NewPasswordManager(config)

	password := "TestPassword123!"
	hash, err := manager.SetPassword(password)
	if err != nil {
		t.Fatalf("Failed to set password: %v", err)
	}

	if hash == "" {
		t.Fatal("Hash should not be empty")
	}

	// Verify the password
	err = manager.VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("Failed to verify password: %v", err)
	}
}

func TestPasswordManager_PasswordHistory(t *testing.T) {
	config := testPasswordConfig()
	config.PasswordHistorySize = 3
	manager := NewPasswordManager(config)

	passwords := []string{
		"FirstPassword123!",
		"SecondPassword123!",
		"ThirdPassword123!",
	}

	// Set passwords and track history
	for _, password := range passwords {
		_, err := manager.SetPassword(password)
		if err != nil {
			t.Fatalf("Failed to set password '%s': %v", password, err)
		}
	}

	// Try to reuse first password (should fail)
	_, err := manager.SetPassword(passwords[0])
	if err == nil {
		t.Fatal("Should have failed to reuse recent password")
	}

	// Try a new password (should succeed)
	newPassword := "FourthPassword123!"
	_, err = manager.SetPassword(newPassword)
	if err != nil {
		t.Fatalf("Failed to set new password: %v", err)
	}

	// Now first password should be allowed again (oldest entry)
	_, err = manager.SetPassword(passwords[0])
	if err != nil {
		t.Fatalf("Should be able to reuse oldest password: %v", err)
	}
}

func TestSecurityService_CreateInitialAdmin(t *testing.T) {
	config := testAuthConfig()
	service := NewSecurityService(config)

	err := service.CreateInitialAdmin("admin", "AdminPassword123!", "admin@example.com")
	if err != nil {
		t.Fatalf("Failed to create initial admin: %v", err)
	}

	// Try to create another admin (should fail)
	err = service.CreateInitialAdmin("admin2", "AdminPassword123!", "admin2@example.com")
	if err == nil {
		t.Fatal("Should have failed to create second admin")
	}
}

func TestSecurityService_Authenticate(t *testing.T) {
	config := testAuthConfig()
	service := NewSecurityService(config)

	// Create initial admin
	err := service.CreateInitialAdmin("admin", "AdminPassword123!", "admin@example.com")
	if err != nil {
		t.Fatalf("Failed to create initial admin: %v", err)
	}

	// Test successful authentication
	response, err := service.Authenticate("admin", "AdminPassword123!", "192.168.1.1", "test-agent")
	if err != nil {
		t.Fatalf("Failed to authenticate: %v", err)
	}

	if !response.Success {
		t.Fatal("Authentication should have succeeded")
	}

	if response.SessionID == "" {
		t.Fatal("Session ID should not be empty")
	}

	if response.User == nil {
		t.Fatal("User info should not be nil")
	}

	// Test failed authentication
	response, err = service.Authenticate("admin", "WrongPassword", "192.168.1.1", "test-agent")
	if err != nil {
		t.Fatalf("Unexpected error during failed authentication: %v", err)
	}

	if response.Success {
		t.Fatal("Authentication should have failed")
	}

	// Test nonexistent user
	response, err = service.Authenticate("nonexistent", "password", "192.168.1.1", "test-agent")
	if err != nil {
		t.Fatalf("Unexpected error during failed authentication: %v", err)
	}

	if response.Success {
		t.Fatal("Authentication should have failed for nonexistent user")
	}
}

func TestSecurityService_SessionValidation(t *testing.T) {
	config := testAuthConfig()
	service := NewSecurityService(config)

	// Create initial admin
	err := service.CreateInitialAdmin("admin", "AdminPassword123!", "admin@example.com")
	if err != nil {
		t.Fatalf("Failed to create initial admin: %v", err)
	}

	// Authenticate to get session
	response, err := service.Authenticate("admin", "AdminPassword123!", "192.168.1.1", "test-agent")
	if err != nil || !response.Success {
		t.Fatalf("Failed to authenticate: %v", err)
	}

	sessionID := response.SessionID

	// Validate session
	user, err := service.ValidateSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to validate session: %v", err)
	}

	if user.Username != "admin" {
		t.Fatalf("Expected username 'admin', got '%s'", user.Username)
	}

	// Test invalid session
	_, err = service.ValidateSession("invalid-session")
	if err == nil {
		t.Fatal("Should have failed to validate invalid session")
	}
}

func TestSecurityService_AccountLockout(t *testing.T) {
	config := testAuthConfig()
	config.MaxFailedAttempts = 3
	service := NewSecurityService(config)

	// Create initial admin
	err := service.CreateInitialAdmin("admin", "AdminPassword123!", "admin@example.com")
	if err != nil {
		t.Fatalf("Failed to create initial admin: %v", err)
	}

	// Make failed attempts
	for i := 0; i < 3; i++ {
		response, err := service.Authenticate("admin", "WrongPassword", "192.168.1.1", "test-agent")
		if err != nil {
			t.Fatalf("Unexpected error during failed authentication: %v", err)
		}
		if response.Success {
			t.Fatal("Authentication should have failed")
		}
	}

	// Account should now be locked
	response, err := service.Authenticate("admin", "AdminPassword123!", "192.168.1.1", "test-agent")
	if err != nil {
		t.Fatalf("Unexpected error during authentication: %v", err)
	}

	if response.Success {
		t.Fatal("Authentication should have failed due to account lockout")
	}

	if response.Message != "Account is temporarily locked. Please try again later." {
		t.Fatalf("Expected lockout message, got: %s", response.Message)
	}
}

func TestSecurityService_RateLimit(t *testing.T) {
	config := testAuthConfig()
	config.LoginRateLimit = 2 // Allow only 2 attempts per minute
	service := NewSecurityService(config)

	// Make rate limit attempts
	for i := 0; i < 3; i++ {
		response, err := service.Authenticate("nonexistent", "password", "192.168.1.1", "test-agent")
		if err != nil {
			t.Fatalf("Unexpected error during authentication: %v", err)
		}

		if i < 2 {
			// First two attempts should proceed normally (but fail due to user not existing)
			if response.Message == "Too many login attempts. Please try again later." {
				t.Fatal("Should not be rate limited yet")
			}
		} else {
			// Third attempt should be rate limited
			if response.Message != "Too many login attempts. Please try again later." {
				t.Fatalf("Expected rate limit message, got: %s", response.Message)
			}
		}
	}
}
