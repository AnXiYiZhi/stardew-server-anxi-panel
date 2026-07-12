package web

import (
	"context"
	"net/http"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
)

type festivalEventTrigger interface {
	TriggerFestivalEvent(ctx context.Context, instance registry.Instance) (*sj.CommandRunResult, error)
}

type jojaRouteEnabler interface {
	EnableJojaRoute(ctx context.Context, instance registry.Instance, confirm string) (*sj.CommandRunResult, error)
}

type gameSaveRequester interface {
	RequestSaveNow(ctx context.Context, instance registry.Instance) (*sj.CommandRunResult, error)
}

type enableJojaRequest struct {
	Confirm string `json:"confirm"`
}

// handleFestivalEventTrigger handles POST /api/instances/:id/festival/event.
func (s *server) handleFestivalEventTrigger(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	instance, ok = s.reconcileInstanceState(w, r, instance)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	trigger, supported := driver.(festivalEventTrigger)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持触发节日活动")
		return
	}

	result, err := trigger.TriggerFestivalEvent(r.Context(), makeRegistryInstance(instance))
	if err != nil {
		if ce, ok := err.(*sj.CommandError); ok {
			status := http.StatusBadRequest
			switch ce.Code {
			case "server_not_running":
				status = http.StatusConflict
			case "not_supported":
				status = http.StatusNotImplemented
			}
			writeError(w, status, ce.Code, ce.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "festival_event_failed", sanitizeErrorMsg(err, "触发节日活动失败"))
		return
	}
	s.recordControlCommandSubmission(r.Context(), actor, instanceID, result, "instance", instanceID, "当前节日")
	s.auditLog(r, &actor, "festival_event_trigger", "instance", instanceID, auditMetadata("commandId", result.CommandID))
	writeJSON(w, http.StatusOK, result)
}

// handleJojaRouteEnable handles POST /api/instances/:id/joja/enable.
func (s *server) handleJojaRouteEnable(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	instance, ok = s.reconcileInstanceState(w, r, instance)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	enabler, supported := driver.(jojaRouteEnabler)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持启用 Joja 路线")
		return
	}

	var body enableJojaRequest
	if !decodeJSON(w, r, &body) {
		return
	}

	result, err := enabler.EnableJojaRoute(r.Context(), makeRegistryInstance(instance), body.Confirm)
	if err != nil {
		if ce, ok := err.(*sj.CommandError); ok {
			status := http.StatusBadRequest
			switch ce.Code {
			case "server_not_running":
				status = http.StatusConflict
			case "not_supported":
				status = http.StatusNotImplemented
			}
			writeError(w, status, ce.Code, ce.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "joja_enable_failed", sanitizeErrorMsg(err, "启用 Joja 路线失败"))
		return
	}
	s.recordControlCommandSubmission(r.Context(), actor, instanceID, result, "instance", instanceID, "Joja 路线")
	s.auditLog(r, &actor, "joja_route_enable", "instance", instanceID, auditMetadata("commandId", result.CommandID, "confirmed", "true"))
	writeJSON(w, http.StatusOK, result)
}

// handleGameSaveRequest handles POST /api/instances/:id/saves/save-now.
func (s *server) handleGameSaveRequest(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	instance, ok = s.reconcileInstanceState(w, r, instance)
	if !ok {
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	requester, supported := driver.(gameSaveRequester)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持游戏内保存请求")
		return
	}

	result, err := requester.RequestSaveNow(r.Context(), makeRegistryInstance(instance))
	if err != nil {
		if ce, ok := err.(*sj.CommandError); ok {
			status := http.StatusBadRequest
			if ce.Code == "server_not_running" {
				status = http.StatusConflict
			}
			writeError(w, status, ce.Code, ce.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "save_request_failed", sanitizeErrorMsg(err, "提交游戏内保存请求失败"))
		return
	}
	s.recordControlCommandSubmission(r.Context(), actor, instanceID, result, "instance", instanceID, "当前存档")
	s.auditLog(r, &actor, "game_save_request", "instance", instanceID, auditMetadata("commandId", result.CommandID))
	writeJSON(w, http.StatusOK, result)
}
