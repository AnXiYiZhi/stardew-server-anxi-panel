package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updater"
)

type FullStackPanelCoordinator interface {
	ApplyStatus() (updater.ApplyStatus, error)
	PreparedPanelTarget(context.Context, string, string) (updater.PanelTarget, error)
	StartApplyPinned(context.Context, string, updater.PanelTarget, string) (updater.ApplyStatus, error)
}

// The Panel helper owns only Panel/SQLite rollback. Game groups are independent
// durable transactions, linked by this bounded authorization and never called
// an atomic replacement of both systems.
func (d *Driver) RunFullStackCoordinator(ctx context.Context, panel FullStackPanelCoordinator) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	lastError := ""
	for {
		if err := d.reconcileFullStackPanel(ctx, panel); err != nil && err.Error() != lastError {
			lastError = err.Error()
			d.logger.Warn("full-stack coordinator is waiting for recovery", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *Driver) reconcileFullStackPanel(ctx context.Context, panel FullStackPanelCoordinator) error {
	if d.panelIdentity.Load() == nil {
		if observed, err := panel.PreparedPanelTarget(ctx, d.panelVersion, d.panelVersion); err == nil && observed.Valid() {
			d.panelIdentity.Store(&observed)
		}
	}
	d.fullStackMu.Lock()
	defer d.fullStackMu.Unlock()
	op, err := d.readFullStackOperation()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if fullStackTerminal(op.Public.Phase) || d.fullStackRunning {
		return nil
	}
	if op.Public.AuthorizedAt == nil {
		if op.Public.Phase == "preparing" && d.jobs != nil {
			active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "system", TargetID: op.Public.OperationID})
			if err == nil && len(active) == 0 {
				op.Public.Phase, op.Public.ErrorCode, op.Public.Error = "preflight_failed", "preflight_interrupted", "下载预检中断，未取得执行授权且原游戏未停服，请重新预检。"
				return d.writeFullStackOperation(op)
			}
		}
		return nil
	}
	if op.Public.Phase == "scope_changed" {
		return nil
	}
	panelStatus, statusErr := panel.ApplyStatus()
	if statusErr == nil && panelStatus.OperationID == op.Public.OperationID {
		op.Public.PanelUpdateID = panelStatus.UpdateID
		if panelStatus.PinnedTarget == nil || !panelStatus.PinnedTarget.SameContent(op.Public.Panel) {
			return d.pauseFullStack(&op, "full_stack_target_changed", "Panel helper 的目标身份不匹配，请重新确认。")
		}
		switch panelStatus.Phase {
		case updater.PhaseFailedRolledBack:
			op.Public.Phase, op.Public.ErrorCode, op.Public.Error = "panel_failed_rolled_back", panelStatus.ErrorCode, panelStatus.Error
			return d.writeFullStackOperation(op)
		case updater.PhaseRollbackFailed:
			op.Public.Phase, op.Public.ErrorCode, op.Public.Error = "manual_action", panelStatus.ErrorCode, panelStatus.Error
			return d.writeFullStackOperation(op)
		}
		if updater.IsActiveApplyPhase(panelStatus.Phase) {
			return d.writeFullStackOperation(op)
		}
	}
	observed, inspectErr := panel.PreparedPanelTarget(ctx, d.panelVersion, d.panelVersion)
	if inspectErr != nil {
		return inspectErr
	}
	if observed.SameContent(op.Public.Panel) {
		if !op.Public.Continuation && (statusErr != nil || panelStatus.OperationID != op.Public.OperationID || panelStatus.Phase != updater.PhaseSucceeded || panelStatus.SelectedDigest != op.Public.Panel.ImageID) {
			return nil
		}
		if op.Public.Phase == "recovering" {
			d.fullStackRunning = true
			go d.runFullStackRecovery(ctx, op)
			return nil
		}
		if !time.Now().Before(op.Public.ExpiresAt) && !fullStackHasStartedGroup(op) {
			return d.pauseFullStack(&op, "full_stack_confirmation_expired", "升级授权已过期；请重新预检确认后继续。")
		}
		op.Public.Phase = "migrating"
		if err := d.writeFullStackOperation(op); err != nil {
			return err
		}
		d.fullStackRunning = true
		go d.runFullStackGroups(ctx, op)
		return nil
	}
	if op.Public.Continuation || strings.TrimPrefix(d.panelVersion, "v") == op.Public.Panel.Version {
		return d.pauseFullStack(&op, "full_stack_target_changed", "当前 Panel 内容与已确认目标不同，请重新确认。")
	}
	if op.Public.Phase == "recovering" {
		return d.pauseFullStack(&op, "full_stack_target_changed", "恢复只处理本次游戏组，当前 Panel 身份不匹配；请先核对原 Panel 更新记录。")
	}
	if statusErr == nil && updater.IsActiveApplyPhase(panelStatus.Phase) && panelStatus.OperationID != op.Public.OperationID {
		return d.pauseFullStack(&op, "full_stack_panel_conflict", "另一个 Panel 更新任务正在执行，本次授权未交给该任务。")
	}
	started, err := panel.StartApplyPinned(ctx, d.panelVersion, op.Public.Panel, op.Public.OperationID)
	if err != nil {
		return d.pauseFullStack(&op, "full_stack_panel_start_failed", err.Error())
	}
	op.Public.PanelUpdateID = started.UpdateID
	return d.writeFullStackOperation(op)
}

