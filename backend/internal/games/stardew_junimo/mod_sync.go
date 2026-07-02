package stardew_junimo

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

// controlModFolderName is the SMAPI mod folder name of the panel's own
// server-side control mod. It must never be included in a player sync pack
// export, regardless of its stored classification.
const controlModFolderName = "StardewAnxiPanel.Control"
const controlModUniqueID = "AnXiYiZhi.StardewAnxiPanel.Control"

// ErrNoSyncMods is returned by ExportModSyncPackZip when no installed mod is
// classified as client_required.
var ErrNoSyncMods = errors.New("没有玩家需同步的 Mod 可导出")

// PlayerSyncPackFileName is the stable download filename presented to callers
// of ExportModSyncPackZip, independent of the unique on-disk temp file name.
const PlayerSyncPackFileName = "stardew-player-sync-pack.zip"

// PlayerModUpdatePackFileName is the stable download filename for the
// lightweight pack intended for players who have already installed SMAPI once.
const PlayerModUpdatePackFileName = "stardew-player-mods-update-pack.zip"

const playerSyncPackVersion = "2"
const playerSyncStateDirName = ".anxi-sync"

const (
	playerSyncPackTypeFull       = "full"
	playerSyncPackTypeModsUpdate = "mods_update"
)

var contentPackFolderPrefixes = []string{
	"[CP]", "[AT]", "[JA]", "[MFM]", "[FTM]", "[TMX]", "[DGA]",
	"[PFM]", "[CFR]", "[STF]", "[FS]", "[BFAV]", "[HD]", "[HDP]",
}

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

// defaultModSyncKind returns the classification a mod should have when only
// the folder name is known. Full ModInfo-based inference happens in
// inferDefaultModSyncClassification.
func defaultModSyncKind(folderName string) string {
	if folderName == controlModFolderName {
		return registry.ModSyncKindServerOnly
	}
	return registry.ModSyncKindClientRequired
}

func isControlModInfo(mod registry.ModInfo) bool {
	return mod.FolderName == controlModFolderName ||
		strings.EqualFold(mod.UniqueID, controlModUniqueID)
}

func isSMAPIRuntimeMod(mod registry.ModInfo) bool {
	return mod.ID == smapiRuntimeModID ||
		strings.EqualFold(mod.UniqueID, "Pathoschild.SMAPI")
}

func modCountsInPlayerSync(mod registry.ModInfo) bool {
	if isSMAPIRuntimeMod(mod) {
		return true
	}
	return !mod.BuiltIn
}

func modHasPackagedSyncFiles(mod registry.ModInfo) bool {
	return !mod.BuiltIn && !isControlModInfo(mod) && !isSMAPIRuntimeMod(mod)
}

func inferDefaultModSyncClassification(mod registry.ModInfo) (string, string) {
	if isControlModInfo(mod) {
		return registry.ModSyncKindServerOnly, "自动识别：面板服务器控制组件"
	}
	if mod.IsContentPack || mod.ContentPackFor != "" || hasContentPackPrefix(mod.FolderName) || hasContentPackPrefix(mod.Name) {
		return registry.ModSyncKindClientRequired, "自动识别：内容包需要玩家同步"
	}
	return registry.ModSyncKindClientRequired, "自动识别：第三方 Mod 默认需要玩家同步"
}

