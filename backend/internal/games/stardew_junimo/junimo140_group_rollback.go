package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func (d *Driver) rollbackJunimo140Group(ctx context.Context, docker junimo140GroupDocker, journal *junimo140GroupJournal, txn *junimo140GroupTransaction, bootstrap bool) (resultErr error) {
	plan := journal.Plan
	primary, _ := plan.primary()
	defer func() {
		if resultErr != nil {
			_ = checkpointJunimo140Group(journal, txn, "rollback_failed")
			_ = writeJunimo140GroupPublicStatus(plan, *txn, RuntimeUpdateApplyRollbackFailed, "shared_runtime_rollback_failed", true)
		}
	}()
	if err := checkpointJunimo140Group(journal, txn, "rolling_back"); err != nil {
		return err
	}
	if !txn.StageCreateIntent && !txn.WorldChangesStarted {
		// Members may already be stopped while a later member's save is still
		// unconfirmed. Never enter the destructive, full-backup rollback then.
		return d.abortJunimo140BeforeStop(ctx, docker, journal, txn, bootstrap)
	}
	if bootstrap || txn.KeepServerStopped {
		txn.KeepServerStopped, bootstrap = true, true
		if err := checkpointJunimo140Group(journal, txn, "rolling_back"); err != nil {
			return err
		}
	}
	if err := docker.RuntimeStopJunimo140Helpers(ctx, primary.DataDir, paneldocker.RuntimeJunimo140HelperScope{ApplyID: plan.ApplyID, Projects: junimo140Projects(plan), ImageRef: txn.Target.Server.Image, ImageID: txn.Target.Server.ImageID}); err != nil {
		return err
	}
	snapshots, err := d.snapshotJunimo140Group(ctx, docker, plan)
	if err != nil {
		return err
	}
	images := junimo140ServiceImages(plan, *txn)
	if err := docker.RuntimeStopJunimo140Services(ctx, primary.DataDir, snapshots, images); err != nil {
		return err
	}
	snapshots, err = d.snapshotJunimo140Group(ctx, docker, plan)
	if err != nil {
		return err
	}
	if err := requireJunimo140Quiescence(snapshots); err != nil {
		return err
	}
	if err := docker.RuntimeAssertJunimo140GameGroup(ctx, primary.DataDir, plan.SourceVolume, junimo140Projects(plan), true); err != nil {
		return err
	}
	if txn.StageCreated && !txn.StageRemoved {
		if err := docker.RuntimeAssertJunimo140GameGroup(ctx, primary.DataDir, plan.StagingVolume, junimo140Projects(plan), true); err != nil {
			return err
		}
	}
	if txn.WorldChangesStarted {
		// Persist all restore intents first. Authoritative recovery can now admit
		// an absent/partial .env only for these exact known member roots.
		for i := range txn.Worlds {
			txn.Worlds[i].RestoreIntent = true
		}
		if err := checkpointJunimo140Group(journal, txn, "rolling_back"); err != nil {
			return err
		}
		if err := docker.RuntimeRemoveJunimo140Services(ctx, primary.DataDir, snapshots, images); err != nil {
			return err
		}
		for i, member := range plan.Members {
			if txn.Worlds[i].Restored {
				continue
			}
			request, err := d.junimo140ArchiveRequest(plan, *txn, i, false)
			if err != nil {
				return err
			}
			if err := docker.RuntimeRestoreJunimo140Instance(ctx, member.DataDir, request); err != nil {
				return err
			}
			if txn.Worlds[i].Original.SessionVolume != "" {
				session, err := d.junimo140ArchiveRequest(plan, *txn, i, true)
				if err != nil {
					return err
				}
				if err := docker.RuntimeRestoreJunimo140Session(ctx, member.DataDir, session, txn.Worlds[i].Original.SessionVolume); err != nil {
					return err
				}
			}
			if err := validateJunimo140OriginalFiles(member, txn.Worlds[i]); err != nil {
				return err
			}
			volume, err := GameDataVolumeName(member.DataDir)
			if err != nil || volume != plan.SourceVolume {
				return errors.New("restored shared runtime game binding does not match original")
			}
			txn.Worlds[i].Restored = true
			if err := checkpointJunimo140Group(journal, txn, "rolling_back"); err != nil {
				return err
			}
		}
		// Recreate after every root is restored. No --start or Compose depends_on
		// is used, and each service uses the captured immutable local image ID.
		for i, member := range plan.Members {
			original := txn.Worlds[i].Original
			if original.ServerPresent {
				if err := docker.RuntimeCreateJunimo140ServicePinned(ctx, member.DataDir, member.Project, "server", original.Server.ImageID); err != nil {
					return err
				}
			}
			if original.AuthPresent {
				if err := docker.RuntimeCreateJunimo140ServicePinned(ctx, member.DataDir, member.Project, "steam-auth", original.Auth.ImageID); err != nil {
					return err
				}
			}
		}
	}
	if txn.StageCreateIntent && !txn.StageRemoved {
		if err := docker.RuntimeRemoveJunimo140Staging(ctx, primary.DataDir, primary.Project, plan.StagingVolume, plan.ApplyID); err != nil {
			return err
		}
		txn.StageRemoved = true
		if err := checkpointJunimo140Group(journal, txn, "rolling_back"); err != nil {
			return err
		}
	}
	if err := d.restoreJunimo140GroupState(ctx, docker, plan, *txn, true, bootstrap); err != nil {
		return err
	}
	if err := checkpointJunimo140Group(journal, txn, "rolled_back"); err != nil {
		return err
	}
	if err := writeJunimo140GroupPublicStatus(plan, *txn, RuntimeUpdateApplyFailedRolledBack, "shared_runtime_rolled_back", bootstrap); err != nil {
		return err
	}
	d.auditJunimo140GroupTerminal(ctx, plan, *txn)
	return releaseJunimo140Group(*journal)
}

