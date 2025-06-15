-- Retention Policies Migration
-- Version: 002
-- Description: Add retention policy management tables

-- Enable foreign key constraints
PRAGMA foreign_keys = ON;

-- Retention policies table
CREATE TABLE IF NOT EXISTS retention_policies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    priority INTEGER NOT NULL DEFAULT 0,
    
    -- Policy rules (stored as JSON)
    time_based_rule TEXT, -- JSON for TimeBasedRetention
    size_based_rule TEXT, -- JSON for SizeBasedRetention
    count_based_rule TEXT, -- JSON for CountBasedRetention
    
    -- Event filtering (stored as JSON arrays)
    event_type_filter TEXT, -- JSON array of event types
    action_filter TEXT,     -- JSON array of actions
    
    -- Execution settings
    execution_schedule TEXT NOT NULL DEFAULT '0 2 * * *', -- Daily at 2 AM
    last_executed DATETIME,
    next_execution DATETIME,
    
    -- Metadata
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Retention policy executions table
CREATE TABLE IF NOT EXISTS retention_policy_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    policy_id INTEGER NOT NULL REFERENCES retention_policies(id) ON DELETE CASCADE,
    execution_time DATETIME NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    entries_processed INTEGER NOT NULL DEFAULT 0,
    entries_deleted INTEGER NOT NULL DEFAULT 0,
    bytes_freed INTEGER NOT NULL DEFAULT 0,
    duration INTEGER NOT NULL DEFAULT 0, -- Duration in nanoseconds
    error_message TEXT,
    details TEXT, -- JSON object with additional details
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_retention_policies_enabled ON retention_policies(enabled);
CREATE INDEX IF NOT EXISTS idx_retention_policies_priority ON retention_policies(priority DESC);
CREATE INDEX IF NOT EXISTS idx_retention_policies_next_execution ON retention_policies(next_execution);

CREATE INDEX IF NOT EXISTS idx_retention_executions_policy_id ON retention_policy_executions(policy_id);
CREATE INDEX IF NOT EXISTS idx_retention_executions_status ON retention_policy_executions(status);
CREATE INDEX IF NOT EXISTS idx_retention_executions_execution_time ON retention_policy_executions(execution_time);

-- Create triggers to update updated_at timestamps
CREATE TRIGGER IF NOT EXISTS update_retention_policies_timestamp 
    AFTER UPDATE ON retention_policies
    BEGIN
        UPDATE retention_policies SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
    END;

-- Insert default retention policy
INSERT OR IGNORE INTO retention_policies (
    name, 
    description, 
    enabled, 
    priority,
    time_based_rule,
    execution_schedule
) VALUES (
    'Default 30-Day Retention',
    'Default policy to retain audit logs for 30 days',
    1,
    100,
    '{"max_age": "720h", "grace_period": "24h"}', -- 30 days + 1 day grace
    '0 2 * * *' -- Daily at 2 AM
);

-- Update schema version
INSERT OR IGNORE INTO schema_versions (version, description) 
VALUES (2, 'Add retention policy management tables'); 