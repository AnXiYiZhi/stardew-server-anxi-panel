CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'canceled')),
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    started_at TEXT,
    finished_at TEXT,
    error_message TEXT,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_created_by ON jobs(created_by);
CREATE INDEX IF NOT EXISTS idx_jobs_target ON jobs(target_type, target_id);

CREATE TABLE IF NOT EXISTS job_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id TEXT NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    level TEXT NOT NULL CHECK (level IN ('info', 'warn', 'error', 'debug')),
    message TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    sequence INTEGER NOT NULL,
    UNIQUE (job_id, sequence)
);

CREATE INDEX IF NOT EXISTS idx_job_logs_job_sequence ON job_logs(job_id, sequence);
CREATE INDEX IF NOT EXISTS idx_job_logs_job_created_at ON job_logs(job_id, created_at);

CREATE TABLE IF NOT EXISTS instance_state (
    instance_id TEXT PRIMARY KEY,
    driver_id TEXT NOT NULL,
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
    last_job_id TEXT REFERENCES jobs(id) ON DELETE SET NULL,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_by INTEGER REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_instance_state_driver ON instance_state(driver_id);