func hasContentPackPrefix(name string) bool {
	upper := strings.ToUpper(strings.TrimSpace(name))
	for _, prefix := range contentPackFolderPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
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
	if info, err := os.Stat(filepath.Join(modsDir(dataDir), modID)); err == nil && info.IsDir() {
		return modID, nil
	}
	if info, err := os.Stat(filepath.Join(disabledModsDir(dataDir), modID)); err == nil && info.IsDir() {
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
		if mods[i].BuiltIn {
			if mods[i].SyncKind == "" {
				mods[i].SyncKind = registry.ModSyncKindServerOnly
			}
			continue
		}
		folderName := mods[i].FolderName
		if err == nil {
			if entry, ok := store.Mods[folderName]; ok && registry.ValidModSyncKind(entry.SyncKind) {
				mods[i].SyncKind = entry.SyncKind
				mods[i].SyncNote = entry.SyncNote
				continue
			}
		}
		mods[i].SyncKind, mods[i].SyncNote = inferDefaultModSyncClassification(mods[i])
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
	mods = ApplyNexusMetadataToMods(dataDir, mods)

	summary := registry.ModSyncSummary{}
	for _, m := range mods {
		if !modCountsInPlayerSync(m) {
			continue
		}
		summary.Total++
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
// pack-manifest.json.
type playerSyncManifestMod struct {
	UniqueID   string `json:"uniqueId"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	FolderName string `json:"folderName"`
	SyncKind   string `json:"syncKind"`
	BuiltIn    bool   `json:"builtIn,omitempty"`
	Packaged   bool   `json:"packaged"`
}

type playerSyncSMAPIManifest struct {
	Required      bool   `json:"required"`
	UniqueID      string `json:"uniqueId"`
	Name          string `json:"name"`
	Version       string `json:"version,omitempty"`
	NexusModID    int    `json:"nexusModId,omitempty"`
	PageURL       string `json:"pageUrl,omitempty"`
	Bundled       bool   `json:"bundled"`
	InstallerFile string `json:"installerFile,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
}

type playerSyncPackManifest struct {
	PackID       string                  `json:"packId"`
	PackVersion  string                  `json:"packVersion"`
	PackType     string                  `json:"packType"`
	ExportedAt   string                  `json:"exportedAt"`
	StateDirName string                  `json:"stateDirName"`
	ChecksumFile string                  `json:"checksumFile"`
	Mods         []playerSyncManifestMod `json:"mods"`
	SMAPI        playerSyncSMAPIManifest `json:"smapi"`
}

type syncPackChecksum struct {
	Path string
	SHA  string
}

type bundledSMAPIInstaller struct {
	Path     string
	FileName string
}

type modSyncPackExportOptions struct {
	packType           string
	includeSMAPIBundle bool
	includeSteamTools  bool
	tempPattern        string
	packIDPrefix       string
	installBatchName   string
	uninstallBatchName string
	readme             string
	requirePackagedMod bool
}

// ExportModSyncPackZip creates a ZIP archive containing only the mods
// classified as client_required, plus a Windows installer/uninstaller toolset,
// pack-manifest.json, and checksums.sha256. The panel's own server-side
// control mod is always excluded, regardless of its stored classification.
// Returns the path to the created ZIP file; caller owns the file and must clean
// it up.
func ExportModSyncPackZip(dataDir string) (string, error) {
	return exportModSyncPackZip(dataDir, modSyncPackExportOptions{
		packType:           playerSyncPackTypeFull,
		includeSMAPIBundle: true,
		includeSteamTools:  true,
		tempPattern:        "stardew-player-sync-pack-*.zip",
		packIDPrefix:       "stardew-player-sync-",
		installBatchName:   "安装玩家同步包.bat",
		uninstallBatchName: "卸载本同步包.bat",
		readme:             playerSyncReadme,
	})
}

// ExportModSyncUpdatePackZip creates a lightweight ZIP for players who already
// ran the full sync pack. It carries only client_required mod payloads and
// installer tools; SMAPI is treated as a pre-existing client requirement.
func ExportModSyncUpdatePackZip(dataDir string) (string, error) {
	return exportModSyncPackZip(dataDir, modSyncPackExportOptions{
		packType:           playerSyncPackTypeModsUpdate,
		includeSMAPIBundle: false,
		includeSteamTools:  false,
		tempPattern:        "stardew-player-mods-update-pack-*.zip",
		packIDPrefix:       "stardew-player-mods-update-",
		installBatchName:   "安装模组更新.bat",
		uninstallBatchName: "卸载本次模组更新.bat",
		readme:             playerModsUpdateReadme,
		requirePackagedMod: true,
	})
}

func exportModSyncPackZip(dataDir string, opts modSyncPackExportOptions) (string, error) {
	root := modsDir(dataDir)
	mods, err := ListMods(dataDir)
	if err != nil {
		return "", err
	}
	mods = ApplyModSyncClassification(dataDir, mods)
	mods = ApplyNexusMetadataToMods(dataDir, mods)

	var selected []registry.ModInfo
	for _, m := range mods {
		if isControlModInfo(m) {
			continue
		}
		if m.SyncKind != registry.ModSyncKindClientRequired {
			continue
		}
		if m.BuiltIn && !isSMAPIRuntimeMod(m) {
			continue
		}
		selected = append(selected, m)
	}
	if len(selected) == 0 {
		return "", ErrNoSyncMods
	}
	if opts.requirePackagedMod {
		hasPackagedMod := false
		for _, m := range selected {
			if modHasPackagedSyncFiles(m) {
				hasPackagedMod = true
				break
			}
		}
		if !hasPackagedMod {
			return "", ErrNoSyncMods
		}
	}

	zf, err := os.CreateTemp("", opts.tempPattern)
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
	var checksums []syncPackChecksum
	for _, m := range selected {
		if !modHasPackagedSyncFiles(m) {
			continue
		}
		if err = addModDirToZipAt(w, root, m.FolderName, "payload/mods", &checksums); err != nil {
			return "", fmt.Errorf("写入 Mod %q 失败: %w", m.FolderName, err)
		}
	}

	smapi := playerSyncSMAPIManifest{
		Required:   true,
		UniqueID:   "Pathoschild.SMAPI",
		Name:       "SMAPI",
		NexusModID: smapiNexusModID,
		PageURL:    nexusModURL(smapiNexusModID),
	}
	if opts.includeSMAPIBundle {
		smapiInstaller, err := findBundledSMAPIInstaller(dataDir)
		if err != nil {
			return "", err
		}
		if smapiInstaller != nil {
			zipPath := path.Join("payload/smapi", smapiInstaller.FileName)
			sha, err := addFileToZip(w, smapiInstaller.Path, zipPath)
			if err != nil {
				return "", fmt.Errorf("写入 SMAPI 安装包失败: %w", err)
			}
			checksums = append(checksums, syncPackChecksum{Path: zipPath, SHA: sha})
			smapi.Bundled = true
			smapi.InstallerFile = smapiInstaller.FileName
			smapi.SHA256 = sha
		}
		if err = writeZipBytes(w, "payload/smapi/smapi.json", mustJSON(smapi)); err != nil {
			return "", fmt.Errorf("写入 SMAPI 元数据失败: %w", err)
		}
	}
	if err = writePackManifest(w, selected, smapi, opts); err != nil {
		return "", fmt.Errorf("写入同步包清单失败: %w", err)
	}
	if err = writeInstallerTools(w, opts); err != nil {
		return "", fmt.Errorf("写入安装脚本失败: %w", err)
	}
	if err = writeChecksums(w, checksums); err != nil {
		return "", fmt.Errorf("写入校验文件失败: %w", err)
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

func writePackManifest(w *zip.Writer, mods []registry.ModInfo, smapi playerSyncSMAPIManifest, opts modSyncPackExportOptions) error {
	now := time.Now().UTC()
	manifest := playerSyncPackManifest{
		PackID:       opts.packIDPrefix + now.Format("20060102T150405Z"),
		PackVersion:  playerSyncPackVersion,
		PackType:     opts.packType,
		ExportedAt:   now.Format(time.RFC3339),
		StateDirName: playerSyncStateDirName,
		ChecksumFile: "checksums.sha256",
		SMAPI:        smapi,
	}
	for _, m := range mods {
		manifest.Mods = append(manifest.Mods, playerSyncManifestMod{
			UniqueID:   m.UniqueID,
			Name:       m.Name,
			Version:    m.Version,
			FolderName: m.FolderName,
			SyncKind:   m.SyncKind,
			BuiltIn:    m.BuiltIn,
			Packaged:   modHasPackagedSyncFiles(m),
		})
	}
	return writeZipBytes(w, "pack-manifest.json", mustJSON(manifest))
}

func mustJSON(v any) []byte {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return append(data, '\n')
}

func findBundledSMAPIInstaller(dataDir string) (*bundledSMAPIInstaller, error) {
	candidates := []string{
		filepath.Join(dataDir, ".local-container", "smapi"),
		filepath.Join(dataDir, ".local-container", "control", "smapi"),
		filepath.Join(dataDir, "smapi"),
	}
	var matches []string
	for _, dir := range candidates {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("读取 SMAPI 安装包目录失败: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			lower := strings.ToLower(name)
			if strings.HasSuffix(lower, ".zip") && strings.Contains(lower, "smapi") {
				matches = append(matches, filepath.Join(dir, name))
			}
		}
	}
	if len(matches) == 0 {
		return nil, nil
	}
	sort.Strings(matches)
	chosen := matches[len(matches)-1]
	return &bundledSMAPIInstaller{Path: chosen, FileName: filepath.Base(chosen)}, nil
}

func writeInstallerTools(w *zip.Writer, opts modSyncPackExportOptions) error {
	files := map[string]string{
		opts.installBatchName:   installBatchScript,
		opts.uninstallBatchName: uninstallBatchScript,
		"README.txt":            opts.readme,
		"tools/install.ps1":     installPowerShellScript,
		"tools/uninstall.ps1":   uninstallPowerShellScript,
		"tools/vdf.ps1":         vdfPowerShellScript,
	}
	if opts.includeSteamTools {
		files["tools/steam-launch-options.ps1"] = steamLaunchOptionsPowerShellScript
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		data := []byte(files[name])
		if strings.HasSuffix(strings.ToLower(name), ".ps1") {
			data = append([]byte{0xEF, 0xBB, 0xBF}, data...)
		}
		if err := writeZipBytes(w, name, data); err != nil {
			return err
		}
	}
	return nil
}

func writeChecksums(w *zip.Writer, checksums []syncPackChecksum) error {
	sort.Slice(checksums, func(i, j int) bool { return checksums[i].Path < checksums[j].Path })
	var b strings.Builder
	for _, item := range checksums {
		b.WriteString(item.SHA)
		b.WriteString("  ")
		b.WriteString(item.Path)
		b.WriteByte('\n')
	}
	return writeZipBytes(w, "checksums.sha256", []byte(b.String()))
}

func addModDirToZipAt(w *zip.Writer, root, dirName, zipBase string, checksums *[]syncPackChecksum) error {
	modPath := filepath.Join(root, dirName)
	return filepath.WalkDir(modPath, func(pathOnDisk string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(root, pathOnDisk)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		name := d.Name()
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		zipName := path.Join(zipBase, relPath)
		if d.IsDir() {
			_, err := w.Create(zipName + "/")
			return err
		}
		sha, err := addFileToZip(w, pathOnDisk, zipName)
		if err != nil {
			return err
		}
		if checksums != nil {
			*checksums = append(*checksums, syncPackChecksum{Path: zipName, SHA: sha})
		}
		return nil
	})
}

func addFileToZip(w *zip.Writer, diskPath, zipName string) (string, error) {
	info, err := os.Stat(diskPath)
	if err != nil {
		return "", err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return "", err
	}
	header.Name = filepath.ToSlash(zipName)
	header.Method = zip.Deflate

	writer, err := w.CreateHeader(header)
	if err != nil {
		return "", err
	}
	file, err := os.Open(diskPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	h := sha256.New()
	if _, err := copyAndHash(writer, file, h); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func copyAndHash(dst io.Writer, src io.Reader, h hash.Hash) (int64, error) {
	return io.Copy(io.MultiWriter(dst, h), src)
}

func writeZipBytes(w *zip.Writer, name string, data []byte) error {
	fw, err := w.Create(name)
	if err != nil {
		return err
	}
	_, err = fw.Write(data)
	return err
}
