package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	uploadTokenTTL    = 10 * time.Minute
	maxUploadFormSize = 110 * 1024 * 1024 // multipart memory: save ZIP limit is 100 MB
	maxModFormSize    = 210 * 1024 * 1024 // multipart memory: mod ZIP limit is 200 MB
	maxRequestBody    = 220 * 1024 * 1024 // hard cap on total request body (slightly above largest ZIP limit)
)

// 鈹€鈹€ Pending upload token store 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

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
		return nil, fmt.Errorf("涓婁紶浠ょ墝鏃犳晥鎴栧凡杩囨湡")
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(s.entries, token)
		_ = os.RemoveAll(entry.TempDir)
		return nil, fmt.Errorf("upload token expired")
	}
	if entry.InstanceID != instanceID {
		return nil, fmt.Errorf("涓婁紶浠ょ墝涓庡疄渚嬩笉鍖归厤")
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

// 鈹€鈹€ Handlers 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

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
		writeError(w, http.StatusInternalServerError, "list_saves_failed", sanitizeErrorMsg(err, "璇诲彇瀛樻。鍒楄〃澶辫触"))
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
		writeError(w, http.StatusBadRequest, "invalid_body", "璇锋眰浣撹В鏋愬け璐?")
		return
	}

	if err := sj.WriteServerSettings(instance.DataDir, cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_config", sanitizeError(err, "閰嶇疆鍙傛暟鏃犳晥"))
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
		writeError(w, http.StatusInternalServerError, "start_failed", sanitizeErrorMsg(err, "鏈嶅姟鍣ㄥ惎鍔ㄥけ璐?"))
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
		writeError(w, http.StatusBadRequest, "parse_form_failed", "瑙ｆ瀽涓婁紶琛ㄥ崟澶辫触锛堟枃浠跺彲鑳借秴杩囧ぇ灏忛檺鍒讹級")
		return
	}

	file, header, err := r.FormFile("save")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing_file", "鏈壘鍒颁笂浼犲瓧娈?'save'")
		return
	}
	defer func() { _ = file.Close() }()

	tmp, err := os.CreateTemp("", "stardew-upload-*.zip")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "鍒涘缓涓存椂鏂囦欢澶辫触")
		return
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := io.Copy(tmp, file); err != nil {
		_ = tmp.Close()
		writeError(w, http.StatusInternalServerError, "write_failed", "鍐欏叆涓存椂鏂囦欢澶辫触")
		return
	}
	_ = tmp.Close()

	saveName, preview, tempDir, err := sj.PreviewSaveZip(tmpPath, header.Filename)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_zip", sanitizeError(err, "瀛樻。 ZIP 鏃犳晥"))
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
		writeError(w, http.StatusBadRequest, "invalid_body", "璇锋眰浣撹В鏋愬け璐?")
		return
	}
	if body.Token == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "token 涓嶈兘涓虹┖")
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
		writeError(w, http.StatusConflict, "token_invalid", sanitizeError(err, "涓婁紶浠ょ墝鏃犳晥"))
		return
	}
	defer func() { _ = os.RemoveAll(entry.TempDir) }()

	if err := sj.ImportSaveToVolume(instance.DataDir, entry.TempDir, entry.SaveName); err != nil {
		writeError(w, http.StatusInternalServerError, "import_failed", sanitizeErrorMsg(err, "瀵煎叆瀛樻。澶辫触"))
		return
	}

	if err := sj.EnsureDisabledModProfileForSave(instance.DataDir, entry.SaveName); err != nil {
		writeError(w, http.StatusInternalServerError, "mod_profile_failed", sanitizeErrorMsg(err, "initialize save mod profile failed"))
		return
	}
	if err := sj.ApplyModProfile(instance.DataDir, entry.SaveName); err != nil {
		writeError(w, http.StatusInternalServerError, "mod_profile_apply_failed", sanitizeErrorMsg(err, "apply save mod profile failed"))
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
		writeError(w, http.StatusInternalServerError, "start_failed", sanitizeErrorMsg(err, "鏈嶅姟鍣ㄥ惎鍔ㄥけ璐?"))
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
		writeError(w, http.StatusInternalServerError, "list_saves_failed", sanitizeErrorMsg(err, "璇诲彇瀛樻。鍒楄〃澶辫触"))
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
		writeError(w, http.StatusBadRequest, "invalid_body", "璇锋眰浣撹В鏋愬け璐?")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "瀛樻。鍚嶇О涓嶈兘涓虹┖")
		return
	}
	if err := sj.ValidateSaveExists(instance.DataDir, body.Name); err != nil {
		writeError(w, http.StatusNotFound, "save_not_found", sanitizeError(err, "瀛樻。涓嶅瓨鍦?"))
		return
	}
	if err := sj.SetActiveSave(instance.DataDir, body.Name); err != nil {
		writeError(w, http.StatusInternalServerError, "select_failed", sanitizeErrorMsg(err, "閫夋嫨瀛樻。澶辫触"))
		return
	}
	if err := sj.ApplyModProfile(instance.DataDir, body.Name); err != nil {
		writeError(w, http.StatusInternalServerError, "mod_profile_apply_failed", sanitizeErrorMsg(err, "apply save mod profile failed"))
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
		writeError(w, http.StatusBadRequest, "invalid_body", "璇锋眰浣撹В鏋愬け璐?")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "瀛樻。鍚嶇О涓嶈兘涓虹┖")
		return
	}
	if err := sj.ValidateSaveExists(instance.DataDir, body.Name); err != nil {
		writeError(w, http.StatusNotFound, "save_not_found", sanitizeError(err, "瀛樻。涓嶅瓨鍦?"))
		return
	}
	if err := sj.SetActiveSave(instance.DataDir, body.Name); err != nil {
		writeError(w, http.StatusInternalServerError, "select_failed", sanitizeErrorMsg(err, "閫夋嫨瀛樻。澶辫触"))
		return
	}
	if err := sj.ApplyModProfile(instance.DataDir, body.Name); err != nil {
		writeError(w, http.StatusInternalServerError, "mod_profile_apply_failed", sanitizeErrorMsg(err, "apply save mod profile failed"))
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
		writeError(w, http.StatusInternalServerError, "start_failed", sanitizeErrorMsg(err, "鏈嶅姟鍣ㄥ惎鍔ㄥけ璐?"))
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
	instance, ok = s.reconcileInstanceState(w, r, instance)
	if !ok {
		return
	}
	activeSaveName := sj.GetActiveSaveName(instance.DataDir)
	if activeSaveName == saveName && (instance.State == storage.InstanceStateRunning || instance.State == storage.InstanceStateStarting) {
		writeError(w, http.StatusConflict, "active_save_running", "褰撳墠鍚姩瀛樻。姝ｅ湪琚湇鍔″櫒浣跨敤锛岃鍏堝仠姝㈡湇鍔″櫒鍐嶅垹闄ゃ€?")
		return
	}
	backupPath, err := sj.DeleteSaveWithBackup(instance.DataDir, saveName)
	if err != nil {
		s.logger.Warn("delete save failed (backup failure blocks deletion)", "instance", instanceID, "save", saveName, "error", err)
		writeError(w, http.StatusInternalServerError, "save_delete_failed", sanitizeErrorMsg(err, "鍒犻櫎瀛樻。澶辫触"))
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
		writeError(w, http.StatusInternalServerError, "export_failed", sanitizeErrorMsg(err, "瀵煎嚭瀛樻。澶辫触"))
		return
	}
	defer func() { _ = os.Remove(zipPath) }()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(zipPath)))
	http.ServeFile(w, r, zipPath)
}

