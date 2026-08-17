package stardew_junimo

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	roleCredentialStoreSchemaVersion  = 1
	roleCredentialStoreFileName       = "role-passwords.json"
	roleCredentialStoreMarkerFileName = "role-passwords.initialized"
	roleCredentialStoreLockFileName   = ".role-passwords.lock"
	roleCredentialStoreMaxBytes       = 1024 * 1024
	roleCredentialStoreLockTimeout    = 5 * time.Second
	roleCredentialStoreLockStaleAge   = 2 * time.Minute

	RoleCredentialStatusWaiting    = "waiting"
	RoleCredentialStatusConfigured = "configured"
	RoleCredentialStatusError      = "error"
)

type roleCredentialSave struct {
	Roles map[string]rolePasswordRecord `json:"roles"`
}

type roleCredentialStore struct {
	SchemaVersion int                           `json:"schemaVersion"`
	Saves         map[string]roleCredentialSave `json:"saves"`
}

type roleCredentialStoreSnapshot struct {
	Store        roleCredentialStore
	Exists       bool
	MarkerExists bool
}

type roleCredentialStoreLock struct {
	path  string
	owner string
}

func roleCredentialStorePath(dataDir string) string {
	return filepath.Join(controlDir(dataDir), roleCredentialStoreFileName)
}

func newRoleCredentialStore() roleCredentialStore {
	return roleCredentialStore{
		SchemaVersion: roleCredentialStoreSchemaVersion,
		Saves:         map[string]roleCredentialSave{},
	}
}

func readRoleCredentialStore(dataDir string) (roleCredentialStoreSnapshot, error) {
	return readRoleCredentialStorePath(roleCredentialStorePath(dataDir))
}

func readRoleCredentialStorePath(path string) (roleCredentialStoreSnapshot, error) {
	markerExists, markerErr := readRoleCredentialStoreMarker(filepath.Dir(path))
	if markerErr != nil {
		return roleCredentialStoreSnapshot{}, markerErr
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if markerExists {
			return roleCredentialStoreSnapshot{}, errors.New("role credential store is missing after initialization")
		}
		return roleCredentialStoreSnapshot{Store: newRoleCredentialStore()}, nil
	}
	if err != nil {
		return roleCredentialStoreSnapshot{}, fmt.Errorf("read role credential store: %w", err)
	}
	if len(raw) == 0 || len(raw) > roleCredentialStoreMaxBytes {
		return roleCredentialStoreSnapshot{}, errors.New("role credential store has an invalid size")
	}
	var store roleCredentialStore
	if err := json.Unmarshal(raw, &store); err != nil {
		return roleCredentialStoreSnapshot{}, fmt.Errorf("decode role credential store: %w", err)
	}
	if err := validateRoleCredentialStore(store); err != nil {
		return roleCredentialStoreSnapshot{}, err
	}
	if !markerExists {
		return roleCredentialStoreSnapshot{}, errors.New("role credential store exists without its initialization marker")
	}
	return roleCredentialStoreSnapshot{Store: store, Exists: true, MarkerExists: markerExists}, nil
}

func readRoleCredentialStoreMarker(dir string) (bool, error) {
	raw, err := os.ReadFile(filepath.Join(dir, roleCredentialStoreMarkerFileName))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read role credential store marker: %w", err)
	}
	if string(raw) != "1\n" {
		return false, errors.New("role credential store marker is invalid")
	}
	return true, nil
}

func validateRoleCredentialStore(store roleCredentialStore) error {
	if store.SchemaVersion != roleCredentialStoreSchemaVersion || store.Saves == nil {
		return errors.New("role credential store schema is missing or unsupported")
	}
	for saveID, save := range store.Saves {
		if !validRoleCredentialSaveID(saveID) || save.Roles == nil {
			return errors.New("role credential store contains an invalid save record")
		}
		for roleID, record := range save.Roles {
			verifier, verifierErr := base64.RawURLEncoding.DecodeString(strings.TrimSpace(record.Verifier))
			if !validPlayerAuthRoleID(roleID) || verifierErr != nil || len(verifier) != 32 {
				return errors.New("role credential store contains an invalid role record")
			}
		}
	}
	return nil
}

