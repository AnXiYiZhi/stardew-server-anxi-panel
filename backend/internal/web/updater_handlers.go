package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updater"
)

type requiredRuntimeStatusReader interface {
	ReadRequiredRuntimeUpdateStatus(registry.Instance) (sj.RequiredRuntimeUpdateStatus, error)
	RuntimeUpdateApplyStatus(registry.Instance) (sj.RuntimeUpdateApplyStatus, error)
}

func (s *server) handleSystemUpdateCapability(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.updater.Capability(r.Context()))
}

func (s *server) handleSystemUpdateDryRun(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		status, err := s.updater.Status()
		if errors.Is(err, updater.ErrNoDryRunStatus) {
			writeError(w, http.StatusNotFound, "dry_run_not_found", "尚无升级环境演练记录")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "dry_run_status_failed", "读取升级演练状态失败")
			return
		}
		writeJSON(w, http.StatusOK, status)
	case http.MethodPost:
		var body struct {
			TargetVersion string `json:"targetVersion"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "请求体解析失败")
			return
		}
		if strings.TrimSpace(body.TargetVersion) == "" {
			writeError(w, http.StatusBadRequest, "missing_field", "targetVersion 不能为空")
			return
		}
		status, err := s.updater.StartDryRun(r.Context(), body.TargetVersion)
		if validation, ok := updater.IsValidationError(err); ok {
			writeError(w, http.StatusBadRequest, validation.Code, validation.Message)
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "dry_run_start_failed", "启动升级环境演练失败")
			return
		}
		writeJSON(w, http.StatusAccepted, status)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *server) handleSystemUpdateApply(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		status, err := s.updater.ApplyStatus()
		if errors.Is(err, updater.ErrNoApplyStatus) {
			writeError(w, http.StatusNotFound, "update_not_found", "尚无面板升级任务")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "update_status_failed", "读取面板升级状态失败")
			return
		}
		s.enrichFullStackUpdateStatus(r.Context(), &status)
		writeJSON(w, http.StatusOK, status)
	case http.MethodPost:
		var body struct {
			ConfirmFullStack bool `json:"confirmFullStack"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "请求体解析失败")
			return
		}
		if !body.ConfirmFullStack {
			writeError(w, http.StatusBadRequest, "full_stack_confirmation_required", "必须确认 Panel 更新可能保存、备份并重启游戏实例")
			return
		}
		versionStatus := s.updateChecker.Status()
		if !versionStatus.UpdateAvailable || strings.TrimSpace(versionStatus.LatestVersion) == "" {
			writeError(w, http.StatusConflict, updater.CodeUpdateNotAvailable, "后端当前没有已确认的正式新版本")
			return
		}
		status, err := s.updater.StartApply(r.Context(), versionStatus.CurrentVersion, versionStatus.LatestVersion)
		if validation, ok := updater.IsValidationError(err); ok {
			httpStatus := http.StatusConflict
			if validation.Code == updater.CodeInvalidTargetVersion {
				httpStatus = http.StatusBadRequest
			}
			writeError(w, httpStatus, validation.Code, validation.Message)
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "update_start_failed", "启动面板升级失败")
			return
		}
		s.enrichFullStackUpdateStatus(r.Context(), &status)
		writeJSON(w, http.StatusAccepted, status)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *server) enrichFullStackUpdateStatus(ctx context.Context, status *updater.ApplyStatus) {
	if status == nil {
		return
	}
	if updater.IsActiveApplyPhase(status.Phase) {
		status.FullStack = &updater.FullStackStatus{Phase: "waiting_panel", Progress: min(status.Progress*40/100, 40), RuntimeRequired: true, Result: "Panel 更新完成后将自动检查并补齐游戏运行栈。"}
		return
	}
	if status.Phase != updater.PhaseSucceeded || normalizeUpdateVersion(status.ToVersion) != normalizeUpdateVersion(s.config.Version) || s.store == nil || s.registry == nil {
		return
	}

	storedInstances, err := s.store.ListInstances(ctx)
	if err != nil {
		status.FullStack = &updater.FullStackStatus{Phase: "failed_safe", Progress: 40, RuntimeRequired: true, ErrorCode: "instances_unavailable", Error: "新 Panel 已启动，但无法枚举游戏实例；运行栈尚未修改。"}
		return
	}
	var items []*updater.FullStackStatus
	for _, stored := range storedInstances {
		items = append(items, s.fullStackInstanceStatus(ctx, makeRegistryInstance(stored)))
	}
	if len(items) == 0 {
		status.FullStack = &updater.FullStackStatus{Phase: "not_needed", Progress: 100, RuntimeRequired: false, Result: "Panel 更新完成；没有需要协调的游戏实例。"}
		return
	}
	aggregate := &updater.FullStackStatus{Phase: "succeeded", Progress: 100, Result: "Panel 与全部游戏实例已完成升级和验证。", Instances: make([]updater.FullStackInstanceStatus, 0, len(items))}
	totalProgress := 0
	for _, item := range items {
		totalProgress += item.Progress
		aggregate.RuntimeRequired = aggregate.RuntimeRequired || item.RuntimeRequired
		aggregate.OnlinePlayers += item.OnlinePlayers
		aggregate.Instances = append(aggregate.Instances, updater.FullStackInstanceStatus{InstanceID: item.InstanceID, Phase: item.Phase, Progress: item.Progress, ServerWasRunning: item.ServerWasRunning, OnlinePlayers: item.OnlinePlayers, BackupName: item.BackupName, ErrorCode: item.ErrorCode, Error: item.Error})
		if item.Phase == "manual_action" {
			aggregate.Phase, aggregate.ErrorCode, aggregate.Error, aggregate.Result = item.Phase, item.ErrorCode, item.Error, "至少一个实例需要人工恢复；其他实例状态已保留。"
		} else if item.Phase == "failed_safe" && aggregate.Phase != "manual_action" {
			aggregate.Phase, aggregate.ErrorCode, aggregate.Error, aggregate.Result = item.Phase, item.ErrorCode, item.Error, "至少一个实例未完成同步，但已安全停止或恢复。"
		} else if item.Progress < 100 && aggregate.Phase != "manual_action" && aggregate.Phase != "failed_safe" {
			aggregate.Phase, aggregate.Result = item.Phase, "正在逐一校验并同步全部游戏实例。"
		}
	}
	aggregate.Progress = totalProgress / len(items)
	status.FullStack = aggregate
}