// handleSaveBackupCreate handles POST /api/instances/:id/saves/:name/backup.
func (s *server) handleSaveBackupCreate(w http.ResponseWriter, r *http.Request, instanceID, saveName string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	backupPath, err := sj.BackupSave(instance.DataDir, saveName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backup_failed", sanitizeErrorMsg(err, "save backup failed"))
		return
	}
	backupName := filepath.Base(backupPath)
	s.auditLog(r, &actor, "save_backup_create", "instance", instanceID, auditMetadata("saveName", saveName, "backupName", backupName))
	writeJSON(w, http.StatusOK, map[string]string{"backupName": backupName})
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
	maintenance, maintenanceErr := sj.RunBackupMaintenance(instance.DataDir)
	if maintenanceErr != nil {
		s.logger.Warn("save backup maintenance failed", "instance", instanceID, "error", maintenanceErr)
	}
	backups, err := sj.ListBackups(instance.DataDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_backups_failed", sanitizeErrorMsg(err, "璇诲彇澶囦唤鍒楄〃澶辫触"))
		return
	}
	policy, policyErr := sj.ReadBackupPolicy(instance.DataDir)
	if policyErr != nil {
		s.logger.Warn("read save backup policy failed", "instance", instanceID, "error", policyErr)
		policy = sj.DefaultBackupPolicy()
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": backups, "policy": policy, "maintenance": maintenance})
}

// handleSavesBackupPolicy handles GET/PUT /api/instances/:id/saves/backups/policy.
func (s *server) handleSavesBackupPolicy(w http.ResponseWriter, r *http.Request, instanceID string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		policy, err := sj.ReadBackupPolicy(instance.DataDir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "backup_policy_failed", sanitizeErrorMsg(err, "read backup policy failed"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
	case http.MethodPut:
		var body sj.BackupPolicy
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "invalid backup policy body")
			return
		}
		policy, err := sj.WriteBackupPolicy(instance.DataDir, body)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "backup_policy_failed", sanitizeErrorMsg(err, "write backup policy failed"))
			return
		}
		policyAudit, _ := json.Marshal(policy)
		s.auditLog(r, &actor, "save_backup_policy_update", "instance", instanceID, auditMetadata("policy", string(policyAudit)))
		writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

