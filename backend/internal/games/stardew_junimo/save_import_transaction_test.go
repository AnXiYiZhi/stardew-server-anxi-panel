package stardew_junimo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func testImportRequest(dir, operationID, platformID string) registry.SaveImportRequest {
	return registry.SaveImportRequest{Instance: registry.Instance{ID: "instance-1", DataDir: dir}, OperationID: operationID, SaveName: "Imported_123", HostHandling: "swap_host_to", PlatformID: platformID, StagedDir: filepath.Join(dir, "staged")}
}

func TestImportJournalIdempotentAndSensitiveIDNotPersisted(t *testing.T) {
	dir := t.TempDir()
	op := "00112233445566778899aabbccddeeff"
	platformID := "76561198012345678"
	first, err := CreateImportJournal(dir, testImportRequest(dir, op, platformID))
	if err != nil {
		t.Fatal(err)
	}
	second, err := CreateImportJournal(dir, testImportRequest(dir, op, platformID))
	if err != nil {
		t.Fatal(err)
	}
	if first.OperationID != second.OperationID || first.Stage != ImportStageValidated {
		t.Fatalf("not idempotent: %#v %#v", first, second)
	}
	data, err := os.ReadFile(importJournalPath(dir, op))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), platformID) {
		t.Fatal("journal leaked full platform ID")
	}
	if first.PlatformIDFingerprint == "" {
		t.Fatal("missing platform fingerprint")
	}
	mismatch := testImportRequest(dir, op, "76561198000000000")
	if _, err := CreateImportJournal(dir, mismatch); err == nil {
		t.Fatal("same operation accepted a different platform fingerprint")
	} else if typed, ok := AsImportTransactionError(err); !ok || typed.Code != "operation_conflict" {
		t.Fatalf("mismatch error=%v", err)
	}
	if info, err := os.Stat(importJournalPath(dir, op)); err != nil {
		t.Fatal(err)
	} else if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("journal mode = %v", info.Mode().Perm())
	}
}

func TestImportJobBindingFailurePreventsRunnerStart(t *testing.T) {
	dataDir := t.TempDir()
	op := "06112233445566778899aabbccddeeff"
	req := testImportRequest(dataDir, op, "")
	j, err := CreateImportJournal(dataDir, req)
	if err != nil {
		t.Fatal(err)
	}
	writeImportSourceFixture(t, importTransactionSourceDir(dataDir, op), req.SaveName, "owned")
	j.SourceOwned = true
	if err := WriteImportJournal(dataDir, j); err != nil {
		t.Fatal(err)
	}
	req.AttachJobIdentity = func(string) error { return errors.New("injected token attach failure") }

	store := newLifecycleTestStore(t)
	manager := jobs.NewManager(store, slog.Default())
	runnerCalls := 0
	payload, _ := json.Marshal(map[string]string{"operationId": op})
	job, err := manager.Start(context.Background(), jobs.Spec{
		Type: SaveImportJobType, TargetType: "instance", TargetID: req.Instance.ID,
		IdempotencyKey: SaveImportJobIdempotencyKey(op), Payload: string(payload),
		BeforeRun: func(_ context.Context, durable storage.Job) error {
			return persistImportJobBinding(req, durable.ID)
		},
		Run: func(context.Context, *jobs.Context) error {
			runnerCalls++
			return nil
		},
	})
	var preparation *jobs.StartPreparationError
	if !errors.As(err, &preparation) || job.ID == "" {
		t.Fatalf("job=%+v err=%v", job, err)
	}
	if runnerCalls != 0 {
		t.Fatalf("runner calls=%d", runnerCalls)
	}
	journal, err := LoadImportJournal(dataDir, op)
	if err != nil || journal.JobID != job.ID || journal.JobBindingState != importJobBindingJournalAttached {
		t.Fatalf("journal=%+v err=%v", journal, err)
	}
	stored, err := store.GetJob(context.Background(), job.ID)
	if err != nil || stored.Status != storage.JobStatusFailed {
		t.Fatalf("stored job=%+v err=%v", stored, err)
	}
}