func (d *Driver) pauseFullStack(op *fullStackOperation, code, message string) error {
	op.Public.Phase, op.Public.ErrorCode, op.Public.Error = "scope_changed", code, message
	return d.writeFullStackOperation(*op)
}

func (d *Driver) runFullStackGroups(ctx context.Context, op fullStackOperation) {
	defer func() { d.fullStackMu.Lock(); d.fullStackRunning = false; d.fullStackMu.Unlock() }()
	ownerCtx := context.WithValue(ctx, fullStackContextKey{}, op.Public.OperationID)
	docker, ok := d.docker.(junimo140GroupDocker)
	if !ok {
		d.finishFullStackFailure(&op, "manual_action", "runtime_contract_missing", "共享事务运行接口不可用。")
		return
	}
	for i, group := range op.Groups {
		if op.Public.Groups[i].Phase == "succeeded" {
			continue
		}
		member, _ := group.Plan.primary()
		instance := registry.Instance{ID: member.ID, DriverID: DriverID, DataDir: member.DataDir}
		current, readErr := readRuntimeUpdateApplyStatus(member.DataDir)
		if readErr != nil || current.ApplyID != group.Plan.ApplyID {
			if ctx.Err() != nil {
				return
			}
			if !time.Now().Before(op.Public.ExpiresAt) {
				d.fullStackMu.Lock()
				_ = d.pauseFullStack(&op, "full_stack_confirmation_expired", "授权已过期；已开始的事务可以恢复，其余游戏组需要重新预检确认。")
				d.fullStackMu.Unlock()
				return
			}
			if err := d.validateFullStackGroup(ownerCtx, op, i); err != nil {
				d.fullStackMu.Lock()
				_ = d.pauseFullStack(&op, "full_stack_scope_changed", err.Error())
				d.fullStackMu.Unlock()
				return
			}
			d.runtimeUpdateMu.Lock()
			now := time.Now().UTC().Format(time.RFC3339)
			current, readErr = d.startJunimo140GroupLocked(ownerCtx, instance, RuntimeUpdateApplyStatus{CreatedBy: op.Public.ActorID, ApplyID: group.Plan.ApplyID, Target: op.Target, Current: group.DryRun.Current, Phase: RuntimeUpdateApplyChecking, StartedAt: now, UpdatedAt: now}, group.DryRun, op.Public.Groups[i].ScopeSHA256)
			d.runtimeUpdateMu.Unlock()
			if readErr != nil {
				d.finishFullStackFailure(&op, "failed_safe", "group_start_failed", readErr.Error())
				return
			}
		}
		op.Public.Groups[i].Phase = "applying"
		d.fullStackMu.Lock()
		writeErr := d.writeFullStackOperation(op)
		d.fullStackMu.Unlock()
		if writeErr != nil {
			return
		}
		for !runtimeUpdateApplyTerminal(current.Phase) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(d.runtimeUpdatePollInterval):
			}
			current, readErr = readRuntimeUpdateApplyStatus(member.DataDir)
			if readErr != nil || current.ApplyID != group.Plan.ApplyID {
				d.finishFullStackFailure(&op, "manual_action", "group_status_unavailable", "事务进度记录缺失或属于另一操作，请保留恢复材料。")
				return
			}
			if txn, err := readJunimo140GroupTransaction(group.Plan); err == nil {
				view := &op.Public.Groups[i]
				view.Phase = txn.Phase
				view.SavedWorlds, view.StoppedWorlds, view.UpdatedWorlds = 0, 0, 0
				for _, world := range txn.Worlds {
					if world.Saved {
						view.SavedWorlds++
					}
					if world.Stopped {
						view.StoppedWorlds++
					}
					if world.Updated {
						view.UpdatedWorlds++
					}
				}
				d.fullStackMu.Lock()
				writeErr = d.writeFullStackOperation(op)
				d.fullStackMu.Unlock()
				if writeErr != nil {
					return
				}
			}
		}
		op.Public.Groups[i].Phase = current.Phase
		op.Public.Groups[i].ErrorCode = current.ErrorCode
		if current.Phase == RuntimeUpdateApplyRollbackFailed {
			d.finishFullStackFailure(&op, "manual_action", current.ErrorCode, current.Error)
			return
		}
		if current.Phase == RuntimeUpdateApplySucceeded {
			if err := d.verifyFullStackGroupTerminal(ownerCtx, docker, group); err != nil {
				d.finishFullStackFailure(&op, "manual_action", "group_terminal_verification_failed", err.Error())
				return
			}
		}
		if err := d.writeFullStackCompatibility(op, i); err != nil {
			d.finishFullStackFailure(&op, "manual_action", "compatibility_receipt_failed", err.Error())
			return
		}
		if current.Phase == RuntimeUpdateApplyFailedRolledBack {
			d.finishFullStackFailure(&op, "failed_safe", current.ErrorCode, "Panel 已更新；此共享组升级失败并已恢复原运行栈。"+current.Error)
			return
		}
		op.Public.Groups[i].Phase = "succeeded"
		d.fullStackMu.Lock()
		writeErr = d.writeFullStackOperation(op)
		d.fullStackMu.Unlock()
		if writeErr != nil {
			return
		}
	}
	op.Public.Phase, op.Public.ErrorCode, op.Public.Error = "succeeded", "", ""
	d.fullStackMu.Lock()
	_ = d.writeFullStackOperation(op)
	d.fullStackMu.Unlock()
}