// handleSavesBackupDelete handles DELETE /api/instances/:id/saves/backups/:backupName.
func (s *server) handleSavesBackupDelete(w http.ResponseWriter, r *http.Request, instanceID, backupName string) {
	actor, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	if err := sj.DeleteBackup(instance.DataDir, backupName); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "涓嶅悎娉?") {
			writeError(w, http.StatusBadRequest, "invalid_backup_name", errMsg)
			return
		}
		if strings.Contains(errMsg, "涓嶅瓨鍦?") {
			writeError(w, http.StatusNotFound, "backup_not_found", errMsg)
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_backup_failed", sanitizeErrorMsg(err, "鍒犻櫎澶囦唤澶辫触"))
		return
	}

	s.auditLog(r, &actor, "save_backup_delete", "instance", instanceID, auditMetadata("backupName", backupName))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
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
		writeError(w, http.StatusBadRequest, "invalid_body", "璇锋眰浣撹В鏋愬け璐?")
		return
	}
	if body.BackupName == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "backupName 涓嶈兘涓虹┖")
		return
	}

	saveName, err := sj.RestoreBackup(instance.DataDir, body.BackupName, body.Overwrite)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "宸插瓨鍦?") {
			writeError(w, http.StatusConflict, "save_exists", errMsg)
			return
		}
		writeError(w, http.StatusInternalServerError, "restore_failed", sanitizeErrorMsg(err, "鎭㈠澶囦唤澶辫触"))
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
		writeError(w, http.StatusInternalServerError, "list_saves_failed", sanitizeErrorMsg(err, "璇诲彇瀛樻。鍒楄〃澶辫触"))
		return
	}
	if len(saves) == 0 {
		writeError(w, http.StatusConflict, "save_required", "娌℃湁鍙敤瀛樻。锛岃鍏堝垱寤哄瓨妗ｅ苟鍚姩鎴栦笂浼犲瓨妗ｅ苟鍚姩")
		return
	}

	// Check active save: must have one selected, and it must still exist.
	activeName := sj.GetActiveSaveName(instance.DataDir)
	if activeName == "" {
		writeError(w, http.StatusConflict, "active_save_required", "娌℃湁宸查€夋嫨鐨勫惎鍔ㄥ瓨妗ｏ紝璇峰厛鍒涘缓銆佷笂浼犳垨閫夋嫨涓€涓瓨妗?")
		return
	}
	if err := sj.ValidateSaveExists(instance.DataDir, activeName); err != nil {
		writeError(w, http.StatusConflict, "active_save_missing", "涓婃閫夋嫨鐨勫瓨妗ｄ笉瀛樺湪锛岃閲嶆柊閫夋嫨瀛樻。")
		return
	}

	job, err := driver.Start(r.Context(), registry.StartRequest{
		Instance: makeRegistryInstance(instance),
		ActorID:  actor.User.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "start_failed", sanitizeErrorMsg(err, "鏈嶅姟鍣ㄥ惎鍔ㄥけ璐?"))
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
		writeError(w, http.StatusInternalServerError, "stop_failed", sanitizeErrorMsg(err, "鏈嶅姟鍣ㄥ仠姝㈠け璐?"))
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
		writeError(w, http.StatusInternalServerError, "restart_failed", sanitizeErrorMsg(err, "鏈嶅姟鍣ㄩ噸鍚け璐?"))
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
		writeError(w, http.StatusNotImplemented, "not_supported", "璇?driver 涓嶆敮鎸佽幏鍙栭個璇风爜")
		return
	}

	code, err := getter.GetInviteCode(r.Context(), makeRegistryInstance(instance))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invite_code_failed", sanitizeErrorMsg(err, "鑾峰彇閭€璇风爜澶辫触"))
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
		writeError(w, http.StatusConflict, "server_running", "鏈嶅姟鍣ㄨ繍琛屼腑锛岃鍏堝仠姝㈡湇鍔″櫒鍐嶆搷浣滃瓨妗ｃ€?")
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
		StateMessage:  "瀛樻。宸查€夋嫨锛屽噯澶囧惎鍔ㄣ€?",
		DriverPhase:   "ready_to_start",
		DriverPayload: instance.DriverPayload,
	})
	return err
}

// 鈹€鈹€ Mods handlers 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

