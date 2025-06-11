package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"parental-control/internal/config"
	"parental-control/internal/database"
	"parental-control/internal/logging"
	"parental-control/internal/models"
)

// TestDatabase provides a test database instance with utilities
type TestDatabase struct {
	DB      *database.DB
	TempDir string
	Config  database.Config
}

// NewTestDatabase creates a new test database instance in a temporary directory
func NewTestDatabase(t *testing.T) *TestDatabase {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	config := database.Config{
		Path:         dbPath,
		MaxOpenConns: 5,
		MaxIdleConns: 2,
		EnableWAL:    true,
	}

	db, err := database.New(config)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	if err := db.InitializeSchema(); err != nil {
		db.Close()
		t.Fatalf("Failed to initialize test database schema: %v", err)
	}

	return &TestDatabase{
		DB:      db,
		TempDir: tempDir,
		Config:  config,
	}
}

// Cleanup closes the database and cleans up temporary files
func (td *TestDatabase) Cleanup() {
	if td.DB != nil {
		td.DB.Close()
	}
}

// Reset clears all data from the database while keeping the schema
func (td *TestDatabase) Reset(t *testing.T) {
	t.Helper()

	// Clear all tables
	tables := []string{
		"quota_usage", "audit_log", "quota_rules", "time_rules",
		"list_entries", "lists", "config",
	}

	for _, table := range tables {
		_, err := td.DB.Connection().Exec(fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			t.Fatalf("Failed to clear table %s: %v", table, err)
		}
	}
}

// TestConfig provides test configuration utilities
type TestConfig struct {
	Config   *config.Config
	TempDir  string
	FilePath string
}

// NewTestConfig creates a new test configuration
func NewTestConfig(t *testing.T) *TestConfig {
	t.Helper()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	cfg := config.Default()
	cfg.Service.DataDirectory = tempDir
	cfg.Service.PIDFile = filepath.Join(tempDir, "test.pid")
	cfg.Database.Path = filepath.Join(tempDir, "test.db")
	cfg.Security.EnableAuth = false
	cfg.Monitoring.Enabled = false

	return &TestConfig{
		Config:   cfg,
		TempDir:  tempDir,
		FilePath: configPath,
	}
}

// SaveToFile saves the test configuration to file
func (tc *TestConfig) SaveToFile(t *testing.T) {
	t.Helper()

	if err := tc.Config.SaveToFile(tc.FilePath); err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}
}

// TestLogger provides test logging utilities
type TestLogger struct {
	Logger logging.Logger
	Level  string
}

// NewTestLogger creates a logger suitable for testing
func NewTestLogger(t *testing.T) *TestLogger {
	t.Helper()

	// Create a logger configuration for testing
	logConfig := logging.Config{
		Level:  logging.WARN,
		Output: os.Stdout,
	}

	logger := logging.New(logConfig)

	return &TestLogger{
		Logger: logger,
		Level:  "WARN",
	}
}

// Quiet sets the logger to only output errors and fatals
func (tl *TestLogger) Quiet() {
	tl.Logger.SetLevel(logging.ERROR)
	tl.Level = "ERROR"
}

// Verbose sets the logger to output all messages
func (tl *TestLogger) Verbose() {
	tl.Logger.SetLevel(logging.DEBUG)
	tl.Level = "DEBUG"
}

// TestFixtures provides test data fixtures
type TestFixtures struct {
	Config     models.Config
	Lists      []models.List
	Entries    []models.ListEntry
	TimeRule   models.TimeRule
	QuotaRule  models.QuotaRule
	QuotaUsage models.QuotaUsage
	AuditLog   models.AuditLog
}

