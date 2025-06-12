package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// PasswordConfig holds password-related configuration
type PasswordConfig struct {
	// BcryptCost for hashing (4-31, recommended: 12)
	BcryptCost int
	// MinLength minimum password length
	MinLength int
	// RequireUppercase requires at least one uppercase letter
	RequireUppercase bool
	// RequireLowercase requires at least one lowercase letter
	RequireLowercase bool
	// RequireNumbers requires at least one number
	RequireNumbers bool
	// RequireSpecialChars requires at least one special character
	RequireSpecialChars bool
	// PasswordHistorySize number of previous passwords to remember
	PasswordHistorySize int
	// PasswordExpireDays password expiration in days (0 = no expiration)
	PasswordExpireDays int
}

// DefaultPasswordConfig returns secure password configuration defaults
func DefaultPasswordConfig() PasswordConfig {
	return PasswordConfig{
		BcryptCost:          12, // Good balance of security and performance
		MinLength:           8,
		RequireUppercase:    true,
		RequireLowercase:    true,
		RequireNumbers:      true,
		RequireSpecialChars: false, // Optional for easier setup
		PasswordHistorySize: 5,
		PasswordExpireDays:  0, // No expiration by default
	}
}

// PasswordHasher handles password hashing and validation operations
type PasswordHasher struct {
	config PasswordConfig
}

// NewPasswordHasher creates a new password hasher with the given configuration
func NewPasswordHasher(config PasswordConfig) *PasswordHasher {
	return &PasswordHasher{
		config: config,
	}
}

// HashPassword generates a bcrypt hash of the given password
func (ph *PasswordHasher) HashPassword(password string) (string, error) {
	if err := ph.ValidatePasswordStrength(password); err != nil {
		return "", fmt.Errorf("password validation failed: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), ph.config.BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hash), nil
}

// VerifyPassword compares a password with its hash
func (ph *PasswordHasher) VerifyPassword(password, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return errors.New("invalid password")
		}
		return fmt.Errorf("password verification failed: %w", err)
	}
	return nil
}

// ValidatePasswordStrength checks if password meets strength requirements
func (ph *PasswordHasher) ValidatePasswordStrength(password string) error {
	var errors []string

	// Check minimum length
	if len(password) < ph.config.MinLength {
		errors = append(errors, fmt.Sprintf("password must be at least %d characters long", ph.config.MinLength))
	}

	// Check for uppercase letters
	if ph.config.RequireUppercase {
		if !regexp.MustCompile(`[A-Z]`).MatchString(password) {
			errors = append(errors, "password must contain at least one uppercase letter")
		}
	}

	// Check for lowercase letters
	if ph.config.RequireLowercase {
		if !regexp.MustCompile(`[a-z]`).MatchString(password) {
			errors = append(errors, "password must contain at least one lowercase letter")
		}
	}

	// Check for numbers
	if ph.config.RequireNumbers {
		if !regexp.MustCompile(`[0-9]`).MatchString(password) {
			errors = append(errors, "password must contain at least one number")
		}
	}

	// Check for special characters
	if ph.config.RequireSpecialChars {
		if !regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password) {
			errors = append(errors, "password must contain at least one special character")
		}
	}

	// Check for common weak passwords
	if ph.isCommonPassword(password) {
		errors = append(errors, "password is too common")
	}

	if len(errors) > 0 {
		return fmt.Errorf("password strength validation failed: %s", strings.Join(errors, "; "))
	}

	return nil
}

// GenerateSecurePassword generates a cryptographically secure random password
func (ph *PasswordHasher) GenerateSecurePassword(length int) (string, error) {
	if length < ph.config.MinLength {
		length = ph.config.MinLength
	}

	// Character sets
	lowercase := "abcdefghijklmnopqrstuvwxyz"
	uppercase := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	numbers := "0123456789"
	special := "!@#$%^&*()_+-=[]{}|;':\",./<>?"

	var charset string
	var password strings.Builder

	// Ensure required character types are included
	if ph.config.RequireLowercase {
		char, err := ph.getRandomChar(lowercase)
		if err != nil {
			return "", err
		}
		password.WriteByte(char)
		charset += lowercase
	}

	if ph.config.RequireUppercase {
		char, err := ph.getRandomChar(uppercase)
		if err != nil {
			return "", err
		}
		password.WriteByte(char)
		charset += uppercase
	}

	if ph.config.RequireNumbers {
		char, err := ph.getRandomChar(numbers)
		if err != nil {
			return "", err
		}
		password.WriteByte(char)
		charset += numbers
	}

	if ph.config.RequireSpecialChars {
		char, err := ph.getRandomChar(special)
		if err != nil {
			return "", err
		}
		password.WriteByte(char)
		charset += special
	}

	// If no specific requirements, use all character sets
	if charset == "" {
		charset = lowercase + uppercase + numbers + special
	}

	// Fill remaining length with random characters
	for password.Len() < length {
		char, err := ph.getRandomChar(charset)
		if err != nil {
			return "", err
		}
		password.WriteByte(char)
	}

	// Shuffle the password to avoid predictable patterns
	return ph.shuffleString(password.String())
}

