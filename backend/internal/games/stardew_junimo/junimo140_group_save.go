package stardew_junimo

import (
	"context"
	"errors"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func (d *Driver) stopJunimo140SavedMember(ctx context.Context, docker junimo140GroupDocker, journal *junimo140GroupJournal, txn *junimo140GroupTransaction, index int) error {
	plan, world := journal.Plan, &txn.Worlds[index]
	world.StopIntent = true
	if err := checkpointJunimo140Group(journal, txn, "quiescing"); err != nil {
		return err
	}
	snapshots, err := d.snapshotJunimo140Group(ctx, docker, plan)
	if err != nil {
		return err
	}
	if err := validateJunimo140OriginalContainers(plan, *txn, snapshots); err != nil {
		return err
	}
	var own []paneldocker.RuntimeJunimo140ServiceSnapshot
	for _, snapshot := range snapshots {
		if snapshot.Project == plan.Members[index].Project {
			own = append(own, snapshot)
		}
	}
	if err := docker.RuntimeStopJunimo140Services(ctx, plan.Members[index].DataDir, own, junimo140ServiceImages(plan, *txn)); err != nil {
		return err
	}
	snapshots, err = d.snapshotJunimo140Group(ctx, docker, plan)
	if err != nil {
		return err
	}
	if err := validateJunimo140OriginalContainers(plan, *txn, snapshots); err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		if snapshot.Project == plan.Members[index].Project && snapshot.Running {
			return errors.New("saved shared runtime member is still writing")
		}
	}
	world.Stopped = true
	return checkpointJunimo140Group(journal, txn, "quiescing")
}

func waitJunimo140GroupSavedWorld(ctx context.Context, docker junimo140GroupDocker, dir string) error {
	state, err := readRuntimeWorldState(ctx, docker, dir)
	if err != nil {
		return err
	}
	if state.Version == nil || state.DayTransitionComplete == nil || state.IsOnline == nil || !*state.IsOnline {
		return errors.New("shared runtime post-Saved cursor is incomplete")
	}
	// This live snapshot is captured after the same-command Saved receipt.
	// A completed transition already proves quiescence; requiring a newer
	// cursor would wait for unrelated gameplay on an idle/paused server.
	if !*state.DayTransitionComplete {
		if err := waitForJunimoDayTransitionComplete(ctx, docker, dir, *state.Version, 90*time.Second, 500*time.Millisecond); err != nil {
			return err
		}
	}
	_, err = waitStableImportSaveDiskState(ctx, dir, GetActiveSaveName(dir), 20*time.Second)
	return err
}

func junimo140GroupHasStopIntent(txn junimo140GroupTransaction) bool {
	for _, world := range txn.Worlds {
		if world.StopIntent {
			return true
		}
	}
	return false
}

