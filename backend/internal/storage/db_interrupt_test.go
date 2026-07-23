package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
)

const cancellableRecursiveQuery = `
WITH RECURSIVE counter(value) AS (
	VALUES(0)
	UNION ALL
	SELECT value + 1 FROM counter WHERE value < 100000000
)
SELECT sum(value) FROM counter`

func TestCanceledQueryDoesNotPoisonNextSQLiteQuery(t *testing.T) {
	store := openInterruptTestStore(t, OpenOptions{})
	defer store.Close()

	cancelLongSQLiteQuery(t, store)

	var got int
	if err := store.db.QueryRowContext(context.Background(), "SELECT 42").Scan(&got); err != nil {
		t.Fatalf("query after canceled request failed: %v", err)
	}
	if got != 42 {
		t.Fatalf("query after canceled request = %d, want 42", got)
	}
}

func TestRepeatedSQLiteInterruptInvokesRecoveryGuard(t *testing.T) {
	triggered := make(chan int, 1)
	observer := newInterruptObserver(3, func(count int) {
		triggered <- count
	})

	observer.observe(sqliteCodeError{code: sqliteInterruptCode})
	observer.observe(nil)
	for range 2 {
		observer.observe(sqliteCodeError{code: sqliteInterruptCode})
	}
	select {
	case count := <-triggered:
		t.Fatalf("recovery guard triggered after only two consecutive interrupts: %d", count)
	default:
	}
	observer.observe(sqliteCodeError{code: sqliteInterruptCode})

	select {
	case count := <-triggered:
		if count != 3 {
			t.Fatalf("recovery guard count = %d, want 3", count)
		}
	case <-time.After(time.Second):
		t.Fatal("recovery guard was not invoked after three consecutive SQLITE_INTERRUPT results")
	}
}

type sqliteCodeError struct {
	code int
}

func (e sqliteCodeError) Error() string {
	return "sqlite test error"
}

func (e sqliteCodeError) Code() int {
	return e.code
}

func openInterruptTestStore(t *testing.T, opts OpenOptions) *Store {
	t.Helper()
	dataDir := t.TempDir()
	store, err := OpenWithOptions(context.Background(), config.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	}, opts)
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	return store
}

func cancelLongSQLiteQuery(t *testing.T, store *Store) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(10*time.Millisecond, cancel)

	var ignored int64
	err := store.db.QueryRowContext(ctx, cancellableRecursiveQuery).Scan(&ignored)
	cancel()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled query error = %v, want context.Canceled", err)
	}
}