func TestImportJournalUnknownSchemaStageAndFieldsFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(string) string
	}{
		{name: "schema", mutate: func(raw string) string { return strings.Replace(raw, `"schemaVersion": 1`, `"schemaVersion": 2`, 1) }},
		{name: "stage", mutate: func(raw string) string {
			return strings.Replace(raw, `"stage": "validated"`, `"stage": "future_stage"`, 1)
		}},
		{name: "unknown field", mutate: func(raw string) string {
			return strings.TrimSuffix(strings.TrimSpace(raw), "}") + ",\n  \"futureField\": true\n}\n"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			op := "09112233445566778899aabbccddeeff"
			if _, err := CreateImportJournal(dir, testImportRequest(dir, op, "")); err != nil {
				t.Fatal(err)
			}
			path := importJournalPath(dir, op)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tc.mutate(string(raw))), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadImportJournal(dir, op); err == nil {
				t.Fatal("unsupported journal was accepted")
			}
			if _, err := HasUnfinishedImportTransaction(dir); err == nil {
				t.Fatal("unsupported journal was treated as safely idle")
			}
		})
	}
}

func TestImportJournalSameNameDoesNotModifyExistingSave(t *testing.T) {
	dir := t.TempDir()
	saveDir := filepath.Join(savesDir(dir), "Saves", "Imported_123")
	if err := os.MkdirAll(saveDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(saveDir, "Imported_123")
	original := []byte("exact existing bytes")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := CreateImportJournal(dir, testImportRequest(dir, "10112233445566778899aabbccddeeff", ""))
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorSaveExists {
		t.Fatalf("error=%v typed=%#v", err, typed)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(original) {
		t.Fatal("existing save bytes changed")
	}
}

func TestRecoverImportTransactionAtEveryStage(t *testing.T) {
	stages := []string{ImportStageValidated, ImportStageStaged, ImportStageBackupCreated, ImportStageRuntimeReady, ImportStageSubmitted, ImportStageConfirmed, ImportStageSaveActivating, ImportStageFinalizeConfirmed, ImportStageSavePersisting, ImportStageSaveVerified}
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			op := "20112233445566778899aabbccddeeff"
			j, err := CreateImportJournal(dir, testImportRequest(dir, op, ""))
			if err != nil {
				t.Fatal(err)
			}
			j.Stage = stage
			j.UpstreamSubmitted = importStageAtLeast(stage, ImportStageSubmitted)
			j.UpstreamConfirmed = importStageAtLeast(stage, ImportStageConfirmed)
			if err := WriteImportJournal(dir, j); err != nil {
				t.Fatal(err)
			}
			recovered, err := RecoverImportTransactions(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(recovered) != 1 {
				t.Fatalf("recoveries=%v", recovered)
			}
			durableResume := stage == ImportStageFinalizeConfirmed || stage == ImportStageSavePersisting || stage == ImportStageSaveVerified
			activationResume := stage == ImportStageConfirmed || stage == ImportStageSaveActivating
			if durableResume && recovered[0].State != "resume_save_verification" {
				t.Fatalf("durable stage was not resumable: %#v", recovered[0])
			}
			if activationResume && recovered[0].State != "resume_activation_verification" {
				t.Fatalf("activation stage was not safely resumable: %#v", recovered[0])
			}
			if importStageAtLeast(stage, ImportStageSubmitted) && !durableResume && !activationResume && recovered[0].State != "manual_required" {
				t.Fatalf("submitted stage was not manual: %#v", recovered[0])
			}
			if !importStageAtLeast(stage, ImportStageSubmitted) && recovered[0].State != "safe_to_resume_or_cleanup" {
				t.Fatalf("pre-submit stage not safe: %#v", recovered[0])
			}
		})
	}
}

