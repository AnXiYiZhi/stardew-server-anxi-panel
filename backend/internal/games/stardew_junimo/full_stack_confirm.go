package stardew_junimo

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updater"
)

func sameFullStackImageContent(a, b RuntimeUpdateSelectedImage, left, right sjconfig.RuntimeStackManifestComponent) bool {
	if a == b {
		return true
	}
	if !runtimeImageDigestPattern.MatchString(a.ImageID) || !runtimeImageDigestPattern.MatchString(a.Digest) || a.ImageID != b.ImageID || a.Digest != b.Digest {
		return false
	}
	// Cached mirror preference is process-local. Both references must still be
	// declared for this exact digest by each captured immutable recommendation.
	for _, policy := range []sjconfig.RuntimeStackManifestComponent{left, right} {
		if policy.Digests[a.Image] != a.Digest || policy.Digests[b.Image] != b.Digest {
			return false
		}
	}
	return true
}

// Compare deployment scope and original running selection, while allowing real
// gameplay to continue between read-only preflight and administrator consent.
func sameFullStackOriginal(a, b junimo140GroupTransaction) bool {
	if !sameFullStackImageContent(a.Target.Server, b.Target.Server, a.Recommendation.Server, b.Recommendation.Server) ||
		!sameFullStackImageContent(a.Target.SteamAuth, b.Target.SteamAuth, a.Recommendation.SteamAuth, b.Recommendation.SteamAuth) || len(a.Worlds) != len(b.Worlds) {
		return false
	}
	for i := range a.Worlds {
		x, y := a.Worlds[i].Original, b.Worlds[i].Original
		if x.Instance.ID != y.Instance.ID || x.Instance.Name != y.Instance.Name || x.Instance.DriverID != y.Instance.DriverID || !sameJunimoGroupPath(x.Instance.DataDir, y.Instance.DataDir) {
			return false
		}
		x.Instance = storage.Instance{}
		y.Instance = storage.Instance{}
		if !reflect.DeepEqual(x, y) {
			return false
		}
	}
	return true
}

func (d *Driver) validateFullStackGroup(ctx context.Context, op fullStackOperation, index int) error {
	group := op.Groups[index]
	lister, ok := d.store.(junimo140InstanceLister)
	docker, dockerOK := d.docker.(junimo140GroupDocker)
	if !ok || !dockerOK {
		return errors.New("full-stack runtime contracts are unavailable")
	}
	instances, err := lister.ListInstances(ctx)
	if err != nil {
		return err
	}
	var primary registry.Instance
	for _, instance := range instances {
		if instance.ID == group.Plan.PrimaryID {
			primary = registry.Instance{ID: instance.ID, DriverID: instance.DriverID, DataDir: instance.DataDir, State: instance.State}
		}
	}
	plan, err := buildJunimo140GroupPlan(instances, primary, group.Plan.ApplyID)
	if err != nil || plan.digest() != group.Plan.digest() {
		return &RuntimeUpdateValidationError{Code: "full_stack_scope_changed", Message: "共享成员、目录、共享卷或运行身份发生变化，请重新预检确认。"}
	}
	for _, member := range plan.Members {
		if selected, ok := group.SaveIDs[member.ID]; !ok || selected != GetActiveSaveName(member.DataDir) {
			return &RuntimeUpdateValidationError{Code: "full_stack_scope_changed", Message: "预检后活动存档已变化，请重新预检确认。"}
		}
	}
	captured, err := d.captureJunimo140Group(ctx, docker, instances, plan, RuntimeUpdateApplyStatus{CreatedBy: op.Public.ActorID, JobID: op.Public.JobID, ApplyID: plan.ApplyID, Target: op.Target})
	if err != nil {
		return err
	}
	if !sameFullStackOriginal(group.Original, captured) {
		return &RuntimeUpdateValidationError{Code: "full_stack_scope_changed", Message: "预检后的原运行选择、容器、配置或精确目标已变化，请重新确认。"}
	}
	return nil
}

func (d *Driver) ConfirmFullStackUpgrade(ctx context.Context, actor int64, id, hash string, panel updater.PanelTarget) (FullStackUpgradeStatus, error) {
	d.fullStackMu.Lock()
	defer d.fullStackMu.Unlock()
	op, err := d.readFullStackOperation()
	if err != nil {
		return FullStackUpgradeStatus{}, err
	}
	if actor < 1 || actor != op.Public.ActorID || id != op.Public.OperationID || hash == "" || hash != op.Public.PlanSHA256 || hash != op.digest() || !panel.SameContent(op.Public.Panel) {
		return op.Public, &RuntimeUpdateValidationError{Code: "full_stack_authorization_invalid", Message: "管理员、操作、范围或目标与预检不一致，请重新确认。"}
	}
	if op.Public.AuthorizedAt != nil {
		return op.Public, nil
	}
	if op.Public.Phase != "awaiting_confirmation" || !time.Now().Before(op.Public.ExpiresAt) {
		return op.Public, &RuntimeUpdateValidationError{Code: "full_stack_confirmation_expired", Message: "本次确认已过期或预检尚未完成。"}
	}
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	for i := range op.Groups {
		if err := d.validateFullStackGroup(ctx, op, i); err != nil {
			return op.Public, err
		}
	}
	now := time.Now().UTC()
	op.Public.AuthorizedAt = &now
	op.Public.Phase = "waiting_panel"
	if err := d.writeFullStackOperation(op); err != nil {
		return op.Public, err
	}
	return op.Public, nil
}

// Canceling before any group starts revokes the bounded authorization. Once a
// group owns a transaction, its own journal and rollback remain authoritative.
func (d *Driver) CancelPreparedFullStackUpgrade(actor int64, id string) (FullStackUpgradeStatus, error) {
	d.fullStackMu.Lock()
	defer d.fullStackMu.Unlock()
	op, err := d.readFullStackOperation()
	if err != nil {
		return FullStackUpgradeStatus{}, err
	}
	if op.Public.ActorID != actor || op.Public.OperationID != id {
		return op.Public, errors.New("full-stack authorization does not belong to this administrator")
	}
	if fullStackTerminal(op.Public.Phase) {
		return op.Public, nil
	}
	if d.fullStackRunning || op.Public.PanelUpdateID != "" && op.Public.Phase != "scope_changed" || op.Public.Phase == "preparing" {
		return op.Public, &RuntimeUpdateValidationError{Code: "full_stack_busy", Message: "升级已交给独立事务；请等待其持久终态或从恢复入口处理。"}
	}
	for _, group := range op.Public.Groups {
		if group.Phase != "prepared" && group.Phase != "succeeded" && group.Phase != RuntimeUpdateApplyFailedRolledBack {
			return op.Public, errors.New("runtime transaction has already started")
		}
	}
	op.Public.Phase, op.Public.ErrorCode, op.Public.Error = "failed_safe", "administrator_cancelled", "管理员已撤销本次尚未执行的升级授权。"
	return op.Public, d.writeFullStackOperation(op)
}
