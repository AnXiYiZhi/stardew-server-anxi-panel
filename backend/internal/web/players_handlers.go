package web

import (
	"context"
	"net/http"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
)

type playerLister interface {
	ListPlayers(ctx context.Context, instance registry.Instance) (*sj.PlayersResult, error)
}

type playerKicker interface {
	KickPlayer(ctx context.Context, instance registry.Instance, uniqueMultiplayerID, name string) (*sj.CommandRunResult, error)
}

type playerHomeWarper interface {
	WarpPlayerHome(ctx context.Context, instance registry.Instance, uniqueMultiplayerID, name string) (*sj.CommandRunResult, error)
}

type playerAuthApprover interface {
	ApproveAuth(ctx context.Context, instance registry.Instance, uniqueMultiplayerID string) (*sj.CommandRunResult, error)
}

type playerBanner interface {
	BanPlayer(ctx context.Context, instance registry.Instance, name, uniqueMultiplayerID string) (*sj.CommandRunResult, error)
}

type farmhandDeleter interface {
	DeleteFarmhand(ctx context.Context, req sj.FarmhandDeleteRequest) (*registry.Job, error)
}

type kickPlayerRequest struct {
	UniqueMultiplayerID string `json:"uniqueMultiplayerId"`
	Name                string `json:"name"`
}

type approveAuthRequest struct {
	UniqueMultiplayerID string `json:"uniqueMultiplayerId"`
}

type warpPlayerHomeRequest struct {
	UniqueMultiplayerID string `json:"uniqueMultiplayerId"`
	Name                string `json:"name"`
}

type banPlayerRequest struct {
	Name                string `json:"name"`
	UniqueMultiplayerID string `json:"uniqueMultiplayerId"`
}

type deleteFarmhandRequest struct {
	UniqueMultiplayerID string `json:"uniqueMultiplayerId"`
	ExpectedName        string `json:"expectedName"`
	ExpectedSaveID      string `json:"expectedSaveId"`
	Acknowledged        bool   `json:"acknowledged"`
}

// handleFarmhandDelete handles POST /api/instances/:id/players/delete-farmhand.
func (s *server) handleFarmhandDelete(w http.ResponseWriter, r *http.Request, instanceID string) {
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
	deleter, supported := driver.(farmhandDeleter)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持删除存档人物")
		return
	}
	var body deleteFarmhandRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if !body.Acknowledged {
		writeError(w, http.StatusBadRequest, "confirmation_required", "必须确认人物、小屋及其内容会被删除")
		return
	}
	playerID := strings.TrimSpace(body.UniqueMultiplayerID)
	if playerID == "" {
		writeError(w, http.StatusBadRequest, "invalid_player", "缺少玩家联机 ID")
		return
	}
	job, err := deleter.DeleteFarmhand(r.Context(), sj.FarmhandDeleteRequest{Instance: makeRegistryInstance(instance), PlayerID: playerID,
		ExpectedName: body.ExpectedName, ExpectedSave: body.ExpectedSaveID, ActorID: actor.User.ID})
	if err != nil {
		if ce, ok := err.(*sj.CommandError); ok {
			status := http.StatusBadRequest
			switch ce.Code {
			case "server_not_running", "active_save_changed", "operation_in_progress", "farmhand_online", "save_in_progress", "world_not_ready":
				status = http.StatusConflict
			case "not_supported", "farmhand_delete_unsupported":
				status = http.StatusNotImplemented
			}
			writeError(w, status, ce.Code, ce.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "farmhand_delete_failed", sanitizeErrorMsg(err, "创建人物删除任务失败"))
		return
	}
	s.auditLog(r, &actor, "farmhand_delete_requested", "instance", instanceID, auditMetadata("jobId", job.ID,
		"uniqueMultiplayerId", playerID, "name", body.ExpectedName, "saveId", body.ExpectedSaveID))
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": job.ID})
}

// handlePlayerKick handles POST /api/instances/:id/players/kick.
func (s *server) handlePlayerKick(w http.ResponseWriter, r *http.Request, instanceID string) {
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
	kicker, supported := driver.(playerKicker)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持踢出玩家")
		return
	}

	var body kickPlayerRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	uniqueMultiplayerID := strings.TrimSpace(body.UniqueMultiplayerID)
	if uniqueMultiplayerID == "" {
		writeError(w, http.StatusBadRequest, "invalid_player", "缺少玩家联机 ID")
		return
	}

	result, err := kicker.KickPlayer(r.Context(), makeRegistryInstance(instance), uniqueMultiplayerID, body.Name)
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
		writeError(w, http.StatusInternalServerError, "kick_failed", sanitizeErrorMsg(err, "踢出玩家失败"))
		return
	}
	s.recordControlCommandSubmission(r.Context(), actor, instanceID, result, "player", uniqueMultiplayerID, body.Name)
	s.auditLog(r, &actor, "player_kick", "instance", instanceID, auditMetadata("commandId", result.CommandID, "uniqueMultiplayerId", uniqueMultiplayerID, "name", body.Name))
	writeJSON(w, http.StatusOK, result)
}

