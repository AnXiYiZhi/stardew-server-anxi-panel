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
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

const (
	maxUploadZipBytes    = 100 * 1024 * 1024 // 100 MB compressed
	maxUncompressedBytes = 512 * 1024 * 1024  // 512 MB uncompressed total
	maxSingleFileBytes   = 64 * 1024 * 1024   // 64 MB per file
)

// savesDir returns the host-side path to the bind-mounted saves directory.
// Stardew saves live at: <savesDir>/Saves/<SaveFolderName>/
func savesDir(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "saves")
}

// savesTemplatesDir returns where save templates should be placed.
func savesTemplatesDir(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "saves-templates")
}

// serverSettingsPath returns where the server-settings.json lives.
func serverSettingsPath(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "settings", "server-settings.json")
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
func readSaveInfo(saveFolder string) registry.SaveInfo {
	name := filepath.Base(saveFolder)
	info := registry.SaveInfo{Name: name}

	// Try to get modification time and size from the main save file.
	mainFile := filepath.Join(saveFolder, name)
	if stat, err := os.Stat(mainFile); err == nil {
		info.FileSizeBytes = stat.Size()
		info.ModifiedAt = stat.ModTime().UTC().Format(time.RFC3339)
	}

	// Try to parse the SaveGame XML.
	saveXML := filepath.Join(saveFolder, "SaveGameInfo")
	xmlData, err := os.ReadFile(saveXML)
	if err != nil {
		info.ParseError = "未找到 SaveGameInfo 文件"
		return info
	}

	var sg struct {
		FarmerName  string `xml:"player>name"`
		FarmName    string `xml:"player>farmName"`
		Year        int    `xml:"year"`
		Season      string `xml:"currentSeason"`
		Day         int    `xml:"dayOfMonth"`
		WhichFarm   int    `xml:"whichFarm"`
	}
	if err := xml.Unmarshal(xmlData, &sg); err != nil {
		info.ParseError = "SaveGameInfo 解析失败"
		return info
	}

	info.FarmerName = sg.FarmerName
	info.FarmName = sg.FarmName
	info.GameYear = sg.Year
	info.GameSeason = sg.Season
	info.GameDay = sg.Day
	info.FarmType = farmTypeLabel(sg.WhichFarm)
	return info
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
	default:
		return "unknown"
	}
}