// NewTestFixtures creates a set of test data fixtures
func NewTestFixtures() *TestFixtures {
	now := time.Now()

	return &TestFixtures{
		Config: models.Config{
			ID:          1,
			Key:         "test_setting",
			Value:       "test_value",
			Description: "Test configuration setting",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Lists: []models.List{
			{
				ID:          1,
				Name:        "Allowed Sites",
				Type:        models.ListTypeWhitelist,
				Description: "List of allowed websites",
				Enabled:     true,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          2,
				Name:        "Blocked Sites",
				Type:        models.ListTypeBlacklist,
				Description: "List of blocked websites",
				Enabled:     true,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
		Entries: []models.ListEntry{
			{
				ID:          1,
				ListID:      1,
				EntryType:   models.EntryTypeURL,
				Pattern:     "example.com",
				PatternType: models.PatternTypeDomain,
				Description: "Example domain",
				Enabled:     true,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          2,
				ListID:      2,
				EntryType:   models.EntryTypeURL,
				Pattern:     "blocked.com",
				PatternType: models.PatternTypeDomain,
				Description: "Blocked domain",
				Enabled:     true,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
		TimeRule: models.TimeRule{
			ID:         1,
			ListID:     1,
			Name:       "School Hours",
			RuleType:   models.RuleTypeAllowDuring,
			DaysOfWeek: []int{1, 2, 3, 4, 5}, // Monday to Friday
			StartTime:  "08:00",
			EndTime:    "16:00",
			Enabled:    true,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		QuotaRule: models.QuotaRule{
			ID:           1,
			ListID:       1,
			Name:         "Daily Limit",
			QuotaType:    models.QuotaTypeDaily,
			LimitSeconds: 3600, // 1 hour
			Enabled:      true,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		QuotaUsage: models.QuotaUsage{
			ID:          1,
			QuotaRuleID: 1,
			PeriodStart: now.Add(-24 * time.Hour),
			PeriodEnd:   now,
			UsedSeconds: 1800, // 30 minutes
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		AuditLog: models.AuditLog{
			ID:          1,
			Timestamp:   now,
			EventType:   "access_attempt",
			TargetType:  models.TargetTypeURL,
			TargetValue: "example.com",
			Action:      models.ActionTypeAllow,
			CreatedAt:   now,
		},
	}
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected error but got nil")
	}
}

// AssertErrorContains fails the test if err is nil or doesn't contain expectedText
func AssertErrorContains(t *testing.T, err error, expectedText string) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected error but got nil")
	}
	if !ContainsString(err.Error(), expectedText) {
		t.Fatalf("Expected error to contain '%s', but got: %v", expectedText, err)
	}
}

// AssertEqual fails the test if expected != actual
func AssertEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()
	if expected != actual {
		t.Fatalf("Expected %v, got %v", expected, actual)
	}
}

// AssertNotEqual fails the test if notExpected == actual
func AssertNotEqual[T comparable](t *testing.T, notExpected, actual T) {
	t.Helper()
	if notExpected == actual {
		t.Fatalf("Expected not %v, but got %v", notExpected, actual)
	}
}

// AssertTrue fails the test if condition is false
func AssertTrue(t *testing.T, condition bool) {
	t.Helper()
	if !condition {
		t.Fatal("Expected condition to be true")
	}
}

// AssertFalse fails the test if condition is true
func AssertFalse(t *testing.T, condition bool) {
	t.Helper()
	if condition {
		t.Fatal("Expected condition to be false")
	}
}

// AssertNil fails the test if value is not nil
func AssertNil(t *testing.T, value interface{}) {
	t.Helper()
	if value == nil {
		return
	}

	// Handle typed nil values (like (*int)(nil))
	v := reflect.ValueOf(value)
	if !v.IsValid() || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return
	}

	t.Fatalf("Expected nil, got %v", value)
}

// AssertNotNil fails the test if value is nil
func AssertNotNil(t *testing.T, value interface{}) {
	t.Helper()
	if value == nil {
		t.Fatal("Expected non-nil value")
	}
}

// ContainsString checks if s contains substr
func ContainsString(s, substr string) bool {
	return findString(s, substr)
}

// findString is a simple string contains check
func findString(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) == 0 {
		return false
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TimeWithinDelta checks if two times are within delta of each other
func TimeWithinDelta(t1, t2 time.Time, delta time.Duration) bool {
	diff := t1.Sub(t2)
	if diff < 0 {
		diff = -diff
	}
	return diff <= delta
}

// AssertTimeWithinDelta fails the test if expected and actual times are not within delta
func AssertTimeWithinDelta(t *testing.T, expected, actual time.Time, delta time.Duration) {
	t.Helper()
	if !TimeWithinDelta(expected, actual, delta) {
		t.Fatalf("Expected time %v to be within %v of %v", actual, delta, expected)
	}
}

// WaitForCondition waits for a condition to become true within timeout
func WaitForCondition(condition func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// AssertEventually fails the test if condition doesn't become true within timeout
func AssertEventually(t *testing.T, condition func() bool, timeout time.Duration) {
	t.Helper()
	if !WaitForCondition(condition, timeout) {
		t.Fatalf("Condition did not become true within %v", timeout)
	}
}

// SetupTestEnvironment creates a complete test environment with database, config, and logger
func SetupTestEnvironment(t *testing.T) (*TestDatabase, *TestConfig, *TestLogger) {
	t.Helper()

	db := NewTestDatabase(t)
	cfg := NewTestConfig(t)
	logger := NewTestLogger(t)

	// Set database path in config to match test database
	cfg.Config.Database = db.Config

	return db, cfg, logger
}

// BenchmarkHelper provides utilities for benchmarking
type BenchmarkHelper struct {
	StartTime time.Time
	EndTime   time.Time
}

// NewBenchmarkHelper creates a new benchmark helper
func NewBenchmarkHelper() *BenchmarkHelper {
	return &BenchmarkHelper{}
}

// Start records the start time
func (bh *BenchmarkHelper) Start() {
	bh.StartTime = time.Now()
}

// Stop records the end time and returns the duration
func (bh *BenchmarkHelper) Stop() time.Duration {
	bh.EndTime = time.Now()
	return bh.Duration()
}

// Duration returns the elapsed time between start and stop
func (bh *BenchmarkHelper) Duration() time.Duration {
	if bh.EndTime.IsZero() {
		return time.Since(bh.StartTime)
	}
	return bh.EndTime.Sub(bh.StartTime)
}
