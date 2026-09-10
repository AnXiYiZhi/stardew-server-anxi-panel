package stardew_junimo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func farmhandDeleteFixture(t *testing.T) (*Driver, *storage.Store, storage.FarmhandDeleteIntent, string) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	s, err := storage.Open(ctx, config.Config{Addr: ":0", DataDir: root, DBPath: filepath.Join(root, "panel.db"), Secret: "test", Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "world")
	if _, err := s.CreateInstance(ctx, storage.CreateInstanceParams{ID: "world", DriverID: storage.DefaultDriverID, Name: "World", DataDir: dir, State: storage.InstanceStateRunning}); err != nil {
		t.Fatal(err)
	}
	if err := SetActiveSave(dir, "Farm_1"); err != nil {
		t.Fatal(err)
	}
	saveDir := filepath.Join(savesDir(dir), "Saves", "Farm_1")
	if err := os.MkdirAll(saveDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "Farm_1"), []byte(`<SaveGame><player><UniqueMultiplayerID>1</UniqueMultiplayerID></player><farmhands></farmhands></SaveGame>`), 0600); err != nil {
		t.Fatal(err)
	}
	intent, err := s.CreateFarmhandDeleteIntent(ctx, storage.CreateFarmhandDeleteIntentParams{InstanceID: "world", OperationID: "0123456789abcdef0123456789abcdef", Mode: storage.FarmhandDeleteModeWait, Status: storage.FarmhandDeleteIntentLaunching, PlayerID: "42", ExpectedSaveID: "Farm_1", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatal(err)
	}
	return New(&fakeConsoleDocker{}, nil, jobs.NewManager(s, nil), s), s, intent, dir
}

