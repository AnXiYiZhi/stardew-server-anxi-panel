package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

func TestImportPlayerReadinessRequiresObservableZeroWithoutCommands(t *testing.T) {
	for _, tc := range []struct {
		name      string
		replies   []string
		wantCode  string
		timeout   bool
		wantCalls int
	}{
		{"initial_snapshot_then_zero", []string{`{}`, `{"playerCount":0}`}, "", false, 2},
		{"player_arrives", []string{`{}`, `{"playerCount":1}`}, ImportErrorPlayersConnected, false, 2},
		{"malformed", []string{`{`}, ImportErrorMaintenanceReady, false, 1},
		{"negative", []string{`{"playerCount":-1}`}, ImportErrorMaintenanceReady, false, 1},
		{"never_observable", []string{`{}`}, ImportErrorMaintenanceReady, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("API_PORT=8080\n"), 0600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			docker := &fakeConsoleDocker{execFunc: func(_ context.Context, _ string, _ string, _ string, args ...string) (paneldocker.CommandResult, error) {
				if len(args) == 0 || args[0] != "curl" {
					t.Fatal("player readiness attempted a console or game mutation")
				}
				index := calls
				if index >= len(tc.replies) {
					index = len(tc.replies) - 1
				}
				calls++
				return paneldocker.CommandResult{Stdout: tc.replies[index]}, nil
			}}
			timeout := time.Second
			if tc.timeout {
				timeout = 25 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			err := waitForImportPlayerCount(ctx, docker, dir, time.Millisecond)
			if tc.timeout {
				if err == nil || !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal("missing count was accepted or lost its bounded deadline", err)
				}
				return
			}
			if tc.wantCode == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				typed, ok := AsImportTransactionError(err)
				if !ok || typed.Code != tc.wantCode {
					t.Fatalf("error=%v want=%s", err, tc.wantCode)
				}
			}
			if tc.wantCalls > 0 && calls != tc.wantCalls {
				t.Fatalf("status reads=%d want=%d", calls, tc.wantCalls)
			}
		})
	}
}
