package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/auth"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type instancesResponse struct {
	Instances []instanceResponse `json:"instances"`
}

type instanceResponse struct {
	IsDefault    bool    `json:"isDefault"`
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
	InstanceID             string                     `json:"instanceId"`
	DriverID               string                     `json:"driverId"`
	Name                   string                     `json:"name"`
	State                  string                     `json:"state"`
	StateMessage           *string                    `json:"stateMessage"`
	DriverPhase            string                     `json:"driverPhase"`
	UpdatedAt              string                     `json:"updatedAt"`
	SteamInviteEnabled     bool                       `json:"steamInviteEnabled"`
	SteamInviteAuthState   string                     `json:"steamInviteAuthState"`
	SteamAuthLoggedIn      bool                       `json:"steamAuthLoggedIn"`
	SteamAuthReady         bool                       `json:"steamAuthReady"`
	InviteCode             string                     `json:"inviteCode,omitempty"`
	UIStatus               string                     `json:"uiStatus"`
	UIStatusUpdatedAt      string                     `json:"uiStatusUpdatedAt"`
	StatusSource           controlStatusSnapshot      `json:"statusSource"`
	PlayersSource          controlPlayersSnapshot     `json:"playersSource"`
	RuntimeDiagnostic      runtimeDiagnostic          `json:"runtimeDiagnostic"`
	InstallationDiagnostic *sj.InstallationDiagnostic `json:"installationDiagnostic,omitempty"`
}

type composeExecPipeDocker interface {
	ComposeExecPipe(ctx context.Context, dir, service, stdinData string, args ...string) (paneldocker.CommandResult, error)
}

type instanceStatusResponse struct {
	Instance instanceResponse       `json:"instance"`
	Status   *registry.ServerStatus `json:"status"`
}

type createInstanceRequest struct {
	Name   string `json:"name"`
	GameID string `json:"gameId"`
}

type createInstanceResponse struct {
	Instance instanceResponse                 `json:"instance"`
	GameID   string                           `json:"gameId"`
	Ports    registry.InstanceProvisionResult `json:"ports"`
}

func (s *server) handleInstances(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.handleInstanceCreate(w, r)
		return
	}
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

