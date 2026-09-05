CREATE TABLE IF NOT EXISTS instance_id_sequences (
    game_id TEXT PRIMARY KEY,
    next_value INTEGER NOT NULL CHECK (next_value >= 2),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
