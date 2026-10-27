package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type junimo140InstanceLister interface {
	ListInstances(context.Context) ([]storage.Instance, error)
}

type junimo140GroupDocker interface {
	RuntimeUpdateApplyDockerService
	junimo140SharedGameDocker
	RuntimeSnapshotJunimo140Services(context.Context, string, []paneldocker.RuntimeJunimo140WorldScope, []string) ([]paneldocker.RuntimeJunimo140ServiceSnapshot, error)
	RuntimeStopJunimo140Services(context.Context, string, []paneldocker.RuntimeJunimo140ServiceSnapshot, map[string][]string) error
	RuntimeStartJunimo140Services(context.Context, string, []paneldocker.RuntimeJunimo140ServiceSnapshot, map[string][]string) error
	RuntimeRemoveJunimo140Services(context.Context, string, []paneldocker.RuntimeJunimo140ServiceSnapshot, map[string][]string) error
	RuntimeCreateJunimo140ServicePinned(context.Context, string, string, string, string) error
	RuntimeAssertJunimo140GameGroup(context.Context, string, string, []string, bool) error
	RuntimeStopJunimo140Helpers(context.Context, string, paneldocker.RuntimeJunimo140HelperScope) error
	RuntimeArchiveJunimo140Instance(context.Context, string, paneldocker.RuntimeInstanceArchiveRequest) (string, error)
	RuntimeRestoreJunimo140Instance(context.Context, string, paneldocker.RuntimeInstanceArchiveRequest) error
	RuntimeArchiveJunimo140Session(context.Context, string, paneldocker.RuntimeInstanceArchiveRequest, string) (string, error)
	RuntimeRestoreJunimo140Session(context.Context, string, paneldocker.RuntimeInstanceArchiveRequest, string) error
	RuntimePrepareJunimo140Permissions(context.Context, string, paneldocker.RuntimeInstanceArchiveRequest, string, string) error
	RuntimePrepareJunimo140GamePermissions(context.Context, string, string, string, string, string, string) error
	RuntimeExtractJunimo140Mod(context.Context, string, paneldocker.RuntimeInstanceArchiveRequest) error
	RuntimeCreateJunimo140Staging(context.Context, string, string, string, string) error
	RuntimeRemoveJunimo140Staging(context.Context, string, string, string, string) error
}

var _ junimo140GroupDocker = (*paneldocker.Client)(nil)
var _ junimo140InstanceLister = (*storage.Store)(nil)

func junimo140WorldScopes(plan junimo140GroupPlan, hostPath func(string) (string, error)) ([]paneldocker.RuntimeJunimo140WorldScope, error) {
	var scopes []paneldocker.RuntimeJunimo140WorldScope
	for _, member := range plan.Members {
		path, err := hostPath(member.DataDir)
		if err != nil {
			return nil, err
		}
		scopes = append(scopes, paneldocker.RuntimeJunimo140WorldScope{Project: member.Project, HostDataDir: path})
	}
	return scopes, nil
}

func junimo140Projects(plan junimo140GroupPlan) []string {
	var projects []string
	for _, member := range plan.Members {
		projects = append(projects, member.Project)
	}
	return projects
}

func (d *Driver) junimo140ArchiveRequest(plan junimo140GroupPlan, txn junimo140GroupTransaction, index int, session bool) (paneldocker.RuntimeInstanceArchiveRequest, error) {
	member, world := plan.Members[index], txn.Worlds[index]
	primary, err := plan.primary()
	if err != nil {
		return paneldocker.RuntimeInstanceArchiveRequest{}, err
	}
	root, err := d.dockerHostPath(member.DataDir)
	if err != nil {
		return paneldocker.RuntimeInstanceArchiveRequest{}, err
	}
	backup, err := d.dockerHostPath(filepath.Join(primary.DataDir, ".local-container", "junimo140-group", plan.ApplyID))
	if err != nil {
		return paneldocker.RuntimeInstanceArchiveRequest{}, err
	}
	request := paneldocker.RuntimeInstanceArchiveRequest{Project: member.Project, InstanceHostDir: root, RecoveryHostDir: backup, Filename: member.Archive, ExpectedSHA256: world.ArchiveSHA256, ImageRef: txn.Target.Server.Image, ImageID: txn.Target.Server.ImageID}
	if session {
		request.Filename, request.ExpectedSHA256 = world.AuthArchive, world.AuthArchiveSHA256
	}
	return request, nil
}

