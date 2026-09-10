CREATE TABLE farmhand_delete_intents (
    instance_id TEXT PRIMARY KEY REFERENCES instances(id) ON DELETE CASCADE,
    operation_id TEXT NOT NULL UNIQUE,
    mode TEXT NOT NULL CHECK (mode IN ('wait', 'maintenance_now')),
    status TEXT NOT NULL CHECK (status IN ('waiting', 'launching', 'countdown', 'active', 'recovery_required', 'completed', 'canceled', 'expired', 'failed')),
    player_id TEXT NOT NULL,
    expected_name TEXT NOT NULL DEFAULT '',
    expected_save_id TEXT NOT NULL,
    created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    job_id TEXT REFERENCES jobs(id) ON DELETE SET NULL,
    expires_at TEXT NOT NULL,
    backup_name TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX idx_farmhand_delete_intents_status_expiry
ON farmhand_delete_intents(status, expires_at);
