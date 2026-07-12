package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func commandTestInstance(t *testing.T) registryInstanceForTest {
	t.Helper()
	return registryInstanceForTest{dataDir: t.TempDir()}
}

// Keep test setup independent from the production registry constructor helpers.
type registryInstanceForTest struct {
	dataDir string
}

func (v registryInstanceForTest) running() registry.Instance {
	instance := makeRunningInstance()
	instance.DataDir = v.dataDir
	return instance
}

func TestTriggerFestivalEventQueuesResultCommand(t *testing.T) {
	fixture := commandTestInstance(t)
	instance := fixture.running()
	result, err := triggerFestivalEvent(instance)
	if err != nil {
		t.Fatal(err)
	}
	if !validCommandID(result.CommandID) {
		t.Fatalf("invalid command ID: %q", result.CommandID)
	}
	if ok, err := commandFileExists(instance.DataDir, result.CommandID); err != nil || !ok {
		t.Fatalf("trigger-event command was not queued: ok=%v err=%v", ok, err)
	}
}

func TestRequestSaveNowQueuesCommandAndRejectsStoppedServer(t *testing.T) {
	fixture := commandTestInstance(t)
	instance := fixture.running()
	result, err := requestSaveNow(instance)
	if err != nil {
		t.Fatal(err)
	}
	if !validCommandID(result.CommandID) {
		t.Fatalf("invalid command ID: %q", result.CommandID)
	}
	instance.State = storage.InstanceStateStopped
	if _, err := requestSaveNow(instance); err == nil || err.(*CommandError).Code != "server_not_running" {
		t.Fatalf("stopped save request error = %v", err)
	}
}

func TestEnableJojaAdminPromotionFailureQueuesStructuredModFailure(t *testing.T) {
	fixture := commandTestInstance(t)
	instance := fixture.running()
	control := controlDir(instance.DataDir)
	if err := os.MkdirAll(control, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(control, "status.json"), []byte(`{"commandResultVersion":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(control, "players.json"), []byte(`{"players":[{"name":"Host","uniqueMultiplayerId":"1","isHost":true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	driver := newTestDriver(&fakeConsoleDocker{execFunc: func(context.Context, string, string, string, ...string) (paneldocker.CommandResult, error) {
		return paneldocker.CommandResult{ExitCode: 7}, errors.New("admin endpoint unavailable")
	}})
	result, err := enableJojaRoute(context.Background(), driver, instance, jojaConfirmText)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != CommandStatusQueued {
		t.Fatalf("status = %q", result.Status)
	}
	entries, err := os.ReadDir(filepath.Join(control, "commands"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("commands = %d err=%v", len(entries), err)
	}
	raw, err := os.ReadFile(filepath.Join(control, "commands", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"adminPromoted": "false"`) {
		t.Fatalf("admin failure evidence missing from command: %s", raw)
	}
}