// Capture only immutable identities and hashes. Expanded Compose environment,
// session contents and credentials never enter the transaction or public state.
func (d *Driver) captureJunimo140Group(ctx context.Context, docker junimo140GroupDocker, instances []storage.Instance, plan junimo140GroupPlan, status RuntimeUpdateApplyStatus) (junimo140GroupTransaction, error) {
	txn := junimo140GroupTransaction{SchemaVersion: 1, PlanSHA256: plan.digest(), ActorID: status.CreatedBy, JobID: status.JobID, Recommendation: status.Target, Target: status.Selected, Phase: "claimed"}
	primary, _ := plan.primary()
	if _, err := docker.DockerVersion(ctx, primary.DataDir); err != nil {
		return txn, err
	}
	if _, err := docker.ComposeVersion(ctx, primary.DataDir); err != nil {
		return txn, err
	}
	var err error
	txn.Target.Server, _ = selectRuntimeUpdateImage(ctx, docker, primary.DataDir, status.Target.Server.TrustedCandidates, status.Target.Server.Digests)
	if txn.Target.Server.Digest != junimo140Digest {
		return txn, errors.New("fixed shared runtime target is unavailable")
	}
	scopes, err := junimo140WorldScopes(plan, d.dockerHostPath)
	if err != nil {
		return txn, err
	}
	snapshots, err := docker.RuntimeSnapshotJunimo140Services(ctx, primary.DataDir, scopes, []string{plan.SourceVolume})
	if err != nil {
		return txn, err
	}
	if err := docker.RuntimeAssertJunimo140GameGroup(ctx, primary.DataDir, plan.SourceVolume, junimo140Projects(plan), false); err != nil {
		return txn, err
	}
	usedSessions := map[string]bool{}
	for _, member := range plan.Members {
		var stored storage.Instance
		for _, candidate := range instances {
			if candidate.ID == member.ID {
				stored = candidate
				break
			}
		}
		if err := rejectUnfinishedNewGameOwner(member.DataDir); err != nil {
			return txn, err
		}
		if err := d.RejectInstanceDeletion(ctx, member.ID); err != nil {
			return txn, err
		}
		if err := d.rejectActiveSaveImport(ctx, member.ID, ""); err != nil {
			return txn, err
		}
		if d.jobs != nil {
			active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: member.ID})
			if err != nil {
				return txn, err
			}
			for _, job := range active {
				if job.ID != status.JobID {
					return txn, errors.New("a shared runtime world has an active job")
				}
			}
		}
		current := InspectManagedRuntimeStack(member.DataDir, stored.State)
		if current.Status == sjconfig.RuntimeStackStatusCustomImages || current.Status == sjconfig.RuntimeStackStatusInvalidConfig {
			return txn, errors.New("shared runtime uses customized or invalid images")
		}
		if current.Current.Server.Tag != "1.5.0-preview.125" && current.Current.Server.Tag != "1.5.0-preview.140" {
			return txn, errors.New("shared runtime source version requires manual migration")
		}
		compose, err := os.ReadFile(filepath.Join(member.DataDir, "docker-compose.yml"))
		if err != nil {
			return txn, err
		}
		invite := sjconfig.SteamInviteEnabled(member.DataDir)
		if _, err := migrateJunimo140ComposeContract(string(compose), invite); err != nil {
			return txn, err
		}
		original := junimo140WorldOriginal{Instance: stored, Current: current.Current, SteamInviteEnabled: invite}
		if original.ControlDLLSHA256, err = runtimeRecoveryFileSHA256(filepath.Join(smapiModDir(member.DataDir), "StardewAnxiPanel.Control.dll")); err != nil {
			return txn, err
		}
		if original.ControlManifestSHA256, err = runtimeRecoveryFileSHA256(filepath.Join(smapiModDir(member.DataDir), "manifest.json")); err != nil {
			return txn, err
		}
		if original.JunimoDLLSHA256, err = runtimeRecoveryFileSHA256(filepath.Join(junimoServerModDir(member.DataDir), junimoServerAssemblyName)); err != nil {
			return txn, err
		}
		if original.EnvSHA256, err = runtimeRecoveryFileSHA256(filepath.Join(member.DataDir, ".env")); err != nil {
			return txn, err
		}
		if original.ComposeSHA256, err = runtimeRecoveryFileSHA256(filepath.Join(member.DataDir, "docker-compose.yml")); err != nil {
			return txn, err
		}
		var config paneldocker.RuntimeComposeConfig
		if invite {
			config, err = docker.RuntimeComposeConfigInspect(ctx, member.DataDir, member.Project)
		} else {
			config, err = docker.RuntimeComposeConfigInspectServer(ctx, member.DataDir, member.Project)
		}
		if err != nil {
			return txn, err
		}
		if config.Project != "" && config.Project != member.Project {
			return txn, errors.New("shared runtime Compose identity changed")
		}
		for _, snapshot := range snapshots {
			if snapshot.Project != member.Project {
				continue
			}
			selected := RuntimeUpdateSelectedImage{Image: snapshot.Image, ImageID: snapshot.ImageID}
			if snapshot.Service == "server" {
				metadata, err := docker.RuntimeImageInspect(ctx, member.DataDir, snapshot.ImageID)
				expectedDigest := junimo140Digest
				if current.Current.Server.Tag == "1.5.0-preview.125" {
					expectedDigest = "sha256:10f438581d741fc146ce710cbe20099475ac68908e99f565cf449f0b8192ccf6"
				}
				if err != nil || metadata.ID != snapshot.ImageID || metadata.Digest != expectedDigest {
					return txn, errors.New("shared runtime original server does not match the fixed source image")
				}
				selected.Digest = metadata.Digest
				original.Server, original.ServerPresent, original.ServerWasRunning, original.ServerContainerID = selected, true, snapshot.Running, snapshot.ContainerID
			} else {
				original.Auth, original.AuthPresent, original.AuthWasRunning, original.AuthContainerID, original.SessionVolume = selected, true, snapshot.Running, snapshot.ContainerID, snapshot.SessionVolume
			}
		}
		if err := captureJunimo140InactiveServerImage(ctx, docker, member.DataDir, config.Images["server"], &original); err != nil {
			return txn, err
		}
		if invite && original.SessionVolume == "" {
			original.SessionVolume = config.SteamSessionVolume
		}
		if original.AuthPresent && !invite {
			return txn, errors.New("disabled authentication still has a container; stop and remove it before group migration")
		}
		if original.SessionVolume != "" {
			if usedSessions[original.SessionVolume] {
				return txn, errors.New("authentication session is shared between worlds")
			}
			usedSessions[original.SessionVolume] = true
		}
		save := GetActiveSaveName(member.DataDir)
		if save != "" {
			if _, err := stableImportSaveDiskState(member.DataDir, save); err != nil {
				return txn, err
			}
			original.HasWorld = true
		} else if entries, err := os.ReadDir(filepath.Join(savesDir(member.DataDir), "Saves")); err == nil && len(entries) > 0 {
			return txn, errors.New("shared runtime has saves without an unambiguous selected world")
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return txn, err
		}
		if original.ServerWasRunning && !original.HasWorld {
			return txn, errors.New("running shared runtime has no verified selected world")
		}
		progress := junimo140WorldProgress{Original: original, ChildApplyID: junimo140ChildApplyID(plan.ApplyID, member.ID)}
		if original.SessionVolume != "" {
			progress.AuthArchive = junimo140AuthArchiveName(member.ID)
		}
		txn.Worlds = append(txn.Worlds, progress)
	}
	for _, world := range txn.Worlds {
		if world.Original.SteamInviteEnabled {
			txn.Target.SteamAuth, _ = selectRuntimeUpdateImage(ctx, docker, primary.DataDir, status.Target.SteamAuth.TrustedCandidates, status.Target.SteamAuth.Digests)
			if txn.Target.SteamAuth.ImageID == "" {
				return txn, errors.New("shared runtime authentication target unavailable")
			}
			break
		}
	}
	txn.InputSHA256 = txn.inputDigest()
	return txn, nil
}

