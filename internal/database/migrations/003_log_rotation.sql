-- Migration 003: Log Rotation Policies and Executions
-- This migration creates tables for managing log rotation and archival

-- Log rotation policies table
CREATE TABLE IF NOT EXISTS log_rotation_policies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    priority INTEGER NOT NULL DEFAULT 0,
    
    -- Rotation rules (JSON fields)
    size_based_rotation TEXT, -- JSON: SizeBasedRotation
    time_based_rotation TEXT, -- JSON: TimeBasedRotation
    
    -- Archival settings (JSON)
    archival_policy TEXT, -- JSON: ArchivalPolicy
    
    -- Target configuration (JSON arrays)
    target_log_files TEXT, -- JSON: []string
    target_log_types TEXT, -- JSON: []string
    
    -- Emergency settings (JSON)
    emergency_config TEXT, -- JSON: EmergencyCleanupConfig
    
    -- Execution settings
    execution_schedule TEXT NOT NULL DEFAULT '0 2 * * *', -- Daily at 2 AM
    last_executed DATETIME,
    next_execution DATETIME,
    
    -- Metadata
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Log rotation executions table
CREATE TABLE IF NOT EXISTS log_rotation_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    policy_id INTEGER NOT NULL,
    execution_time DATETIME NOT NULL,
    status TEXT NOT NULL DEFAULT 'running', -- running, completed, failed
    trigger_reason TEXT NOT NULL, -- scheduled, size_limit, time_limit, emergency, manual
    
    -- Results
    files_rotated INTEGER NOT NULL DEFAULT 0,
    files_archived INTEGER NOT NULL DEFAULT 0,
    files_deleted INTEGER NOT NULL DEFAULT 0,
    bytes_compressed INTEGER NOT NULL DEFAULT 0,
    bytes_freed INTEGER NOT NULL DEFAULT 0,
    compression_ratio REAL NOT NULL DEFAULT 0.0,
    
    -- Performance
    duration_ms INTEGER NOT NULL DEFAULT 0, -- Duration in milliseconds
    error_message TEXT,
    details TEXT, -- JSON details
    
    -- Metadata
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (policy_id) REFERENCES log_rotation_policies(id) ON DELETE CASCADE
);

-- Indexes for performance

-- Rotation policies indexes
CREATE INDEX IF NOT EXISTS idx_log_rotation_policies_enabled ON log_rotation_policies(enabled);
CREATE INDEX IF NOT EXISTS idx_log_rotation_policies_priority ON log_rotation_policies(priority DESC);
CREATE INDEX IF NOT EXISTS idx_log_rotation_policies_next_execution ON log_rotation_policies(next_execution);

-- Rotation executions indexes
CREATE INDEX IF NOT EXISTS idx_log_rotation_executions_policy_id ON log_rotation_executions(policy_id);
CREATE INDEX IF NOT EXISTS idx_log_rotation_executions_execution_time ON log_rotation_executions(execution_time DESC);
CREATE INDEX IF NOT EXISTS idx_log_rotation_executions_status ON log_rotation_executions(status);
CREATE INDEX IF NOT EXISTS idx_log_rotation_executions_trigger ON log_rotation_executions(trigger_reason);
CREATE INDEX IF NOT EXISTS idx_log_rotation_executions_policy_time ON log_rotation_executions(policy_id, execution_time DESC);

-- Insert default log rotation policy
INSERT OR REPLACE INTO log_rotation_policies (
    id,
    name,
    description,
    enabled,
    priority,
    size_based_rotation,
    time_based_rotation,
    archival_policy,
    target_log_files,
    execution_schedule,
    next_execution
) VALUES (
    1,
    'Default Log Rotation',
    'Default log rotation policy for system logs with daily rotation and 7-day retention',
    1,
    100,
    '{"max_file_size": 104857600, "max_total_size": 1073741824, "rotation_threshold": 0.8}',
    '{"rotation_interval": "24h0m0s", "retain_duration": "168h0m0s", "archive_older_than": "72h0m0s"}',
    '{"enable_compression": true, "compression_format": "gzip", "archive_location": "data/archives", "max_archive_size": 2147483648, "archive_retention": "720h0m0s", "compression_level": 6, "encrypt_archives": false}',
    '["*.log", "data/*.db-wal", "logs/*"]',
    '0 2 * * *',
    datetime('now', '+1 day')
);

-- Insert emergency cleanup policy
INSERT OR REPLACE INTO log_rotation_policies (
    id,
    name,
    description,
    enabled,
    priority,
    size_based_rotation,
    emergency_config,
    target_log_files,
    execution_schedule,
    next_execution
) VALUES (
    2,
    'Emergency Disk Space Protection',
    'Emergency policy to prevent disk space exhaustion',
    1,
    200,
    '{"max_file_size": 52428800, "max_total_size": 536870912, "rotation_threshold": 0.9}',
    '{"disk_space_threshold": 0.85, "emergency_actions": [{"priority": 1, "action_type": "delete_oldest", "target_files": ["*.log.*", "data/*.db-wal"], "max_delete": 100, "description": "Delete oldest rotated log files"}], "monitoring_interval": "5m0s", "alert_thresholds": [{"disk_usage_percent": 0.8, "alert_level": "warning", "notification_channels": ["system"]}, {"disk_usage_percent": 0.9, "alert_level": "critical", "notification_channels": ["system"]}]}',
    '["*.log.*", "data/*.db-wal", "logs/*"]',
    '*/5 * * * *', -- Every 5 minutes
    datetime('now', '+5 minutes')
);

-- Update schema version
INSERT OR IGNORE INTO schema_versions (version, description) 
VALUES (3, 'Add log rotation and archival management tables'); 