func TestCleanupImportStopsAfterUpstreamSubmission(t *testing.T) {
	dir := t.TempDir()
	op := "30112233445566778899aabbccddeeff"
	j, err := CreateImportJournal(dir, testImportRequest(dir, op, ""))
	if err != nil {
		t.Fatal(err)
	}
	if err := CleanupUnsubmittedImport(dir, op); err != nil {
		t.Fatal(err)
	}
	canceled, err := LoadImportJournal(dir, op)
	if err != nil || canceled.Stage != ImportStageCanceled {
		t.Fatalf("cleanup marker=%+v err=%v", canceled, err)
	}
	if err := FinalizeCanceledImportCleanup(dir, op); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadImportJournal(dir, op); !os.IsNotExist(err) {
		t.Fatalf("finalized journal still exists: %v", err)
	}
	j, err = CreateImportJournal(dir, testImportRequest(dir, op, ""))
	if err != nil {
		t.Fatal(err)
	}
	j.Stage, j.UpstreamSubmitted = ImportStageSubmitted, true
	if err := WriteImportJournal(dir, j); err != nil {
		t.Fatal(err)
	}
	err = CleanupUnsubmittedImport(dir, op)
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("cleanup error=%v", err)
	}
	if _, err := LoadImportJournal(dir, op); err != nil {
		t.Fatalf("submitted journal was removed: %v", err)
	}
}

func TestCleanupImportStopsWhileMaintenanceMayStillBeRunning(t *testing.T) {
	dir := t.TempDir()
	op := "31112233445566778899aabbccddeeff"
	j, err := CreateImportJournal(dir, testImportRequest(dir, op, ""))
	if err != nil {
		t.Fatal(err)
	}
	j.Stage = ImportStageRuntimeReady
	j.MaintenanceStarted = true
	if err := WriteImportJournal(dir, j); err != nil {
		t.Fatal(err)
	}
	err = CleanupUnsubmittedImport(dir, op)
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired {
		t.Fatalf("cleanup error=%v", err)
	}
	if _, err := LoadImportJournal(dir, op); err != nil {
		t.Fatalf("live maintenance journal was removed: %v", err)
	}
}

func TestCleanupUnsubmittedSaveImportHoldsLifecycleLockAndRejectsRunningCompose(t *testing.T) {
	dir := t.TempDir()
	op := "32112233445566778899aabbccddeeff"
	if _, err := CreateImportJournal(dir, testImportRequest(dir, op, "")); err != nil {
		t.Fatal(err)
	}
	store := &importMaintenanceStore{instance: storage.Instance{
		ID: "instance-1", DriverID: DriverID, DataDir: dir, State: storage.InstanceStateGameInstalled,
		StateMessage: sql.NullString{String: "installed", Valid: true}, DriverPhase: "game_installed", DriverPayload: `{}`,
	}}
	entered := make(chan struct{})
	release := make(chan struct{})
	fake := &fakeConsoleDocker{composePsFunc: func(context.Context, string) (paneldocker.ComposePsResult, error) {
		close(entered)
		<-release
		return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "running"}}}, nil
	}}
	driver := New(fake, nil, nil, store)
	errCh := make(chan error, 1)
	go func() {
		errCh <- driver.CleanupUnsubmittedSaveImport(context.Background(), registry.Instance{ID: "instance-1", DataDir: dir}, op)
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not reach Compose preflight")
	}
	if driver.runtimeUpdateMu.TryLock() {
		driver.runtimeUpdateMu.Unlock()
		close(release)
		t.Fatal("cleanup released lifecycle lock between Compose preflight and filesystem gates")
	}
	close(release)
	err := <-errCh
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorSaveInProgress {
		t.Fatalf("cleanup error=%v", err)
	}
	if _, err := LoadImportJournal(dir, op); err != nil {
		t.Fatalf("running Compose cleanup removed journal: %v", err)
	}
}

