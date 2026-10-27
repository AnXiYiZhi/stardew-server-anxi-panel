package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updatecheck"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updater"
)

func fixedFullStackJunimo140Target() (sjconfig.RuntimeStackRecommendation, error) {
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		return sjconfig.RuntimeStackRecommendation{}, err
	}
	raw, err := jsonMarshalRuntimeRecommendation(manifest)
	if err != nil {
		return sjconfig.RuntimeStackRecommendation{}, err
	}
	target := raw
	if manifest.FullStackUpgrade == nil {
		return sjconfig.RuntimeStackRecommendation{}, errors.New("this Panel release does not contain a full-stack upgrade target")
	}
	target.StackVersion = "explicit-junimo140-auth-" + manifest.FullStackUpgrade.SteamAuth.Tag + "-control-" + manifest.Control.Version
	target.Server = manifest.FullStackUpgrade.Server
	target.SteamAuth = manifest.FullStackUpgrade.SteamAuth
	return target, nil
}

func jsonMarshalRuntimeRecommendation(m sjconfig.RuntimeStackManifest) (sjconfig.RuntimeStackRecommendation, error) {
	// The durable operation carries the same fixed content and Auth identities
	// as the Panel release that provides the migration.
	return sjconfig.RuntimeStackRecommendation{StackVersion: m.StackVersion, Channel: m.Channel, MinimumPanelVersion: m.MinimumPanelVersion, RuntimeUpdatePolicy: m.RuntimeUpdatePolicy, Server: m.Server, SteamAuth: m.SteamAuth, Game: m.Game, SDK: m.SDK, SMAPI: m.SMAPI, Control: m.Control, Status: m.Status, Tested: m.Tested, ReleaseNotes: m.ReleaseNotes}, nil
}

