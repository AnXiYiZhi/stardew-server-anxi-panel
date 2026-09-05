CREATE TABLE instance_deletions (
    instance_id TEXT PRIMARY KEY,
    plan TEXT NOT NULL,
    completed INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- The tombstone outlives the world, prevents ID reuse and excludes late jobs.
CREATE TRIGGER deleting_instance_jobs BEFORE INSERT ON jobs
WHEN (NEW.target_type = 'instance' AND EXISTS (SELECT 1 FROM instance_deletions WHERE instance_id = NEW.target_id))
  OR (NEW.target_type != 'instance' AND EXISTS (SELECT 1 FROM instance_deletions WHERE completed = 0))
BEGIN SELECT RAISE(ABORT, 'instance deletion in progress'); END;
CREATE TRIGGER deleting_instance_updates BEFORE UPDATE ON instances
WHEN EXISTS (SELECT 1 FROM instance_deletions WHERE instance_id = OLD.id)
BEGIN SELECT RAISE(ABORT, 'instance deletion in progress'); END;
CREATE TRIGGER deleted_instance_reuse BEFORE INSERT ON instances
WHEN EXISTS (SELECT 1 FROM instance_deletions WHERE instance_id = NEW.id)
BEGIN SELECT RAISE(ABORT, 'instance was deleted'); END;
