//go:build integration

package stardew_junimo

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

type realHostBedControlStatus struct {
	SaveID  string `json:"saveId"`
	HostBed struct {
		State             string `json:"state"`
		Healthy           bool   `json:"healthy"`
		ErrorCode         string `json:"errorCode"`
		HouseUpgradeLevel int    `json:"houseUpgradeLevel"`
		ExpectedBedType   string `json:"expectedBedType"`
		ActualBedType     string `json:"actualBedType"`
		BedTileX          *int   `json:"bedTileX"`
		BedTileY          *int   `json:"bedTileY"`
		PlayerBedSpotX    *int   `json:"playerBedSpotX"`
		PlayerBedSpotY    *int   `json:"playerBedSpotY"`
		FurnitureCount    int    `json:"furnitureCount"`
		BedCount          int    `json:"bedCount"`
		LayoutSource      string `json:"layoutSource"`
		RepairAttempted   bool   `json:"repairAttempted"`
		Repaired          bool   `json:"repaired"`
	} `json:"hostBed"`
	HostControl struct {
		Mode                 string `json:"mode"`
		AutomationKnown      bool   `json:"automationKnown"`
		AutomationEnabled    bool   `json:"automationEnabled"`
		ManualControl        bool   `json:"manualControl"`
		Paused               bool   `json:"paused"`
		PauseReason          string `json:"pauseReason"`
		HostVisible          bool   `json:"hostVisible"`
		DisplayFarmer        bool   `json:"displayFarmer"`
		FarmerHidden         bool   `json:"farmerHidden"`
		VisibilityConsistent bool   `json:"visibilityConsistent"`
		ConnectedClients     *int   `json:"connectedClients"`
	} `json:"hostControl"`
}

type realHostBedPlayer struct {
	IsHost       bool   `json:"isHost"`
	LocationName string `json:"locationName"`
	TileX        *int   `json:"tileX"`
	TileY        *int   `json:"tileY"`
	PixelX       *int   `json:"pixelX"`
	PixelY       *int   `json:"pixelY"`
}

type realHostBedDiagnostics struct {
	DayOfMonth              int      `json:"dayOfMonth"`
	Season                  string   `json:"season"`
	Year                    int      `json:"year"`
	OnlineFarmerCount       int      `json:"onlineFarmerCount"`
	MasterName              string   `json:"masterName"`
	FarmHouseFurnitureCount int      `json:"farmHouseFurnitureCount"`
	SaveImportFinalizeCount int      `json:"saveImportFinalizeCount"`
	FailedFields            []string `json:"failedFields"`
	FarmhandData            []struct {
		Name         string `json:"name"`
		IsCustomized bool   `json:"isCustomized"`
		HasUserID    bool   `json:"hasUserId"`
	} `json:"farmhandData"`
}

