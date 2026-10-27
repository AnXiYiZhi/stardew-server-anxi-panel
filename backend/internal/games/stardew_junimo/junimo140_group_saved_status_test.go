package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

type junimo140SavedStatusDocker struct {
	*junimo140CoordinatorDocker
	complete bool
	offline  bool
	waits    int
}

func (f *junimo140SavedStatusDocker) ComposeExecPipe(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
	state := map[string]any{"isOnline": !f.offline, "dayTransitionComplete": f.complete, "playerCount": 0, "version": 3}
	if strings.Contains(args[len(args)-1], "/wait/status?") {
		f.waits++
		if f.complete {
			return paneldocker.CommandResult{}, errors.New("no unrelated snapshot change occurs after an already completed save")
		}
		state["version"], state["dayTransitionComplete"] = 4, true
		raw, _ := json.Marshal(state)
		return paneldocker.CommandResult{Stdout: string(raw) + "\n200"}, nil
	}
	raw, _ := json.Marshal(state)
	return paneldocker.CommandResult{Stdout: string(raw)}, nil
}

func TestJunimo140PostSavedSnapshotUsesCurrentCompletion(t *testing.T) {
	for _, complete := range []bool{true, false} {
		t.Run(map[bool]string{true: "already complete", false: "transition still pending"}[complete], func(t *testing.T) {
			dir := t.TempDir()
			writeNewGameTestSave(t, dir, "Saved_123", "0")
			if err := writeGameloaderPointer(dir, "Saved_123"); err != nil {
				t.Fatal(err)
			}
			fake := &junimo140SavedStatusDocker{complete: complete}
			if err := waitJunimo140GroupSavedWorld(context.Background(), fake, dir); err != nil {
				t.Fatal(err)
			}
			wantWaits := 1
			if complete {
				wantWaits = 0
			}
			if fake.waits != wantWaits {
				t.Fatalf("wait calls=%d, want %d", fake.waits, wantWaits)
			}
		})
	}
}

func TestJunimo140PostSavedSnapshotRejectsOfflineWorld(t *testing.T) {
	fake := &junimo140SavedStatusDocker{complete: true, offline: true}
	if err := waitJunimo140GroupSavedWorld(context.Background(), fake, t.TempDir()); err == nil || fake.waits != 0 {
		t.Fatal("offline state was accepted as live post-Saved evidence")
	}
}
