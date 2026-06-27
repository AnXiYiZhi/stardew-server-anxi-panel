package stardew_junimo

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

const (
	maxModZipBytes       = 200 * 1024 * 1024 // 200 MB compressed
	maxModUncompressed   = 512 * 1024 * 1024 // 512 MB uncompressed total
	maxModSingleFile     = 64 * 1024 * 1024  // 64 MB per file
	restartRequiredKey   = "modsRestartRequired"
)

// modsDir returns the host-side path to the mods directory.
// Mods live at: <dataDir>/.local-container/mods/
func modsDir(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "mods")
}

// modManifest represents the SMAPI manifest.json structure.
type modManifest struct {
	Name              string `json:"Name"`
	UniqueID          string `json:"UniqueID"`
	Version           string `json:"Version"`
	Author            string `json:"Author"`
	Description       string `json:"Description"`
	MinimumApiVersion string `json:"MinimumApiVersion,omitempty"`
}

// ListMods scans the mods root directory for subdirectories, reads each manifest.json,
// and returns a list of ModInfo. Directories without a manifest are included with ParseError.
func ListMods(dataDir string) ([]registry.ModInfo, error) {
	root := modsDir(dataDir)
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read mods dir: %w", err)
	}

	var mods []registry.ModInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		folderName := e.Name()
		mod := readModInfo(filepath.Join(root, folderName), folderName)
		mods = append(mods, mod)
	}
	return mods, nil
}

// readModInfo reads a single mod directory and parses its manifest.json.
func readModInfo(modPath, folderName string) registry.ModInfo {
	info := registry.ModInfo{
		ID:         folderName,
		FolderName: folderName,
	}

	manifestPath := filepath.Join(modPath, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		info.ParseError = "缺少 manifest.json"
		return info
	}

	var m modManifest
	if err := json.Unmarshal(data, &m); err != nil {
		info.ParseError = "manifest.json 解析失败: " + err.Error()
		return info
	}

	info.UniqueID = m.UniqueID
	info.Name = m.Name
	info.Version = m.Version
	info.Author = m.Author
	info.Description = m.Description

	if info.UniqueID == "" {
		info.ParseError = "manifest.json 缺少 UniqueID"
	}
	if info.Name == "" {
		info.ParseError = "manifest.json 缺少 Name"
	}

	return info
}

// FindModByUniqueID searches the mods directory for a mod with the given UniqueID.
// Returns the folder name if found, or empty string if not found.
func FindModByUniqueID(dataDir, uniqueID string) (string, error) {
	mods, err := ListMods(dataDir)
	if err != nil {
		return "", err
	}
	for _, m := range mods {
		if m.UniqueID == uniqueID {
			return m.FolderName, nil
		}
	}
	return "", nil
}

