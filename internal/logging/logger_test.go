package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{FATAL, "FATAL"},
		{LogLevel(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("LogLevel.String() = %v, want %v", got, tt.expected)
		}
	}
}

func TestNew(t *testing.T) {
	var buf bytes.Buffer
	config := Config{
		Level:  DEBUG,
		Output: &buf,
	}

	logger := New(config)
	if logger == nil {
		t.Fatal("New() returned nil")
	}

	if logger.level != DEBUG {
		t.Errorf("Expected level DEBUG, got %v", logger.level)
	}
}

func TestNewDefault(t *testing.T) {
	logger := NewDefault()
	if logger == nil {
		t.Fatal("NewDefault() returned nil")
	}

	if logger.level != INFO {
		t.Errorf("Expected default level INFO, got %v", logger.level)
	}
}

func TestLogger_SetLevel(t *testing.T) {
	logger := NewDefault()
	logger.SetLevel(ERROR)

	if logger.level != ERROR {
		t.Errorf("Expected level ERROR, got %v", logger.level)
	}
}

func TestLogger_LogLevels(t *testing.T) {
	var buf bytes.Buffer
	config := Config{
		Level:  DEBUG,
		Output: &buf,
	}
	logger := New(config)

	// Test each log level
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 4 {
		t.Errorf("Expected 4 log lines, got %d", len(lines))
	}

	// Check that each line contains the expected level
	expectedLevels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	for i, expectedLevel := range expectedLevels {
		if !strings.Contains(lines[i], "["+expectedLevel+"]") {
			t.Errorf("Line %d should contain [%s], got: %s", i, expectedLevel, lines[i])
		}
	}
}

func TestLogger_LogFiltering(t *testing.T) {
	var buf bytes.Buffer
	config := Config{
		Level:  WARN, // Only WARN and above should be logged
		Output: &buf,
	}
	logger := New(config)

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should only have WARN and ERROR messages
	if len(lines) != 2 {
		t.Errorf("Expected 2 log lines, got %d", len(lines))
	}

	if !strings.Contains(lines[0], "[WARN]") {
		t.Errorf("First line should be WARN, got: %s", lines[0])
	}

	if !strings.Contains(lines[1], "[ERROR]") {
		t.Errorf("Second line should be ERROR, got: %s", lines[1])
	}
}

func TestFields(t *testing.T) {
	var buf bytes.Buffer
	config := Config{
		Level:  INFO,
		Output: &buf,
	}
	logger := New(config)

	logger.Info("test message",
		String("key1", "value1"),
		Int("key2", 42),
		Bool("key3", true),
	)

	output := buf.String()

	// Check that fields are included in the output
	expectedFields := []string{`key1="value1"`, "key2=42", "key3=true"}
	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("Output should contain field %s, got: %s", field, output)
		}
	}
}

func TestField_String(t *testing.T) {
	tests := []struct {
		field    Field
		expected string
	}{
		{String("key", "value"), `key="value"`},
		{Int("num", 42), "num=42"},
		{Bool("flag", true), "flag=true"},
		{Bool("flag", false), "flag=false"},
	}

	for _, tt := range tests {
		if got := tt.field.String(); got != tt.expected {
			t.Errorf("Field.String() = %v, want %v", got, tt.expected)
		}
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		value    interface{}
		expected string
	}{
		{"string", `"string"`},
		{42, "42"},
		{true, "true"},
		{false, "false"},
		{3.14, "3.14"},
	}

	for _, tt := range tests {
		if got := formatValue(tt.value); got != tt.expected {
			t.Errorf("formatValue(%v) = %v, want %v", tt.value, got, tt.expected)
		}
	}
}

func TestGlobalLogger(t *testing.T) {
	var buf bytes.Buffer
	config := Config{
		Level:  INFO,
		Output: &buf,
	}
	logger := New(config)

	// Set global logger
	SetGlobalLogger(logger)

	// Test global functions
	Info("global info message")

	output := buf.String()
	if !strings.Contains(output, "[INFO]") || !strings.Contains(output, "global info message") {
		t.Errorf("Global logger should log message, got: %s", output)
	}

	// Verify we can get the global logger
	globalLogger := GetGlobalLogger()
	if globalLogger != logger {
		t.Error("GetGlobalLogger() should return the same logger instance")
	}
} 