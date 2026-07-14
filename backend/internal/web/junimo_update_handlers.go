package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type junimoUpdateDryRunDriver interface {
	StartRuntimeUpdateDryRun(context.Context, registry.Instance, int64) (sj.RuntimeUpdateDryRunStatus, error)
	RuntimeUpdateDryRunStatus(registry.Instance) (sj.RuntimeUpdateDryRunStatus, error)
}

type junimoUpdateApplyDriver interface {
	StartRuntimeUpdateApply(context.Context, registry.Instance, int64) (sj.RuntimeUpdateApplyStatus, error)
	RuntimeUpdateApplyStatus(registry.Instance) (sj.RuntimeUpdateApplyStatus, error)
}

type junimoUpdateConfigRepairDriver interface {
	RepairRuntimeStackConfig(context.Context, registry.Instance) (sj.RuntimeStackConfigRepairResult, error)
}

type runtimeComponentsDriver interface {
	InspectRuntimeComponents(context.Context, registry.Instance) (sj.RuntimeComponentsInspection, error)
	RunRuntimeComponentsPreflight(context.Context, registry.Instance) (sj.RuntimeComponentsPreflight, error)
	RuntimeComponentsPreflight(registry.Instance) (sj.RuntimeComponentsPreflight, error)
}

type smapiUpdateDriver interface {
	InspectSMAPIUpdate(context.Context, registry.Instance) (sj.SMAPIUpdateInfo, error)
	RunSMAPIUpdateDryRun(context.Context, registry.Instance) (sj.SMAPIUpdateStatus, error)
	SMAPIUpdateDryRunStatus(registry.Instance) (sj.SMAPIUpdateStatus, error)
	StartSMAPIUpdateApply(context.Context, registry.Instance, int64) (sj.SMAPIUpdateStatus, error)
	SMAPIUpdateApplyStatus(registry.Instance) (sj.SMAPIUpdateStatus, error)
}

type junimoUpdateApplyRequest struct {
	Confirm bool `json:"confirm"`
}

type junimoUpdateResponse struct {
	sjconfig.RuntimeStackInspection
	ReleaseNotes      []string `json:"releaseNotes"`
	ServerRunning     bool     `json:"serverRunning"`
	SteamAuthLoggedIn bool     `json:"steamAuthLoggedIn"`
}

type junimoUpdateConfigRepairResponse struct {
	junimoUpdateResponse
	Repaired bool   `json:"repaired"`
	BackupID string `json:"backupId"`
}

