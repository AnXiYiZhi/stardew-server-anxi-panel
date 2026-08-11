//go:build integration

package stardew_junimo

import (
	"context"
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

// TestRealNewGameMaterializesSMAPIModsBeforeFirstSaveOptIn is the release
// acceptance for a genuinely empty saves bind. It only reads the supplied game
// volume, clones it into an owned volume, starts the real Junimo/SMAPI stack,
// creates one save, and proves bundled SMAPI support Mods were published before
// the new-game transaction snapshot was prepared.
func TestRealNewGameMaterializesSMAPIModsBeforeFirstSaveOptIn(t *testing.T) {
	sourceGameVolume := strings.TrimSpace(os.Getenv("ANXI_REAL_NEW_GAME_SOURCE_GAME_VOLUME"))
	if sourceGameVolume == "" {
		t.Skip("set ANXI_REAL_NEW_GAME_SOURCE_GAME_VOLUME")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	suffix := strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	project := "anxirealnewgame" + suffix
	dataDir := filepath.Join(os.TempDir(), project)
	gameVolume := project + "_game-data"
	steamVolume := project + "_steam-session"
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
		ID: "stardew", DriverID: DriverID, Name: "real new game", DataDir: dataDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	stored, err = store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
		ID: stored.ID, State: storage.InstanceStateAdminCreated, DriverPhase: "empty", DriverPayload: "{}",
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
	driver := New(client, slog.Default(), manager, store, "0.4.11")
	if err := driver.Prepare(ctx, instance); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{
		"GAME_DATA_VOLUME":    gameVolume,
		"GAME_PORT":           "0",
		"QUERY_PORT":          "0",
		"VNC_PORT":            reserveIntegrationTCPPort(t),
		"API_PORT":            reserveIntegrationTCPPort(t),
		"VNC_PASSWORD":        "real-new-game-e2e",
		"STEAM_USERNAME":      "",
		"STEAM_PASSWORD":      "",
		"STEAM_REFRESH_TOKEN": "",
	}); err != nil {
		t.Fatal(err)
	}
	run("volume", "create", "--label", "com.openai.codex.owner=anxi-real-new-game-gate", gameVolume)
	run("volume", "create", "--label", "com.openai.codex.owner=anxi-real-new-game-gate", steamVolume)
	run("run", "--rm", "--network", "none",
		"--mount", "type=volume,src="+sourceGameVolume+",dst=/source,readonly",
		"--mount", "type=volume,src="+gameVolume+",dst=/target",
		"alpine:3.20", "sh", "-c", "cd /source && tar cf - . | tar xf - -C /target")

	stored, err = store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
		ID: stored.ID, State: storage.InstanceStateSaveRequired, StateMessage: "create first save", DriverPhase: "save_required", DriverPayload: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	instance.State, instance.DriverPhase, instance.DriverPayload = stored.State, stored.DriverPhase, stored.DriverPayload
	if inspection := InspectManagedRuntimeStack(dataDir, instance.State); inspection.Status != sjconfig.RuntimeStackStatusUpToDate {
		t.Fatalf("prepared real runtime stack is not current: status=%s code=%s reason=%s", inspection.Status, inspection.Code, inspection.Reason)
	}
	if _, err := os.Stat(filepath.Join(modsDir(dataDir), "smapi")); !os.IsNotExist(err) {
		t.Fatalf("managed SMAPI destination must be absent before first Start: %v", err)
	}

	config := registry.NewGameConfig{
		FarmName: "Release Gate", FarmType: "standard", StartingCabins: 1, MaxPlayers: 4,
		CabinLayout: "nearby", CabinMode: "recommended", ProfitMargin: "100", MoneyMode: "shared",
		FarmerName: "GateHost", FavoriteThing: "Reliable releases", Gender: "female", PetType: "Cat",
	}
	job, err := driver.Start(ctx, registry.StartRequest{
		Instance: instance, ActorID: 0, NewGame: true, NewGameConfig: &config,
	})
	if err != nil {
		t.Fatal(err)
	}
	var finished storage.Job
	for deadline := time.Now().Add(12 * time.Minute); time.Now().Before(deadline); {
		finished, err = store.GetJob(ctx, job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if finished.Status != storage.JobStatusQueued && finished.Status != storage.JobStatusRunning {
			break
		}
		time.Sleep(time.Second)
	}
	logs, logsErr := store.ListJobLogs(context.Background(), job.ID, 0, 1000)
	if logsErr != nil {
		t.Fatal(logsErr)
	}
	if finished.Status != storage.JobStatusSucceeded {
		start := 0
		if len(logs) > 25 {
			start = len(logs) - 25
		}
		var tail []string
		for _, line := range logs[start:] {
			tail = append(tail, line.Message)
		}
		t.Fatalf("real new-game job status=%s error=%s logs=%s", finished.Status, finished.ErrorMessage.String, strings.Join(tail, " | "))
	}

	materializedSequence, transactionSequence := int64(0), int64(0)
	for _, line := range logs {
		if strings.Contains(line.Message, "物化 SMAPI 内置支持 Mod") {
			materializedSequence = line.Sequence
		}
		if strings.Contains(line.Message, "新建存档事务已准备") {
			transactionSequence = line.Sequence
		}
	}
	if materializedSequence == 0 || transactionSequence == 0 || materializedSequence >= transactionSequence {
		t.Fatalf("SMAPI materialization was not recorded before the new-game snapshot: materialized=%d transaction=%d", materializedSequence, transactionSequence)
	}
	for _, manifest := range []string{
		filepath.Join(modsDir(dataDir), "smapi", "ConsoleCommands", "manifest.json"),
		filepath.Join(modsDir(dataDir), "smapi", "SaveBackup", "manifest.json"),
	} {
		if info, statErr := os.Stat(manifest); statErr != nil || info.Size() == 0 {
			t.Fatalf("materialized SMAPI manifest missing or empty: %s: %v", manifest, statErr)
		}
	}
	ownedArtifacts, err := filepath.Glob(filepath.Join(modsDir(dataDir), ".smapi-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ownedArtifacts) != 0 {
		t.Fatalf("successful first save left SMAPI staging artifacts: %v", ownedArtifacts)
	}
	saves, err := driver.ListSaves(ctx, instance)
	if err != nil {
		t.Fatal(err)
	}
	if len(saves) != 1 || saves[0].ParseError != "" || saves[0].FarmName != config.FarmName || !saves[0].IsActive {
		t.Fatalf("real first save integrity mismatch: %+v", saves)
	}
	if active := GetActiveSaveName(dataDir); active == "" || active != saves[0].Name {
		t.Fatalf("active save=%q, created save=%q", active, saves[0].Name)
	}
	if err := driver.Stop(ctx, instance); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(2 * time.Minute); time.Now().Before(deadline); {
		current, getErr := store.GetInstance(ctx, instance.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if current.DriverPhase == "stopped" {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	current, err := store.GetInstance(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.DriverPhase != "stopped" {
		t.Fatalf("real first-save stack did not stop cleanly: state=%s phase=%s", current.State, current.DriverPhase)
	}
	if output, psErr := exec.CommandContext(ctx, "docker", "ps", "-aq", "--filter", "label=com.docker.compose.project="+project).CombinedOutput(); psErr != nil || strings.TrimSpace(string(output)) != "" {
		t.Fatalf("real first-save project left containers after Stop: %v %s", psErr, output)
	}
	t.Logf("real first save created: project=%s save=%s farm=%s materializedSequence=%d transactionSequence=%d", project, saves[0].Name, saves[0].FarmName, materializedSequence, transactionSequence)
}
