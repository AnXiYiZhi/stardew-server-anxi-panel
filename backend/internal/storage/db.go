package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
)

// Store wraps the panel SQLite database.
type Store struct {
	db *sql.DB
}

// OpenOptions configures process-level handling for repeated SQLite failures.
type OpenOptions struct {
	// OnRepeatedInterrupt is called after three consecutive database operations
	// end with SQLITE_INTERRUPT. Panel uses this to terminate the process so its
	// Docker restart policy can replace a connection pool that cannot recover.
	OnRepeatedInterrupt func(count int)
}

// Open creates required directories, opens SQLite, and applies connection settings.
func Open(ctx context.Context, cfg config.Config) (*Store, error) {
	return OpenWithOptions(ctx, cfg, OpenOptions{})
}

// OpenWithOptions creates required directories, opens SQLite, and applies
// connection settings with an optional repeated-interrupt process guard.
func OpenWithOptions(ctx context.Context, cfg config.Config, opts OpenOptions) (*Store, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbDir := filepath.Dir(cfg.DBPath)
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return nil, fmt.Errorf("create database dir: %w", err)
	}

	driverName := registerObservedSQLiteDriver(newInterruptObserver(3, opts.OnRepeatedInterrupt))
	db, err := sql.Open(driverName, cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &Store{db: db}
	if err := store.applyPragmas(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.Ping(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

// Close releases the SQLite connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Ping verifies that the SQLite database is reachable.
func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("database is not initialized")
	}
	return s.db.PingContext(ctx)
}

func (s *Store) applyPragmas(ctx context.Context) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	}
	for _, pragma := range pragmas {
		if _, err := s.db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("apply sqlite pragma %q: %w", pragma, err)
		}
	}
	return nil
}
