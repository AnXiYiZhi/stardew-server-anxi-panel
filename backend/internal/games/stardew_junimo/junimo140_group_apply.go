package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// The parent owns every world and all rollback materials. Per-world acceptance
// never publishes success before every member has passed and state is restored.
func (d *Driver) runJunimo140GroupApply(ctx context.Context, job *jobs.Context, docker junimo140GroupDocker, journal junimo140GroupJournal, txn junimo140GroupTransaction) (resultErr error) {
	defer func() {
		if failure := recover(); failure != nil {
			resultErr = errors.New("shared runtime coordinator interrupted by an internal error")
		}
		if resultErr == nil {
			return
		}
		persisted, readErr := readJunimo140GroupTransaction(journal.Plan)
		if readErr != nil {
			resultErr = errors.Join(resultErr, readErr)
			return
		}
		txn = persisted
		if txn.Phase == "committed" {
			return
		}
		recoveryCtx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()
		if err := d.rollbackJunimo140Group(recoveryCtx, docker, &journal, &txn, false); err != nil {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	plan := journal.Plan
	primary, _ := plan.primary()
	if err := checkpointJunimo140Group(&journal, &txn, "quiescing"); err != nil {
		return err
	}
	if err := writeJunimo140GroupPublicStatus(plan, txn, RuntimeUpdateApplyStopping, "", false); err != nil {
		return err
	}
	for i, member := range plan.Members {
		world := &txn.Worlds[i]
		if err := validateJunimo140OriginalFiles(member, *world); err != nil {
			return err
		}
		if world.Original.ServerWasRunning && world.Original.HasWorld {
			commandID, err := randomHex(16)
			if err != nil {
				return err
			}
			world.SaveIntent, world.SaveCommandID = true, commandID
			if err := checkpointJunimo140Group(&journal, &txn, "quiescing"); err != nil {
				return err
			}
			submittedAt := time.Now().UTC()
			if err := writePanelCommandWithID(member.DataDir, commandID, "save-now", nil); err != nil {
				return err
			}
			options := defaultImportDurableSaveOptions()
			options.GetOutcome = GetCommandOutcome
			outcome, err := waitForDurableSaveOutcome(ctx, member.DataDir, commandID, options)
			if err != nil {
				return err
			}
			if outcome.ErrorCode != "ok" || outcome.Details["event"] != "GameLoop.Saved" || outcome.Details["saveId"] != GetActiveSaveName(member.DataDir) || outcome.CreatedAt.Before(submittedAt.Add(-time.Second)) {
				return errors.New("shared runtime save did not prove the selected world persisted")
			}
			if err := waitJunimo140GroupSavedWorld(ctx, docker, member.DataDir); err != nil {
				return err
			}
			world.Saved = true
			if err := checkpointJunimo140Group(&journal, &txn, "quiescing"); err != nil {
				return err
			}
		}
		// Close this world's writer before waiting on another world's save.
		// Saved alone is not a maintenance freeze: players can still mutate it.
		if err := d.stopJunimo140SavedMember(ctx, docker, &journal, &txn, i); err != nil {
			return err
		}
	}
	snapshots, err := d.snapshotJunimo140Group(ctx, docker, plan)
	if err != nil {
		return err
	}
	if err := validateJunimo140OriginalContainers(plan, txn, snapshots); err != nil {
		return err
	}
	if err := requireJunimo140Quiescence(snapshots); err != nil {
		return err
	}
	if err := docker.RuntimeAssertJunimo140GameGroup(ctx, primary.DataDir, plan.SourceVolume, junimo140Projects(plan), true); err != nil {
		return err
	}
	for i := range txn.Worlds {
		txn.Worlds[i].Stopped = true
	}
	if err := checkpointJunimo140Group(&journal, &txn, "backing_up"); err != nil {
		return err
	}
	for i, member := range plan.Members {
		if err := validateJunimo140OriginalFiles(member, txn.Worlds[i]); err != nil {
			return err
		}
		request, err := d.junimo140ArchiveRequest(plan, txn, i, false)
		if err != nil {
			return err
		}
		txn.Worlds[i].ArchiveSHA256, err = docker.RuntimeArchiveJunimo140Instance(ctx, member.DataDir, request)
		if err != nil {
			return err
		}
		if err := checkpointJunimo140Group(&journal, &txn, "backing_up"); err != nil {
			return err
		}
		if txn.Worlds[i].Original.SessionVolume != "" {
			request, err = d.junimo140ArchiveRequest(plan, txn, i, true)
			if err != nil {
				return err
			}
			txn.Worlds[i].AuthArchiveSHA256, err = docker.RuntimeArchiveJunimo140Session(ctx, member.DataDir, request, txn.Worlds[i].Original.SessionVolume)
			if err != nil {
				return err
			}
			if err := checkpointJunimo140Group(&journal, &txn, "backing_up"); err != nil {
				return err
			}
		}
	}
	txn.StageCreateIntent = true
	if err := checkpointJunimo140Group(&journal, &txn, "preparing_game"); err != nil {
		return err
	}
	if err := docker.RuntimeCreateJunimo140Staging(ctx, primary.DataDir, primary.Project, plan.StagingVolume, plan.ApplyID); err != nil {
		return err
	}
	txn.StageCreated = true
	if err := checkpointJunimo140Group(&journal, &txn, "preparing_game"); err != nil {
		return err
	}
	if _, err := prepareJunimo140SharedGameStaging(ctx, docker, primary.DataDir, plan.SourceVolume, plan.StagingVolume, txn.Target.Server); err != nil {
		return err
	}
	if err := docker.RuntimePrepareJunimo140GamePermissions(ctx, primary.DataDir, plan.StagingVolume, txn.Target.Server.Image, txn.Target.Server.ImageID, plan.UID, plan.GID); err != nil {
		return err
	}
	txn.StageReady = true
	if err := checkpointJunimo140Group(&journal, &txn, "preparing_game"); err != nil {
		return err
	}
	txn.WorldChangesStarted = true
	if err := checkpointJunimo140Group(&journal, &txn, "updating_worlds"); err != nil {
		return err
	}
	for i := range plan.Members {
		txn.Worlds[i].UpdateIntent = true
		if err := checkpointJunimo140Group(&journal, &txn, "updating_worlds"); err != nil {
			return err
		}
		if _, err := job.Info(ctx, fmt.Sprintf("正在更新共享组世界 %d/%d。", i+1, len(plan.Members))); err != nil {
			return err
		}
		if err := d.updateJunimo140GroupWorld(ctx, job, docker, plan, txn, i); err != nil {
			return err
		}
		txn.Worlds[i].Updated = true
		if err := checkpointJunimo140Group(&journal, &txn, "updating_worlds"); err != nil {
			return err
		}
	}
	if err := checkpointJunimo140Group(&journal, &txn, "restoring_state"); err != nil {
		return err
	}
	if err := d.restoreJunimo140GroupState(ctx, docker, plan, txn, false, false); err != nil {
		return err
	}
	if err := checkpointJunimo140Group(&journal, &txn, "committed"); err != nil {
		return err
	}
	if err := writeJunimo140GroupPublicStatus(plan, txn, RuntimeUpdateApplySucceeded, "", false); err != nil {
		return err
	}
	d.auditJunimo140GroupTerminal(ctx, plan, txn)
	return releaseJunimo140Group(journal)
}

func validateJunimo140OriginalContainers(plan junimo140GroupPlan, txn junimo140GroupTransaction, snapshots []paneldocker.RuntimeJunimo140ServiceSnapshot) error {
	seen := map[string]bool{}
	for _, snapshot := range snapshots {
		for i, member := range plan.Members {
			if member.Project != snapshot.Project {
				continue
			}
			original := txn.Worlds[i].Original
			want := original.ServerContainerID
			if snapshot.Service == "steam-auth" {
				want = original.AuthContainerID
			}
			if want == "" || snapshot.ContainerID != want || snapshot.GameVolume != plan.SourceVolume {
				return errors.New("shared runtime container changed after capture")
			}
			seen[member.Project+"/"+snapshot.Service] = true
		}
	}
	for i, member := range plan.Members {
		original := txn.Worlds[i].Original
		if original.ServerPresent && !seen[member.Project+"/server"] || original.AuthPresent && !seen[member.Project+"/steam-auth"] {
			return errors.New("shared runtime original container disappeared")
		}
	}
	return nil
}

func (d *Driver) updateJunimo140GroupWorld(ctx context.Context, job *jobs.Context, docker junimo140GroupDocker, plan junimo140GroupPlan, txn junimo140GroupTransaction, index int) error {
	member, world := plan.Members[index], txn.Worlds[index]
	if err := validateJunimo140OriginalFiles(member, world); err != nil {
		return err
	}
	request, err := d.junimo140ArchiveRequest(plan, txn, index, false)
	if err != nil {
		return err
	}
	if err := docker.RuntimePrepareJunimo140Permissions(ctx, member.DataDir, request, plan.UID, plan.GID); err != nil {
		return err
	}
	snapshots, err := d.snapshotJunimo140Group(ctx, docker, plan)
	if err != nil {
		return err
	}
	var own []paneldocker.RuntimeJunimo140ServiceSnapshot
	for _, snapshot := range snapshots {
		if snapshot.Project == member.Project {
			own = append(own, snapshot)
		}
	}
	if err := docker.RuntimeRemoveJunimo140Services(ctx, member.DataDir, own, junimo140ServiceImages(plan, txn)); err != nil {
		return err
	}
	if err := docker.RuntimeExtractJunimo140Mod(ctx, member.DataDir, request); err != nil {
		return err
	}
	primary, _ := plan.primary()
	recoveryDir := filepath.Join(primary.DataDir, ".local-container", "junimo140-group", plan.ApplyID)
	extracted := filepath.Join(recoveryDir, "target-mod-"+strings.TrimSuffix(member.Archive, ".tar"))
	if err := validateExtractedJunimoServerMod(extracted, txn.Recommendation.Server.Tag); err != nil {
		return err
	}
	if _, err := replaceJunimoServerMod(member.DataDir, extracted, filepath.Join(recoveryDir, "original-mod-"+strings.TrimSuffix(member.Archive, ".tar"))); err != nil {
		return err
	}
	if err := installSMAPIMod(member.DataDir); err != nil {
		return err
	}
	if err := EnsureGameDataVolumeBinding(member.DataDir); err != nil {
		return err
	}
	if err := writeRuntimeTargetEnvAtomic(member.DataDir, txn.Recommendation, txn.Target, world.Original.SteamInviteEnabled); err != nil {
		return err
	}
	if err := switchGameDataVolumeAtomic(member.DataDir, plan.StagingVolume); err != nil {
		return err
	}
	if _, err := ensureJunimo140ComposeContract(member.DataDir, world.Original.SteamInviteEnabled); err != nil {
		return err
	}
	if err := docker.RuntimePrepareJunimo140Permissions(ctx, member.DataDir, request, plan.UID, plan.GID); err != nil {
		return err
	}
	var validateErr error
	if world.Original.SteamInviteEnabled {
		validateErr = docker.RuntimeComposeConfigValidateImages(ctx, member.DataDir, member.Project, txn.Target.Server.ImageID, txn.Target.SteamAuth.ImageID)
	} else {
		validateErr = docker.RuntimeComposeConfigValidateServerImage(ctx, member.DataDir, member.Project, txn.Target.Server.ImageID)
	}
	if validateErr != nil {
		return validateErr
	}
	// A never-started template has no game process to validate or recreate.
	// Its game/SDK, image-owned SMAPI, Mod and Compose are verified offline.
	if !world.Original.HasWorld {
		if world.Original.ServerPresent {
			if err := docker.RuntimeCreateJunimo140ServicePinned(ctx, member.DataDir, member.Project, "server", txn.Target.Server.ImageID); err != nil {
				return err
			}
		}
		if world.Original.AuthPresent {
			if err := docker.RuntimeCreateJunimo140ServicePinned(ctx, member.DataDir, member.Project, "steam-auth", txn.Target.SteamAuth.ImageID); err != nil {
				return err
			}
		}
		return nil
	}
	if world.Original.SteamInviteEnabled {
		if err := docker.RuntimeCreateJunimo140ServicePinned(ctx, member.DataDir, member.Project, "steam-auth", txn.Target.SteamAuth.ImageID); err != nil {
			return err
		}
	}
	if err := docker.RuntimeCreateJunimo140ServicePinned(ctx, member.DataDir, member.Project, "server", txn.Target.Server.ImageID); err != nil {
		return err
	}
	instance := junimo140MemberInstance(member, world)
	if err := (&lifecycleRunner{driver: d, lifecycle: docker, instance: world.Original.Instance}).clearRuntimeControlSnapshots(ctx, job); err != nil {
		return err
	}
	snapshots, err = d.snapshotJunimo140Group(ctx, docker, plan)
	if err != nil {
		return err
	}
	var auth, server []paneldocker.RuntimeJunimo140ServiceSnapshot
	for _, snapshot := range snapshots {
		if snapshot.Project == member.Project {
			if snapshot.Service == "server" {
				server = append(server, snapshot)
			} else {
				auth = append(auth, snapshot)
			}
		}
	}
	if len(server) != 1 || world.Original.SteamInviteEnabled && len(auth) != 1 {
		return errors.New("target shared runtime service missing")
	}
	images := junimo140ServiceImages(plan, txn)
	if err := docker.RuntimeStartJunimo140Services(ctx, member.DataDir, auth, images); err != nil {
		return err
	}
	if err := docker.RuntimeStartJunimo140Services(ctx, member.DataDir, server, images); err != nil {
		return err
	}
	manifest := runtimeUpdateRecoveryManifest{SchemaVersion: 4, SteamInviteEnabled: world.Original.SteamInviteEnabled, Project: member.Project, Target: txn.Target, TargetServerVersion: txn.Recommendation.Server.Tag, AuthImageChanged: world.Original.SteamInviteEnabled}
	if err := d.verifyRuntimeTarget(ctx, docker, instance, manifest); err != nil {
		return err
	}
	if err := d.verifyJunimo140GroupWorld(ctx, docker, member.DataDir); err != nil {
		return err
	}
	snapshots, err = d.snapshotJunimo140Group(ctx, docker, plan)
	if err != nil {
		return err
	}
	own = nil
	for _, snapshot := range snapshots {
		if snapshot.Project == member.Project {
			own = append(own, snapshot)
		}
	}
	if err := docker.RuntimeStopJunimo140Services(ctx, member.DataDir, own, images); err != nil {
		return err
	}
	snapshots, err = d.snapshotJunimo140Group(ctx, docker, plan)
	if err != nil {
		return err
	}
	return requireJunimo140Quiescence(snapshots)
}

func (d *Driver) verifyJunimo140GroupWorld(ctx context.Context, docker junimo140GroupDocker, dir string) error {
	deadline := time.Now().Add(d.runtimeUpdateServerTimeout)
	for time.Now().Before(deadline) {
		evidence, err := captureImportActivationEvidence(ctx, docker, dir)
		if err == nil && evidence.RuntimeSaveID == GetActiveSaveName(dir) && !evidence.PendingIntent.Exists && activationWorldStable(evidence) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d.runtimeUpdatePollInterval):
		}
	}
	return errors.New("target shared runtime world did not become stably ready")
}