func TestImportSaveAndStartRejectsUnsafeStateBeforeOwnership(t *testing.T) {
	for _, state := range []string{
		storage.InstanceStateUninitialized,
		storage.InstanceStateJunimoScaffolded,
		storage.InstanceStateSteamAuthRunning,
		storage.InstanceStateStarting,
		storage.InstanceStateRunning,
	} {
		t.Run(state, func(t *testing.T) {
			dataDir := filepath.Join(t.TempDir(), "instance")
			if err := os.MkdirAll(dataDir, 0o700); err != nil {
				t.Fatal(err)
			}
			store := newLifecycleTestStore(t)
			stored, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
				ID: "stardew", DriverID: DriverID, Name: "test", DataDir: dataDir,
			})
			if err != nil {
				t.Fatal(err)
			}
			stored, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
				ID: stored.ID, State: state, StateMessage: "unsafe", DriverPhase: "unsafe", DriverPayload: `{"kept":true}`,
			})
			if err != nil {
				t.Fatal(err)
			}
			fake, _ := newMaintenanceFake(maintenanceFakeConfig{})
			driver := New(fake, slog.Default(), jobs.NewManager(store, slog.Default()), store)
			transferred := false
			op := NewImportOperationID()
			_, err = driver.ImportSaveAndStart(context.Background(), registry.SaveImportRequest{
				Instance:    registry.Instance{ID: stored.ID, DriverID: stored.DriverID, DataDir: dataDir, State: stored.State},
				OperationID: op, SaveName: "Upload_1", HostHandling: "server_owns_original",
				TransferSourceOwnership: func(string) error { transferred = true; return nil },
			})
			typed, ok := AsImportTransactionError(err)
			if !ok || typed.Code != ImportErrorSaveInProgress {
				t.Fatalf("error=%v", err)
			}
			if transferred {
				t.Fatal("unsafe state transferred upload ownership")
			}
			if _, err := LoadImportJournal(dataDir, op); !os.IsNotExist(err) {
				t.Fatalf("unsafe state created transaction journal: %v", err)
			}
		})
	}
}

func TestImportSaveAndStartRejectsStaleDataDirectoryBeforeFilesystemOwnership(t *testing.T) {
	root := t.TempDir()
	authoritativeDir := filepath.Join(root, "authoritative")
	staleDir := filepath.Join(root, "stale")
	if err := os.MkdirAll(authoritativeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staleDir, 0o700); err != nil {
		t.Fatal(err)
	}
	store := newLifecycleTestStore(t)
	stored, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: "stardew", DriverID: DriverID, Name: "test", DataDir: authoritativeDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	stored, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: stored.ID, State: storage.InstanceStateGameInstalled, DriverPhase: "game_installed", DriverPayload: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	driver := New(&fakeDocker{}, slog.Default(), jobs.NewManager(store, slog.Default()), store)
	transferred := false
	op := NewImportOperationID()
	_, err = driver.ImportSaveAndStart(context.Background(), registry.SaveImportRequest{
		Instance:    registry.Instance{ID: stored.ID, DriverID: stored.DriverID, DataDir: staleDir, State: stored.State},
		OperationID: op, SaveName: "Upload_1", HostHandling: "server_owns_original",
		TransferSourceOwnership: func(string) error { transferred = true; return nil },
	})
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired || transferred {
		t.Fatalf("error=%v transferred=%v", err, transferred)
	}
	if _, err := os.Stat(importJournalPath(authoritativeDir, op)); !os.IsNotExist(err) {
		t.Fatalf("authoritative directory received stale request journal: %v", err)
	}
	if _, err := os.Stat(importJournalPath(staleDir, op)); !os.IsNotExist(err) {
		t.Fatalf("stale directory received transaction journal: %v", err)
	}
}

