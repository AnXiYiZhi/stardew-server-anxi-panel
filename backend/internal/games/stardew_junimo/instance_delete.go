package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type instanceDeletionDocker interface {
	PlanInstanceDeletion(context.Context, string, string, string, string) (paneldocker.DeletionPlan, error)
	ApplyInstanceDeletion(context.Context, string, paneldocker.DeletionPlan) error
}

type instanceDeletionStore interface {
	GetInstanceDeletion(context.Context, string) (storage.InstanceDeletion, error)
	ListInstances(context.Context) ([]storage.Instance, error)
	BeginInstanceDeletion(context.Context, string, string, string) error
	CompleteInstanceDeletion(context.Context, string) error
}

func deletionError(message string) error {
	return &NewGameOwnerError{Code: "instance_delete_blocked", Message: message}
}

// RejectInstanceDeletion also fences bootstrap and background workflows after
// restart. The durable journal must not be replaced by Prepare or reconciliation.
func (d *Driver) RejectInstanceDeletion(ctx context.Context, id string) error {
	if provisions, ok := d.store.(interface {
		InstanceProvisionOwner(context.Context, string) (string, error)
	}); ok {
		token, err := provisions.InstanceProvisionOwner(ctx, id)
		if err != nil {
			return err
		}
		if token != "" && ctx.Value(provisionContextKey{}) != token {
			return deletionError("世界创建或恢复正在占用此实例，请等待完成后重试。")
		}
	}
	store, ok := d.store.(instanceDeletionStore)
	if !ok {
		return nil
	}
	_, err := store.GetInstanceDeletion(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return deletionError("世界删除尚未完成，请返回世界列表重试彻底删除。")
}

func (d *Driver) DeleteInstance(ctx context.Context, instance registry.Instance, defaultID string) error {
	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	store, ok := d.store.(instanceDeletionStore)
	if !ok {
		return deletionError("当前存储不支持安全删除世界。")
	}
	if defaultID == "" || instance.ID == defaultID {
		return deletionError("默认世界不能删除。")
	}
	if provisions, ok := d.store.(instanceProvisionStore); ok {
		if _, err := provisions.GetInstanceProvision(ctx, instance.ID); err == nil {
			return d.cleanupProvisionLocked(ctx, instance)
		} else if !errors.Is(err, storage.ErrNotFound) {
			return err
		}
	}
	engine, ok := d.docker.(instanceDeletionDocker)
	if !ok {
		return deletionError("当前 Docker 环境不支持安全删除世界。")
	}
	root, err := filepath.Abs(filepath.Join(d.containerDataDir, "instances"))
	if err != nil {
		return err
	}
	target, err := filepath.Abs(instance.DataDir)
	if err != nil {
		return err
	}
	if !runtimeComposeProjectPattern.MatchString(instance.ID) || filepath.Join(root, instance.ID) != target {
		return deletionError("世界目录不属于受管实例，无法安全删除。")
	}
	// Resolve every existing ancestor, including the instance root, to reject
	// symlink/reparse aliases. RemoveAll itself does not follow child symlinks.
	for cursor := target; ; cursor = filepath.Dir(cursor) {
		info, e := os.Lstat(cursor)
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if e == nil {
			resolved, e := filepath.EvalSymlinks(cursor)
			if e != nil {
				return e
			}
			absolute, e := filepath.Abs(resolved)
			if e != nil {
				return e
			}
			if info.Mode()&os.ModeSymlink != 0 || absolute != cursor {
				return deletionError("世界目录包含路径别名，无法安全删除。")
			}
		}
		if filepath.Dir(cursor) == cursor {
			break
		}
	}
	if err := validateWorldDeletionTree(target); err != nil {
		return err
	}
	record, recordErr := store.GetInstanceDeletion(ctx, instance.ID)
	if recordErr != nil && !errors.Is(recordErr, storage.ErrNotFound) {
		return recordErr
	}
	if recordErr == nil && record.Completed {
		return nil
	}
	var plan paneldocker.DeletionPlan
	if errors.Is(recordErr, storage.ErrNotFound) {
		current, e := d.store.GetInstance(ctx, instance.ID)
		if e != nil {
			return e
		}
		if current.State == storage.InstanceStateRunning || current.State == storage.InstanceStateStarting || current.DriverPhase == "instance_provisioning" {
			return deletionError("请先停止服务器，并等待当前任务完成后再删除。")
		}
		if e = rejectUnfinishedNewGameOwner(target); e != nil {
			return e
		}
		busy, e := HasUnfinishedImportTransaction(target)
		if e != nil {
			return e
		}
		if busy {
			return deletionError("存档事务尚未结束，请先完成恢复。")
		}
		if status, e := readRuntimeUpdateApplyStatus(target); e != nil && !os.IsNotExist(e) {
			return deletionError("运行栈事务记录不可读，请先恢复事务。")
		} else if e == nil && status.Phase != "idle" && status.Phase != RuntimeUpdateApplySucceeded && status.Phase != RuntimeUpdateApplyFailedRolledBack {
			return deletionError("运行栈升级事务尚未安全结束，请先恢复事务。")
		}
		if status, e := readSMAPIUpdateStatus(target, "apply-status.json"); e != nil && !os.IsNotExist(e) {
			return deletionError("SMAPI 事务记录不可读，请先恢复事务。")
		} else if e == nil && status.Phase != SMAPIApplyIdle && status.Phase != SMAPIApplySucceeded && status.Phase != SMAPIApplyFailedRolledBack {
			return deletionError("SMAPI 升级事务尚未安全结束，请先恢复事务。")
		}
		if d.jobs == nil {
			return deletionError("无法确认当前任务状态。")
		}
		active, e := d.jobs.Active(ctx, storage.ListActiveJobsFilter{})
		if e != nil {
			return e
		}
		for _, job := range active {
			if job.TargetType != "instance" || job.TargetID == instance.ID {
				return deletionError("仍有安装、备份或维护任务，请等待任务完成。")
			}
		}
		volume, e := GameDataVolumeName(target)
		if e != nil {
			return e
		}
		fields, e := sjconfig.ReadEnvFile(filepath.Join(target, ".env"))
		if e != nil {
			return e
		}
		hostDir := strings.TrimSpace(fields["INSTANCE_HOST_DATA_DIR"])
		expectedHost := target
		if d.hostDataDir != "" {
			expectedHost = filepath.Join(d.hostDataDir, "instances", instance.ID)
		}
		normalize := func(v string) string { return strings.TrimRight(strings.ReplaceAll(v, `\`, "/"), "/") }
		if normalize(hostDir) != normalize(expectedHost) {
			return deletionError("世界宿主目录与配置不一致，无法安全删除。")
		}
		existing, e := store.ListInstances(ctx)
		if e != nil {
			return e
		}
		for _, other := range existing {
			if other.ID == instance.ID {
				continue
			}
			otherDir, e := filepath.Abs(other.DataDir)
			if e != nil {
				return e
			}
			if otherDir == target || strings.HasPrefix(otherDir, target+string(filepath.Separator)) || strings.HasPrefix(target, otherDir+string(filepath.Separator)) {
				return deletionError("世界目录与其它实例重叠。")
			}
			if other.DriverID == DriverID {
				otherVolume, e := GameDataVolumeName(other.DataDir)
				if e != nil {
					return e
				}
				if otherVolume == volume {
					return deletionError("游戏数据卷由其它世界共享，无法删除。")
				}
			}
		}
		plan, e = engine.PlanInstanceDeletion(ctx, root, instance.ID, hostDir, volume)
		if e != nil {
			return deletionError("资源归属或停止状态无法确认，请检查世界容器、共享挂载和 Docker 后重试。")
		}
		encoded, e := json.Marshal(plan)
		if e != nil {
			return e
		}
		if e = store.BeginInstanceDeletion(ctx, instance.ID, defaultID, string(encoded)); e != nil {
			return deletionError("世界状态已变化或有活动任务，请等待后重试。")
		}
	} else {
		if err = json.Unmarshal([]byte(record.Plan), &plan); err != nil {
			return err
		}
		if plan.Project != instance.ID || plan.HostDir == "" {
			return deletionError("删除记录无效，已停止资源清理。")
		}
	}
	// Recheck even on retry: another stopped world may have acquired a volume
	// reference since a prior failed attempt without creating a container yet.
	others, err := store.ListInstances(ctx)
	if err != nil {
		return err
	}
	for _, other := range others {
		if other.ID == instance.ID || other.DriverID != DriverID {
			continue
		}
		volume, e := GameDataVolumeName(other.DataDir)
		if e != nil {
			return e
		}
		if _, shared := plan.Volumes[volume]; shared {
			return deletionError("待删除数据卷已被其它世界引用，已停止清理。")
		}
	}
	if err = engine.ApplyInstanceDeletion(ctx, root, plan); err != nil {
		return deletionError("Docker 资源清理未完成，请检查资源占用或 Docker 状态后重试删除。")
	}
	if err := validateWorldDeletionTree(target); err != nil {
		return err
	}
	if err = os.RemoveAll(target); err != nil {
		return deletionError("世界文件或备份清理未完成，请检查目录权限后重试删除。")
	}
	if _, err = os.Lstat(target); !os.IsNotExist(err) {
		return deletionError("世界目录仍存在，请重试删除。")
	}
	return store.CompleteInstanceDeletion(ctx, instance.ID)
}

// Data outside the managed tree must never be traversed as part of recursive
// deletion, including bind mounts and directory aliases invisible to Docker.
func validateWorldDeletionTree(target string) error {
	if runtime.GOOS == "linux" {
		mounts, err := os.ReadFile("/proc/self/mountinfo")
		if err != nil {
			return deletionError("无法核对世界目录挂载边界。")
		}
		unescape := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
		for _, line := range strings.Split(string(mounts), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				continue
			}
			mount := filepath.Clean(unescape.Replace(fields[4]))
			if mount == target || strings.HasPrefix(mount, target+string(filepath.Separator)) {
				return deletionError("世界目录含独立挂载，无法安全递归删除。")
			}
		}
	}
	err := filepath.WalkDir(target, func(path string, entry fs.DirEntry, err error) error {
		if os.IsNotExist(err) && path == target {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return deletionError("世界目录含符号链接，请先核对并解除外部引用。")
		}
		if entry.IsDir() {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			if resolved != path {
				return deletionError("世界目录含路径别名，无法安全删除。")
			}
		}
		return nil
	})
	if err != nil {
		return deletionError("世界目录边界或权限无法确认，请检查后重试删除。")
	}
	return nil
}
