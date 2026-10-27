package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type junimo140CoordinatorStore struct {
	StateStore
	instances []storage.Instance
	restored  []storage.Instance
}

func (s *junimo140CoordinatorStore) ListInstances(context.Context) ([]storage.Instance, error) {
	return append([]storage.Instance(nil), s.instances...), nil
}
func (s *junimo140CoordinatorStore) GetInstance(_ context.Context, id string) (storage.Instance, error) {
	for _, instance := range s.instances {
		if instance.ID == id {
			return instance, nil
		}
	}
	return storage.Instance{}, storage.ErrNotFound
}
func (s *junimo140CoordinatorStore) RestoreInstanceStateSnapshot(_ context.Context, state storage.Instance) (storage.Instance, error) {
	s.restored = append(s.restored, state)
	for i := range s.instances {
		if s.instances[i].ID == state.ID {
			s.instances[i] = state
		}
	}
	return state, nil
}

type junimo140CoordinatorDocker struct {
	junimo140GroupDocker
	plan         junimo140GroupPlan
	txn          junimo140GroupTransaction
	services     []paneldocker.RuntimeJunimo140ServiceSnapshot
	events       []string
	originals    map[string]map[string][]byte
	failRestore  string
	failHolder   bool
	removedStage bool
	failHealth   bool
}

func (f *junimo140CoordinatorDocker) RuntimeServerHealth(context.Context, string, string) error {
	if f.failHealth {
		return errors.New("original runtime health failed")
	}
	return nil
}
func (f *junimo140CoordinatorDocker) ComposeExecPipe(_ context.Context, _ string, _ string, _ string, args ...string) (paneldocker.CommandResult, error) {
	if len(args) > 0 && args[0] == "wc" {
		return paneldocker.CommandResult{Stdout: "0\n"}, nil
	}
	if len(args) > 0 && args[0] == "tail" {
		return paneldocker.CommandResult{Stdout: "--- Server Info ---\nVersion: 1.5.0-preview.125\n"}, nil
	}
	return paneldocker.CommandResult{}, nil
}

func (f *junimo140CoordinatorDocker) RuntimeSnapshotJunimo140Services(context.Context, string, []paneldocker.RuntimeJunimo140WorldScope, []string) ([]paneldocker.RuntimeJunimo140ServiceSnapshot, error) {
	f.events = append(f.events, "snapshot")
	return append([]paneldocker.RuntimeJunimo140ServiceSnapshot(nil), f.services...), nil
}
func (f *junimo140CoordinatorDocker) RuntimeStopJunimo140Helpers(context.Context, string, paneldocker.RuntimeJunimo140HelperScope) error {
	f.events = append(f.events, "helpers stopped")
	return nil
}
func (f *junimo140CoordinatorDocker) RuntimeStopJunimo140Services(_ context.Context, _ string, snapshots []paneldocker.RuntimeJunimo140ServiceSnapshot, _ map[string][]string) error {
	f.events = append(f.events, "services stopped")
	for i := range f.services {
		f.services[i].Running = false
	}
	return nil
}
func (f *junimo140CoordinatorDocker) RuntimeAssertJunimo140GameGroup(_ context.Context, _ string, _ string, _ []string, quiescent bool) error {
	if !quiescent {
		return errors.New("recovery requested a live group")
	}
	f.events = append(f.events, "holder check")
	if f.failHolder {
		return errors.New("unknown volume writer")
	}
	return nil
}
func (f *junimo140CoordinatorDocker) RuntimeRemoveJunimo140Services(_ context.Context, _ string, snapshots []paneldocker.RuntimeJunimo140ServiceSnapshot, _ map[string][]string) error {
	f.events = append(f.events, "services removed")
	for _, snapshot := range snapshots {
		if snapshot.Running {
			return errors.New("removing running service")
		}
	}
	f.services = nil
	return nil
}
func (f *junimo140CoordinatorDocker) RuntimeRestoreJunimo140Instance(_ context.Context, _ string, request paneldocker.RuntimeInstanceArchiveRequest) error {
	f.events = append(f.events, "restore "+request.Project)
	if f.failRestore == request.Project {
		f.failRestore = ""
		_ = os.Remove(filepath.Join(request.InstanceHostDir, ".env"))
		return errors.New("injected interruption after root purge")
	}
	for name, data := range f.originals[request.Project] {
		if err := os.WriteFile(filepath.Join(request.InstanceHostDir, name), data, 0600); err != nil {
			return err
		}
	}
	return nil
}
func (f *junimo140CoordinatorDocker) RuntimeCreateJunimo140ServicePinned(_ context.Context, _ string, project, service, imageID string) error {
	f.events = append(f.events, "create stopped "+project+"/"+service)
	f.services = append(f.services, paneldocker.RuntimeJunimo140ServiceSnapshot{Project: project, Service: service, RuntimeServiceMetadata: paneldocker.RuntimeServiceMetadata{ContainerID: strings.Repeat("e", 64), ImageID: imageID}, GameVolume: f.plan.SourceVolume})
	return nil
}
func (f *junimo140CoordinatorDocker) RuntimeRemoveJunimo140Staging(context.Context, string, string, string, string) error {
	f.events = append(f.events, "stage removed")
	for _, service := range f.services {
		if service.GameVolume == f.plan.StagingVolume {
			return errors.New("stage still attached")
		}
	}
	f.removedStage = true
	return nil
}
func (f *junimo140CoordinatorDocker) RuntimeStartJunimo140Services(_ context.Context, _ string, snapshots []paneldocker.RuntimeJunimo140ServiceSnapshot, _ map[string][]string) error {
	for _, snapshot := range snapshots {
		f.events = append(f.events, "start "+snapshot.Project+"/"+snapshot.Service)
		for i := range f.services {
			if f.services[i].Project == snapshot.Project && f.services[i].Service == snapshot.Service {
				f.services[i].Running = true
			}
		}
	}
	return nil
}

