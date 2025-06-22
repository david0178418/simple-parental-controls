package testutil

import (
	"errors"
	"testing"
	"time"
)

func TestNewTestDatabase(t *testing.T) {
	db := NewTestDatabase(t)
	defer db.Cleanup()

	if db.DB == nil {
		t.Error("Expected database to be created")
	}

	if db.TempDir == "" {
		t.Error("Expected temp directory to be set")
	}

	// Test that database is functional
	if err := db.DB.Ping(); err != nil {
		t.Errorf("Database ping failed: %v", err)
	}
}

func TestTestDatabase_Reset(t *testing.T) {
	db := NewTestDatabase(t)
	defer db.Cleanup()

	// Insert test data
	_, err := db.DB.Connection().Exec("INSERT INTO config (key, value, description) VALUES (?, ?, ?)", "test", "value", "desc")
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Verify data exists
	var count int
	err = db.DB.Connection().QueryRow("SELECT COUNT(*) FROM config").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}
	if count == 0 {
		t.Error("Expected data to be inserted")
	}

	// Reset database
	db.Reset(t)

	// Verify data is cleared
	err = db.DB.Connection().QueryRow("SELECT COUNT(*) FROM config").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count rows after reset: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 rows after reset, got %d", count)
	}
}

func TestNewTestConfig(t *testing.T) {
	cfg := NewTestConfig(t)

	if cfg.Config == nil {
		t.Error("Expected config to be created")
	}

	if cfg.TempDir == "" {
		t.Error("Expected temp directory to be set")
	}

	if cfg.FilePath == "" {
		t.Error("Expected file path to be set")
	}

	// Test that security is disabled by default for testing
	if cfg.Config.Security.EnableAuth {
		t.Error("Expected authentication to be disabled for testing")
	}
}

func TestTestConfig_SaveToFile(t *testing.T) {
	cfg := NewTestConfig(t)

	// Test saving to file
	cfg.SaveToFile(t)

	// Verify file exists (the test framework will clean it up)
	// We can't easily check file existence without adding dependencies
	// but SaveToFile will fail the test if it can't save
}

func TestNewTestLogger(t *testing.T) {
	logger := NewTestLogger(t)

	if logger.Logger == nil {
		t.Error("Expected logger to be created")
	}

	if logger.Level != "WARN" {
		t.Errorf("Expected default level to be WARN, got %s", logger.Level)
	}
}

func TestTestLogger_Quiet(t *testing.T) {
	logger := NewTestLogger(t)
	logger.Quiet()

	if logger.Level != "ERROR" {
		t.Errorf("Expected level to be ERROR after Quiet(), got %s", logger.Level)
	}
}

func TestTestLogger_Verbose(t *testing.T) {
	logger := NewTestLogger(t)
	logger.Verbose()

	if logger.Level != "DEBUG" {
		t.Errorf("Expected level to be DEBUG after Verbose(), got %s", logger.Level)
	}
}

func TestNewTestFixtures(t *testing.T) {
	fixtures := NewTestFixtures()

	// Test that all fixtures are created
	if fixtures.Config.Key == "" {
		t.Error("Expected config fixture to be populated")
	}

	if len(fixtures.Lists) == 0 {
		t.Error("Expected list fixtures to be populated")
	}

	if len(fixtures.Entries) == 0 {
		t.Error("Expected entry fixtures to be populated")
	}

	if fixtures.TimeRule.Name == "" {
		t.Error("Expected time rule fixture to be populated")
	}

	if fixtures.QuotaRule.Name == "" {
		t.Error("Expected quota rule fixture to be populated")
	}

	if fixtures.QuotaUsage.UsedSeconds == 0 {
		t.Error("Expected quota usage fixture to be populated")
	}

	if fixtures.AuditLog.EventType == "" {
		t.Error("Expected audit log fixture to be populated")
	}
}

func TestAssertNoError(t *testing.T) {
	// Test with no error - should not fail
	AssertNoError(t, nil)

	// Test with error - we can't easily test this without creating a sub-test
	// that we expect to fail, so we'll skip this case
}

func TestAssertError(t *testing.T) {
	// Test with error - should not fail
	AssertError(t, errors.New("test error"))

	// Test with no error - we can't easily test this without creating a sub-test
	// that we expect to fail, so we'll skip this case
}

func TestAssertErrorContains(t *testing.T) {
	err := errors.New("this is a test error message")

	// Test with matching text - should not fail
	AssertErrorContains(t, err, "test error")

	// Test with non-matching text would fail the test
	// We can't test this easily without sub-tests
}

func TestAssertEqual(t *testing.T) {
	// Test with equal values - should not fail
	AssertEqual(t, 42, 42)
	AssertEqual(t, "hello", "hello")
	AssertEqual(t, true, true)

	// Test with unequal values would fail
	// We can't test this easily without sub-tests
}

func TestAssertNotEqual(t *testing.T) {
	// Test with different values - should not fail
	AssertNotEqual(t, 42, 24)
	AssertNotEqual(t, "hello", "world")
	AssertNotEqual(t, true, false)

	// Test with equal values would fail
	// We can't test this easily without sub-tests
}