func TestCleanupUnsubmittedSaveImportRejectsStaleDataDirectory(t *testing.T) {
	root := t.TempDir()
	authoritativeDir := filepath.Join(root, "authoritative")
	staleDir := filepath.Join(root, "stale")
	if err := os.MkdirAll(authoritativeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(staleDir, 0o700); err != nil {
		t.Fatal(err)
	}
	op := NewImportOperationID()
	if _, err := CreateImportJournal(authoritativeDir, registry.SaveImportRequest{
		Instance:    registry.Instance{ID: "stardew", DriverID: DriverID, DataDir: authoritativeDir},
		OperationID: op, SaveName: "Upload_1", HostHandling: "server_owns_original",
	}); err != nil {
		t.Fatal(err)
	}
	staleSentinel := filepath.Join(staleDir, "sentinel")
	if err := os.WriteFile(staleSentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &importMaintenanceStore{instance: storage.Instance{
		ID: "stardew", DriverID: DriverID, DataDir: authoritativeDir, State: storage.InstanceStateGameInstalled,
	}}
	strictDir := ""
	fake := &fakeConsoleDocker{composePsStrictFunc: func(_ context.Context, dir string) (paneldocker.ComposePsResult, error) {
		strictDir = dir
		return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "exited"}}}, nil
	}}
	driver := New(fake, nil, nil, store)
	err := driver.CleanupUnsubmittedSaveImport(context.Background(), registry.Instance{
		ID: "stardew", DriverID: DriverID, DataDir: staleDir, State: storage.InstanceStateGameInstalled,
	}, op)
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorRecoveryRequired || strictDir != authoritativeDir {
		t.Fatalf("error=%v strictDir=%q", err, strictDir)
	}
	if _, err := LoadImportJournal(authoritativeDir, op); err != nil {
		t.Fatalf("authoritative journal was modified: %v", err)
	}
	if content, err := os.ReadFile(staleSentinel); err != nil || string(content) != "keep" {
		t.Fatalf("stale directory was modified: content=%q err=%v", content, err)
	}
}

func TestImportSaveAndStartRejectsRunningComposeBeforeOwnership(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "instance")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	store := newLifecycleTestStore(t)
	stored, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: "stardew", DriverID: DriverID, Name: "test", DataDir: dataDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	stored, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: stored.ID, State: storage.InstanceStateGameInstalled, StateMessage: "installed", DriverPhase: "game_installed", DriverPayload: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	fake, _ := newMaintenanceFake(maintenanceFakeConfig{})
	fake.composePsFunc = func(context.Context, string) (paneldocker.ComposePsResult, error) {
		return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "running", Status: "Up"}}}, nil
	}
	driver := New(fake, slog.Default(), jobs.NewManager(store, slog.Default()), store)
	transferred := false
	op := NewImportOperationID()
	_, err = driver.ImportSaveAndStart(context.Background(), registry.SaveImportRequest{
		Instance:    registry.Instance{ID: stored.ID, DriverID: stored.DriverID, DataDir: dataDir, State: stored.State},
		OperationID: op, SaveName: "Upload_1", HostHandling: "server_owns_original",
		TransferSourceOwnership: func(string) error { transferred = true; return nil },
	})
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorSaveInProgress || transferred {
		t.Fatalf("error=%v transferred=%v", err, transferred)
	}
	if _, err := LoadImportJournal(dataDir, op); !os.IsNotExist(err) {
		t.Fatalf("running Compose created transaction journal: %v", err)
	}
}