func validRoleCredentialSaveID(saveID string) bool {
	if strings.TrimSpace(saveID) != saveID || saveID == "" || utf8.RuneCountInString(saveID) > 512 {
		return false
	}
	for _, value := range saveID {
		if unicode.IsControl(value) {
			return false
		}
	}
	return true
}

func roleCredentialsForSave(dataDir, saveID string, legacy rolePasswordPayload) (map[string]rolePasswordRecord, bool, error) {
	snapshot, err := readRoleCredentialStore(dataDir)
	if err != nil {
		return nil, false, err
	}
	saveID = strings.TrimSpace(saveID)
	if !snapshot.Exists {
		if len(legacy.Roles) == 0 {
			return map[string]rolePasswordRecord{}, false, nil
		}
		if !validRoleCredentialSaveID(saveID) {
			return nil, false, errors.New("active save is unavailable for legacy role credentials")
		}
		return cloneRolePasswordRecords(legacy.Roles), true, nil
	}
	if !validRoleCredentialSaveID(saveID) {
		return nil, false, errors.New("active save is unavailable for role credentials")
	}
	save, ok := snapshot.Store.Saves[saveID]
	if !ok {
		return map[string]rolePasswordRecord{}, false, nil
	}
	return cloneRolePasswordRecords(save.Roles), false, nil
}

func mutateRoleCredentialStore(
	dataDir string,
	saveID string,
	legacy rolePasswordPayload,
	mutate func(map[string]rolePasswordRecord) error,
) error {
	return mutateRoleCredentialStoreAndCommit(dataDir, saveID, legacy, mutate, nil)
}

func mutateRoleCredentialStoreAndCommit(
	dataDir string,
	saveID string,
	legacy rolePasswordPayload,
	mutate func(map[string]rolePasswordRecord) error,
	commit func() error,
) error {
	saveID = strings.TrimSpace(saveID)
	if !validRoleCredentialSaveID(saveID) {
		return &CommandError{Code: "player_auth_save_unavailable", Message: "当前存档身份不可用，无法修改角色密码"}
	}
	lock, err := acquireRoleCredentialStoreLock(dataDir)
	if err != nil {
		return &CommandError{Code: "role_credential_store_busy", Message: "角色凭据正在被其他登录或管理操作修改，请稍后重试"}
	}
	defer lock.release()

	snapshot, err := readRoleCredentialStore(dataDir)
	if err != nil {
		return &CommandError{Code: "role_credential_store_invalid", Message: "角色凭据文件损坏或无法读取；为防止串号，登录和修改均已拒绝"}
	}
	store := cloneRoleCredentialStore(snapshot.Store)
	if !snapshot.Exists && len(legacy.Roles) > 0 {
		store.Saves[saveID] = roleCredentialSave{Roles: cloneRolePasswordRecords(legacy.Roles)}
	}
	save, ok := store.Saves[saveID]
	if !ok {
		save = roleCredentialSave{Roles: map[string]rolePasswordRecord{}}
	}
	if save.Roles == nil {
		return &CommandError{Code: "role_credential_store_invalid", Message: "角色凭据文件损坏或无法读取；为防止串号，登录和修改均已拒绝"}
	}
	if err := mutate(save.Roles); err != nil {
		return err
	}
	store.Saves[saveID] = save
	if err := writeRoleCredentialStore(dataDir, store); err != nil {
		if rollbackErr := restoreRoleCredentialStoreSnapshot(dataDir, snapshot); rollbackErr != nil {
			return playerAuthTransactionRollbackError()
		}
		return fmt.Errorf("write role credential store: %w", err)
	}
	if commit == nil {
		return nil
	}
	if err := commit(); err != nil {
		if rollbackErr := restoreRoleCredentialStoreSnapshot(dataDir, snapshot); rollbackErr != nil {
			return playerAuthTransactionRollbackError()
		}
		return err
	}
	return nil
}

func cloneRoleCredentialStore(source roleCredentialStore) roleCredentialStore {
	cloned := roleCredentialStore{
		SchemaVersion: source.SchemaVersion,
		Saves:         make(map[string]roleCredentialSave, len(source.Saves)),
	}
	for saveID, save := range source.Saves {
		cloned.Saves[saveID] = roleCredentialSave{Roles: cloneRolePasswordRecords(save.Roles)}
	}
	return cloned
}

