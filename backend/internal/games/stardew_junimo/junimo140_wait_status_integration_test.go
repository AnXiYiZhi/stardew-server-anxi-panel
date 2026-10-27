//go:build integration

package stardew_junimo

import (
	"context"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

type isolatedJunimo140StatusExecutor struct{ container string }

func (executor isolatedJunimo140StatusExecutor) ComposeExecPipe(ctx context.Context, dir, service, stdin string, args ...string) (paneldocker.CommandResult, error) {
	if service != "server" {
		return paneldocker.CommandResult{}, errors.New("unexpected real status service")
	}
	command := exec.CommandContext(ctx, "docker", append([]string{"exec", "-i", executor.container}, args...)...)
	command.Dir, command.Stdin = dir, strings.NewReader(stdin)
	output, err := command.CombinedOutput()
	code := -1
	if command.ProcessState != nil {
		code = command.ProcessState.ExitCode()
	}
	return paneldocker.CommandResult{Stdout: string(output), ExitCode: code}, err
}

func TestJunimo140WaitStatusRealCursorOptIn(t *testing.T) {
	container := os.Getenv("ANXI_REAL_JUNIMO140_CONTAINER")
	if container == "" {
		t.Skip("set ANXI_REAL_JUNIMO140_CONTAINER and its isolated fixture API key")
	}
	if !strings.HasPrefix(container, "junimo140-20261015-") {
		t.Fatal("real probe requires the task-owned fixture namespace")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	key := os.Getenv("ANXI_REAL_JUNIMO140_API_KEY")
	if strings.ContainsAny(key, "\r\n") {
		t.Fatal("invalid isolated API key")
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("API_PORT=8080\nAPI_KEY="+key+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	executor := isolatedJunimo140StatusExecutor{container: container}
	state, err := readRuntimeWorldState(ctx, executor, dir)
	if err != nil || state.Version == nil || *state.Version <= 0 {
		t.Fatalf("actual sequence unavailable: %v", err)
	}
	next, matched, unsupported, err := readJunimoWaitStatus(ctx, executor, dir, *state.Version)
	if err != nil || !matched || unsupported || next.Version == nil || *next.Version <= *state.Version {
		t.Fatalf("actual wait did not prove a newer completed transition: matched=%v unsupported=%v err=%v", matched, unsupported, err)
	}
	_, matched, unsupported, err = readJunimoWaitStatus(ctx, executor, dir, math.MaxInt64)
	if err != nil || matched || unsupported {
		t.Fatalf("future cursor did not time out without a match: matched=%v unsupported=%v err=%v", matched, unsupported, err)
	}
	t.Log("actual .140 sequence normalized, wait returned a newer snapshot, future cursor returned a bounded 408 without fallback")
}
