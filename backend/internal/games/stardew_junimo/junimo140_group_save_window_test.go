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
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type junimo140SaveWindowDocker struct{ *junimo140CoordinatorDocker }

func (f *junimo140SaveWindowDocker) ComposeExecPipe(ctx context.Context, dir, project, service string, args ...string) (paneldocker.CommandResult, error) {
	if len(args) > 0 && args[0] == "curl" {
		state := map[string]any{"isOnline": true, "dayTransitionComplete": true, "playerCount": 0, "version": 3}
		if strings.Contains(args[len(args)-1], "/wait/status?") {
			state["version"] = 4
		}
		raw, _ := json.Marshal(state)
		if strings.Contains(args[len(args)-1], "/wait/status?") {
			return paneldocker.CommandResult{Stdout: string(raw) + "\n200"}, nil
		}
		return paneldocker.CommandResult{Stdout: string(raw)}, nil
	}
	return f.junimo140CoordinatorDocker.ComposeExecPipe(ctx, dir, project, service, args...)
}

func (f *junimo140SaveWindowDocker) RuntimeAssertJunimo140GameGroup(context.Context, string, string, []string, bool) error {
	return nil
}

func (f *junimo140SaveWindowDocker) RuntimeStopJunimo140Services(_ context.Context, _ string, snapshots []paneldocker.RuntimeJunimo140ServiceSnapshot, _ map[string][]string) error {
	for _, snapshot := range snapshots {
		for i := range f.services {
			if f.services[i].ContainerID == snapshot.ContainerID {
				f.services[i].Running = false
			}
		}
	}
	return nil
}

func setupJunimo140PartialSave(t *testing.T) (*Driver, *junimo140SaveWindowDocker, junimo140GroupJournal, junimo140GroupTransaction) {
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
		world.Original.ServerWasRunning = true
		world.Original.Instance.State = storage.InstanceStateRunning
		world.Original.Current.Server.Tag = "1.5.0-preview.125"
		world.SaveIntent, world.SaveCommandID = true, strings.Repeat(string(rune('a'+i)), 32)
		store.instances = append(store.instances, world.Original.Instance)
		base.services = append(base.services, paneldocker.RuntimeJunimo140ServiceSnapshot{Project: member.Project, Service: "server", RuntimeServiceMetadata: paneldocker.RuntimeServiceMetadata{ContainerID: world.Original.ServerContainerID, ImageID: world.Original.Server.ImageID}, Running: i != 0, GameVolume: plan.SourceVolume})
	}
	txn.Worlds[0].Saved, txn.Worlds[0].StopIntent, txn.Worlds[0].Stopped = true, true, true
	txn.InputSHA256 = txn.inputDigest()
	journal, err := claimJunimo140Group(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJunimo140GroupTransaction(plan, &txn); err != nil {
		t.Fatal(err)
	}
	fake := &junimo140SaveWindowDocker{base}
	return New(fake, nil, nil, store), fake, journal, txn
}

func TestJunimo140PartialStopUnconfirmedPeerPreservesWriterAcrossRestart(t *testing.T) {
	driver, fake, journal, txn := setupJunimo140PartialSave(t)
	for attempt := 0; attempt < 2; attempt++ {
		if err := driver.rollbackJunimo140Group(context.Background(), fake, &journal, &txn, attempt != 0); err == nil {
			t.Fatal("pending peer save was treated as a completed rollback")
		}
		if fake.services[0].Running || !fake.services[1].Running {
			t.Fatal("partial stop recovery killed the unsaved peer or restarted an earlier writer")
		}
		for _, member := range journal.Plan.Members {
			if err := rejectJunimo140GroupLease(member.DataDir); err == nil {
				t.Fatal("unconfirmed peer released group lease")
			}
		}
		var err error
		txn, err = readJunimo140GroupTransaction(journal.Plan)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, member := range journal.Plan.Members {
		entries, err := os.ReadDir(filepath.Join(controlDir(member.DataDir), "commands"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatal("recovery resubmitted a save")
		}
	}
}

func TestJunimo140PartialStopRejectedPeerRestoresOnlyOriginalSelection(t *testing.T) {
	for _, bootstrap := range []bool{false, true} {
		t.Run(map[bool]string{false: "live abort", true: "panel restart"}[bootstrap], func(t *testing.T) {
			driver, fake, journal, txn := setupJunimo140PartialSave(t)
			member, world := journal.Plan.Members[1], txn.Worlds[1]
			if err := os.MkdirAll(commandResultsDir(member.DataDir), 0700); err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(CommandOutcome{CommandID: world.SaveCommandID, Status: CommandStatusFailed, ErrorCode: "save_ui_busy"})
			if err := os.WriteFile(filepath.Join(commandResultsDir(member.DataDir), world.SaveCommandID+".json"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err := driver.rollbackJunimo140Group(context.Background(), fake, &journal, &txn, bootstrap); err != nil {
				t.Fatal(err)
			}
			if fake.services[0].Running == bootstrap || !fake.services[1].Running {
				t.Fatal("abort changed an untouched writer or lost original running selection")
			}
			for i, member := range journal.Plan.Members {
				status, err := readRuntimeUpdateApplyStatus(member.DataDir)
				if err != nil || status.ServerRunning != fake.services[i].Running {
					t.Fatal("public abort status disagrees with preserved runtime")
				}
			}
			for _, member := range journal.Plan.Members {
				if err := rejectJunimo140GroupLease(member.DataDir); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestJunimo140PartialStopCancelledPeerDoesNotDestroyUnbackedWorld(t *testing.T) {
	driver, fake, journal, txn := setupJunimo140PartialSave(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := driver.rollbackJunimo140Group(ctx, fake, &journal, &txn, false); err == nil {
		t.Fatal("cancellation released an unconfirmed save")
	}
	if fake.services[0].Running || !fake.services[1].Running {
		t.Fatal("cancellation killed the remaining unique writer")
	}
	if txn.WorldChangesStarted || txn.StageCreateIntent {
		t.Fatal("cancellation crossed complete backup barrier")
	}
}

func TestJunimo140PartialStopLateReceiptResolvesSameCommandOnRestart(t *testing.T) {
	driver, fake, journal, txn := setupJunimo140PartialSave(t)
	member, world := journal.Plan.Members[1], txn.Worlds[1]
	writeNewGameTestSave(t, member.DataDir, "Late_123", "0")
	if err := writeGameloaderPointer(member.DataDir, "Late_123"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(commandResultsDir(member.DataDir), 0700); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(CommandOutcome{CommandID: world.SaveCommandID, Status: CommandStatusSucceeded, ErrorCode: "ok", CreatedAt: time.Now().UTC(), Details: map[string]string{"event": "GameLoop.Saved", "saveId": "Late_123"}})
	if err := os.WriteFile(filepath.Join(commandResultsDir(member.DataDir), world.SaveCommandID+".json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := driver.rollbackJunimo140Group(context.Background(), fake, &journal, &txn, true); err != nil {
		t.Fatal(err)
	}
	if fake.services[0].Running || !fake.services[1].Running || !txn.Worlds[1].Saved {
		t.Fatal("late receipt recovery lost per-world state")
	}
	for _, member := range journal.Plan.Members {
		entries, _ := os.ReadDir(filepath.Join(controlDir(member.DataDir), "commands"))
		if len(entries) != 0 {
			t.Fatal("late receipt resubmitted save")
		}
		if err := rejectJunimo140GroupLease(member.DataDir); err != nil {
			t.Fatal(err)
		}
	}
}
