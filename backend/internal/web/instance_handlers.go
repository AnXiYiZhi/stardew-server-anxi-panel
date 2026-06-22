package web

import (
	"errors"
	"net/http"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type instanceStateResponse struct {
	InstanceID   string  `json:"instanceId"`
	DriverID     string  `json:"driverId"`
	State        string  `json:"state"`
	StateMessage *string `json:"stateMessage"`
	LastJobID    *string `json:"lastJobId"`
	UpdatedAt    string  `json:"updatedAt"`
	UpdatedBy    *int64  `json:"updatedBy"`
}

func (s *server) handleStardewState(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	state, err := s.store.EnsureDefaultInstanceState(r.Context())
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "instance_not_found", "实例状态不存在")
			return
		}
		s.logger.Error("failed to load stardew state", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	writeJSON(w, http.StatusOK, makeInstanceStateResponse(state))
}

func makeInstanceStateResponse(state storage.InstanceState) instanceStateResponse {
	return instanceStateResponse{
		InstanceID:   state.InstanceID,
		DriverID:     state.DriverID,
		State:        state.State,
		StateMessage: nullableString(state.StateMessage),
		LastJobID:    nullableString(state.LastJobID),
		UpdatedAt:    state.UpdatedAt,
		UpdatedBy:    nullableInt64(state.UpdatedBy),
	}
}
