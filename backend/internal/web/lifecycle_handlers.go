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
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	uploadTokenTTL    = 10 * time.Minute
	maxUploadFormSize = 110 * 1024 * 1024 // multipart memory: save ZIP limit is 100 MB
	maxModFormSize    = 210 * 1024 * 1024 // multipart memory: mod ZIP limit is 200 MB
	maxRequestBody    = 220 * 1024 * 1024 // hard cap on total request body (slightly above largest ZIP limit)
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
		writeError(w, http.StatusInternalServerError, "list_saves_failed", sanitizeErrorMsg(err, "读取存档列表失败"))
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
		writeError(w, http.StatusBadRequest, "invalid_config", sanitizeError(err, "配置参数无效"))
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
		writeError(w, http.StatusInternalServerError, "start_failed", sanitizeErrorMsg(err, "服务器启动失败"))
		return
	}

	s.logger.Info("new-game + start", "instance", instanceID, "job", job.ID)
	s.auditLog(r, &actor, "save_new_game", "instance", instanceID, auditMetadata("jobId", job.ID))
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

	// Hard cap on total request body to prevent disk exhaustion from oversized uploads.
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)

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
		writeError(w, http.StatusBadRequest, "invalid_zip", sanitizeError(err, "存档 ZIP 无效"))
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
		writeError(w, http.StatusConflict, "token_invalid", sanitizeError(err, "上传令牌无效"))
		return
	}
	defer func() { _ = os.RemoveAll(entry.TempDir) }()

	if err := sj.ImportSaveToVolume(instance.DataDir, entry.TempDir, entry.SaveName); err != nil {
		writeError(w, http.StatusInternalServerError, "import_failed", sanitizeErrorMsg(err, "导入存档失败"))
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
		writeError(w, http.StatusInternalServerError, "start_failed", sanitizeErrorMsg(err, "服务器启动失败"))
		return
	}

	s.logger.Info("upload commit + start", "instance", instanceID, "job", job.ID, "save", entry.SaveName)
	s.auditLog(r, &actor, "save_upload_start", "instance", instanceID, auditMetadata("saveName", entry.SaveName, "jobId", job.ID))
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
		writeError(w, http.StatusInternalServerError, "list_saves_failed", sanitizeErrorMsg(err, "读取存档列表失败"))
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
		writeError(w, http.StatusNotFound, "save_not_found", sanitizeError(err, "存档不存在"))
		return
	}
	if err := sj.SetActiveSave(instance.DataDir, body.Name); err != nil {
		writeError(w, http.StatusInternalServerError, "select_failed", sanitizeErrorMsg(err, "选择存档失败"))
		return
	}
	// Advance state if currently in save_required.
	if instance.State == storage.InstanceStateSaveRequired || instance.State == storage.InstanceStateGameInstalled {
		if err := s.advanceToReadyToStart(r, instance); err != nil {
			s.logger.Warn("advance state after select save", "instance", instanceID, "error", err)
		}
	}
	s.logger.Info("save selected", "instance", instanceID, "save", body.Name)
	s.auditLog(r, &actor, "save_select", "instance", instanceID, auditMetadata("saveName", body.Name))
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
		writeError(w, http.StatusNotFound, "save_not_found", sanitizeError(err, "存档不存在"))
		return
	}
	if err := sj.SetActiveSave(instance.DataDir, body.Name); err != nil {
		writeError(w, http.StatusInternalServerError, "select_failed", sanitizeErrorMsg(err, "选择存档失败"))
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
		writeError(w, http.StatusInternalServerError, "start_failed", sanitizeErrorMsg(err, "服务器启动失败"))
		return
	}
	s.logger.Info("select-and-start", "instance", instanceID, "job", job.ID, "save", body.Name)
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": job.ID})
}

// handleSaveDelete handles DELETE /api/instances/:id/saves/:name.
func (s *server) handleSaveDelete(w http.ResponseWriter, r *http.Request, instanceID, saveName string) {
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
	backupPath, err := sj.DeleteSaveWithBackup(instance.DataDir, saveName)
	if err != nil {
		s.logger.Warn("delete save failed (backup failure blocks deletion)", "instance", instanceID, "save", saveName, "error", err)
		writeError(w, http.StatusInternalServerError, "save_delete_failed", sanitizeErrorMsg(err, "删除存档失败"))
		return
	}
	if backupPath != "" {
		s.logger.Info("backup created before delete", "instance", instanceID, "save", saveName, "backup", backupPath)
	}
	s.logger.Info("save deleted", "instance", instanceID, "save", saveName)
	s.auditLog(r, &actor, "save_delete", "instance", instanceID, auditMetadata("saveName", saveName, "backup", backupPath))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true, "backupCreated": backupPath != ""})
}