func fullStackHasStartedGroup(op fullStackOperation) bool {
	for _, group := range op.Groups {
		if _, err := readJunimo140GroupTransaction(group.Plan); err == nil {
			return true
		}
	}
	return false
}

func (d *Driver) verifyFullStackGroupTerminal(ctx context.Context, docker junimo140GroupDocker, group fullStackPreparedGroup) error {
	txn, err := readJunimo140GroupTransaction(group.Plan)
	if err != nil || txn.Phase != "committed" {
		return errors.New("group success has no durable commit")
	}
	snapshots, err := d.snapshotJunimo140Group(ctx, docker, group.Plan)
	if err != nil {
		return err
	}
	for i, member := range group.Plan.Members {
		world := txn.Worlds[i]
		found := false
		for _, snapshot := range snapshots {
			if snapshot.Project != member.Project || snapshot.Service != "server" {
				continue
			}
			found = true
			if snapshot.ImageID != txn.Target.Server.ImageID || snapshot.GameVolume != group.Plan.StagingVolume || snapshot.Running != world.Original.ServerWasRunning {
				return errors.New("group terminal runtime identity or running selection changed")
			}
		}
		if found != (world.Original.ServerPresent || world.Original.HasWorld) {
			return errors.New("group terminal container presence changed")
		}
		if !world.Original.HasWorld && GetActiveSaveName(member.DataDir) != "" {
			return errors.New("cold template unexpectedly gained a world")
		}
		if world.Original.ServerWasRunning {
			manifest := runtimeUpdateRecoveryManifest{SchemaVersion: 4, Project: member.Project, Target: txn.Target, TargetServerVersion: txn.Recommendation.Server.Tag, SteamInviteEnabled: world.Original.SteamInviteEnabled, AuthImageChanged: world.Original.SteamInviteEnabled}
			if err := d.verifyRuntimeTarget(ctx, docker, junimo140MemberInstance(member, world), manifest); err != nil {
				return err
			}
			if err := d.verifyJunimo140GroupWorld(ctx, docker, member.DataDir); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *Driver) finishFullStackFailure(op *fullStackOperation, phase, code, message string) {
	if phase == "failed_safe" && op.Public.AuthorizedAt != nil {
		op.Public.Phase, op.Public.ErrorCode, op.Public.Error = "verifying_remaining", code, message
		d.fullStackMu.Lock()
		writeErr := d.writeFullStackOperation(*op)
		d.fullStackMu.Unlock()
		if writeErr != nil {
			d.logger.Error("cannot persist remaining-group verification", "error", writeErr)
			return
		}
		ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), fullStackContextKey{}, op.Public.OperationID), 5*time.Minute)
		defer cancel()
		for i := range op.Groups {
			if err := d.writeFullStackTerminalCompatibility(ctx, op, i); err != nil {
				phase, code, message = "manual_action", "remaining_group_unverified", message+"；其余组原状态尚未完全核对："+err.Error()
				break
			}
		}
	}
	d.fullStackMu.Lock()
	defer d.fullStackMu.Unlock()
	op.Public.Phase, op.Public.ErrorCode, op.Public.Error = phase, code, message
	if err := d.writeFullStackOperation(*op); err != nil {
		d.logger.Error("cannot persist full-stack terminal", "error", err)
	}
}
