package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
)

func (d *Driver) StartFullStackRecovery(ctx context.Context, actor int64, id, hash string) (FullStackUpgradeStatus, error) {
	d.fullStackMu.Lock()
	defer d.fullStackMu.Unlock()
	op, err := d.readFullStackOperation()
	if err != nil {
		return FullStackUpgradeStatus{}, err
	}
	if actor != op.Public.ActorID || id != op.Public.OperationID || op.Public.AuthorizedAt == nil || hash == "" || hash != op.Public.PlanSHA256 {
		return op.Public, errors.New("recovery does not match the original full-stack authorization")
	}
	if d.fullStackRunning || op.Public.Phase == "recovering" {
		return op.Public, nil
	}
	if op.Public.Phase != "manual_action" && op.Public.Phase != "failed_safe" {
		return op.Public, errors.New("this operation is not waiting for recovery")
	}
	if d.jobs == nil {
		return op.Public, errors.New("recovery jobs are unavailable")
	}
	gate := make(chan struct{})
	var initialErr error
	job, err := d.jobs.Start(ctx, jobs.Spec{Type: "panel_full_stack_recovery", DisplayName: "恢复本次全栈升级游戏组", TargetType: "system", TargetID: id, CreatedBy: actor, Timeout: 2 * time.Hour, Run: func(runCtx context.Context, job *jobs.Context) error {
		<-gate
		if initialErr != nil {
			return initialErr
		}
		d.runFullStackRecovery(runCtx, op)
		view, err := d.FullStackUpgradeStatus()
		if err == nil && view.Phase == "manual_action" {
			return errors.New(view.Error)
		}
		return err
	}})
	if err != nil {
		return op.Public, err
	}
	op.Public.Phase, op.Public.JobID = "recovering", job.ID
	d.fullStackRunning = true
	initialErr = d.writeFullStackOperation(op)
	if initialErr != nil {
		d.fullStackRunning = false
	}
	close(gate)
	return op.Public, initialErr
}

func (d *Driver) runFullStackRecovery(ctx context.Context, op fullStackOperation) {
	defer func() { d.fullStackMu.Lock(); d.fullStackRunning = false; d.fullStackMu.Unlock() }()
	ownerCtx := context.WithValue(ctx, fullStackContextKey{}, op.Public.OperationID)
	docker, ok := d.docker.(junimo140GroupDocker)
	if !ok {
		d.finishFullStackFailure(&op, "manual_action", "runtime_contract_missing", "共享恢复接口不可用。")
		return
	}
	allCommitted := true
	for i, group := range op.Groups {
		txn, err := readJunimo140GroupTransaction(group.Plan)
		if errors.Is(err, os.ErrNotExist) || err == nil && fullStackClaimWithoutMutation(txn) {
			allCommitted = false
			if err := d.writeFullStackTerminalCompatibility(ownerCtx, &op, i); err != nil {
				d.finishFullStackFailure(&op, "manual_action", "remaining_group_unverified", err.Error())
				return
			}
			continue
		}
		if err != nil {
			d.finishFullStackFailure(&op, "manual_action", "group_journal_invalid", err.Error())
			return
		}
		if txn.Phase != "committed" && txn.Phase != "rolled_back" {
			member, _ := group.Plan.primary()
			if err := d.RecoverRuntimeUpdateApply(ownerCtx, registry.Instance{ID: member.ID, DriverID: DriverID, DataDir: member.DataDir}); err != nil {
				d.finishFullStackFailure(&op, "manual_action", "group_recovery_incomplete", err.Error())
				return
			}
			txn, err = readJunimo140GroupTransaction(group.Plan)
			if err != nil {
				d.finishFullStackFailure(&op, "manual_action", "group_journal_invalid", err.Error())
				return
			}
		}
		if txn.Phase == "committed" {
			if err := d.verifyFullStackGroupTerminal(ownerCtx, docker, group); err != nil {
				d.finishFullStackFailure(&op, "manual_action", "group_terminal_verification_failed", err.Error())
				return
			}
			op.Public.Groups[i].Phase = "succeeded"
		} else if txn.Phase == "rolled_back" {
			allCommitted = false
			op.Public.Groups[i].Phase = RuntimeUpdateApplyFailedRolledBack
		} else {
			d.finishFullStackFailure(&op, "manual_action", "group_recovery_incomplete", "恢复尚未取得持久终态，原 writer 与成员锁继续保留。")
			return
		}
		if err := d.writeFullStackCompatibility(op, i); err != nil {
			d.finishFullStackFailure(&op, "manual_action", "compatibility_receipt_failed", err.Error())
			return
		}
	}
	if allCommitted {
		op.Public.Phase, op.Public.ErrorCode, op.Public.Error = "succeeded", "", ""
	} else {
		op.Public.Phase, op.Public.ErrorCode, op.Public.Error = "failed_safe", "runtime_groups_restored", "Panel 已更新；本次未完成的游戏组已恢复，兼容回执已持久化。按各世界运行状态决定是否手动启动。"
	}
	d.fullStackMu.Lock()
	_ = d.writeFullStackOperation(op)
	d.fullStackMu.Unlock()
}
