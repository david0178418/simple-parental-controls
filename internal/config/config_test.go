package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	config := Default()

	if config == nil {
		t.Fatal("Default() returned nil")
	}

	// Test service defaults
	if config.Service.PIDFile != "./data/parental-control.pid" {
		t.Errorf("Expected default PID file './data/parental-control.pid', got %s", config.Service.PIDFile)
	}
	if config.Service.ShutdownTimeout != 30*time.Second {
		t.Errorf("Expected default shutdown timeout 30s, got %v", config.Service.ShutdownTimeout)
	}

	// Test logging defaults
	if config.Logging.Level != "INFO" {
		t.Errorf("Expected default log level 'INFO', got %s", config.Logging.Level)
	}

	// Test web defaults
	if config.Web.Port != 8080 {
		t.Errorf("Expected default web port 8080, got %d", config.Web.Port)
	}

	// Test validation passes with defaults
	if err := config.Validate(); err != nil {
		t.Errorf("Default configuration should be valid, got: %v", err)
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Config)
		expectError bool
		errorText   string
	}{
		{
			name: "valid default config",
			modify: func(c *Config) {
				// Set required auth fields to make config valid
				c.Security.AdminPassword = "password123"
				c.Security.SessionSecret = "this-is-a-very-long-secret-key-that-is-at-least-32-chars"
			},
			expectError: false,
		},
		{
			name: "empty PID file",
			modify: func(c *Config) {
				c.Service.PIDFile = ""
			},
			expectError: true,
			errorText:   "service.pid_file cannot be empty",
		},
		{
			name: "invalid shutdown timeout",
			modify: func(c *Config) {
				c.Service.ShutdownTimeout = -1 * time.Second
			},
			expectError: true,
			errorText:   "service.shutdown_timeout must be positive",
		},
		{
			name: "invalid log level",
			modify: func(c *Config) {
				c.Logging.Level = "INVALID"
			},
			expectError: true,
			errorText:   "logging.level must be one of",
		},
		{
			name: "invalid web port",
			modify: func(c *Config) {
				c.Web.Port = -1
			},
			expectError: true,
			errorText:   "web.port must be between 1 and 65535",
		},
		{
			name: "TLS enabled without cert file",
			modify: func(c *Config) {
				c.Web.TLSEnabled = true
				c.Web.TLSCertFile = ""
			},
			expectError: true,
			errorText:   "web.tls_cert_file is required when TLS is enabled",
		},
		{
			name: "auth enabled without password",
			modify: func(c *Config) {
				c.Security.EnableAuth = true
				c.Security.AdminPassword = ""
			},
			expectError: true,
			errorText:   "security.admin_password is required when authentication is enabled",
		},
		{
			name: "session secret too short",
			modify: func(c *Config) {
				c.Security.EnableAuth = true
				c.Security.AdminPassword = "password"
				c.Security.SessionSecret = "short"
			},
			expectError: true,
			errorText:   "security.session_secret must be at least 32 characters",
		},
		{
			name: "port conflict",
			modify: func(c *Config) {
				c.Web.Port = 9090
				c.Monitoring.MetricsPort = 9090
			},
			expectError: true,
			errorText:   "web.port and monitoring.metrics_port cannot be the same",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Default()
			tt.modify(config)

			err := config.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected validation error, but got none")
				} else if !strings.Contains(err.Error(), tt.errorText) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorText, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, got: %v", err)
				}
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Create a test config file
	configContent := `
service:
  pid_file: ./test.pid
  shutdown_timeout: 60s
  data_directory: ./test-data

database:
  path: ./test.db
  max_open_conns: 10

logging:
  level: DEBUG
  format: json

web:
  enabled: false
  port: 9090

security:
  enable_auth: false

monitoring:
  enabled: false
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	config, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load config from file: %v", err)
	}

	// Verify loaded values
	if config.Service.PIDFile != "./test.pid" {
		t.Errorf("Expected PID file './test.pid', got %s", config.Service.PIDFile)
	}
	if config.Service.ShutdownTimeout != 60*time.Second {
		t.Errorf("Expected shutdown timeout 60s, got %v", config.Service.ShutdownTimeout)
	}
	if config.Logging.Level != "DEBUG" {
		t.Errorf("Expected log level 'DEBUG', got %s", config.Logging.Level)
	}
	if config.Web.Enabled != false {
		t.Errorf("Expected web enabled false, got %v", config.Web.Enabled)
	}
}

func TestLoadFromNonExistentFile(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error when loading non-existent file")
	}
	if !strings.Contains(err.Error(), "configuration file not found") {
		t.Errorf("Expected 'file not found' error, got: %v", err)
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	// Set test environment variables
	testEnvVars := map[string]string{
		"PC_SERVICE_PID_FILE":                "./env-test.pid",
		"PC_SERVICE_SHUTDOWN_TIMEOUT":        "45s",
		"PC_DATABASE_PATH":                   "./env-test.db",
		"PC_DATABASE_MAX_OPEN_CONNS":         "15",
		"PC_LOGGING_LEVEL":                   "WARN",
		"PC_LOGGING_FORMAT":                  "json",
		"PC_WEB_ENABLED":                     "false",
		"PC_WEB_PORT":                        "9000",
		"PC_SECURITY_ENABLE_AUTH":            "false",
		"PC_MONITORING_ENABLED":              "false",
	}

	// Set environment variables
	for key, value := range testEnvVars {
		t.Setenv(key, value)
	}

	config, err := LoadFromEnvironment()
	if err != nil {
		t.Fatalf("Failed to load config from environment: %v", err)
	}

	// Verify environment overrides
	if config.Service.PIDFile != "./env-test.pid" {
		t.Errorf("Expected PID file './env-test.pid', got %s", config.Service.PIDFile)
	}
	if config.Service.ShutdownTimeout != 45*time.Second {
		t.Errorf("Expected shutdown timeout 45s, got %v", config.Service.ShutdownTimeout)
	}
	if config.Database.Path != "./env-test.db" {
		t.Errorf("Expected database path './env-test.db', got %s", config.Database.Path)
	}
	if config.Database.MaxOpenConns != 15 {
		t.Errorf("Expected max open conns 15, got %d", config.Database.MaxOpenConns)
	}
	if config.Logging.Level != "WARN" {
		t.Errorf("Expected log level 'WARN', got %s", config.Logging.Level)
	}
	if config.Web.Enabled != false {
		t.Errorf("Expected web enabled false, got %v", config.Web.Enabled)
	}
	if config.Web.Port != 9000 {
		t.Errorf("Expected web port 9000, got %d", config.Web.Port)
	}
}

func TestSaveToFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "subdir", "config.yaml")

	config := Default()
	config.Service.PIDFile = "./save-test.pid"
	config.Security.EnableAuth = false // Make config valid
	config.Monitoring.Enabled = false

	if err := config.SaveToFile(configPath); err != nil {
		t.Fatalf("Failed to save config to file: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("Config file was not created: %v", err)
	}

	// Load the saved config and verify
	loadedConfig, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loadedConfig.Service.PIDFile != "./save-test.pid" {
		t.Errorf("Expected saved PID file './save-test.pid', got %s", loadedConfig.Service.PIDFile)
	}
}

func TestClone(t *testing.T) {
	original := Default()
	clone := original.Clone()

	// Verify clone is separate instance
	if original == clone {
		t.Error("Clone should return a different instance")
	}

	// Verify values are copied
	if clone.Service.PIDFile != original.Service.PIDFile {
		t.Error("Clone should have same values as original")
	}

	// Verify modifying clone doesn't affect original
	clone.Service.PIDFile = "./modified.pid"
	if original.Service.PIDFile == "./modified.pid" {
		t.Error("Modifying clone should not affect original")
	}
}

func TestGetterMethods(t *testing.T) {
	config := Default()

	serviceConfig := config.GetServiceConfig()
	if serviceConfig.PIDFile != config.Service.PIDFile {
		t.Error("GetServiceConfig should return service configuration")
	}

	dbConfig := config.GetDatabaseConfig()
	if dbConfig.Path != config.Database.Path {
		t.Error("GetDatabaseConfig should return database configuration")
	}

	loggingConfig := config.GetLoggingConfig()
	if loggingConfig.Level != config.Logging.Level {
		t.Error("GetLoggingConfig should return logging configuration")
	}

	webConfig := config.GetWebConfig()
	if webConfig.Port != config.Web.Port {
		t.Error("GetWebConfig should return web configuration")
	}

	securityConfig := config.GetSecurityConfig()
	if securityConfig.EnableAuth != config.Security.EnableAuth {
		t.Error("GetSecurityConfig should return security configuration")
	}

	monitoringConfig := config.GetMonitoringConfig()
	if monitoringConfig.Enabled != config.Monitoring.Enabled {
		t.Error("GetMonitoringConfig should return monitoring configuration")
	}
}

func TestParseIntFromEnv(t *testing.T) {
	tests := []struct {
		input       string
		expected    int
		expectError bool
	}{
		{"123", 123, false},
		{"0", 0, false},
		{"-1", -1, false},
		{"", 0, true},
		{"abc", 0, true},
		{"12.34", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseIntFromEnv(tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for input '%s', but got none", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for input '%s', got: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d for input '%s', got %d", tt.expected, tt.input, result)
				}
			}
		})
	}
}

func TestInvalidYAMLFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid.yaml")

	// Create invalid YAML content
	invalidContent := `
service:
  pid_file: ./test.pid
database:
	# This indentation is invalid in YAML
	  path: ./test.db
`

	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	_, err := LoadFromFile(configPath)
	if err == nil {
		t.Error("Expected error when loading invalid YAML file")
	}
	if !strings.Contains(err.Error(), "failed to parse configuration file") {
		t.Errorf("Expected parse error, got: %v", err)
	}
}

func TestFileAndEnvironmentCombination(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "combo.yaml")

	// Create config file
	configContent := `
service:
  pid_file: ./file.pid
logging:
  level: INFO
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Set environment variable that should override file
	t.Setenv("PC_SERVICE_PID_FILE", "./env-override.pid")
	t.Setenv("PC_LOGGING_LEVEL", "DEBUG")
	t.Setenv("PC_SECURITY_ENABLE_AUTH", "false")
	t.Setenv("PC_MONITORING_ENABLED", "false")

	config, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Environment should override file
	if config.Service.PIDFile != "./env-override.pid" {
		t.Errorf("Expected environment override './env-override.pid', got %s", config.Service.PIDFile)
	}

	// Environment should override file
	if config.Logging.Level != "DEBUG" {
		t.Errorf("Expected environment override 'DEBUG', got %s", config.Logging.Level)
	}
} 