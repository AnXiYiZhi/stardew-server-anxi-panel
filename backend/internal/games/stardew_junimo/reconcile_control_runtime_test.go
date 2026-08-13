package stardew_junimo

import (
	"context"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type reconcileComposeCountingDocker struct {
	*fakeDocker
	composeUpCalls int
}

func (f *reconcileComposeCountingDocker) ComposeUp(context.Context, string) (paneldocker.CommandResult, error) {
	f.composeUpCalls++
	return paneldocker.CommandResult{ExitCode: 0}, nil
}

func TestDriverReconcileStateKeepsUnfinishedNewGameErrorOwner(t *testing.T) {
	dataDir, expectedControl := setupControlRuntimeGateTest(t)
	if _, _, err := beginOrResumeNewGameTransaction(
		dataDir, newGameTestConfig("standard"), "reconcile-owner-request", "job-owner",
	); err != nil {
		t.Fatal(err)
	}
	writeControlRuntimeOptions(t, dataDir, `{"controlModVersion":"`+expectedControl+`"}`)
	docker := &reconcileComposeCountingDocker{fakeDocker: &fakeDocker{
		psResult: paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
			Service: "server", State: "running", Status: "Up 1 minute",
		}}},
	}}
	store := &fakeStore{instance: storage.Instance{
		ID: "stardew", DataDir: dataDir, State: storage.InstanceStateError,
		DriverPhase: "new_game_recovery_required", DriverPayload: `{"kept":true}`,
	}}
	driver := New(docker, nil, nil, store)

	updated, err := driver.ReconcileState(context.Background(), store.instance)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != storage.InstanceStateError || updated.DriverPhase != "new_game_recovery_required" {
		t.Fatalf("unfinished owner was promoted: %+v", updated)
	}
	if len(store.updated) != 0 {
		t.Fatalf("unfinished owner must block all state writes, got %+v", store.updated)
	}
	if docker.composeUpCalls != 0 {
		t.Fatalf("ReconcileState must never start an unfinished new-game runtime, ComposeUp calls=%d", docker.composeUpCalls)
	}
}

func TestDriverReconcileStateAfterHostRestartStopsWithoutComposeUp(t *testing.T) {
	docker := &reconcileComposeCountingDocker{fakeDocker: &fakeDocker{
		psResult: paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
			Service: "steam-auth", State: "running", Status: "Up 1 minute",
		}}},
	}}
	store := &fakeStore{instance: storage.Instance{
		ID: "stardew", DataDir: t.TempDir(), State: storage.InstanceStateRunning,
		DriverPhase: "running", DriverPayload: `{"kept":true}`,
	}}
	driver := New(docker, nil, nil, store)

	updated, err := driver.ReconcileState(context.Background(), store.instance)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != storage.InstanceStateStopped || updated.DriverPhase != "container_stopped" {
		t.Fatalf("absent server must remain off after host restart: %+v", updated)
	}
	if len(store.updated) != 1 || store.updated[0].DriverPayload != `{"kept":true}` {
		t.Fatalf("unexpected host-restart reconciliation writes: %+v", store.updated)
	}
	if docker.composeUpCalls != 0 {
		t.Fatalf("host-restart reconciliation must not auto-start the game, ComposeUp calls=%d", docker.composeUpCalls)
	}
}
