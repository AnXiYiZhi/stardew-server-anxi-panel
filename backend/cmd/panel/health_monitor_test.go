package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestSQLiteInterruptTrackerRequiresThreeConsecutiveHealthFailures(t *testing.T) {
	tracker := sqliteInterruptTracker{limit: 3}
	interrupt := healthSQLiteCodeError{code: 9}

	for range 2 {
		count, reached := tracker.observe(interrupt)
		if reached {
			t.Fatalf("limit reached after only %d consecutive interrupts", count)
		}
	}
	if count, reached := tracker.observe(nil); count != 0 || reached {
		t.Fatalf("successful health probe did not reset counter: count=%d reached=%v", count, reached)
	}
	for range 2 {
		tracker.observe(interrupt)
	}
	if count, reached := tracker.observe(errors.New("database busy")); count != 0 || reached {
		t.Fatalf("non-interrupt failure did not reset counter: count=%d reached=%v", count, reached)
	}
	for range 2 {
		tracker.observe(interrupt)
	}
	count, reached := tracker.observe(interrupt)
	if count != 3 || !reached {
		t.Fatalf("third consecutive interrupt: count=%d reached=%v, want count=3 reached=true", count, reached)
	}
}

func TestSQLiteInterruptTrackerUsesNativeCode(t *testing.T) {
	tracker := sqliteInterruptTracker{limit: 3}

	for range 3 {
		if count, reached := tracker.observe(errors.New("interrupted (9)")); count != 0 || reached {
			t.Fatalf("text-only error counted as SQLITE_INTERRUPT: count=%d reached=%v", count, reached)
		}
	}
	if count, reached := tracker.observe(healthSQLiteCodeError{code: 5}); count != 0 || reached {
		t.Fatalf("different SQLite code counted as SQLITE_INTERRUPT: count=%d reached=%v", count, reached)
	}
}

func TestPanelHealthMonitorStopsAfterThirdConsecutiveInterrupt(t *testing.T) {
	interrupt := healthSQLiteCodeError{code: 9}
	results := []error{interrupt, interrupt, nil, interrupt, interrupt, interrupt, interrupt}
	var mu sync.Mutex
	calls := 0
	triggered := make(chan int, 1)

	monitor := panelHealthMonitor{
		interval: time.Millisecond,
		timeout:  50 * time.Millisecond,
		limit:    3,
		check: func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			err := results[calls]
			calls++
			return err
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		onLimit: func(count int, _ error) {
			triggered <- count
		},
	}

	done := make(chan struct{})
	go func() {
		monitor.run(context.Background())
		close(done)
	}()

	select {
	case count := <-triggered:
		if count != 3 {
			t.Fatalf("trigger count = %d, want 3", count)
		}
	case <-time.After(time.Second):
		t.Fatal("health monitor did not trigger")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("health monitor did not stop after triggering recovery")
	}

	mu.Lock()
	defer mu.Unlock()
	if calls != 6 {
		t.Fatalf("health checks = %d, want 6", calls)
	}
}

type healthSQLiteCodeError struct {
	code int
}

func (e healthSQLiteCodeError) Error() string {
	return "sqlite health test error"
}

func (e healthSQLiteCodeError) Code() int {
	return e.code
}
