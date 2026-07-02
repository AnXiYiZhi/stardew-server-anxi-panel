package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// GetPanelSetting returns a panel-wide setting value.
func (s *Store) GetPanelSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM panel_settings WHERE key = ?`, key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get panel setting %q: %w", key, err)
	}
	return value, nil
}

// SetPanelSetting creates or updates a panel-wide setting.
func (s *Store) SetPanelSetting(ctx context.Context, key string, value string) error {
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO panel_settings (key, value, updated_at)
		VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
	`, key, value); err != nil {
		return fmt.Errorf("set panel setting %q: %w", key, err)
	}
	return nil
}

// DeletePanelSetting removes a panel-wide setting. Missing keys are treated as
// success so callers can make delete operations idempotent.
func (s *Store) DeletePanelSetting(ctx context.Context, key string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM panel_settings WHERE key = ?`, key); err != nil {
		return fmt.Errorf("delete panel setting %q: %w", key, err)
	}
	return nil
}