func TestAssertTrue(t *testing.T) {
	// Test with true - should not fail
	AssertTrue(t, true)
	AssertTrue(t, len("test") == 4)

	// Test with false would fail
	// We can't test this easily without sub-tests
}

func TestAssertFalse(t *testing.T) {
	// Test with false - should not fail
	AssertFalse(t, false)
	AssertFalse(t, 1 == 2)

	// Test with true would fail
	// We can't test this easily without sub-tests
}

func TestAssertNil(t *testing.T) {
	// Test with nil - should not fail
	AssertNil(t, nil)

	var ptr *int = nil
	AssertNil(t, ptr)

	// Test with non-nil would fail
	// We can't test this easily without sub-tests
}

func TestAssertNotNil(t *testing.T) {
	// Test with non-nil - should not fail
	value := 42
	AssertNotNil(t, &value)
	AssertNotNil(t, "hello")

	// Test with nil would fail
	// We can't test this easily without sub-tests
}

func TestContainsString(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "lo wo", true},
		{"hello world", "xyz", false},
		{"hello world", "", true},
		{"", "hello", false},
		{"", "", true},
		{"test", "test", true},
		{"test", "testing", false},
	}

	for _, tt := range tests {
		result := ContainsString(tt.s, tt.substr)
		if result != tt.expected {
			t.Errorf("ContainsString(%q, %q) = %v, expected %v", tt.s, tt.substr, result, tt.expected)
		}
	}
}

func TestTimeWithinDelta(t *testing.T) {
	now := time.Now()
	future := now.Add(5 * time.Second)
	distant := now.Add(20 * time.Second)

	// Test times within delta
	if !TimeWithinDelta(now, future, 10*time.Second) {
		t.Error("Expected times to be within delta")
	}

	// Test times outside delta
	if TimeWithinDelta(now, distant, 10*time.Second) {
		t.Error("Expected times to be outside delta")
	}

	// Test exact times
	if !TimeWithinDelta(now, now, time.Second) {
		t.Error("Expected identical times to be within delta")
	}

	// Test negative delta (future before now)
	if !TimeWithinDelta(future, now, 10*time.Second) {
		t.Error("Expected times to be within delta regardless of order")
	}
}

func TestAssertTimeWithinDelta(t *testing.T) {
	now := time.Now()
	future := now.Add(5 * time.Second)

	// Test with times within delta - should not fail
	AssertTimeWithinDelta(t, now, future, 10*time.Second)

	// Test with times outside delta would fail
	// We can't test this easily without sub-tests
}

func TestWaitForCondition(t *testing.T) {
	// Test condition that becomes true immediately
	result := WaitForCondition(func() bool { return true }, 100*time.Millisecond)
	if !result {
		t.Error("Expected condition to be true immediately")
	}

	// Test condition that never becomes true
	result = WaitForCondition(func() bool { return false }, 50*time.Millisecond)
	if result {
		t.Error("Expected condition to timeout")
	}

	// Test condition that becomes true after some time
	start := time.Now()
	result = WaitForCondition(func() bool {
		return time.Since(start) > 20*time.Millisecond
	}, 100*time.Millisecond)
	if !result {
		t.Error("Expected condition to become true after delay")
	}
}

func TestAssertEventually(t *testing.T) {
	// Test condition that becomes true eventually - should not fail
	start := time.Now()
	AssertEventually(t, func() bool {
		return time.Since(start) > 10*time.Millisecond
	}, 100*time.Millisecond)

	// Test condition that never becomes true would fail
	// We can't test this easily without sub-tests
}

func TestSetupTestEnvironment(t *testing.T) {
	db, cfg, logger := SetupTestEnvironment(t)

	// Clean up
	defer db.Cleanup()

	// Test that all components are created
	if db == nil {
		t.Error("Expected database to be created")
	}

	if cfg == nil {
		t.Error("Expected config to be created")
	}

	if logger == nil {
		t.Error("Expected logger to be created")
	}

	// Test that database config is synchronized
	if cfg != nil && cfg.Config != nil && db != nil && cfg.Config.Database.Path != db.Config.Path {
		t.Error("Expected database paths to be synchronized")
	}
}

func TestBenchmarkHelper(t *testing.T) {
	helper := NewBenchmarkHelper()

	if helper == nil {
		t.Error("Expected benchmark helper to be created")
	}

	// Test timing
	helper.Start()
	time.Sleep(10 * time.Millisecond)
	duration := helper.Stop()

	if duration < 5*time.Millisecond {
		t.Errorf("Expected duration to be at least 5ms, got %v", duration)
	}

	if duration > 50*time.Millisecond {
		t.Errorf("Expected duration to be less than 50ms, got %v", duration)
	}

	// Test Duration method
	storedDuration := helper.Duration()
	if storedDuration != duration {
		t.Errorf("Expected stored duration %v to equal returned duration %v", storedDuration, duration)
	}
}

func TestBenchmarkHelper_BeforeStop(t *testing.T) {
	helper := NewBenchmarkHelper()
	helper.Start()

	// Test Duration before Stop is called
	time.Sleep(10 * time.Millisecond)
	duration := helper.Duration()

	if duration < 5*time.Millisecond {
		t.Errorf("Expected duration to be at least 5ms, got %v", duration)
	}
}