// handleModsList handles GET /api/instances/:id/mods.
func (s *server) handleModsList(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	activeSaveName := sj.GetActiveSaveName(instance.DataDir)
	mods, err := sj.ListModsWithState(instance.DataDir, activeSaveName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_mods_failed", sanitizeErrorMsg(err, "璇诲彇 Mod 鍒楄〃澶辫触"))
		return
	}
	mods = sj.ApplyModSyncClassification(instance.DataDir, mods)
	mods = sj.EnrichNexusMetadataForMods(r.Context(), instance.DataDir, mods)
	restartRequired := modsRestartRequiredForState(instance, instance.DataDir)
	writeJSON(w, http.StatusOK, registry.ModsListResult{
		Mods:            mods,
		RestartRequired: restartRequired,
	})
}

func modsRestartRequiredForState(instance storage.Instance, dataDir string) bool {
	if instance.State != storage.InstanceStateRunning && instance.State != storage.InstanceStateStarting {
		return false
	}
	return sj.GetModsRestartRequired(dataDir)
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
		writeError(w, http.StatusBadRequest, "parse_form_failed", "瑙ｆ瀽涓婁紶琛ㄥ崟澶辫触锛堟枃浠跺彲鑳借秴杩囧ぇ灏忛檺鍒讹級")
		return
	}

	modFiles := r.MultipartForm.File["mod"]
	modFiles = append(modFiles, r.MultipartForm.File["mods"]...)
	if len(modFiles) == 0 {
		writeError(w, http.StatusBadRequest, "missing_file", "鏈壘鍒颁笂浼犲瓧娈?'mod'")
		return
	}

	var imported []registry.ModInfo
	for i, header := range modFiles {
		file, err := header.Open()
		if err != nil {
			rollbackImportedMods(instance.DataDir, imported, s.logger, instanceID)
			writeError(w, http.StatusBadRequest, "missing_file", fmt.Sprintf("璇诲彇绗?%d 涓?Mod ZIP 澶辫触", i+1))
			return
		}

		tmp, err := os.CreateTemp("", "stardew-mod-upload-*.zip")
		if err != nil {
			_ = file.Close()
			rollbackImportedMods(instance.DataDir, imported, s.logger, instanceID)
			writeError(w, http.StatusInternalServerError, "internal_error", "鍒涘缓涓存椂鏂囦欢澶辫触")
			return
		}
		tmpPath := tmp.Name()

		if _, err := io.Copy(tmp, file); err != nil {
			_ = file.Close()
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			rollbackImportedMods(instance.DataDir, imported, s.logger, instanceID)
			writeError(w, http.StatusInternalServerError, "write_failed", "鍐欏叆涓存椂鏂囦欢澶辫触")
			return
		}
		_ = file.Close()
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmpPath)
			rollbackImportedMods(instance.DataDir, imported, s.logger, instanceID)
			writeError(w, http.StatusInternalServerError, "write_failed", "鍐欏叆涓存椂鏂囦欢澶辫触")
			return
		}

		batch, err := sj.UploadModZip(instance.DataDir, tmpPath)
		_ = os.Remove(tmpPath)
		if err != nil {
			rollbackImportedMods(instance.DataDir, imported, s.logger, instanceID)
			writeError(w, http.StatusBadRequest, modUploadErrorCode(err), sanitizeError(err, fmt.Sprintf("绗?%d 涓?Mod ZIP 鏃犳晥", i+1)))
			return
		}
		imported = append(imported, batch...)
	}

	// Mod writes are only allowed while the game server is stopped, so the next
	// normal start will load the new files without requiring an extra restart.
	if activeSaveName := sj.GetActiveSaveName(instance.DataDir); activeSaveName != "" {
		if err := sj.MarkImportedModsEnabledForSave(instance.DataDir, activeSaveName, imported); err != nil {
			s.logger.Warn("mark imported mods enabled", "instance", instanceID, "save", activeSaveName, "error", err)
		}
	}
	if err := sj.ClearModsRestartRequired(instance.DataDir); err != nil {
		s.logger.Warn("clear mods restart required", "instance", instanceID, "error", err)
	}

	s.logger.Info("mods uploaded", "instance", instanceID, "count", len(imported))
	s.auditLog(r, &actor, "mod_upload", "instance", instanceID, auditMetadata("count", fmt.Sprintf("%d", len(imported))))
	writeJSON(w, http.StatusOK, registry.ModsListResult{
		Mods:            imported,
		RestartRequired: modsRestartRequiredForState(instance, instance.DataDir),
	})
}

