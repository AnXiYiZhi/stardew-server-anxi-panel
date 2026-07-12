package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
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
	InstanceID        string                 `json:"instanceId"`
	DriverID          string                 `json:"driverId"`
	Name              string                 `json:"name"`
	State             string                 `json:"state"`
	StateMessage      *string                `json:"stateMessage"`
	DriverPhase       string                 `json:"driverPhase"`
	UpdatedAt         string                 `json:"updatedAt"`
	SteamAuthLoggedIn bool                   `json:"steamAuthLoggedIn"`
	SteamAuthReady    bool                   `json:"steamAuthReady"`
	InviteCode        string                 `json:"inviteCode,omitempty"`
	UIStatus          string                 `json:"uiStatus"`
	UIStatusUpdatedAt string                 `json:"uiStatusUpdatedAt"`
	StatusSource      controlStatusSnapshot  `json:"statusSource"`
	PlayersSource     controlPlayersSnapshot `json:"playersSource"`
	RuntimeDiagnostic runtimeDiagnostic      `json:"runtimeDiagnostic"`
}

type composeExecPipeDocker interface {
	ComposeExecPipe(ctx context.Context, dir, service, stdinData string, args ...string) (paneldocker.CommandResult, error)
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
	if len(parts) == 3 && parts[1] == "config" && parts[2] == "server-runtime-settings" {
		s.handleInstanceServerRuntimeSettings(w, r, instanceID)
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

func (s *server) makeInstanceStateResponse(ctx context.Context, instance storage.Instance) instanceStateResponse {
	uiStatus, uiStatusUpdatedAt := s.resolveInstanceUIStatus(ctx, instance)
	controlDir := filepath.Join(instance.DataDir, ".local-container", "control")
	var statusSource controlStatusSnapshot
	var playersSource controlPlayersSnapshot
	readControlJSON(filepath.Join(controlDir, "status.json"), &statusSource)
	readControlJSON(filepath.Join(controlDir, "players.json"), &playersSource)
	runtimeDiagnostic := buildRuntimeDiagnostic(instance, statusSource, playersSource)
	runtimeDiagnostic.CommandProtocol = s.commandProtocolDiagnostics(ctx, instance)
	return instanceStateResponse{
		InstanceID:        instance.ID,
		DriverID:          instance.DriverID,
		Name:              instance.Name,
		State:             instance.State,
		StateMessage:      nullableString(instance.StateMessage),
		DriverPhase:       instance.DriverPhase,
		UpdatedAt:         instance.UpdatedAt,
		SteamAuthLoggedIn: sjconfig.SteamAuthLoggedIn(instance.DataDir),
		SteamAuthReady:    s.probeSteamAuthReady(ctx, instance),
		InviteCode:        inviteCodeFromDriverPayload(instance.DriverPayload),
		UIStatus:          uiStatus,
		UIStatusUpdatedAt: uiStatusUpdatedAt,
		StatusSource:      statusSource,
		PlayersSource:     playersSource,
		RuntimeDiagnostic: runtimeDiagnostic,
	}
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
