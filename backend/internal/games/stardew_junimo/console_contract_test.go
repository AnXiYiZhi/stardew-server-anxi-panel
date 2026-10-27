package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

func TestConsoleContractUsesDeployedImageAcrossUpgradeAndRollback(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		env, fifo, uid, gid string
	}{
		{"SERVER_IMAGE=sdvd/server:1.5.0-preview.125\n", serverInputFIFO, "", ""},
		{"SERVER_IMAGE=mirror.example/sdvd/server:1.5.0-preview.140\n", junimo140InputFIFO, "1000", "1000"},
		{"SERVER_IMAGE=sdvd/server@" + junimo140Digest + "\nUSER_ID=1200\nGROUP_ID=1201\n", junimo140InputFIFO, "1200", "1201"},
		{"IMAGE_VERSION=1.5.0-preview.140\n", junimo140InputFIFO, "1000", "1000"},
		{"SERVER_IMAGE=sha256:" + strings.Repeat("a", 64) + "\nIMAGE_VERSION=1.5.0-preview.140\n", junimo140InputFIFO, "1000", "1000"},
		{"SERVER_IMAGE=sha256:" + strings.Repeat("b", 64) + "\nIMAGE_VERSION=1.5.0-preview.125\n", serverInputFIFO, "", ""},
		{"SERVER_IMAGE=sdvd/server:1.5.0-preview.125\nIMAGE_VERSION=1.5.0-preview.140\n", serverInputFIFO, "", ""},
	} {
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(tc.env), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := readConsoleRuntimeContract(dir)
		if err != nil || got.fifo != tc.fifo || got.uid != tc.uid || got.gid != tc.gid {
			t.Fatalf("contract=%+v err=%v; want FIFO %s, uid/gid %s/%s", got, err, tc.fifo, tc.uid, tc.gid)
		}
	}
}

func TestConsoleRejectsInvalidRuntimeIdentityBeforeDocker(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SERVER_IMAGE=sdvd/server:1.5.0-preview.140\nUSER_ID=1000;id\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := 0
	fake := &fakeConsoleDocker{execFunc: func(context.Context, string, string, string, ...string) (paneldocker.CommandResult, error) {
		calls++
		return paneldocker.CommandResult{}, nil
	}}
	if _, err := writeServerInput(context.Background(), fake, dir, "info"); err == nil || calls != 0 {
		t.Fatalf("invalid identity reached Docker: calls=%d err=%v", calls, err)
	}
}

func TestConsoleRefusesMissingOrRegularFIFOWithoutWriting(t *testing.T) {
	for _, code := range []int{1, 2} {
		writes := 0
		fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, stdin string, args ...string) (paneldocker.CommandResult, error) {
			if stdin != "" {
				writes++
			}
			return paneldocker.CommandResult{ExitCode: code}, nil
		}}
		if _, err := writeServerInput(context.Background(), fake, t.TempDir(), "info"); err == nil || writes != 0 {
			t.Fatalf("invalid FIFO accepted: writes=%d err=%v", writes, err)
		}
	}
}

func TestConsoleWriteDoesNotTurnWriterEchoIntoReceipt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, stdin string, args ...string) (paneldocker.CommandResult, error) {
		if args[0] == "wc" {
			return paneldocker.CommandResult{Stdout: "0 " + serverOutputLog}, nil
		}
		return paneldocker.CommandResult{Stdout: stdin}, nil
	}}
	output, code, _, err := sendServerCommand(ctx, fake, t.TempDir(), "info")
	if !errors.Is(err, context.DeadlineExceeded) || code == 0 || strings.TrimSpace(output) != "" {
		t.Fatalf("writer echo accepted as receipt: output=%q code=%d err=%v", output, code, err)
	}
}

func TestConsoleReceiptRejectsUnrelatedLogAndCommandEcho(t *testing.T) {
	for _, output := range []string{"info\n", "$> info\n", "[SMAPI] unrelated tick log", "[JunimoServer] Saved successfully"} {
		if serverCommandResponseMatches("info", output) {
			t.Fatalf("unrelated output accepted as info receipt: %q", output)
		}
	}
	if !serverCommandResponseMatches("info", "[JunimoServer] Players: 1/4") || !serverCommandResponseMatches("saves info Fixture_1", "Save: Fixture_1") {
		t.Fatal("recognized business response was rejected")
	}
	if serverCommandResponseMatches("saves info Fixture_1", "$>saves info Fixture_1") || serverCommandResponseMatches("saves info Fixture_1", "Save: Fixture_10") {
		t.Fatal("save query echo or another save was accepted")
	}
}
