CREATE TABLE IF NOT EXISTS control_commands (
    command_id TEXT PRIMARY KEY,
    instance_id TEXT NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    command_type TEXT NOT NULL,
    target_type TEXT,
    target_id TEXT,
    target_label TEXT,
    actor_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    actor_username TEXT,
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'dispatched', 'failed', 'expired', 'unknown')),
    result_supported INTEGER NOT NULL DEFAULT 1 CHECK (result_supported IN (0, 1)),
    error_code TEXT,
    result_message TEXT,
    result_details_json TEXT NOT NULL DEFAULT '{}',
    submitted_at TEXT NOT NULL,
    completed_at TEXT,
    updated_at TEXT NOT NULL,
    imported_at TEXT,
    final_audit_written INTEGER NOT NULL DEFAULT 0 CHECK (final_audit_written IN (0, 1))
);

CREATE INDEX IF NOT EXISTS idx_control_commands_instance_recent
    ON control_commands(instance_id, submitted_at DESC, command_id DESC);
CREATE INDEX IF NOT EXISTS idx_control_commands_status
    ON control_commands(instance_id, status, updated_at);
CREATE INDEX IF NOT EXISTS idx_control_commands_actor
    ON control_commands(actor_user_id, submitted_at DESC);