// handleSaveExport handles POST /api/instances/:id/saves/:name/export.
func (s *server) handleSaveExport(w http.ResponseWriter, r *http.Request, instanceID, saveName string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}

	zipPath, err := sj.ExportSaveZip(instance.DataDir, saveName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export_failed", sanitizeErrorMsg(err, "导出存档失败"))
		return
	}
	defer func() { _ = os.Remove(zipPath) }()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(zipPath)))
	http.ServeFile(w, r, zipPath)
}

// handleSavesBackupsList handles GET /api/instances/:id/saves/backups.
func (s *server) handleSavesBackupsList(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	backups, err := sj.ListBackups(instance.DataDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_backups_failed", sanitizeErrorMsg(err, "读取备份列表失败"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": backups})
}

// handleSavesBackupRestore handles POST /api/instances/:id/saves/backups/restore.
func (s *server) handleSavesBackupRestore(w http.ResponseWriter, r *http.Request, instanceID string) {
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
		BackupName string `json:"backupName"`
		Overwrite  bool   `json:"overwrite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体解析失败")
		return
	}
	if body.BackupName == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "backupName 不能为空")
		return
	}

	saveName, err := sj.RestoreBackup(instance.DataDir, body.BackupName, body.Overwrite)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "已存在") {
			writeError(w, http.StatusConflict, "save_exists", errMsg)
			return
		}
		writeError(w, http.StatusInternalServerError, "restore_failed", sanitizeErrorMsg(err, "恢复备份失败"))
		return
	}

	s.auditLog(r, &actor, "save_restore", "instance", instanceID, auditMetadata("backupName", body.BackupName, "saveName", saveName))
	writeJSON(w, http.StatusOK, map[string]string{"saveName": saveName})
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
		writeError(w, http.StatusInternalServerError, "list_saves_failed", sanitizeErrorMsg(err, "读取存档列表失败"))
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
		writeError(w, http.StatusInternalServerError, "start_failed", sanitizeErrorMsg(err, "服务器启动失败"))
		return
	}
	s.auditLog(r, &actor, "instance_start", "instance", instanceID, auditMetadata("jobId", job.ID))
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": job.ID})
}

// handleInstanceStop handles POST /api/instances/:id/stop.
func (s *server) handleInstanceStop(w http.ResponseWriter, r *http.Request, instanceID string) {
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
	if err := driver.Stop(r.Context(), makeRegistryInstance(instance)); err != nil {
		writeError(w, http.StatusInternalServerError, "stop_failed", sanitizeErrorMsg(err, "服务器停止失败"))
		return
	}
	s.auditLog(r, &actor, "instance_stop", "instance", instanceID, "{}")
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

// handleInstanceRestart handles POST /api/instances/:id/restart.
func (s *server) handleInstanceRestart(w http.ResponseWriter, r *http.Request, instanceID string) {
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
	if err := driver.Restart(r.Context(), makeRegistryInstance(instance)); err != nil {
		writeError(w, http.StatusInternalServerError, "restart_failed", sanitizeErrorMsg(err, "服务器重启失败"))
		return
	}
	s.auditLog(r, &actor, "instance_restart", "instance", instanceID, "{}")
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
		writeError(w, http.StatusInternalServerError, "invite_code_failed", sanitizeErrorMsg(err, "获取邀请码失败"))
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

// ── Mods handlers ─────────────────────────────────────────────────────────────

// handleModsList handles GET /api/instances/:id/mods.
func (s *server) handleModsList(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	mods, err := sj.ListMods(instance.DataDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_mods_failed", sanitizeErrorMsg(err, "读取 Mod 列表失败"))
		return
	}
	restartRequired := sj.GetModsRestartRequired(instance.DataDir)
	writeJSON(w, http.StatusOK, registry.ModsListResult{
		Mods:            mods,
		RestartRequired: restartRequired,
	})
}

// handleModsUpload handles POST /api/instances/:id/mods/upload.
func (s *server) handleModsUpload(w http.ResponseWriter, r *http.Request, instanceID string) {
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

	// Hard cap on total request body to prevent disk exhaustion from oversized uploads.
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)

	if err := r.ParseMultipartForm(maxModFormSize); err != nil {
		writeError(w, http.StatusBadRequest, "parse_form_failed", "解析上传表单失败（文件可能超过大小限制）")
		return
	}

	file, _, err := r.FormFile("mod")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_file", "未找到上传字段 'mod'")
		return
	}
	defer func() { _ = file.Close() }()

	tmp, err := os.CreateTemp("", "stardew-mod-upload-*.zip")
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

	imported, err := sj.UploadModZip(instance.DataDir, tmpPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_mod_zip", sanitizeError(err, "Mod ZIP 无效"))
		return
	}

	// Set restart required flag.
	if err := sj.SetModsRestartRequired(instance.DataDir); err != nil {
		s.logger.Warn("set mods restart required", "instance", instanceID, "error", err)
	}

	s.logger.Info("mods uploaded", "instance", instanceID, "count", len(imported))
	s.auditLog(r, &actor, "mod_upload", "instance", instanceID, auditMetadata("count", fmt.Sprintf("%d", len(imported))))
	writeJSON(w, http.StatusOK, registry.ModsListResult{
		Mods:            imported,
		RestartRequired: true,
	})
}

// handleModDelete handles DELETE /api/instances/:id/mods/:modId.
func (s *server) handleModDelete(w http.ResponseWriter, r *http.Request, instanceID, modID string) {
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
	if err := sj.DeleteMod(instance.DataDir, modID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed", sanitizeErrorMsg(err, "删除 Mod 失败"))
		return
	}
	// Set restart required flag.
	if err := sj.SetModsRestartRequired(instance.DataDir); err != nil {
		s.logger.Warn("set mods restart required", "instance", instanceID, "error", err)
	}
	s.logger.Info("mod deleted", "instance", instanceID, "mod", modID)
	s.auditLog(r, &actor, "mod_delete", "instance", instanceID, auditMetadata("modId", modID))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleModsExport handles POST /api/instances/:id/mods/export.
func (s *server) handleModsExport(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}

	zipPath, err := sj.ExportModsZip(instance.DataDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export_failed", sanitizeErrorMsg(err, "导出 Mod 失败"))
		return
	}
	defer func() { _ = os.Remove(zipPath) }()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(zipPath)))
	http.ServeFile(w, r, zipPath)
}

// ── Console / Commands handlers ───────────────────────────────────────────────

// consoleRunner is the interface for drivers that support console commands.
type consoleRunner interface {
	RunAllowlistedCommand(ctx context.Context, instance registry.Instance, req sj.CommandRequest, isAdmin bool) (*sj.CommandRunResult, error)
	SendSay(ctx context.Context, instance registry.Instance, message string) (*sj.CommandRunResult, error)
}

// handleCommandsList handles GET /api/instances/:id/commands.
func (s *server) handleCommandsList(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	isAdmin := actor.User.Role == "admin"
	cmds := sj.ListCommands(isAdmin)
	writeJSON(w, http.StatusOK, map[string]any{"commands": cmds})
}

// handleCommandRun handles POST /api/instances/:id/commands/run.
func (s *server) handleCommandRun(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	// Reconcile to get real Docker container state before executing commands.
	instance, ok = s.reconcileInstanceState(w, r, instance)
	if !ok {
		return
	}
	if instance.State != storage.InstanceStateRunning {
		writeError(w, http.StatusConflict, "server_not_running", "服务器未运行，无法执行命令")
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}

	runner, supported := driver.(consoleRunner)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持命令执行")
		return
	}

	var req sj.CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "请求格式错误")
		return
	}

	result, err := runner.RunAllowlistedCommand(r.Context(), makeRegistryInstance(instance), req, actor.User.Role == "admin")
	if err != nil {
		if ce, ok := err.(*sj.CommandError); ok {
			status := http.StatusBadRequest
			switch ce.Code {
			case "server_not_running":
				status = http.StatusConflict
			case "forbidden":
				status = http.StatusForbidden
			case "not_supported", "command_not_supported":
				status = http.StatusNotImplemented
			}
			writeError(w, status, ce.Code, ce.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "command_failed", sanitizeErrorMsg(err, "执行命令失败"))
		return
	}
	s.auditLog(r, &actor, "command_run", "instance", instanceID, auditMetadata("command", req.Command))
	writeJSON(w, http.StatusOK, result)
}

// handleCommandSay handles POST /api/instances/:id/commands/say.
func (s *server) handleCommandSay(w http.ResponseWriter, r *http.Request, instanceID string) {
	_, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	// Reconcile to get real Docker container state before sending say.
	instance, ok = s.reconcileInstanceState(w, r, instance)
	if !ok {
		return
	}
	if instance.State != storage.InstanceStateRunning {
		writeError(w, http.StatusConflict, "server_not_running", "服务器未运行，无法发送喊话")
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}

	runner, supported := driver.(consoleRunner)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "该 driver 不支持喊话")
		return
	}

	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "请求格式错误")
		return
	}

	result, err := runner.SendSay(r.Context(), makeRegistryInstance(instance), body.Message)
	if err != nil {
		if ce, ok := err.(*sj.CommandError); ok {
			status := http.StatusBadRequest
			switch ce.Code {
			case "server_not_running":
				status = http.StatusConflict
			case "not_supported", "command_not_supported":
				status = http.StatusNotImplemented
			}
			writeError(w, status, ce.Code, ce.Message)
			return
		}
		writeError(w, http.StatusInternalServerError, "say_failed", sanitizeErrorMsg(err, "发送喊话失败"))
		return
	}
	writeJSON(w, http.StatusOK, result)
}