func checkpointJunimo140Group(journal *junimo140GroupJournal, txn *junimo140GroupTransaction, phase string) error {
	txn.Phase = phase
	if err := writeJunimo140GroupTransaction(journal.Plan, txn); err != nil {
		return err
	}
	return writeJunimo140GroupJournal(journal, phase)
}

func junimo140ServiceImages(plan junimo140GroupPlan, txn junimo140GroupTransaction) map[string][]string {
	images := map[string][]string{}
	for i, member := range plan.Members {
		world := txn.Worlds[i]
		images[member.Project+"/server"] = []string{world.Original.Server.ImageID}
		images[member.Project+"/steam-auth"] = []string{world.Original.Auth.ImageID}
		if world.UpdateIntent {
			images[member.Project+"/server"] = append(images[member.Project+"/server"], txn.Target.Server.ImageID)
			if world.Original.SteamInviteEnabled {
				images[member.Project+"/steam-auth"] = append(images[member.Project+"/steam-auth"], txn.Target.SteamAuth.ImageID)
			}
		}
	}
	return images
}

func (d *Driver) snapshotJunimo140Group(ctx context.Context, docker junimo140GroupDocker, plan junimo140GroupPlan) ([]paneldocker.RuntimeJunimo140ServiceSnapshot, error) {
	primary, _ := plan.primary()
	scopes, err := junimo140WorldScopes(plan, d.dockerHostPath)
	if err != nil {
		return nil, err
	}
	return docker.RuntimeSnapshotJunimo140Services(ctx, primary.DataDir, scopes, []string{plan.SourceVolume, plan.StagingVolume})
}