// getRandomChar returns a cryptographically secure random character from the given charset
func (ph *PasswordHasher) getRandomChar(charset string) (byte, error) {
	max := big.NewInt(int64(len(charset)))
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0, fmt.Errorf("failed to generate random character: %w", err)
	}
	return charset[n.Int64()], nil
}

// shuffleString shuffles the characters in a string using Fisher-Yates algorithm
func (ph *PasswordHasher) shuffleString(s string) (string, error) {
	chars := []byte(s)
	for i := len(chars) - 1; i > 0; i-- {
		max := big.NewInt(int64(i + 1))
		j, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("failed to shuffle password: %w", err)
		}
		chars[i], chars[j.Int64()] = chars[j.Int64()], chars[i]
	}
	return string(chars), nil
}

// isCommonPassword checks if the password is in a list of common weak passwords
func (ph *PasswordHasher) isCommonPassword(password string) bool {
	// List of common weak passwords - in production, this could be loaded from a file
	commonPasswords := []string{
		"password", "123456", "password123", "admin", "qwerty",
		"letmein", "welcome", "monkey", "dragon", "master",
		"shadow", "123456789", "football", "baseball", "superman",
		"princess", "sunshine", "iloveyou", "trustno1", "starwars",
	}

	lowerPassword := strings.ToLower(password)
	for _, common := range commonPasswords {
		if lowerPassword == common {
			return true
		}
	}
	return false
}

// PasswordHistory represents a historical password entry
type PasswordHistory struct {
	Hash      string    `json:"hash" db:"hash"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// PasswordManager handles password management operations including history
type PasswordManager struct {
	hasher  *PasswordHasher
	history []PasswordHistory
}

// NewPasswordManager creates a new password manager
func NewPasswordManager(config PasswordConfig) *PasswordManager {
	return &PasswordManager{
		hasher:  NewPasswordHasher(config),
		history: make([]PasswordHistory, 0, config.PasswordHistorySize),
	}
}

// SetPassword sets a new password after validating it doesn't match recent passwords
func (pm *PasswordManager) SetPassword(newPassword string) (string, error) {
	// Validate password strength
	if err := pm.hasher.ValidatePasswordStrength(newPassword); err != nil {
		return "", err
	}

	// Check against password history
	if err := pm.checkPasswordHistory(newPassword); err != nil {
		return "", err
	}

	// Hash the new password
	hash, err := pm.hasher.HashPassword(newPassword)
	if err != nil {
		return "", err
	}

	// Add to history
	pm.addToHistory(hash)

	return hash, nil
}

// VerifyPassword verifies a password against the current hash
func (pm *PasswordManager) VerifyPassword(password, currentHash string) error {
	return pm.hasher.VerifyPassword(password, currentHash)
}

// GeneratePassword generates a secure password
func (pm *PasswordManager) GeneratePassword(length int) (string, error) {
	return pm.hasher.GenerateSecurePassword(length)
}

// checkPasswordHistory ensures the new password hasn't been used recently
func (pm *PasswordManager) checkPasswordHistory(newPassword string) error {
	for _, historyEntry := range pm.history {
		if pm.hasher.VerifyPassword(newPassword, historyEntry.Hash) == nil {
			return errors.New("password has been used recently and cannot be reused")
		}
	}
	return nil
}

// addToHistory adds a password hash to the history, maintaining the configured size
func (pm *PasswordManager) addToHistory(hash string) {
	entry := PasswordHistory{
		Hash:      hash,
		CreatedAt: time.Now(),
	}

	pm.history = append(pm.history, entry)

	// Maintain history size limit
	if len(pm.history) > pm.hasher.config.PasswordHistorySize {
		pm.history = pm.history[1:] // Remove oldest entry
	}
}

// LoadHistory loads password history (would typically come from database)
func (pm *PasswordManager) LoadHistory(history []PasswordHistory) {
	pm.history = make([]PasswordHistory, len(history))
	copy(pm.history, history)
}

// GetHistory returns the current password history
func (pm *PasswordManager) GetHistory() []PasswordHistory {
	result := make([]PasswordHistory, len(pm.history))
	copy(result, pm.history)
	return result
}
