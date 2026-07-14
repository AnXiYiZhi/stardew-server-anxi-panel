package stardew_junimo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type RuntimeStackConfigRepairResult struct {
	sjconfig.RuntimeStackInspection
	Repaired bool   `json:"repaired"`
	BackupID string `json:"backupId"`
}

func (d *Driver) RepairRuntimeStackConfig(ctx context.Context, instance registry.Instance) (RuntimeStackConfigRepairResult, error) {
	if d.jobs == nil {
		return RuntimeStackConfigRepairResult{}, errors.New("runtime config repair service is not configured")
	}
	if instance.DriverID != DriverID {
		return RuntimeStackConfigRepairResult{}, &RuntimeUpdateValidationError{Code: "unsupported/driver", Message: "实例不是 stardew_junimo driver。"}
	}
	if previous, err := readRuntimeUpdateApplyStatus(instance.DataDir); err == nil && previous.Phase == RuntimeUpdateApplyRollbackFailed {
		return RuntimeStackConfigRepairResult{}, &RuntimeUpdateValidationError{Code: "manual_recovery_required", Message: "上次升级自动回滚失败；必须先人工处理并保全恢复材料，禁止自动修复配置。"}
	}

	d.runtimeUpdateMu.Lock()
	defer d.runtimeUpdateMu.Unlock()
	active, err := d.jobs.Active(ctx, storage.ListActiveJobsFilter{
		TargetType: "instance",
		TargetID:   instance.ID,
		Types:      []string{"stardew_install", "stardew_lifecycle", RuntimeUpdateDryRunJobType, RuntimeUpdateApplyJobType, SMAPIUpdateDryRunJobType, SMAPIUpdateApplyJobType},
	})
	if err != nil {
		return RuntimeStackConfigRepairResult{}, fmt.Errorf("list conflicting jobs: %w", err)
	}
	if len(active) > 0 {
		return RuntimeStackConfigRepairResult{}, &RuntimeUpdateValidationError{Code: "runtime_update_busy", Message: "实例存在安装、生命周期或组件升级任务，请等待任务结束。"}
	}

	installed := instance.State != storage.InstanceStateUninitialized && instance.State != storage.InstanceStateAdminCreated
	plan := sjconfig.PlanRuntimeStackConfigRepair(instance.DataDir, installed)
	if !plan.Repairable {
		return RuntimeStackConfigRepairResult{}, &RuntimeUpdateValidationError{Code: plan.Code, Message: plan.Reason}
	}
	envPath := filepath.Join(instance.DataDir, ".env")
	original, err := os.ReadFile(envPath)
	if err != nil {
		return RuntimeStackConfigRepairResult{}, fmt.Errorf("read runtime config before repair: %w", err)
	}
	backupID := newRuntimeDryRunID()
	backupRoot := filepath.Join(instance.DataDir, ".local-container", "junimo-update", "config-repair")
	if err := os.MkdirAll(backupRoot, 0o700); err != nil {
		return RuntimeStackConfigRepairResult{}, fmt.Errorf("create runtime config repair backup: %w", err)
	}
	if err := os.Chmod(backupRoot, 0o700); err != nil {
		return RuntimeStackConfigRepairResult{}, fmt.Errorf("protect runtime config repair backup root: %w", err)
	}
	backupDir := filepath.Join(backupRoot, backupID)
	if err := os.Mkdir(backupDir, 0o700); err != nil {
		return RuntimeStackConfigRepairResult{}, fmt.Errorf("reserve runtime config repair backup: %w", err)
	}
	if err := os.Chmod(backupDir, 0o700); err != nil {
		return RuntimeStackConfigRepairResult{}, fmt.Errorf("protect runtime config repair backup: %w", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "original.env"), original, 0o600); err != nil {
		return RuntimeStackConfigRepairResult{}, fmt.Errorf("write runtime config repair backup: %w", err)
	}
	if err := writeRuntimeEnvUpdatesAtomic(instance.DataDir, original, plan.Updates); err != nil {
		return RuntimeStackConfigRepairResult{}, fmt.Errorf("write repaired runtime config: %w", err)
	}

	inspection := InspectRuntimeStack(instance.DataDir, instance.State)
	if inspection.Status != sjconfig.RuntimeStackStatusUpdateAvailable && inspection.Status != sjconfig.RuntimeStackStatusUpToDate {
		if restoreErr := writeRuntimeEnvBytesAtomic(instance.DataDir, original); restoreErr != nil {
			return RuntimeStackConfigRepairResult{}, fmt.Errorf("config repair verification failed (%s) and original config restore failed: %w", inspection.Code, restoreErr)
		}
		return RuntimeStackConfigRepairResult{}, &RuntimeUpdateValidationError{Code: "config_repair_verification_failed", Message: "候选配置规范化后仍无法确认版本状态，已恢复原配置。"}
	}
	return RuntimeStackConfigRepairResult{RuntimeStackInspection: inspection, Repaired: true, BackupID: backupID}, nil
}

func writeRuntimeEnvUpdatesAtomic(dataDir string, original []byte, updates map[string]string) error {
	tmp, err := os.CreateTemp(dataDir, ".runtime-config-repair-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if err := tmp.Close(); err != nil {
		return err
	}
	defer os.Remove(tmpName)
	if err := os.WriteFile(tmpName, original, 0o600); err != nil {
		return err
	}
	if err := sjconfig.UpdateEnvFile(tmpName, updates); err != nil {
		return err
	}
	return replaceRuntimeUpdateStatusFile(tmpName, filepath.Join(dataDir, ".env"))
}

func writeRuntimeEnvBytesAtomic(dataDir string, data []byte) error {
	tmp, err := os.CreateTemp(dataDir, ".runtime-config-restore-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if err := tmp.Close(); err != nil {
		return err
	}
	defer os.Remove(tmpName)
	if err := os.WriteFile(tmpName, data, 0o600); err != nil {
		return err
	}
	return replaceRuntimeUpdateStatusFile(tmpName, filepath.Join(dataDir, ".env"))
}
