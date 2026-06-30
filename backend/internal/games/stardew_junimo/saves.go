package stardew_junimo

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

const (
	maxUploadZipBytes    = 100 * 1024 * 1024 // 100 MB compressed
	maxUncompressedBytes = 512 * 1024 * 1024 // 512 MB uncompressed total
	maxSingleFileBytes   = 64 * 1024 * 1024  // 64 MB per file
)

// savesDir returns the host-side path to the bind-mounted saves directory.
// Stardew saves live at: <savesDir>/Saves/<SaveFolderName>/
func savesDir(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "saves")
}

// SetActiveSave writes the JunimoServer gameloader config so the given save is
// loaded on next startup.  This does not require the server to be running.
func SetActiveSave(dataDir, saveName string) error {
	if err := validateSaveName(saveName); err != nil {
		return fmt.Errorf("存档名称不合法: %w", err)
	}
	cfgDir := filepath.Join(savesDir(dataDir), ".smapi", "mod-data", "junimohost.server")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return fmt.Errorf("create gameloader dir: %w", err)
	}
	obj := map[string]string{"SaveNameToLoad": saveName}
	data, err := marshalJSON(obj)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cfgDir, "junimohost.gameloader.json"), data, 0o644)
}

// savesTemplatesDir returns where save templates should be placed.
func savesTemplatesDir(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "saves-templates")
}

// serverSettingsPath returns where the server-settings.json lives.
func serverSettingsPath(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "settings", "server-settings.json")
}

// controlDir is the host-side directory shared with the panel control mod.
func controlDir(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "control")
}

// DeleteAllSaves removes every save folder under <savesDir>/Saves/ and the SMAPI
// cache so JunimoServer creates a brand-new game on next start.
func DeleteAllSaves(dataDir string) error {
	savesPath := filepath.Join(savesDir(dataDir), "Saves")
	entries, err := os.ReadDir(savesPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			if err := os.RemoveAll(filepath.Join(savesPath, e.Name())); err != nil {
				return fmt.Errorf("delete save %s: %w", e.Name(), err)
			}
		}
	}
	// Also clear SMAPI mod cache that remembers the last-loaded save name.
	smaCacheDir := filepath.Join(savesDir(dataDir), ".smapi")
	_ = os.RemoveAll(smaCacheDir)

	// Clear gameloader config so JunimoServer doesn't try to load a deleted save.
	gameloaderPath := filepath.Join(savesDir(dataDir), ".smapi", "mod-data", "junimohost.server", "junimohost.gameloader.json")
	_ = os.Remove(gameloaderPath)
	return nil
}