func requireJunimo140Quiescence(snapshots []paneldocker.RuntimeJunimo140ServiceSnapshot) error {
	for _, snapshot := range snapshots {
		if snapshot.Running {
			return errors.New("shared runtime service is still writing")
		}
	}
	return nil
}

func rejectJunimo140GroupLease(dataDir string) error {
	// Prepare/bootstrap may legitimately run before the instance root exists.
	// Require strict canonical parent validation only when an owner is present;
	// Lstat still distinguishes a dangling lease symlink from an absent owner.
	if _, err := os.Lstat(junimo140GroupLeasePath(dataDir)); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return &RuntimeUpdateValidationError{Code: "shared_runtime_recovery_required", Message: "无法确认共享事务归属，请保留当前数据。"}
	}
	_, err := readJunimo140GroupLease(dataDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return &RuntimeUpdateValidationError{Code: "shared_runtime_recovery_required", Message: "共享运行组件事务记录需要恢复，请保留当前数据。"}
	}
	return &RuntimeUpdateValidationError{Code: "shared_runtime_busy", Message: "此世界的共享运行组件正在升级或恢复，请等待整组任务结束。"}
}

func writeJunimo140GroupPublicStatus(plan junimo140GroupPlan, txn junimo140GroupTransaction, phase, code string, bootstrap bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	for i, member := range plan.Members {
		world := txn.Worlds[i]
		status := RuntimeUpdateApplyStatus{CreatedBy: txn.ActorID, ApplyID: plan.ApplyID, JobID: txn.JobID, Phase: phase, Current: world.Original.Current, Target: txn.Recommendation, Selected: txn.Target, Checks: []RuntimeUpdateDryRunCheck{{Name: "shared_game_group", Status: "ok", Message: fmt.Sprintf("本次共享组件事务包含 %d 个世界。", len(plan.Members))}}, Warnings: []string{}, Logs: []RuntimeUpdateDryRunLog{}, ErrorCode: code, UpdatedAt: now, ServerWasRunning: world.Original.ServerWasRunning}
		if runtimeUpdateApplyTerminal(phase) {
			status.Progress, status.FinishedAt = 100, now
		}
		status.ServerRunning = !bootstrap && world.Original.ServerWasRunning && phase != RuntimeUpdateApplyRollbackFailed
		if phase == RuntimeUpdateApplyFailedRolledBack {
			status.Error = "共享运行组件升级已整组回滚。"
			if code == "shared_runtime_aborted_before_stop" {
				status.Error = "升级在停服前结束，原有运行状态已保留。"
			}
			if code == "shared_runtime_aborted_before_update" {
				status.Error = "升级在共享组件变更前结束，各世界原有数据已保留。"
			}
		}
		if phase == RuntimeUpdateApplyRollbackFailed {
			status.Error = "共享运行组件恢复未完成；恢复材料和成员锁已保留。"
			status.ManualAction = "请保留恢复材料和事务锁，核对当前容器与保存状态后再恢复。"
		}
		if bootstrap && phase != RuntimeUpdateApplyRollbackFailed {
			status.ManualAction = "恢复已收敛，游戏保持关闭；请在面板中手动启动。"
		}
		if err := writeRuntimeUpdateApplyStatus(member.DataDir, status); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) auditJunimo140GroupTerminal(ctx context.Context, plan junimo140GroupPlan, txn junimo140GroupTransaction) {
	for _, member := range plan.Members {
		status, err := readRuntimeUpdateApplyStatus(member.DataDir)
		if err != nil || status.ApplyID != plan.ApplyID || !runtimeUpdateApplyTerminal(status.Phase) {
			continue
		}
		status.CreatedBy = txn.ActorID
		d.auditRuntimeUpdateTerminal(ctx, member.ID, status)
	}
}

func junimo140MemberInstance(member junimo140GroupMember, world junimo140WorldProgress) registry.Instance {
	return registry.Instance{ID: member.ID, DriverID: DriverID, DataDir: member.DataDir, State: world.Original.Instance.State}
}

func validateJunimo140OriginalFiles(member junimo140GroupMember, world junimo140WorldProgress) error {
	for name, want := range map[string]string{".env": world.Original.EnvSHA256, "docker-compose.yml": world.Original.ComposeSHA256} {
		got, err := runtimeRecoveryFileSHA256(filepath.Join(member.DataDir, name))
		if err != nil || got != want {
			return errors.New("shared runtime configuration changed after capture")
		}
	}
	for path, want := range map[string]string{
		filepath.Join(smapiModDir(member.DataDir), "StardewAnxiPanel.Control.dll"):  world.Original.ControlDLLSHA256,
		filepath.Join(smapiModDir(member.DataDir), "manifest.json"):                 world.Original.ControlManifestSHA256,
		filepath.Join(junimoServerModDir(member.DataDir), junimoServerAssemblyName): world.Original.JunimoDLLSHA256,
	} {
		if want == "" {
			continue
		} // Legacy schema-1 journals predate artifact pins.
		got, err := runtimeRecoveryFileSHA256(path)
		if err != nil || got != want {
			return errors.New("shared runtime original artifacts changed after capture")
		}
	}
	return nil
}

func junimo140FixedTarget(target sjconfig.RuntimeStackRecommendation) bool {
	return strings.TrimSpace(target.Server.Tag) == "1.5.0-preview.140"
}
