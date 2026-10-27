package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type junimo140PrepareFailureDocker struct {
	*junimo140CoordinatorDocker
	archives int
}

type junimo140ColdWorldDocker struct{ *junimo140CoordinatorDocker }

func (f *junimo140ColdWorldDocker) RuntimePrepareJunimo140Permissions(context.Context, string, paneldocker.RuntimeInstanceArchiveRequest, string, string) error {
	return nil
}
func (f *junimo140ColdWorldDocker) RuntimeComposeConfigValidateServerImage(context.Context, string, string, string) error {
	return nil
}
func (f *junimo140ColdWorldDocker) RuntimeExtractJunimo140Mod(_ context.Context, _ string, request paneldocker.RuntimeInstanceArchiveRequest) error {
	dir := filepath.Join(request.RecoveryHostDir, "target-mod-"+strings.TrimSuffix(request.Filename, ".tar"))
	if err := os.Mkdir(dir, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"Name":"JunimoServer","Version":"1.5.0-preview.140","UniqueID":"JunimoHost.Server"}`), 0600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "JunimoServer.dll"), []byte("test-only extracted package"), 0600)
}

func TestJunimo140GroupColdWorldUpdatesOfflineWithoutCreatingFarm(t *testing.T) {
	plan, txn := junimo140TransactionFixture(t)
	member := plan.Members[0]
	if err := os.WriteFile(filepath.Join(member.DataDir, "docker-compose.yml"), []byte(junimoComposeTemplate), 0600); err != nil {
		t.Fatal(err)
	}
	world := &txn.Worlds[0]
	world.Original.HasWorld = false
	world.Original.ServerPresent = false
	world.Original.EnvSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(member.DataDir, ".env"))
	world.Original.ComposeSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(member.DataDir, "docker-compose.yml"))
	readyJunimo140Transaction(&txn)
	world.UpdateIntent = true
	txn.InputSHA256 = txn.inputDigest()
	if _, err := claimJunimo140Group(plan); err != nil {
		t.Fatal(err)
	}
	if err := writeJunimo140GroupTransaction(plan, &txn); err != nil {
		t.Fatal(err)
	}
	base := &junimo140CoordinatorDocker{plan: plan}
	fake := &junimo140ColdWorldDocker{base}
	driver := New(fake, nil, nil, nil)
	if err := driver.updateJunimo140GroupWorld(context.Background(), nil, fake, plan, txn, 0); err != nil {
		t.Fatal(err)
	}
	volume, err := GameDataVolumeName(member.DataDir)
	if err != nil || volume != plan.StagingVolume {
		t.Fatal("cold template did not select verified staging")
	}
	if GetActiveSaveName(member.DataDir) != "" {
		t.Fatal("cold template gained a world")
	}
	if _, err := os.Stat(filepath.Join(savesDir(member.DataDir), "Saves")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cold template created a farm directory")
	}
	for _, event := range base.events {
		if strings.HasPrefix(event, "start ") || strings.HasPrefix(event, "create stopped ") {
			t.Fatal("never-started template created a game container", base.events)
		}
	}
}

func (f *junimo140PrepareFailureDocker) RuntimeArchiveJunimo140Instance(_ context.Context, _ string, request paneldocker.RuntimeInstanceArchiveRequest) (string, error) {
	f.events = append(f.events, "archive "+request.Project)
	f.archives++
	return strings.Repeat("f", 64), nil
}
func (f *junimo140PrepareFailureDocker) RuntimeCreateJunimo140Staging(context.Context, string, string, string, string) error {
	f.events = append(f.events, "create stage")
	return errors.New("injected staging allocation failure")
}

func TestJunimo140GroupStageFailureOccursAfterEveryBackupAndPreservesColdWorlds(t *testing.T) {
	plan, txn := junimo140TransactionFixture(t)
	base := &junimo140CoordinatorDocker{plan: plan}
	store := &junimo140CoordinatorStore{}
	for i, member := range plan.Members {
		if err := os.WriteFile(filepath.Join(member.DataDir, "docker-compose.yml"), []byte("original compose\n"), 0600); err != nil {
			t.Fatal(err)
		}
		world := &txn.Worlds[i]
		world.Original.EnvSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(member.DataDir, ".env"))
		world.Original.ComposeSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(member.DataDir, "docker-compose.yml"))
		world.Original.HasWorld = false
		world.Original.ServerContainerID = strings.Repeat(string(rune('a'+i)), 64)
		world.Original.Instance.State = storage.InstanceStateStopped
		store.instances = append(store.instances, world.Original.Instance)
		base.services = append(base.services, paneldocker.RuntimeJunimo140ServiceSnapshot{Project: member.Project, Service: "server", RuntimeServiceMetadata: paneldocker.RuntimeServiceMetadata{ContainerID: world.Original.ServerContainerID, ImageID: world.Original.Server.ImageID}, GameVolume: plan.SourceVolume})
	}
	txn.InputSHA256 = txn.inputDigest()
	journal, err := claimJunimo140Group(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJunimo140GroupTransaction(plan, &txn); err != nil {
		t.Fatal(err)
	}
	fake := &junimo140PrepareFailureDocker{junimo140CoordinatorDocker: base}
	driver := New(fake, nil, nil, store)
	if err := driver.runJunimo140GroupApply(context.Background(), nil, fake, journal, txn); err == nil || !strings.Contains(err.Error(), "injected staging allocation failure") {
		t.Fatalf("wrong staging failure: %v", err)
	}
	if fake.archives != len(plan.Members) {
		t.Fatal("game staging preceded complete world backups")
	}
	events := strings.Join(fake.events, "\n")
	if strings.LastIndex(events, "archive ") > strings.Index(events, "create stage") {
		t.Fatal("staging mutated before last archive", events)
	}
	for _, event := range fake.events {
		if strings.HasPrefix(event, "start ") || strings.HasPrefix(event, "create stopped") || strings.HasPrefix(event, "restore ") {
			t.Fatal("cold fixture started or changed after failed staging", events)
		}
	}
	for i, member := range plan.Members {
		if err := validateJunimo140OriginalFiles(member, txn.Worlds[i]); err != nil {
			t.Fatal("failed preparation changed original files", err)
		}
		if err := rejectJunimo140GroupLease(member.DataDir); err != nil {
			t.Fatal("failed preparation left lease", err)
		}
	}
}

func TestJunimo140ColdTemplateRetainsSaveRequiredStateThroughCommitAndRecovery(t *testing.T) {
	for _, rollback := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "bootstrap rollback"}[rollback], func(t *testing.T) {
			plan, txn := junimo140TransactionFixture(t)
			store := &junimo140CoordinatorStore{}
			for i := range txn.Worlds {
				world := &txn.Worlds[i]
				world.Original.ServerPresent = false
				world.Original.HasWorld = false
				world.Original.Instance.State = storage.InstanceStateSaveRequired
				world.Original.Instance.DriverPayload = `{"template":"keep"}`
				store.instances = append(store.instances, world.Original.Instance)
			}
			fake := &junimo140CoordinatorDocker{plan: plan}
			driver := New(fake, nil, nil, store)
			if err := driver.restoreJunimo140GroupState(context.Background(), fake, plan, txn, rollback, rollback); err != nil {
				t.Fatal(err)
			}
			for _, state := range store.restored {
				if state.State != storage.InstanceStateSaveRequired || state.DriverPayload != `{"template":"keep"}` {
					t.Fatal("cold template lost its pending-save state or payload")
				}
			}
		})
	}
}