func TestFarmhandDeleteCompletedSurvivesStaleScheduler(t *testing.T) {
	d, s, intent, _ := farmhandDeleteFixture(t)
	ctx := context.Background()
	job, err := s.CreateJob(ctx, storage.CreateJobParams{Type: FarmhandDeleteJobType, TargetType: "instance", TargetID: "world"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.BindFarmhandDeleteIntentJob(ctx, "world", intent.OperationID, job.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	stale, err := s.GetFarmhandDeleteIntent(ctx, "world")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateFarmhandDeleteIntent(ctx, "world", intent.OperationID, storage.FarmhandDeleteIntentCompleted, "protect.zip", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FinishJob(ctx, job.ID); err != nil {
		t.Fatal(err)
	}
	if err := d.processFarmhandDeleteIntent(ctx, s, stale); err != nil {
		t.Fatal(err)
	}
	if err := s.ReconcileFarmhandDeleteIntent(ctx, stale, storage.FarmhandDeleteIntentRecoveryRequired, "stale"); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("stale update = %v", err)
	}
	got, err := s.GetFarmhandDeleteIntent(ctx, "world")
	if err != nil || got.Status != storage.FarmhandDeleteIntentCompleted || got.BackupName != "protect.zip" {
		t.Fatalf("completed intent = %+v, %v", got, err)
	}
}

func TestFarmhandDeleteInterruptedRecoveryIgnoresWaitExpiry(t *testing.T) {
	d, s, intent, dir := farmhandDeleteFixture(t)
	ctx := context.Background()
	if err := s.UpdateFarmhandDeleteIntent(ctx, "world", intent.OperationID, storage.FarmhandDeleteIntentRecoveryRequired, "protect.zip", "save failed"); err != nil {
		t.Fatal(err)
	}
	writeFarmhandDeleteMarker(t, dir, intent)
	if _, err := s.BeginFarmhandDeleteRecovery(ctx, "world", intent.OperationID); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(filepath.Dir(dir), "panel.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	old := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := db.ExecContext(ctx, "UPDATE farmhand_delete_intents SET updated_at=?, expires_at=? WHERE instance_id=?", old, old, "world"); err != nil {
		t.Fatal(err)
	}
	launching, err := s.GetFarmhandDeleteIntent(ctx, "world")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.processFarmhandDeleteIntent(ctx, s, launching); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetFarmhandDeleteIntent(ctx, "world")
	if err != nil || got.Status != storage.FarmhandDeleteIntentRecoveryRequired {
		t.Fatalf("recovery intent = %+v, %v", got, err)
	}
}

func TestFarmhandDeleteStoppedSaveSwitchCancelsWait(t *testing.T) {
	d, s, intent, dir := farmhandDeleteFixture(t)
	ctx := context.Background()
	if err := s.ResetLaunchingFarmhandDeleteIntent(ctx, "world", intent.OperationID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateInstanceState(ctx, storage.UpdateInstanceStateParams{ID: "world", State: storage.InstanceStateStopped, DriverPhase: "stopped", DriverPayload: "{}"}); err != nil {
		t.Fatal(err)
	}
	if err := SetActiveSave(dir, "Other_2"); err != nil {
		t.Fatal(err)
	}
	waiting, err := s.GetFarmhandDeleteIntent(ctx, "world")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.processFarmhandDeleteIntent(ctx, s, waiting); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetFarmhandDeleteIntent(ctx, "world")
	if err != nil || got.Status != storage.FarmhandDeleteIntentCanceled {
		t.Fatalf("waiting intent = %+v, %v", got, err)
	}
}

func TestFarmhandDeleteRapidSaveSelectionCannotReviveWait(t *testing.T) {
	d, s, intent, dir := farmhandDeleteFixture(t)
	ctx := context.Background()
	if err := s.ResetLaunchingFarmhandDeleteIntent(ctx, "world", intent.OperationID); err != nil {
		t.Fatal(err)
	}
	instance := registry.Instance{ID: "world", DataDir: dir, State: storage.InstanceStateStopped}
	for _, selected := range []string{"Other_2", "Farm_1"} {
		if err := SetActiveSave(dir, selected); err != nil {
			t.Fatal(err)
		}
		if err := d.CancelFarmhandDeleteForSaveChange(ctx, instance, selected); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.GetFarmhandDeleteIntent(ctx, "world")
	if err != nil || got.Status != storage.FarmhandDeleteIntentCanceled {
		t.Fatalf("old wait survived rapid save selection: %+v, %v", got, err)
	}
}

func TestFarmhandDeleteRequiresFreshCompleteWorldSnapshot(t *testing.T) {
	r := farmhandDeleteRunner{expectedSave: "Farm_1", playerID: "42"}
	zero, two := 0, 2
	for _, name := range []string{"unavailable", "old", "different_world", "count_mismatch", "anonymous"} {
		t.Run(name, func(t *testing.T) {
			snapshot := &PlayersResult{Source: "smapi_control", ParseStatus: "exact", SaveID: "Farm_1", UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano), OnlineCount: &zero}
			switch name {
			case "unavailable":
				snapshot.Source, snapshot.ParseStatus, snapshot.OnlineCount = "junimo_info", "partial", &two
			case "old":
				snapshot.UpdatedAt = "2020-01-01T00:00:00Z"
			case "different_world":
				snapshot.SaveID = "Other_2"
			case "count_mismatch":
				snapshot.OnlineCount = &two
			case "anonymous":
				snapshot.Players = []PlayerInfo{{Status: "online"}}
			}
			if _, err := r.connectedHumansFromSnapshot(snapshot); err == nil {
				t.Fatal("unreliable snapshot was accepted")
			}
		})
	}
	valid := &PlayersResult{Source: "smapi_control", ParseStatus: "exact", SaveID: "Farm_1", UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano), OnlineCount: &zero}
	if players, err := r.connectedHumansFromSnapshot(valid); err != nil || len(players) != 0 {
		t.Fatalf("fresh empty snapshot = %v, %v", players, err)
	}
}

func TestFarmhandDeleteTimedOutBeginCannotRemainQueued(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := registry.Instance{ID: "world", DataDir: t.TempDir(), State: storage.InstanceStateRunning}
	r := farmhandDeleteRunner{driver: d, instance: instance, operationID: "0123456789abcdef0123456789abcdef", expectedSave: "Farm_1", playerID: "42", mode: "wait"}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := r.beginMaintenance(ctx); err == nil {
		t.Fatal("expected timeout")
	}
	paths, err := filepath.Glob(filepath.Join(controlDir(instance.DataDir), "commands", "*.json"))
	if err != nil || len(paths) != 0 {
		t.Fatalf("late commands = %v, %v", paths, err)
	}
}

func TestFarmhandDeleteReleaseFailureRemainsReconciliable(t *testing.T) {
	for _, scenario := range []string{"seal_ack_lost", "guarded_release_failed", "seal_rejected_released"} {
		t.Run(scenario, func(t *testing.T) {
			d, s, intent, dir := farmhandDeleteFixture(t)
			ctx := context.Background()
			writeJSON := func(path string, value any) {
				t.Helper()
				raw, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path+".tmp", raw, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(path+".tmp", path); err != nil {
					t.Fatal(err)
				}
			}
			writeJSON(filepath.Join(controlDir(dir), "players.json"), map[string]any{
				"saveId": intent.ExpectedSaveID, "updatedAt": time.Now().UTC().Format(time.RFC3339Nano), "players": []any{},
			})
			savePath := filepath.Join(savesDir(dir), "Saves", intent.ExpectedSaveID, intent.ExpectedSaveID)
			if err := os.WriteFile(savePath, []byte(`<SaveGame><player><UniqueMultiplayerID>1</UniqueMultiplayerID></player><farmhands><Farmer><UniqueMultiplayerID>42</UniqueMultiplayerID></Farmer></farmhands></SaveGame>`), 0600); err != nil {
				t.Fatal(err)
			}
			d.docker = &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
				if strings.HasSuffix(args[len(args)-1], "/farmhands") {
					return paneldocker.CommandResult{Stdout: `{"farmhands":[{"id":42,"name":"Alice","isCustomized":true}]}`}, nil
				}
				return paneldocker.CommandResult{}, errors.New("fixture has no other Junimo API")
			}}
			markerPath := filepath.Join(controlDir(dir), "farmhand-delete-maintenance.json")
			releaseAvailable := scenario == "seal_rejected_released"
			pumpUntil := func(done func() bool) {
				t.Helper()
				deadline := time.Now().Add(10 * time.Second)
				for !done() {
					if time.Now().After(deadline) {
						t.Fatal("fixture command processing timed out")
					}
					paths, err := filepath.Glob(filepath.Join(controlDir(dir), "commands", "*.json"))
					if err != nil {
						t.Fatal(err)
					}
					for _, path := range paths {
						var command struct{ ID, Name string }
						raw, err := os.ReadFile(path)
						if err != nil || json.Unmarshal(raw, &command) != nil {
							t.Fatalf("read fixture command: %v", err)
						}
						outcome := CommandOutcome{CommandID: command.ID, Status: CommandStatusSucceeded, UpdatedAt: time.Now().UTC()}
						resultPath := filepath.Join(commandResultsDir(dir), command.ID+".json")
						lostResult := false
						switch command.Name {
						case "farmhand-delete-maintenance-begin":
							expires := time.Now().Add(10 * time.Minute)
							writeJSON(markerPath, farmhandDeleteMaintenanceMarker{SchemaVersion: 1, OperationID: intent.OperationID, ExpectedSaveID: intent.ExpectedSaveID, TargetPlayerID: intent.PlayerID, Phase: "guarded", ExpiresAt: &expires})
						case "farmhand-delete-maintenance-check":
							if scenario == "guarded_release_failed" {
								outcome.Status, outcome.ErrorCode = CommandStatusFailed, "sleep_in_progress"
							}
						case "farmhand-delete-maintenance-seal":
							if scenario == "seal_ack_lost" {
								writeFarmhandDeleteMarker(t, dir, intent)
								lostResult = true
							} else {
								outcome.Status, outcome.ErrorCode = CommandStatusFailed, "sleep_in_progress"
							}
						case "farmhand-delete-maintenance-end":
							lostResult = !releaseAvailable
							if releaseAvailable {
								if err := os.Remove(markerPath); err != nil {
									t.Fatal(err)
								}
							}
						}
						if lostResult {
							// Make the acknowledgement unreadable after Control's side effect.
							if err := os.MkdirAll(resultPath, 0700); err != nil {
								t.Fatal(err)
							}
						} else {
							writeJSON(resultPath, outcome)
						}
						if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
							t.Fatal(err)
						}
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
			job, err := d.startFarmhandDeleteJob(ctx, registry.Instance{ID: "world", DataDir: dir, State: storage.InstanceStateRunning}, intent)
			if err != nil {
				t.Fatal(err)
			}
			pumpUntil(func() bool {
				current, err := s.GetJob(ctx, job.ID)
				if err != nil {
					t.Fatal(err)
				}
				return current.Status != storage.JobStatusQueued && current.Status != storage.JobStatusRunning
			})
			current, err := s.GetFarmhandDeleteIntent(ctx, intent.InstanceID)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "seal_rejected_released" {
				if current.Status != storage.FarmhandDeleteIntentCanceled {
					t.Fatalf("confirmed seal rejection should stay canceled: %+v", current)
				}
				return
			}
			if current.Status != storage.FarmhandDeleteIntentActive {
				t.Fatalf("unconfirmed release lost its recoverable intent: %+v", current)
			}
			if scenario == "seal_ack_lost" && current.BackupName == "" {
				t.Fatal("sealed operation lost its protection backup")
			}
			releaseAvailable = true
			reconciled := make(chan error, 1)
			go func() { reconciled <- d.processFarmhandDeleteIntent(ctx, s, current) }()
			pumpUntil(func() bool {
				select {
				case err := <-reconciled:
					if err != nil {
						t.Fatal(err)
					}
					return true
				default:
					return false
				}
			})
			got, err := s.GetFarmhandDeleteIntent(ctx, intent.InstanceID)
			want := storage.FarmhandDeleteIntentCanceled
			if scenario == "seal_ack_lost" {
				want = storage.FarmhandDeleteIntentRecoveryRequired
			}
			if err != nil || got.Status != want || got.OperationID != intent.OperationID || got.BackupName != current.BackupName {
				t.Fatalf("reconciled intent = %+v, error=%v; want %s", got, err, want)
			}
		})
	}
}

func TestFarmhandDeleteUsesContainerPortWithCustomHostPublication(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("API_PORT=59871\n"), 0600); err != nil {
		t.Fatal(err)
	}
	lifecycle := &fakeConsoleDocker{execFunc: func(_ context.Context, _, service, _ string, args ...string) (paneldocker.CommandResult, error) {
		if service != "server" || args[len(args)-1] != "http://localhost:8080/farmhands?playerId=-42" {
			t.Fatalf("DELETE used host publication inside container: service=%s args=%v", service, args)
		}
		return paneldocker.CommandResult{Stdout: "{\"success\":true}\n200\n"}, nil
	}}
	runner := farmhandDeleteRunner{lifecycle: lifecycle, instance: registry.Instance{DataDir: dir}, playerID: "-42"}
	if err := runner.deleteViaJunimo(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func writeFarmhandDeleteMarker(t *testing.T, dir string, intent storage.FarmhandDeleteIntent) {
	t.Helper()
	raw, err := json.Marshal(farmhandDeleteMaintenanceMarker{SchemaVersion: 1, OperationID: intent.OperationID, ExpectedSaveID: intent.ExpectedSaveID, TargetPlayerID: intent.PlayerID, Phase: "destructive"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(controlDir(dir), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(controlDir(dir), "farmhand-delete-maintenance.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestFarmhandDeleteOnlyMatchingExplicitRestoreClearsGate(t *testing.T) {
	d, s, intent, dir := farmhandDeleteFixture(t)
	ctx := context.Background()
	if err := s.UpdateFarmhandDeleteIntent(ctx, "world", intent.OperationID, storage.FarmhandDeleteIntentRecoveryRequired, "protect.zip", "save failed"); err != nil {
		t.Fatal(err)
	}
	writeFarmhandDeleteMarker(t, dir, intent)
	instance := registry.Instance{ID: "world", DataDir: dir, State: storage.InstanceStateStopped}
	if job, err := d.RestoreBackupWithRestart(ctx, instance, "other.zip", true, 0); err == nil || job != nil {
		t.Fatalf("wrong backup was not rejected before starting lifecycle work: job=%v, error=%v", job, err)
	}
	if err := d.CompleteFarmhandDeleteBackupRestore(ctx, instance, "other.zip", "Farm_1"); err == nil {
		t.Fatal("wrong backup cleared recovery")
	}
	if err := d.CompleteFarmhandDeleteBackupRestore(ctx, instance, "protect.zip", "Farm_1"); err == nil {
		t.Fatal("missing target cleared recovery")
	}
	path := filepath.Join(savesDir(dir), "Saves", "Farm_1", "Farm_1")
	if err := os.WriteFile(path, []byte(`<SaveGame><player><UniqueMultiplayerID>1</UniqueMultiplayerID></player><farmhands><Farmer><UniqueMultiplayerID>42</UniqueMultiplayerID></Farmer></farmhands></SaveGame>`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := d.CompleteFarmhandDeleteBackupRestore(ctx, instance, "protect.zip", "Farm_1"); err != nil {
		t.Fatal(err)
	}
	if _, err := readFarmhandDeleteMaintenanceMarker(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("marker remains: %v", err)
	}
	got, err := s.GetFarmhandDeleteIntent(ctx, "world")
	if err != nil || got.Status != storage.FarmhandDeleteIntentCanceled {
		t.Fatalf("restored intent = %+v, %v", got, err)
	}
}
