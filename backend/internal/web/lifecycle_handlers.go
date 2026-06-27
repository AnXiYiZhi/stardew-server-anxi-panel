package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	uploadTokenTTL    = 10 * time.Minute
	maxUploadFormSize = 110 * 1024 * 1024 // multipart memory: ZIP limit is 100 MB
)

// ── Pending upload token store ─────────────────────────────────────────────────

type pendingUpload struct {
	InstanceID string
	TempDir    string
	SaveName   string
	Preview    registry.SaveInfo
	ExpiresAt  time.Time
}

type pendingUploadStore struct {
	mu      sync.Mutex
	entries map[string]*pendingUpload
}

func newPendingUploadStore() *pendingUploadStore {
	return &pendingUploadStore{entries: make(map[string]*pendingUpload)}
}

func (s *pendingUploadStore) put(instanceID, tempDir, saveName string, preview registry.SaveInfo) string {
	token := newToken()
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.entries {
		if now.After(v.ExpiresAt) {
			_ = os.RemoveAll(v.TempDir)
			delete(s.entries, k)
		}
	}
	s.entries[token] = &pendingUpload{
		InstanceID: instanceID,
		TempDir:    tempDir,
		SaveName:   saveName,
		Preview:    preview,
		ExpiresAt:  now.Add(uploadTokenTTL),
	}
	return token
}

func (s *pendingUploadStore) claim(token, instanceID string) (*pendingUpload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[token]
	if !ok {
		return nil, fmt.Errorf("上传令牌无效或已过期")
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(s.entries, token)
		_ = os.RemoveAll(entry.TempDir)
		return nil, fmt.Errorf("上传令牌已过期")
	}
	if entry.InstanceID != instanceID {
		return nil, fmt.Errorf("上传令牌与实例不匹配")
	}
	delete(s.entries, token)
	return entry, nil
}

func (s *pendingUploadStore) cancel(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry, ok := s.entries[token]; ok {
		delete(s.entries, token)
		_ = os.RemoveAll(entry.TempDir)
	}
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// handleSavesPreflight handles GET /api/instances/:id/saves/preflight.
func (s *server) handleSavesPreflight(w http.ResponseWriter, r *http.Request, instanceID string) {
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
	saves, err := driver.ListSaves(r.Context(), makeRegistryInstance(instance))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_saves_failed", "读取存档列表失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, registry.PreflightResult{
		HasSaves:          len(saves) > 0,
		Saves:             saves,
		TemplateAvailable: sj.HasTemplates(instance.DataDir),
	})
}

// handleSavesCustomNewGame handles POST /api/instances/:id/saves/custom-new-game.
func (s *server) handleSavesCustomNewGame(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	instance, ok = s.ensureInstanceNotRunning(w, r, instance)
	if !ok {
		return
	}

	var cfg registry.NewGameConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体解析失败")
		return
	}

	if err := sj.WriteServerSettings(instance.DataDir, cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_config", err.Error())
		return
	}

	if err := s.advanceToReadyToStart(r, instance); err != nil {
		s.logger.Warn("advance state after new-game config", "instance", instanceID, "error", err)
	}

	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	// Re-load instance after state advance so Start sees updated state.
	instance, _ = s.loadInstance(w, r, instanceID)
	job, err := driver.Start(r.Context(), registry.StartRequest{
		Instance: makeRegistryInstance(instance),
		ActorID:  actor.User.ID,
		// NewGame signals the lifecycle job to send "settings newgame --confirm"
		// via attach-cli so JunimoServer creates a fresh save with the new config.
		NewGame: true,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "start_failed", "服务器启动失败: "+err.Error())
		return
	}

	s.logger.Info("new-game + start", "instance", instanceID, "job", job.ID)
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": job.ID})
}

// handleSavesUploadPreview handles POST /api/instances/:id/saves/upload-preview.
func (s *server) handleSavesUploadPreview(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	_, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}

	if err := r.ParseMultipartForm(maxUploadFormSize); err != nil {
		writeError(w, http.StatusBadRequest, "parse_form_failed", "解析上传表单失败（文件可能超过大小限制）")
		return
	}

	file, header, err := r.FormFile("save")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_file", "未找到上传字段 'save'")
		return
	}
	defer func() { _ = file.Close() }()

	tmp, err := os.CreateTemp("", "stardew-upload-*.zip")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "创建临时文件失败")
		return
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := io.Copy(tmp, file); err != nil {
		_ = tmp.Close()
		writeError(w, http.StatusInternalServerError, "write_failed", "写入临时文件失败")
		return
	}
	_ = tmp.Close()

	saveName, preview, tempDir, err := sj.PreviewSaveZip(tmpPath, header.Filename)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_zip", err.Error())
		return
	}

	token := s.pendingUploads.put(instanceID, tempDir, saveName, preview)
	writeJSON(w, http.StatusOK, registry.UploadPreviewResult{
		Token:    token,
		Preview:  preview,
		SaveName: saveName,
	})
}

