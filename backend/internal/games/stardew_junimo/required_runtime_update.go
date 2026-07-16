package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	requiredRuntimePhaseChecking  = "checking"
	requiredRuntimePhaseRepairing = "repairing"
	requiredRuntimePhasePreflight = "preflighting"
	requiredRuntimePhaseApplying  = "applying"
	requiredRuntimePhaseSucceeded = "succeeded"
	requiredRuntimePhaseFailed    = "failed"
	requiredRuntimePhaseManual    = "manual_action"
	requiredRuntimePhaseNotNeeded = "not_needed"
)

// RequiredRuntimeUpdateStatus is private coordinator state. Public runtime
// dry-run/apply status remains the source of detailed user-visible progress.
// This file only prevents an identical Panel/stack pair from retrying forever
// after a deterministic failure on every Panel restart.
type RequiredRuntimeUpdateStatus struct {
	SchemaVersion int    `json:"schemaVersion"`
	PanelVersion  string `json:"panelVersion"`
	StackVersion  string `json:"stackVersion"`
	Phase         string `json:"phase"`
	ErrorCode     string `json:"errorCode,omitempty"`
	Error         string `json:"error,omitempty"`
	StartedAt     string `json:"startedAt,omitempty"`
	UpdatedAt     string `json:"updatedAt,omitempty"`
	FinishedAt    string `json:"finishedAt,omitempty"`
}

func (d *Driver) StartRequiredRuntimeUpdate(ctx context.Context, instance registry.Instance) {
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil || manifest.RuntimeUpdatePolicy != sjconfig.RuntimeUpdatePolicyRequired {
		return
	}
	if instance.State == storage.InstanceStateUninitialized || instance.State == storage.InstanceStateAdminCreated {
		return
	}

	d.requiredRuntimeMu.Lock()
	if d.requiredRuntimeRunning[instance.ID] {
		d.requiredRuntimeMu.Unlock()
		return
	}
	if previous, readErr := readRequiredRuntimeUpdateStatus(instance.DataDir); readErr == nil && previous.PanelVersion == d.panelVersion && previous.StackVersion == manifest.StackVersion {
		if previous.Phase == requiredRuntimePhaseFailed || previous.Phase == requiredRuntimePhaseManual {
			d.requiredRuntimeMu.Unlock()
			return
		}
		if previous.Phase == requiredRuntimePhaseSucceeded && InspectRuntimeStack(instance.DataDir, instance.State).Status == sjconfig.RuntimeStackStatusUpToDate {
			d.requiredRuntimeMu.Unlock()
			return
		}
	}
	d.requiredRuntimeRunning[instance.ID] = true
	d.requiredRuntimeMu.Unlock()

	go func() {
		defer func() {
			d.requiredRuntimeMu.Lock()
			delete(d.requiredRuntimeRunning, instance.ID)
			d.requiredRuntimeMu.Unlock()
		}()
		if err := d.runRequiredRuntimeUpdate(ctx, instance, manifest); err != nil {
			d.logger.Error("required Junimo runtime update failed", "instance", instance.ID, "stack", manifest.StackVersion, "error", err)
		}
	}()
}

func (d *Driver) requireCurrentRuntimeStack(instance registry.Instance) error {
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil || manifest.RuntimeUpdatePolicy != sjconfig.RuntimeUpdatePolicyRequired {
		return err
	}
	if instance.State == storage.InstanceStateUninitialized || instance.State == storage.InstanceStateAdminCreated {
		return nil
	}
	inspection := InspectRuntimeStack(instance.DataDir, instance.State)
	if inspection.Status == sjconfig.RuntimeStackStatusUpToDate {
		return nil
	}
	return &RuntimeUpdateValidationError{
		Code:    "required_runtime_update",
		Message: "当前 Panel 强制要求 JunimoServer " + manifest.Server.Tag + "；自动升级尚未完成，暂不能启动旧运行组件。",
	}
}