func restoreRoleCredentialStoreSnapshot(dataDir string, snapshot roleCredentialStoreSnapshot) error {
	if snapshot.Exists {
		if err := writeRoleCredentialStore(dataDir, snapshot.Store); err != nil {
			return err
		}
		if !snapshot.MarkerExists {
			if err := os.Remove(filepath.Join(controlDir(dataDir), roleCredentialStoreMarkerFileName)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		return nil
	}
	for _, path := range []string{
		roleCredentialStorePath(dataDir),
		filepath.Join(controlDir(dataDir), roleCredentialStoreMarkerFileName),
	} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func cloneRolePasswordRecords(source map[string]rolePasswordRecord) map[string]rolePasswordRecord {
	cloned := make(map[string]rolePasswordRecord, len(source))
	for roleID, record := range source {
		cloned[roleID] = record
	}
	return cloned
}

func acquireRoleCredentialStoreLock(dataDir string) (*roleCredentialStoreLock, error) {
	dir := controlDir(dataDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	tokenBytes, err := randomBytes(24)
	if err != nil {
		return nil, err
	}
	owner := base64.RawURLEncoding.EncodeToString(tokenBytes)
	path := filepath.Join(dir, roleCredentialStoreLockFileName)
	deadline := time.Now().Add(roleCredentialStoreLockTimeout)
	for {
		file, openErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if openErr == nil {
			if _, err := file.WriteString(owner); err != nil {
				_ = file.Close()
				_ = os.Remove(path)
				return nil, err
			}
			if err := file.Sync(); err != nil {
				_ = file.Close()
				_ = os.Remove(path)
				return nil, err
			}
			if err := file.Close(); err != nil {
				_ = os.Remove(path)
				return nil, err
			}
			return &roleCredentialStoreLock{path: path, owner: owner}, nil
		}
		if !errors.Is(openErr, os.ErrExist) {
			return nil, openErr
		}
		breakStaleRoleCredentialStoreLock(path)
		if time.Now().After(deadline) {
			return nil, errors.New("timed out waiting for role credential store lock")
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func breakStaleRoleCredentialStoreLock(path string) {
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) <= roleCredentialStoreLockStaleAge {
		return
	}
	token, err := randomBytes(12)
	if err != nil {
		return
	}
	stalePath := path + ".stale-" + base64.RawURLEncoding.EncodeToString(token)
	if os.Rename(path, stalePath) == nil {
		_ = os.Remove(stalePath)
	}
}

func (lock *roleCredentialStoreLock) release() {
	if lock == nil {
		return
	}
	raw, err := os.ReadFile(lock.path)
	if err == nil && string(raw) == lock.owner {
		_ = os.Remove(lock.path)
	}
}

func writeRoleCredentialStore(dataDir string, store roleCredentialStore) error {
	if err := validateRoleCredentialStore(store); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if len(raw) > roleCredentialStoreMaxBytes {
		return errors.New("role credential store exceeds its size limit")
	}
	dir := controlDir(dataDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	token, err := randomBytes(12)
	if err != nil {
		return err
	}
	tempPath := filepath.Join(dir, roleCredentialStoreFileName+".tmp-"+base64.RawURLEncoding.EncodeToString(token))
	file, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := publishRoleCredentialStoreTemp(dataDir, tempPath); err != nil {
		return err
	}
	removeTemp = false
	return nil
}

func publishRoleCredentialStoreTemp(dataDir, tempPath string) error {
	// Publish the durable initialization marker first. If the process stops
	// before the store rename, readers fail closed instead of treating every
	// existing role as eligible for first-login enrollment again.
	if err := writeRoleCredentialStoreMarker(controlDir(dataDir)); err != nil {
		return err
	}
	return replaceRoleCredentialStoreFile(tempPath, roleCredentialStorePath(dataDir))
}

func writeRoleCredentialStoreMarker(dir string) error {
	path := filepath.Join(dir, roleCredentialStoreMarkerFileName)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		_, markerErr := readRoleCredentialStoreMarker(dir)
		return markerErr
	}
	if err != nil {
		return err
	}
	if _, err := file.WriteString("1\n"); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	return file.Close()
}
