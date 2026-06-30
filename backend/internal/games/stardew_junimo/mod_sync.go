package stardew_junimo

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

// controlModFolderName is the SMAPI mod folder name of the panel's own
// server-side control mod. It must never be included in a player sync pack
// export, regardless of its stored classification.
const controlModFolderName = "StardewAnxiPanel.Control"

// ErrNoSyncMods is returned by ExportModSyncPackZip when no installed mod is
// classified as client_required.
var ErrNoSyncMods = errors.New("没有玩家需同步的 Mod 可导出")

// PlayerSyncPackFileName is the stable download filename presented to callers
// of ExportModSyncPackZip, independent of the unique on-disk temp file name.
const PlayerSyncPackFileName = "stardew-player-sync-pack.zip"

// modSyncFilePath returns the host-side path to the panel's own mod sync
// classification file. This is panel metadata; it is never written into a
// mod's own manifest.json.
func modSyncFilePath(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "control", "mod-sync.json")
}

type modSyncEntry struct {
	SyncKind string `json:"syncKind"`
	SyncNote string `json:"syncNote,omitempty"`
}

type modSyncStore struct {
	Mods map[string]modSyncEntry `json:"mods"`
}

// loadModSyncStore reads the classification file. A missing file is not an
// error: it simply means no mod has been classified yet.
func loadModSyncStore(dataDir string) (modSyncStore, error) {
	data, err := os.ReadFile(modSyncFilePath(dataDir))
	if os.IsNotExist(err) {
		return modSyncStore{Mods: map[string]modSyncEntry{}}, nil
	}
	if err != nil {
		return modSyncStore{}, fmt.Errorf("read mod sync file: %w", err)
	}
	var s modSyncStore
	if err := json.Unmarshal(data, &s); err != nil {
		return modSyncStore{}, fmt.Errorf("parse mod sync file: %w", err)
	}
	if s.Mods == nil {
		s.Mods = map[string]modSyncEntry{}
	}
	return s, nil
}

// saveModSyncStore persists atomically: write to a temp file in the same
// directory, then rename over the target, so a crash mid-write never leaves a
// truncated mod-sync.json behind.
func saveModSyncStore(dataDir string, s modSyncStore) error {
	path := modSyncFilePath(dataDir)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create control dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mod sync file: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".mod-sync-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp mod sync file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp mod sync file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp mod sync file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename mod sync file: %w", err)
	}
	return nil
}

// defaultModSyncKind returns the classification a mod should have when no
// explicit entry exists in the sync file yet.
func defaultModSyncKind(folderName string) string {
	if folderName == controlModFolderName {
		return registry.ModSyncKindServerOnly
	}
	return registry.ModSyncKindUnknown
}

// GetModSyncClassification returns the stored sync classification for a mod
// folder, falling back to the default when no entry exists or the stored
// value is no longer a recognized kind.
func GetModSyncClassification(dataDir, folderName string) (syncKind string, syncNote string) {
	store, err := loadModSyncStore(dataDir)
	if err != nil {
		return defaultModSyncKind(folderName), ""
	}
	entry, ok := store.Mods[folderName]
	if !ok || !registry.ValidModSyncKind(entry.SyncKind) {
		return defaultModSyncKind(folderName), ""
	}
	return entry.SyncKind, entry.SyncNote
}

// modSyncLocksMu guards modSyncLocks, the registry of per-dataDir mutexes
// used to serialize SetModSyncClassification's load-modify-save sequence.
var (
	modSyncLocksMu sync.Mutex
	modSyncLocks   = map[string]*sync.Mutex{}
)

// modSyncLockFor returns the mutex guarding mod-sync.json for a given data
// directory, creating it on first use.
func modSyncLockFor(dataDir string) *sync.Mutex {
	modSyncLocksMu.Lock()
	defer modSyncLocksMu.Unlock()
	lock, ok := modSyncLocks[dataDir]
	if !ok {
		lock = &sync.Mutex{}
		modSyncLocks[dataDir] = lock
	}
	return lock
}

// SetModSyncClassification persists the sync classification for a mod
// folder. It only writes the panel's own metadata file and never touches the
// mod's manifest.json, so it is safe to call regardless of server run state.
// The load-modify-save sequence is serialized per dataDir so two concurrent
// classification updates cannot race and silently drop one another.
func SetModSyncClassification(dataDir, folderName, syncKind, syncNote string) error {
	if !registry.ValidModSyncKind(syncKind) {
		return fmt.Errorf("无效的同步分类 %q", syncKind)
	}
	lock := modSyncLockFor(dataDir)
	lock.Lock()
	defer lock.Unlock()

	store, err := loadModSyncStore(dataDir)
	if err != nil {
		return err
	}
	store.Mods[folderName] = modSyncEntry{SyncKind: syncKind, SyncNote: syncNote}
	return saveModSyncStore(dataDir, store)
}

