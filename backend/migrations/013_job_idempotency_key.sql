ALTER TABLE jobs ADD COLUMN idempotency_key TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_idempotency_key
ON jobs(type, target_type, target_id, idempotency_key)
WHERE idempotency_key IS NOT NULL AND idempotency_key <> '';
