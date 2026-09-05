CREATE TABLE instance_provisions (
    instance_id TEXT PRIMARY KEY,
    template_id TEXT NOT NULL,
    token TEXT NOT NULL UNIQUE
);

-- A copy owns both ends until publication or verified rollback. This also
-- fences late background/global jobs and another process using the same DB.
CREATE TRIGGER provisioning_instance_jobs BEFORE INSERT ON jobs
WHEN EXISTS (SELECT 1 FROM instance_provisions
    WHERE NEW.target_type != 'instance'
       OR NEW.target_id IN (instance_id, template_id))
BEGIN SELECT RAISE(ABORT, 'instance provisioning in progress'); END;