func (s *server) fullStackInstanceStatus(ctx context.Context, instance registry.Instance) *updater.FullStackStatus {
	driver, err := s.registry.Get(instance.DriverID)
	if err != nil {
		return &updater.FullStackStatus{Phase: "failed_safe", Progress: 40, InstanceID: instance.ID, RuntimeRequired: true, ErrorCode: "driver_unavailable", Error: "新 Panel 已启动，但无法加载游戏驱动；运行栈尚未修改。"}
	}
	reader, ok := driver.(requiredRuntimeStatusReader)
	if !ok {
		return &updater.FullStackStatus{Phase: "not_needed", Progress: 100, InstanceID: instance.ID, RuntimeRequired: false, Result: "Panel 更新完成；当前游戏驱动没有必须跟随更新的运行栈。"}
	}

	required, readErr := reader.ReadRequiredRuntimeUpdateStatus(instance)
	if readErr != nil || normalizeUpdateVersion(required.PanelVersion) != normalizeUpdateVersion(s.config.Version) {
		inspection := sj.InspectManagedRuntimeStack(instance.DataDir, instance.State)
		if inspection.Status == "up_to_date" {
			return &updater.FullStackStatus{Phase: "not_needed", Progress: 100, InstanceID: instance.ID, RuntimeRequired: false, Result: "Panel 与游戏运行栈均已是目标版本。"}
		}
		return &updater.FullStackStatus{Phase: "checking_runtime", Progress: 42, InstanceID: instance.ID, RuntimeRequired: true, Result: "新 Panel 正在接管并检查游戏运行栈。"}
	}

	full := &updater.FullStackStatus{InstanceID: instance.ID, RuntimeRequired: true, ServerWasRunning: required.ServerWasRunning, OnlinePlayers: required.OnlinePlayers, BackupName: required.BackupName, ErrorCode: required.ErrorCode, Error: required.Error, UpdatedAt: required.UpdatedAt, FinishedAt: required.FinishedAt}
	switch required.Phase {
	case "checking", "repairing", "preflighting":
		full.Phase, full.Progress, full.Result = "checking_runtime", 45, "正在检查并准备游戏运行栈。"
	case "notifying_players":
		full.Phase, full.Progress, full.Result = "notifying_players", 48, "正在向在线玩家发送维护通告。"
	case "saving_game":
		full.Phase, full.Progress, full.Result = "saving_game", 52, "正在等待游戏确认保存完成。"
	case "backing_up_save":
		full.Phase, full.Progress, full.Result = "backing_up_save", 56, "正在创建整档保护备份。"
	case "applying":
		full.Phase, full.Progress, full.Result = "updating_runtime", 60, "正在更新、重启并验证游戏运行栈。"
		if apply, applyErr := reader.RuntimeUpdateApplyStatus(instance); applyErr == nil {
			full.Progress = 60 + max(0, min(apply.Progress, 100))*39/100
			switch apply.Phase {
			case sj.RuntimeUpdateApplyVerifyingAuth, sj.RuntimeUpdateApplyVerifyingServer:
				full.Phase, full.Result = "verifying_runtime", "正在验证新版本容器、Junimo、SMAPI 与控制协议。"
			case sj.RuntimeUpdateApplyRestoringState:
				full.Phase, full.Result = "restoring_server", "正在恢复升级前的运行或停止状态。"
			case sj.RuntimeUpdateApplyRollingBack:
				full.Phase, full.Result = "rolling_back_runtime", "运行栈升级未通过，正在自动恢复原版本。"
			}
		}
	case "succeeded":
		full.Phase, full.Progress, full.Result = "succeeded", 100, "Panel、Control 与游戏运行栈已完成升级和验证。"
	case "failed":
		full.Phase, full.Progress, full.Result = "failed_safe", 100, "Panel 已升级；游戏运行栈升级未完成，未修改或已恢复原运行栈。"
	case "manual_action":
		full.Phase, full.Progress, full.Result = "manual_action", 100, "Panel 已升级，但游戏运行栈需要人工恢复。"
	case "not_needed":
		full.Phase, full.Progress, full.RuntimeRequired, full.Result = "not_needed", 100, false, "Panel 与游戏运行栈均已是目标版本。"
	default:
		full.Phase, full.Progress, full.Result = "checking_runtime", 42, "新 Panel 正在接管全栈升级。"
	}
	return full
}

func normalizeUpdateVersion(value string) string {
	return strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(value), "v"), "V")
}