func (d *Driver) restoreJunimo140GroupState(ctx context.Context, docker junimo140GroupDocker, plan junimo140GroupPlan, txn junimo140GroupTransaction, rollback, bootstrap bool) error {
	snapshots, err := d.snapshotJunimo140Group(ctx, docker, plan)
	if err != nil {
		return err
	}
	if err := requireJunimo140Quiescence(snapshots); err != nil {
		return err
	}
	images := junimo140ServiceImages(plan, txn)
	for i, member := range plan.Members {
		world := txn.Worlds[i]
		wantServer := world.Original.ServerPresent || !rollback && world.Original.HasWorld
		wantAuth := world.Original.AuthPresent || !rollback && world.Original.HasWorld && world.Original.SteamInviteEnabled
		foundServer, foundAuth := false, false
		for _, snapshot := range snapshots {
			if snapshot.Project != member.Project {
				continue
			}
			imageID, volume := txn.Target.Server.ImageID, plan.StagingVolume
			if rollback {
				imageID, volume = world.Original.Server.ImageID, plan.SourceVolume
			}
			if snapshot.Service == "server" {
				foundServer = true
			} else {
				foundAuth = true
				imageID = txn.Target.SteamAuth.ImageID
				if rollback {
					imageID = world.Original.Auth.ImageID
				}
				if snapshot.SessionVolume != world.Original.SessionVolume {
					return errors.New("restored authentication session scope changed")
				}
			}
			if snapshot.ImageID != imageID || snapshot.GameVolume != volume {
				return errors.New("restored shared runtime service identity changed")
			}
		}
		if foundServer != wantServer || foundAuth != wantAuth {
			return errors.New("restored shared runtime container presence is incomplete")
		}
	}
	var auth, server []paneldocker.RuntimeJunimo140ServiceSnapshot
	if !bootstrap {
		for _, snapshot := range snapshots {
			for i, member := range plan.Members {
				if member.Project != snapshot.Project {
					continue
				}
				world := txn.Worlds[i]
				if snapshot.Service == "steam-auth" && world.Original.AuthWasRunning {
					auth = append(auth, snapshot)
				}
				if snapshot.Service == "server" && world.Original.ServerWasRunning {
					server = append(server, snapshot)
				}
			}
		}
	}
	primary, _ := plan.primary()
	if err := docker.RuntimeStartJunimo140Services(ctx, primary.DataDir, auth, images); err != nil {
		return err
	}
	if err := docker.RuntimeStartJunimo140Services(ctx, primary.DataDir, server, images); err != nil {
		return err
	}
	if !bootstrap {
		for i, member := range plan.Members {
			world := txn.Worlds[i]
			if !world.Original.ServerWasRunning {
				continue
			}
			if rollback {
				deadline := time.Now().Add(d.runtimeUpdateServerTimeout)
				ready := false
				for time.Now().Before(deadline) {
					if docker.RuntimeServerHealth(ctx, member.DataDir, member.Project) == nil && runtimeInfoContractReady(ctx, docker, member.DataDir, world.Original.Current.Server.Tag) {
						ready = true
						break
					}
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(d.runtimeUpdatePollInterval):
					}
				}
				if !ready {
					return errors.New("original shared runtime did not recover health")
				}
			} else {
				manifest := runtimeUpdateRecoveryManifest{SchemaVersion: 4, SteamInviteEnabled: world.Original.SteamInviteEnabled, Project: member.Project, Target: txn.Target, TargetServerVersion: txn.Recommendation.Server.Tag, AuthImageChanged: world.Original.SteamInviteEnabled}
				if err := d.verifyRuntimeTarget(ctx, docker, junimo140MemberInstance(member, world), manifest); err != nil {
					return err
				}
				if err := d.verifyJunimo140GroupWorld(ctx, docker, member.DataDir); err != nil {
					return err
				}
			}
		}
	}
	for i := range plan.Members {
		world := txn.Worlds[i]
		state := world.Original.Instance
		if bootstrap && (world.Original.ServerWasRunning || isRunningState(state.State)) {
			state.State = storage.InstanceStateStopped
		} else if !rollback && world.Original.ServerWasRunning {
			state.State = storage.InstanceStateRunning
		} else if !rollback && isRunningState(state.State) {
			state.State = storage.InstanceStateStopped
		}
		// A cold template may be save_required or game_installed. Keeping that
		// state prevents an offline asset upgrade from presenting an empty world
		// as a previously started game; retain its full long-lived payload too.
		if _, err := d.store.RestoreInstanceStateSnapshot(ctx, state); err != nil {
			return err
		}
	}
	return nil
}
