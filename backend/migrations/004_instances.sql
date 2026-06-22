CREATE TABLE IF NOT EXISTS instances (
    id TEXT PRIMARY KEY,
    driver_id TEXT NOT NULL,
    name TEXT NOT NULL,
    data_dir TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN (
        'uninitialized',
        'admin_created',
        'junimo_scaffolded',
        'credentials_required',
        'steam_auth_running',
        'steam_auth_failed',
        'steam_auth_done',
        'game_installed',
        'save_required',
        'ready_to_start',
        'starting',
        'running',
        'stopped',
        'error'
    )),
    state_message TEXT,
    driver_phase TEXT NOT NULL DEFAULT 'empty',
    driver_payload TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_instances_driver_id ON instances(driver_id);
CREATE INDEX IF NOT EXISTS idx_instances_state ON instances(state);