func TestImportSaveAndStartRejectsFreshRunningWhenComposeCacheIsStopped(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("controlled Docker Client cache/parser test runs on the Linux target filesystem")
	}
	dataDir := filepath.Join(t.TempDir(), "instance")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	store := newLifecycleTestStore(t)
	stored, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: "stardew", DriverID: DriverID, Name: "test", DataDir: dataDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	stored, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: stored.ID, State: storage.InstanceStateGameInstalled, StateMessage: "installed", DriverPhase: "game_installed", DriverPayload: `{}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	client := paneldocker.NewClient(paneldocker.Options{
		DockerPath:   writeSaveImportProbeDocker(t, "exited", "running"),
		ComposePsTTL: time.Minute,
	})
	if cached, err := client.ComposePs(context.Background(), dataDir); err != nil || len(cached.Services) != 1 || cached.Services[0].State != "exited" {
		t.Fatalf("prime stopped cache result=%+v err=%v", cached, err)
	}
	driver := New(client, slog.Default(), jobs.NewManager(store, slog.Default()), store)
	transferred := false
	op := NewImportOperationID()
	_, err = driver.ImportSaveAndStart(context.Background(), registry.SaveImportRequest{
		Instance:    registry.Instance{ID: stored.ID, DriverID: stored.DriverID, DataDir: dataDir, State: stored.State},
		OperationID: op, SaveName: "Upload_1", HostHandling: "server_owns_original",
		TransferSourceOwnership: func(string) error { transferred = true; return nil },
	})
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorSaveInProgress || transferred {
		t.Fatalf("error=%v transferred=%v", err, transferred)
	}
	if _, err := LoadImportJournal(dataDir, op); !os.IsNotExist(err) {
		t.Fatalf("fresh running state created transaction journal: %v", err)
	}
	if _, err := os.Stat(filepath.Join(savesDir(dataDir), "Saves", importBootstrapSaveName(op))); !os.IsNotExist(err) {
		t.Fatalf("fresh running state created bootstrap data: %v", err)
	}
}

func TestImportSaveAndStartCompletesFromGameInstalledFirstUpload(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "instance")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, ".env"), []byte("IMAGE_VERSION="+TestedImageTag+"\nAPI_PORT=5110\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	modDir := junimoServerModDir(dataDir)
	if err := os.MkdirAll(modDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "manifest.json"), []byte(`{"Version":"`+TestedImageTag+`","UniqueID":"JunimoHost.Server"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "JunimoServer.dll"), []byte("dll"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := installSMAPIMod(dataDir); err != nil {
		t.Fatal(err)
	}
	runtimeManifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(controlDir(dataDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(controlDir(dataDir), "options.json"), []byte(readyControlRuntimeOptions(runtimeManifest.Control.Version)), 0o600); err != nil {
		t.Fatal(err)
	}

	stagedDir := filepath.Join(t.TempDir(), "upload")
	stagedSaveDir := filepath.Join(stagedDir, "Upload_1")
	if err := os.MkdirAll(stagedSaveDir, 0o700); err != nil {
		t.Fatal(err)
	}
	mainXML := `<SaveGame><player><name>Imported</name></player></SaveGame>`
	if err := os.WriteFile(filepath.Join(stagedSaveDir, "Upload_1"), []byte(mainXML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedSaveDir, "SaveGameInfo"), []byte(`<Farmer><name>Imported</name></Farmer>`), 0o600); err != nil {
		t.Fatal(err)
	}

	store := newLifecycleTestStore(t)
	stored, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: "stardew", DriverID: DriverID, Name: "test", DataDir: dataDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	stored, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: stored.ID, State: storage.InstanceStateGameInstalled, StateMessage: "game installed", DriverPhase: "game_installed", DriverPayload: `{"install":"complete"}`,
	})
	if err != nil {
		t.Fatal(err)
	}

	fake, record := newMaintenanceFake(maintenanceFakeConfig{})
	baseExec := fake.execFunc
	fake.execFunc = func(ctx context.Context, dir, service, stdin string, args ...string) (paneldocker.CommandResult, error) {
		if len(args) > 0 && args[0] == "tee" && strings.HasPrefix(stdin, "saves import Upload_1 --reload") {
			if err := writeGameloaderPointer(dataDir, "Upload_1"); err != nil {
				return paneldocker.CommandResult{}, err
			}
			if err := os.WriteFile(filepath.Join(controlDir(dataDir), "status.json"), []byte(`{"saveId":"Upload_1"}`), 0o600); err != nil {
				return paneldocker.CommandResult{}, err
			}
			return baseExec(ctx, dir, service, stdin, args...)
		}
		if len(args) > 0 && args[0] == "curl" && strings.HasSuffix(args[len(args)-1], "/status") {
			return paneldocker.CommandResult{Stdout: `{"playerCount":0,"isOnline":true,"isReady":true,"dayTransitionComplete":true,"version":10}`}, nil
		}
		return baseExec(ctx, dir, service, stdin, args...)
	}
	manager := jobs.NewManager(store, slog.Default())
	driver := New(fake, slog.Default(), manager, store)
	op := NewImportOperationID()
	job, err := driver.ImportSaveAndStart(context.Background(), registry.SaveImportRequest{
		Instance: registry.Instance{ID: stored.ID, DriverID: stored.DriverID, Name: stored.Name, DataDir: dataDir,
			State: stored.State, StateMessage: stored.StateMessage.String, DriverPhase: stored.DriverPhase, DriverPayload: stored.DriverPayload},
		OperationID: op, StagedDir: stagedDir, SaveName: "Upload_1", HostHandling: "server_owns_original",
		AttachJobIdentity: func(string) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		storedJob, loadErr := store.GetJob(context.Background(), job.ID)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		if storedJob.Status == storage.JobStatusSucceeded {
			break
		}
		if storedJob.Status == storage.JobStatusFailed || storedJob.Status == storage.JobStatusCanceled || time.Now().After(deadline) {
			logs, _ := store.ListJobLogs(context.Background(), job.ID, 0, 100)
			t.Fatalf("job status=%s error=%q logs=%+v", storedJob.Status, storedJob.ErrorMessage.String, logs)
		}
		time.Sleep(20 * time.Millisecond)
	}
	journal, err := LoadImportJournal(dataDir, op)
	if err != nil || journal.Stage != ImportStageCompleted || journal.BootstrapSaveName == "" || !journal.BootstrapCleanupCompleted {
		t.Fatalf("journal=%+v err=%v", journal, err)
	}
	if GetActiveSaveName(dataDir) != "Upload_1" {
		t.Fatalf("active save=%q", GetActiveSaveName(dataDir))
	}
	record.mu.Lock()
	commands := strings.Join(record.stdin, "\n")
	record.mu.Unlock()
	if strings.Count(commands, "saves import Upload_1 --reload") != 1 {
		t.Fatalf("import command count mismatch: %q", commands)
	}
	if _, err := os.Stat(filepath.Join(savesDir(dataDir), "Saves", journal.BootstrapSaveName)); !os.IsNotExist(err) {
		t.Fatalf("completed bootstrap still exists: %v", err)
	}
}

