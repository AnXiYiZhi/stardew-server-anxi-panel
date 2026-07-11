CREATE TABLE IF NOT EXISTS save_identities (
    instance_id TEXT NOT NULL,
    stable_save_id TEXT NOT NULL,
    base_save_id TEXT NOT NULL,
    full_save_id TEXT,
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    PRIMARY KEY (instance_id, stable_save_id),
    FOREIGN KEY (instance_id) REFERENCES instances(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_save_identities_alias
    ON save_identities(instance_id, base_save_id, full_save_id);

CREATE TABLE IF NOT EXISTS player_roster (
    instance_id TEXT NOT NULL,
    stable_save_id TEXT NOT NULL,
    player_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    role TEXT,
    is_host INTEGER NOT NULL DEFAULT 0,
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    last_online_at TEXT,
    location TEXT,
    location_name TEXT,
    location_display_name TEXT,
    tile_x INTEGER,
    tile_y INTEGER,
    pixel_x INTEGER,
    pixel_y INTEGER,
    money INTEGER,
    farm_income INTEGER,
    personal_income INTEGER,
    total_money_earned INTEGER,
    wallet_mode TEXT,
    snapshot_source TEXT,
    snapshot_observed_at TEXT NOT NULL,
    current_status TEXT NOT NULL DEFAULT 'offline' CHECK (current_status IN ('online', 'offline')),
    PRIMARY KEY (instance_id, stable_save_id, player_id),
    FOREIGN KEY (instance_id, stable_save_id)
        REFERENCES save_identities(instance_id, stable_save_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_player_roster_recent
    ON player_roster(instance_id, stable_save_id, last_seen_at DESC);

CREATE INDEX IF NOT EXISTS idx_player_roster_name
    ON player_roster(instance_id, stable_save_id, display_name);

CREATE TABLE IF NOT EXISTS player_events (
    id TEXT PRIMARY KEY,
    instance_id TEXT NOT NULL,
    stable_save_id TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('seen', 'joined', 'left')),
    player_id TEXT NOT NULL,
    player_name TEXT NOT NULL,
    is_host INTEGER NOT NULL DEFAULT 0,
    location TEXT,
    location_name TEXT,
    location_display_name TEXT,
    occurred_at TEXT NOT NULL,
    FOREIGN KEY (instance_id, stable_save_id)
        REFERENCES save_identities(instance_id, stable_save_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_player_events_recent
    ON player_events(instance_id, stable_save_id, occurred_at DESC, id DESC);
