package database

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	if config.Path != "./data/parental-control.db" {
		t.Errorf("Expected default path './data/parental-control.db', got %s", config.Path)
	}
	
	if config.MaxOpenConns != 10 {
		t.Errorf("Expected MaxOpenConns 10, got %d", config.MaxOpenConns)
	}
	
	if config.MaxIdleConns != 5 {
		t.Errorf("Expected MaxIdleConns 5, got %d", config.MaxIdleConns)
	}
	
	if config.ConnMaxLifetime != time.Hour {
		t.Errorf("Expected ConnMaxLifetime 1h, got %v", config.ConnMaxLifetime)
	}
	
	if !config.EnableWAL {
		t.Error("Expected EnableWAL to be true")
	}
}

func TestNew(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	config := Config{
		Path:            dbPath,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 30 * time.Minute,
		EnableWAL:       true,
		Timeout:         10 * time.Second,
	}
	
	db, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()
	
	if db.Path() != dbPath {
		t.Errorf("Expected path %s, got %s", dbPath, db.Path())
	}
	
	// Test connection
	if err := db.Ping(); err != nil {
		t.Errorf("Database ping failed: %v", err)
	}
}

func TestInitializeSchema(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	config := Config{
		Path:         dbPath,
		MaxOpenConns: 5,
		MaxIdleConns: 2,
		EnableWAL:    true,
	}
	
	db, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()
	
	// Initialize schema
	if err := db.InitializeSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}
	
	// Verify schema version
	version, err := db.getCurrentSchemaVersion()
	if err != nil {
		t.Errorf("Failed to get schema version: %v", err)
	}
	
	if version != 1 {
		t.Errorf("Expected schema version 1, got %d", version)
	}
}

func TestHealthCheck(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	config := DefaultConfig()
	config.Path = dbPath
	
	db, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()
	
	// Initialize schema first
	if err := db.InitializeSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}
	
	// Run health check
	if err := db.HealthCheck(); err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}

func TestHealthCheckMissingTables(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	config := DefaultConfig()
	config.Path = dbPath
	
	db, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()
	
	// Don't initialize schema - should fail health check
	err = db.HealthCheck()
	if err == nil {
		t.Error("Expected health check to fail with missing tables")
	}
}

func TestGetStats(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	config := DefaultConfig()
	config.Path = dbPath
	
	db, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()
	
	// Initialize schema
	if err := db.InitializeSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}
	
	stats, err := db.GetStats()
	if err != nil {
		t.Errorf("Failed to get stats: %v", err)
	}
	
	// Check that expected stats are present
	expectedKeys := []string{
		"open_connections", "in_use", "idle", "wait_count",
		"file_size", "schema_version",
	}
	
	for _, key := range expectedKeys {
		if _, exists := stats[key]; !exists {
			t.Errorf("Expected stat key %s not found", key)
		}
	}
	
	// Verify schema version
	if stats["schema_version"] != 1 {
		t.Errorf("Expected schema version 1, got %v", stats["schema_version"])
	}
}

func TestConfigTableInitialization(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	config := DefaultConfig()
	config.Path = dbPath
	
	db, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()
	
	// Initialize schema
	if err := db.InitializeSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}
	
	// Check that default config values were inserted
	var count int
	err = db.Connection().QueryRow("SELECT COUNT(*) FROM config").Scan(&count)
	if err != nil {
		t.Errorf("Failed to query config table: %v", err)
	}
	
	if count == 0 {
		t.Error("Expected default config values to be inserted")
	}
	
	// Check for specific config values
	var value string
	err = db.Connection().QueryRow("SELECT value FROM config WHERE key = 'server_port'").Scan(&value)
	if err != nil {
		t.Errorf("Failed to query server_port config: %v", err)
	}
	
	if value != "8080" {
		t.Errorf("Expected server_port to be '8080', got %s", value)
	}
}

func TestCloseNilConnection(t *testing.T) {
	db := &DB{conn: nil}
	err := db.Close()
	if err != nil {
		t.Errorf("Close with nil connection should not return error, got: %v", err)
	}
}

func TestDatabaseFileCreation(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "subdir", "test.db")
	
	config := DefaultConfig()
	config.Path = dbPath
	
	db, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()
	
	// Check that the directory was created
	if _, err := os.Stat(filepath.Dir(dbPath)); err != nil {
		t.Errorf("Database directory was not created: %v", err)
	}
	
	// Initialize schema to create the file
	if err := db.InitializeSchema(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}
	
	// Check that the database file was created
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("Database file was not created: %v", err)
	}
} 