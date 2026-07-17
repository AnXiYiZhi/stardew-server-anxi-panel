package updater

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeDatabaseBackupper struct {
	err   error
	calls int
}

func (f *fakeDatabaseBackupper) BackupTo(_ context.Context, target string) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	return os.WriteFile(target, []byte("consistent sqlite backup"), 0o600)
}

func newApplyServiceTest(t *testing.T, database *fakeDatabaseBackupper) (*Service, *fakeRuntime) {
	t.Helper()
	dataDir := t.TempDir()
	installDir := t.TempDir()
	composeFile := filepath.Join(installDir, "docker-compose.yml")
	runtime := &fakeRuntime{dockerOK: true, composeOK: true, info: standardContainer(composeFile, dataDir)}
	service := NewService(ServiceOptions{
		Docker: runtime, DataDir: dataDir, DatabasePath: filepath.Join(dataDir, "panel.db"), Database: database,
		ContainerRef: "1234567890ab", ContainerDataDir: "/data",
	})
	return service, runtime
}

func TestApplyServiceCreatesBackupBeforeStartingHelper(t *testing.T) {
	database := &fakeDatabaseBackupper{}
	service, runtime := newApplyServiceTest(t, database)
	status, err := service.StartApply(context.Background(), "0.1.14", "v0.1.15")
	if err != nil {
		t.Fatal(err)
	}
	if database.calls != 1 || len(runtime.applySpecs) != 1 {
		t.Fatalf("backup calls=%d helper=%d", database.calls, len(runtime.applySpecs))
	}
	if status.Phase != PhaseBackingUp || runtime.applySpecs[0].TargetVersion != "0.1.15" {
		t.Fatalf("status=%+v spec=%+v", status, runtime.applySpecs[0])
	}
	if _, err := os.Stat(filepath.Join(service.dataDir, "updater", "backups", status.UpdateID, "panel.db")); err != nil {
		t.Fatal(err)
	}
}

func TestApplyServiceDatabaseBackupFailurePreventsHelper(t *testing.T) {
	database := &fakeDatabaseBackupper{err: errors.New("disk full")}
	service, runtime := newApplyServiceTest(t, database)
	status, _ := service.StartApply(context.Background(), "0.1.14", "0.1.15")
	if status.Phase != PhaseFailedRolledBack || status.ErrorCode != CodeDatabaseBackupFailed {
		t.Fatalf("status=%+v", status)
	}
	if len(runtime.applySpecs) != 0 {
		t.Fatal("helper started despite failed database backup")
	}
}

func TestApplyServiceRejectsConcurrentUpgrade(t *testing.T) {
	service, _ := newApplyServiceTest(t, &fakeDatabaseBackupper{})
	if err := service.applyStore.Write(ApplyStatus{UpdateID: "active", Phase: PhasePulling, Logs: []LogEntry{}}); err != nil {
		t.Fatal(err)
	}
	status, err := service.StartApply(context.Background(), "0.1.14", "0.1.15")
	validation, ok := IsValidationError(err)
	if !ok || validation.Code != CodeUpdateInProgress || status.UpdateID != "active" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}

func TestApplyStatusSurvivesServiceRestartAndIsAtomicallyReplaced(t *testing.T) {
	service, runtime := newApplyServiceTest(t, &fakeDatabaseBackupper{})
	want := ApplyStatus{UpdateID: "persisted", Phase: PhaseWaitingHealth, Progress: 75, FromVersion: "0.1.14", ToVersion: "0.1.15", Logs: []LogEntry{}}
	if err := service.applyStore.Write(want); err != nil {
		t.Fatal(err)
	}
	restarted := NewService(ServiceOptions{
		Docker: runtime, DataDir: service.dataDir, DatabasePath: filepath.Join(service.dataDir, "panel.db"),
		ContainerRef: "1234567890ab", ContainerDataDir: "/data",
	})
	got, err := restarted.ApplyStatus()
	if err != nil || got.UpdateID != want.UpdateID || got.Phase != want.Phase || got.Progress != want.Progress {
		t.Fatalf("status after restart=%+v err=%v", got, err)
	}
	temps, err := filepath.Glob(filepath.Join(service.dataDir, "updater", ".apply-*.tmp"))
	if err != nil || len(temps) != 0 {
		t.Fatalf("atomic temp files=%v err=%v", temps, err)
	}
}

func TestApplyServiceRejectsDevSameAndDowngrade(t *testing.T) {
	service, _ := newApplyServiceTest(t, &fakeDatabaseBackupper{})
	for _, versions := range [][2]string{{"dev", "0.1.15"}, {"0.1.15", "0.1.15"}, {"0.1.16", "0.1.15"}} {
		if _, err := service.StartApply(context.Background(), versions[0], versions[1]); err == nil {
			t.Fatalf("versions %v accepted", versions)
		}
	}
}

func TestReconcileCompletedImageCleanupFromPreviousReleaseHelper(t *testing.T) {
	service, _ := newApplyServiceTest(t, &fakeDatabaseBackupper{})
	oldImage := "anxiyizhi/stardew-server-anxi-panel:0.1.14"
	newImage := "anxiyizhi/stardew-server-anxi-panel:0.1.15"
	executor := &applyScenarioExecutor{oldImage: oldImage, newImage: newImage}
	status := ApplyStatus{
		UpdateID: "previous-helper", Phase: PhaseSucceeded, FromVersion: "0.1.14", ToVersion: "0.1.15",
		OriginalImage: oldImage, OriginalDigest: oldImage + "@sha256:old",
		SelectedImage: newImage, SelectedDigest: newImage + "@sha256:new", Logs: []LogEntry{},
	}
	if err := service.applyStore.Write(status); err != nil {
		t.Fatal(err)
	}
	done, err := service.reconcileCompletedImageCleanupOnce(context.Background(), "0.1.15", executor)
	if err != nil || !done {
		t.Fatalf("done=%v err=%v", done, err)
	}
	updated, err := service.applyStore.Read()
	if err != nil || !updated.CleanupCompleted {
		t.Fatalf("status=%+v err=%v", updated, err)
	}
	joined := flattenCalls(executor.calls)
	if !strings.Contains(joined, "image rm "+oldImage) || !strings.Contains(joined, "image prune") {
		t.Fatalf("previous helper cleanup was not reconciled: %s", joined)
	}
}

func TestReconcileImageCleanupWaitsForMatchingActiveUpgradeOnly(t *testing.T) {
	service, _ := newApplyServiceTest(t, &fakeDatabaseBackupper{})
	if err := service.applyStore.Write(ApplyStatus{UpdateID: "active", Phase: PhaseWaitingHealth, ToVersion: "0.1.15"}); err != nil {
		t.Fatal(err)
	}
	done, err := service.reconcileCompletedImageCleanupOnce(context.Background(), "0.1.15", &applyScenarioExecutor{})
	if err != nil || done {
		t.Fatalf("matching active upgrade should keep reconciliation alive: done=%v err=%v", done, err)
	}
	done, err = service.reconcileCompletedImageCleanupOnce(context.Background(), "0.1.16", &applyScenarioExecutor{})
	if err != nil || !done {
		t.Fatalf("unrelated target should not be reconciled: done=%v err=%v", done, err)
	}
}