// ResolveModFolder resolves a mod identifier (folder name or UniqueID) to its
// on-disk folder name within the mods directory. Mirrors the lookup order
// used by DeleteMod.
func ResolveModFolder(dataDir, modID string) (string, error) {
	if err := ValidateModName(modID); err != nil {
		return "", err
	}
	root := modsDir(dataDir)
	if info, err := os.Stat(filepath.Join(root, modID)); err == nil && info.IsDir() {
		return modID, nil
	}
	folder, err := FindModByUniqueID(dataDir, modID)
	if err != nil {
		return "", err
	}
	if folder == "" {
		return "", fmt.Errorf("Mod %q 不存在", modID)
	}
	return folder, nil
}

// ApplyModSyncClassification fills in SyncKind/SyncNote on each ModInfo based
// on the persisted classification file. Mods without a stored entry get the
// default classification.
func ApplyModSyncClassification(dataDir string, mods []registry.ModInfo) []registry.ModInfo {
	store, err := loadModSyncStore(dataDir)
	for i := range mods {
		folderName := mods[i].FolderName
		if err == nil {
			if entry, ok := store.Mods[folderName]; ok && registry.ValidModSyncKind(entry.SyncKind) {
				mods[i].SyncKind = entry.SyncKind
				mods[i].SyncNote = entry.SyncNote
				continue
			}
		}
		mods[i].SyncKind = defaultModSyncKind(folderName)
		mods[i].SyncNote = ""
	}
	return mods
}

// BuildModSyncPlan lists installed mods together with their sync
// classification and a summary count by classification.
func BuildModSyncPlan(dataDir string) (registry.ModSyncPlanResult, error) {
	mods, err := ListMods(dataDir)
	if err != nil {
		return registry.ModSyncPlanResult{}, err
	}
	mods = ApplyModSyncClassification(dataDir, mods)

	summary := registry.ModSyncSummary{Total: len(mods)}
	for _, m := range mods {
		switch m.SyncKind {
		case registry.ModSyncKindServerOnly:
			summary.ServerOnly++
		case registry.ModSyncKindClientRequired:
			summary.ClientRequired++
		default:
			summary.Unknown++
		}
	}

	return registry.ModSyncPlanResult{Mods: mods, Summary: summary}, nil
}

// playerSyncManifestMod describes one mod entry in the exported sync pack's
// player-sync-manifest.json.
type playerSyncManifestMod struct {
	UniqueID   string `json:"uniqueId"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	FolderName string `json:"folderName"`
	SyncKind   string `json:"syncKind"`
}

type playerSyncManifest struct {
	ExportedAt string                  `json:"exportedAt"`
	Mods       []playerSyncManifestMod `json:"mods"`
}

// ExportModSyncPackZip creates a ZIP archive containing only the mods
// classified as client_required, plus a player-sync-manifest.json describing
// the contents. The panel's own server-side control mod is always excluded,
// regardless of its stored classification. Returns the path to the created
// ZIP file; caller owns the file and must clean it up.
func ExportModSyncPackZip(dataDir string) (string, error) {
	root := modsDir(dataDir)
	mods, err := ListMods(dataDir)
	if err != nil {
		return "", err
	}
	mods = ApplyModSyncClassification(dataDir, mods)

	var selected []registry.ModInfo
	for _, m := range mods {
		if m.FolderName == controlModFolderName {
			continue
		}
		if m.SyncKind != registry.ModSyncKindClientRequired {
			continue
		}
		selected = append(selected, m)
	}
	if len(selected) == 0 {
		return "", ErrNoSyncMods
	}

	zf, err := os.CreateTemp("", "stardew-player-sync-pack-*.zip")
	if err != nil {
		return "", fmt.Errorf("创建 ZIP 文件: %w", err)
	}
	tmpPath := zf.Name()
	defer func() {
		if err != nil {
			_ = zf.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	w := zip.NewWriter(zf)
	for _, m := range selected {
		if err = addModDirToZip(w, root, m.FolderName); err != nil {
			return "", fmt.Errorf("写入 Mod %q 失败: %w", m.FolderName, err)
		}
	}
	if err = writeSyncManifest(w, selected); err != nil {
		return "", fmt.Errorf("写入同步说明文件失败: %w", err)
	}

	if closeErr := w.Close(); closeErr != nil {
		err = closeErr
		return "", fmt.Errorf("关闭 ZIP: %w", closeErr)
	}
	if closeErr := zf.Close(); closeErr != nil {
		err = closeErr
		return "", fmt.Errorf("关闭文件: %w", closeErr)
	}

	return tmpPath, nil
}

func writeSyncManifest(w *zip.Writer, mods []registry.ModInfo) error {
	manifest := playerSyncManifest{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
	}
	for _, m := range mods {
		manifest.Mods = append(manifest.Mods, playerSyncManifestMod{
			UniqueID:   m.UniqueID,
			Name:       m.Name,
			Version:    m.Version,
			FolderName: m.FolderName,
			SyncKind:   m.SyncKind,
		})
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	fw, err := w.Create("player-sync-manifest.json")
	if err != nil {
		return err
	}
	_, err = fw.Write(data)
	return err
}
