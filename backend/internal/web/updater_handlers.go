package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updater"
)

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
		writeJSON(w, http.StatusOK, status)
	case http.MethodPost:
		if r.ContentLength != 0 {
			writeError(w, http.StatusBadRequest, "apply_body_not_allowed", "升级目标由后端检测结果决定，请勿提交请求参数")
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
		writeJSON(w, http.StatusAccepted, status)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}