func setupJunimo140CoordinatorRecovery(t *testing.T) (*Driver, *junimo140CoordinatorDocker, *junimo140CoordinatorStore, junimo140GroupJournal, junimo140GroupTransaction) {
	t.Helper()
	plan, txn := junimo140TransactionFixture(t)
	store := &junimo140CoordinatorStore{}
	fake := &junimo140CoordinatorDocker{plan: plan, originals: map[string]map[string][]byte{}}
	for i, member := range plan.Members {
		compose := []byte("original compose " + member.ID + "\n")
		if err := os.WriteFile(filepath.Join(member.DataDir, "docker-compose.yml"), compose, 0600); err != nil {
			t.Fatal(err)
		}
		env, err := os.ReadFile(filepath.Join(member.DataDir, ".env"))
		if err != nil {
			t.Fatal(err)
		}
		fake.originals[member.Project] = map[string][]byte{".env": env, "docker-compose.yml": compose}
		world := &txn.Worlds[i]
		world.Original.EnvSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(member.DataDir, ".env"))
		world.Original.ComposeSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(member.DataDir, "docker-compose.yml"))
		world.Original.Instance.DriverPayload = `{"keep":"long-lived"}`
		world.Original.Current.Server.Tag = "1.5.0-preview.125"
		world.Original.ServerContainerID = strings.Repeat(string(rune('a'+i)), 64)
		if i == 0 {
			world.Original.ServerWasRunning = true
			world.Original.Instance.State = storage.InstanceStateRunning
			world.SaveIntent, world.Saved, world.SaveCommandID = true, true, strings.Repeat("a", 32)
		}
		world.UpdateIntent, world.Updated = true, true
		store.instances = append(store.instances, world.Original.Instance)
		fake.services = append(fake.services, paneldocker.RuntimeJunimo140ServiceSnapshot{Project: member.Project, Service: "server", RuntimeServiceMetadata: paneldocker.RuntimeServiceMetadata{ContainerID: strings.Repeat(string(rune('c'+i)), 64), ImageID: txn.Target.Server.ImageID}, Running: true, GameVolume: plan.StagingVolume})
	}
	readyJunimo140Transaction(&txn)
	txn.InputSHA256 = txn.inputDigest()
	fake.txn = txn
	journal, err := claimJunimo140Group(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJunimo140GroupTransaction(plan, &txn); err != nil {
		t.Fatal(err)
	}
	driver := New(fake, nil, nil, store)
	driver.runtimeUpdateServerTimeout = 10 * time.Millisecond
	driver.runtimeUpdatePollInterval = time.Millisecond
	return driver, fake, store, journal, txn
}

