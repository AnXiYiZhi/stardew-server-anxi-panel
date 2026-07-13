package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BackupTo creates a consistent SQLite snapshot while the WAL database remains
// online. It deliberately uses VACUUM INTO instead of copying panel.db.
func (s *Store) BackupTo(ctx context.Context, target string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("database is not initialized")
	}
	target = filepath.Clean(strings.TrimSpace(target))
	if !filepath.IsAbs(target) {
		return fmt.Errorf("backup target must be absolute")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return fmt.Errorf("create database backup dir: %w", err)
	}
	if err := os.Chmod(filepath.Dir(target), 0o700); err != nil {
		return fmt.Errorf("secure database backup dir: %w", err)
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale database backup: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO ?", target); err != nil {
		return fmt.Errorf("vacuum database backup: %w", err)
	}
	if err := os.Chmod(target, 0o600); err != nil {
		return fmt.Errorf("secure database backup: %w", err)
	}
	backupDB, err := sql.Open("sqlite", "file:"+filepath.ToSlash(target)+"?mode=ro")
	if err != nil {
		return fmt.Errorf("open database backup for verification: %w", err)
	}
	defer backupDB.Close()
	var integrity string
	if err := backupDB.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return fmt.Errorf("verify database backup: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("database backup integrity check failed")
	}
	return nil
}
