//go:build integration

package stardew_junimo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// TestRealHostFarmhouseLevelPreservedAcrossLoadOptIn proves the exact supported
// JunimoServer runtime accepts the Harmony prefix and does not rewrite an
// upgraded host's houseUpgradeLevel during SaveLoaded. The source game volume
// and source save are read-only inputs; all mutations happen in task-owned
// copies and the saved result must still contain level 2 after GameLoop.Saved.
func TestRealHostFarmhouseLevelPreservedAcrossLoadOptIn(t *testing.T) {
	sourceGameVolume := strings.TrimSpace(os.Getenv("ANXI_REAL_HOST_HOUSE_SOURCE_GAME_VOLUME"))
	sourceSaveDir := strings.TrimSpace(os.Getenv("ANXI_REAL_HOST_HOUSE_SOURCE_SAVE_DIR"))
	if sourceGameVolume == "" || sourceSaveDir == "" {
		t.Skip("set ANXI_REAL_HOST_HOUSE_SOURCE_GAME_VOLUME and ANXI_REAL_HOST_HOUSE_SOURCE_SAVE_DIR")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	suffix := strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	project := "anxihosthouse" + suffix
	dataDir := filepath.Join(os.TempDir(), project)
	gameVolume := project + "_game-data"
	steamVolume := project + "_steam-session"
	keepFailed := strings.TrimSpace(os.Getenv("ANXI_REAL_HOST_HOUSE_KEEP_FAILED")) == "1"
	run := func(args ...string) string {
		t.Helper()
		output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker %v: %v: %s", args, err, output)
		}
		return string(output)
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if keepFailed && t.Failed() {
			t.Logf("retaining failed host-house fixture: project=%s dataDir=%s volumes=%s,%s", project, dataDir, gameVolume, steamVolume)
			return
		}
		_ = exec.Command("docker", "compose", "--project-name", project, "--project-directory", dataDir, "down", "--volumes", "--remove-orphans").Run()
		_ = exec.Command("docker", "volume", "rm", "-f", gameVolume, steamVolume).Run()
		_ = os.RemoveAll(dataDir)
	})

	store, err := storage.Open(ctx, appconfig.Config{DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel-e2e.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	stored, err := store.EnsureDefaultInstance(ctx, storage.EnsureDefaultInstanceParams{
		ID: "stardew", DriverID: DriverID, Name: "host house preservation", DataDir: dataDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	stored, err = store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
		ID: stored.ID, State: storage.InstanceStateAdminCreated, DriverPhase: "admin_created", DriverPayload: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	instance := registry.Instance{
		ID: stored.ID, DriverID: stored.DriverID, Name: stored.Name, DataDir: stored.DataDir,
		State: stored.State, DriverPhase: stored.DriverPhase, DriverPayload: stored.DriverPayload,
	}
	client := paneldocker.NewClient(paneldocker.Options{DockerPath: "docker"})
	manager := jobs.NewManager(store, slog.Default())
	driver := New(client, slog.Default(), manager, store, "host-house-real-gate")
	if err := driver.Prepare(ctx, instance); err != nil {
		t.Fatal(err)
	}
	stored, err = store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
		ID: stored.ID, State: storage.InstanceStateStopped, DriverPhase: "stopped", DriverPayload: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	instance.State = stored.State
	instance.DriverPhase = stored.DriverPhase
	instance.DriverPayload = stored.DriverPayload
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{
		"GAME_DATA_VOLUME":    gameVolume,
		"GAME_PORT":           "0",
		"QUERY_PORT":          "0",
		"VNC_PORT":            reserveIntegrationTCPPort(t),
		"API_PORT":            reserveIntegrationTCPPort(t),
		"VNC_PASSWORD":        "host-house-real-gate",
		"STEAM_USERNAME":      "",
		"STEAM_PASSWORD":      "",
		"STEAM_REFRESH_TOKEN": "",
	}); err != nil {
		t.Fatal(err)
	}
	run("volume", "create", "--label", "com.openai.codex.owner=anxi-host-house-gate", gameVolume)
	run("volume", "create", "--label", "com.openai.codex.owner=anxi-host-house-gate", steamVolume)
	run("run", "--rm", "--network", "none",
		"--mount", "type=volume,src="+sourceGameVolume+",dst=/source,readonly",
		"--mount", "type=volume,src="+gameVolume+",dst=/target",
		"alpine:3.20", "sh", "-c", "cd /source && tar cf - . | tar xf - -C /target")

	saveName := filepath.Base(filepath.Clean(sourceSaveDir))
	if saveName == "." || saveName == string(filepath.Separator) || strings.TrimSpace(saveName) == "" {
		t.Fatal("source save directory has no valid base name")
	}
	targetSaveDir := filepath.Join(savesDir(dataDir), "Saves", saveName)
	if err := copyHostHouseSaveFixture(sourceSaveDir, targetSaveDir); err != nil {
		t.Fatal(err)
	}
	mainSavePath := filepath.Join(targetSaveDir, saveName)
	mainSave, err := os.ReadFile(mainSavePath)
	if err != nil {
		t.Fatal(err)
	}
	levelZero := []byte("<houseUpgradeLevel>0</houseUpgradeLevel>")
	levelTwo := []byte("<houseUpgradeLevel>2</houseUpgradeLevel>")
	if !bytes.Contains(mainSave, levelZero) {
		t.Fatal("source save does not contain a level-zero host house fixture")
	}
	mainSave = bytes.Replace(mainSave, levelZero, levelTwo, 1)
	if err := os.WriteFile(mainSavePath, mainSave, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := firstHostHouseUpgradeLevel(mainSave); got != "2" {
		t.Fatalf("prepared host house level=%q, want 2", got)
	}
	if err := SetActiveSave(dataDir, saveName); err != nil {
		t.Fatal(err)
	}
	if inspection := InspectManagedRuntimeStack(dataDir, instance.State); inspection.Status != sjconfig.RuntimeStackStatusUpToDate {
		t.Fatalf("prepared runtime stack is not current: status=%s code=%s reason=%s", inspection.Status, inspection.Code, inspection.Reason)
	}

	job, err := driver.Start(ctx, registry.StartRequest{Instance: instance, ActorID: 0})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitForHostHouseJob(t, ctx, store, job.ID, 20*time.Minute)
	if finished.Status != storage.JobStatusSucceeded {
		logs, _ := store.ListJobLogs(context.Background(), job.ID, 0, 1000)
		start := 0
		if len(logs) > 25 {
			start = len(logs) - 25
		}
		var tail []string
		for _, line := range logs[start:] {
			tail = append(tail, line.Message)
		}
		t.Fatalf("host-house start status=%s error=%s logs=%s", finished.Status, finished.ErrorMessage.String, strings.Join(tail, " | "))
	}
	gate := InspectControlRuntimeGate(dataDir)
	if gate.State != ControlRuntimeGateReady {
		t.Fatalf("Control host-house patch gate=%+v", gate)
	}
	stored, err = store.GetInstance(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	instance.State = stored.State
	instance.DriverPhase = stored.DriverPhase
	instance.DriverPayload = stored.DriverPayload
	submitted, err := driver.RequestSaveNow(ctx, instance)
	if err != nil {
		t.Fatal(err)
	}
	var outcome CommandOutcome
	for deadline := time.Now().Add(3 * time.Minute); time.Now().Before(deadline); {
		outcome, err = GetCommandOutcome(dataDir, submitted.CommandID)
		if err != nil {
			t.Fatal(err)
		}
		if outcome.Status == CommandStatusSucceeded || outcome.Status == CommandStatusFailed || outcome.Status == CommandStatusExpired {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if outcome.Status != CommandStatusSucceeded {
		t.Fatalf("save-now outcome=%#v", outcome)
	}

	persisted, err := os.ReadFile(mainSavePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := firstHostHouseUpgradeLevel(persisted); got != "2" {
		t.Fatalf("host house level after real SaveLoaded + GameLoop.Saved=%q, want 2", got)
	}
	if err := driver.Stop(ctx, instance); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(2 * time.Minute); time.Now().Before(deadline); {
		stored, err = store.GetInstance(ctx, instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if stored.DriverPhase == "stopped" {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if stored.DriverPhase != "stopped" {
		t.Fatalf("host-house stack did not stop cleanly: state=%s phase=%s", stored.State, stored.DriverPhase)
	}
	t.Log("real JunimoServer .125 load/save preserved host farmhouse level 2")
}

func waitForHostHouseJob(t *testing.T, ctx context.Context, store *storage.Store, jobID string, timeout time.Duration) storage.Job {
	t.Helper()
	var current storage.Job
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); {
		var err error
		current, err = store.GetJob(ctx, jobID)
		if err != nil {
			t.Fatal(err)
		}
		if current.Status != storage.JobStatusQueued && current.Status != storage.JobStatusRunning {
			return current
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("job %s did not finish within %s; last status=%s", jobID, timeout, current.Status)
	return storage.Job{}
}

func copyHostHouseSaveFixture(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o700)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("source save contains non-regular entry: %s", entry.Name())
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputCloseErr := input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputCloseErr != nil {
			return inputCloseErr
		}
		return closeErr
	})
}

func firstHostHouseUpgradeLevel(save []byte) string {
	const open = "<houseUpgradeLevel>"
	const close = "</houseUpgradeLevel>"
	start := bytes.Index(save, []byte(open))
	if start < 0 {
		return ""
	}
	start += len(open)
	end := bytes.Index(save[start:], []byte(close))
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(string(save[start : start+end]))
}
