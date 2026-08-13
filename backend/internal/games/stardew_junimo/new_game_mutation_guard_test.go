package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestMutationExecutorsKeepOwnerCheckAndWriteInOneCriticalSection(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(*Driver, registry.Instance, func() error) error
	}{
		{name: "ownership", run: func(driver *Driver, instance registry.Instance, mutate func() error) error {
			return driver.WithMutationOwnership(context.Background(), instance, mutate)
		}},
		{name: "offline", run: func(driver *Driver, instance registry.Instance, mutate func() error) error {
			return driver.WithOfflineMutation(context.Background(), instance, mutate)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			driver, _, instance, _ := setupRuntimeUpdateDriver(t, storage.InstanceStateStopped)
			driver.runtimeUpdateMu.Lock()
			var writes atomic.Int32
			result := make(chan error, 1)
			go func() {
				result <- test.run(driver, instance, func() error {
					writes.Add(1)
					return nil
				})
			}()
			select {
			case err := <-result:
				driver.runtimeUpdateMu.Unlock()
				t.Fatalf("mutation escaped runtimeUpdateMu: %v", err)
			case <-time.After(50 * time.Millisecond):
			}
			seedUnfinishedNewGameOwnerForSettingsTest(t, instance.DataDir)
			driver.runtimeUpdateMu.Unlock()
			err := <-result
			var ownerErr *NewGameOwnerError
			if !errors.As(err, &ownerErr) || ownerErr.Code != "new_game_in_progress" {
				t.Fatalf("mutation error=%v, want new_game_in_progress", err)
			}
			if writes.Load() != 0 {
				t.Fatalf("mutation callback ran %d times after owner appeared", writes.Load())
			}
		})
	}
}

func TestUnfinishedNewGameOwnerGuardsEveryInstanceMutationEntry(t *testing.T) {
	driver, _, instance, fake := setupRuntimeUpdateDriver(t, storage.InstanceStateStopped)
	dataDir := instance.DataDir
	tx, _, err := beginOrResumeNewGameTransaction(
		dataDir,
		newGameTestConfig("standard"),
		"request-mutation-guard",
		"job-mutation-guard",
	)
	if err != nil {
		t.Fatalf("create persistent new-game owner: %v", err)
	}
	wantRecord := tx.record
	tests := []struct {
		name string
		run  func() error
	}{
		{name: "offline mutation guard", run: func() error {
			return driver.EnsureOfflineMutationAllowed(context.Background(), instance)
		}},
		{name: "Panel bootstrap prepare", run: func() error {
			return driver.Prepare(context.Background(), instance)
		}},
		{name: "install", run: func() error {
			_, err := driver.Install(context.Background(), registry.InstallRequest{Instance: instance})
			return err
		}},
		{name: "prepare farm mods", run: func() error {
			_, err := driver.PrepareFarmMods(context.Background(), instance, "standard")
			return err
		}},
		{name: "runtime dry-run", run: func() error {
			_, err := driver.StartRuntimeUpdateDryRun(context.Background(), instance, 1)
			return err
		}},
		{name: "runtime apply", run: func() error {
			_, err := driver.StartRuntimeUpdateApply(context.Background(), instance, 1)
			return err
		}},
		{name: "runtime repair", run: func() error {
			_, err := driver.StartRuntimeUpdateRepair(context.Background(), instance, 1)
			return err
		}},
		{name: "runtime config repair", run: func() error {
			_, err := driver.RepairRuntimeStackConfig(context.Background(), instance)
			return err
		}},
		{name: "runtime Panel-restart recovery", run: func() error {
			return driver.RecoverRuntimeUpdateApply(context.Background(), instance)
		}},
		{name: "SMAPI dry-run", run: func() error {
			_, err := driver.RunSMAPIUpdateDryRun(context.Background(), instance)
			return err
		}},
		{name: "SMAPI apply", run: func() error {
			_, err := driver.StartSMAPIUpdateApply(context.Background(), instance, 1)
			return err
		}},
		{name: "SMAPI Panel-restart recovery", run: func() error {
			return driver.RecoverSMAPIUpdateApply(context.Background(), instance)
		}},
		{name: "import save and start", run: func() error {
			_, err := driver.ImportSaveAndStart(context.Background(), registry.SaveImportRequest{Instance: instance})
			return err
		}},
		{name: "delete farmhand", run: func() error {
			_, err := driver.DeleteFarmhand(context.Background(), FarmhandDeleteRequest{Instance: instance})
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			beforeFiles := snapshotMutationGuardFiles(t, dataDir)
			beforeDockerCalls := len(fake.calls)
			err := test.run()
			var ownerErr *NewGameOwnerError
			if !errors.As(err, &ownerErr) || ownerErr.Code != "new_game_in_progress" {
				t.Fatalf("error = %T %v, want new_game_in_progress", err, err)
			}
			after, loadErr := LoadNewGameTransaction(dataDir, wantRecord.TransactionID)
			if loadErr != nil {
				t.Fatalf("load owner transaction after rejection: %v", loadErr)
			}
			if after.Stage != wantRecord.Stage || after.OwnerToken != wantRecord.OwnerToken || after.JobID != wantRecord.JobID {
				t.Fatalf("owner transaction mutated after rejection: before=%+v after=%+v", wantRecord, after)
			}
			if got := snapshotMutationGuardFiles(t, dataDir); !reflect.DeepEqual(got, beforeFiles) {
				t.Fatalf("instance files mutated after owner rejection: before=%v after=%v", beforeFiles, got)
			}
			if len(fake.calls) != beforeDockerCalls {
				t.Fatalf("Docker called after owner rejection: %v", fake.calls[beforeDockerCalls:])
			}
		})
	}
}

func snapshotMutationGuardFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[relative] = string(data)
		return nil
	}); err != nil {
		t.Fatalf("snapshot instance files: %v", err)
	}
	return files
}

func TestPanelBootstrapCoordinatorsDoNotActOnUnfinishedNewGameOwner(t *testing.T) {
	driver, _, instance, fake := setupRuntimeUpdateDriver(t, storage.InstanceStateStopped)
	dataDir := instance.DataDir
	if _, _, err := beginOrResumeNewGameTransaction(
		dataDir,
		newGameTestConfig("standard"),
		"request-bootstrap-guard",
		"job-bootstrap-guard",
	); err != nil {
		t.Fatalf("create persistent new-game owner: %v", err)
	}
	beforeRequiredFiles := snapshotMutationGuardFiles(t, dataDir)
	beforeDockerCalls := len(fake.calls)
	driver.StartRequiredRuntimeUpdate(context.Background(), instance)
	if _, err := os.Stat(requiredRuntimeUpdateStatusPath(dataDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("required runtime coordinator wrote status under new-game owner: %v", err)
	}
	driver.requiredRuntimeMu.Lock()
	started := driver.requiredRuntimeRunning[instance.ID]
	driver.requiredRuntimeMu.Unlock()
	if started {
		t.Fatal("required runtime coordinator started under new-game owner")
	}
	if got := snapshotMutationGuardFiles(t, dataDir); !reflect.DeepEqual(got, beforeRequiredFiles) {
		t.Fatalf("required runtime coordinator mutated instance files: before=%v after=%v", beforeRequiredFiles, got)
	}
	if len(fake.calls) != beforeDockerCalls {
		t.Fatalf("required runtime coordinator called Docker: %v", fake.calls[beforeDockerCalls:])
	}

	eventDir := filepath.Join(dataDir, ".local-container", "control", "save-events")
	if err := os.MkdirAll(eventDir, 0o755); err != nil {
		t.Fatal(err)
	}
	eventPath := filepath.Join(eventDir, "must-remain.json")
	if err := os.WriteFile(eventPath, []byte("invalid but owned evidence"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	driver.RunBackupMaintenanceScheduler(ctx, []registry.Instance{instance})
	if _, err := os.Stat(eventPath); err != nil {
		t.Fatalf("bootstrap backup maintenance consumed owner evidence: %v", err)
	}
	if len(fake.calls) != beforeDockerCalls {
		t.Fatalf("backup maintenance called Docker under new-game owner: %v", fake.calls[beforeDockerCalls:])
	}
}
