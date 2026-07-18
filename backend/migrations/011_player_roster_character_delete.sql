ALTER TABLE player_roster ADD COLUMN character_deleted_at TEXT;
ALTER TABLE player_roster ADD COLUMN character_delete_operation_id TEXT;

CREATE INDEX IF NOT EXISTS idx_player_roster_visible
    ON player_roster(instance_id, stable_save_id, character_deleted_at);
