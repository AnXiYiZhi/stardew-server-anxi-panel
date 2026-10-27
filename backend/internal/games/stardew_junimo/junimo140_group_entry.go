package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
)

func (d *Driver) junimo140PlanFromStore(ctx context.Context, instance registry.Instance, applyID string) (junimo140GroupPlan, error) {
	lister, ok := d.store.(junimo140InstanceLister)
	if !ok {
		return junimo140GroupPlan{}, errors.New("shared runtime migration requires authoritative instance enumeration")
	}
	instances, err := lister.ListInstances(ctx)
	if err != nil {
		return junimo140GroupPlan{}, err
	}
	return buildJunimo140GroupPlan(instances, instance, applyID)
}

// The public apply entry passes only the current trusted built-in
// recommendation. A browser cannot supply target images, peers or root paths.
func (d *Driver) startJunimo140GroupLocked(ctx context.Context, instance registry.Instance, status RuntimeUpdateApplyStatus, dryRun RuntimeUpdateDryRunStatus, confirmedScope string) (RuntimeUpdateApplyStatus, error) {
	if !junimo140GroupSHA256.MatchString(confirmedScope) || confirmedScope != dryRun.SharedGroupSHA256 {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "shared_group_confirmation_required", Message: "请由管理员完成预检，并确认本次共享世界范围后再应用升级。"}
	}
	docker, ok := d.docker.(junimo140GroupDocker)
	if !ok {
		return RuntimeUpdateApplyStatus{}, errors.New("shared runtime migration Docker contract unavailable")
	}
	lister, ok := d.store.(junimo140InstanceLister)
	if !ok {
		return RuntimeUpdateApplyStatus{}, errors.New("shared runtime migration requires authoritative instance enumeration")
	}
	instances, err := lister.ListInstances(ctx)
	if err != nil {
		return RuntimeUpdateApplyStatus{}, err
	}
	plan, err := buildJunimo140GroupPlan(instances, instance, status.ApplyID)
	if err != nil {
		return RuntimeUpdateApplyStatus{}, err
	}
	if dryRun.SharedGroupSHA256 != plan.membershipDigest() || dryRun.SharedGroupMembers != len(plan.Members) {
		return RuntimeUpdateApplyStatus{}, &RuntimeUpdateValidationError{Code: "shared_group_changed", Message: "共享组件的世界范围发生变化，请重新预检并确认。"}
	}
	txn, err := d.captureJunimo140Group(ctx, docker, instances, plan, status)
	if err != nil {
		return RuntimeUpdateApplyStatus{}, err
	}
	journal, err := claimJunimo140Group(plan)
	if err != nil {
		return RuntimeUpdateApplyStatus{}, err
	}
	gate := make(chan struct{})
	var initializeErr error
	job, err := d.jobs.Start(ctx, jobs.Spec{Type: RuntimeUpdateApplyJobType, DisplayName: "Junimo 共享组件升级", TargetType: "instance", TargetID: instance.ID, CreatedBy: status.CreatedBy, Timeout: 2 * time.Hour, Run: func(runCtx context.Context, job *jobs.Context) error {
		<-gate
		if initializeErr != nil {
			return initializeErr
		}
		return d.runJunimo140GroupApply(runCtx, job, docker, journal, txn)
	}})
	if err != nil {
		if markErr := writeJunimo140GroupJournal(&journal, "claim_failed"); markErr != nil {
			return RuntimeUpdateApplyStatus{}, errors.Join(err, markErr)
		}
		return RuntimeUpdateApplyStatus{}, errors.Join(err, releaseJunimo140Group(journal))
	}
	status.JobID, status.Selected = job.ID, txn.Target
	txn.JobID = job.ID
	txn.InputSHA256 = txn.inputDigest()
	initializeErr = writeJunimo140GroupTransaction(plan, &txn)
	if initializeErr == nil {
		initializeErr = writeJunimo140GroupPublicStatus(plan, txn, RuntimeUpdateApplyChecking, "", false)
	}
	defer close(gate)
	if initializeErr != nil {
		// No save, stop or data mutation has run while the gate is closed.
		// Retain evidence but release all leases after a durable failed claim.
		if record, readErr := readJunimo140GroupTransaction(plan); readErr == nil {
			record.Phase = "claim_failed"
			if err := writeJunimo140GroupTransaction(plan, &record); err != nil {
				return RuntimeUpdateApplyStatus{}, errors.Join(initializeErr, err)
			}
		} else if !errors.Is(readErr, os.ErrNotExist) {
			return RuntimeUpdateApplyStatus{}, errors.Join(initializeErr, readErr)
		}
		if err := writeJunimo140GroupJournal(&journal, "claim_failed"); err != nil {
			return RuntimeUpdateApplyStatus{}, errors.Join(initializeErr, err)
		}
		if err := writeJunimo140GroupPublicStatus(plan, txn, RuntimeUpdateApplyFailedRolledBack, "shared_runtime_initialization_failed", false); err != nil {
			return RuntimeUpdateApplyStatus{}, errors.Join(initializeErr, err)
		}
		initializeErr = errors.Join(initializeErr, releaseJunimo140Group(journal))
		return RuntimeUpdateApplyStatus{}, initializeErr
	}
	return status, nil
}
