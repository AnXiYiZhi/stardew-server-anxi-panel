//go:build linux

package stardew_junimo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestNewGameOwnerRenameNeedsFallback(t *testing.T) {
	for _, err := range []error{unix.EINVAL, unix.ENOSYS, unix.EOPNOTSUPP} {
		if !newGameOwnerRenameNeedsFallback(err) {
			t.Fatalf("newGameOwnerRenameNeedsFallback(%v) = false", err)
		}
	}
	if newGameOwnerRenameNeedsFallback(unix.EPERM) {
		t.Fatal("EPERM must not enable the compatibility fallback")
	}
}

func TestRenameNewGameOwnerNoReplacePublishesOnCurrentFilesystem(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, ".new-game-owner.claim-test")
	target := filepath.Join(root, "new-game-owner")
	if err := os.Mkdir(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"transactionId":"test"}`)
	if err := os.WriteFile(filepath.Join(staging, "owner.json"), want, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := renameNewGameOwnerNoReplace(staging, target); err != nil {
		t.Fatalf("publish owner: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(target, "owner.json"))
	if err != nil {
		t.Fatalf("read published owner: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("published owner = %q, want %q", got, want)
	}
}

func TestRenameNewGameOwnerViaExclusiveDirectory(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, ".new-game-owner.claim-test")
	target := filepath.Join(root, "new-game-owner")
	if err := os.Mkdir(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"transactionId":"test"}`)
	if err := os.WriteFile(filepath.Join(staging, "owner.json"), want, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := renameNewGameOwnerViaExclusiveDirectory(staging, target); err != nil {
		t.Fatalf("fallback publish: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(target, "owner.json"))
	if err != nil {
		t.Fatalf("read published owner: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("published owner = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(staging, "owner.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged owner still exists: %v", err)
	}
}

func TestRenameNewGameOwnerViaExclusiveDirectoryDoesNotReplace(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, ".new-game-owner.claim-test")
	target := filepath.Join(root, "new-game-owner")
	if err := os.Mkdir(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "owner.json"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "owner.json"), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := renameNewGameOwnerViaExclusiveDirectory(staging, target)
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("fallback error = %v, want os.ErrExist", err)
	}
	got, readErr := os.ReadFile(filepath.Join(target, "owner.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "existing" {
		t.Fatalf("existing owner was replaced: %q", got)
	}
}

func TestRenameNewGameOwnerViaExclusiveDirectoryRejectsNonFileOwner(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, ".new-game-owner.claim-test")
	target := filepath.Join(root, "new-game-owner")
	if err := os.Mkdir(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(staging, "owner.json"), 0o700); err != nil {
		t.Fatal(err)
	}

	err := renameNewGameOwnerViaExclusiveDirectory(staging, target)
	if err == nil {
		t.Fatal("fallback unexpectedly accepted a directory owner.json")
	}
	if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed claim left target behind: %v", statErr)
	}
}
