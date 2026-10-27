package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type junimo140PreStopDocker struct{ *junimo140CoordinatorDocker }

func (f *junimo140PreStopDocker) RuntimeAssertJunimo140GameGroup(_ context.Context, _ string, _ string, _ []string, quiescent bool) error {
	if quiescent {
		return errors.New("pre-stop abort must preserve original running services")
	}
	f.events = append(f.events, "read-only group check")
	return nil
}

func setupJunimo140PreStop(t *testing.T, pending bool) (*Driver, *junimo140PreStopDocker, *junimo140CoordinatorStore, junimo140GroupJournal, junimo140GroupTransaction) {
	t.Helper()
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
		world.Original.ServerContainerID = strings.Repeat(string(rune('a'+i)), 64)
		if i == 0 {
			world.Original.ServerWasRunning = true
			world.Original.Instance.State = storage.InstanceStateRunning
		}
		store.instances = append(store.instances, world.Original.Instance)
		base.services = append(base.services, paneldocker.RuntimeJunimo140ServiceSnapshot{Project: member.Project, Service: "server", RuntimeServiceMetadata: paneldocker.RuntimeServiceMetadata{ContainerID: world.Original.ServerContainerID, ImageID: world.Original.Server.ImageID}, Running: world.Original.ServerWasRunning, GameVolume: plan.SourceVolume})
	}
	if pending {
		world := &txn.Worlds[0]
		world.SaveIntent, world.SaveCommandID = true, strings.Repeat("a", 32)
		if err := writePanelCommandWithID(plan.Members[0].DataDir, world.SaveCommandID, "save-now", nil); err != nil {
			t.Fatal(err)
		}
	}
	txn.InputSHA256 = txn.inputDigest()
	journal, err := claimJunimo140Group(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJunimo140GroupTransaction(plan, &txn); err != nil {
		t.Fatal(err)
	}
	fake := &junimo140PreStopDocker{base}
	driver := New(fake, nil, nil, store)
	return driver, fake, store, journal, txn
}

func TestJunimo140PreStopFailureDoesNotBounceOrRestoreUnchangedWorlds(t *testing.T) {
	driver, fake, store, journal, txn := setupJunimo140PreStop(t, false)
	if err := driver.rollbackJunimo140Group(context.Background(), fake, &journal, &txn, true); err != nil {
		t.Fatal(err)
	}
	if !fake.services[0].Running || len(store.restored) != 0 {
		t.Fatal("pre-stop failure changed original runtime/database")
	}
	for _, event := range fake.events {
		if event != "snapshot" && event != "read-only group check" {
			t.Fatal("pre-stop failure mutated services", fake.events)
		}
	}
	for _, member := range journal.Plan.Members {
		if err := rejectJunimo140GroupLease(member.DataDir); err != nil {
			t.Fatal("clean pre-stop abort retained lease", err)
		}
	}
}

func TestJunimo140UnconfirmedSavePreservesOnlyWriterAndDoesNotResubmit(t *testing.T) {
	driver, fake, _, journal, txn := setupJunimo140PreStop(t, true)
	if err := driver.rollbackJunimo140Group(context.Background(), fake, &journal, &txn, true); err == nil {
		t.Fatal("unconfirmed save released shared owner")
	}
	member := journal.Plan.Members[0]
	if err := driver.RecoverRuntimeUpdateApply(context.Background(), registry.Instance{ID: member.ID, DriverID: DriverID, DataDir: member.DataDir}); err == nil {
		t.Fatal("pending save was silently bypassed on peer recovery")
	}
	if !fake.services[0].Running {
		t.Fatal("unconfirmed save killed its only writer without backup")
	}
	for _, event := range fake.events {
		if event != "snapshot" {
			t.Fatal("pending save triggered runtime/data mutation", fake.events)
		}
	}
	entries, err := os.ReadDir(filepath.Join(controlDir(member.DataDir), "commands"))
	if err != nil || len(entries) != 1 {
		t.Fatal("recovery submitted a duplicate save command")
	}
	for _, member := range journal.Plan.Members {
		if err := rejectJunimo140GroupLease(member.DataDir); err == nil {
			t.Fatal("unconfirmed persistence released peer lease")
		}
	}
}

func TestJunimo140RejectedSaveMenuIsNoOpAndReleasesGroupForRetry(t *testing.T) {
	driver, fake, _, journal, txn := setupJunimo140PreStop(t, true)
	member := journal.Plan.Members[0]
	commandID := txn.Worlds[0].SaveCommandID
	raw, err := json.Marshal(CommandOutcome{CommandID: commandID, Status: CommandStatusFailed, ErrorCode: "save_ui_busy"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(commandResultsDir(member.DataDir), commandID+".json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := driver.rollbackJunimo140Group(context.Background(), fake, &journal, &txn, false); err != nil {
		t.Fatal(err)
	}
	if !fake.services[0].Running {
		t.Fatal("proven no-op save failure stopped the original game")
	}
	for _, member := range journal.Plan.Members {
		if err := rejectJunimo140GroupLease(member.DataDir); err != nil {
			t.Fatal("menu busy prevented safe retry", err)
		}
	}
}
