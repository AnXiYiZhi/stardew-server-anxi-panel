UPDATE jobs
SET status = 'failed',
    finished_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    error_message = 'Panel 升级时检测到同一实例存在重复的活动安装任务，旧任务已安全终止。',
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE type = 'stardew_install'
  AND status IN ('queued', 'running')
  AND EXISTS (
      SELECT 1
      FROM jobs AS newer
      WHERE newer.type = jobs.type
        AND newer.target_type = jobs.target_type
        AND newer.target_id = jobs.target_id
        AND newer.status IN ('queued', 'running')
        AND (
            newer.created_at > jobs.created_at
            OR (newer.created_at = jobs.created_at AND newer.id > jobs.id)
        )
  );

CREATE UNIQUE INDEX IF NOT EXISTS idx_jobs_one_active_stardew_install
ON jobs(target_type, target_id)
WHERE type = 'stardew_install' AND status IN ('queued', 'running');
