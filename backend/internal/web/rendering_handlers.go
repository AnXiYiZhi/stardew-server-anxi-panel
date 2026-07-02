package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type renderingController interface {
	GetRenderingFPS(ctx context.Context, instance registry.Instance) (*sj.RenderingResult, error)
	SetRenderingFPS(ctx context.Context, instance registry.Instance, fps int) (*sj.RenderingResult, error)
}

type setRenderingRequest struct {
	FPS *int `json:"fps"`
}

// handleInstanceRendering handles GET/POST /api/instances/:id/rendering.
func (s *server) handleInstanceRendering(w http.ResponseWriter, r *http.Request, instanceID string) {
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
	if instance.State != storage.InstanceStateRunning {
		writeError(w, http.StatusConflict, "server_not_running", "服务器未运行，无法控制 VNC 显示")
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	controller, supported := driver.(renderingController)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持 VNC 显示控制")
		return
	}

	if r.Method == http.MethodGet {
		result, err := controller.GetRenderingFPS(r.Context(), makeRegistryInstance(instance))
		if err != nil {
			s.writeRenderingError(w, err, "读取 VNC 显示状态失败")
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}

	var body setRenderingRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "请求格式错误")
		return
	}
	fps := 15
	if body.FPS != nil {
		fps = *body.FPS
	}
	result, err := controller.SetRenderingFPS(r.Context(), makeRegistryInstance(instance), fps)
	if err != nil {
		s.writeRenderingError(w, err, "打开 VNC 显示失败")
		return
	}
	s.auditLog(r, &actor, "instance_rendering_update", "instance", instanceID, auditMetadata("fps", strconv.Itoa(fps)))
	writeJSON(w, http.StatusOK, result)
}

func (s *server) writeRenderingError(w http.ResponseWriter, err error, fallback string) {
	if ce, ok := err.(*sj.CommandError); ok {
		status := http.StatusBadRequest
		switch ce.Code {
		case "server_not_running":
			status = http.StatusConflict
		case "forbidden":
			status = http.StatusForbidden
		case "not_supported":
			status = http.StatusNotImplemented
		}
		writeError(w, status, ce.Code, ce.Message)
		return
	}
	writeError(w, http.StatusBadGateway, "rendering_failed", sanitizeErrorMsg(err, fallback))
}
