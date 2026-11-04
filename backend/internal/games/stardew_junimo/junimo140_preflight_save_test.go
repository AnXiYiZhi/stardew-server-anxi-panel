package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJunimo140PreflightRunningSaveWaitsForAtomicReplacement(t *testing.T) {
	dir, name := t.TempDir(), "Selected_123"
	saveDir := filepath.Join(savesDir(dir), "Saves", name)
	if err := os.MkdirAll(saveDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(saveDir, name)
	payload := []byte("<SaveGame><player><name>Existing</name></player></SaveGame>")
	if err := os.WriteFile(path+".pending", payload, 0o600); err != nil {
		t.Fatal(err)
	}
	written := make(chan error, 1)
	go func() {
		time.Sleep(30 * time.Millisecond)
		written <- os.Rename(path+".pending", path)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := captureJunimo140SelectedSave(ctx, dir, name, true)
	if writeErr := <-written; writeErr != nil {
		t.Fatal(writeErr)
	}
	if err != nil {
		t.Fatal("live save replacement should settle without stopping or saving the game", err)
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != string(payload) {
		t.Fatal("preflight changed the selected save", err)
	}
	if _, err := os.Stat(filepath.Join(controlDir(dir), "commands")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("read-only preflight submitted a game command", err)
	}
}

func TestJunimo140PreflightRunningSaveHonorsCancellationAndDeadline(t *testing.T) {
	for _, cancelImmediately := range []bool{true, false} {
		t.Run(map[bool]string{true: "cancelled", false: "deadline"}[cancelImmediately], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			expected := context.DeadlineExceeded
			if cancelImmediately {
				cancel()
				expected = context.Canceled
			}
			err := captureJunimo140SelectedSave(ctx, t.TempDir(), "Selected_123", true)
			if !errors.Is(err, expected) || !cancelImmediately && !errors.Is(err, os.ErrNotExist) {
				t.Fatal("live save wait lost cancellation, deadline, or the original retry error", err)
			}
		})
	}
}

func TestJunimo140PreflightRetainsStoppedAndInvalidSaveChecks(t *testing.T) {
	if err := captureJunimo140SelectedSave(context.Background(), t.TempDir(), "Selected_123", false); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("stopped world with a missing selected save was accepted", err)
	}
	dir, name := t.TempDir(), "Selected_123"
	path := filepath.Join(savesDir(dir), "Saves", name, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("<OtherRoot/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := captureJunimo140SelectedSave(ctx, dir, name, true); err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("malformed stable save must still fail without waiting out the deadline", err)
	}
}
