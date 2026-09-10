//go:build integration

package stardew_junimo

import (
	"archive/zip"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
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

// Lose one response after the real DELETE executed, reproducing the uncertain
// destructive boundary without changing the game or faking its save outcomes.
type farmhandDeleteResponseLoss struct {
	*paneldocker.Client
	lose atomic.Bool
}

func (d *farmhandDeleteResponseLoss) ComposeExecPipe(ctx context.Context, dir, service, input string, args ...string) (paneldocker.CommandResult, error) {
	result, err := d.Client.ComposeExecPipe(ctx, dir, service, input, args...)
	if err == nil && strings.Contains(strings.Join(args, " "), "-X DELETE") {
		body, status, splitErr := splitCurlResponse(result)
		var response junimoFarmhandDeleteResponse
		if splitErr == nil && status == "200" && json.Unmarshal([]byte(body), &response) == nil && response.Success && d.lose.CompareAndSwap(true, false) {
			return paneldocker.CommandResult{}, errors.New("test: DELETE response lost after execution")
		}
	}
	return result, err
}

// Opt-in only: the root must contain independent game/ and client-mods/ copies
// and the pinned upstream test-client startup assets. No production save is used.
func TestRealFarmhandDeleteOptIn(t *testing.T) {
	root := os.Getenv("ANXI_FARMHAND_E2E_ROOT")
	if root == "" {
		t.Skip("set ANXI_FARMHAND_E2E_ROOT to an isolated anxi-farmhand-review-* directory")
	}
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) || !strings.HasPrefix(filepath.Base(root), "anxi-farmhand-review-") {
		t.Fatal("refusing a test root outside the explicit farmhand review boundary")
	}
	project := filepath.Base(root) + "-world"
	dir := filepath.Join(root, project)
	image := os.Getenv("ANXI_FARMHAND_E2E_IMAGE")
	if image == "" {
		image = "sdvd/server:1.5.0-preview.125"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker %v: %v: %s", args, err, out)
		}
		return string(out)
	}
	if run("ps", "-a", "--filter", "label=com.docker.compose.project="+project, "--format", "{{.Names}}") != "" {
		t.Fatal("test project already exists; inspect it before a deliberate retry")
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	volume := project + "_game-data"
	if err := exec.CommandContext(ctx, "docker", "volume", "inspect", volume).Run(); err == nil {
		t.Fatal("test game volume already exists")
	}
	run("volume", "create", "--label", "codex.task="+filepath.Base(root), "--driver", "local", "--opt", "type=none", "--opt", "o=bind", "--opt", "device="+filepath.Join(root, "game"), volume)
	clients := []string{project + "-client-a", project + "-client-b"}
	t.Cleanup(func() {
		if t.Failed() && os.Getenv("ANXI_FARMHAND_E2E_KEEP_FAILED") == "1" {
			t.Logf("retained isolated test project %s at %s", project, dir)
			return
		}
		for _, name := range clients {
			_ = exec.Command("docker", "rm", "-f", "-v", name).Run()
		}
		_ = exec.Command("docker", "compose", "--project-name", project, "--project-directory", dir, "down", "--volumes", "--remove-orphans").Run()
		_ = exec.Command("docker", "volume", "rm", volume).Run()
	})
	s, err := storage.Open(ctx, appconfig.Config{DataDir: dir, DBPath: filepath.Join(dir, "panel-e2e.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); s.Close() }()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	stored, err := s.EnsureDefaultInstance(ctx, storage.EnsureDefaultInstanceParams{ID: "stardew", DriverID: DriverID, Name: "Farmhand review", DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	instance := makeRegistryInstanceFromStorage(stored)
	docker := &farmhandDeleteResponseLoss{Client: paneldocker.NewClient(paneldocker.Options{DockerPath: "docker"})}
	manager := jobs.NewManager(s, slog.Default())
	d := New(docker, slog.Default(), manager, s, "dev")
	if err := d.Prepare(ctx, instance); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.UpdateEnvFile(filepath.Join(dir, ".env"), map[string]string{
		"GAME_DATA_VOLUME": volume, "SERVER_IMAGE": image, "GAME_PORT": "0", "QUERY_PORT": "0",
		"VNC_PORT": reserveIntegrationTCPPort(t), "API_PORT": reserveIntegrationTCPPort(t), "VNC_PASSWORD": "review-only",
		"STEAM_INVITE_ENABLED": "false", "STEAM_INVITE_AUTH_STATE": "disabled", "STEAM_USERNAME": "", "STEAM_PASSWORD": "", "STEAM_REFRESH_TOKEN": "", "STEAM_AUTH_COMPLETED": "",
		"SERVER_PASSWORD": "", "SAP_PLAYER_AUTH_MODE": "none", "SERVER_FPS": "15",
	}); err != nil {
		t.Fatal(err)
	}
	composePath := filepath.Join(dir, "docker-compose.yml")
	raw, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatal(err)
	}
	compose := strings.Replace(string(raw), "  server:\n", "  server:\n    cpus: 0.8\n    mem_limit: 2g\n    labels:\n      codex.task: "+filepath.Base(root)+"\n", 1)
	compose = strings.ReplaceAll(compose, "    cap_add:\n      - SYS_TIME\n", "")
	for _, port := range []string{"GAME_PORT", "QUERY_PORT", "VNC_PORT", "API_PORT"} {
		compose = strings.ReplaceAll(compose, "\"${"+port, "\"127.0.0.1:${"+port)
	}
	if err := os.WriteFile(composePath, []byte(compose), 0640); err != nil {
		t.Fatal(err)
	}
	stored, err = s.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{ID: instance.ID, State: storage.InstanceStateSaveRequired, DriverPhase: "save_required", DriverPayload: "{}"})
	if err != nil {
		t.Fatal(err)
	}
	instance = makeRegistryInstanceFromStorage(stored)
	cfg := registry.NewGameConfig{FarmName: "ReviewFarm", FarmType: "standard", StartingCabins: 2, MaxPlayers: 4, CabinLayout: "nearby", CabinMode: "recommended", ProfitMargin: "100", MoneyMode: "shared", FarmerName: "ReviewHost", FavoriteThing: "Safe saves", Gender: "male", PetType: "Cat"}
	t.Log("creating an independent real game world")
	job, err := d.Start(ctx, registry.StartRequest{Instance: instance, NewGame: true, NewGameConfig: &cfg, RequestID: "farmhand-review-create"})
	if err != nil {
		t.Fatal(err)
	}
	assertRealHostBedJobSucceeded(t, ctx, s, job.ID, 5*time.Minute)
	instance = refreshRealHostBedInstance(t, ctx, s, instance)
	saveID := GetActiveSaveName(dir)
	if saveID == "" {
		t.Fatal("new game has no save identity")
	}
	if err := os.Chmod(filepath.Join(root, "upstream/docker/rootfs-test-client/startapp.sh"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range clients {
		run("run", "-d", "--name", name, "--label", "codex.task="+filepath.Base(root), "--network", project+"_default", "--cpus", "0.5", "--memory", "1536m",
			"--health-cmd", "curl -fsS http://localhost:5123/ping >/dev/null", "--health-interval", "10s", "--health-start-period", "60s",
			"-e", "APP_NAME=Farmhand Review Client", "-e", "CLIENT_FPS=15", "-e", "JUNIMO_TEST_PORT=5123", "-e", "SMAPI_VERSION=4.5.2", "-e", "VNC_PASSWORD=review-only",
			"--mount", "type=bind,src="+filepath.Join(root, "game")+",dst=/data/game",
			"--mount", "type=bind,src="+filepath.Join(root, "client-mods")+",dst=/data/Mods",
			"--mount", "type=bind,src="+filepath.Join(root, "upstream/docker/rootfs-test-client/startapp.sh")+",dst=/startapp.sh,readonly",
			"--mount", "type=bind,src="+filepath.Join(root, "upstream/docker/rootfs-test-client/data/smapi-config.json")+",dst=/data/smapi-config.json,readonly", image)
		waitRealHostBedClientReady(t, ctx, name, 3*time.Minute)
	}
	request := func(client, method, path, body string) []byte {
		return runRealHostBedClientRequest(t, ctx, client, method, path, body)
	}
	ok := func(client, method, path, body string) {
		t.Helper()
		assertRealHostBedClientSuccess(t, request(client, method, path, body), path)
	}
	claim := func(client, name string) string {
		t.Helper()
		ok(client, "POST", "/connect/lan", `{"address":"server:24642"}`)
		ok(client, "GET", "/wait/farmhands?timeout=60000", "")
		var slots struct {
			Farmhands []struct {
				Index      int    `json:"index"`
				Name       string `json:"name"`
				Customized bool   `json:"isCustomized"`
			} `json:"farmhands"`
		}
		if err := json.Unmarshal(request(client, "GET", "/farmhands", ""), &slots); err != nil {
			t.Fatal(err)
		}
		index, customized := -1, false
		for _, slot := range slots.Farmhands {
			if slot.Name == name && slot.Customized {
				index, customized = slot.Index, true
				break
			}
			if index < 0 && !slot.Customized {
				index = slot.Index
			}
		}
		if index < 0 {
			t.Fatalf("no available farmhand for %s", name)
		}
		ok(client, "POST", "/farmhands/select", fmt.Sprintf(`{"slotIndex":%d}`, index))
		if !customized {
			ok(client, "GET", "/wait/character?timeout=60000", "")
			ok(client, "POST", "/character/customize", fmt.Sprintf(`{"name":%q,"favoriteThing":"review"}`, name))
			ok(client, "POST", "/character/confirm", `{}`)
		}
		ok(client, "GET", "/wait/world-ready?timeout=90000", "")
		var farmer struct {
			UniqueID string `json:"uniqueId"`
		}
		if err := json.Unmarshal(request(client, "GET", "/farmer", ""), &farmer); err != nil || farmer.UniqueID == "" {
			t.Fatalf("farmer identity: %+v %v", farmer, err)
		}
		return farmer.UniqueID
	}
	exitClient := func(client string) {
		t.Helper()
		ok(client, "POST", "/exit", `{}`)
		ok(client, "POST", "/navigate", `{"target":"title"}`)
		ok(client, "GET", "/wait/disconnected?timeout=60000", "")
	}
	connections := func(count int) {
		t.Helper()
		waitRealHostBedControl(t, dir, time.Minute, func(status realHostBedControlStatus) bool {
			return status.SaveID == saveID && status.HostControl.ConnectedClients != nil && *status.HostControl.ConnectedClients == count
		})
	}
	waitIntent := func(status string) storage.FarmhandDeleteIntent {
		t.Helper()
		for deadline := time.Now().Add(4 * time.Minute); time.Now().Before(deadline); {
			intent, err := s.GetFarmhandDeleteIntent(ctx, instance.ID)
			if err == nil && intent.Status == status {
				return intent
			}
			if err == nil && (intent.Status == storage.FarmhandDeleteIntentFailed || intent.Status == storage.FarmhandDeleteIntentExpired || intent.Status == storage.FarmhandDeleteIntentCanceled || intent.Status == storage.FarmhandDeleteIntentRecoveryRequired) {
				t.Fatalf("delete failed: %+v", intent)
			}
			time.Sleep(250 * time.Millisecond)
		}
		intent, _ := s.GetFarmhandDeleteIntent(ctx, instance.ID)
		t.Fatalf("waiting for %s: %+v", status, intent)
		return intent
	}
	submit := func(id, name, mode string) {
		t.Helper()
		instance = refreshRealHostBedInstance(t, ctx, s, instance)
		if _, err := d.DeleteFarmhand(ctx, FarmhandDeleteRequest{Instance: instance, PlayerID: id, ExpectedName: name, ExpectedSave: saveID, Mode: mode}); err != nil {
			t.Fatal(err)
		}
	}
	verifyDeleted := func(id string, intent storage.FarmhandDeleteIntent) {
		t.Helper()
		if err := verifyFarmhandAbsentOnDisk(dir, saveID, id); err != nil {
			t.Fatal(err)
		}
		farms, err := readJunimoFarmhands(ctx, docker, dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range farms.Farmhands {
			if fmt.Sprint(f.ID) == id {
				t.Fatal("runtime still contains deleted character")
			}
		}
		if _, err := readFarmhandDeleteMaintenanceMarker(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("maintenance not released: %v", err)
		}
		archive, err := zip.OpenReader(filepath.Join(backupsDir(dir), intent.BackupName))
		if err != nil {
			t.Fatal(err)
		}
		defer archive.Close()
		found := false
		for _, file := range archive.File {
			if !strings.HasSuffix(file.Name, "/"+saveID) && file.Name != saveID {
				continue
			}
			f, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(f)
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
			var roster saveRosterXML
			if err := xml.Unmarshal(data, &roster); err != nil {
				t.Fatal(err)
			}
			for _, farmer := range roster.Farmhands {
				if farmer.UniqueMultiplayerID == id || farmer.UniqueMultiplayerIDFallback == id {
					found = true
				}
			}
		}
		if !found {
			t.Fatal("protection backup lacks the deleted character")
		}
	}
	go d.RunFarmhandDeleteScheduler(ctx)
	a := claim(clients[0], "DeleteTarget")
	b := claim(clients[1], "Spectator")
	exitClient(clients[0])
	connections(1)
	t.Log("wait mode must preserve the online spectator")
	submit(a, "DeleteTarget", "wait")
	for i := 0; i < 3; i++ {
		time.Sleep(time.Second)
		intent, err := s.GetFarmhandDeleteIntent(ctx, instance.ID)
		if err != nil || intent.Status != "waiting" || intent.JobID.Valid {
			t.Fatalf("wait disturbed an online world: %+v %v", intent, err)
		}
	}
	exitClient(clients[1])
	connections(0)
	completed := waitIntent("completed")
	verifyDeleted(a, completed)
	t.Log("wait deletion persisted and its backup contains the original target")
	restoreBaseline, err := d.RestoreBackupWithRestart(ctx, instance, completed.BackupName, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	assertRealHostBedJobSucceeded(t, ctx, s, restoreBaseline.ID, 4*time.Minute)
	instance = refreshRealHostBedInstance(t, ctx, s, instance)
	claim(clients[1], "Spectator")
	connections(1)
	submit(a, "DeleteTarget", "maintenance_now")
	countdown := waitIntent("countdown")
	if err := d.CancelFarmhandDeleteIntent(ctx, instance, countdown.OperationID); err != nil {
		t.Fatal(err)
	}
	waitIntent("canceled")
	connections(1)
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		active, _ := manager.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: instance.ID})
		if len(active) == 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	docker.lose.Store(true)
	t.Log("maintenance countdown then loss of the real DELETE response")
	historyCtx, cancelHistory := context.WithCancel(ctx)
	defer cancelHistory()
	historyResult := make(chan string, 1)
	go func() {
		var history strings.Builder
		defer func() { historyResult <- history.String() }()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-historyCtx.Done():
				return
			case <-ticker.C:
				raw, err := exec.CommandContext(historyCtx, "docker", "exec", clients[1], "curl", "--max-time", "2", "-fsS", "http://localhost:5123/chat/history?count=50").Output()
				var decoded any
				if err == nil && json.Unmarshal(raw, &decoded) == nil {
					text, _ := json.Marshal(decoded)
					history.Write(text)
				}
			}
		}
	}()
	submit(a, "DeleteTarget", "maintenance_now")
	recovery := waitIntent("recovery_required")
	cancelHistory()
	history := <-historyResult
	if docker.lose.Load() {
		t.Fatal("the real DELETE did not succeed before response-loss injection")
	}
	if recovery.BackupName == "" {
		t.Fatal("uncertain deletion lost its protection backup identity")
	}
	marker, err := readFarmhandDeleteMaintenanceMarker(dir)
	if err != nil || marker.Phase != "destructive" {
		t.Fatalf("uncertain deletion released gate: %+v %v", marker, err)
	}
	present, err := farmhandPresentOnDisk(dir, saveID, a)
	if err != nil || !present {
		t.Fatalf("expected pre-delete disk state after lost response: %v %v", present, err)
	}
	disconnected := false
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		var connection struct {
			Connected  bool `json:"isConnected"`
			WorldReady bool `json:"worldReady"`
			Client     bool `json:"isClient"`
		}
		if err := json.Unmarshal(request(clients[1], "GET", "/connection", ""), &connection); err != nil {
			t.Fatal(err)
		}
		if !connection.Connected && !connection.WorldReady && !connection.Client {
			disconnected = true
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if !disconnected {
		t.Fatal("spectator still has an active game connection after maintenance")
	}
	for _, seconds := range []int{60, 50, 40, 30, 20, 10} {
		if !strings.Contains(history, fmt.Sprintf("%d 秒", seconds)) {
			t.Fatalf("missing countdown %d in chat: %s", seconds, history)
		}
	}
	serverName := project + "-server-1"
	clockResult := run("exec", serverName, "curl", "--max-time", "10", "-fsS", "-X", "POST", "--data", "", "http://localhost:8080/time?value=610")
	assertRealHostBedClientSuccess(t, []byte(clockResult), "set server regression clock")
	clockConfirmed := false
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); {
		raw, e := readJunimoAPI(ctx, docker, dir, "/diagnostics/state")
		var diagnostic struct {
			TimeOfDay int `json:"timeOfDay"`
		}
		if e == nil && json.Unmarshal(raw, &diagnostic) == nil && diagnostic.TimeOfDay == 610 {
			clockConfirmed = true
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if !clockConfirmed {
		t.Fatal("did not establish the Junimo 06:10 reopening regression condition")
	}
	assertJoinBlocked := func() {
		t.Helper()
		exitClient(clients[1])
		ok(clients[1], "POST", "/connect/lan", `{"address":"server:24642"}`)
		var joined struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		output := run("exec", clients[1], "curl", "--max-time", "5", "-sS", "http://localhost:5123/wait/farmhands?timeout=3000")
		if err := json.Unmarshal([]byte(output), &joined); err != nil || joined.Success || joined.Error == "" {
			t.Fatalf("client entered maintenance or returned no failure evidence: %+v %v", joined, err)
		}
		exitClient(clients[1])
	}
	assertJoinBlocked()
	run("kill", "--signal", "KILL", serverName)
	run("start", serverName)
	restoredTargetFound := false
	for deadline := time.Now().Add(2 * time.Minute); time.Now().Before(deadline); {
		farms, e := readJunimoFarmhands(ctx, docker, dir)
		if e == nil {
			found := false
			for _, f := range farms.Farmhands {
				found = found || fmt.Sprint(f.ID) == a
			}
			if found {
				restoredTargetFound = true
				break
			}
		}
		time.Sleep(time.Second)
	}
	if !restoredTargetFound {
		t.Fatal("crash recovery did not reload the pre-delete character from disk")
	}
	marker, err = readFarmhandDeleteMaintenanceMarker(dir)
	if err != nil || marker.Phase != "destructive" {
		t.Fatalf("game restart unlocked uncertain deletion: %+v %v", marker, err)
	}
	assertJoinBlocked()
	t.Log("explicit protection-backup restore must unlock the restarted world")
	restore, err := d.RestoreBackupWithRestart(ctx, instance, recovery.BackupName, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	assertRealHostBedJobSucceeded(t, ctx, s, restore.ID, 4*time.Minute)
	waitIntent("canceled")
	claim(clients[1], "Spectator")
	connections(1)
	t.Log("sleep during countdown must cancel before deletion")
	beforeSleep := readRealHostBedDiagnostics(t, ctx, docker, dir)
	submit(a, "DeleteTarget", "maintenance_now")
	waitIntent("countdown")
	ok(clients[1], "POST", "/actions/sleep", `{}`)
	waitIntent("canceled")
	dayReady := false
	for deadline := time.Now().Add(3 * time.Minute); time.Now().Before(deadline); {
		raw, err := readJunimoAPI(ctx, docker, dir, "/diagnostics/state")
		if err == nil {
			var after struct {
				DayOfMonth int    `json:"dayOfMonth"`
				Season     string `json:"season"`
				Year       int    `json:"year"`
				NewDaySync struct {
					Active bool `json:"isActive"`
				} `json:"newDaySync"`
			}
			if err := json.Unmarshal(raw, &after); err != nil {
				t.Fatal(err)
			}
			if after.DayOfMonth > 0 && !after.NewDaySync.Active && (after.DayOfMonth != beforeSleep.DayOfMonth || after.Season != beforeSleep.Season || after.Year != beforeSleep.Year) {
				dayReady = true
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !dayReady {
		t.Fatal("the real sleep transition did not settle into the next day")
	}
	ok(clients[1], "GET", "/wait/world-ready?timeout=90000", "")
	exitClient(clients[1])
	connections(0)
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
		active, _ := manager.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: instance.ID})
		if len(active) == 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	docker.lose.Store(true)
	submit(a, "DeleteTarget", "wait")
	recovery = waitIntent("recovery_required")
	if docker.lose.Load() {
		t.Fatal("retry fixture did not execute the real DELETE")
	}
	waitForHostHouseJob(t, ctx, s, recovery.JobID.String, 30*time.Second)
	t.Log("retry final persistence must save without repeating DELETE")
	retry, err := d.RetryFarmhandDeletePersistence(ctx, instance, recovery.OperationID, 0)
	if err != nil {
		t.Fatal(err)
	}
	assertRealHostBedJobSucceeded(t, ctx, s, retry.JobID, 3*time.Minute)
	completed = waitIntent("completed")
	verifyDeleted(a, completed)
	farms, err := readJunimoFarmhands(ctx, docker, dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range farms.Farmhands {
		found = found || fmt.Sprint(f.ID) == b
	}
	if !found {
		t.Fatal("unrelated spectator character was removed")
	}
	t.Log("real wait, countdown/cancel, sleep cancel, uncertain DELETE, restart, protection restore and retry-save passed")
}
