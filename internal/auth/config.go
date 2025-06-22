package auth

import (
	"fmt"
	"parental-control/internal/config"
)

// ConvertSecurityConfig converts the main SecurityConfig to AuthConfig
func ConvertSecurityConfig(securityConfig config.SecurityConfig) AuthConfig {
	return AuthConfig{
		Password: PasswordConfig{
			BcryptCost:          securityConfig.BcryptCost,
			MinLength:           securityConfig.MinPasswordLength,
			RequireUppercase:    securityConfig.RequireUppercase,
			RequireLowercase:    securityConfig.RequireLowercase,
			RequireNumbers:      securityConfig.RequireNumbers,
			RequireSpecialChars: securityConfig.RequireSpecialChars,
			PasswordHistorySize: securityConfig.PasswordHistorySize,
			PasswordExpireDays:  securityConfig.PasswordExpireDays,
		},
		SessionTimeout:        securityConfig.SessionTimeout,
		SessionSecret:         securityConfig.SessionSecret,
		RememberMeDuration:    securityConfig.RememberMeDuration,
		MaxFailedAttempts:     securityConfig.MaxFailedAttempts,
		LockoutDuration:       securityConfig.LockoutDuration,
		LoginRateLimit:        securityConfig.LoginRateLimit,
		RequireTwoFactor:      false, // Not implemented yet
		AllowMultipleSessions: securityConfig.AllowMultipleSessions,
		MaxSessions:           securityConfig.MaxSessions,
	}
}

// IsAuthenticationEnabled checks if authentication is enabled in the config
func IsAuthenticationEnabled(cfg *config.Config) bool {
	return cfg.Security.EnableAuth
}

// ValidateAuthConfig validates the authentication configuration
func ValidateAuthConfig(cfg AuthConfig) error {
	// Use the password hasher validation
	hasher := NewPasswordHasher(cfg.Password)

	// Validate with a test password to ensure config is valid
	testPassword := "TestPassword123!"
	if err := hasher.ValidatePasswordStrength(testPassword); err != nil {
		// If even a strong test password fails, the config is likely too restrictive
		return fmt.Errorf("password configuration is too restrictive: %w", err)
	}

	// Validate session settings
	if cfg.SessionTimeout <= 0 {
		return fmt.Errorf("session timeout must be positive")
	}

	if cfg.RememberMeDuration <= 0 {
		return fmt.Errorf("remember me duration must be positive")
	}

	if cfg.MaxFailedAttempts <= 0 {
		return fmt.Errorf("max failed attempts must be positive")
	}

	if cfg.LockoutDuration <= 0 {
		return fmt.Errorf("lockout duration must be positive")
	}

	return nil
}
