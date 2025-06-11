-- Initial schema for Parental Control Application
-- Version: 001
-- Description: Create tables for rules, lists, configurations, and audit logs

-- Enable foreign key constraints
PRAGMA foreign_keys = ON;

-- Configuration table for application settings
CREATE TABLE IF NOT EXISTS config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Lists table for whitelists and blacklists
CREATE TABLE IF NOT EXISTS lists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL CHECK (type IN ('whitelist', 'blacklist')),
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- List entries for executables and URLs
CREATE TABLE IF NOT EXISTS list_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    list_id INTEGER NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    entry_type TEXT NOT NULL CHECK (entry_type IN ('executable', 'url')),
    pattern TEXT NOT NULL,
    pattern_type TEXT NOT NULL CHECK (pattern_type IN ('exact', 'wildcard', 'domain')),
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Time window rules
CREATE TABLE IF NOT EXISTS time_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    list_id INTEGER NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    rule_type TEXT NOT NULL CHECK (rule_type IN ('allow_during', 'block_during')),
    days_of_week TEXT NOT NULL, -- JSON array of day numbers (0=Sunday, 1=Monday, etc.)
    start_time TEXT NOT NULL,   -- HH:MM format
    end_time TEXT NOT NULL,     -- HH:MM format
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Quota rules for duration-based limits
CREATE TABLE IF NOT EXISTS quota_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    list_id INTEGER NOT NULL REFERENCES lists(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    quota_type TEXT NOT NULL CHECK (quota_type IN ('daily', 'weekly', 'monthly')),
    limit_seconds INTEGER NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Quota usage tracking
CREATE TABLE IF NOT EXISTS quota_usage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    quota_rule_id INTEGER NOT NULL REFERENCES quota_rules(id) ON DELETE CASCADE,
    period_start DATETIME NOT NULL,
    period_end DATETIME NOT NULL,
    used_seconds INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(quota_rule_id, period_start)
);

-- Audit log for tracking all actions
CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    event_type TEXT NOT NULL,
    target_type TEXT NOT NULL, -- 'executable' or 'url'
    target_value TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('allow', 'block')),
    rule_type TEXT, -- which type of rule triggered (time, quota, list)
    rule_id INTEGER, -- ID of the specific rule that triggered
    details TEXT, -- JSON object with additional details
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Schema version tracking
CREATE TABLE IF NOT EXISTS schema_versions (
    version INTEGER PRIMARY KEY,
    applied_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);

-- Insert initial schema version
INSERT OR IGNORE INTO schema_versions (version, description) 
VALUES (1, 'Initial schema with lists, rules, quotas, and audit logging');

-- Insert default configuration values
INSERT OR IGNORE INTO config (key, value, description) VALUES
    ('admin_password_hash', '', 'Bcrypt hash of admin password'),
    ('server_port', '8080', 'HTTP server port'),
    ('server_bind_address', '0.0.0.0', 'Server bind address'),
    ('audit_retention_days', '30', 'Number of days to retain audit logs'),
    ('enable_https', 'false', 'Enable HTTPS with self-signed certificate'),
    ('log_level', 'INFO', 'Application log level');

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_list_entries_list_id ON list_entries(list_id);
CREATE INDEX IF NOT EXISTS idx_list_entries_type ON list_entries(entry_type);
CREATE INDEX IF NOT EXISTS idx_list_entries_pattern ON list_entries(pattern);

CREATE INDEX IF NOT EXISTS idx_time_rules_list_id ON time_rules(list_id);
CREATE INDEX IF NOT EXISTS idx_time_rules_enabled ON time_rules(enabled);

CREATE INDEX IF NOT EXISTS idx_quota_rules_list_id ON quota_rules(list_id);
CREATE INDEX IF NOT EXISTS idx_quota_rules_enabled ON quota_rules(enabled);

CREATE INDEX IF NOT EXISTS idx_quota_usage_rule_period ON quota_usage(quota_rule_id, period_start);

CREATE INDEX IF NOT EXISTS idx_audit_log_timestamp ON audit_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_log_event_type ON audit_log(event_type);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON audit_log(action);
CREATE INDEX IF NOT EXISTS idx_audit_log_target ON audit_log(target_type, target_value);

-- Create triggers to update updated_at timestamps
CREATE TRIGGER IF NOT EXISTS update_config_timestamp 
    AFTER UPDATE ON config
    BEGIN
        UPDATE config SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_lists_timestamp 
    AFTER UPDATE ON lists
    BEGIN
        UPDATE lists SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_list_entries_timestamp 
    AFTER UPDATE ON list_entries
    BEGIN
        UPDATE list_entries SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_time_rules_timestamp 
    AFTER UPDATE ON time_rules
    BEGIN
        UPDATE time_rules SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_quota_rules_timestamp 
    AFTER UPDATE ON quota_rules
    BEGIN
        UPDATE quota_rules SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
    END;

CREATE TRIGGER IF NOT EXISTS update_quota_usage_timestamp 
    AFTER UPDATE ON quota_usage
    BEGIN
        UPDATE quota_usage SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
    END; 