// ValidateModName rejects dangerous mod folder names.
func ValidateModName(name string) error {
	if name == "" {
		return fmt.Errorf("mod 名称不能为空")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("mod 名称不能是 %q", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("mod 名称不能包含路径分隔符")
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("mod 名称不能是绝对路径")
	}
	return nil
}

// UploadModZip validates a mod ZIP upload and extracts it to the mods directory.
// Returns the list of imported mod folder names.
func UploadModZip(dataDir, zipPath string) ([]registry.ModInfo, error) {
	stat, err := os.Stat(zipPath)
	if err != nil {
		return nil, fmt.Errorf("stat upload: %w", err)
	}
	if stat.Size() > maxModZipBytes {
		return nil, fmt.Errorf("压缩包超过 %d MB 限制", maxModZipBytes/1024/1024)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("打开 ZIP 失败: %w", err)
	}
	defer func() { _ = zr.Close() }()

	// Security checks.
	if err := validateModZip(zr); err != nil {
		return nil, err
	}

	// Detect mod structure.
	topDirs, err := detectModTopDirs(zr)
	if err != nil {
		return nil, err
	}

	// Validate each top dir has a manifest.
	root := modsDir(dataDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create mods dir: %w", err)
	}

	// Check for duplicate UniqueIDs before extracting.
	for _, dir := range topDirs {
		if err := ValidateModName(dir); err != nil {
			return nil, fmt.Errorf("mod 目录名不合法: %w", err)
		}
	}

	// Extract to temp dir first, then validate, then move atomically.
	td, err := os.MkdirTemp("", "stardew-mod-upload-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录: %w", err)
	}
	defer func() { _ = os.RemoveAll(td) }()

	if err := extractModZip(zr, td); err != nil {
		return nil, err
	}

	// ── Pre-validation: check ALL mods before moving any ──
	type modCandidate struct {
		dir  string
		info registry.ModInfo
	}
	candidates := make([]modCandidate, 0, len(topDirs))
	seenUniqueIDs := map[string]string{} // uniqueID → folderName (within this ZIP)

	for _, dir := range topDirs {
		modPath := filepath.Join(td, dir)
		info := readModInfo(modPath, dir)
		if info.ParseError != "" {
			return nil, fmt.Errorf("mod %q 不是合法的 SMAPI Mod: %s", dir, info.ParseError)
		}

		// Check for duplicate UniqueID within this ZIP.
		if prev, dup := seenUniqueIDs[info.UniqueID]; dup {
			return nil, fmt.Errorf("ZIP 内 UniqueID %q 重复（%q 和 %q）", info.UniqueID, prev, dir)
		}
		seenUniqueIDs[info.UniqueID] = dir

		// Check for duplicate UniqueID against already-installed mods.
		existing, err := FindModByUniqueID(dataDir, info.UniqueID)
		if err != nil {
			return nil, fmt.Errorf("检查已有 Mod 失败: %w", err)
		}
		if existing != "" {
			return nil, fmt.Errorf("UniqueID %q 已存在于 Mod %q 中 (mod_exists)", info.UniqueID, existing)
		}

		// Check target directory doesn't already exist.
		dest := filepath.Join(root, dir)
		if _, err := os.Stat(dest); err == nil {
			return nil, fmt.Errorf("Mod 目录 %q 已存在", dir)
		}

		candidates = append(candidates, modCandidate{dir: dir, info: info})
	}

	// ── All checks passed — move all mods with rollback on failure ──
	var imported []registry.ModInfo
	var moved []string // tracks successfully moved dest dirs for rollback

	for _, c := range candidates {
		src := filepath.Join(td, c.dir)
		dest := filepath.Join(root, c.dir)
		if err := os.Rename(src, dest); err != nil {
			// Cross-filesystem fallback.
			if err := copyDir(src, dest); err != nil {
				// Rollback: remove all previously moved directories.
				for _, d := range moved {
					_ = os.RemoveAll(d)
				}
				return nil, fmt.Errorf("导入 Mod %q 失败: %w", c.dir, err)
			}
		}
		moved = append(moved, dest)
		finalInfo := readModInfo(dest, c.dir)
		imported = append(imported, finalInfo)
	}

	return imported, nil
}

// validateModZip performs security checks on the ZIP entries.
func validateModZip(zr *zip.ReadCloser) error {
	var totalUncompressed uint64
	for _, f := range zr.File {
		if f.FileInfo().Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("ZIP 包含符号链接，拒绝处理")
		}
		name := filepath.ToSlash(f.Name)
		if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
			return fmt.Errorf("ZIP 包含绝对路径 %q", f.Name)
		}
		// Trim trailing "/" for directory entries.
		trimmed := strings.TrimSuffix(name, "/")
		for _, seg := range strings.Split(trimmed, "/") {
			if seg == ".." {
				return fmt.Errorf("ZIP 路径 %q 包含目录穿越 (..)", f.Name)
			}
			if seg == "." {
				return fmt.Errorf("ZIP 路径 %q 包含无效的当前目录引用 (.)", f.Name)
			}
			if seg == "" {
				return fmt.Errorf("ZIP 路径 %q 包含空路径段", f.Name)
			}
		}
		totalUncompressed += f.UncompressedSize64
		if f.UncompressedSize64 > maxModSingleFile {
			return fmt.Errorf("ZIP 内单个文件超过 %d MB", maxModSingleFile/1024/1024)
		}
		if totalUncompressed > maxModUncompressed {
			return fmt.Errorf("ZIP 解压总大小超过 %d MB", maxModUncompressed/1024/1024)
		}
	}
	return nil
}