func rollbackImportedMods(dataDir string, imported []registry.ModInfo, logger *slog.Logger, instanceID string) {
	for i := len(imported) - 1; i >= 0; i-- {
		folder := imported[i].FolderName
		if folder == "" {
			continue
		}
		if err := sj.DeleteMod(dataDir, folder); err != nil && logger != nil {
			logger.Warn("rollback imported mod after batch upload failure", "instance", instanceID, "mod", folder, "error", err)
		}
	}
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
		writeError(w, http.StatusInternalServerError, "delete_failed", sanitizeErrorMsg(err, "鍒犻櫎 Mod 澶辫触"))
		return
	}
	// Mod writes are only allowed while stopped; clear any stale restart marker.
	if err := sj.ClearModsRestartRequired(instance.DataDir); err != nil {
		s.logger.Warn("clear mods restart required", "instance", instanceID, "error", err)
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
		writeError(w, http.StatusInternalServerError, "export_failed", sanitizeErrorMsg(err, "瀵煎嚭 Mod 澶辫触"))
		return
	}
	defer func() { _ = os.Remove(zipPath) }()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(zipPath)))
	http.ServeFile(w, r, zipPath)
}

// handleModSyncPlan handles GET /api/instances/:id/mods/sync-plan.
func (s *server) handleModSyncPlan(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	plan, err := sj.BuildModSyncPlan(instance.DataDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sync_plan_failed", sanitizeErrorMsg(err, "璇诲彇鍚屾鍒嗙被澶辫触"))
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

// modSyncClassificationRequest is the body of PUT .../mods/:modId/sync-classification.
type modSyncClassificationRequest struct {
	SyncKind string `json:"syncKind"`
	SyncNote string `json:"syncNote,omitempty"`
}

type modEnabledRequest struct {
	Enabled  bool   `json:"enabled"`
	SaveName string `json:"saveName,omitempty"`
}

// handleModEnabledUpdate handles PUT /api/instances/:id/mods/:modId/enabled.
// First-stage profile changes move folders between mods and mods-disabled, so
// the server must be stopped.
func (s *server) handleModEnabledUpdate(w http.ResponseWriter, r *http.Request, instanceID, modID string) {
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
	var req modEnabledRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	saveName := strings.TrimSpace(req.SaveName)
	if saveName == "" {
		saveName = sj.GetActiveSaveName(instance.DataDir)
	}
	if saveName == "" {
		writeError(w, http.StatusConflict, "active_save_required", "active save is required")
		return
	}
	mods, err := sj.SetModEnabledForSaveCascade(instance.DataDir, saveName, modID, req.Enabled)
	if err != nil {
		writeError(w, http.StatusBadRequest, "mod_enable_failed", sanitizeErrorMsg(err, "update mod enabled state failed"))
		return
	}
	affectedNames := make([]string, 0, len(mods))
	for _, mod := range mods {
		affectedNames = append(affectedNames, mod.FolderName)
	}
	s.logger.Info("mod enabled state updated", "instance", instanceID, "save", saveName, "mod", modID, "enabled", req.Enabled, "affected", strings.Join(affectedNames, ","))
	s.auditLog(r, &actor, "mod_enabled_update", "instance", instanceID, auditMetadata("saveName", saveName, "modId", modID, "enabled", strconv.FormatBool(req.Enabled), "affected", strings.Join(affectedNames, ",")))
	writeJSON(w, http.StatusOK, map[string]any{
		"mods":     mods,
		"enabled":  req.Enabled,
		"saveName": saveName,
	})
}

// handleModSyncClassificationUpdate handles PUT /api/instances/:id/mods/:modId/sync-classification.
// This only writes the panel's own classification metadata, so it is allowed
// regardless of whether the server is running.
func (s *server) handleModSyncClassificationUpdate(w http.ResponseWriter, r *http.Request, instanceID, modID string) {
	actor, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}
	var req modSyncClassificationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !registry.ValidModSyncKind(req.SyncKind) {
		writeError(w, http.StatusBadRequest, "invalid_sync_kind", "鏃犳晥鐨勫悓姝ュ垎绫?")
		return
	}
	mods, err := sj.SetModSyncClassificationCascade(instance.DataDir, modID, req.SyncKind, req.SyncNote)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			writeError(w, http.StatusNotFound, "mod_not_found", "Mod 涓嶅瓨鍦?")
			return
		}
		writeError(w, http.StatusBadRequest, "sync_classification_failed", sanitizeErrorMsg(err, "鏇存柊鍚屾鍒嗙被澶辫触"))
		return
	}
	affectedNames := make([]string, 0, len(mods))
	for _, mod := range mods {
		affectedNames = append(affectedNames, mod.FolderName)
	}
	s.logger.Info("mod sync classification updated", "instance", instanceID, "mod", modID, "syncKind", req.SyncKind, "affected", strings.Join(affectedNames, ","))
	s.auditLog(r, &actor, "mod_sync_classification_update", "instance", instanceID, auditMetadata("modId", modID, "syncKind", req.SyncKind, "affected", strings.Join(affectedNames, ",")))
	writeJSON(w, http.StatusOK, map[string]any{
		"mods":     mods,
		"syncKind": req.SyncKind,
	})
}

