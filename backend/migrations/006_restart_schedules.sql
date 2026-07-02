CREATE TABLE IF NOT EXISTS restart_schedules (
    instance_id TEXT PRIMARY KEY REFERENCES instances(id) ON DELETE CASCADE,
    enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    shutdown_time TEXT NOT NULL,
    startup_time TEXT NOT NULL,
    timezone TEXT NOT NULL,
    warning_minutes_json TEXT NOT NULL DEFAULT '[]',
    backup_before_shutdown INTEGER NOT NULL DEFAULT 1 CHECK (backup_before_shutdown IN (0, 1)),
    skip_if_players_online INTEGER NOT NULL DEFAULT 0 CHECK (skip_if_players_online IN (0, 1)),
    last_shutdown_at TEXT,
    last_startup_at TEXT,
    last_status TEXT,
    last_message TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_restart_schedules_enabled ON restart_schedules(enabled);