func TestValidateImportCapabilityVersionDLLAndFIFO(t *testing.T) {
	writeRuntime := func(t *testing.T, version, modVersion string, fifo bool) error {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("IMAGE_VERSION="+version+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		modDir := junimoServerModDir(dir)
		if err := os.MkdirAll(modDir, 0o700); err != nil {
			t.Fatal(err)
		}
		manifest := `{"Name":"JunimoServer","Version":"` + modVersion + `","UniqueID":"JunimoHost.Server"}`
		if err := os.WriteFile(filepath.Join(modDir, "manifest.json"), []byte(manifest), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(modDir, "JunimoServer.dll"), []byte("dll"), 0o600); err != nil {
			t.Fatal(err)
		}
		return ValidateImportCapability(dir, fifo)
	}
	for _, tc := range []struct {
		name, image, mod string
		fifo, wantOK     bool
	}{
		{"121", "1.5.0-preview.121", "1.5.0-preview.121", true, false},
		{"125", "1.5.0-preview.125", "1.5.0-preview.125", true, true},
		{"126_not_yet_qualified", "1.5.0-preview.126", "1.5.0-preview.126", true, false},
		{"old_dll", "1.5.0-preview.125", "1.5.0-preview.121", true, false},
		{"fifo_missing", "1.5.0-preview.125", "1.5.0-preview.125", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := writeRuntime(t, tc.image, tc.mod, tc.fifo)
			if (err == nil) != tc.wantOK {
				t.Fatalf("err=%v wantOK=%v", err, tc.wantOK)
			}
		})
	}
}