// handleModSyncPackExport handles POST /api/instances/:id/mods/sync-pack/export.
// Export is allowed while the server is running so players can download the
// pack at any time.
func (s *server) handleModSyncPackExport(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}

	zipPath, err := sj.ExportModSyncPackZip(instance.DataDir)
	if err != nil {
		if errors.Is(err, sj.ErrNoSyncMods) {
			writeError(w, http.StatusBadRequest, "no_sync_mods", "娌℃湁鐜╁闇€鍚屾鐨?Mod 鍙鍑?")
			return
		}
		writeError(w, http.StatusInternalServerError, "export_sync_pack_failed", sanitizeErrorMsg(err, "瀵煎嚭鍚屾鍖呭け璐?"))
		return
	}
	defer func() { _ = os.Remove(zipPath) }()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", sj.PlayerSyncPackFileName))
	http.ServeFile(w, r, zipPath)
}

// handleModSyncUpdatePackExport handles POST /api/instances/:id/mods/sync-pack/export-update.
// It exports a lightweight mod-only pack for players who already have SMAPI.
func (s *server) handleModSyncUpdatePackExport(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}

	zipPath, err := sj.ExportModSyncUpdatePackZip(instance.DataDir)
	if err != nil {
		if errors.Is(err, sj.ErrNoSyncMods) {
			writeError(w, http.StatusBadRequest, "no_sync_mods", "没有可打包的玩家同步 Mod")
			return
		}
		writeError(w, http.StatusInternalServerError, "export_sync_pack_failed", sanitizeErrorMsg(err, "导出模组更新包失败"))
		return
	}
	defer func() { _ = os.Remove(zipPath) }()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", sj.PlayerModUpdatePackFileName))
	http.ServeFile(w, r, zipPath)
}

// handleModNexusExtensionDownload handles GET /api/instances/:id/mods/nexus/extension/download.
func (s *server) handleModNexusExtensionDownload(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}

	zipPath, err := sj.EnsureNexusInstallerExtensionZip(instance.DataDir)
	if err != nil {
		if errors.Is(err, sj.ErrNexusInstallerExtensionNotFound) {
			writeError(w, http.StatusNotFound, "nexus_extension_not_found", "浏览器扩展包不存在，请检查面板部署是否包含 browser-extensions 目录")
			return
		}
		writeError(w, http.StatusInternalServerError, "nexus_extension_export_failed", sanitizeErrorMsg(err, "打包浏览器扩展失败"))
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", sj.NexusInstallerExtensionFileName))
	http.ServeFile(w, r, zipPath)
}