// detectModTopDirs finds the top-level directories in the ZIP.
// If the ZIP has a single top-level dir containing manifest.json, it's a single mod.
// If multiple top-level dirs exist, each must be a valid mod.
func detectModTopDirs(zr *zip.ReadCloser) ([]string, error) {
	topDirs := map[string]bool{}
	hasManifest := map[string]bool{}

	for _, f := range zr.File {
		parts := strings.SplitN(filepath.ToSlash(f.Name), "/", 3)
		if parts[0] == "" {
			continue
		}
		topDirs[parts[0]] = true
		// Check if this file is a manifest.json at the top level of a dir.
		if len(parts) >= 2 && parts[1] == "manifest.json" && (len(parts) == 2 || parts[2] == "") {
			hasManifest[parts[0]] = true
		}
	}

	if len(topDirs) == 0 {
		return nil, fmt.Errorf("ZIP 为空或没有有效文件")
	}

	// If there's exactly one top dir with manifest.json, treat as single mod.
	if len(topDirs) == 1 {
		for name := range topDirs {
			if hasManifest[name] {
				return []string{name}, nil
			}
			// Single dir without manifest — still try to extract, will fail validation later.
			return []string{name}, nil
		}
	}

	// Multiple top dirs: each must have a manifest.
	var dirs []string
	for name := range topDirs {
		if !hasManifest[name] {
			return nil, fmt.Errorf("ZIP 包含多个顶层目录，但 %q 缺少 manifest.json", name)
		}
		dirs = append(dirs, name)
	}
	return dirs, nil
}

// extractModZip extracts the ZIP to destDir with path escape verification.
func extractModZip(zr *zip.ReadCloser, destDir string) error {
	for _, f := range zr.File {
		if f.FileInfo().Mode()&fs.ModeSymlink != 0 {
			continue
		}
		outPath := filepath.Join(destDir, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(filepath.Clean(outPath)+string(os.PathSeparator), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("zip-slip 检测：路径 %q 逃逸", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(outPath, 0o755); err != nil {
				return fmt.Errorf("创建目录 %s: %w", outPath, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return fmt.Errorf("创建父目录 %s: %w", filepath.Dir(outPath), err)
		}
		if err := extractModFile(f, outPath); err != nil {
			return err
		}
	}
	return nil
}

func extractModFile(f *zip.File, outPath string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open zip entry %s: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()

	dst, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	defer func() { _ = dst.Close() }()

	lr := &io.LimitedReader{R: rc, N: maxModSingleFile + 1}
	if _, err := io.Copy(dst, lr); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	if lr.N <= 0 {
		return fmt.Errorf("文件 %s 解压后超过大小限制", f.Name)
	}
	return nil
}

// DeleteMod removes a mod folder by UniqueID or folderName.
// The target must be within the mods root directory.
func DeleteMod(dataDir, modID string) error {
	if err := ValidateModName(modID); err != nil {
		return err
	}

	root := modsDir(dataDir)
	targetPath := filepath.Join(root, modID)
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve mods root: %w", err)
	}
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("resolve mod path: %w", err)
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("mod 路径不合法: %q", modID)
	}
	// Reject if target resolves to the mods root itself.
	if absTarget == absRoot {
		return fmt.Errorf("不能删除 mods 根目录")
	}

	info, statErr := os.Stat(absTarget)
	if os.IsNotExist(statErr) {
		// Try finding by UniqueID.
		folder, findErr := FindModByUniqueID(dataDir, modID)
		if findErr != nil {
			return fmt.Errorf("查找 Mod 失败: %w", findErr)
		}
		if folder == "" {
			return fmt.Errorf("Mod %q 不存在", modID)
		}
		absTarget = filepath.Join(root, folder)
		info, statErr = os.Stat(absTarget)
		if statErr != nil {
			return fmt.Errorf("检查 Mod %q 失败: %w", folder, statErr)
		}
	}
	if statErr != nil {
		return fmt.Errorf("检查 Mod %q 失败: %w", modID, statErr)
	}
	if !info.IsDir() {
		return fmt.Errorf("Mod %q 不是目录", modID)
	}

	if err := os.RemoveAll(absTarget); err != nil {
		return fmt.Errorf("删除 Mod %q 失败: %w", modID, err)
	}
	return nil
}

// ExportModsZip creates a ZIP archive of all mods in the mods directory.
// Returns the path to the created ZIP file. Caller owns the file and must clean it up.
func ExportModsZip(dataDir string) (string, error) {
	root := modsDir(dataDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("mods 目录不存在")
		}
		return "", fmt.Errorf("读取 mods 目录: %w", err)
	}

	// Filter to directories only.
	var dirs []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		}
	}
	if len(dirs) == 0 {
		return "", fmt.Errorf("没有已安装的 Mod 可导出")
	}

	// Build a human-readable ZIP name: mod名_作者名.zip for single mod,
	// or stardew-mods-N.zip for multiple mods.
	zipName := buildModsZipName(root, dirs)
	tmpPath := filepath.Join(os.TempDir(), zipName)

	zf, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("创建 ZIP 文件: %w", err)
	}
	defer func() {
		if err != nil {
			_ = zf.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	w := zip.NewWriter(zf)
	for _, dir := range dirs {
		modPath := filepath.Join(root, dir.Name())
		err = filepath.WalkDir(modPath, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relPath, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relPath = filepath.ToSlash(relPath)

			// Skip hidden files and temp files.
			name := d.Name()
			if strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if d.IsDir() {
				// Add directory entry.
				_, err := w.Create(relPath + "/")
				return err
			}

			info, err := d.Info()
			if err != nil {
				return err
			}
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			header.Name = relPath
			header.Method = zip.Deflate

			writer, err := w.CreateHeader(header)
			if err != nil {
				return err
			}
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer func() { _ = file.Close() }()
			_, err = io.Copy(writer, file)
			return err
		})
		if err != nil {
			break
		}
	}

	if err := w.Close(); err != nil {
		_ = zf.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("关闭 ZIP: %w", err)
	}
	if err := zf.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("关闭文件: %w", err)
	}

	return tmpPath, nil
}

