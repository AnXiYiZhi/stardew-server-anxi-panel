ALTER TABLE users
    ADD COLUMN is_super_admin INTEGER NOT NULL DEFAULT 0 CHECK (is_super_admin IN (0, 1));

UPDATE users
SET is_super_admin = 1
WHERE id = (
    SELECT id
    FROM users
    WHERE role = 'admin'
    ORDER BY id ASC
    LIMIT 1
)
AND NOT EXISTS (
    SELECT 1
    FROM users
    WHERE role = 'admin' AND is_super_admin = 1
);

CREATE INDEX IF NOT EXISTS idx_users_super_admin_active ON users(is_super_admin, is_active);
