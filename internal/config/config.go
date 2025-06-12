package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"parental-control/internal/database"

	"gopkg.in/yaml.v3"
)

// Config represents the complete application configuration
type Config struct {
	// Service configuration
	Service ServiceConfig `yaml:"service" json:"service"`

	// Database configuration
	Database database.Config `yaml:"database" json:"database"`

	// Logging configuration
	Logging LoggingConfig `yaml:"logging" json:"logging"`

	// Web interface configuration
	Web WebConfig `yaml:"web" json:"web"`

	// Security configuration
	Security SecurityConfig `yaml:"security" json:"security"`

	// Monitoring configuration
	Monitoring MonitoringConfig `yaml:"monitoring" json:"monitoring"`
}

// ServiceConfig holds service-specific settings
type ServiceConfig struct {
	// PIDFile path for storing process ID
	PIDFile string `yaml:"pid_file" json:"pid_file"`

	// ShutdownTimeout for graceful shutdown
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" json:"shutdown_timeout"`

	// HealthCheckInterval for periodic health checks
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval"`

	// DataDirectory for application data
	DataDirectory string `yaml:"data_directory" json:"data_directory"`

	// ConfigDirectory for configuration files
	ConfigDirectory string `yaml:"config_directory" json:"config_directory"`
}

// LoggingConfig holds logging-specific settings
type LoggingConfig struct {
	// Level sets the logging level (DEBUG, INFO, WARN, ERROR, FATAL)
	Level string `yaml:"level" json:"level"`

	// Format sets the log format (json, text)
	Format string `yaml:"format" json:"format"`

	// Output sets the log output (stdout, stderr, file path)
	Output string `yaml:"output" json:"output"`

	// EnableTimestamp includes timestamps in logs
	EnableTimestamp bool `yaml:"enable_timestamp" json:"enable_timestamp"`

	// EnableCaller includes caller information in logs
	EnableCaller bool `yaml:"enable_caller" json:"enable_caller"`
}

// WebConfig holds web interface settings
type WebConfig struct {
	// Enabled indicates if web interface is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Port for the web interface
	Port int `yaml:"port" json:"port"`

	// Host to bind the web interface to
	Host string `yaml:"host" json:"host"`

	// StaticDir for static web assets
	StaticDir string `yaml:"static_dir" json:"static_dir"`

	// TLSEnabled indicates if HTTPS is enabled
	TLSEnabled bool `yaml:"tls_enabled" json:"tls_enabled"`

	// TLSCertFile path to TLS certificate
	TLSCertFile string `yaml:"tls_cert_file" json:"tls_cert_file"`

	// TLSKeyFile path to TLS private key
	TLSKeyFile string `yaml:"tls_key_file" json:"tls_key_file"`
}

// SecurityConfig holds security-related settings
type SecurityConfig struct {
	// EnableAuth indicates if authentication is required
	EnableAuth bool `yaml:"enable_auth" json:"enable_auth"`

	// AdminPassword for admin access (should be hashed)
	AdminPassword string `yaml:"admin_password" json:"admin_password"`

	// SessionSecret for session management
	SessionSecret string `yaml:"session_secret" json:"session_secret"`

	// SessionTimeout for session expiration
	SessionTimeout time.Duration `yaml:"session_timeout" json:"session_timeout"`

	// MaxFailedAttempts before account lockout
	MaxFailedAttempts int `yaml:"max_failed_attempts" json:"max_failed_attempts"`

	// LockoutDuration for account lockout
	LockoutDuration time.Duration `yaml:"lockout_duration" json:"lockout_duration"`

	// Password configuration
	BcryptCost          int  `yaml:"bcrypt_cost" json:"bcrypt_cost"`
	MinPasswordLength   int  `yaml:"min_password_length" json:"min_password_length"`
	RequireUppercase    bool `yaml:"require_uppercase" json:"require_uppercase"`
	RequireLowercase    bool `yaml:"require_lowercase" json:"require_lowercase"`
	RequireNumbers      bool `yaml:"require_numbers" json:"require_numbers"`
	RequireSpecialChars bool `yaml:"require_special_chars" json:"require_special_chars"`
	PasswordHistorySize int  `yaml:"password_history_size" json:"password_history_size"`
	PasswordExpireDays  int  `yaml:"password_expire_days" json:"password_expire_days"`

	// Rate limiting
	LoginRateLimit int `yaml:"login_rate_limit" json:"login_rate_limit"`

	// Session management
	RememberMeDuration    time.Duration `yaml:"remember_me_duration" json:"remember_me_duration"`
	AllowMultipleSessions bool          `yaml:"allow_multiple_sessions" json:"allow_multiple_sessions"`
	MaxSessions           int           `yaml:"max_sessions" json:"max_sessions"`
}