func TestJunimo140CoordinatorRollbackRestoresWholeGroupAndRunningChoice(t *testing.T) {
	for _, bootstrap := range []bool{false, true} {
		t.Run(map[bool]string{false: "live rollback", true: "bootstrap keeps game stopped"}[bootstrap], func(t *testing.T) {
			driver, fake, store, journal, txn := setupJunimo140CoordinatorRecovery(t)
			if err := driver.rollbackJunimo140Group(context.Background(), fake, &journal, &txn, bootstrap); err != nil {
				t.Fatal(err)
			}
			if txn.Phase != "rolled_back" || !fake.removedStage || len(store.restored) != len(journal.Plan.Members) {
				t.Fatal("incomplete parent rollback")
			}
			starts := 0
			for _, event := range fake.events {
				if strings.HasPrefix(event, "start ") {
					starts++
				}
			}
			if starts != map[bool]int{false: 1, true: 0}[bootstrap] {
				t.Fatalf("wrong running restoration: %v", fake.events)
			}
			for _, member := range journal.Plan.Members {
				if err := rejectJunimo140GroupLease(member.DataDir); err != nil {
					t.Fatal("terminal member lease retained", err)
				}
			}
			for _, state := range store.restored {
				if state.DriverPayload != `{"keep":"long-lived"}` {
					t.Fatal("long-lived database payload lost")
				}
			}
			sequence := strings.Join(fake.events, "\n")
			if strings.Index(sequence, "services stopped") > strings.Index(sequence, "restore ") || strings.Index(sequence, "restore ") > strings.Index(sequence, "create stopped") {
				t.Fatal("unsafe restore ordering", sequence)
			}
		})
	}
}

func TestJunimo140CoordinatorRecoveryReentersAfterPartialRootPurge(t *testing.T) {
	driver, fake, _, journal, txn := setupJunimo140CoordinatorRecovery(t)
	fake.failRestore = journal.Plan.Members[1].Project
	if err := driver.rollbackJunimo140Group(context.Background(), fake, &journal, &txn, true); err == nil {
		t.Fatal("injected restore interruption ignored")
	}
	missing := journal.Plan.Members[1]
	if _, err := os.Stat(filepath.Join(missing.DataDir, ".env")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("fixture did not purge env")
	}
	if err := rejectUnfinishedNewGameOwner(missing.DataDir); err == nil {
		t.Fatal("partial rollback lost peer mutation fence")
	}
	if err := driver.RecoverRuntimeUpdateApply(context.Background(), registry.Instance{ID: missing.ID, DriverID: DriverID, DataDir: missing.DataDir}); err != nil {
		t.Fatal(err)
	}
	for _, member := range journal.Plan.Members {
		if err := rejectJunimo140GroupLease(member.DataDir); err != nil {
			t.Fatal("recovered lease retained", err)
		}
	}
	for _, event := range fake.events {
		if strings.HasPrefix(event, "start ") {
			t.Fatal("bootstrap started game", fake.events)
		}
	}
	if !fake.removedStage {
		t.Fatal("recovery left unused staging")
	}
}

func TestJunimo140CoordinatorUnknownHolderPreservesRecoveryMaterials(t *testing.T) {
	driver, fake, _, journal, txn := setupJunimo140CoordinatorRecovery(t)
	fake.failHolder = true
	if err := driver.rollbackJunimo140Group(context.Background(), fake, &journal, &txn, true); err == nil {
		t.Fatal("unknown holder accepted")
	}
	for _, event := range fake.events {
		if strings.HasPrefix(event, "restore ") || event == "services removed" || event == "stage removed" {
			t.Fatal("unknown writer caused destructive recovery", fake.events)
		}
	}
	for _, member := range journal.Plan.Members {
		if err := rejectJunimo140GroupLease(member.DataDir); err == nil {
			t.Fatal("failed rollback released peer")
		}
	}
}

