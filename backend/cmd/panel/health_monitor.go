package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	panelHealthCheckInterval  = time.Minute
	panelHealthCheckTimeout   = 5 * time.Second
	panelHealthInterruptLimit = 3
)

type healthCheckFunc func(context.Context) error

type panelHealthMonitor struct {
	interval time.Duration
	timeout  time.Duration
	limit    int
	check    healthCheckFunc
	logger   *slog.Logger
	onLimit  func(count int, err error)
}

type sqliteInterruptTracker struct {
	consecutive int
	limit       int
}

func (t *sqliteInterruptTracker) observe(err error) (count int, limitReached bool) {
	if !storage.IsSQLiteInterrupt(err) {
		t.consecutive = 0
		return 0, false
	}

	t.consecutive++
	return t.consecutive, t.limit > 0 && t.consecutive >= t.limit
}

func (m panelHealthMonitor) run(ctx context.Context) {
	if m.check == nil || m.interval <= 0 || m.timeout <= 0 || m.limit <= 0 {
		return
	}
	if m.logger == nil {
		m.logger = slog.Default()
	}

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	tracker := sqliteInterruptTracker{limit: m.limit}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkCtx, cancel := context.WithTimeout(ctx, m.timeout)
			err := m.check(checkCtx)
			cancel()
			if ctx.Err() != nil {
				return
			}

			previous := tracker.consecutive
			count, limitReached := tracker.observe(err)
			switch {
			case storage.IsSQLiteInterrupt(err):
				m.logger.Warn("panel health probe returned SQLITE_INTERRUPT",
					"consecutive_interrupts", count,
					"limit", m.limit,
					"error", err,
				)
			case err != nil:
				m.logger.Warn("panel health probe failed without SQLITE_INTERRUPT; restart counter reset",
					"previous_consecutive_interrupts", previous,
					"error", err,
				)
			case previous > 0:
				m.logger.Info("panel health probe recovered; SQLITE_INTERRUPT counter reset",
					"previous_consecutive_interrupts", previous,
				)
			}

			if limitReached {
				if m.onLimit != nil {
					m.onLimit(count, err)
				}
				return
			}
		}
	}
}
