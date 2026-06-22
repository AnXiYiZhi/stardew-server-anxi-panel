CREATE TABLE IF NOT EXISTS panel_metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

INSERT INTO panel_metadata (key, value)
VALUES ('schema_initialized', 'true')
ON CONFLICT(key) DO NOTHING;