func TestPrepareImportRuntimeAssetsSynchronizesFreshInstanceBeforeValidation(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, ".env"), []byte("IMAGE_VERSION="+TestedImageTag+"\nSERVER_IMAGE=sdvd/server:"+TestedImageTag+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	extractor := &fakeDocker{}
	docker := &fakeConsoleDocker{runContainerFunc: extractor.RunContainerTTY}
	driver := New(docker, nil, nil, nil)
	instance := registry.Instance{ID: "fresh", DriverID: DriverID, DataDir: dataDir, State: "stopped"}
	if err := driver.prepareImportRuntimeAssets(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	if extractor.containerRuns != 1 {
		t.Fatalf("Junimo extraction runs=%d, want 1", extractor.containerRuns)
	}
	if err := validateImportStaticCapability(dataDir); err != nil {
		t.Fatalf("synchronized runtime failed validation: %v", err)
	}
	if err := driver.prepareImportRuntimeAssets(context.Background(), instance); err != nil {
		t.Fatalf("idempotent runtime preparation failed: %v", err)
	}
	if extractor.containerRuns != 1 {
		t.Fatalf("idempotent preparation extracted again: %d", extractor.containerRuns)
	}
}

func TestPrepareImportRuntimeAssetsKeepsVersionMismatchAsUpgradeError(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, ".env"), []byte("IMAGE_VERSION=1.5.0-preview.124\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	extractor := &fakeDocker{}
	driver := New(&fakeConsoleDocker{runContainerFunc: extractor.RunContainerTTY}, nil, nil, nil)
	err := driver.prepareImportRuntimeAssets(context.Background(), registry.Instance{ID: "old", DriverID: DriverID, DataDir: dataDir})
	typed, ok := AsImportTransactionError(err)
	if !ok || typed.Code != ImportErrorUnsupported || extractor.containerRuns != 0 {
		t.Fatalf("error=%v extractionRuns=%d", err, extractor.containerRuns)
	}
}

func TestPrepareImportRuntimeAssetsReportsPreparationFailuresWithoutUpgradeAdvice(t *testing.T) {
	t.Run("runtime config unreadable", func(t *testing.T) {
		driver := New(&fakeConsoleDocker{}, nil, nil, nil)
		err := driver.prepareImportRuntimeAssets(context.Background(), registry.Instance{ID: "missing-env", DriverID: DriverID, DataDir: t.TempDir()})
		typed, ok := AsImportTransactionError(err)
		if !ok || typed.Code != ImportErrorRuntimePrepare {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("image extraction failed", func(t *testing.T) {
		dataDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dataDir, ".env"), []byte("IMAGE_VERSION="+TestedImageTag+"\nSERVER_IMAGE=sdvd/server:"+TestedImageTag+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		docker := &fakeConsoleDocker{runContainerFunc: func(context.Context, paneldocker.ContainerTTYRunOpts, <-chan string, func(string)) (int, error) {
			return 1, errors.New("test extraction failure")
		}}
		driver := New(docker, nil, nil, nil)
		err := driver.prepareImportRuntimeAssets(context.Background(), registry.Instance{ID: "extract-failure", DriverID: DriverID, DataDir: dataDir})
		typed, ok := AsImportTransactionError(err)
		if !ok || typed.Code != ImportErrorRuntimePrepare {
			t.Fatalf("error=%v", err)
		}
	})
}
