package database

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"parental-control/internal/logging"

	// SQLite driver
	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps the sql.DB connection with additional functionality
type DB struct {
	conn *sql.DB
	path string
}

// Config holds database configuration
type Config struct {
	// Path to the database file
	Path string
	// Maximum number of open connections
	MaxOpenConns int
	// Maximum number of idle connections
	MaxIdleConns int
	// Connection maximum lifetime
	ConnMaxLifetime time.Duration
	// Enable WAL mode for better concurrency
	EnableWAL bool
	// Timeout for database operations
	Timeout time.Duration
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() Config {
	return Config{
		Path:            "./data/parental-control.db",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		EnableWAL:       true,
		Timeout:         30 * time.Second,
	}
}

// New creates a new database connection with the given configuration
func New(config Config) (*DB, error) {
	// Ensure the directory exists
	dir := filepath.Dir(config.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Build connection string with options
	dsn := config.Path
	if config.EnableWAL {
		dsn += "?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=1000&_foreign_keys=1"
	} else {
		dsn += "?_foreign_keys=1"
	}

	// Open database connection
	conn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	conn.SetMaxOpenConns(config.MaxOpenConns)
	conn.SetMaxIdleConns(config.MaxIdleConns)
	conn.SetConnMaxLifetime(config.ConnMaxLifetime)

	db := &DB{
		conn: conn,
		path: config.Path,
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("database connection test failed: %w", err)
	}

	logging.Info("Database connection established", logging.String("path", config.Path))

	return db, nil
}

// Ping tests the database connection
func (db *DB) Ping() error {
	return db.conn.Ping()
}

// Close closes the database connection
func (db *DB) Close() error {
	if db.conn != nil {
		logging.Info("Closing database connection")
		return db.conn.Close()
	}
	return nil
}

// Connection returns the underlying sql.DB connection
func (db *DB) Connection() *sql.DB {
	return db.conn
}

// Path returns the database file path
func (db *DB) Path() string {
	return db.path
}

// InitializeSchema runs all pending migrations to set up the database schema
func (db *DB) InitializeSchema() error {
	logging.Info("Initializing database schema")

	// Get current schema version
	currentVersion, err := db.getCurrentSchemaVersion()
	if err != nil {
		return fmt.Errorf("failed to get current schema version: %w", err)
	}

	// Apply migrations
	if err := db.applyMigrations(currentVersion); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	logging.Info("Database schema initialization complete")
	return nil
}

// getCurrentSchemaVersion returns the current schema version
func (db *DB) getCurrentSchemaVersion() (int, error) {
	// Check if schema_versions table exists
	var exists bool
	err := db.conn.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM sqlite_master 
		WHERE type='table' AND name='schema_versions'
	`).Scan(&exists)
	
	if err != nil {
		return 0, err
	}

	if !exists {
		return 0, nil // No schema version table means version 0
	}

	// Get the latest version
	var version int
	err = db.conn.QueryRow("SELECT MAX(version) FROM schema_versions").Scan(&version)
	if err != nil {
		return 0, err
	}

	return version, nil
}

// applyMigrations applies all migrations newer than the current version
func (db *DB) applyMigrations(currentVersion int) error {
	// Read migration files
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Apply each migration file
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		
		// Read migration content
		content, err := migrationsFS.ReadFile("migrations/" + filename)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", filename, err)
		}

		logging.Info("Applying migration", logging.String("file", filename))

		// Execute migration in a transaction
		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("failed to start transaction for migration %s: %w", filename, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", filename, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", filename, err)
		}

		logging.Info("Migration applied successfully", logging.String("file", filename))
	}

	return nil
}

// HealthCheck performs a comprehensive health check of the database
func (db *DB) HealthCheck() error {
	// Test basic connectivity
	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	// Test a simple query
	var result int
	if err := db.conn.QueryRow("SELECT 1").Scan(&result); err != nil {
		return fmt.Errorf("test query failed: %w", err)
	}

	// Check that required tables exist
	requiredTables := []string{
		"config", "lists", "list_entries", "time_rules", 
		"quota_rules", "quota_usage", "audit_log", "schema_versions",
	}

	for _, table := range requiredTables {
		var exists bool
		err := db.conn.QueryRow(`
			SELECT COUNT(*) > 0 
			FROM sqlite_master 
			WHERE type='table' AND name=?
		`, table).Scan(&exists)
		
		if err != nil {
			return fmt.Errorf("failed to check table %s: %w", table, err)
		}
		
		if !exists {
			return fmt.Errorf("required table %s does not exist", table)
		}
	}

	return nil
}

// GetStats returns database statistics
func (db *DB) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	
	// Connection pool stats
	dbStats := db.conn.Stats()
	stats["open_connections"] = dbStats.OpenConnections
	stats["in_use"] = dbStats.InUse
	stats["idle"] = dbStats.Idle
	stats["wait_count"] = dbStats.WaitCount
	stats["wait_duration"] = dbStats.WaitDuration.String()
	stats["max_idle_closed"] = dbStats.MaxIdleClosed
	stats["max_lifetime_closed"] = dbStats.MaxLifetimeClosed

	// Database file info
	if info, err := os.Stat(db.path); err == nil {
		stats["file_size"] = info.Size()
		stats["modified_time"] = info.ModTime()
	}

	// Schema version
	if version, err := db.getCurrentSchemaVersion(); err == nil {
		stats["schema_version"] = version
	}

	return stats, nil
} 