// handlePlayerWarpHome handles POST /api/instances/:id/players/warp-home.
func (s *server) handlePlayerWarpHome(w http.ResponseWriter, r *http.Request, instanceID string) {
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
	warper, supported := driver.(playerHomeWarper)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持传送玩家回家")
		return
	}

	var body warpPlayerHomeRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	uniqueMultiplayerID := strings.TrimSpace(body.UniqueMultiplayerID)
	if uniqueMultiplayerID == "" {
		writeError(w, http.StatusBadRequest, "invalid_player", "缺少玩家联机 ID")
		return
	}

	result, err := warper.WarpPlayerHome(r.Context(), makeRegistryInstance(instance), uniqueMultiplayerID, body.Name)
	if err != nil {
		if ce, ok := err.(*sj.CommandError); ok {
			status := http.StatusBadRequest
			switch ce.Code {
			case "server_not_running", "warp_home_bridge_unavailable":
				status = http.StatusConflict
			case "not_supported":
				status = http.StatusNotImplemented
			}
			writeError(w, status, ce.Code, ce.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "warp_home_failed", sanitizeErrorMsg(err, "传送玩家回家失败"))
		return
	}
	s.recordControlCommandSubmission(r.Context(), actor, instanceID, result, "player", uniqueMultiplayerID, body.Name)
	s.auditLog(r, &actor, "player_warp_home", "instance", instanceID, auditMetadata("commandId", result.CommandID, "uniqueMultiplayerId", uniqueMultiplayerID, "name", body.Name))
	writeJSON(w, http.StatusOK, result)
}

// handlePlayerApproveAuth handles POST /api/instances/:id/players/approve-auth.
func (s *server) handlePlayerApproveAuth(w http.ResponseWriter, r *http.Request, instanceID string) {
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
	approver, supported := driver.(playerAuthApprover)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持批准认证")
		return
	}

	var body approveAuthRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	uniqueMultiplayerID := strings.TrimSpace(body.UniqueMultiplayerID)
	if uniqueMultiplayerID == "" {
		writeError(w, http.StatusBadRequest, "invalid_player", "缺少玩家联机 ID")
		return
	}

	result, err := approver.ApproveAuth(r.Context(), makeRegistryInstance(instance), uniqueMultiplayerID)
	if err != nil {
		if ce, ok := err.(*sj.CommandError); ok {
			status := http.StatusBadRequest
			switch ce.Code {
			case "server_not_running":
				status = http.StatusConflict
			case "password_bridge_unavailable":
				status = http.StatusConflict
			case "not_supported":
				status = http.StatusNotImplemented
			}
			writeError(w, status, ce.Code, ce.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "approve_auth_failed", sanitizeErrorMsg(err, "批准认证失败"))
		return
	}
	s.recordControlCommandSubmission(r.Context(), actor, instanceID, result, "player", uniqueMultiplayerID, "")
	s.auditLog(r, &actor, "player_approve_auth", "instance", instanceID, auditMetadata("commandId", result.CommandID, "uniqueMultiplayerId", uniqueMultiplayerID))
	writeJSON(w, http.StatusOK, result)
}

// handlePlayerBan handles POST /api/instances/:id/players/ban.
func (s *server) handlePlayerBan(w http.ResponseWriter, r *http.Request, instanceID string) {
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
	banner, supported := driver.(playerBanner)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持封禁玩家")
		return
	}

	var body banPlayerRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "invalid_player", "缺少玩家名字")
		return
	}

	result, err := banner.BanPlayer(r.Context(), makeRegistryInstance(instance), name, body.UniqueMultiplayerID)
	if err != nil {
		if ce, ok := err.(*sj.CommandError); ok {
			status := http.StatusBadRequest
			switch ce.Code {
			case "server_not_running", "host_unknown", "junimo_api_unavailable":
				status = http.StatusConflict
			case "not_supported":
				status = http.StatusNotImplemented
			}
			writeError(w, status, ce.Code, ce.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "ban_failed", sanitizeErrorMsg(err, "封禁玩家失败"))
		return
	}
	s.recordControlCommandSubmission(r.Context(), actor, instanceID, result, "player", body.UniqueMultiplayerID, name)
	s.auditLog(r, &actor, "player_ban", "instance", instanceID, auditMetadata("commandId", result.CommandID, "name", name, "uniqueMultiplayerId", body.UniqueMultiplayerID))
	writeJSON(w, http.StatusOK, result)
}

// handlePlayersList handles GET /api/instances/:id/players.
func (s *server) handlePlayersList(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
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
	lister, supported := driver.(playerLister)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持玩家状态读取")
		return
	}
	result, err := lister.ListPlayers(r.Context(), makeRegistryInstance(instance))
	if err != nil {
		if ce, ok := err.(*sj.CommandError); ok {
			status := http.StatusBadRequest
			if ce.Code == "server_not_running" {
				status = http.StatusConflict
			}
			writeError(w, status, ce.Code, ce.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "players_failed", sanitizeErrorMsg(err, "读取在线玩家失败"))
		return
	}
	writeJSON(w, http.StatusOK, result)
}
