-- Simplified retention policy migration

-- Retention policies table
CREATE TABLE IF NOT EXISTS retention_policies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    priority INTEGER NOT NULL DEFAULT 0,
    time_based_rule TEXT,
    size_based_rule TEXT,
    count_based_rule TEXT,
    event_type_filter TEXT,
    action_filter TEXT,
    execution_schedule TEXT NOT NULL DEFAULT '0 2 * * *',
    last_executed DATETIME,
    next_execution DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Retention policy executions table
CREATE TABLE IF NOT EXISTS retention_policy_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    policy_id INTEGER NOT NULL,
    execution_time DATETIME NOT NULL,
    status TEXT NOT NULL,
    entries_processed INTEGER NOT NULL DEFAULT 0,
    entries_deleted INTEGER NOT NULL DEFAULT 0,
    bytes_freed INTEGER NOT NULL DEFAULT 0,
    duration INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    details TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_retention_policies_enabled ON retention_policies(enabled);
CREATE INDEX IF NOT EXISTS idx_retention_policies_priority ON retention_policies(priority DESC);
CREATE INDEX IF NOT EXISTS idx_retention_executions_policy_id ON retention_policy_executions(policy_id);

-- Insert default policy
INSERT OR IGNORE INTO retention_policies (
    name, description, enabled, priority, time_based_rule, execution_schedule
) VALUES (
    'Default 30-Day Retention',
    'Default policy to retain audit logs for 30 days',
    1, 100,
    '{"max_age": "720h", "grace_period": "24h"}',
    '0 2 * * *'
); 