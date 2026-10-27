package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updater"
)

type fullStackRequest struct {
	Action, OperationID, PlanSHA256, TargetVersion, NoticeDigest string
	Confirm                                                      bool
}

func runtimeInspectionFromDriver(driver any, instance registry.Instance) sjconfig.RuntimeStackInspection {
	if provider, ok := driver.(interface {
		EffectiveRuntimeInspection(registry.Instance) sjconfig.RuntimeStackInspection
	}); ok {
		return provider.EffectiveRuntimeInspection(instance)
	}
	return sj.InspectRuntimeStack(instance.DataDir, instance.State)
}

func (s *server) instanceRuntimeInspection(instance storage.Instance) sjconfig.RuntimeStackInspection {
	var driver any
	if s.registry != nil {
		driver, _ = s.registry.Get(instance.DriverID)
	}
	return runtimeInspectionFromDriver(driver, makeRegistryInstance(instance))
}

func decodeFullStackRequest(w http.ResponseWriter, r *http.Request) (fullStackRequest, error) {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	var body fullStackRequest
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return body, errors.New("expected one JSON object")
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return body, err
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return body, errors.New("duplicate or invalid field")
		}
		seen[key] = true
		switch key {
		case "action":
			err = decoder.Decode(&body.Action)
		case "operationId":
			err = decoder.Decode(&body.OperationID)
		case "planSha256":
			err = decoder.Decode(&body.PlanSHA256)
		case "targetVersion":
			err = decoder.Decode(&body.TargetVersion)
		case "confirm":
			err = decoder.Decode(&body.Confirm)
		case "noticeDigest":
			err = decoder.Decode(&body.NoticeDigest)
		default:
			return body, errors.New("unknown field")
		}
		if err != nil {
			return body, err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return body, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return body, errors.New("trailing JSON")
	}
	return body, nil
}

func (s *server) fullStackDriver() (*sj.Driver, bool) {
	if s.registry == nil {
		return nil, false
	}
	driver, err := s.registry.Get(sj.DriverID)
	if err != nil {
		return nil, false
	}
	stardew, ok := driver.(*sj.Driver)
	return stardew, ok
}

func (s *server) enrichAuthorizedFullStackStatus(status *updater.ApplyStatus) bool {
	driver, ok := s.fullStackDriver()
	if !ok {
		return false
	}
	op, err := driver.FullStackUpgradeStatus()
	if err != nil || op.AuthorizedAt == nil || op.PanelUpdateID != "" && op.PanelUpdateID != status.UpdateID {
		return false
	}
	if op.Panel.Version != normalizeUpdateVersion(status.ToVersion) {
		return false
	}
	phase := "checking_runtime"
	switch op.Phase {
	case "waiting_panel":
		phase = "waiting_panel"
	case "scope_changed":
		phase = "awaiting_confirmation"
	case "succeeded":
		phase = "succeeded"
	case "panel_failed_rolled_back":
		phase = "failed_safe"
	case "failed_safe", "manual_action":
		phase = op.Phase
	}
	full := &updater.FullStackStatus{Phase: phase, RuntimeRequired: true, ErrorCode: op.ErrorCode, Error: op.Error, Result: "全栈操作 " + op.OperationID + "；游戏组全部验收后才完成。", Instances: []updater.FullStackInstanceStatus{}}
	if phase == "succeeded" {
		full.Progress = 100
		full.Result = "Panel 与本次授权范围内的全部游戏组已完成升级和验收。"
	}
	for _, group := range op.Groups {
		for _, id := range group.Members {
			full.Instances = append(full.Instances, updater.FullStackInstanceStatus{InstanceID: id, Phase: group.Phase, ErrorCode: group.ErrorCode})
		}
	}
	status.FullStack = full
	return true
}