// MonitoringConfig holds monitoring settings
type MonitoringConfig struct {
	// Enabled indicates if monitoring is enabled
	Enabled bool `yaml:"enabled" json:"enabled"`

	// MetricsPort for metrics endpoint
	MetricsPort int `yaml:"metrics_port" json:"metrics_port"`

	// MetricsPath for metrics endpoint
	MetricsPath string `yaml:"metrics_path" json:"metrics_path"`

	// HealthCheckPath for health check endpoint
	HealthCheckPath string `yaml:"health_check_path" json:"health_check_path"`
}

// Default returns a configuration with sensible defaults
func Default() *Config {
	return &Config{
		Service: ServiceConfig{
			PIDFile:             "./data/parental-control.pid",
			ShutdownTimeout:     30 * time.Second,
			HealthCheckInterval: 30 * time.Second,
			DataDirectory:       "./data",
			ConfigDirectory:     "./config",
		},
		Database: database.DefaultConfig(),
		Logging: LoggingConfig{
			Level:           "INFO",
			Format:          "text",
			Output:          "stdout",
			EnableTimestamp: true,
			EnableCaller:    false,
		},
		Web: WebConfig{
			Enabled:     true,
			Port:        8080,
			Host:        "localhost",
			StaticDir:   "./web/static",
			TLSEnabled:  false,
			TLSCertFile: "",
			TLSKeyFile:  "",
		},
		Security: SecurityConfig{
			EnableAuth:            false, // Disabled by default for easier setup
			AdminPassword:         "",
			SessionSecret:         "",
			SessionTimeout:        24 * time.Hour,
			MaxFailedAttempts:     5,
			LockoutDuration:       15 * time.Minute,
			BcryptCost:            12, // Good balance of security and performance
			MinPasswordLength:     8,
			RequireUppercase:      true,
			RequireLowercase:      true,
			RequireNumbers:        true,
			RequireSpecialChars:   false, // Optional for easier setup
			PasswordHistorySize:   5,
			PasswordExpireDays:    0,                   // No expiration by default
			LoginRateLimit:        10,                  // 10 attempts per minute
			RememberMeDuration:    30 * 24 * time.Hour, // 30 days
			AllowMultipleSessions: false,
			MaxSessions:           1,
		},
		Monitoring: MonitoringConfig{
			Enabled:         true,
			MetricsPort:     9090,
			MetricsPath:     "/metrics",
			HealthCheckPath: "/health",
		},
	}
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(path string) (*Config, error) {
	// Start with defaults
	config := Default()

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return config, fmt.Errorf("configuration file not found: %s", path)
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse configuration file: %w", err)
	}

	// Apply environment variable overrides
	if err := applyEnvironmentOverrides(config); err != nil {
		return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// LoadFromEnvironment loads configuration from environment variables only
func LoadFromEnvironment() (*Config, error) {
	config := Default()

	if err := applyEnvironmentOverrides(config); err != nil {
		return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// applyEnvironmentOverrides applies environment variable overrides to the configuration
func applyEnvironmentOverrides(config *Config) error {
	// Service configuration
	if val := os.Getenv("PC_SERVICE_PID_FILE"); val != "" {
		config.Service.PIDFile = val
	}
	if val := os.Getenv("PC_SERVICE_SHUTDOWN_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.Service.ShutdownTimeout = duration
		}
	}
	if val := os.Getenv("PC_SERVICE_HEALTH_CHECK_INTERVAL"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.Service.HealthCheckInterval = duration
		}
	}
	if val := os.Getenv("PC_SERVICE_DATA_DIRECTORY"); val != "" {
		config.Service.DataDirectory = val
	}
	if val := os.Getenv("PC_SERVICE_CONFIG_DIRECTORY"); val != "" {
		config.Service.ConfigDirectory = val
	}

	// Database configuration
	if val := os.Getenv("PC_DATABASE_PATH"); val != "" {
		config.Database.Path = val
	}
	if val := os.Getenv("PC_DATABASE_MAX_OPEN_CONNS"); val != "" {
		if parsed, err := parseIntFromEnv(val); err == nil {
			config.Database.MaxOpenConns = parsed
		}
	}
	if val := os.Getenv("PC_DATABASE_MAX_IDLE_CONNS"); val != "" {
		if parsed, err := parseIntFromEnv(val); err == nil {
			config.Database.MaxIdleConns = parsed
		}
	}
	if val := os.Getenv("PC_DATABASE_ENABLE_WAL"); val != "" {
		config.Database.EnableWAL = strings.ToLower(val) == "true"
	}

	// Logging configuration
	if val := os.Getenv("PC_LOGGING_LEVEL"); val != "" {
		config.Logging.Level = val
	}
	if val := os.Getenv("PC_LOGGING_FORMAT"); val != "" {
		config.Logging.Format = val
	}
	if val := os.Getenv("PC_LOGGING_OUTPUT"); val != "" {
		config.Logging.Output = val
	}
	if val := os.Getenv("PC_LOGGING_ENABLE_TIMESTAMP"); val != "" {
		config.Logging.EnableTimestamp = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("PC_LOGGING_ENABLE_CALLER"); val != "" {
		config.Logging.EnableCaller = strings.ToLower(val) == "true"
	}

	// Web configuration
	if val := os.Getenv("PC_WEB_ENABLED"); val != "" {
		config.Web.Enabled = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("PC_WEB_PORT"); val != "" {
		if parsed, err := parseIntFromEnv(val); err == nil {
			config.Web.Port = parsed
		}
	}
	if val := os.Getenv("PC_WEB_HOST"); val != "" {
		config.Web.Host = val
	}
	if val := os.Getenv("PC_WEB_STATIC_DIR"); val != "" {
		config.Web.StaticDir = val
	}
	if val := os.Getenv("PC_WEB_TLS_ENABLED"); val != "" {
		config.Web.TLSEnabled = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("PC_WEB_TLS_CERT_FILE"); val != "" {
		config.Web.TLSCertFile = val
	}
	if val := os.Getenv("PC_WEB_TLS_KEY_FILE"); val != "" {
		config.Web.TLSKeyFile = val
	}

	// Security configuration
	if val := os.Getenv("PC_SECURITY_ENABLE_AUTH"); val != "" {
		config.Security.EnableAuth = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("PC_SECURITY_ADMIN_PASSWORD"); val != "" {
		config.Security.AdminPassword = val
	}
	if val := os.Getenv("PC_SECURITY_SESSION_SECRET"); val != "" {
		config.Security.SessionSecret = val
	}
	if val := os.Getenv("PC_SECURITY_SESSION_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.Security.SessionTimeout = duration
		}
	}
	if val := os.Getenv("PC_SECURITY_BCRYPT_COST"); val != "" {
		if parsed, err := parseIntFromEnv(val); err == nil && parsed >= 4 && parsed <= 31 {
			config.Security.BcryptCost = parsed
		}
	}
	if val := os.Getenv("PC_SECURITY_MIN_PASSWORD_LENGTH"); val != "" {
		if parsed, err := parseIntFromEnv(val); err == nil && parsed > 0 {
			config.Security.MinPasswordLength = parsed
		}
	}
	if val := os.Getenv("PC_SECURITY_REQUIRE_UPPERCASE"); val != "" {
		config.Security.RequireUppercase = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("PC_SECURITY_REQUIRE_LOWERCASE"); val != "" {
		config.Security.RequireLowercase = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("PC_SECURITY_REQUIRE_NUMBERS"); val != "" {
		config.Security.RequireNumbers = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("PC_SECURITY_REQUIRE_SPECIAL_CHARS"); val != "" {
		config.Security.RequireSpecialChars = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("PC_SECURITY_LOGIN_RATE_LIMIT"); val != "" {
		if parsed, err := parseIntFromEnv(val); err == nil && parsed > 0 {
			config.Security.LoginRateLimit = parsed
		}
	}
	if val := os.Getenv("PC_SECURITY_MAX_FAILED_ATTEMPTS"); val != "" {
		if parsed, err := parseIntFromEnv(val); err == nil {
			config.Security.MaxFailedAttempts = parsed
		}
	}
	if val := os.Getenv("PC_SECURITY_LOCKOUT_DURATION"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.Security.LockoutDuration = duration
		}
	}

	// Monitoring configuration
	if val := os.Getenv("PC_MONITORING_ENABLED"); val != "" {
		config.Monitoring.Enabled = strings.ToLower(val) == "true"
	}
	if val := os.Getenv("PC_MONITORING_METRICS_PORT"); val != "" {
		if parsed, err := parseIntFromEnv(val); err == nil {
			config.Monitoring.MetricsPort = parsed
		}
	}
	if val := os.Getenv("PC_MONITORING_METRICS_PATH"); val != "" {
		config.Monitoring.MetricsPath = val
	}
	if val := os.Getenv("PC_MONITORING_HEALTH_CHECK_PATH"); val != "" {
		config.Monitoring.HealthCheckPath = val
	}

	return nil
}

// Validate validates the configuration for correctness
func (c *Config) Validate() error {
	var errors []string

	// Validate service configuration
	if c.Service.PIDFile == "" {
		errors = append(errors, "service.pid_file cannot be empty")
	}
	if c.Service.ShutdownTimeout <= 0 {
		errors = append(errors, "service.shutdown_timeout must be positive")
	}
	if c.Service.HealthCheckInterval <= 0 {
		errors = append(errors, "service.health_check_interval must be positive")
	}
	if c.Service.DataDirectory == "" {
		errors = append(errors, "service.data_directory cannot be empty")
	}
	if c.Service.ConfigDirectory == "" {
		errors = append(errors, "service.config_directory cannot be empty")
	}

	// Validate database configuration
	if c.Database.Path == "" {
		errors = append(errors, "database.path cannot be empty")
	}
	if c.Database.MaxOpenConns <= 0 {
		errors = append(errors, "database.max_open_conns must be positive")
	}
	if c.Database.MaxIdleConns < 0 {
		errors = append(errors, "database.max_idle_conns cannot be negative")
	}
	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		errors = append(errors, "database.max_idle_conns cannot exceed max_open_conns")
	}

	// Validate logging configuration
	validLogLevels := map[string]bool{
		"DEBUG": true, "INFO": true, "WARN": true, "ERROR": true, "FATAL": true,
	}
	if !validLogLevels[strings.ToUpper(c.Logging.Level)] {
		errors = append(errors, "logging.level must be one of: DEBUG, INFO, WARN, ERROR, FATAL")
	}

	validLogFormats := map[string]bool{"json": true, "text": true}
	if !validLogFormats[strings.ToLower(c.Logging.Format)] {
		errors = append(errors, "logging.format must be one of: json, text")
	}

	// Validate web configuration
	if c.Web.Enabled {
		if c.Web.Port <= 0 || c.Web.Port > 65535 {
			errors = append(errors, "web.port must be between 1 and 65535")
		}
		if c.Web.Host == "" {
			errors = append(errors, "web.host cannot be empty when web interface is enabled")
		}
		if c.Web.TLSEnabled {
			if c.Web.TLSCertFile == "" {
				errors = append(errors, "web.tls_cert_file is required when TLS is enabled")
			}
			if c.Web.TLSKeyFile == "" {
				errors = append(errors, "web.tls_key_file is required when TLS is enabled")
			}
		}
	}

	// Validate security configuration
	if c.Security.EnableAuth {
		if c.Security.AdminPassword == "" {
			errors = append(errors, "security.admin_password is required when authentication is enabled")
		}
		if c.Security.SessionSecret == "" {
			errors = append(errors, "security.session_secret is required when authentication is enabled")
		}
		if len(c.Security.SessionSecret) < 32 {
			errors = append(errors, "security.session_secret must be at least 32 characters")
		}
	}
	if c.Security.SessionTimeout <= 0 {
		errors = append(errors, "security.session_timeout must be positive")
	}
	if c.Security.MaxFailedAttempts <= 0 {
		errors = append(errors, "security.max_failed_attempts must be positive")
	}
	if c.Security.LockoutDuration <= 0 {
		errors = append(errors, "security.lockout_duration must be positive")
	}

	// Validate password configuration
	if c.Security.BcryptCost < 4 || c.Security.BcryptCost > 31 {
		errors = append(errors, "security.bcrypt_cost must be between 4 and 31")
	}
	if c.Security.MinPasswordLength < 1 {
		errors = append(errors, "security.min_password_length must be positive")
	}
	if c.Security.PasswordHistorySize < 0 {
		errors = append(errors, "security.password_history_size cannot be negative")
	}
	if c.Security.PasswordExpireDays < 0 {
		errors = append(errors, "security.password_expire_days cannot be negative")
	}
	if c.Security.LoginRateLimit <= 0 {
		errors = append(errors, "security.login_rate_limit must be positive")
	}
	if c.Security.RememberMeDuration <= 0 {
		errors = append(errors, "security.remember_me_duration must be positive")
	}
	if c.Security.MaxSessions <= 0 {
		errors = append(errors, "security.max_sessions must be positive")
	}

	// Validate monitoring configuration
	if c.Monitoring.Enabled {
		if c.Monitoring.MetricsPort <= 0 || c.Monitoring.MetricsPort > 65535 {
			errors = append(errors, "monitoring.metrics_port must be between 1 and 65535")
		}
		if c.Monitoring.MetricsPath == "" {
			errors = append(errors, "monitoring.metrics_path cannot be empty when monitoring is enabled")
		}
		if c.Monitoring.HealthCheckPath == "" {
			errors = append(errors, "monitoring.health_check_path cannot be empty when monitoring is enabled")
		}
		// Check for port conflicts
		if c.Web.Enabled && c.Web.Port == c.Monitoring.MetricsPort {
			errors = append(errors, "web.port and monitoring.metrics_port cannot be the same")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// SaveToFile saves the configuration to a YAML file
func (c *Config) SaveToFile(path string) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	return nil
}

// Clone creates a deep copy of the configuration
func (c *Config) Clone() *Config {
	clone := *c
	return &clone
}

// GetServiceConfig returns the service configuration
func (c *Config) GetServiceConfig() ServiceConfig {
	return c.Service
}

// GetDatabaseConfig returns the database configuration
func (c *Config) GetDatabaseConfig() database.Config {
	return c.Database
}

// GetLoggingConfig returns the logging configuration
func (c *Config) GetLoggingConfig() LoggingConfig {
	return c.Logging
}

// GetWebConfig returns the web configuration
func (c *Config) GetWebConfig() WebConfig {
	return c.Web
}

// GetSecurityConfig returns the security configuration
func (c *Config) GetSecurityConfig() SecurityConfig {
	return c.Security
}

// GetMonitoringConfig returns the monitoring configuration
func (c *Config) GetMonitoringConfig() MonitoringConfig {
	return c.Monitoring
}

// parseIntFromEnv parses an integer from environment variable string
func parseIntFromEnv(val string) (int, error) {
	if val == "" {
		return 0, fmt.Errorf("empty value")
	}

	// Parse as integer, reject if it contains decimal points
	if strings.Contains(val, ".") {
		return 0, fmt.Errorf("invalid integer (contains decimal): %s", val)
	}

	// Simple integer parsing
	var result int
	if _, err := fmt.Sscanf(val, "%d", &result); err != nil {
		return 0, fmt.Errorf("invalid integer: %s", val)
	}

	return result, nil
}

// DefaultSecurityConfig returns default security configuration
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		EnableAuth:            false, // Disabled by default for easier setup
		AdminPassword:         "",
		SessionSecret:         "",
		SessionTimeout:        24 * time.Hour,
		MaxFailedAttempts:     5,
		LockoutDuration:       15 * time.Minute,
		BcryptCost:            12, // Good balance of security and performance
		MinPasswordLength:     8,
		RequireUppercase:      true,
		RequireLowercase:      true,
		RequireNumbers:        true,
		RequireSpecialChars:   false, // Optional for easier setup
		PasswordHistorySize:   5,
		PasswordExpireDays:    0,                   // No expiration by default
		LoginRateLimit:        10,                  // 10 attempts per minute
		RememberMeDuration:    30 * 24 * time.Hour, // 30 days
		AllowMultipleSessions: false,
		MaxSessions:           1,
	}
}
