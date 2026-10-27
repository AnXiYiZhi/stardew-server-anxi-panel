package stardew_junimo

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

func TestJunimo140StatusNormalizesSnapshotCursor(t *testing.T) {
	for _, test := range []struct {
		name      string
		raw       string
		value     int64
		wantError bool
	}{
		{"legacy", `{"version":12}`, 12, false},
		{"image140", `{"sequence":13}`, 13, false},
		{"consistent dual", `{"version":14,"sequence":14}`, 14, false},
		{"conflict", `{"version":14,"sequence":15}`, 0, true},
		{"negative", `{"sequence":-1}`, 0, true},
		{"fraction", `{"sequence":1.5}`, 0, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var state JunimoRuntimeWorldState
			err := json.Unmarshal([]byte(test.raw), &state)
			if (err != nil) != test.wantError {
				t.Fatalf("cursor decode err=%v wantError=%v", err, test.wantError)
			}
			if err == nil && (state.Version == nil || *state.Version != test.value) {
				t.Fatal("snapshot cursor was not normalized")
			}
		})
	}
}

func TestJunimo140WaitStatusRequiresFreshSnapshot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("API_PORT=5110\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		body    string
		matched bool
	}{
		{"sequence fresh", `{"sequence":11,"dayTransitionComplete":true}`, true},
		{"legacy fresh", `{"version":11,"dayTransitionComplete":true}`, true},
		{"sequence stale", `{"sequence":10,"dayTransitionComplete":true}`, false},
		{"sequence regressed", `{"sequence":9,"dayTransitionComplete":true}`, false},
		{"transition pending", `{"sequence":11,"dayTransitionComplete":false}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeConsoleDocker{}
			fake.execFunc = func(context.Context, string, string, string, ...string) (paneldocker.CommandResult, error) {
				return paneldocker.CommandResult{Stdout: test.body + "\n200"}, nil
			}
			_, matched, unsupported, err := readJunimoWaitStatus(context.Background(), fake, dir, 10)
			if err != nil || unsupported || matched != test.matched {
				t.Fatalf("freshness matched=%v unsupported=%v err=%v", matched, unsupported, err)
			}
		})
	}
}