// TestRealSwapHostRepairsBedManualControlAndSleepsOptIn exercises the full
// supported JunimoServer runtime. It creates a real game save, previews a ZIP
// whose directory lacks the runtime identity suffix, imports it via
// swap_host_to, proves the map-derived bed survives save/restart, selects the
// demoted original host with the official Junimo test client, sends F9/F10 over
// raw VNC, and sleeps into the next day.
func TestRealSwapHostRepairsBedManualControlAndSleepsOptIn(t *testing.T) {
	sourceGameVolume := strings.TrimSpace(os.Getenv("ANXI_REAL_HOST_BED_SOURCE_GAME_VOLUME"))
	testClientModVolume := strings.TrimSpace(os.Getenv("ANXI_REAL_HOST_BED_TEST_CLIENT_MOD_VOLUME"))
	junimoSourceRoot := strings.TrimSpace(os.Getenv("ANXI_REAL_HOST_BED_JUNIMO_SOURCE_ROOT"))
	if sourceGameVolume == "" || testClientModVolume == "" || junimoSourceRoot == "" {
		t.Skip("set ANXI_REAL_HOST_BED_SOURCE_GAME_VOLUME, ANXI_REAL_HOST_BED_TEST_CLIENT_MOD_VOLUME, and ANXI_REAL_HOST_BED_JUNIMO_SOURCE_ROOT")
	}
	clientStartApp := filepath.Join(junimoSourceRoot, "docker", "rootfs-test-client", "startapp.sh")
	clientSMAPIConfig := filepath.Join(junimoSourceRoot, "docker", "rootfs-test-client", "data", "smapi-config.json")
	for _, required := range []string{clientStartApp, clientSMAPIConfig} {
		if info, err := os.Stat(required); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("required Junimo test-client asset unavailable: %s: %v", required, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
	defer cancel()
	suffix := strings.ToLower(strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", ""))
	project := "anxihostbed" + suffix
	dataDir := filepath.Join(os.TempDir(), project)
	gameVolume := project + "_game-data"
	steamVolume := project + "_steam-session"
	clientModsVolume := project + "_client-mods"
	clientName := project + "-client"
	serverName := project + "-server-1"
	keepFailed := strings.TrimSpace(os.Getenv("ANXI_REAL_HOST_BED_KEEP_FAILED")) == "1"
	serverImage := strings.TrimSpace(os.Getenv("ANXI_REAL_HOST_BED_SERVER_IMAGE"))
	if serverImage == "" {
		serverImage = "sdvd/server:1.5.0-preview.125"
	}
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
			t.Logf("retaining failed host-bed E2E: project=%s dataDir=%s client=%s volumes=%s,%s,%s", project, dataDir, clientName, gameVolume, steamVolume, clientModsVolume)
			return
		}
		_ = exec.Command("docker", "rm", "-f", "-v", clientName).Run()
		_ = exec.Command("docker", "compose", "--project-name", project, "--project-directory", dataDir, "down", "--volumes", "--remove-orphans").Run()
		_ = exec.Command("docker", "volume", "rm", "-f", gameVolume, steamVolume, clientModsVolume).Run()
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
		ID: "stardew", DriverID: DriverID, Name: "host bed real E2E", DataDir: dataDir,
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
	driver := New(client, slog.Default(), manager, store, "host-bed-real-e2e")
	if err := driver.Prepare(ctx, instance); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(dataDir, ".env"), map[string]string{
		"GAME_DATA_VOLUME":     gameVolume,
		"GAME_PORT":            "0",
		"QUERY_PORT":           "0",
		"VNC_PORT":             reserveIntegrationTCPPort(t),
		"API_PORT":             reserveIntegrationTCPPort(t),
		"VNC_PASSWORD":         "",
		"ALLOW_INSECURE_SETUP": "true",
		"SERVER_FPS":           "30",
		"STEAM_USERNAME":       "",
		"STEAM_PASSWORD":       "",
		"STEAM_REFRESH_TOKEN":  "",
	}); err != nil {
		t.Fatal(err)
	}
	run("volume", "create", "--label", "com.openai.codex.owner=anxi-host-bed-gate", gameVolume)
	run("volume", "create", "--label", "com.openai.codex.owner=anxi-host-bed-gate", steamVolume)
	run("volume", "create", "--label", "com.openai.codex.owner=anxi-host-bed-gate", clientModsVolume)
	run("run", "--rm", "--network", "none",
		"--mount", "type=volume,src="+sourceGameVolume+",dst=/source,readonly",
		"--mount", "type=volume,src="+gameVolume+",dst=/target",
		"alpine:3.20", "sh", "-c", "cd /source && tar cf - . | tar xf - -C /target")
	run("run", "--rm", "--network", "none",
		"--mount", "type=volume,src="+testClientModVolume+",dst=/source,readonly",
		"--mount", "type=volume,src="+clientModsVolume+",dst=/target",
		"alpine:3.20", "sh", "-c", "mkdir -p /target/JunimoTestClient && cp -a /source/. /target/JunimoTestClient/")

	stored, err = store.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{
		ID: stored.ID, State: storage.InstanceStateSaveRequired, StateMessage: "create host-bed source", DriverPhase: "save_required", DriverPayload: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	instance.State, instance.DriverPhase, instance.DriverPayload = stored.State, stored.DriverPhase, stored.DriverPayload
	skin, hair, accessory := 3, 14, 7
	newGame := registry.NewGameConfig{
		FarmName: "Host Bed Gate", FarmType: "standard", StartingCabins: 2, MaxPlayers: 4,
		CabinLayout: "nearby", CabinMode: "recommended", ProfitMargin: "100", MoneyMode: "shared",
		FarmerName: "OriginalOwner", FavoriteThing: "Consistent saves", Gender: "female", PetType: "Cat",
		Skin: &skin, Hair: &hair, Shirt: "1001", Pants: "1", Accessory: &accessory,
	}
	createJob, err := driver.Start(ctx, registry.StartRequest{
		Instance: instance, ActorID: 0, NewGame: true, NewGameConfig: &newGame, RequestID: "host-bed-real-source",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRealHostBedJobSucceeded(t, ctx, store, createJob.ID, 40*time.Minute)
	instance = refreshRealHostBedInstance(t, ctx, store, instance)
	saves, err := driver.ListSaves(ctx, instance)
	if err != nil || len(saves) != 1 || saves[0].ParseError != "" {
		t.Fatalf("real source save mismatch: saves=%+v err=%v", saves, err)
	}
	saveName := saves[0].Name
	sourceStatus := waitRealHostBedControl(t, dataDir, 2*time.Minute, func(status realHostBedControlStatus) bool {
		return status.SaveID == saveName && status.HostBed.Healthy && status.HostBed.BedCount == 1
	})
	if sourceStatus.HostBed.Repaired || sourceStatus.HostBed.HouseUpgradeLevel != 0 || sourceStatus.HostBed.ActualBedType != "Single" {
		t.Fatalf("official source bed unexpectedly repaired or invalid: %+v", sourceStatus.HostBed)
	}

	stopRealHostBedInstance(t, ctx, driver, store, &instance)
	canonicalCloneRoot := filepath.Join(dataDir, "host-bed-canonical-clone")
	if err := os.MkdirAll(canonicalCloneRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	sourceSaveDir := filepath.Join(savesDir(dataDir), "Saves", saveName)
	sourceMainHash, err := stableFileSHA256(filepath.Join(sourceSaveDir, saveName))
	if err != nil {
		t.Fatal(err)
	}
	importedSaveName, err := cloneRealHostBedSave(sourceSaveDir, canonicalCloneRoot, saveName)
	if err != nil {
		t.Fatalf("clone real source into an independent upload save: %v", err)
	}
	canonicalCloneDir := filepath.Join(canonicalCloneRoot, importedSaveName)
	canonicalCloneHash, err := stableFileSHA256(filepath.Join(canonicalCloneDir, importedSaveName))
	if err != nil {
		t.Fatal(err)
	}
	noncanonicalName := strings.SplitN(importedSaveName, "_", 2)[0]
	if noncanonicalName == "" || noncanonicalName == importedSaveName {
		t.Fatalf("independent save %q cannot produce a non-canonical upload identity", importedSaveName)
	}
	noncanonicalZip := filepath.Join(dataDir, "host-bed-noncanonical.zip")
	if err := writeNoncanonicalRealHostBedArchive(canonicalCloneDir, importedSaveName, noncanonicalName, noncanonicalZip); err != nil {
		t.Fatalf("write non-canonical real save ZIP: %v", err)
	}
	previewName, preview, stagedRoot, err := PreviewSaveZip(noncanonicalZip, filepath.Base(noncanonicalZip))
	if err != nil {
		t.Fatalf("preview non-canonical real save ZIP: %v", err)
	}
	defer os.RemoveAll(stagedRoot)
	if previewName != importedSaveName || preview.Name != importedSaveName {
		t.Fatalf("real preview did not normalize runtime identity: name=%q preview=%q want=%q", previewName, preview.Name, importedSaveName)
	}
	if _, err := os.Stat(filepath.Join(stagedRoot, noncanonicalName)); !os.IsNotExist(err) {
		t.Fatalf("real preview retained non-canonical source directory %q: %v", noncanonicalName, err)
	}
	previewMainHash, err := stableFileSHA256(filepath.Join(stagedRoot, importedSaveName, importedSaveName))
	if err != nil || previewMainHash != canonicalCloneHash {
		t.Fatalf("real preview changed canonicalized main save: before=%s after=%s err=%v", canonicalCloneHash, previewMainHash, err)
	}
	operationID := NewImportOperationID()
	importJob, err := driver.ImportSaveAndStart(ctx, registry.SaveImportRequest{
		Instance: instance, ActorID: 0, OperationID: operationID, StagedDir: stagedRoot,
		SaveName: importedSaveName, HostHandling: "swap_host_to", PlatformID: "76561190000000001",
		AttachJobIdentity: func(string) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRealHostBedJobSucceeded(t, ctx, store, importJob.ID, 30*time.Minute)
	instance = refreshRealHostBedInstance(t, ctx, store, instance)
	journal, err := LoadImportJournal(dataDir, operationID)
	if err != nil {
		t.Fatal(err)
	}
	if journal.Stage != ImportStageCompleted || journal.ActivationEvidence == nil || journal.ActivationEvidence.HostBed == nil || !journal.ActivationEvidence.HostBed.Repaired {
		t.Fatalf("swap activation did not retain repaired-bed evidence: %+v", journal)
	}
	repaired := waitRealHostBedControl(t, dataDir, 2*time.Minute, func(status realHostBedControlStatus) bool {
		return status.SaveID == importedSaveName && status.HostBed.Healthy && status.HostBed.Repaired && status.HostBed.BedCount == 1
	})
	assertRealHostBedShape(t, repaired, "repaired")
	diagnostics := readRealHostBedDiagnostics(t, ctx, client, dataDir)
	if diagnostics.MasterName != "Server" || diagnostics.SaveImportFinalizeCount != 1 || diagnostics.FarmHouseFurnitureCount != 1 || len(diagnostics.FailedFields) != 0 {
		t.Fatalf("swap diagnostics mismatch: %+v", diagnostics)
	}
	formerOwnerFound := false
	for _, farmhand := range diagnostics.FarmhandData {
		if farmhand.Name == newGame.FarmerName && farmhand.IsCustomized && !farmhand.HasUserID {
			formerOwnerFound = true
		}
	}
	if !formerOwnerFound {
		t.Fatalf("demoted original owner data was not preserved and durably unbound: %+v", diagnostics.FarmhandData)
	}
	if !journal.FarmhandUnbindVerified || journal.FarmhandCount != len(diagnostics.FarmhandData) || journal.CustomizedFarmhandCount < 1 {
		t.Fatalf("durable farmhand-unbind evidence mismatch: journal=%+v diagnostics=%+v", journal, diagnostics.FarmhandData)
	}
	unchangedSourceHash, err := stableFileSHA256(filepath.Join(sourceSaveDir, saveName))
	if err != nil || unchangedSourceHash != sourceMainHash {
		t.Fatalf("non-target source save changed during import: before=%s after=%s err=%v", sourceMainHash, unchangedSourceHash, err)
	}

	saveCommand, err := driver.RequestSaveNow(ctx, instance)
	if err != nil {
		t.Fatal(err)
	}
	waitRealHostBedSaveCommand(t, dataDir, saveCommand.CommandID, 3*time.Minute)
	repairedTileX, repairedTileY := *repaired.HostBed.BedTileX, *repaired.HostBed.BedTileY
	stopRealHostBedInstance(t, ctx, driver, store, &instance)
	restartJob, err := driver.Start(ctx, registry.StartRequest{Instance: instance, ActorID: 0})
	if err != nil {
		t.Fatal(err)
	}
	assertRealHostBedJobSucceeded(t, ctx, store, restartJob.ID, 20*time.Minute)
	instance = refreshRealHostBedInstance(t, ctx, store, instance)
	afterRestart := waitRealHostBedControl(t, dataDir, 2*time.Minute, func(status realHostBedControlStatus) bool {
		return status.SaveID == importedSaveName && status.HostBed.Healthy && status.HostBed.BedCount == 1 && !status.HostBed.Repaired
	})
	assertRealHostBedShape(t, afterRestart, "healthy")
	if *afterRestart.HostBed.BedTileX != repairedTileX || *afterRestart.HostBed.BedTileY != repairedTileY {
		t.Fatalf("bed moved across save/restart: before=(%d,%d) after=(%d,%d)", repairedTileX, repairedTileY, *afterRestart.HostBed.BedTileX, *afterRestart.HostBed.BedTileY)
	}

	if _, exitCode, stderr, err := sendServerCommand(ctx, client, dataDir, "debug warp Farm 64 15"); err != nil || exitCode != 0 {
		t.Fatalf("warp host for manual input gate: exit=%d stderr=%s err=%v", exitCode, stderr, err)
	}
	waitRealHostBedPlayer(t, dataDir, 30*time.Second, func(player realHostBedPlayer) bool {
		return player.LocationName == "Farm"
	})
	sendRealVNCKey(t, ctx, serverName, 0xffc6, 100*time.Millisecond) // F9
	waitRealHostBedControl(t, dataDir, 30*time.Second, func(status realHostBedControlStatus) bool {
		return status.HostControl.AutomationKnown && !status.HostControl.AutomationEnabled && status.HostControl.ManualControl &&
			!status.HostControl.Paused && status.HostControl.PauseReason == "ManualControl" &&
			status.HostControl.HostVisible && status.HostControl.DisplayFarmer && !status.HostControl.FarmerHidden && status.HostControl.VisibilityConsistent &&
			status.HostControl.ConnectedClients != nil && *status.HostControl.ConnectedClients == 0
	})
	beforeMove := waitRealHostBedPlayer(t, dataDir, 15*time.Second, func(player realHostBedPlayer) bool {
		return player.PixelX != nil && player.PixelY != nil
	})
	currentPosition := beforeMove
	moved := false
	for _, key := range []uint32{0x77, 0x64, 0x73, 0x61} { // W, D, S, A
		sendRealVNCKey(t, ctx, serverName, key, 2500*time.Millisecond)
		time.Sleep(750 * time.Millisecond)
		afterInput := waitRealHostBedPlayer(t, dataDir, 3*time.Second, func(player realHostBedPlayer) bool {
			return player.PixelX != nil && player.PixelY != nil
		})
		if *afterInput.PixelX != *currentPosition.PixelX || *afterInput.PixelY != *currentPosition.PixelY {
			moved = true
			break
		}
		currentPosition = afterInput
	}
	if !moved {
		t.Fatalf("W/D/S/A delivered over VNC did not move the visible host in manual mode from (%d,%d)", *beforeMove.PixelX, *beforeMove.PixelY)
	}
	sendRealVNCKey(t, ctx, serverName, 0xffc6, 100*time.Millisecond) // F9 restore automation
	automatic := waitRealHostBedControl(t, dataDir, 30*time.Second, func(status realHostBedControlStatus) bool {
		return status.HostControl.AutomationEnabled && !status.HostControl.ManualControl && status.HostControl.Paused && status.HostControl.PauseReason == "NoConnectedClients"
	})
	visible := automatic.HostControl.HostVisible
	for i := 0; i < 4; i++ {
		visible = !visible
		sendRealVNCKey(t, ctx, serverName, 0xffc7, 100*time.Millisecond) // F10
		currentVisible := visible
		status := waitRealHostBedControl(t, dataDir, 30*time.Second, func(status realHostBedControlStatus) bool {
			return status.HostControl.HostVisible == currentVisible && status.HostControl.VisibilityConsistent
		})
		if status.HostControl.DisplayFarmer != currentVisible || status.HostControl.FarmerHidden == currentVisible {
			t.Fatalf("F10 produced a sprite/shadow half-state: %+v", status.HostControl)
		}
	}

	run("run", "-d", "--name", clientName,
		"--label", "com.openai.codex.owner=anxi-host-bed-gate",
		"--network", project+"_default",
		"-e", "APP_NAME=Junimo Host Bed Test Client",
		"-e", "CLIENT_FPS=30",
		"-e", "JUNIMO_TEST_PORT=5123",
		"-e", "SMAPI_VERSION=4.5.2",
		"-e", "VNC_PASSWORD=host-bed-client",
		"--mount", "type=volume,src="+gameVolume+",dst=/data/game",
		"--mount", "type=volume,src="+clientModsVolume+",dst=/data/Mods",
		"--mount", "type=bind,src="+clientStartApp+",dst=/startapp.sh,readonly",
		"--mount", "type=bind,src="+clientSMAPIConfig+",dst=/data/smapi-config.json,readonly",
		serverImage)
	waitRealHostBedClientReady(t, ctx, clientName, 3*time.Minute)
	assertRealHostBedClientSuccess(t, runRealHostBedClientRequest(t, ctx, clientName, "POST", "/connect/lan", `{"address":"server:24642"}`), "connect LAN")
	assertRealHostBedClientSuccess(t, runRealHostBedClientRequest(t, ctx, clientName, "GET", "/wait/farmhands?timeout=60000", ""), "wait farmhands")
	var slots struct {
		Farmhands []struct {
			Index        int    `json:"index"`
			Name         string `json:"name"`
			IsCustomized bool   `json:"isCustomized"`
		} `json:"farmhands"`
	}
	if err := json.Unmarshal(runRealHostBedClientRequest(t, ctx, clientName, "GET", "/farmhands", ""), &slots); err != nil {
		t.Fatal(err)
	}
	slotIndex := -1
	for _, slot := range slots.Farmhands {
		if slot.Name == newGame.FarmerName && slot.IsCustomized {
			slotIndex = slot.Index
			break
		}
	}
	if slotIndex < 0 {
		t.Fatalf("demoted original host was not selectable after real import/restart: %+v", slots.Farmhands)
	}
	assertRealHostBedClientSuccess(t, runRealHostBedClientRequest(t, ctx, clientName, "POST", "/farmhands/select", fmt.Sprintf(`{"slotIndex":%d}`, slotIndex)), "select demoted original host")
	assertRealHostBedClientSuccess(t, runRealHostBedClientRequest(t, ctx, clientName, "GET", "/wait/world-ready?timeout=90000", ""), "wait client world")
	waitRealHostBedControl(t, dataDir, time.Minute, func(status realHostBedControlStatus) bool {
		return status.HostControl.ConnectedClients != nil && *status.HostControl.ConnectedClients > 0 && !status.HostControl.Paused
	})

	beforeDay := readRealHostBedDiagnostics(t, ctx, client, dataDir)
	logOffset := readServerLogSize(ctx, client, dataDir)
	assertRealHostBedClientSuccess(t, runRealHostBedClientRequest(t, ctx, clientName, "POST", "/actions/sleep", `{}`), "sleep farmhand")
	afterDay := waitRealHostBedNextDay(t, ctx, client, dataDir, beforeDay, 5*time.Minute)
	assertRealHostBedClientSuccess(t, runRealHostBedClientRequest(t, ctx, clientName, "GET", "/wait/world-ready?timeout=90000", ""), "wait next-day client world")
	afterDayStatus := waitRealHostBedControl(t, dataDir, time.Minute, func(status realHostBedControlStatus) bool {
		return status.HostBed.Healthy && status.HostBed.BedCount == 1 && status.HostControl.VisibilityConsistent
	})
	assertRealHostBedShape(t, afterDayStatus, afterDayStatus.HostBed.State)
	dayLog := readServerLogSince(ctx, client, dataDir, logOffset)
	if count := strings.Count(dayLog, "Host is home, sleeping in place (FarmHouse)"); count > 1 {
		t.Fatalf("host sleep fallback repeated %d times before day transition: %s", count, dayLog)
	}
	for _, forbidden := range []string{"host_bed_missing", "Forcing day", "force ending day", "sleep timeout"} {
		if strings.Contains(strings.ToLower(dayLog), strings.ToLower(forbidden)) {
			t.Fatalf("sleep path hit forbidden fallback %q: %s", forbidden, dayLog)
		}
	}
	if beforeDay.DayOfMonth == afterDay.DayOfMonth && beforeDay.Season == afterDay.Season && beforeDay.Year == afterDay.Year {
		t.Fatalf("real sleep did not enter the next day: before=%+v after=%+v", beforeDay, afterDay)
	}

	run("rm", "-f", "-v", clientName)
	stopRealHostBedInstance(t, ctx, driver, store, &instance)
	t.Logf("real non-canonical host-swap E2E passed: source=%s upload=%s canonical=%s original_host_selectable=true bed=(%d,%d) day=%s %d Y%d -> %s %d Y%d", saveName, noncanonicalName, importedSaveName, repairedTileX, repairedTileY, beforeDay.Season, beforeDay.DayOfMonth, beforeDay.Year, afterDay.Season, afterDay.DayOfMonth, afterDay.Year)
}

func assertRealHostBedShape(t *testing.T, status realHostBedControlStatus, expectedState string) {
	t.Helper()
	bed := status.HostBed
	if !bed.Healthy || bed.ErrorCode != "" || bed.State != expectedState || bed.HouseUpgradeLevel != 0 ||
		bed.ExpectedBedType != "Single" || bed.ActualBedType != "Single" || bed.BedCount != 1 ||
		bed.BedTileX == nil || bed.BedTileY == nil || bed.PlayerBedSpotX == nil || bed.PlayerBedSpotY == nil ||
		*bed.PlayerBedSpotX != *bed.BedTileX+1 || *bed.PlayerBedSpotY != *bed.BedTileY+1 ||
		bed.LayoutSource != "FarmHouse.Back.DefaultBedPosition" {
		t.Fatalf("invalid map-derived host bed: %+v", bed)
	}
}

func assertRealHostBedJobSucceeded(t *testing.T, ctx context.Context, store *storage.Store, jobID string, timeout time.Duration) {
	t.Helper()
	job := waitForHostHouseJob(t, ctx, store, jobID, timeout)
	if job.Status == storage.JobStatusSucceeded {
		return
	}
	logs, _ := store.ListJobLogs(context.Background(), jobID, 0, 1000)
	start := 0
	if len(logs) > 30 {
		start = len(logs) - 30
	}
	var tail []string
	for _, line := range logs[start:] {
		tail = append(tail, line.Message)
	}
	t.Fatalf("job %s status=%s error=%s logs=%s", jobID, job.Status, job.ErrorMessage.String, strings.Join(tail, " | "))
}

func refreshRealHostBedInstance(t *testing.T, ctx context.Context, store *storage.Store, instance registry.Instance) registry.Instance {
	t.Helper()
	stored, err := store.GetInstance(ctx, instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	instance.State = stored.State
	instance.StateMessage = stored.StateMessage.String
	instance.DriverPhase = stored.DriverPhase
	instance.DriverPayload = stored.DriverPayload
	return instance
}

func stopRealHostBedInstance(t *testing.T, ctx context.Context, driver *Driver, store *storage.Store, instance *registry.Instance) {
	t.Helper()
	if err := driver.Stop(ctx, *instance); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(2 * time.Minute); time.Now().Before(deadline); {
		*instance = refreshRealHostBedInstance(t, ctx, store, *instance)
		if instance.DriverPhase == "stopped" {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("instance did not stop: state=%s phase=%s", instance.State, instance.DriverPhase)
}

func waitRealHostBedControl(t *testing.T, dataDir string, timeout time.Duration, predicate func(realHostBedControlStatus) bool) realHostBedControlStatus {
	t.Helper()
	var status realHostBedControlStatus
	var lastErr error
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); {
		raw, err := os.ReadFile(filepath.Join(controlDir(dataDir), "status.json"))
		if err == nil {
			err = json.Unmarshal(raw, &status)
		}
		if err == nil && predicate(status) {
			return status
		}
		lastErr = err
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("Control status predicate timed out after %s: status=%+v err=%v", timeout, status, lastErr)
	return status
}

func waitRealHostBedPlayer(t *testing.T, dataDir string, timeout time.Duration, predicate func(realHostBedPlayer) bool) realHostBedPlayer {
	t.Helper()
	var host realHostBedPlayer
	var lastErr error
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); {
		var players struct {
			Players []realHostBedPlayer `json:"players"`
		}
		raw, err := os.ReadFile(filepath.Join(controlDir(dataDir), "players.json"))
		if err == nil {
			err = json.Unmarshal(raw, &players)
		}
		if err == nil {
			for _, player := range players.Players {
				if player.IsHost {
					host = player
					if predicate(host) {
						return host
					}
				}
			}
		}
		lastErr = err
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("host player predicate timed out after %s: player=%+v err=%v", timeout, host, lastErr)
	return host
}

func waitRealHostBedSaveCommand(t *testing.T, dataDir, commandID string, timeout time.Duration) {
	t.Helper()
	var outcome CommandOutcome
	var err error
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); {
		outcome, err = GetCommandOutcome(dataDir, commandID)
		if err == nil && outcome.Status == CommandStatusSucceeded {
			return
		}
		if err == nil && (outcome.Status == CommandStatusFailed || outcome.Status == CommandStatusExpired) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("save-now did not complete through GameLoop.Saved: outcome=%+v err=%v", outcome, err)
}

func readRealHostBedDiagnostics(t *testing.T, ctx context.Context, executor commandExecutor, dataDir string) realHostBedDiagnostics {
	t.Helper()
	raw, err := readJunimoAPI(ctx, executor, dataDir, "/diagnostics/state")
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics realHostBedDiagnostics
	if err := json.Unmarshal(raw, &diagnostics); err != nil {
		t.Fatal(err)
	}
	return diagnostics
}

func waitRealHostBedNextDay(t *testing.T, ctx context.Context, executor commandExecutor, dataDir string, before realHostBedDiagnostics, timeout time.Duration) realHostBedDiagnostics {
	t.Helper()
	var current realHostBedDiagnostics
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); {
		current = readRealHostBedDiagnostics(t, ctx, executor, dataDir)
		if current.DayOfMonth != before.DayOfMonth || current.Season != before.Season || current.Year != before.Year {
			return current
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("day did not advance within %s: before=%+v current=%+v", timeout, before, current)
	return current
}

func waitRealHostBedClientReady(t *testing.T, ctx context.Context, container string, timeout time.Duration) {
	t.Helper()
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); {
		cmd := exec.CommandContext(ctx, "docker", "exec", container, "curl", "-fsS", "http://localhost:5123/ping")
		if output, err := cmd.CombinedOutput(); err == nil && strings.Contains(string(output), "pong") {
			return
		}
		time.Sleep(time.Second)
	}
	logs, _ := exec.CommandContext(ctx, "docker", "logs", "--tail", "120", container).CombinedOutput()
	t.Fatalf("test client API not ready within %s: %s", timeout, logs)
}

// cloneRealHostBedSave creates a test-only independent save identity. Stardew
// derives Constants.SaveFolderName from farmName + uniqueIDForThisGame after
// loading, so merely renaming a cloned directory would intentionally trip
// JunimoServer's wrong-save finalizer guard.
func cloneRealHostBedSave(sourceDir, stagedRoot, sourceName string) (string, error) {
	separator := strings.LastIndexByte(sourceName, '_')
	if separator <= 0 || separator == len(sourceName)-1 {
		return "", fmt.Errorf("source save name %q has no numeric identity suffix", sourceName)
	}
	sourceIDText := sourceName[separator+1:]
	sourceID, err := strconv.ParseUint(sourceIDText, 10, 64)
	if err != nil {
		return "", fmt.Errorf("parse source save identity: %w", err)
	}
	const identityOffset uint64 = 999983
	if sourceID > ^uint64(0)-identityOffset {
		return "", errors.New("source save identity cannot be safely incremented")
	}
	targetIDText := strconv.FormatUint(sourceID+identityOffset, 10)
	targetName := sourceName[:separator+1] + targetIDText
	targetDir := filepath.Join(stagedRoot, targetName)
	if err := copyHostHouseSaveFixture(sourceDir, targetDir); err != nil {
		return "", err
	}

	sourceMain := filepath.Join(targetDir, sourceName)
	targetMain := filepath.Join(targetDir, targetName)
	if err := os.Rename(sourceMain, targetMain); err != nil {
		return "", fmt.Errorf("rename cloned main save: %w", err)
	}
	if err := restampRealHostBedSaveIdentity(targetMain, sourceIDText, targetIDText, true); err != nil {
		return "", err
	}
	if err := restampRealHostBedSaveIdentity(filepath.Join(targetDir, "SaveGameInfo"), sourceIDText, targetIDText, false); err != nil {
		return "", err
	}
	for _, stale := range []string{sourceName + "_old", "SaveGameInfo_old"} {
		if err := os.Remove(filepath.Join(targetDir, stale)); err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("remove cloned recovery file %s: %w", stale, err)
		}
	}
	return targetName, nil
}

// writeNoncanonicalRealHostBedArchive preserves a genuine save byte-for-byte
// while presenting its top-level directory and primary file under the raw
// pre-SMAPI name. PreviewSaveZip must derive and materialize the canonical
// <raw>_<uniqueIDForThisGame> identity before the durable transaction owns it.
func writeNoncanonicalRealHostBedArchive(sourceDir, canonicalName, noncanonicalName, zipPath string) error {
	if canonicalName == "" || noncanonicalName == "" || canonicalName == noncanonicalName {
		return errors.New("real save archive requires distinct canonical and non-canonical names")
	}
	archiveFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	archive := zip.NewWriter(archiveFile)
	walkErr := filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		mapped := relative
		switch relative {
		case canonicalName:
			mapped = noncanonicalName
		case canonicalName + "_old":
			mapped = noncanonicalName + "_old"
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(filepath.Join(noncanonicalName, mapped))
		header.Method = zip.Deflate
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(writer, source)
		closeErr := source.Close()
		return errors.Join(copyErr, closeErr)
	})
	closeArchiveErr := archive.Close()
	closeFileErr := archiveFile.Close()
	if err := errors.Join(walkErr, closeArchiveErr, closeFileErr); err != nil {
		_ = os.Remove(zipPath)
		return err
	}
	return nil
}

func restampRealHostBedSaveIdentity(path, sourceID, targetID string, required bool) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if !required && os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read cloned save identity file %s: %w", filepath.Base(path), err)
	}
	oldToken := []byte("<uniqueIDForThisGame>" + sourceID + "</uniqueIDForThisGame>")
	count := bytes.Count(raw, oldToken)
	if count == 0 && !required {
		return nil
	}
	if count != 1 {
		return fmt.Errorf("%s contains %d matching save identities, want 1", filepath.Base(path), count)
	}
	newToken := []byte("<uniqueIDForThisGame>" + targetID + "</uniqueIDForThisGame>")
	raw = bytes.Replace(raw, oldToken, newToken, 1)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("write cloned save identity file %s: %w", filepath.Base(path), err)
	}
	return nil
}

func runRealHostBedClientRequest(t *testing.T, ctx context.Context, container, method, path, body string) []byte {
	t.Helper()
	args := []string{"exec", container, "curl", "-fsS", "-X", method, "-H", "Content-Type: application/json"}
	if body != "" {
		args = append(args, "--data", body)
	}
	args = append(args, "http://localhost:5123"+path)
	output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("client %s %s: %v: %s", method, path, err, output)
	}
	return output
}

func assertRealHostBedClientSuccess(t *testing.T, raw []byte, action string) {
	t.Helper()
	var result struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || !result.Success {
		t.Fatalf("%s failed: result=%s err=%v", action, raw, err)
	}
}

func sendRealVNCKey(t *testing.T, ctx context.Context, container string, key uint32, hold time.Duration) {
	t.Helper()
	vncCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	cmd := exec.CommandContext(vncCtx, "docker", "exec", "-i", container, "nc", "-U", "/tmp/vnc.sock")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		t.Fatalf("open VNC stdin for %s: %v", container, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatalf("open VNC stdout for %s: %v", container, err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("connect container VNC socket in %s: %v: %s", container, err, stderr.String())
	}
	defer func() {
		_ = stdin.Close()
		cancel()
		_ = cmd.Wait()
	}()
	protocol := make([]byte, 12)
	if _, err := io.ReadFull(stdout, protocol); err != nil {
		t.Fatalf("read VNC protocol from %s: %v: %s", container, err, stderr.String())
	}
	if !strings.HasPrefix(string(protocol), "RFB 003.") {
		t.Fatalf("unexpected RFB protocol: %q", protocol)
	}
	if _, err := stdin.Write([]byte("RFB 003.008\n")); err != nil {
		t.Fatal(err)
	}
	count := make([]byte, 1)
	if _, err := io.ReadFull(stdout, count); err != nil {
		t.Fatal(err)
	}
	if count[0] == 0 {
		var reasonLength uint32
		if err := binary.Read(stdout, binary.BigEndian, &reasonLength); err != nil {
			t.Fatal(err)
		}
		reason := make([]byte, reasonLength)
		_, _ = io.ReadFull(stdout, reason)
		t.Fatalf("VNC server rejected security negotiation: %s", reason)
	}
	types := make([]byte, int(count[0]))
	if _, err := io.ReadFull(stdout, types); err != nil {
		t.Fatal(err)
	}
	if !containsRealHostBedByte(types, 1) {
		t.Fatalf("raw VNC fixture did not offer None authentication: %v", types)
	}
	if _, err := stdin.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	var securityResult uint32
	if err := binary.Read(stdout, binary.BigEndian, &securityResult); err != nil {
		t.Fatal(err)
	}
	if securityResult != 0 {
		t.Fatalf("VNC security result=%d", securityResult)
	}
	if _, err := stdin.Write([]byte{1}); err != nil { // shared desktop
		t.Fatal(err)
	}
	serverInit := make([]byte, 24)
	if _, err := io.ReadFull(stdout, serverInit); err != nil {
		t.Fatal(err)
	}
	nameLength := binary.BigEndian.Uint32(serverInit[20:24])
	if nameLength > 1<<20 {
		t.Fatalf("invalid VNC desktop name length %d", nameLength)
	}
	if _, err := io.CopyN(io.Discard, stdout, int64(nameLength)); err != nil {
		t.Fatal(err)
	}
	send := func(down bool) {
		message := make([]byte, 8)
		message[0] = 4
		if down {
			message[1] = 1
		}
		binary.BigEndian.PutUint32(message[4:], key)
		if _, err := stdin.Write(message); err != nil {
			t.Fatal(err)
		}
	}
	send(true)
	time.Sleep(hold)
	send(false)
	time.Sleep(50 * time.Millisecond)
}

func containsRealHostBedByte(values []byte, target byte) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