// Called under the common driver mutation mutex before legacy recovery reads
// .env or repairs Compose. A peer can resolve the same authoritative parent.
func (d *Driver) recoverJunimo140GroupLocked(ctx context.Context, instance registry.Instance) (bool, error) {
	if _, err := os.Lstat(junimo140GroupLeasePath(instance.DataDir)); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return true, err
	}
	_, err := readJunimo140GroupLease(instance.DataDir)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	lister, ok := d.store.(junimo140InstanceLister)
	if !ok {
		return true, errors.New("shared runtime recovery requires authoritative instance enumeration")
	}
	instances, err := lister.ListInstances(ctx)
	if err != nil {
		return true, err
	}
	journal, err := readRecoverableJunimo140Group(instances, instance)
	if err != nil {
		return true, err
	}
	if d.jobs != nil {
		for _, member := range journal.Plan.Members {
			active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: member.ID})
			if err != nil {
				return true, err
			}
			if len(active) > 0 {
				return true, errors.New("shared runtime recovery waits for active jobs to finish")
			}
		}
	}
	txn, err := readJunimo140GroupTransaction(journal.Plan)
	if errors.Is(err, os.ErrNotExist) {
		// Capturing originals and acquiring leases perform no runtime mutation.
		// A crash before transaction.json therefore needs only release, never a
		// speculative stop, image pull or file reconstruction.
		if journal.Phase != "claiming" && journal.Phase != "claimed" && journal.Phase != "claim_failed" {
			return true, errors.New("shared runtime mutation record is missing")
		}
		if err := writeJunimo140GroupJournal(&journal, "claim_failed"); err != nil {
			return true, err
		}
		return true, releaseJunimo140Group(journal)
	}
	if err != nil {
		return true, err
	}
	if txn.Phase == "claim_failed" {
		if err := writeJunimo140GroupJournal(&journal, "claim_failed"); err != nil {
			return true, err
		}
		if err := writeJunimo140GroupPublicStatus(journal.Plan, txn, RuntimeUpdateApplyFailedRolledBack, "shared_runtime_initialization_failed", false); err != nil {
			return true, err
		}
		d.auditJunimo140GroupTerminal(ctx, journal.Plan, txn)
		return true, releaseJunimo140Group(journal)
	}
	if txn.Phase == "committed" || txn.Phase == "rolled_back" {
		if err := writeJunimo140GroupJournal(&journal, txn.Phase); err != nil {
			return true, err
		}
		phase, code := RuntimeUpdateApplySucceeded, ""
		if txn.Phase == "rolled_back" {
			phase, code = RuntimeUpdateApplyFailedRolledBack, "shared_runtime_rolled_back"
			if !junimo140GroupHasStopIntent(txn) && !txn.WorldChangesStarted {
				code = "shared_runtime_aborted_before_stop"
			}
		}
		if err := writeJunimo140GroupPublicStatus(journal.Plan, txn, phase, code, txn.KeepServerStopped); err != nil {
			return true, err
		}
		d.auditJunimo140GroupTerminal(ctx, journal.Plan, txn)
		return true, releaseJunimo140Group(journal)
	}
	docker, ok := d.docker.(junimo140GroupDocker)
	if !ok {
		return true, errors.New("shared runtime recovery Docker contract unavailable")
	}
	recoveryCtx, cancel := context.WithTimeout(ctx, 2*time.Hour)
	defer cancel()
	return true, d.rollbackJunimo140Group(recoveryCtx, docker, &journal, &txn, true)
}
