//go:build integration

package stardew_junimo

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	appconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type realNewGameCountingDocker struct {
	*paneldocker.Client
	newGamePosts atomic.Int32
}

func (d *realNewGameCountingDocker) ComposeExecPipe(ctx context.Context, dir, service, stdinData string, args ...string) (paneldocker.CommandResult, error) {
	joined := strings.Join(args, " ")
	if service == "server" && strings.Contains(joined, "-X POST") && strings.Contains(joined, "http://localhost:8080/newgame") {
		d.newGamePosts.Add(1)
	}
	return d.Client.ComposeExecPipe(ctx, dir, service, stdinData, args...)
}

// TestRealNewGameMaterializesSMAPIModsBeforeFirstSaveOptIn is the release
// acceptance for both persistent creation writers. It only reads the supplied
// game volume, clones it into an owned volume, creates the first save through
// the startup writer, then creates a second save from the stopped first-save
// baseline through exactly one HTTP writer POST. Both transactions must pass
// every Control/save-now/disk durability gate, and the old save must stay byte
// identical across the HTTP writer transaction.
func TestRealNewGameMaterializesSMAPIModsBeforeFirstSaveOptIn(t *testing.T) {
	sourceGameVolume := strings.TrimSpace(os.Getenv("ANXI_REAL_NEW_GAME_SOURCE_GAME_VOLUME"))
	if sourceGameVolume == "" {
		t.Skip("set ANXI_REAL_NEW_GAME_SOURCE_GAME_VOLUME")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
	defer cancel()
	suffix := strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	project := "anxirealnewgame" + suffix
	dataDir := filepath.Join(os.TempDir(), project)
	gameVolume := project + "_game-data"
	steamVolume := project + "_steam-session"
	keepFailed := strings.TrimSpace(os.Getenv("ANXI_REAL_NEW_GAME_KEEP_FAILED")) == "1"
	t.Logf("real new-game isolated project=%s dataDir=%s", project, dataDir)
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
			t.Logf("retaining failed real new-game fixture for diagnosis: project=%s dataDir=%s volumes=%s,%s", project, dataDir, gameVolume, steamVolume)
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
	client := &realNewGameCountingDocker{Client: paneldocker.NewClient(paneldocker.Options{DockerPath: "docker"})}
	manager := jobs.NewManager(store, slog.Default())
	driver := New(client, slog.Default(), manager, store, "0.4.12")
	waitForJob := func(jobID string, timeout time.Duration) storage.Job {
		t.Helper()
		var current storage.Job
		for deadline := time.Now().Add(timeout); time.Now().Before(deadline); {
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
	stopAndWait := func(label string) {
		t.Helper()
		if err := driver.Stop(ctx, instance); err != nil {
			t.Fatal(err)
		}
		for deadline := time.Now().Add(2 * time.Minute); time.Now().Before(deadline); {
			current, getErr := store.GetInstance(ctx, instance.ID)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if current.DriverPhase == "stopped" {
				instance.State = current.State
				instance.DriverPhase = current.DriverPhase
				instance.DriverPayload = current.DriverPayload
				break
			}
			time.Sleep(250 * time.Millisecond)
		}
		current, getErr := store.GetInstance(ctx, instance.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if current.DriverPhase != "stopped" {
			t.Fatalf("%s stack did not stop cleanly: state=%s phase=%s", label, current.State, current.DriverPhase)
		}
		instance.State = current.State
		instance.DriverPhase = current.DriverPhase
		instance.DriverPayload = current.DriverPayload
	}
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

	skin, hair, accessory := 3, 14, 7
	config := registry.NewGameConfig{
		FarmName: "Release Gate", FarmType: "standard", StartingCabins: 1, MaxPlayers: 4,
		CabinLayout: "nearby", CabinMode: "recommended", ProfitMargin: "100", MoneyMode: "shared",
		FarmerName: "GateHost", FavoriteThing: "Reliable releases", Gender: "female", PetType: "Cat",
		Skin: &skin, Hair: &hair, Shirt: "1001", Pants: "1", Accessory: &accessory,
		EyeColor: &registry.RgbColor{R: 11, G: 22, B: 33}, HairColor: &registry.RgbColor{R: 44, G: 55, B: 66},
		PantsColor: &registry.RgbColor{R: 77, G: 88, B: 99},
	}
	const requestID = "real-new-game-release-gate"
	job, err := driver.Start(ctx, registry.StartRequest{
		Instance: instance, ActorID: 0, NewGame: true, NewGameConfig: &config, RequestID: requestID,
	})
	if err != nil {
		t.Fatal(err)
	}
	finished := waitForJob(job.ID, 40*time.Minute)
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
		if strings.Contains(line.Message, "已在持久 owner 保护下同步 SMAPI 内置支持 Mod") {
			materializedSequence = line.Sequence
		}
		if strings.Contains(line.Message, "新建存档事务已准备") {
			transactionSequence = line.Sequence
		}
	}
	if materializedSequence == 0 || transactionSequence == 0 || materializedSequence >= transactionSequence {
		t.Fatalf("SMAPI materialization was not recorded inside the owner before config preparation: materialized=%d transaction=%d", materializedSequence, transactionSequence)
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
	if len(saves) != 1 || saves[0].ParseError != "" || saves[0].FarmerName != config.FarmerName || saves[0].FarmName != config.FarmName || !saves[0].IsActive {
		t.Fatalf("real first save integrity mismatch: %+v", saves)
	}
	if active := GetActiveSaveName(dataDir); active == "" || active != saves[0].Name {
		t.Fatalf("active save=%q, created save=%q", active, saves[0].Name)
	}
	record, err := findNewGameTransactionByRequest(dataDir, requestID)
	if err != nil || record == nil {
		t.Fatalf("load persistent real new-game transaction: record=%#v err=%v", record, err)
	}
	if record.Stage != newGameStateSuccess || record.Result != "success" || record.CreationWriter != newGameCreationWriterStartup ||
		record.CommandCalled || record.CandidateSave != saves[0].Name || record.SaveLoadedAt == nil ||
		record.CustomizationVerifiedAt == nil || record.DurableSaveCommandID == "" || record.DurableGameLoopSavedAt == nil ||
		record.DiskVerifiedAt == nil || !isValidNewGameSHA256(record.MainSaveSHA256) || !isValidNewGameSHA256(record.SaveGameInfoSHA256) {
		t.Fatalf("real new-game durability transaction incomplete: %#v", record)
	}
	outcome, err := GetCommandOutcome(dataDir, record.DurableSaveCommandID)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateNewGameDurableSaveOutcome(outcome, record.TransactionID, record.CandidateSave, *record.SaveLoadedAt); err != nil {
		t.Fatalf("real same-ID GameLoop.Saved outcome invalid: %v; outcome=%#v", err, outcome)
	}
	if _, err := LoadNewGameOwner(dataDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("terminal real new-game owner was not released: %v", err)
	}
	if _, err := os.Stat(newGamePendingPath(dataDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("terminal real new-game marker was not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(controlDir(dataDir), "pending-save-command.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("terminal real new-game save journal was not removed: %v", err)
	}
	if got := client.newGamePosts.Load(); got != 0 {
		t.Fatalf("startup writer /newgame POST count=%d, want 0", got)
	}
	oldSaveName := saves[0].Name
	stopAndWait("real first-save")
	oldMainPath := filepath.Join(savesDir(dataDir), "Saves", oldSaveName, oldSaveName)
	oldInfoPath := filepath.Join(savesDir(dataDir), "Saves", oldSaveName, "SaveGameInfo")
	oldMainHash, err := stableFileSHA256(oldMainPath)
	if err != nil {
		t.Fatal(err)
	}
	oldInfoHash, err := stableFileSHA256(oldInfoPath)
	if err != nil {
		t.Fatal(err)
	}

	secondSkin, secondHair, secondAccessory := 5, 19, 2
	secondConfig := config
	secondConfig.FarmName = "HTTP Release Gate"
	secondConfig.FarmerName = "HTTPGateHost"
	secondConfig.FavoriteThing = "Exactly once"
	secondConfig.Gender = "male"
	secondConfig.PetType = "Dog"
	secondConfig.Skin = &secondSkin
	secondConfig.Hair = &secondHair
	secondConfig.Shirt = "1003"
	secondConfig.Pants = "2"
	secondConfig.Accessory = &secondAccessory
	secondConfig.EyeColor = &registry.RgbColor{R: 12, G: 23, B: 34}
	secondConfig.HairColor = &registry.RgbColor{R: 45, G: 56, B: 67}
	secondConfig.PantsColor = &registry.RgbColor{R: 78, G: 89, B: 100}
	const secondRequestID = "real-new-game-http-writer-gate"
	secondJob, err := driver.Start(ctx, registry.StartRequest{
		Instance: instance, ActorID: 0, NewGame: true, NewGameConfig: &secondConfig, RequestID: secondRequestID,
	})
	if err != nil {
		t.Fatal(err)
	}
	secondFinished := waitForJob(secondJob.ID, 40*time.Minute)
	secondLogs, logsErr := store.ListJobLogs(context.Background(), secondJob.ID, 0, 1000)
	if logsErr != nil {
		t.Fatal(logsErr)
	}
	if secondFinished.Status != storage.JobStatusSucceeded {
		start := 0
		if len(secondLogs) > 25 {
			start = len(secondLogs) - 25
		}
		var tail []string
		for _, line := range secondLogs[start:] {
			tail = append(tail, line.Message)
		}
		t.Fatalf("real HTTP-writer job status=%s error=%s logs=%s", secondFinished.Status, secondFinished.ErrorMessage.String, strings.Join(tail, " | "))
	}
	if got := client.newGamePosts.Load(); got != 1 {
		t.Fatalf("HTTP writer /newgame POST count=%d, want exactly 1", got)
	}
	allSaves, err := driver.ListSaves(ctx, instance)
	if err != nil {
		t.Fatal(err)
	}
	if len(allSaves) != 2 {
		t.Fatalf("real HTTP writer save count=%d, want 2: %+v", len(allSaves), allSaves)
	}
	var newSaveName string
	oldFound := false
	for _, save := range allSaves {
		if save.Name == oldSaveName {
			oldFound = true
			continue
		}
		if save.ParseError == "" && save.IsActive && save.FarmerName == secondConfig.FarmerName && save.FarmName == secondConfig.FarmName {
			newSaveName = save.Name
		}
	}
	if !oldFound || newSaveName == "" {
		t.Fatalf("HTTP writer did not preserve old save and activate exact new save: old=%s saves=%+v", oldSaveName, allSaves)
	}
	newOldMainHash, err := stableFileSHA256(oldMainPath)
	if err != nil {
		t.Fatal(err)
	}
	newOldInfoHash, err := stableFileSHA256(oldInfoPath)
	if err != nil {
		t.Fatal(err)
	}
	if newOldMainHash != oldMainHash || newOldInfoHash != oldInfoHash {
		t.Fatalf("HTTP writer changed old save bytes: main %s -> %s info %s -> %s", oldMainHash, newOldMainHash, oldInfoHash, newOldInfoHash)
	}
	secondRecord, err := findNewGameTransactionByRequest(dataDir, secondRequestID)
	if err != nil || secondRecord == nil {
		t.Fatalf("load persistent HTTP-writer transaction: record=%#v err=%v", secondRecord, err)
	}
	if secondRecord.Stage != newGameStateSuccess || secondRecord.Result != "success" || secondRecord.CreationWriter != newGameCreationWriterHTTP ||
		!secondRecord.CommandCalled || secondRecord.CandidateSave != newSaveName || secondRecord.SaveLoadedAt == nil ||
		secondRecord.CustomizationVerifiedAt == nil || secondRecord.DurableSaveCommandID == "" || secondRecord.DurableGameLoopSavedAt == nil ||
		secondRecord.DiskVerifiedAt == nil || !isValidNewGameSHA256(secondRecord.MainSaveSHA256) || !isValidNewGameSHA256(secondRecord.SaveGameInfoSHA256) {
		t.Fatalf("real HTTP-writer durability transaction incomplete: %#v", secondRecord)
	}
	secondOutcome, err := GetCommandOutcome(dataDir, secondRecord.DurableSaveCommandID)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateNewGameDurableSaveOutcome(secondOutcome, secondRecord.TransactionID, secondRecord.CandidateSave, *secondRecord.SaveLoadedAt); err != nil {
		t.Fatalf("real HTTP-writer same-ID GameLoop.Saved outcome invalid: %v; outcome=%#v", err, secondOutcome)
	}
	if _, err := LoadNewGameOwner(dataDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("terminal HTTP-writer owner was not released: %v", err)
	}
	if _, err := os.Stat(newGamePendingPath(dataDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("terminal HTTP-writer marker was not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(controlDir(dataDir), "pending-save-command.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("terminal HTTP-writer save journal was not removed: %v", err)
	}
	stopAndWait("real HTTP-writer")
	if output, psErr := exec.CommandContext(ctx, "docker", "ps", "-aq", "--filter", "label=com.docker.compose.project="+project).CombinedOutput(); psErr != nil || strings.TrimSpace(string(output)) != "" {
		t.Fatalf("real two-writer project left containers after Stop: %v %s", psErr, output)
	}
	t.Logf("real startup + HTTP saves created: project=%s startup=%s http=%s oldHashes=%s/%s materializedSequence=%d transactionSequence=%d", project, oldSaveName, newSaveName, oldMainHash, oldInfoHash, materializedSequence, transactionSequence)
}