func (d *Driver) runRequiredRuntimeUpdate(ctx context.Context, instance registry.Instance, manifest sjconfig.RuntimeStackManifest) error {
	now := time.Now().UTC().Format(time.RFC3339)
	status := RequiredRuntimeUpdateStatus{SchemaVersion: 1, PanelVersion: d.panelVersion, StackVersion: manifest.StackVersion, Phase: requiredRuntimePhaseChecking, StartedAt: now, UpdatedAt: now}
	set := func(phase, code, message string, terminal bool) error {
		status.Phase, status.ErrorCode, status.Error = phase, code, message
		status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if terminal {
			status.FinishedAt = status.UpdatedAt
		}
		return writeRequiredRuntimeUpdateStatus(instance.DataDir, status)
	}
	if err := set(requiredRuntimePhaseChecking, "", "", false); err != nil {
		return err
	}
	for d.RuntimeUpdateApplyInProgress(instance) || d.SMAPIUpdateApplyInProgress(instance) {
		if apply, err := d.RuntimeUpdateApplyStatus(instance); err == nil && apply.Phase == RuntimeUpdateApplyRollbackFailed {
			_ = set(requiredRuntimePhaseManual, "manual_recovery_required", "已有运行组件升级需要人工恢复，强制升级未继续。", true)
			return errors.New("manual_recovery_required")
		}
		if err := waitRequiredRuntimePoll(ctx, d.runtimeUpdatePollInterval); err != nil {
			_ = set(requiredRuntimePhaseFailed, "context_cancelled", "等待已有维护任务时中断。", true)
			return err
		}
	}

	refreshInstance := func() registry.Instance {
		stored, err := d.store.GetInstance(ctx, instance.ID)
		if err == nil {
			instance.State, instance.DriverPhase, instance.DriverPayload = stored.State, stored.DriverPhase, stored.DriverPayload
		}
		return instance
	}
	instance = refreshInstance()
	inspection := InspectRuntimeStack(instance.DataDir, instance.State)
	if inspection.Status == sjconfig.RuntimeStackStatusUpToDate {
		return set(requiredRuntimePhaseSucceeded, "", "", true)
	}
	if inspection.Repairable {
		if err := set(requiredRuntimePhaseRepairing, "", "", false); err != nil {
			return err
		}
		if _, err := d.RepairRuntimeStackConfig(ctx, instance); err != nil {
			code := runtimeUpdateErrorCode(err)
			_ = set(requiredRuntimePhaseFailed, code, "强制运行组件升级前的可信配置修复失败。", true)
			return err
		}
		instance = refreshInstance()
		inspection = InspectRuntimeStack(instance.DataDir, instance.State)
	}
	if inspection.Status != sjconfig.RuntimeStackStatusUpdateAvailable {
		phase := requiredRuntimePhaseFailed
		if inspection.Status == sjconfig.RuntimeStackStatusCustomImages || inspection.Status == sjconfig.RuntimeStackStatusInvalidConfig {
			phase = requiredRuntimePhaseManual
		}
		_ = set(phase, inspection.Code, inspection.Reason, true)
		return &RuntimeUpdateValidationError{Code: inspection.Code, Message: inspection.Reason}
	}

	if err := set(requiredRuntimePhasePreflight, "", "", false); err != nil {
		return err
	}
	dryRun, err := d.StartRuntimeUpdateDryRun(ctx, instance, 0)
	if err != nil {
		code := runtimeUpdateErrorCode(err)
		_ = set(requiredRuntimePhaseFailed, code, "强制运行组件升级预检无法启动。", true)
		return err
	}
	for {
		if err := waitRequiredRuntimePoll(ctx, d.runtimeUpdatePollInterval); err != nil {
			_ = set(requiredRuntimePhaseFailed, "context_cancelled", "强制运行组件升级已中断。", true)
			return err
		}
		current, readErr := d.RuntimeUpdateDryRunStatus(instance)
		if readErr != nil || current.DryRunID != dryRun.DryRunID {
			continue
		}
		switch current.Phase {
		case RuntimeUpdatePhaseSucceeded:
			goto apply
		case RuntimeUpdatePhaseFailed, RuntimeUpdatePhaseUnsupported:
			_ = set(requiredRuntimePhaseFailed, current.ErrorCode, current.Error, true)
			return errors.New(current.ErrorCode)
		}
	}

apply:
	if err := set(requiredRuntimePhaseApplying, "", "", false); err != nil {
		return err
	}
	instance = refreshInstance()
	applyStatus, err := d.StartRuntimeUpdateApply(ctx, instance, 0)
	if err != nil {
		code := runtimeUpdateErrorCode(err)
		_ = set(requiredRuntimePhaseFailed, code, "强制运行组件升级无法启动。", true)
		return err
	}
	for {
		if err := waitRequiredRuntimePoll(ctx, d.runtimeUpdatePollInterval); err != nil {
			_ = set(requiredRuntimePhaseFailed, "context_cancelled", "强制运行组件升级已中断。", true)
			return err
		}
		current, readErr := d.RuntimeUpdateApplyStatus(instance)
		if readErr != nil || current.ApplyID != applyStatus.ApplyID {
			continue
		}
		switch current.Phase {
		case RuntimeUpdateApplySucceeded:
			return set(requiredRuntimePhaseSucceeded, "", "", true)
		case RuntimeUpdateApplyFailedRolledBack:
			_ = set(requiredRuntimePhaseFailed, current.ErrorCode, current.Error, true)
			return errors.New(current.ErrorCode)
		case RuntimeUpdateApplyRollbackFailed:
			_ = set(requiredRuntimePhaseManual, current.ErrorCode, current.Error, true)
			return errors.New(current.ErrorCode)
		}
	}
}

func waitRequiredRuntimePoll(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = time.Second
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func requiredRuntimeUpdateStatusPath(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "junimo-update", "required-status.json")
}

func readRequiredRuntimeUpdateStatus(dataDir string) (RequiredRuntimeUpdateStatus, error) {
	data, err := readRuntimeStatusFile(requiredRuntimeUpdateStatusPath(dataDir))
	if err != nil {
		return RequiredRuntimeUpdateStatus{}, err
	}
	var status RequiredRuntimeUpdateStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return RequiredRuntimeUpdateStatus{}, fmt.Errorf("decode required runtime update status: %w", err)
	}
	return status, nil
}

func writeRequiredRuntimeUpdateStatus(dataDir string, status RequiredRuntimeUpdateStatus) error {
	path := requiredRuntimeUpdateStatusPath(dataDir)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".required-runtime-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return replaceRuntimeUpdateStatusFile(tmpName, path)
}