func TestJunimo140CoordinatorDoesNotReportRecoveredWhenOldWorldIsUnhealthy(t *testing.T) {
	driver, fake, _, journal, txn := setupJunimo140CoordinatorRecovery(t)
	fake.failHealth = true
	if err := driver.rollbackJunimo140Group(context.Background(), fake, &journal, &txn, false); err == nil {
		t.Fatal("unhealthy original world reported recovered")
	}
	if txn.Phase != "rollback_failed" {
		t.Fatal("unhealthy rollback lost durable failure")
	}
	for _, member := range journal.Plan.Members {
		if err := rejectJunimo140GroupLease(member.DataDir); err == nil {
			t.Fatal("unhealthy rollback released peer")
		}
	}
}

func TestJunimo140LeaseFenceAllowsFreshUnpreparedInstance(t *testing.T) {
	root := filepath.Join(t.TempDir(), "not-prepared")
	if err := rejectUnfinishedNewGameOwner(root); err != nil {
		t.Fatal("missing instance root treated as an active shared lease", err)
	}
	driver := New(nil, nil, nil, nil)
	if handled, err := driver.recoverJunimo140GroupLocked(context.Background(), registry.Instance{ID: "fresh", DriverID: DriverID, DataDir: root}); handled || err != nil {
		t.Fatal("fresh bootstrap requires shared recovery", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".local-container"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(junimo140GroupLeasePath(root), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := rejectJunimo140GroupLease(root); err == nil {
		t.Fatal("corrupt present owner treated as absent")
	}
}

func TestJunimo140GroupRequiresExactScopeBeforeAnyDockerOperation(t *testing.T) {
	driver := New(nil, nil, nil, nil)
	dryRun := RuntimeUpdateDryRunStatus{SharedGroupSHA256: strings.Repeat("a", 64), SharedGroupMembers: 2}
	for _, scope := range []string{"", strings.Repeat("b", 64), strings.Repeat("a", 63)} {
		_, err := driver.startJunimo140GroupLocked(context.Background(), registry.Instance{}, RuntimeUpdateApplyStatus{}, dryRun, scope)
		validation, ok := IsRuntimeUpdateValidationError(err)
		if !ok || validation.Code != "shared_group_confirmation_required" {
			t.Fatal("unconfirmed scope reached Docker contract", err)
		}
	}
}

func TestJunimo140TerminalPeerLeaseCleanupRetainsDurableRunningPolicy(t *testing.T) {
	for _, bootstrap := range []bool{false, true} {
		t.Run(map[bool]string{false: "live rollback", true: "bootstrap rollback"}[bootstrap], func(t *testing.T) {
			driver, fake, _, journal, txn := setupJunimo140CoordinatorRecovery(t)
			if err := driver.rollbackJunimo140Group(context.Background(), fake, &journal, &txn, bootstrap); err != nil {
				t.Fatal(err)
			}
			if txn.KeepServerStopped != bootstrap {
				t.Fatal("rollback did not persist its running policy")
			}
			lease := junimo140GroupLease{SchemaVersion: 1, ApplyID: journal.Plan.ApplyID, PrimaryID: journal.Plan.PrimaryID, PlanSHA256: journal.PlanSHA256}
			raw, err := json.Marshal(lease)
			if err != nil {
				t.Fatal(err)
			}
			member := journal.Plan.Members[0]
			if err := os.WriteFile(junimo140GroupLeasePath(member.DataDir), raw, 0600); err != nil {
				t.Fatal(err)
			}
			eventsBefore := len(fake.events)
			if err := driver.RecoverRuntimeUpdateApply(context.Background(), registry.Instance{ID: member.ID, DriverID: DriverID, DataDir: member.DataDir}); err != nil {
				t.Fatal(err)
			}
			status, err := readRuntimeUpdateApplyStatus(member.DataDir)
			if err != nil {
				t.Fatal(err)
			}
			if status.ServerRunning == bootstrap || len(fake.events) != eventsBefore {
				t.Fatal("terminal lease cleanup changed or misreported the completed runtime policy")
			}
		})
	}
}
