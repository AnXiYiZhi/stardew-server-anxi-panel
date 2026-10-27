//go:build !windows

package stardew_junimo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestConsoleWriterRealFIFONoReaderAndMissingPath(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "smapi-input")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{fifo, filepath.Join(dir, "missing")} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		args := consoleWriterArgs(consoleRuntimeContract{fifo: path})
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdin = strings.NewReader("info\n")
		output, err := cmd.CombinedOutput()
		cancel()
		if err == nil || ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("writer did not reject missing reader/path promptly: %s %v %s", path, err, output)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "missing")); !os.IsNotExist(err) {
		t.Fatalf("writer created a regular file: %v", err)
	}
}

func TestConsoleWriterRealFIFODropsToOwner(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requires isolated Linux root to verify UID drop")
	}
	dir, err := os.MkdirTemp("/tmp", "sap-fifo-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "smapi-input")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(fifo, 1000, 1000); err != nil {
		t.Fatal(err)
	}
	reader, err := os.OpenFile(fifo, os.O_RDWR|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	args := consoleWriterArgs(consoleRuntimeContract{fifo: fifo, uid: "1000", gid: "1000"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdin = strings.NewReader("info\n")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("non-root FIFO write failed: %v %s", err, output)
	}
	buf := make([]byte, 64)
	n, err := reader.Read(buf)
	if err != nil || string(buf[:n]) != "info\n" {
		t.Fatalf("FIFO did not deliver exact command: n=%d err=%v", n, err)
	}
}
