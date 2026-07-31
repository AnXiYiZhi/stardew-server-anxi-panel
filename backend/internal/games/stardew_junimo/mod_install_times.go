package stardew_junimo

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

// Mod installs can be started by independent HTTP/job entry points. Serialize
// the small sidecar read-modify-write cycle so two successful installs cannot
// overwrite each other's timestamps.
var modInstallTimesMu sync.Mutex

const (
	modInstallTimesSchemaVersion = 1
	maxModInstallTimesBytes      = 2 * 1024 * 1024
)

type modInstallTimeEntry struct {
	UniqueID    string `json:"uniqueId,omitempty"`
	InstalledAt string `json:"installedAt"`
}

type modInstallTimesStore struct {
	SchemaVersion int                            `json:"schemaVersion"`
	Mods          map[string]modInstallTimeEntry `json:"mods"`
}

func modInstallTimesFilePath(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "control", "mod-install-times.json")
}

func loadModInstallTimesStore(dataDir string) (modInstallTimesStore, error) {
	path := modInstallTimesFilePath(dataDir)
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return newModInstallTimesStore(), nil
	}
	if err != nil {
		return modInstallTimesStore{}, fmt.Errorf("read mod install times: %w", err)
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, maxModInstallTimesBytes+1))
	if err != nil {
		return modInstallTimesStore{}, fmt.Errorf("read mod install times: %w", err)
	}
	if len(data) > maxModInstallTimesBytes {
		return modInstallTimesStore{}, fmt.Errorf("mod install times file exceeds %d bytes", maxModInstallTimesBytes)
	}

	var store modInstallTimesStore
	if err := json.Unmarshal(data, &store); err != nil {
		return modInstallTimesStore{}, fmt.Errorf("parse mod install times: %w", err)
	}
	if store.SchemaVersion != modInstallTimesSchemaVersion {
		return modInstallTimesStore{}, fmt.Errorf("unsupported mod install times schema version %d", store.SchemaVersion)
	}
	if store.Mods == nil {
		store.Mods = map[string]modInstallTimeEntry{}
	}
	return store, nil
}

func newModInstallTimesStore() modInstallTimesStore {
	return modInstallTimesStore{
		SchemaVersion: modInstallTimesSchemaVersion,
		Mods:          map[string]modInstallTimeEntry{},
	}
}

func saveModInstallTimesStore(dataDir string, store modInstallTimesStore) error {
	store.SchemaVersion = modInstallTimesSchemaVersion
	if store.Mods == nil {
		store.Mods = map[string]modInstallTimeEntry{}
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mod install times: %w", err)
	}
	if len(data) > maxModInstallTimesBytes {
		return fmt.Errorf("mod install times file exceeds %d bytes", maxModInstallTimesBytes)
	}

	path := modInstallTimesFilePath(dataDir)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create mod install times directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".mod-install-times-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create mod install times temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("secure mod install times temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write mod install times temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync mod install times temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close mod install times temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("publish mod install times: %w", err)
	}
	return nil
}

func recordModInstallTimes(dataDir string, mods []registry.ModInfo, installedAt time.Time) ([]registry.ModInfo, error) {
	if len(mods) == 0 {
		return mods, nil
	}
	modInstallTimesMu.Lock()
	defer modInstallTimesMu.Unlock()
	store, err := loadModInstallTimesStore(dataDir)
	if err != nil {
		return nil, err
	}
	timestamp := installedAt.UTC().Format(time.RFC3339Nano)
	result := append([]registry.ModInfo(nil), mods...)
	for i := range result {
		folderName := strings.TrimSpace(result[i].FolderName)
		if folderName == "" {
			return nil, fmt.Errorf("record mod install time: empty folder name")
		}
		store.Mods[folderName] = modInstallTimeEntry{
			UniqueID:    strings.TrimSpace(result[i].UniqueID),
			InstalledAt: timestamp,
		}
		result[i].InstalledAt = timestamp
	}
	if err := saveModInstallTimesStore(dataDir, store); err != nil {
		return nil, err
	}
	return result, nil
}

func applyModInstallTimes(dataDir string, mods []registry.ModInfo) []registry.ModInfo {
	store, err := loadModInstallTimesStore(dataDir)
	if err != nil || len(store.Mods) == 0 {
		return mods
	}
	for i := range mods {
		entry, ok := store.Mods[mods[i].FolderName]
		if !ok {
			continue
		}
		if entry.UniqueID != "" && mods[i].UniqueID != "" && !strings.EqualFold(entry.UniqueID, mods[i].UniqueID) {
			continue
		}
		parsed, err := time.Parse(time.RFC3339Nano, entry.InstalledAt)
		if err != nil || parsed.IsZero() {
			continue
		}
		mods[i].InstalledAt = parsed.UTC().Format(time.RFC3339Nano)
	}
	return mods
}

func deleteModInstallTimes(dataDir string, folderNames []string) error {
	if len(folderNames) == 0 {
		return nil
	}
	modInstallTimesMu.Lock()
	defer modInstallTimesMu.Unlock()
	store, err := loadModInstallTimesStore(dataDir)
	if err != nil {
		return err
	}
	changed := false
	for _, folderName := range folderNames {
		if _, ok := store.Mods[folderName]; ok {
			delete(store.Mods, folderName)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return saveModInstallTimesStore(dataDir, store)
}