// buildModsZipName constructs a human-readable ZIP filename for a mods export.
// Single mod: "mod名_作者名.zip"
// Multiple mods: "stardew-mods-N.zip"
func buildModsZipName(root string, dirs []os.DirEntry) string {
	if len(dirs) == 1 {
		info := readModInfo(filepath.Join(root, dirs[0].Name()), dirs[0].Name())
		parts := []string{}
		if info.Name != "" {
			parts = append(parts, sanitizeFileNamePart(info.Name))
		}
		if info.Author != "" {
			parts = append(parts, sanitizeFileNamePart(info.Author))
		}
		if len(parts) > 0 {
			return strings.Join(parts, "_") + ".zip"
		}
		return dirs[0].Name() + ".zip"
	}
	return fmt.Sprintf("stardew-mods-%d.zip", len(dirs))
}

// sanitizeFileNamePart removes characters that are unsafe in filenames.
func sanitizeFileNamePart(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, `\`, "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "?", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "<", "")
	s = strings.ReplaceAll(s, ">", "")
	s = strings.ReplaceAll(s, "|", "")
	return s
}

// GetModsRestartRequired reads the restart-required flag from the driver payload.
func GetModsRestartRequired(dataDir string) bool {
	// Read from a simple flag file.
	flagPath := filepath.Join(modsDir(dataDir), ".restart-required")
	_, err := os.Stat(flagPath)
	return err == nil
}

// SetModsRestartRequired writes the restart-required flag.
func SetModsRestartRequired(dataDir string) error {
	flagPath := filepath.Join(modsDir(dataDir), ".restart-required")
	if err := os.MkdirAll(filepath.Dir(flagPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(flagPath, []byte(time.Now().Format(time.RFC3339)), 0o644)
}

// ClearModsRestartRequired removes the restart-required flag.
func ClearModsRestartRequired(dataDir string) error {
	flagPath := filepath.Join(modsDir(dataDir), ".restart-required")
	_ = os.Remove(flagPath)
	return nil
}

// migrateModsCompose adds the mods bind mount to docker-compose.yml if missing.
// Returns true if the file was changed.
func migrateModsCompose(composePath string) (bool, error) {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return false, err
	}
	content := string(data)

	// Check if the mods mount already exists.
	if strings.Contains(content, "./.local-container/mods:/data/Mods") {
		return false, nil
	}

	// Add the mods mount after the SMAPI control mod mount.
	oldLine := "      - ./.local-container/mods/StardewAnxiPanel.Control:/data/Mods/StardewAnxiPanel.Control"
	newLines := oldLine + "\n      - ./.local-container/mods:/data/Mods"
	content = strings.Replace(content, oldLine, newLines, 1)

	// If the SMAPI control mount doesn't exist, add after settings mount.
	if !strings.Contains(content, "./.local-container/mods:/data/Mods") {
		oldLine = "      - ./.local-container/settings:/data/settings"
		newLines = oldLine + "\n      - ./.local-container/mods:/data/Mods"
		content = strings.Replace(content, oldLine, newLines, 1)
	}

	if err := os.WriteFile(composePath, []byte(content), 0o644); err != nil {
		return false, err
	}
	return true, nil
}