// listSaveDirs returns each save folder name found under <savesDir>/Saves/.
func listSaveDirs(dataDir string) ([]string, error) {
	savesPath := filepath.Join(savesDir(dataDir), "Saves")
	entries, err := os.ReadDir(savesPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// readSaveInfo reads metadata from a single save folder and returns a SaveInfo.
// On XML parse error, ParseError is set and other fields are best-effort.
// Supports two XML structures:
//   - <SaveGame> with nested <player> (full save file)
//   - <Farmer> with direct fields (Junimo/SaveGameInfo format)
func readSaveInfo(saveFolder string) registry.SaveInfo {
	name := filepath.Base(saveFolder)
	info := registry.SaveInfo{Name: name}

	// Try to get modification time and size from the main save file.
	mainFile := filepath.Join(saveFolder, name)
	if stat, err := os.Stat(mainFile); err == nil {
		info.FileSizeBytes = stat.Size()
		info.ModifiedAt = stat.ModTime().UTC().Format(time.RFC3339)
	}

	// Try to parse the SaveGame XML.  Stardew saves may use different file names:
	// - "SaveGameInfo" (1.5 standard)
	// - "SaveGameInfo.xml" (some versions)
	// - The main save file itself (<saveName>) as a fallback
	var xmlData []byte
	var err error
	for _, candidate := range []string{
		filepath.Join(saveFolder, "SaveGameInfo"),
		filepath.Join(saveFolder, "SaveGameInfo.xml"),
		mainFile,
	} {
		xmlData, err = os.ReadFile(candidate)
		if err == nil && len(xmlData) > 0 {
			break
		}
	}
	if err != nil || len(xmlData) == 0 {
		info.ParseError = "未找到 SaveGameInfo 文件"
		return info
	}

	fillSaveInfoFromXML(&info, xmlData, func() string {
		return readWhichFarmFromMainFile(saveFolder, name)
	})
	return info
}

func fillSaveInfoFromXML(info *registry.SaveInfo, xmlData []byte, farmTypeFallback func() string) {
	// Try to parse as <SaveGame> structure (full save file).
	// whichFarm can be an int (0-7) or a string (e.g. "MeadowlandsFarm").
	type saveGameXML struct {
		XMLName xml.Name `xml:"SaveGame"`
		Player  struct {
			Name     string `xml:"name"`
			FarmName string `xml:"farmName"`
		} `xml:"player"`
		Year      int    `xml:"year"`
		Season    string `xml:"currentSeason"`
		Day       int    `xml:"dayOfMonth"`
		WhichFarm string `xml:"whichFarm"` // string: handles both "0" and "MeadowlandsFarm"
	}
	var sg saveGameXML
	if err := xml.Unmarshal(xmlData, &sg); err == nil && sg.XMLName.Local == "SaveGame" {
		info.FarmerName = sg.Player.Name
		info.FarmName = sg.Player.FarmName
		info.GameYear = sg.Year
		info.GameSeason = sg.Season
		info.GameDay = sg.Day
		if sg.WhichFarm != "" {
			info.FarmType = farmTypeLabelFromString(sg.WhichFarm)
		}
		return
	}

	// Try to parse as <Farmer> structure (Junimo SaveGameInfo format).
	type farmerXML struct {
		XMLName           xml.Name `xml:"Farmer"`
		Name              string   `xml:"name"`
		FarmName          string   `xml:"farmName"`
		DayOfMonthForSave int      `xml:"dayOfMonthForSaveGame"`
		SeasonForSave     *int     `xml:"seasonForSaveGame"` // pointer: 0=spring is valid
		YearForSave       int      `xml:"yearForSaveGame"`
	}
	var fm farmerXML
	if err := xml.Unmarshal(xmlData, &fm); err == nil && fm.XMLName.Local == "Farmer" {
		info.FarmerName = fm.Name
		info.FarmName = fm.FarmName
		info.GameYear = fm.YearForSave
		info.GameDay = fm.DayOfMonthForSave
		if fm.SeasonForSave != nil {
			info.GameSeason = seasonFromInt(*fm.SeasonForSave)
		}
		// <Farmer> does not contain whichFarm — try reading it from the main save file.
		if info.FarmType == "" && farmTypeFallback != nil {
			info.FarmType = farmTypeFallback()
		}
		return
	}

	info.ParseError = "SaveGameInfo 解析失败"
}

// whichFarmRe matches <whichFarm>...</whichFarm> in the main save file.
var whichFarmRe = regexp.MustCompile(`<whichFarm>([^<]+)</whichFarm>`)

// readWhichFarmFromMainFile reads whichFarm from the main save file
// (Saves/<saveName>/<saveName>) which is a full <SaveGame> XML.
// Returns the farm type label, or empty string if not found.
func readWhichFarmFromMainFile(saveFolder, saveName string) string {
	mainFile := filepath.Join(saveFolder, saveName)
	data, err := os.ReadFile(mainFile)
	if err != nil {
		return ""
	}
	return readWhichFarmFromData(data)
}

func readWhichFarmFromData(data []byte) string {
	// Use a limited reader approach: search for whichFarm in the raw data.
	// The main save file can be very large (10+ MB), but whichFarm appears early.
	matches := whichFarmRe.FindSubmatch(data)
	if len(matches) < 2 {
		return ""
	}
	return farmTypeLabelFromString(string(matches[1]))
}

// seasonFromInt maps Junimo's seasonForSaveGame integer to a season string.
func seasonFromInt(v int) string {
	switch v {
	case 0:
		return "spring"
	case 1:
		return "summer"
	case 2:
		return "fall"
	case 3:
		return "winter"
	default:
		return fmt.Sprintf("unknown(%d)", v)
	}
}

func farmTypeLabel(whichFarm int) string {
	switch whichFarm {
	case 0:
		return "standard"
	case 1:
		return "riverland"
	case 2:
		return "forest"
	case 3:
		return "hilltop"
	case 4:
		return "wilderness"
	case 5:
		return "fourcorners"
	case 6:
		return "beach"
	case 7:
		return "meadowlands"
	default:
		return "unknown"
	}
}

// farmTypeLabelFromString converts a whichFarm string value to a farm type label.
// whichFarm can be an integer ("0"-"7") or a string name like "MeadowlandsFarm".
func farmTypeLabelFromString(whichFarm string) string {
	whichFarm = strings.TrimSpace(whichFarm)
	// Try integer first.
	if id, err := strconv.Atoi(whichFarm); err == nil {
		return farmTypeLabel(id)
	}
	// Map known string names.
	switch strings.ToLower(whichFarm) {
	case "standardfarm":
		return "standard"
	case "riverlandfarm":
		return "riverland"
	case "forestfarm":
		return "forest"
	case "hilltopfarm":
		return "hilltop"
	case "wildernessfarm":
		return "wilderness"
	case "fourcornersfarm":
		return "fourcorners"
	case "beachfarm":
		return "beach"
	case "meadowlandsfarm":
		return "meadowlands"
	default:
		return ""
	}
}

// ListSaves scans the bind-mounted saves directory and returns parsed metadata for each save.
func (d *Driver) ListSaves(ctx context.Context, instance registry.Instance) ([]registry.SaveInfo, error) {
	names, err := listSaveDirs(instance.DataDir)
	if err != nil {
		return nil, fmt.Errorf("list saves: %w", err)
	}
	activeName := GetActiveSaveName(instance.DataDir)
	savesPath := filepath.Join(savesDir(instance.DataDir), "Saves")
	result := make([]registry.SaveInfo, 0, len(names))
	for _, name := range names {
		info := readSaveInfo(filepath.Join(savesPath, name))
		if name == activeName {
			info.IsActive = true
		}
		result = append(result, info)
	}
	return result, nil
}

// PreviewSaveZip validates a ZIP upload, extracts to a temp directory, parses metadata,
// and returns the preview. The caller owns the returned tempDir and must clean it up.
func PreviewSaveZip(zipPath string, originalName string) (saveName string, preview registry.SaveInfo, tempDir string, err error) {
	// Check file size.
	stat, err := os.Stat(zipPath)
	if err != nil {
		return "", registry.SaveInfo{}, "", fmt.Errorf("stat upload: %w", err)
	}
	if stat.Size() > maxUploadZipBytes {
		return "", registry.SaveInfo{}, "", fmt.Errorf("压缩包超过 %d MB 限制", maxUploadZipBytes/1024/1024)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", registry.SaveInfo{}, "", fmt.Errorf("打开 ZIP 失败: %w", err)
	}
	defer func() { _ = zr.Close() }()

	// Security checks: symlinks, absolute paths, traversal, size bomb.
	if err := validateZipEntries(zr.File); err != nil {
		return "", registry.SaveInfo{}, "", err
	}

	// Detect save folder name: find the top-level directory.
	detectedSaveName, err := detectSaveFolderName(zr)
	if err != nil {
		return "", registry.SaveInfo{}, "", err
	}

	// Validate the detected save name for safety (path traversal, reserved names, etc).
	if err := validateSaveName(detectedSaveName); err != nil {
		return "", registry.SaveInfo{}, "", fmt.Errorf("ZIP 存档目录名不合法: %w", err)
	}

	// Extract to temp dir.
	td, err := os.MkdirTemp("", "stardew-upload-*")
	if err != nil {
		return "", registry.SaveInfo{}, "", fmt.Errorf("创建临时目录: %w", err)
	}

	if err := extractZipSecure(zr, td); err != nil {
		_ = os.RemoveAll(td)
		return "", registry.SaveInfo{}, "", err
	}

	// Find extracted save dir.
	saveDir, err := findSaveDir(td, detectedSaveName)
	if err != nil {
		_ = os.RemoveAll(td)
		return "", registry.SaveInfo{}, "", err
	}

	si := readSaveInfo(saveDir)
	si.Name = detectedSaveName
	return detectedSaveName, si, td, nil
}

// detectSaveFolderName finds the single top-level directory in the ZIP.
// A valid Stardew save ZIP contains exactly one top-level folder (the save ID, e.g. "FarmerName_12345678").
func detectSaveFolderName(zr *zip.ReadCloser) (string, error) {
	topDirs := map[string]struct{}{}
	for _, f := range zr.File {
		parts := strings.SplitN(filepath.ToSlash(f.Name), "/", 2)
		if parts[0] != "" {
			topDirs[parts[0]] = struct{}{}
		}
	}
	if len(topDirs) == 0 {
		return "", fmt.Errorf("ZIP 为空或没有有效文件")
	}
	if len(topDirs) > 1 {
		return "", fmt.Errorf("ZIP 包含多个顶级目录，Stardew 存档应只有一个文件夹")
	}
	for name := range topDirs {
		return name, nil
	}
	return "", fmt.Errorf("无法确定存档文件夹名")
}

// validateZipEntries performs full security validation on ZIP entries:
// symlinks, absolute paths, path traversal (..), empty segments,
// single-file size limit, and total uncompressed size limit.
// Call this before extractZipSecure to reject malicious archives early.
func validateZipEntries(files []*zip.File) error {
	var totalUncompressed uint64
	for _, f := range files {
		if f.FileInfo().Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("ZIP 包含符号链接，拒绝处理")
		}
		name := filepath.ToSlash(f.Name)
		if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
			return fmt.Errorf("ZIP 包含绝对路径 %q", f.Name)
		}
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
		if f.UncompressedSize64 > maxSingleFileBytes {
			return fmt.Errorf("ZIP 内单个文件超过 %d MB", maxSingleFileBytes/1024/1024)
		}
		if totalUncompressed > maxUncompressedBytes {
			return fmt.Errorf("ZIP 解压总大小超过 %d MB", maxUncompressedBytes/1024/1024)
		}
	}
	return nil
}