// handleModNexusSearch handles GET /api/instances/:id/mods/nexus/search?q=...
// Any logged-in user (not just admins) may search and open the Nexus page;
// this phase is read-only and never proxies a download.
func (s *server) handleModNexusSearch(w http.ResponseWriter, r *http.Request, instanceID string) {
	if _, ok := s.requireAuth(w, r); !ok {
		return
	}
	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	page := positiveIntQuery(r, "page", 1)
	pageSize := positiveIntQuery(r, "pageSize", 20)

	apiKey, err := s.nexusAPIKey(r.Context())
	if err != nil {
		s.logger.Error("failed to load nexus api key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	searchCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 20*time.Second)
	defer cancel()

	result, err := sj.SearchNexusModsPage(searchCtx, query, apiKey, page, pageSize)
	if err != nil {
		s.writeNexusError(w, err)
		return
	}
	activeSaveName := sj.GetActiveSaveName(instance.DataDir)
	result.Results = sj.ApplyNexusInstalledMatch(instance.DataDir, activeSaveName, result.Results)
	writeJSON(w, http.StatusOK, result)
}

func positiveIntQuery(r *http.Request, key string, fallback int) int {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

type nexusInstallRequest struct {
	ModID            int    `json:"modId"`
	Name             string `json:"name"`
	Summary          string `json:"summary,omitempty"`
	Author           string `json:"author,omitempty"`
	Version          string `json:"version,omitempty"`
	UpdatedAt        string `json:"updatedAt,omitempty"`
	EndorsementCount int    `json:"endorsementCount"`
	DownloadCount    int    `json:"downloadCount"`
	PictureURL       string `json:"pictureUrl,omitempty"`
	NexusURL         string `json:"nexusUrl"`
}

func (req nexusInstallRequest) toSearchResult() sj.NexusModSearchResult {
	return sj.NexusModSearchResult{
		ModID:            req.ModID,
		Name:             req.Name,
		Summary:          req.Summary,
		Author:           req.Author,
		Version:          req.Version,
		UpdatedAt:        req.UpdatedAt,
		EndorsementCount: req.EndorsementCount,
		DownloadCount:    req.DownloadCount,
		PictureURL:       req.PictureURL,
		NexusURL:         req.NexusURL,
	}
}

func modInstallJobDisplayName(jobType string, result sj.NexusModSearchResult) string {
	name := strings.Join(strings.Fields(result.Name), " ")
	if name == "" && result.ModID > 0 {
		name = fmt.Sprintf("Nexus Mod #%d", result.ModID)
	}
	if name == "" {
		return ""
	}
	runes := []rune(name)
	if len(runes) > 80 {
		name = string(runes[:80]) + "..."
	}
	return fmt.Sprintf("%s · %s", jobType, name)
}

type remoteInstallRequest struct {
	URL string              `json:"url"`
	Mod nexusInstallRequest `json:"mod,omitempty"`
}

// handleModNexusInstall handles POST /api/instances/:id/mods/nexus/install.
func (s *server) handleModNexusInstall(w http.ResponseWriter, r *http.Request, instanceID string) {
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

	var req nexusInstallRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ModID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_query", "Nexus Mod ID 鏃犳晥")
		return
	}

	apiKey, err := s.nexusAPIKey(r.Context())
	if err != nil {
		s.logger.Error("failed to load nexus api key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	if strings.TrimSpace(apiKey) == "" {
		s.writeNexusError(w, sj.ErrNexusAPIKeyMissing)
		return
	}

	result := req.toSearchResult()
	job, err := s.jobs.Start(r.Context(), jobs.Spec{
		Type:        "mod_nexus_install",
		DisplayName: modInstallJobDisplayName("mod_nexus_install", result),
		TargetType:  "instance",
		TargetID:    instanceID,
		CreatedBy:   actor.User.ID,
		Timeout:     30 * time.Minute,
		Run: func(ctx context.Context, job *jobs.Context) error {
			_, _ = job.Info(ctx, fmt.Sprintf("准备安装 Nexus Mod #%d", result.ModID))
			imported, err := sj.InstallNexusMod(ctx, instance.DataDir, apiKey, result, func(message string) {
				_, _ = job.Info(ctx, message)
			})
			if err != nil {
				return err
			}
			for _, mod := range imported {
				name := mod.Name
				if name == "" {
					name = mod.FolderName
				}
				_, _ = job.Info(ctx, fmt.Sprintf("已导入：%s", name))
			}
			if activeSaveName := sj.GetActiveSaveName(instance.DataDir); activeSaveName != "" {
				if err := sj.MarkImportedModsEnabledForSave(instance.DataDir, activeSaveName, imported); err != nil {
					s.logger.Warn("mark nexus installed mods enabled", "instance", instanceID, "save", activeSaveName, "error", err)
					_, _ = job.Info(ctx, "安装完成，但当前存档启用状态更新失败，请到配置模组页手动启用")
				} else {
					_, _ = job.Info(ctx, fmt.Sprintf("已为当前存档启用：%s", activeSaveName))
				}
			}
			_ = sj.ClearModsRestartRequired(instance.DataDir)
			return nil
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "job_create_failed", sanitizeErrorMsg(err, "鍒涘缓 Nexus 瀹夎浠诲姟澶辫触"))
		return
	}

	s.logger.Info("nexus mod install queued", "instance", instanceID, "job", job.ID, "modId", req.ModID)
	s.auditLog(r, &actor, "mod_nexus_install", "instance", instanceID, auditMetadata("jobId", job.ID, "modId", fmt.Sprintf("%d", req.ModID)))
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": job.ID})
}

// handleModRemoteInstall handles POST /api/instances/:id/mods/remote/install.
func (s *server) handleModRemoteInstall(w http.ResponseWriter, r *http.Request, instanceID string) {
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

	var req remoteInstallRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	rawURL := strings.TrimSpace(req.URL)
	if rawURL == "" {
		writeError(w, http.StatusBadRequest, "invalid_remote_mod_url", "杩滅▼ Mod 涓嬭浇閾炬帴涓嶈兘涓虹┖")
		return
	}

	apiKey, err := s.nexusAPIKey(r.Context())
	if err != nil {
		s.logger.Error("failed to load nexus api key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	result := req.Mod.toSearchResult()
	job, err := s.jobs.Start(r.Context(), jobs.Spec{
		Type:        "mod_remote_install",
		DisplayName: modInstallJobDisplayName("mod_remote_install", result),
		TargetType:  "instance",
		TargetID:    instanceID,
		CreatedBy:   actor.User.ID,
		Timeout:     30 * time.Minute,
		Run: func(ctx context.Context, job *jobs.Context) error {
			_, _ = job.Info(ctx, "准备从远程链接安装 Mod")
			imported, err := sj.InstallRemoteMod(ctx, instance.DataDir, rawURL, apiKey, result, func(message string) {
				_, _ = job.Info(ctx, message)
			})
			if err != nil {
				return err
			}
			for _, mod := range imported {
				name := mod.Name
				if name == "" {
					name = mod.FolderName
				}
				_, _ = job.Info(ctx, fmt.Sprintf("已导入：%s", name))
			}
			if activeSaveName := sj.GetActiveSaveName(instance.DataDir); activeSaveName != "" {
				if err := sj.MarkImportedModsEnabledForSave(instance.DataDir, activeSaveName, imported); err != nil {
					s.logger.Warn("mark remote installed mods enabled", "instance", instanceID, "save", activeSaveName, "error", err)
					_, _ = job.Info(ctx, "安装完成，但当前存档启用状态更新失败，请到配置模组页手动启用")
				} else {
					_, _ = job.Info(ctx, fmt.Sprintf("已为当前存档启用：%s", activeSaveName))
				}
			}
			_ = sj.ClearModsRestartRequired(instance.DataDir)
			return nil
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "job_create_failed", sanitizeErrorMsg(err, "鍒涘缓杩滅▼瀹夎浠诲姟澶辫触"))
		return
	}

	s.logger.Info("remote mod install queued", "instance", instanceID, "job", job.ID)
	s.auditLog(r, &actor, "mod_remote_install", "instance", instanceID, auditMetadata("jobId", job.ID))
	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": job.ID})
}

// writeNexusError maps Nexus client errors to structured HTTP responses
// without ever including the upstream response body (which could echo
// request details) in the message sent to the browser.
func (s *server) writeNexusError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sj.ErrNexusAPIKeyMissing):
		writeError(w, http.StatusServiceUnavailable, "nexus_api_key_missing", "未配置 Nexus Mods API Key")
		return
	case errors.Is(err, sj.ErrInvalidNexusQuery):
		writeError(w, http.StatusBadRequest, "invalid_query", "搜索关键词不能为空")
		return
	case errors.Is(err, sj.ErrNexusAuthRequired):
		writeError(w, http.StatusBadGateway, "nexus_auth_required", "该查询需要 Nexus OAuth/认证能力")
		return
	}

	var apiErr *sj.NexusAPIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusNotFound:
			writeError(w, http.StatusNotFound, "nexus_mod_not_found", "未找到该 Mod")
		case http.StatusUnauthorized, http.StatusForbidden:
			writeError(w, http.StatusBadGateway, "nexus_unauthorized", "Nexus API Key 无效或权限不足")
		case http.StatusTooManyRequests:
			writeError(w, http.StatusTooManyRequests, "nexus_rate_limited", "Nexus 请求过于频繁，请稍后重试")
		default:
			writeError(w, http.StatusBadGateway, "nexus_request_failed", "Nexus 请求失败")
		}
		return
	}

	var requestErr *sj.NexusRequestError
	if errors.As(err, &requestErr) {
		s.logger.Warn("nexus request failed", "error", requestErr.Unwrap())
		writeError(w, http.StatusBadGateway, "nexus_network_failed", "Nexus 网络连接失败，请确认面板服务器能访问 api.nexusmods.com")
		return
	}

	s.logger.Warn("nexus search failed", "error", err)
	writeError(w, http.StatusBadGateway, "nexus_request_failed", "Nexus 请求失败，请稍后重试")
}

// 鈹€鈹€ Console / Commands handlers 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

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
		writeError(w, http.StatusConflict, "server_not_running", "鏈嶅姟鍣ㄦ湭杩愯锛屾棤娉曟墽琛屽懡浠?")
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}

	runner, supported := driver.(consoleRunner)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "璇?driver 涓嶆敮鎸佸懡浠ゆ墽琛?")
		return
	}

	var req sj.CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "璇锋眰鏍煎紡閿欒")
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
		writeError(w, http.StatusInternalServerError, "command_failed", sanitizeErrorMsg(err, "鎵ц鍛戒护澶辫触"))
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
		writeError(w, http.StatusConflict, "server_not_running", "鏈嶅姟鍣ㄦ湭杩愯锛屾棤娉曞彂閫佸枈璇?")
		return
	}
	driver, ok := s.loadDriver(w, instance.DriverID)
	if !ok {
		return
	}

	runner, supported := driver.(consoleRunner)
	if !supported {
		writeError(w, http.StatusNotImplemented, "not_supported", "璇?driver 涓嶆敮鎸佸枈璇?")
		return
	}

	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "璇锋眰鏍煎紡閿欒")
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
		writeError(w, http.StatusInternalServerError, "say_failed", sanitizeErrorMsg(err, "鍙戦€佸枈璇濆け璐?"))
		return
	}
	writeJSON(w, http.StatusOK, result)
}