// A failed/cancelled save before the stop barrier must not kill its only writer
// without a backup. Preserve that runtime and its lease until the same command
// proves persistence; no replacement command is submitted during recovery.
func (d *Driver) abortJunimo140BeforeStop(ctx context.Context, docker junimo140GroupDocker, journal *junimo140GroupJournal, txn *junimo140GroupTransaction, bootstrap bool) error {
	plan := journal.Plan
	primary, _ := plan.primary()
	snapshots, err := d.snapshotJunimo140Group(ctx, docker, plan)
	if err != nil {
		return err
	}
	if err := validateJunimo140OriginalContainers(plan, *txn, snapshots); err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		for i, member := range plan.Members {
			if member.Project != snapshot.Project {
				continue
			}
			world := txn.Worlds[i]
			running := world.Original.ServerWasRunning
			if snapshot.Service == "steam-auth" {
				running = world.Original.AuthWasRunning
			}
			// Only a persisted member stop intent permits a changed running state.
			if snapshot.Running != running && !world.StopIntent {
				return errors.New("pre-stop shared runtime state changed; original containers preserved")
			}
		}
	}
	for i, member := range plan.Members {
		world := &txn.Worlds[i]
		if !world.SaveIntent || world.Saved {
			continue
		}
		outcome, err := GetCommandOutcome(member.DataDir, world.SaveCommandID)
		if err != nil {
			return err
		}
		// Control returns this before constructing SaveGameMenu or applying any
		// pre-save action. A busy menu is a proven no-op, so preserve the running
		// source and let the administrator retry after closing it.
		if outcome.CommandID == world.SaveCommandID && outcome.Status == CommandStatusFailed && outcome.ErrorCode == "save_ui_busy" {
			continue
		}
		if outcome.CommandID != world.SaveCommandID || outcome.Status != CommandStatusSucceeded || outcome.ErrorCode != "ok" || outcome.Details["event"] != "GameLoop.Saved" || outcome.Details["saveId"] != GetActiveSaveName(member.DataDir) {
			return errors.New("shared runtime save remains unconfirmed; original writer and leases preserved")
		}
		if err := waitJunimo140GroupSavedWorld(ctx, docker, member.DataDir); err != nil {
			return err
		}
		world.Saved = true
		if err := checkpointJunimo140Group(journal, txn, "rolling_back"); err != nil {
			return err
		}
	}
	if err := docker.RuntimeAssertJunimo140GameGroup(ctx, primary.DataDir, plan.SourceVolume, junimo140Projects(plan), false); err != nil {
		return err
	}
	if junimo140GroupHasStopIntent(*txn) {
		// No files, images or game bindings have changed yet. Preserve live peers,
		// and restart only stopped originals after every pending save is resolved.
		// Bootstrap keeps those stopped members stopped, without killing a peer.
		bootstrap = bootstrap || txn.KeepServerStopped
		if bootstrap {
			txn.KeepServerStopped = true
			if err := checkpointJunimo140Group(journal, txn, "rolling_back"); err != nil {
				return err
			}
		} else {
			var auth, server []paneldocker.RuntimeJunimo140ServiceSnapshot
			for _, snapshot := range snapshots {
				if snapshot.Running {
					continue
				}
				for i, member := range plan.Members {
					if member.Project != snapshot.Project {
						continue
					}
					original := txn.Worlds[i].Original
					if snapshot.Service == "steam-auth" && original.AuthWasRunning {
						auth = append(auth, snapshot)
					}
					if snapshot.Service == "server" && original.ServerWasRunning {
						server = append(server, snapshot)
					}
				}
			}
			images := junimo140ServiceImages(plan, *txn)
			if err := docker.RuntimeStartJunimo140Services(ctx, primary.DataDir, auth, images); err != nil {
				return err
			}
			if err := docker.RuntimeStartJunimo140Services(ctx, primary.DataDir, server, images); err != nil {
				return err
			}
		}
		snapshots, err = d.snapshotJunimo140Group(ctx, docker, plan)
		if err != nil {
			return err
		}
		if err := validateJunimo140OriginalContainers(plan, *txn, snapshots); err != nil {
			return err
		}
		for i, member := range plan.Members {
			if err := validateJunimo140OriginalFiles(member, txn.Worlds[i]); err != nil {
				return err
			}
			state := txn.Worlds[i].Original.Instance
			for _, snapshot := range snapshots {
				if snapshot.Project != member.Project || snapshot.Service != "server" {
					continue
				}
				if !bootstrap && snapshot.Running != txn.Worlds[i].Original.ServerWasRunning {
					return errors.New("original shared runtime running selection did not recover")
				}
				if snapshot.Running {
					deadline := time.Now().Add(d.runtimeUpdateServerTimeout)
					for {
						if docker.RuntimeServerHealth(ctx, member.DataDir, member.Project) == nil && runtimeInfoContractReady(ctx, docker, member.DataDir, txn.Worlds[i].Original.Current.Server.Tag) {
							break
						}
						if time.Now().After(deadline) {
							return errors.New("original shared runtime did not recover health after partial stop")
						}
						select {
						case <-ctx.Done():
							return ctx.Err()
						case <-time.After(d.runtimeUpdatePollInterval):
						}
					}
				} else if txn.Worlds[i].Original.ServerWasRunning || isRunningState(state.State) {
					state.State = storage.InstanceStateStopped
				}
			}
			if _, err := d.store.RestoreInstanceStateSnapshot(ctx, state); err != nil {
				return err
			}
		}
	}
	if err := checkpointJunimo140Group(journal, txn, "rolled_back"); err != nil {
		return err
	}
	code := "shared_runtime_aborted_before_stop"
	if junimo140GroupHasStopIntent(*txn) {
		code = "shared_runtime_aborted_before_update"
	}
	if err := writeJunimo140GroupPublicStatus(plan, *txn, RuntimeUpdateApplyFailedRolledBack, code, false); err != nil {
		return err
	}
	// Partial-stop bootstrap deliberately preserves an unsaved live peer. Its
	// public status must describe each actual member, not a group-wide guess.
	for _, member := range plan.Members {
		status, err := readRuntimeUpdateApplyStatus(member.DataDir)
		if err != nil {
			return err
		}
		status.ServerRunning = false
		for _, snapshot := range snapshots {
			if snapshot.Project == member.Project && snapshot.Service == "server" {
				status.ServerRunning = snapshot.Running
			}
		}
		if status.ServerWasRunning && !status.ServerRunning {
			status.ManualAction = "保存及恢复已确认；此世界保持关闭，可在面板中手动启动。"
		}
		if err := writeRuntimeUpdateApplyStatus(member.DataDir, status); err != nil {
			return err
		}
	}
	d.auditJunimo140GroupTerminal(ctx, plan, *txn)
	return releaseJunimo140Group(*journal)
}