// extractZipSecure extracts zr into destDir, verifying no path escapes during extraction.
// Caller must have already validated entries with validateZipEntries.
func extractZipSecure(zr *zip.ReadCloser, destDir string) error {
	for _, f := range zr.File {
		if f.FileInfo().Mode()&fs.ModeSymlink != 0 {
			continue // already rejected by validateZipEntries, skip defensively
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
		if err := extractFile(f, outPath); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(f *zip.File, outPath string) error {
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

	lr := &io.LimitedReader{R: rc, N: maxSingleFileBytes + 1}
	if _, err := io.Copy(dst, lr); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	if lr.N <= 0 {
		return fmt.Errorf("文件 %s 解压后超过大小限制", f.Name)
	}
	return nil
}

// findSaveDir looks for the save directory under tempDir/saveName or tempDir.
func findSaveDir(tempDir, saveName string) (string, error) {
	// Try direct: tempDir/saveName
	direct := filepath.Join(tempDir, saveName)
	if stat, err := os.Stat(direct); err == nil && stat.IsDir() {
		return direct, nil
	}
	// Search one level deep.
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return "", fmt.Errorf("read temp dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			candidate := filepath.Join(tempDir, e.Name())
			// Check if it contains SaveGameInfo (Stardew save marker).
			if _, err := os.Stat(filepath.Join(candidate, "SaveGameInfo")); err == nil {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("ZIP 中未找到有效的 Stardew 存档文件夹（缺少 SaveGameInfo）")
}

// ImportSaveToVolume moves the save from tempDir into the bind-mounted saves directory.
// saveName is the expected folder name.
func ImportSaveToVolume(dataDir, tempDir, saveName string) error {
	if err := validateSaveName(saveName); err != nil {
		return fmt.Errorf("存档名称不合法: %w", err)
	}

	savesRoot := filepath.Join(savesDir(dataDir), "Saves")
	if err := os.MkdirAll(savesRoot, 0o755); err != nil {
		return fmt.Errorf("create saves dir: %w", err)
	}

	src, err := findSaveDir(tempDir, saveName)
	if err != nil {
		return err
	}

	dest := resolveSavePath(savesRoot, saveName)
	if dest == "" {
		return fmt.Errorf("存档目标路径不合法: %q", saveName)
	}
	// Reject if target resolves to the Saves root itself.
	absRoot, _ := filepath.Abs(savesRoot)
	if dest == absRoot {
		return fmt.Errorf("存档目标路径不能是 Saves 根目录")
	}

	// Remove dest if it already exists (replace).
	if _, err := os.Stat(dest); err == nil {
		if err := os.RemoveAll(dest); err != nil {
			return fmt.Errorf("remove existing save %s: %w", saveName, err)
		}
	}

	// Try os.Rename first (fast if same filesystem), fall back to copy.
	if err := os.Rename(src, dest); err != nil {
		if err := copyDir(src, dest); err != nil {
			return fmt.Errorf("copy save to volume: %w", err)
		}
	}
	return nil
}

func copyDir(src, dest string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	return err
}

// WriteServerSettings writes a server-settings.json file from NewGameConfig.
// This controls what Junimo will use when creating the first game.
// Fields that cannot be pre-configured are noted in comments.
func WriteServerSettings(dataDir string, cfg registry.NewGameConfig) error {
	// Normalise first.
	normalizeCfg(&cfg)
	if err := validateCfg(cfg); err != nil {
		return err
	}

	farmTypeID := junimoFarmTypeID(cfg.FarmType)
	profitPercent := profitMarginPercent(cfg.ProfitMargin)
	// JunimoServer uses nested PascalCase JSON: {"Game":{...}, "Server":{...}}.
	// cabinLayout "nearby" → CabinLayoutNearby=true; moneyMode "shared" → SeparateWallets=false.
	cabinLayoutNearby := cfg.CabinLayout == "nearby"
	separateWallets := cfg.MoneyMode == "separate"
	spawnMonsters := "false"
	if cfg.SpawnMonstersOnFarm {
		spawnMonsters = "true"
	}

	// Build server-settings.json matching JunimoServer's ServerSettings class structure.
	// Game section: world creation params. Server section: runtime params.
	// FarmerName/FavoriteThing/Gender are applied via server-init.json + SMAPI mod.
	obj := map[string]any{
		"Game": map[string]any{
			"FarmName":             cfg.FarmName,
			"FarmType":             farmTypeID,
			"StartingCabins":       cfg.StartingCabins,
			"CabinLayoutNearby":    cabinLayoutNearby,
			"ProfitMargin":         profitPercent,
			"PetBreed":             cfg.PetBreed,
			"RemixBundles":         cfg.RemixedCommunityCenter,
			"RemixMines":           cfg.RemixedMineRewards,
			"SpawnMonstersAtNight": spawnMonsters,
		},
		"Server": map[string]any{
			"SeparateWallets": separateWallets,
		},
	}

	settingsPath := serverSettingsPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	data, err := marshalJSON(obj)
	if err != nil {
		return fmt.Errorf("marshal server-settings.json: %w", err)
	}
	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		return err
	}
	return WriteInitConfig(dataDir, cfg)
}

// normalizeCfg applies defaults.
func normalizeCfg(cfg *registry.NewGameConfig) {
	if cfg.FarmType == "" {
		cfg.FarmType = "standard"
	}
	if cfg.CabinLayout == "" {
		cfg.CabinLayout = "nearby"
	}
	if cfg.ProfitMargin == "" {
		cfg.ProfitMargin = "100"
	}
	if cfg.MoneyMode == "" {
		cfg.MoneyMode = "shared"
	}
	if cfg.Gender == "" {
		cfg.Gender = "male"
	}
	if cfg.PetType == "" {
		cfg.PetType = "Cat"
	}
}

// validateCfg checks the config fields.
func validateCfg(cfg registry.NewGameConfig) error {
	if strings.TrimSpace(cfg.FarmName) == "" {
		return fmt.Errorf("farmName 不能为空")
	}
	if !utf8.ValidString(cfg.FarmName) || len(cfg.FarmName) > 100 {
		return fmt.Errorf("farmName 包含无效字符或过长")
	}
	if cfg.FarmerName != "" && (!utf8.ValidString(cfg.FarmerName) || len(cfg.FarmerName) > 100) {
		return fmt.Errorf("farmerName 包含无效字符或过长")
	}
	validFarms := map[string]bool{
		"standard": true, "riverland": true, "forest": true,
		"hilltop": true, "wilderness": true, "fourcorners": true, "beach": true, "meadowlands": true,
	}
	if !validFarms[cfg.FarmType] {
		return fmt.Errorf("farmType 必须是 standard/riverland/forest/hilltop/wilderness/fourcorners/beach/meadowlands 之一")
	}
	if cfg.StartingCabins < 0 || cfg.StartingCabins > 3 {
		return fmt.Errorf("startingCabins 必须在 0~3 之间")
	}
	if cfg.CabinLayout != "nearby" && cfg.CabinLayout != "separate" {
		return fmt.Errorf("cabinLayout 必须是 nearby 或 separate")
	}
	validProfit := map[string]bool{"100": true, "75": true, "50": true, "25": true}
	if !validProfit[cfg.ProfitMargin] {
		return fmt.Errorf("profitMargin 必须是 100/75/50/25 之一")
	}
	if cfg.PetBreed < 0 || cfg.PetBreed > 4 {
		return fmt.Errorf("petBreed 必须在 0~4 之间")
	}
	if cfg.PetBreedID != "" {
		id, err := strconv.Atoi(cfg.PetBreedID)
		if err != nil || id < 0 || id > 4 {
			return fmt.Errorf("petBreedId 必须是 0~4 的数字")
		}
		if id != cfg.PetBreed {
			return fmt.Errorf("petBreed 与 petBreedId 必须对应同一品种")
		}
	}
	if cfg.MoneyMode != "shared" && cfg.MoneyMode != "separate" {
		return fmt.Errorf("moneyMode 必须是 shared 或 separate")
	}
	if cfg.Gender != "" && cfg.Gender != "male" && cfg.Gender != "female" {
		return fmt.Errorf("gender 必须是 male 或 female")
	}
	if cfg.PetType != "" && cfg.PetType != "Cat" && cfg.PetType != "Dog" {
		return fmt.Errorf("petType 必须是 Cat 或 Dog")
	}
	return nil
}

func junimoFarmTypeID(farmType string) int {
	m := map[string]int{
		"standard": 0, "riverland": 1, "forest": 2,
		"hilltop": 3, "wilderness": 4, "fourcorners": 5, "beach": 6, "meadowlands": 7,
	}
	if id, ok := m[farmType]; ok {
		return id
	}
	return 0
}

func profitMarginPercent(profitMargin string) float64 {
	switch profitMargin {
	case "75":
		return 0.75
	case "50":
		return 0.5
	case "25":
		return 0.25
	default:
		return 1.0
	}
}

// serverInitPath returns where the server-init.json lives in the control dir.
func serverInitPath(dataDir string) string {
	return filepath.Join(controlDir(dataDir), "server-init.json")
}

// initConfigJSON is the structure written to server-init.json for the SMAPI mod.
type initConfigJSON struct {
	Mode                 string   `json:"mode"`
	FarmerName           string   `json:"farmerName"`
	FarmName             string   `json:"farmName"`
	FavoriteThing        string   `json:"favoriteThing,omitempty"`
	Gender               string   `json:"gender,omitempty"`
	PetType              string   `json:"petType,omitempty"`
	PetBreed             string   `json:"petBreed,omitempty"`
	Skin                 *int     `json:"skin,omitempty"`
	Hair                 *int     `json:"hair,omitempty"`
	Shirt                string   `json:"shirt,omitempty"`
	Pants                string   `json:"pants,omitempty"`
	Accessory            *int     `json:"accessory,omitempty"`
	EyeColor             *rgbJSON `json:"eyeColor,omitempty"`
	HairColor            *rgbJSON `json:"hairColor,omitempty"`
	PantsColor           *rgbJSON `json:"pantsColor,omitempty"`
	FarmType             string   `json:"farmType,omitempty"`
	CabinCount           int      `json:"cabinCount"`
	CabinLayout          string   `json:"cabinLayout,omitempty"`
	MoneyMode            string   `json:"moneyMode,omitempty"`
	ProfitMargin         int      `json:"profitMargin"`
	SkipIntro            bool     `json:"skipIntro"`
	AutoPause            bool     `json:"autoPause"`
	BundlesRemix         bool     `json:"bundlesRemix"`
	MinesRemix           bool     `json:"minesRemix"`
	SpawnMonstersAtNight bool     `json:"spawnMonstersAtNight"`
}

type rgbJSON struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

// WriteInitConfig writes server-init.json to the control directory.
// The SMAPI mod reads this on game launch and applies character customization
// on the SaveCreating event (works in Junimo runtime).
func WriteInitConfig(dataDir string, cfg registry.NewGameConfig) error {
	if cfg.FarmerName == "" && cfg.FavoriteThing == "" && cfg.Gender == "" {
		// No SMAPI character fields provided — skip writing init config.
		return nil
	}

	profitInt := 100
	switch cfg.ProfitMargin {
	case "75":
		profitInt = 75
	case "50":
		profitInt = 50
	case "25":
		profitInt = 25
	}

	petBreedID := cfg.PetBreedID
	if petBreedID == "" {
		petBreedID = fmt.Sprintf("%d", cfg.PetBreed)
	}

	// Junimo uses "nearby"/"separate"; SMAPI uses "close"/"separate".
	smapicabinLayout := cfg.CabinLayout
	if smapicabinLayout == "nearby" {
		smapicabinLayout = "close"
	}

	ic := initConfigJSON{
		Mode:                 "native-create",
		FarmerName:           cfg.FarmerName,
		FarmName:             cfg.FarmName,
		FavoriteThing:        cfg.FavoriteThing,
		Gender:               cfg.Gender,
		PetType:              cfg.PetType,
		PetBreed:             petBreedID,
		Skin:                 cfg.Skin,
		Hair:                 cfg.Hair,
		Shirt:                cfg.Shirt,
		Pants:                cfg.Pants,
		Accessory:            cfg.Accessory,
		FarmType:             cfg.FarmType,
		CabinCount:           cfg.StartingCabins,
		CabinLayout:          smapicabinLayout,
		MoneyMode:            cfg.MoneyMode,
		ProfitMargin:         profitInt,
		SkipIntro:            true,
		AutoPause:            true,
		BundlesRemix:         cfg.RemixedCommunityCenter,
		MinesRemix:           cfg.RemixedMineRewards,
		SpawnMonstersAtNight: cfg.SpawnMonstersOnFarm,
	}
	if cfg.EyeColor != nil {
		ic.EyeColor = &rgbJSON{R: cfg.EyeColor.R, G: cfg.EyeColor.G, B: cfg.EyeColor.B}
	}
	if cfg.HairColor != nil {
		ic.HairColor = &rgbJSON{R: cfg.HairColor.R, G: cfg.HairColor.G, B: cfg.HairColor.B}
	}
	if cfg.PantsColor != nil {
		ic.PantsColor = &rgbJSON{R: cfg.PantsColor.R, G: cfg.PantsColor.G, B: cfg.PantsColor.B}
	}

	initPath := serverInitPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(initPath), 0o755); err != nil {
		return fmt.Errorf("create control dir: %w", err)
	}
	data, err := marshalJSON(ic)
	if err != nil {
		return fmt.Errorf("marshal server-init.json: %w", err)
	}
	return os.WriteFile(initPath, data, 0o644)
}

// marshalJSON produces indented JSON for human-readable settings files.
func marshalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf) // will use encoding/json below
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GetActiveSaveName reads the JunimoServer gameloader config and returns
// the save name that will be loaded on next startup.  Returns empty string
// if no save is configured.
func GetActiveSaveName(dataDir string) string {
	gameloaderPath := filepath.Join(savesDir(dataDir), ".smapi", "mod-data", "junimohost.server", "junimohost.gameloader.json")
	data, err := os.ReadFile(gameloaderPath)
	if err != nil {
		return ""
	}
	var cfg struct {
		SaveNameToLoad string `json:"SaveNameToLoad"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return cfg.SaveNameToLoad
}

// reservedSaveNames are route path segments that would conflict with
// DELETE /api/instances/:id/saves/:name routing.
var reservedSaveNames = map[string]bool{
	"preflight":               true,
	"custom-new-game":         true,
	"upload-preview":          true,
	"upload-commit-and-start": true,
	"select":                  true,
	"select-and-start":        true,
	"delete":                  true,
}

// validateSaveName rejects dangerous save names before any path construction.
func validateSaveName(saveName string) error {
	if saveName == "" {
		return fmt.Errorf("save name 不能为空")
	}
	if saveName == "." || saveName == ".." {
		return fmt.Errorf("save name 不能是 %q", saveName)
	}
	if strings.ContainsAny(saveName, `/\`) {
		return fmt.Errorf("save name 不能包含路径分隔符")
	}
	if filepath.IsAbs(saveName) {
		return fmt.Errorf("save name 不能是绝对路径")
	}
	cleaned := filepath.Clean(saveName)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("save name 尝试目录穿越")
	}
	if reservedSaveNames[saveName] {
		return fmt.Errorf("save name %q 与系统路由冲突，请使用其他名称", saveName)
	}
	return nil
}

// resolveSavePath returns the absolute path of a save directory if it is
// contained within savesRoot.  Returns empty string if the path escapes.
func resolveSavePath(savesRoot, saveName string) string {
	absRoot, err := filepath.Abs(savesRoot)
	if err != nil {
		return ""
	}
	target := filepath.Join(absRoot, saveName)
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return ""
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return absTarget
}

// ValidateSaveExists checks that a save folder with the given name exists
// and is a directory under the instance's Saves directory.
func ValidateSaveExists(dataDir, saveName string) error {
	if err := validateSaveName(saveName); err != nil {
		return err
	}
	savesRoot := filepath.Join(savesDir(dataDir), "Saves")
	targetPath := resolveSavePath(savesRoot, saveName)
	if targetPath == "" {
		return fmt.Errorf("存档路径不合法: %q", saveName)
	}
	info, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("存档 %q 不存在", saveName)
	}
	if err != nil {
		return fmt.Errorf("检查存档 %q 失败: %w", saveName, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("存档 %q 不是目录", saveName)
	}
	return nil
}

// DeleteSave removes a single save folder from the bind-mounted saves directory.
func DeleteSave(dataDir, saveName string) error {
	if err := validateSaveName(saveName); err != nil {
		return err
	}
	savesRoot := filepath.Join(savesDir(dataDir), "Saves")
	targetPath := resolveSavePath(savesRoot, saveName)
	if targetPath == "" {
		return fmt.Errorf("存档路径不合法: %q", saveName)
	}
	info, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("存档 %q 不存在", saveName)
	}
	if err != nil {
		return fmt.Errorf("检查存档 %q 失败: %w", saveName, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("存档 %q 不是目录", saveName)
	}
	if err := os.RemoveAll(targetPath); err != nil {
		return fmt.Errorf("删除存档 %q 失败: %w", saveName, err)
	}
	// If this was the active save, clear the gameloader config.
	active := GetActiveSaveName(dataDir)
	if active == saveName {
		gameloaderPath := filepath.Join(savesDir(dataDir), ".smapi", "mod-data", "junimohost.server", "junimohost.gameloader.json")
		_ = os.Remove(gameloaderPath)
	}
	return nil
}

// ExportSaveZip creates a ZIP archive of a single save folder.
// The ZIP filename follows the pattern: saveName_游戏时间.zip
// e.g. "FarmerName_12345_1年_春_1日.zip"
func ExportSaveZip(dataDir, saveName string) (string, error) {
	if err := validateSaveName(saveName); err != nil {
		return "", err
	}
	savesRoot := filepath.Join(savesDir(dataDir), "Saves")
	saveDir := filepath.Join(savesRoot, saveName)
	info, err := os.Stat(saveDir)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("存档 %q 不存在", saveName)
	}
	if err != nil {
		return "", fmt.Errorf("检查存档 %q 失败: %w", saveName, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("存档 %q 不是目录", saveName)
	}

	// Parse save info for the filename.
	si := readSaveInfo(saveDir)
	zipName := buildSaveZipName(saveName, si)
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
	err = filepath.WalkDir(saveDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(savesRoot, path)
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
			_, err := w.Create(relPath + "/")
			return err
		}

		fi, err := d.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(fi)
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

// buildSaveZipName constructs a human-readable ZIP filename for a save export.
// Pattern: saveName_游戏时间.zip  e.g. "FarmerName_12345_1年_春_1日.zip"
func buildSaveZipName(saveName string, info registry.SaveInfo) string {
	sanitized := strings.ReplaceAll(saveName, " ", "_")
	if info.GameYear > 0 && info.GameSeason != "" && info.GameDay > 0 {
		seasonCN := seasonLabelCN(info.GameSeason)
		return fmt.Sprintf("%s_%d年_%s_%d日.zip", sanitized, info.GameYear, seasonCN, info.GameDay)
	}
	return fmt.Sprintf("%s.zip", sanitized)
}

func seasonLabelCN(season string) string {
	switch season {
	case "spring":
		return "春"
	case "summer":
		return "夏"
	case "fall":
		return "秋"
	case "winter":
		return "冬"
	default:
		return season
	}
}

// HasTemplates returns true if at least one save template directory exists.
func HasTemplates(dataDir string) bool {
	tDir := savesTemplatesDir(dataDir)
	entries, err := os.ReadDir(tDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			return true
		}
	}
	return false
}

// ── Backup / Restore ──────────────────────────────────────────────────────────

// backupsDir returns the path to the saves backup directory.
func backupsDir(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "backups", "saves")
}

// BackupInfo describes a single backup file.
type BackupInfo struct {
	Name          string `json:"name"`
	SaveName      string `json:"saveName"`
	Size          int64  `json:"size"`
	CreatedAt     string `json:"createdAt"`
	FarmerName    string `json:"farmerName,omitempty"`
	FarmName      string `json:"farmName,omitempty"`
	GameYear      int    `json:"gameYear,omitempty"`
	GameSeason    string `json:"gameSeason,omitempty"`
	GameDay       int    `json:"gameDay,omitempty"`
	FarmType      string `json:"farmType,omitempty"`
	FileSizeBytes int64  `json:"fileSizeBytes,omitempty"`
	ParseError    string `json:"parseError,omitempty"`
}

// BackupSave creates a ZIP backup of the specified save in the backups directory.
// The backup filename includes the save name and a timestamp.
// Returns the backup file path on success.
func BackupSave(dataDir, saveName string) (string, error) {
	if err := validateSaveName(saveName); err != nil {
		return "", err
	}
	savesRoot := filepath.Join(savesDir(dataDir), "Saves")
	saveDir := filepath.Join(savesRoot, saveName)
	info, err := os.Stat(saveDir)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("存档 %q 不存在，无法备份", saveName)
	}
	if err != nil {
		return "", fmt.Errorf("检查存档 %q 失败: %w", saveName, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("存档 %q 不是目录", saveName)
	}

	backupDir := backupsDir(dataDir)
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", fmt.Errorf("创建备份目录失败: %w", err)
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	backupName := fmt.Sprintf("%s_%s.zip", saveName, timestamp)
	backupPath := filepath.Join(backupDir, backupName)

	zf, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("创建备份文件失败: %w", err)
	}
	defer func() {
		if err != nil {
			_ = zf.Close()
			_ = os.Remove(backupPath)
		}
	}()

	w := zip.NewWriter(zf)
	err = filepath.WalkDir(saveDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(savesRoot, path)
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

		if d.IsDir() {
			_, err := w.Create(relPath + "/")
			return err
		}

		fi, err := d.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(fi)
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

	if err := w.Close(); err != nil {
		_ = zf.Close()
		_ = os.Remove(backupPath)
		return "", fmt.Errorf("关闭备份 ZIP 失败: %w", err)
	}
	if err := zf.Close(); err != nil {
		_ = os.Remove(backupPath)
		return "", fmt.Errorf("关闭备份文件失败: %w", err)
	}
	if err != nil {
		return "", err
	}
	return backupPath, nil
}

// DeleteSaveWithBackup creates a backup before deleting the save.
// If the backup fails, the delete is aborted to prevent unrecoverable data loss.
func DeleteSaveWithBackup(dataDir, saveName string) (backupPath string, err error) {
	// Attempt backup first — failure blocks deletion.
	backupPath, backupErr := BackupSave(dataDir, saveName)
	if backupErr != nil {
		return "", fmt.Errorf("备份失败，已中止删除以保护数据: %w", backupErr)
	}
	// Delete the save.
	if err := DeleteSave(dataDir, saveName); err != nil {
		return backupPath, err
	}
	return backupPath, nil
}

// ListBackups returns all backup files in the backups directory.
func ListBackups(dataDir string) ([]BackupInfo, error) {
	backupDir := backupsDir(dataDir)
	entries, err := os.ReadDir(backupDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取备份目录失败: %w", err)
	}

	var backups []BackupInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".zip") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		saveName := parseBackupSaveName(e.Name())
		backup := BackupInfo{
			Name:      e.Name(),
			SaveName:  saveName,
			Size:      fi.Size(),
			CreatedAt: fi.ModTime().UTC().Format(time.RFC3339),
		}
		enrichBackupInfo(filepath.Join(backupDir, e.Name()), &backup)
		backups = append(backups, backup)
	}
	return backups, nil
}

func enrichBackupInfo(backupPath string, backup *BackupInfo) {
	zr, err := zip.OpenReader(backupPath)
	if err != nil {
		backup.ParseError = "打开备份 ZIP 失败"
		return
	}
	defer func() { _ = zr.Close() }()

	if err := validateZipEntries(zr.File); err != nil {
		backup.ParseError = err.Error()
		return
	}
	saveName, err := detectSaveFolderName(zr)
	if err != nil {
		backup.ParseError = err.Error()
		return
	}
	if saveName != "" {
		backup.SaveName = saveName
	}

	mainPath := filepath.ToSlash(filepath.Join(saveName, saveName))
	mainData, mainSize, _ := readZipEntry(zr.File, mainPath)
	if mainSize > 0 {
		backup.FileSizeBytes = int64(mainSize)
	}

	var xmlData []byte
	for _, candidate := range []string{
		filepath.ToSlash(filepath.Join(saveName, "SaveGameInfo")),
		filepath.ToSlash(filepath.Join(saveName, "SaveGameInfo.xml")),
		mainPath,
	} {
		data, _, ok := readZipEntry(zr.File, candidate)
		if ok && len(data) > 0 {
			xmlData = data
			break
		}
	}
	if len(xmlData) == 0 {
		backup.ParseError = "未找到 SaveGameInfo 文件"
		return
	}

	info := registry.SaveInfo{Name: saveName}
	fillSaveInfoFromXML(&info, xmlData, func() string {
		if len(mainData) == 0 {
			return ""
		}
		return readWhichFarmFromData(mainData)
	})
	backup.FarmerName = info.FarmerName
	backup.FarmName = info.FarmName
	backup.GameYear = info.GameYear
	backup.GameSeason = info.GameSeason
	backup.GameDay = info.GameDay
	backup.FarmType = info.FarmType
	backup.ParseError = info.ParseError
}

func readZipEntry(files []*zip.File, name string) ([]byte, uint64, bool) {
	name = filepath.ToSlash(name)
	for _, f := range files {
		if filepath.ToSlash(f.Name) != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, f.UncompressedSize64, false
		}
		defer func() { _ = rc.Close() }()
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, f.UncompressedSize64, false
		}
		return data, f.UncompressedSize64, true
	}
	return nil, 0, false
}

func validateBackupName(backupName string) error {
	if backupName == "" || strings.ContainsAny(backupName, "/\\:") {
		return fmt.Errorf("备份文件名不合法")
	}
	if strings.Contains(backupName, "..") {
		return fmt.Errorf("备份文件名不合法")
	}
	if !strings.HasSuffix(backupName, ".zip") {
		return fmt.Errorf("备份文件必须是 .zip")
	}
	return nil
}

// DeleteBackup permanently deletes one backup ZIP file.
func DeleteBackup(dataDir, backupName string) error {
	if err := validateBackupName(backupName); err != nil {
		return err
	}
	backupPath := filepath.Join(backupsDir(dataDir), backupName)
	info, err := os.Stat(backupPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("备份文件 %q 不存在", backupName)
	}
	if err != nil {
		return fmt.Errorf("检查备份文件失败: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("备份文件 %q 不是文件", backupName)
	}
	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("删除备份文件失败: %w", err)
	}
	return nil
}

// RestoreBackup restores a backup ZIP to a save directory.
// If a save with the same name already exists and overwrite is false, returns ErrConflict.
// When overwriting, the old save is backed up first, then the backup is extracted
// to a temporary directory and atomically moved into place. This prevents data loss
// if extraction fails midway.
func RestoreBackup(dataDir, backupName string, overwrite bool) (string, error) {
	if err := validateBackupName(backupName); err != nil {
		return "", err
	}

	backupPath := filepath.Join(backupsDir(dataDir), backupName)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return "", fmt.Errorf("备份文件 %q 不存在", backupName)
	}

	// Open the ZIP to detect the save name.
	zr, err := zip.OpenReader(backupPath)
	if err != nil {
		return "", fmt.Errorf("打开备份 ZIP 失败: %w", err)
	}

	// Full security validation before any extraction.
	if err := validateZipEntries(zr.File); err != nil {
		_ = zr.Close()
		return "", err
	}

	saveName, err := detectSaveFolderName(zr)
	if err != nil {
		_ = zr.Close()
		return "", fmt.Errorf("无法从备份中识别存档名: %w", err)
	}
	if err := validateSaveName(saveName); err != nil {
		_ = zr.Close()
		return "", fmt.Errorf("备份中的存档名不合法: %w", err)
	}

	savesRoot := filepath.Join(savesDir(dataDir), "Saves")
	targetDir := filepath.Join(savesRoot, saveName)

	// Check for existing save.
	if _, err := os.Stat(targetDir); err == nil {
		if !overwrite {
			_ = zr.Close()
			return saveName, fmt.Errorf("存档 %q 已存在，请使用覆盖选项或先删除已有存档", saveName)
		}
		// Backup existing save before overwriting.
		if _, backupErr := BackupSave(dataDir, saveName); backupErr != nil {
			_ = zr.Close()
			return "", fmt.Errorf("覆盖前备份已有存档失败，已中止恢复以保护数据: %w", backupErr)
		}
	}

	// Close and re-open the ZIP before extraction to ensure a clean read state.
	// This avoids issues on some platforms where iterating zr.File headers
	// can affect the underlying reader state.
	_ = zr.Close()
	zr, err = zip.OpenReader(backupPath)
	if err != nil {
		return "", fmt.Errorf("重新打开备份 ZIP 失败: %w", err)
	}
	defer func() { _ = zr.Close() }()

	// Extract to a temporary directory first — atomic approach.
	tempDir, err := os.MkdirTemp(savesRoot, ".restore-tmp-*")
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(tempDir)
		}
	}()

	if err := extractZipSecure(zr, tempDir); err != nil {
		return "", fmt.Errorf("解压备份失败: %w", err)
	}

	// Verify the extracted save is valid (contains SaveGameInfo).
	extractedSave := filepath.Join(tempDir, saveName)
	if _, err := os.Stat(filepath.Join(extractedSave, "SaveGameInfo")); err != nil {
		return "", fmt.Errorf("恢复的存档缺少 SaveGameInfo，可能不是有效的 Stardew 存档")
	}

	// Atomic replace: remove old, move new into place.
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return "", fmt.Errorf("删除已有存档失败: %w", err)
		}
	}
	if err := os.Rename(extractedSave, targetDir); err != nil {
		return "", fmt.Errorf("移动恢复存档到目标位置失败: %w", err)
	}

	success = true
	return saveName, nil
}

// parseBackupSaveName extracts the save name from a backup filename like
// "SaveName_20260627-150405.zip" → "SaveName".
func parseBackupSaveName(filename string) string {
	name := strings.TrimSuffix(filename, ".zip")
	// Find the last underscore followed by a timestamp pattern.
	idx := strings.LastIndex(name, "_")
	if idx > 0 {
		candidate := name[idx+1:]
		// Check if it looks like a timestamp (digits and hyphens).
		if len(candidate) >= 15 && strings.ContainsAny(candidate, "0123456789-") {
			return name[:idx]
		}
	}
	return name
}