func (d *Driver) PrepareFullStackUpgrade(ctx context.Context, actor int64, panel updater.PanelTarget, continuation bool) (FullStackUpgradeStatus, error) {
	if err := updatecheck.ValidateTransition(d.panelVersion, panel.Version); err != nil {
		return FullStackUpgradeStatus{}, &RuntimeUpdateValidationError{Code: updater.CodeUpgradePrerequisiteRequired, Message: err.Error()}
	}
	if err := updater.BridgeReady(d.containerDataDir, d.panelVersion); err != nil {
		return FullStackUpgradeStatus{}, err
	}
	if actor < 1 || !panel.Valid() || d.jobs == nil || d.store == nil {
		return FullStackUpgradeStatus{}, errors.New("full-stack administrator, exact target and jobs are required")
	}
	d.fullStackMu.Lock()
	defer d.fullStackMu.Unlock()
	if old, err := d.readFullStackOperation(); err == nil {
		if old.Public.AuthorizedAt != nil && (!fullStackTerminal(old.Public.Phase) || old.Public.Phase == "manual_action") {
			return old.Public, &RuntimeUpdateValidationError{Code: "full_stack_busy", Message: "已有全栈升级操作，请继续查看同一任务。"}
		}
		if old.Public.ActorID == actor && old.Public.Panel.SameContent(panel) && old.Public.Continuation == continuation && time.Now().Before(old.Public.ExpiresAt) && (old.Public.Phase == "preparing" || old.Public.Phase == "awaiting_confirmation") {
			return old.Public, nil
		}
		if old.Public.Phase == "preparing" {
			return old.Public, &RuntimeUpdateValidationError{Code: "full_stack_busy", Message: "已有下载预检正在执行，请等待同一预检的持久终态。"}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return FullStackUpgradeStatus{}, err
	}
	target, err := fixedFullStackJunimo140Target()
	if err != nil {
		return FullStackUpgradeStatus{}, err
	}
	id, err := randomHex(16)
	if err != nil {
		return FullStackUpgradeStatus{}, err
	}
	op := fullStackOperation{Schema: 1, Target: target, Public: FullStackUpgradeStatus{OperationID: "stack_" + id, ActorID: actor, Panel: panel, Continuation: continuation, Phase: "preparing", RuntimeVersion: target.Server.Tag, ControlVersion: target.Control.Version, CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(24 * time.Hour), Groups: []FullStackGroupStatus{}}}
	gate := make(chan struct{})
	op.Public.RuntimeDigest = junimo140Digest
	op.Public.ControlSHA256 = target.Control.DLLSHA256
	op.Public.GameBuildID, op.Public.SDKBuildID = target.Game.BuildID, target.SDK.BuildID
	op.Public.SMAPIIdentity = "Junimo .140 image-owned patched SMAPI"
	op.Public.AuthVersion = target.SteamAuth.Tag
	if len(target.SteamAuth.Images) > 0 {
		op.Public.AuthDigest = target.SteamAuth.Digests[target.SteamAuth.Images[0]]
	}
	var initialErr error
	job, err := d.jobs.Start(ctx, jobs.Spec{Type: "panel_full_stack_preflight", DisplayName: "全栈升级下载与范围预检", TargetType: "system", TargetID: op.Public.OperationID, CreatedBy: actor, Timeout: time.Hour, Run: func(runCtx context.Context, job *jobs.Context) error {
		<-gate
		if initialErr != nil {
			return initialErr
		}
		return d.prepareFullStackGroups(runCtx, job, op)
	}})
	if err != nil {
		return FullStackUpgradeStatus{}, err
	}
	op.Public.JobID = job.ID
	initialErr = d.writeFullStackOperation(op)
	close(gate)
	return op.Public, initialErr
}

func (d *Driver) prepareFullStackGroups(ctx context.Context, job *jobs.Context, op fullStackOperation) (resultErr error) {
	defer func() {
		d.fullStackMu.Lock()
		defer d.fullStackMu.Unlock()
		if resultErr != nil {
			op.Public.Phase, op.Public.ErrorCode, op.Public.Error = "preflight_failed", "full_stack_preflight_failed", resultErr.Error()
			var validation *RuntimeUpdateValidationError
			if errors.As(resultErr, &validation) {
				op.Public.ErrorCode, op.Public.Error = validation.Code, validation.Message
			}
		} else {
			op.Public.Phase = "awaiting_confirmation"
			op.Public.PlanSHA256 = op.digest()
		}
		if err := d.writeFullStackOperation(op); err != nil {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	lister, ok := d.store.(junimo140InstanceLister)
	docker, dockerOK := d.docker.(junimo140GroupDocker)
	if !ok || !dockerOK {
		return errors.New("full-stack shared-runtime contracts are unavailable")
	}
	instances, err := lister.ListInstances(ctx)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, stored := range instances {
		if stored.DriverID != DriverID || stored.State == storage.InstanceStateUninitialized || stored.State == storage.InstanceStateAdminCreated || seen[stored.ID] {
			continue
		}
		instance := registry.Instance{ID: stored.ID, DriverID: stored.DriverID, Name: stored.Name, DataDir: stored.DataDir, State: stored.State}
		plan, err := buildJunimo140GroupPlan(instances, instance, newRuntimeApplyID())
		if err != nil {
			return err
		}
		for _, member := range plan.Members {
			seen[member.ID] = true
			if sjconfig.SteamInviteEnabled(member.DataDir) && !sjconfig.HasJunimo140SteamAuthTarget(op.Target.SteamAuth) {
				return &RuntimeUpdateValidationError{Code: "junimo140_auth_target_unavailable", Message: "此组启用了邀请码，但目标未包含已核对版本和 digest 的 .140 SteamAuth。原游戏保持运行，请使用配套完整的升级版本。"}
			}
		}
		// Prefer an already cached, digest-verified candidate without causing
		// needless network work. Missing artifacts still use the trusted order.
		for _, image := range op.Target.Server.TrustedCandidates {
			meta, err := docker.RuntimeImageInspect(ctx, instance.DataDir, image)
			if err == nil && meta.Digest == junimo140Digest && runtimeImageDigestPattern.MatchString(meta.ID) {
				op.Target.Server.TrustedCandidates = append([]string{image}, op.Target.Server.TrustedCandidates...)
				break
			}
		}
		now := time.Now().UTC().Format(time.RFC3339)
		dry := RuntimeUpdateDryRunStatus{DryRunID: newRuntimeDryRunID(), JobID: job.ID, Phase: RuntimeUpdatePhaseStarting, Current: InspectManagedRuntimeStack(instance.DataDir, instance.State).Current, Target: op.Target, Checks: []RuntimeUpdateDryRunCheck{}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{}, StartedAt: now, UpdatedAt: now}
		if err := d.runRuntimeUpdateDryRun(ctx, job, docker, instance, dry); err != nil {
			return err
		}
		dry, err = readRuntimeUpdateDryRunStatus(instance.DataDir)
		if err != nil || dry.Phase != RuntimeUpdatePhaseSucceeded || dry.SharedGroupSHA256 != plan.membershipDigest() {
			return errors.New("full-stack group preflight did not prove the original scope")
		}
		captured, err := d.captureJunimo140Group(ctx, docker, instances, plan, RuntimeUpdateApplyStatus{CreatedBy: op.Public.ActorID, ApplyID: plan.ApplyID, JobID: job.ID, Target: op.Target})
		if err != nil {
			return err
		}
		group := FullStackGroupStatus{PrimaryID: plan.PrimaryID, ScopeSHA256: plan.membershipDigest(), ApplyID: plan.ApplyID, Phase: "prepared", Members: []string{}}
		group.MemberNames = map[string]string{}
		for i, member := range plan.Members {
			for _, stored := range instances {
				if stored.ID == member.ID {
					group.MemberNames[member.ID] = stored.Name
				}
			}
			if captured.Worlds[i].Original.ServerWasRunning {
				group.RunningWorlds++
			}
			if !captured.Worlds[i].Original.HasWorld {
				group.ColdTemplates++
			}
		}
		for _, member := range plan.Members {
			group.Members = append(group.Members, member.ID)
		}
		saveIDs := map[string]string{}
		for _, member := range plan.Members {
			saveIDs[member.ID] = GetActiveSaveName(member.DataDir)
		}
		op.Groups = append(op.Groups, fullStackPreparedGroup{Plan: plan, Original: captured, DryRun: dry, SaveIDs: saveIDs})
		op.Public.Groups = append(op.Public.Groups, group)
		_, _ = job.Info(ctx, fmt.Sprintf("已下载并核对共享组，包含 %d 个世界；尚未停服。", len(plan.Members)))
	}
	return nil
}