func (s *server) handleInstanceCreate(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	var request createInstanceRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	request.GameID = strings.TrimSpace(request.GameID)
	if request.Name == "" || len([]rune(request.Name)) > 40 || strings.IndexFunc(request.Name, unicode.IsControl) >= 0 {
		writeError(w, http.StatusBadRequest, "invalid_instance_name", "世界名称需为 1–40 个字符且不能包含控制字符")
		return
	}
	if request.GameID != "stardew" {
		writeError(w, http.StatusBadRequest, "game_not_supported", "当前阶段只支持创建星露谷世界")
		return
	}
	driverID := sj.DriverID

	driver, err := s.registry.Get(driverID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "driver_not_registered", "请求的游戏 driver 未注册")
		return
	}
	provisioner, ok := driver.(registry.InstanceProvisioner)
	if !ok {
		writeError(w, http.StatusNotImplemented, "instance_creation_unavailable", "该游戏尚未提供安全的实例创建能力")
		return
	}

	s.instanceCreateMu.Lock()
	defer s.instanceCreateMu.Unlock()
	template, err := s.store.GetInstance(r.Context(), s.config.DefaultInstanceID)
	if err != nil || template.DriverID != driverID {
		writeError(w, http.StatusConflict, "game_installation_required", "请先完成该游戏的全局安装")
		return
	}
	existing, err := s.store.ListInstances(r.Context())
	if err != nil {
		s.logger.Error("failed to list instances before provisioning", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	instancesRoot := filepath.Clean(filepath.Join(s.config.DataDir, "instances"))
	var reserved storage.Instance
	for attempt := 0; attempt < 32; attempt++ {
		ordinal, err := s.store.AllocateInstanceOrdinal(r.Context(), request.GameID, driverID, request.GameID)
		if err != nil {
			s.logger.Error("failed to allocate instance ordinal", "game", request.GameID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
			return
		}
		instanceID := request.GameID + "-" + strconv.Itoa(ordinal)
		dataDir := filepath.Clean(filepath.Join(instancesRoot, instanceID))
		rel, err := filepath.Rel(instancesRoot, dataDir)
		if err != nil || rel != instanceID || filepath.IsAbs(rel) || strings.Contains(rel, string(filepath.Separator)) {
			s.logger.Error("generated instance ID could not map to a safe directory", "instance", instanceID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
			return
		}
		if _, err := os.Lstat(dataDir); err == nil {
			s.logger.Warn("skipping generated instance ID with existing resources", "instance", instanceID)
			continue
		} else if !os.IsNotExist(err) {
			s.logger.Error("failed to inspect generated instance directory", "instance", instanceID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
			return
		}

		reserved, err = s.store.CreateInstance(r.Context(), storage.CreateInstanceParams{
			ID:            instanceID,
			DriverID:      driverID,
			Name:          request.Name,
			DataDir:       dataDir,
			State:         storage.InstanceStateAdminCreated,
			StateMessage:  "正在创建独立世界实例。",
			DriverPhase:   "instance_provisioning",
			DriverPayload: "{}",
		})
		if err == nil {
			break
		}
		if errors.Is(err, storage.ErrConflict) {
			continue
		}
		s.logger.Error("failed to reserve generated instance", "instance", instanceID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	if reserved.ID == "" {
		s.logger.Error("failed to find an unused generated instance ID", "game", request.GameID)
		writeError(w, http.StatusInternalServerError, "instance_id_allocation_failed", "暂时无法分配新的世界编号，请稍后重试")
		return
	}
	target := makeRegistryInstance(reserved)
	registryExisting := make([]registry.Instance, 0, len(existing)+1)
	for _, instance := range existing {
		registryExisting = append(registryExisting, makeRegistryInstance(instance))
	}
	registryExisting = append(registryExisting, target)
	result, provisionErr := provisioner.ProvisionInstance(r.Context(), registry.InstanceProvisionRequest{
		Template: makeRegistryInstance(template), Target: target, Existing: registryExisting, ActorID: session.User.ID,
	})
	if provisionErr != nil {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		cleanupErr := provisioner.CleanupProvisionedInstance(cleanupCtx, target)
		cancel()
		if cleanupErr == nil {
			_, deleteErr := s.store.DeleteInstanceIfPhase(context.Background(), target.ID, "instance_provisioning")
			if deleteErr != nil {
				s.logger.Error("failed to release instance reservation after provisioning failure", "instance", target.ID, "error", deleteErr)
			}
		} else {
			s.logger.Error("failed to clean provisioned instance", "instance", target.ID, "error", cleanupErr)
			_, _ = s.store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
				ID: target.ID, State: storage.InstanceStateError,
				StateMessage: "实例创建失败且资源清理未完成，请查看诊断后人工处理。",
				DriverPhase:  "instance_provision_cleanup_failed", DriverPayload: "{}",
			})
		}
		s.logger.Warn("instance provisioning failed", "instance", target.ID, "game", request.GameID, "error", provisionErr)
		if cleanupErr != nil {
			writeError(w, http.StatusInternalServerError, "instance_provision_cleanup_failed", "世界创建失败，部分资源尚未回收；请检查资源占用后在世界列表重试删除。")
			return
		}
		switch {
		case errors.Is(provisionErr, sj.ErrInstanceProvisionTemplateRequired):
			writeError(w, http.StatusConflict, "game_installation_required", "请先完成该游戏的全局安装")
		case errors.Is(provisionErr, sj.ErrInstanceProvisionTemplateBusy):
			writeError(w, http.StatusConflict, "game_installation_busy", "游戏安装环境正在维护，请稍后再创建世界")
		case errors.Is(provisionErr, sj.ErrInstanceProvisionDockerUnsupported):
			writeError(w, http.StatusNotImplemented, "instance_creation_unavailable", "当前环境不支持安全复制游戏运行文件")
		default:
			writeError(w, http.StatusInternalServerError, "instance_provision_failed", "世界实例创建失败，已回收本次创建的资源")
		}
		return
	}
	created, err := s.store.GetInstance(r.Context(), target.ID)
	if err != nil {
		s.logger.Error("failed to load newly provisioned instance", "instance", target.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "实例已创建，但读取结果失败")
		return
	}
	s.auditLog(r, &session, "instance.create", "instance", created.ID, auditMetadata("driverId", created.DriverID, "gameId", request.GameID))
	writeJSON(w, http.StatusCreated, createInstanceResponse{
		Instance: s.makeInstanceResponse(created), GameID: request.GameID, Ports: result,
	})
}

func (s *server) handleInstanceByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/instances/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	instanceID := parts[0]
	if len(parts) == 1 && r.Method == http.MethodDelete {
		s.handleInstanceDelete(w, r, instanceID)
		return
	}
	operationLock := s.instanceOperationLock(instanceID)
	operationLock.RLock()
	defer operationLock.RUnlock()
	if _, err := s.store.GetInstanceProvision(r.Context(), instanceID); err == nil {
		if _, ok := s.requireAuth(w, r); !ok {
			return
		}
		writeError(w, http.StatusConflict, "instance_provisioning", "世界创建或恢复尚未完成，请等待或重试删除。")
		return
	} else if !errors.Is(err, storage.ErrNotFound) {
		writeError(w, 500, "internal_error", "无法读取创建状态")
		return
	}
	if _, err := s.store.GetInstanceDeletion(r.Context(), instanceID); err == nil {
		if _, ok := s.requireAuth(w, r); !ok {
			return
		}
		writeError(w, http.StatusConflict, "instance_deleting", "世界删除尚未完成，请返回世界列表重试彻底删除。")
		return
	} else if !errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal_error", "无法读取世界删除状态")
		return
	}
	if len(parts) == 1 && r.Method == http.MethodPatch {
		s.handleInstanceRename(w, r, instanceID)
		return
	}
	if importMutexEndpoint(r.Method, parts) {
		session, authenticated := s.requireAuth(w, r)
		if !authenticated {
			return
		}
		instance, err := s.store.GetInstance(r.Context(), instanceID)
		if err == nil {
			// A terminal failed import can leave a durable journal until Web has
			// reconciled its exact job/token owner. Admin mutations are a normal
			// recovery entry point too; converge the strictly safe case before the
			// generic mutex turns the durable marker into a false busy response.
			if session.User.Role == auth.RoleAdmin {
				if _, recoverErr := s.autoRecoverSafeFailedSaveImport(r.Context(), instance); recoverErr != nil {
					writeSaveImportSubmitError(w, recoverErr)
					return
				}
			}
			busy, busyErr := sj.HasUnfinishedImportTransaction(instance.DataDir)
			if busyErr != nil {
				writeError(w, http.StatusInternalServerError, "import_recovery_check_failed", "failed to inspect import recovery state")
				return
			}
			if busy {
				writeError(w, http.StatusConflict, sj.ImportErrorBusy, "a save import transaction is active or requires recovery")
				return
			}
		}
	}
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
	if len(parts) == 2 && parts[1] == "junimo-update" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceJunimoUpdate(w, r, instanceID)
		return
	}
	if len(parts) == 2 && parts[1] == "smapi-update" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceSMAPIUpdate(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "smapi-update" && parts[2] == "dry-run" {
		s.handleInstanceSMAPIUpdateDryRun(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "smapi-update" && parts[2] == "apply" {
		s.handleInstanceSMAPIUpdateApply(w, r, instanceID)
		return
	}
	if len(parts) == 2 && parts[1] == "runtime-components" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceRuntimeComponents(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "runtime-components" && parts[2] == "dry-run" {
		s.handleInstanceRuntimeComponentsPreflight(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "junimo-update" && parts[2] == "dry-run" {
		s.handleInstanceJunimoUpdateDryRun(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "junimo-update" && parts[2] == "apply" {
		s.handleInstanceJunimoUpdateApply(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "junimo-update" && parts[2] == "repair" {
		s.handleInstanceJunimoUpdateRepair(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "junimo-update" && parts[2] == "repair-config" {
		s.handleInstanceJunimoUpdateConfigRepair(w, r, instanceID)
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
	if len(parts) == 2 && parts[1] == "metrics" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceMetrics(w, r, instanceID)
		return
	}
	if len(parts) == 2 && parts[1] == "players" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handlePlayersList(w, r, instanceID)
		return
	}
	// GET /api/instances/:id/players/:uniqueMultiplayerId/mods
	if len(parts) == 4 && parts[1] == "players" && parts[3] == "mods" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handlePlayerModDetails(w, r, instanceID, parts[2])
		return
	}
	// POST /api/instances/:id/players/kick
	if len(parts) == 3 && parts[1] == "players" && parts[2] == "kick" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handlePlayerKick(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/players/warp-home
	if len(parts) == 3 && parts[1] == "players" && parts[2] == "warp-home" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handlePlayerWarpHome(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/players/approve-auth
	if len(parts) == 3 && parts[1] == "players" && parts[2] == "approve-auth" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handlePlayerApproveAuth(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/players/ban
	if len(parts) == 3 && parts[1] == "players" && parts[2] == "ban" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handlePlayerBan(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/players/delete-farmhand
	if len(parts) == 3 && parts[1] == "players" && parts[2] == "delete-farmhand" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleFarmhandDelete(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/festival/event
	if len(parts) == 3 && parts[1] == "festival" && parts[2] == "event" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleFestivalEventTrigger(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/joja/enable
	if len(parts) == 3 && parts[1] == "joja" && parts[2] == "enable" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleJojaRouteEnable(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/saves/save-now
	if len(parts) == 3 && parts[1] == "saves" && parts[2] == "save-now" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleGameSaveRequest(w, r, instanceID)
		return
	}
	// GET /api/instances/:id/password-status
	if len(parts) == 2 && parts[1] == "password-status" {
		s.handleInstancePasswordStatus(w, r, instanceID)
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
	if len(parts) == 2 && parts[1] == "install-options" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceInstallOptions(w, r, instanceID)
		return
	}
	if len(parts) == 2 && parts[1] == "prepare" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstancePrepare(w, r, instanceID)
		return
	}
	if len(parts) == 2 && parts[1] == "install" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceInstall(w, r, instanceID)
		return
	}
	if len(parts) == 2 && parts[1] == "steam-credentials" {
		if r.Method != http.MethodPut {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceSteamCredentials(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "steam-guard" && parts[2] == "input" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceSteamGuardInput(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "steam-auth" && parts[2] == "login" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceSteamAuthLogin(w, r, instanceID)
		return
	}
	if len(parts) == 2 && parts[1] == "start" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceStart(w, r, instanceID)
		return
	}
	if len(parts) == 2 && parts[1] == "stop" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceStop(w, r, instanceID)
		return
	}
	if len(parts) == 2 && parts[1] == "restart" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceRestart(w, r, instanceID)
		return
	}
	if len(parts) == 2 && parts[1] == "restart-schedule" {
		s.handleRestartSchedule(w, r, instanceID)
		return
	}
	if len(parts) == 2 && parts[1] == "invite-code" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceInviteCode(w, r, instanceID)
		return
	}
	if len(parts) == 2 && parts[1] == "public-ip" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstancePublicIP(w, r, instanceID)
		return
	}
	if len(parts) == 2 && parts[1] == "rendering" {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleInstanceRendering(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "config" && parts[2] == "vnc-port" {
		s.handleInstanceVNCConfig(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "config" && parts[2] == "server-password" {
		s.handleInstanceServerPassword(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "config" && parts[2] == "player-auth" {
		s.handleInstancePlayerAuthConfig(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "config" && parts[2] == "server-runtime-settings" {
		s.handleInstanceServerRuntimeSettings(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "config" && parts[2] == "game-language" {
		s.handleInstanceGameLanguage(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "saves" && parts[2] == "preflight" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleSavesPreflight(w, r, instanceID)
		return
	}
	// GET /api/instances/:id/saves/farm-types
	if len(parts) == 3 && parts[1] == "saves" && parts[2] == "farm-types" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleFarmTypeCatalog(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/saves/farm-types/prepare
	if len(parts) == 4 && parts[1] == "saves" && parts[2] == "farm-types" && parts[3] == "prepare" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handlePrepareFarmType(w, r, instanceID)
		return
	}
	// GET /api/instances/:id/saves/farm-types/:farmId/icon
	if len(parts) == 5 && parts[1] == "saves" && parts[2] == "farm-types" && parts[4] == "icon" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleFarmTypeIcon(w, r, instanceID, parts[3])
		return
	}
	// GET /api/instances/:id/saves
	if len(parts) == 2 && parts[1] == "saves" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleSavesList(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/saves/select
	if len(parts) == 3 && parts[1] == "saves" && parts[2] == "select" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleSaveSelect(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/saves/select-and-start
	if len(parts) == 3 && parts[1] == "saves" && parts[2] == "select-and-start" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleSaveSelectAndStart(w, r, instanceID)
		return
	}
	// DELETE /api/instances/:id/saves/:name
	if len(parts) == 3 && parts[1] == "saves" && r.Method == http.MethodDelete {
		s.handleSaveDelete(w, r, instanceID, parts[2])
		return
	}
	// POST /api/instances/:id/saves/:name/export
	if len(parts) == 4 && parts[1] == "saves" && parts[3] == "export" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleSaveExport(w, r, instanceID, parts[2])
		return
	}
	// POST /api/instances/:id/saves/:name/backup
	if len(parts) == 4 && parts[1] == "saves" && parts[3] == "backup" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleSaveBackupCreate(w, r, instanceID, parts[2])
		return
	}
	// GET /api/instances/:id/saves/backups
	if len(parts) == 3 && parts[1] == "saves" && parts[2] == "backups" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleSavesBackupsList(w, r, instanceID)
		return
	}
	// GET/PUT /api/instances/:id/saves/backups/policy
	if len(parts) == 4 && parts[1] == "saves" && parts[2] == "backups" && parts[3] == "policy" {
		s.handleSavesBackupPolicy(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/saves/backups/restore
	if len(parts) == 4 && parts[1] == "saves" && parts[2] == "backups" && parts[3] == "restore" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleSavesBackupRestore(w, r, instanceID)
		return
	}
	// DELETE /api/instances/:id/saves/backups/:backupName
	if len(parts) == 4 && parts[1] == "saves" && parts[2] == "backups" && r.Method == http.MethodDelete {
		s.handleSavesBackupDelete(w, r, instanceID, parts[3])
		return
	}
	if len(parts) == 3 && parts[1] == "saves" && parts[2] == "custom-new-game" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleSavesCustomNewGame(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "saves" && parts[2] == "upload-preview" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleSavesUploadPreview(w, r, instanceID)
		return
	}
	if len(parts) == 3 && parts[1] == "saves" && parts[2] == "upload-commit-and-start" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleSavesUploadCommitAndStart(w, r, instanceID)
		return
	}
	// GET /api/instances/:id/mod-updates
	if len(parts) == 2 && parts[1] == "mod-updates" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleModUpdatesGet(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/mod-updates/check
	if len(parts) == 3 && parts[1] == "mod-updates" && parts[2] == "check" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleModUpdatesCheck(w, r, instanceID)
		return
	}
	// GET /api/instances/:id/mods
	if len(parts) == 2 && parts[1] == "mods" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleModsList(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/mods/upload
	if len(parts) == 3 && parts[1] == "mods" && parts[2] == "upload" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleModsUpload(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/mods/export
	if len(parts) == 3 && parts[1] == "mods" && parts[2] == "export" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleModsExport(w, r, instanceID)
		return
	}
	// GET /api/instances/:id/mods/nexus/search
	if len(parts) == 4 && parts[1] == "mods" && parts[2] == "nexus" && parts[3] == "search" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleModNexusSearch(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/mods/nexus/install
	if len(parts) == 4 && parts[1] == "mods" && parts[2] == "nexus" && parts[3] == "install" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleModNexusInstall(w, r, instanceID)
		return
	}
	// GET /api/instances/:id/mods/nexus/extension/download
	if len(parts) == 5 && parts[1] == "mods" && parts[2] == "nexus" && parts[3] == "extension" && parts[4] == "download" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleModNexusExtensionDownload(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/mods/remote/install
	if len(parts) == 4 && parts[1] == "mods" && parts[2] == "remote" && parts[3] == "install" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleModRemoteInstall(w, r, instanceID)
		return
	}
	// GET /api/instances/:id/mods/sync-plan
	if len(parts) == 3 && parts[1] == "mods" && parts[2] == "sync-plan" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleModSyncPlan(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/mods/sync-pack/export
	if len(parts) == 4 && parts[1] == "mods" && parts[2] == "sync-pack" && parts[3] == "export" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleModSyncPackExport(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/mods/sync-pack/export-update
	if len(parts) == 4 && parts[1] == "mods" && parts[2] == "sync-pack" && parts[3] == "export-update" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleModSyncUpdatePackExport(w, r, instanceID)
		return
	}
	// PUT /api/instances/:id/mods/:modId/sync-classification
	if len(parts) == 4 && parts[1] == "mods" && parts[3] == "sync-classification" {
		if r.Method != http.MethodPut {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleModSyncClassificationUpdate(w, r, instanceID, parts[2])
		return
	}
	// PUT /api/instances/:id/mods/enabled
	if len(parts) == 3 && parts[1] == "mods" && parts[2] == "enabled" {
		if r.Method != http.MethodPut {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleAllModsEnabledUpdate(w, r, instanceID)
		return
	}
	// PUT /api/instances/:id/mods/:modId/enabled
	if len(parts) == 4 && parts[1] == "mods" && parts[3] == "enabled" {
		if r.Method != http.MethodPut {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleModEnabledUpdate(w, r, instanceID, parts[2])
		return
	}
	// DELETE /api/instances/:id/mods/:modId
	if len(parts) == 3 && parts[1] == "mods" && r.Method == http.MethodDelete {
		s.handleModDelete(w, r, instanceID, parts[2])
		return
	}
	// GET /api/instances/:id/commands
	if len(parts) == 2 && parts[1] == "control-commands" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleControlCommandHistory(w, r, instanceID)
		return
	}
	// GET /api/instances/:id/commands
	if len(parts) == 3 && parts[1] == "commands" && parts[2] != "run" && parts[2] != "say" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleCommandOutcome(w, r, instanceID, parts[2])
		return
	}
	if len(parts) == 2 && parts[1] == "commands" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleCommandsList(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/commands/run
	if len(parts) == 3 && parts[1] == "commands" && parts[2] == "run" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleCommandRun(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/commands/say
	if len(parts) == 3 && parts[1] == "commands" && parts[2] == "say" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleCommandSay(w, r, instanceID)
		return
	}
	// POST /api/instances/:id/support-bundle
	if len(parts) == 2 && parts[1] == "support-bundle" {
		s.handleSupportBundle(w, r, instanceID)
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "resource not found")
}

func importMutexEndpoint(method string, parts []string) bool {
	if method == http.MethodGet || len(parts) < 2 {
		return false
	}
	area := parts[1]
	if area == "start" || area == "stop" || area == "restart" || area == "install" || area == "junimo-update" || area == "smapi-update" {
		return true
	}
	if area == "mods" {
		return true
	}
	if area != "saves" {
		return false
	}
	if len(parts) >= 3 && (parts[2] == "upload-preview" || parts[2] == "upload-commit-and-start") {
		return false
	}
	return true
}

func (s *server) handleInstanceDetail(w http.ResponseWriter, r *http.Request, instanceID string) {
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
	instance, ok = s.reconcileInstanceState(w, r, instance)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.makeInstanceStateResponse(r.Context(), instance))
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

func (s *server) reconcileInstanceState(w http.ResponseWriter, r *http.Request, instance storage.Instance) (storage.Instance, bool) {
	type stateReconciler interface {
		ReconcileState(ctx context.Context, instance storage.Instance) (storage.Instance, error)
	}
	driver, err := s.registry.Get(instance.DriverID)
	if err != nil {
		return instance, true
	}
	reconciler, ok := driver.(stateReconciler)
	if !ok {
		return instance, true
	}
	updated, err := reconciler.ReconcileState(r.Context(), instance)
	if err != nil {
		s.logger.Warn("failed to reconcile instance state", "instance", instance.ID, "driver", instance.DriverID, "error", err)
		writeError(w, http.StatusInternalServerError, "state_reconcile_failed", "实例状态校验失败")
		return storage.Instance{}, false
	}
	return updated, true
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

func (s *server) handleInstanceRename(w http.ResponseWriter, r *http.Request, instanceID string) {
	session, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	if _, ok := s.loadInstance(w, r, instanceID); !ok {
		return
	}
	var request struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	name := strings.TrimSpace(request.Name)
	if name == "" || len([]rune(name)) > 40 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		writeError(w, http.StatusBadRequest, "invalid_instance_name", "世界名称需为 1–40 个字符且不能包含控制字符")
		return
	}
	instance, err := s.store.RenameInstance(r.Context(), instanceID, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rename_failed", "世界名称保存失败，请重试")
		return
	}
	s.auditLog(r, &session, "instance.rename", "instance", instanceID, "{}")
	writeJSON(w, http.StatusOK, map[string]any{"instance": s.makeInstanceResponse(instance)})
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
		IsDefault:    instance.ID == s.config.DefaultInstanceID,
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

func (s *server) makeInstanceStateResponse(ctx context.Context, instance storage.Instance) instanceStateResponse {
	uiStatus, uiStatusUpdatedAt := s.resolveInstanceUIStatus(ctx, instance)
	controlDir := filepath.Join(instance.DataDir, ".local-container", "control")
	var statusSource controlStatusSnapshot
	var playersSource controlPlayersSnapshot
	readControlJSON(filepath.Join(controlDir, "status.json"), &statusSource)
	readControlJSON(filepath.Join(controlDir, "players.json"), &playersSource)
	runtimeDiagnostic := buildRuntimeDiagnostic(instance, statusSource, playersSource)
	runtimeDiagnostic.CommandProtocol = s.commandProtocolDiagnostics(ctx, instance)
	var installationDiagnostic *sj.InstallationDiagnostic
	if driver, err := s.registry.Get(instance.DriverID); err == nil {
		type installationDiagnosticProvider interface {
			InstallationDiagnostic(context.Context, registry.Instance) sj.InstallationDiagnostic
		}
		if provider, ok := driver.(installationDiagnosticProvider); ok {
			diagnostic := provider.InstallationDiagnostic(ctx, makeRegistryInstance(instance))
			installationDiagnostic = &diagnostic
			runtimeDiagnostic.InstalledControlVersion = runtimeDiagnostic.ControlModVersion
			runtimeDiagnostic.InstalledControlMatches = diagnostic.Control.Static == "match"
			runtimeDiagnostic.RuntimeControlState = diagnostic.Control.Runtime
			runtimeDiagnostic.RuntimeControlVersion = diagnostic.Control.ObservedVersion
			runtimeDiagnostic.RuntimeControlMatches = diagnostic.Control.Runtime == "match"
		}
	}
	steamInviteEnabled := sjconfig.SteamInviteEnabled(instance.DataDir)
	inviteCode := visibleInviteCode(instance, steamInviteEnabled)
	return instanceStateResponse{
		InstanceID:             instance.ID,
		DriverID:               instance.DriverID,
		Name:                   instance.Name,
		State:                  instance.State,
		StateMessage:           nullableString(instance.StateMessage),
		DriverPhase:            instance.DriverPhase,
		UpdatedAt:              instance.UpdatedAt,
		SteamInviteEnabled:     steamInviteEnabled,
		SteamInviteAuthState:   sjconfig.SteamInviteAuthState(instance.DataDir),
		SteamAuthLoggedIn:      sjconfig.SteamAuthLoggedIn(instance.DataDir),
		SteamAuthReady:         s.probeSteamAuthReady(ctx, instance),
		InviteCode:             inviteCode,
		UIStatus:               uiStatus,
		UIStatusUpdatedAt:      uiStatusUpdatedAt,
		StatusSource:           statusSource,
		PlayersSource:          playersSource,
		RuntimeDiagnostic:      runtimeDiagnostic,
		InstallationDiagnostic: installationDiagnostic,
	}
}

func visibleInviteCode(instance storage.Instance, steamInviteEnabled bool) string {
	if !steamInviteEnabled || instance.State != storage.InstanceStateRunning {
		return ""
	}
	return inviteCodeFromDriverPayload(instance.DriverPayload)
}

func inviteCodeFromDriverPayload(payload string) string {
	if strings.TrimSpace(payload) == "" {
		return ""
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return ""
	}
	code, _ := parsed["invite_code"].(string)
	return strings.TrimSpace(code)
}

func (s *server) probeSteamAuthReady(ctx context.Context, instance storage.Instance) bool {
	if instance.DriverID != "stardew_junimo" {
		return false
	}
	if !sjconfig.SteamInviteEnabled(instance.DataDir) {
		return false
	}
	execDocker, ok := s.docker.(composeExecPipeDocker)
	if !ok {
		return false
	}
	if instance.State != storage.InstanceStateRunning && instance.State != storage.InstanceStateStarting {
		return false
	}

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	result, err := execDocker.ComposeExecPipe(probeCtx, instance.DataDir, "server", "",
		"sh", "-lc", "printf 'GET /steam/ready HTTP/1.1\\r\\nHost: steam-auth\\r\\nConnection: close\\r\\n\\r\\n' | nc -w 3 steam-auth 3001")
	if err != nil || result.ExitCode != 0 {
		return false
	}
	return strings.Contains(result.Stdout, " 200 ") || strings.HasPrefix(result.Stdout, "HTTP/1.1 200")
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