// handleSavesUploadCommitAndStart handles POST /api/instances/:id/saves/upload-commit-and-start.
func (s *server) handleSavesUploadCommitAndStart(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}

	var body struct {
		Token  string `json:"token"`
		Cancel bool   `json:"cancel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体解析失败")
		return
	}
	if body.Token == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "token 不能为空")
		return
	}

	if body.Cancel {
		s.pendingUploads.cancel(body.Token)
		writeJSON(w, http.StatusOK, map[string]bool{"cancelled": true})
		return
	}

	instance, ok = s.ensureInstanceNotRunning(w, r, instance)
	if !ok {
		return
	}

	entry, err := s.pendingUploads.claim(body.Token, instanceID)
	if err != nil {
		writeError(w, http.StatusConflict, "token_invalid", err.Error())
		return
	}
	defer func() { _ = os.RemoveAll(entry.TempDir) }()

	if err := sj.ImportSaveToVolume(instance.DataDir, entry.TempDir, entry.SaveName); err != nil {
		writeError(w, http.StatusInternalServerError, "import_failed", "导入存档失败: "+err.Error())
		return
	}

	// Tell JunimoServer to load the uploaded save on next start.
	if err := sj.SetActiveSave(instance.DataDir, entry.SaveName); err != nil {
		s.logger.Warn("set active save after upload", "instance", instanceID, "save", entry.SaveName, "error", err)
	}

	if err := s.advanceToReadyToStart(r, instance); err != nil {
		s.logger.Warn("advance state after upload", "instance", instanceID, "error", err)
	}

	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	instance, _ = s.loadInstance(w, r, instanceID)
	job, err := driver.Start(r.Context(), registry.StartRequest{
		Instance: makeRegistryInstance(instance),
		ActorID:  actor.User.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "start_failed", "服务器启动失败: "+err.Error())
		return
	}

	s.logger.Info("upload commit + start", "instance", instanceID, "job", job.ID, "save", entry.SaveName)
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": job.ID, "saveName": entry.SaveName})
}

// handleSavesList handles GET /api/instances/:id/saves.
func (s *server) handleSavesList(w http.ResponseWriter, r *http.Request, instanceID string) {
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
	saves, err := driver.ListSaves(r.Context(), makeRegistryInstance(instance))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_saves_failed", "读取存档列表失败: "+err.Error())
		return
	}
	activeName := sj.GetActiveSaveName(instance.DataDir)
	writeJSON(w, http.StatusOK, registry.SavesListResult{
		Saves:          saves,
		ActiveSaveName: activeName,
	})
}

// handleSaveSelect handles POST /api/instances/:id/saves/select.
func (s *server) handleSaveSelect(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	instance, ok = s.ensureInstanceNotRunning(w, r, instance)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体解析失败")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "存档名称不能为空")
		return
	}
	if err := sj.ValidateSaveExists(instance.DataDir, body.Name); err != nil {
		writeError(w, http.StatusNotFound, "save_not_found", err.Error())
		return
	}
	if err := sj.SetActiveSave(instance.DataDir, body.Name); err != nil {
		writeError(w, http.StatusInternalServerError, "select_failed", "选择存档失败: "+err.Error())
		return
	}
	// Advance state if currently in save_required.
	if instance.State == storage.InstanceStateSaveRequired || instance.State == storage.InstanceStateGameInstalled {
		if err := s.advanceToReadyToStart(r, instance); err != nil {
			s.logger.Warn("advance state after select save", "instance", instanceID, "error", err)
		}
	}
	s.logger.Info("save selected", "instance", instanceID, "save", body.Name)
	writeJSON(w, http.StatusOK, map[string]string{"activeSaveName": body.Name})
}

// handleSaveSelectAndStart handles POST /api/instances/:id/saves/select-and-start.
func (s *server) handleSaveSelectAndStart(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	instance, ok = s.ensureInstanceNotRunning(w, r, instance)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体解析失败")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "存档名称不能为空")
		return
	}
	if err := sj.ValidateSaveExists(instance.DataDir, body.Name); err != nil {
		writeError(w, http.StatusNotFound, "save_not_found", err.Error())
		return
	}
	if err := sj.SetActiveSave(instance.DataDir, body.Name); err != nil {
		writeError(w, http.StatusInternalServerError, "select_failed", "选择存档失败: "+err.Error())
		return
	}
	if err := s.advanceToReadyToStart(r, instance); err != nil {
		s.logger.Warn("advance state after select-and-start", "instance", instanceID, "error", err)
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}
	instance, _ = s.loadInstance(w, r, instanceID)
	job, err := driver.Start(r.Context(), registry.StartRequest{
		Instance: makeRegistryInstance(instance),
		ActorID:  actor.User.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "start_failed", "服务器启动失败: "+err.Error())
		return
	}
	s.logger.Info("select-and-start", "instance", instanceID, "job", job.ID, "save", body.Name)
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": job.ID})
}

// handleSaveDelete handles DELETE /api/instances/:id/saves/:name.
func (s *server) handleSaveDelete(w http.ResponseWriter, r *http.Request, instanceID, saveName string) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	instance, ok = s.ensureInstanceNotRunning(w, r, instance)
	if !ok {
		return
	}
	if err := sj.DeleteSave(instance.DataDir, saveName); err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed", "删除存档失败: "+err.Error())
		return
	}
	s.logger.Info("save deleted", "instance", instanceID, "save", saveName)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleInstanceStart handles POST /api/instances/:id/start.
func (s *server) handleInstanceStart(w http.ResponseWriter, r *http.Request, instanceID string) {
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
	// A plain start resumes Junimo's existing save selection. Never let it fall
	// through to Junimo's default new-game behaviour when the panel has no save.
	// Creating or importing a save must always go through its explicit workflow.
	saves, err := driver.ListSaves(r.Context(), makeRegistryInstance(instance))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_saves_failed", "读取存档列表失败: "+err.Error())
		return
	}
	if len(saves) == 0 {
		writeError(w, http.StatusConflict, "save_required", "没有可用存档，请先创建存档并启动或上传存档并启动")
		return
	}

	// Check active save: must have one selected, and it must still exist.
	activeName := sj.GetActiveSaveName(instance.DataDir)
	if activeName == "" {
		writeError(w, http.StatusConflict, "active_save_required", "没有已选择的启动存档，请先创建、上传或选择一个存档")
		return
	}
	if err := sj.ValidateSaveExists(instance.DataDir, activeName); err != nil {
		writeError(w, http.StatusConflict, "active_save_missing", "上次选择的存档不存在，请重新选择存档")
		return
	}

	job, err := driver.Start(r.Context(), registry.StartRequest{
		Instance: makeRegistryInstance(instance),
		ActorID:  actor.User.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "start_failed", "服务器启动失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": job.ID})
}

// handleInstanceStop handles POST /api/instances/:id/stop.
func (s *server) handleInstanceStop(w http.ResponseWriter, r *http.Request, instanceID string) {
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
	if err := driver.Stop(r.Context(), makeRegistryInstance(instance)); err != nil {
		writeError(w, http.StatusInternalServerError, "stop_failed", "服务器停止失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

// handleInstanceRestart handles POST /api/instances/:id/restart.
func (s *server) handleInstanceRestart(w http.ResponseWriter, r *http.Request, instanceID string) {
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
	if err := driver.Restart(r.Context(), makeRegistryInstance(instance)); err != nil {
		writeError(w, http.StatusInternalServerError, "restart_failed", "服务器重启失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

// handleInstanceInviteCode handles GET /api/instances/:id/invite-code.
func (s *server) handleInstanceInviteCode(w http.ResponseWriter, r *http.Request, instanceID string) {
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

	type inviteCodeGetter interface {
		GetInviteCode(ctx context.Context, instance registry.Instance) (string, error)
	}
	getter, supported := driver.(inviteCodeGetter)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持获取邀请码")
		return
	}

	code, err := getter.GetInviteCode(r.Context(), makeRegistryInstance(instance))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invite_code_failed", "获取邀请码失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, registry.InviteCodeResult{InviteCode: code})
}

// ensureInstanceNotRunning reconciles instance state with real Docker state and
// returns false (with an HTTP 409 response written) if the server is running or starting.
// Callers should return immediately when this returns (instance, false).
func (s *server) ensureInstanceNotRunning(w http.ResponseWriter, r *http.Request, instance storage.Instance) (storage.Instance, bool) {
	instance, ok := s.reconcileInstanceState(w, r, instance)
	if !ok {
		return instance, false
	}
	if instance.State == storage.InstanceStateRunning || instance.State == storage.InstanceStateStarting {
		writeError(w, http.StatusConflict, "server_running", "服务器运行中，请先停止服务器再操作存档。")
		return instance, false
	}
	return instance, true
}

// advanceToReadyToStart moves instance state to ready_to_start when applicable.
func (s *server) advanceToReadyToStart(r *http.Request, instance storage.Instance) error {
	if instance.State != storage.InstanceStateGameInstalled &&
		instance.State != storage.InstanceStateSaveRequired {
		return nil
	}
	_, err := s.store.UpdateInstanceState(r.Context(), storage.UpdateInstanceStateParams{
		ID:            instance.ID,
		State:         storage.InstanceStateReadyToStart,
		StateMessage:  "存档已选择，准备启动。",
		DriverPhase:   "ready_to_start",
		DriverPayload: instance.DriverPayload,
	})
	return err
}