func (s *server) handleInstanceJunimoUpdateConfigRepair(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if !acceptStrictEmptyObject(r.Body) {
		writeError(w, http.StatusBadRequest, "config_repair_body_not_allowed", "请求体只能为空或严格空对象")
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	repairer, ok := driver.(junimoUpdateConfigRepairDriver)
	if !ok {
		writeError(w, http.StatusConflict, "unsupported/driver", "实例 driver 不支持 Junimo 配置修复")
		return
	}
	result, err := repairer.RepairRuntimeStackConfig(r.Context(), makeRegistryInstance(instance))
	if err != nil {
		if validation, yes := sj.IsRuntimeUpdateValidationError(err); yes {
			writeError(w, http.StatusConflict, validation.Code, validation.Message)
			return
		}
		s.logger.Error("failed to repair Junimo runtime config", "instance", instance.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "config_repair_failed", "修复 Junimo 运行组件配置失败；未继续执行升级")
		return
	}
	s.auditLog(r, &actor, "junimo_runtime_config_repaired", "instance", instance.ID, auditMetadata("backupId", result.BackupID, "status", result.Status))
	writeJSON(w, http.StatusOK, junimoUpdateConfigRepairResponse{
		junimoUpdateResponse: junimoUpdateResponse{
			RuntimeStackInspection: result.RuntimeStackInspection,
			ReleaseNotes:           result.Recommended.ReleaseNotes,
			ServerRunning:          instance.State == storage.InstanceStateRunning,
			SteamAuthLoggedIn:      sjconfig.SteamAuthLoggedIn(instance.DataDir),
		},
		Repaired: result.Repaired,
		BackupID: result.BackupID,
	})
}

func (s *server) handleInstanceJunimoUpdateApply(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	applier, ok := driver.(junimoUpdateApplyDriver)
	if !ok {
		writeError(w, http.StatusConflict, "unsupported/driver", "实例 driver 不支持 Junimo 成对升级")
		return
	}
	switch r.Method {
	case http.MethodGet:
		status, err := applier.RuntimeUpdateApplyStatus(makeRegistryInstance(instance))
		if err != nil {
			s.logger.Error("failed to load Junimo update apply status", "instance", instance.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "apply_status_failed", "读取升级执行状态失败")
			return
		}
		writeJSON(w, http.StatusOK, status)
	case http.MethodPost:
		if !acceptStrictApplyConfirmation(r.Body) {
			writeError(w, http.StatusBadRequest, "apply_confirmation_required", "请求体必须严格为 {\"confirm\":true}，且不得包含目标或命令参数")
			return
		}
		status, err := applier.StartRuntimeUpdateApply(r.Context(), makeRegistryInstance(instance), actor.User.ID)
		if err != nil {
			if validation, yes := sj.IsRuntimeUpdateValidationError(err); yes {
				writeError(w, http.StatusConflict, validation.Code, validation.Message)
				return
			}
			s.logger.Error("failed to start Junimo update apply", "instance", instance.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "apply_start_failed", "启动成对升级失败")
			return
		}
		s.auditLog(r, &actor, "junimo_runtime_update_apply_started", "instance", instance.ID, auditMetadata("applyId", status.ApplyID, "targetStackVersion", status.Target.StackVersion))
		writeJSON(w, http.StatusAccepted, status)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func acceptStrictApplyConfirmation(body io.ReadCloser) bool {
	if body == nil {
		return false
	}
	defer body.Close()
	decoder := json.NewDecoder(io.LimitReader(body, 4097))
	decoder.DisallowUnknownFields()
	var request junimoUpdateApplyRequest
	if err := decoder.Decode(&request); err != nil || !request.Confirm {
		return false
	}
	var extra json.RawMessage
	return decoder.Decode(&extra) == io.EOF
}

func (s *server) handleInstanceJunimoUpdateDryRun(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	dryRunner, ok := driver.(junimoUpdateDryRunDriver)
	if !ok {
		writeError(w, http.StatusConflict, "unsupported/driver", "实例 driver 不支持 Junimo 运行组件升级预检")
		return
	}

	switch r.Method {
	case http.MethodGet:
		status, err := dryRunner.RuntimeUpdateDryRunStatus(makeRegistryInstance(instance))
		if err != nil {
			s.logger.Error("failed to load Junimo update dry-run status", "instance", instance.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "dry_run_status_failed", "读取升级预检状态失败")
			return
		}
		writeJSON(w, http.StatusOK, status)
	case http.MethodPost:
		if !acceptStrictEmptyObject(r.Body) {
			writeError(w, http.StatusBadRequest, "dry_run_body_not_allowed", "请求体只能为空或严格空对象")
			return
		}
		status, err := dryRunner.StartRuntimeUpdateDryRun(r.Context(), makeRegistryInstance(instance), actor.User.ID)
		if err != nil {
			if validation, isValidation := sj.IsRuntimeUpdateValidationError(err); isValidation {
				writeError(w, http.StatusConflict, validation.Code, validation.Message)
				return
			}
			s.logger.Error("failed to start Junimo update dry-run", "instance", instance.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "dry_run_start_failed", "启动升级预检失败")
			return
		}
		writeJSON(w, http.StatusAccepted, status)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func acceptStrictEmptyObject(body io.ReadCloser) bool {
	if body == nil {
		return true
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, 4097))
	if err != nil || len(data) > 4096 {
		return false
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return true
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(data, &value); err != nil || value == nil || len(value) != 0 {
		return false
	}
	return true
}

func (s *server) handleInstanceRuntimeComponents(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	inspector, ok := driver.(runtimeComponentsDriver)
	if !ok {
		writeError(w, http.StatusConflict, "unsupported/driver", "实例 driver 不支持游戏运行文件版本检测")
		return
	}
	inspection, err := inspector.InspectRuntimeComponents(r.Context(), makeRegistryInstance(instance))
	if err != nil {
		s.logger.Error("failed to inspect runtime components", "instance", instance.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "runtime_components_failed", "读取游戏运行文件版本失败")
		return
	}
	response := struct {
		sj.RuntimeComponentsInspection
		SMAPI *sj.SMAPIUpdateInfo `json:"smapi,omitempty"`
	}{RuntimeComponentsInspection: inspection}
	if smapiInspector, supported := driver.(smapiUpdateDriver); supported {
		smapi, smapiErr := smapiInspector.InspectSMAPIUpdate(r.Context(), makeRegistryInstance(instance))
		if smapiErr != nil {
			s.logger.Warn("failed to include SMAPI in runtime components", "instance", instance.ID, "error", smapiErr)
		} else {
			response.SMAPI = &smapi
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) handleInstanceSMAPIUpdate(w http.ResponseWriter, r *http.Request, instanceID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	inspector, ok := driver.(smapiUpdateDriver)
	if !ok {
		writeError(w, http.StatusConflict, "unsupported/driver", "实例 driver 不支持 SMAPI 实际版本检测")
		return
	}
	inspection, err := inspector.InspectSMAPIUpdate(r.Context(), makeRegistryInstance(instance))
	if err != nil {
		s.logger.Error("failed to inspect SMAPI", "instance", instance.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "smapi_update_failed", "读取 SMAPI 版本失败")
		return
	}
	writeJSON(w, http.StatusOK, inspection)
}

func (s *server) handleInstanceSMAPIUpdateDryRun(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	workflow, ok := driver.(smapiUpdateDriver)
	if !ok {
		writeError(w, http.StatusConflict, "unsupported/driver", "实例 driver 不支持 SMAPI dry-run")
		return
	}
	switch r.Method {
	case http.MethodGet:
		status, err := workflow.SMAPIUpdateDryRunStatus(makeRegistryInstance(instance))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "smapi_dry_run_status_failed", "读取 SMAPI dry-run 状态失败")
			return
		}
		writeJSON(w, http.StatusOK, status)
	case http.MethodPost:
		if !acceptStrictEmptyObject(r.Body) {
			writeError(w, http.StatusBadRequest, "dry_run_body_not_allowed", "请求体只能为空或严格空对象")
			return
		}
		status, err := workflow.RunSMAPIUpdateDryRun(r.Context(), makeRegistryInstance(instance))
		if err != nil {
			s.logger.Error("SMAPI dry-run failed", "instance", instance.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "smapi_dry_run_failed", "SMAPI dry-run 失败")
			return
		}
		s.auditLog(r, &actor, "smapi_update_dry_run", "instance", instance.ID, auditMetadata("phase", status.Phase))
		writeJSON(w, http.StatusOK, status)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *server) handleInstanceSMAPIUpdateApply(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	workflow, ok := driver.(smapiUpdateDriver)
	if !ok {
		writeError(w, http.StatusConflict, "unsupported/driver", "实例 driver 不支持 SMAPI apply")
		return
	}
	switch r.Method {
	case http.MethodGet:
		status, err := workflow.SMAPIUpdateApplyStatus(makeRegistryInstance(instance))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "smapi_apply_status_failed", "读取 SMAPI apply 状态失败")
			return
		}
		writeJSON(w, http.StatusOK, status)
	case http.MethodPost:
		if !acceptStrictApplyConfirmation(r.Body) {
			writeError(w, http.StatusBadRequest, "apply_confirmation_required", "请求体必须严格为 {\"confirm\":true}，且不得包含目标参数")
			return
		}
		status, err := workflow.StartSMAPIUpdateApply(r.Context(), makeRegistryInstance(instance), actor.User.ID)
		if err != nil {
			if validation, yes := sj.IsRuntimeUpdateValidationError(err); yes {
				writeError(w, http.StatusConflict, validation.Code, validation.Message)
				return
			}
			s.logger.Error("SMAPI apply start failed", "instance", instance.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "smapi_apply_start_failed", "启动 SMAPI apply 失败")
			return
		}
		s.auditLog(r, &actor, "smapi_update_apply_started", "instance", instance.ID, auditMetadata("updateId", status.UpdateID, "targetVersion", status.Target.Version))
		writeJSON(w, http.StatusAccepted, status)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *server) handleInstanceRuntimeComponentsPreflight(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	preflight, ok := driver.(runtimeComponentsDriver)
	if !ok {
		writeError(w, http.StatusConflict, "unsupported/driver", "实例 driver 不支持游戏运行文件只读预检")
		return
	}
	switch r.Method {
	case http.MethodGet:
		status, err := preflight.RuntimeComponentsPreflight(makeRegistryInstance(instance))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "runtime_preflight_status_failed", "读取游戏运行文件预检状态失败")
			return
		}
		writeJSON(w, http.StatusOK, status)
	case http.MethodPost:
		if !acceptStrictEmptyObject(r.Body) {
			writeError(w, http.StatusBadRequest, "dry_run_body_not_allowed", "请求体只能为空或严格空对象")
			return
		}
		status, err := preflight.RunRuntimeComponentsPreflight(r.Context(), makeRegistryInstance(instance))
		if err != nil {
			s.logger.Error("failed to run runtime components preflight", "instance", instance.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "runtime_preflight_failed", "游戏运行文件预检失败")
			return
		}
		s.auditLog(r, &actor, "runtime_components_preflight", "instance", instance.ID, auditMetadata("phase", status.Phase))
		writeJSON(w, http.StatusOK, status)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

// handleInstanceJunimoUpdate is intentionally read-only. It does not accept a
// request body or invoke pull, env update, stop, compose up, or container APIs.
func (s *server) handleInstanceJunimoUpdate(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	inspection := sj.InspectRuntimeStack(instance.DataDir, instance.State)
	writeJSON(w, http.StatusOK, junimoUpdateResponse{
		RuntimeStackInspection: inspection,
		ReleaseNotes:           inspection.Recommended.ReleaseNotes,
		ServerRunning:          instance.State == storage.InstanceStateRunning,
		SteamAuthLoggedIn:      sjconfig.SteamAuthLoggedIn(instance.DataDir),
	})
}
