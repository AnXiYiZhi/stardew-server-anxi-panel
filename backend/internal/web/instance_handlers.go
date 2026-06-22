package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type instancesResponse struct {
	Instances []instanceResponse `json:"instances"`
}

type instanceResponse struct {
	ID           string  `json:"id"`
	DriverID     string  `json:"driverId"`
	DriverName   string  `json:"driverName,omitempty"`
	Name         string  `json:"name"`
	State        string  `json:"state"`
	StateMessage *string `json:"stateMessage"`
	DriverPhase  string  `json:"driverPhase"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

type instanceStateResponse struct {
	InstanceID   string  `json:"instanceId"`
	DriverID     string  `json:"driverId"`
	Name         string  `json:"name"`
	State        string  `json:"state"`
	StateMessage *string `json:"stateMessage"`
	DriverPhase  string  `json:"driverPhase"`
	UpdatedAt    string  `json:"updatedAt"`
}

type instanceStatusResponse struct {
	Instance instanceResponse       `json:"instance"`
	Status   *registry.ServerStatus `json:"status"`
}

func (s *server) handleInstances(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instances, err := s.store.ListInstances(r.Context())
	if err != nil {
		s.logger.Error("failed to list instances", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	response := instancesResponse{Instances: make([]instanceResponse, 0, len(instances))}
	for _, instance := range instances {
		response.Instances = append(response.Instances, s.makeInstanceResponse(instance))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) handleInstanceByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/instances/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	instanceID := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceDetail(w, r, instanceID)
		return
	}
	if len(parts) == 2 && parts[1] == "state" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceState(w, r, instanceID)
		return
	}
	if len(parts) == 2 && parts[1] == "status" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceStatus(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "docker" && parts[2] == "ps" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceDockerPs(w, r, instanceID)
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "resource not found")
}

func (s *server) handleInstanceDetail(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.makeInstanceResponse(instance))
}

func (s *server) handleInstanceState(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, makeInstanceStateResponse(instance))
}

func (s *server) handleInstanceStatus(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
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
	status, err := driver.Status(r.Context(), makeRegistryInstance(instance))
	if err != nil {
		s.logger.Error("failed to load instance status", "instance", instance.ID, "driver", instance.DriverID, "error", err)
		writeError(w, http.StatusInternalServerError, "driver_status_failed", "实例状态读取失败")
		return
	}
	writeJSON(w, http.StatusOK, instanceStatusResponse{Instance: s.makeInstanceResponse(instance), Status: status})
}

func (s *server) handleInstanceDockerPs(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	s.writeComposePs(w, r, instance.DataDir)
}

func (s *server) loadInstance(w http.ResponseWriter, r *http.Request, instanceID string) (storage.Instance, bool) {
	instance, err := s.store.GetInstance(r.Context(), instanceID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "instance_not_found", "实例不存在")
			return storage.Instance{}, false
		}
		s.logger.Error("failed to load instance", "instance", instanceID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return storage.Instance{}, false
	}
	return instance, true
}

func (s *server) loadDriver(w http.ResponseWriter, driverID string) (registry.GameDriver, bool) {
	driver, err := s.registry.Get(driverID)
	if err != nil {
		if errors.Is(err, registry.ErrDriverNotFound) {
			writeError(w, http.StatusInternalServerError, "driver_not_registered", "实例配置的 driver 未注册")
			return nil, false
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return nil, false
	}
	return driver, true
}

func (s *server) makeInstanceResponse(instance storage.Instance) instanceResponse {
	response := instanceResponse{
		ID:           instance.ID,
		DriverID:     instance.DriverID,
		Name:         instance.Name,
		State:        instance.State,
		StateMessage: nullableString(instance.StateMessage),
		DriverPhase:  instance.DriverPhase,
		CreatedAt:    instance.CreatedAt,
		UpdatedAt:    instance.UpdatedAt,
	}
	if driver, err := s.registry.Get(instance.DriverID); err == nil {
		response.DriverName = driver.Name()
	}
	return response
}

func makeInstanceStateResponse(instance storage.Instance) instanceStateResponse {
	return instanceStateResponse{
		InstanceID:   instance.ID,
		DriverID:     instance.DriverID,
		Name:         instance.Name,
		State:        instance.State,
		StateMessage: nullableString(instance.StateMessage),
		DriverPhase:  instance.DriverPhase,
		UpdatedAt:    instance.UpdatedAt,
	}
}

func makeRegistryInstance(instance storage.Instance) registry.Instance {
	return registry.Instance{
		ID:            instance.ID,
		DriverID:      instance.DriverID,
		Name:          instance.Name,
		DataDir:       instance.DataDir,
		State:         instance.State,
		StateMessage:  instance.StateMessage.String,
		DriverPhase:   instance.DriverPhase,
		DriverPayload: instance.DriverPayload,
		CreatedAt:     instance.CreatedAt,
		UpdatedAt:     instance.UpdatedAt,
	}
}