// ListSaves scans the bind-mounted saves directory and returns parsed metadata for each save.
func (d *Driver) ListSaves(ctx context.Context, instance registry.Instance) ([]registry.SaveInfo, error) {
	names, err := listSaveDirs(instance.DataDir)
	if err != nil {
		return nil, fmt.Errorf("list saves: %w", err)
	}
	savesPath := filepath.Join(savesDir(instance.DataDir), "Saves")
	result := make([]registry.SaveInfo, 0, len(names))
	for _, name := range names {
		info := readSaveInfo(filepath.Join(savesPath, name))
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

	// Security checks: zip-slip, absolute paths, symlinks, size bomb.
	var totalUncompressed uint64
	for _, f := range zr.File {
		if f.FileInfo().Mode()&fs.ModeSymlink != 0 {
			return "", registry.SaveInfo{}, "", fmt.Errorf("ZIP 包含符号链接，拒绝处理")
		}
		name := filepath.ToSlash(f.Name)
		if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
			return "", registry.SaveInfo{}, "", fmt.Errorf("ZIP 包含绝对路径 %q", f.Name)
		}
		cleaned := filepath.Clean(name)
		if strings.HasPrefix(cleaned, "..") {
			return "", registry.SaveInfo{}, "", fmt.Errorf("ZIP 路径 %q 尝试目录穿越", f.Name)
		}
		totalUncompressed += f.UncompressedSize64
		if f.UncompressedSize64 > maxSingleFileBytes {
			return "", registry.SaveInfo{}, "", fmt.Errorf("ZIP 内单个文件超过 %d MB", maxSingleFileBytes/1024/1024)
		}
		if totalUncompressed > maxUncompressedBytes {
			return "", registry.SaveInfo{}, "", fmt.Errorf("ZIP 解压总大小超过 %d MB", maxUncompressedBytes/1024/1024)
		}
	}

	// Detect save folder name: find the top-level directory.
	detectedSaveName, err := detectSaveFolderName(zr)
	if err != nil {
		return "", registry.SaveInfo{}, "", err
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

// extractZipSecure extracts zr into destDir, verifying no path escapes during extraction.
func extractZipSecure(zr *zip.ReadCloser, destDir string) error {
	for _, f := range zr.File {
		if f.FileInfo().Mode()&fs.ModeSymlink != 0 {
			continue // already rejected by pre-check, skip defensively
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
	savesPath := filepath.Join(savesDir(dataDir), "Saves")
	if err := os.MkdirAll(savesPath, 0o755); err != nil {
		return fmt.Errorf("create saves dir: %w", err)
	}

	src, err := findSaveDir(tempDir, saveName)
	if err != nil {
		return err
	}

	dest := filepath.Join(savesPath, saveName)
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
	cabinLayout := 0
	if cfg.CabinLayout == "separate" {
		cabinLayout = 1
	}
	moneyMode := 0
	if cfg.MoneyMode == "shared" {
		moneyMode = 1
	}

	// Build minimal server-settings.json. Only fields supported by JunimoServer
	// are written; FarmerName/FavoriteThing/Gender require a save template.
	obj := map[string]any{
		"farmName":        cfg.FarmName,
		"whichFarm":       farmTypeID,
		"startingCabins":  cfg.StartingCabins,
		"cabinLayout":     cabinLayout,
		"profitMargin":    profitPercent,
		"petBreed":        cfg.PetBreed,
		"moneyMode":       moneyMode,
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
		"hilltop": true, "wilderness": true, "fourcorners": true, "beach": true,
	}
	if !validFarms[cfg.FarmType] {
		return fmt.Errorf("farmType 必须是 standard/riverland/forest/hilltop/wilderness/fourcorners/beach 之一")
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
	if cfg.PetBreed < 0 || cfg.PetBreed > 3 {
		return fmt.Errorf("petBreed 必须在 0~3 之间")
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
		"hilltop": 3, "wilderness": 4, "fourcorners": 5, "beach": 6,
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
	Mode          string     `json:"mode"`
	FarmerName    string     `json:"farmerName"`
	FarmName      string     `json:"farmName"`
	FavoriteThing string     `json:"favoriteThing,omitempty"`
	Gender        string     `json:"gender,omitempty"`
	PetType       string     `json:"petType,omitempty"`
	PetBreed      string     `json:"petBreed,omitempty"`
	Skin          *int       `json:"skin,omitempty"`
	Hair          *int       `json:"hair,omitempty"`
	Shirt         string     `json:"shirt,omitempty"`
	Pants         string     `json:"pants,omitempty"`
	Accessory     *int       `json:"accessory,omitempty"`
	EyeColor      *rgbJSON   `json:"eyeColor,omitempty"`
	HairColor     *rgbJSON   `json:"hairColor,omitempty"`
	PantsColor    *rgbJSON   `json:"pantsColor,omitempty"`
	FarmType      string     `json:"farmType,omitempty"`
	CabinCount    int        `json:"cabinCount"`
	CabinLayout   string     `json:"cabinLayout,omitempty"`
	MoneyMode     string     `json:"moneyMode,omitempty"`
	ProfitMargin  int        `json:"profitMargin"`
	SkipIntro     bool       `json:"skipIntro"`
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
		Mode:          "create-or-load",
		FarmerName:    cfg.FarmerName,
		FarmName:      cfg.FarmName,
		FavoriteThing: cfg.FavoriteThing,
		Gender:        cfg.Gender,
		PetType:       cfg.PetType,
		PetBreed:      petBreedID,
		Skin:          cfg.Skin,
		Hair:          cfg.Hair,
		Shirt:         cfg.Shirt,
		Pants:         cfg.Pants,
		Accessory:     cfg.Accessory,
		FarmType:      cfg.FarmType,
		CabinCount:    cfg.StartingCabins,
		CabinLayout:   smapicabinLayout,
		MoneyMode:     cfg.MoneyMode,
		ProfitMargin:  profitInt,
		SkipIntro:     true,
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