func (s *server) handleFullStackUpdate(w http.ResponseWriter, r *http.Request) {
	admin, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	driver, driverOK := s.fullStackDriver()
	panel, panelOK := s.updater.(sj.FullStackPanelCoordinator)
	if !driverOK || !panelOK {
		writeError(w, http.StatusServiceUnavailable, "full_stack_unavailable", "当前环境尚未配置完整全栈协调器。")
		return
	}
	if r.Method == http.MethodGet {
		status, err := driver.FullStackUpgradeStatus()
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "full_stack_not_found", "尚未创建全栈升级预检。")
			return
		}
		if err != nil {
			writeError(w, http.StatusConflict, "full_stack_recovery_required", "全栈升级记录需要恢复，请保留现有数据。")
			return
		}
		writeJSON(w, http.StatusOK, status)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	body, err := decodeFullStackRequest(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求只接受明确的升级动作及其确认标识。")
		return
	}
	var status sj.FullStackUpgradeStatus
	switch body.Action {
	case "prepare":
		if body.OperationID != "" || body.PlanSHA256 != "" || body.Confirm {
			writeError(w, http.StatusBadRequest, "invalid_body", "预检不接受执行授权。")
			return
		}
		if !driver.FullStackUpgradeAvailable() {
			writeError(w, http.StatusConflict, "game_upgrade_not_in_release", "本版仅更新面板并保留当前游戏运行栈；实例大版本升级将在对应版本提供范围确认。")
			return
		}
		target, normalizeErr := updater.NormalizeTargetVersion(body.TargetVersion)
		current, _ := updater.NormalizeTargetVersion(s.config.Version)
		latest := s.updateChecker.Status()
		if target == current && normalizeUpdateVersion(latest.LatestVersion) == current && latest.Content.Blocked {
			writeError(w, http.StatusConflict, "update_precondition_failed", "此版本暂缓升级，请查看重要提示。")
			return
		}
		if target != current {
			if err := latest.ValidateAction(latest.LatestVersion, "", false); err != nil {
				writeError(w, http.StatusConflict, "update_precondition_failed", err.Error())
				return
			}
		}
		expected, _ := updater.NormalizeTargetVersion(latest.LatestVersion)
		if normalizeErr != nil || target != current && (!latest.UpdateAvailable || target != expected) {
			writeError(w, http.StatusConflict, "full_stack_target_changed", "目标必须是当前 Panel 或后端已核对的精确新版本。")
			return
		}
		if !driver.FullStackUpgradeAppliesToPanelVersion(s.config.Version, target) {
			writeError(w, http.StatusConflict, "game_upgrade_not_in_release", "此目标先按普通面板更新处理；对应实例大版本发布后，再显示精确世界范围确认。")
			return
		}
		pin, pinErr := panel.PreparedPanelTarget(r.Context(), current, target)
		if pinErr != nil {
			writeError(w, http.StatusConflict, "full_stack_panel_preflight_required", pinErr.Error())
			return
		}
		status, err = driver.PrepareFullStackUpgrade(r.Context(), admin.User.ID, pin, target == current)
	case "confirm":
		if !body.Confirm || body.TargetVersion != "" {
			writeError(w, http.StatusBadRequest, "full_stack_confirmation_required", "必须确认已显示的精确目标和共享成员范围。")
			return
		}
		plan, planErr := driver.FullStackUpgradeStatus()
		if planErr != nil {
			err = planErr
			break
		}
		latestAtConfirmation := s.updateChecker.Status()
		if plan.AuthorizedAt == nil {
			latestAtConfirmation = s.updateChecker.Check(r.Context())
		}
		if plan.AuthorizedAt == nil && normalizeUpdateVersion(plan.Panel.Version) != normalizeUpdateVersion(s.config.Version) {
			if err := latestAtConfirmation.ValidateAction("v"+normalizeUpdateVersion(plan.Panel.Version), body.NoticeDigest, true); err != nil {
				writeError(w, http.StatusConflict, "update_precondition_failed", err.Error())
				return
			}
		}
		if plan.AuthorizedAt == nil && normalizeUpdateVersion(plan.Panel.Version) == normalizeUpdateVersion(s.config.Version) {
			latest := latestAtConfirmation
			if normalizeUpdateVersion(latest.LatestVersion) == normalizeUpdateVersion(plan.Panel.Version) {
				if err := latest.ValidateNotice(body.NoticeDigest); err != nil {
					writeError(w, http.StatusConflict, "update_precondition_failed", err.Error())
					return
				}
			}
		}
		pin, pinErr := panel.PreparedPanelTarget(r.Context(), s.config.Version, plan.Panel.Version)
		if pinErr != nil {
			err = pinErr
			break
		}
		status, err = driver.ConfirmFullStackUpgrade(r.Context(), admin.User.ID, body.OperationID, body.PlanSHA256, pin)
	case "cancel":
		if body.Confirm || body.PlanSHA256 != "" || body.TargetVersion != "" {
			writeError(w, http.StatusBadRequest, "invalid_body", "撤销只接受本次操作标识。")
			return
		}
		status, err = driver.CancelPreparedFullStackUpgrade(admin.User.ID, body.OperationID)
	case "recover":
		if !body.Confirm || body.TargetVersion != "" {
			writeError(w, http.StatusBadRequest, "full_stack_confirmation_required", "恢复必须明确确认本次操作。")
			return
		}
		status, err = driver.StartFullStackRecovery(r.Context(), admin.User.ID, body.OperationID, body.PlanSHA256)
	default:
		writeError(w, http.StatusBadRequest, "invalid_action", "未知全栈升级动作。")
		return
	}
	if err != nil {
		var validation *sj.RuntimeUpdateValidationError
		code, message := "full_stack_rejected", err.Error()
		if errors.As(err, &validation) {
			code, message = validation.Code, validation.Message
		}
		writeError(w, http.StatusConflict, code, strings.TrimSpace(message))
		return
	}
	writeJSON(w, http.StatusAccepted, status)
}
