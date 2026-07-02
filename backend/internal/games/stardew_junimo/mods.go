package stardew_junimo

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

const (
	maxModZipBytes     = 200 * 1024 * 1024 // 200 MB compressed
	maxModUncompressed = 512 * 1024 * 1024 // 512 MB uncompressed total
	maxModSingleFile   = 64 * 1024 * 1024  // 64 MB per file
	restartRequiredKey = "modsRestartRequired"
	smapiRuntimeModID  = "__smapi_runtime"
	smapiNexusModID    = 2400
)

// modsDir returns the host-side path to the mods directory.
// Mods live at: <dataDir>/.local-container/mods/
func modsDir(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "mods")
}

// disabledModsDir stores installed mods that should not be mounted into the
// current Stardew server process.
func disabledModsDir(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "mods-disabled")
}

// modManifest represents the SMAPI manifest.json structure.
type modManifest struct {
	Name              string                  `json:"Name"`
	UniqueID          string                  `json:"UniqueID"`
	Version           string                  `json:"Version"`
	Author            string                  `json:"Author"`
	Description       string                  `json:"Description"`
	MinimumApiVersion string                  `json:"MinimumApiVersion,omitempty"`
	UpdateKeys        []string                `json:"UpdateKeys,omitempty"`
	Dependencies      []modManifestDependency `json:"Dependencies,omitempty"`
	ContentPackFor    *modManifestDependency  `json:"ContentPackFor,omitempty"`
}

type modManifestDependency struct {
	UniqueID       string `json:"UniqueID"`
	MinimumVersion string `json:"MinimumVersion,omitempty"`
	IsRequired     *bool  `json:"IsRequired,omitempty"`
}

// parseNexusModIDFromUpdateKeys scans a SMAPI manifest's UpdateKeys for a
// "Nexus:<id>" entry (case-insensitive site name) and returns the numeric
// mod ID. Returns 0, false if none is present or parseable.
func parseNexusModIDFromUpdateKeys(updateKeys []string) (int, bool) {
	for _, key := range updateKeys {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "Nexus") {
			continue
		}
		idPart := strings.TrimSpace(parts[1])
		// Some UpdateKeys append a sub-key after the id, e.g. "Nexus:123:abc".
		if i := strings.IndexByte(idPart, ':'); i >= 0 {
			idPart = idPart[:i]
		}
		id, err := strconv.Atoi(idPart)
		if err != nil || id <= 0 {
			continue
		}
		return id, true
	}
	return 0, false
}

// ListMods scans the mods root directory for subdirectories, reads each manifest.json,
// and returns a list of ModInfo. Directories without a manifest are included with ParseError.
func ListMods(dataDir string) ([]registry.ModInfo, error) {
	mods, err := listModsFromRoot(modsDir(dataDir), true, true)
	if err != nil {
		return nil, err
	}
	return applyModDependencyStatus(mods), nil
}

// ListModsWithState returns active and disabled mods in one list. When
// saveName is set, persisted per-save enable settings are overlaid on the
// physical directory state for display.
func ListModsWithState(dataDir, saveName string) ([]registry.ModInfo, error) {
	mods, err := listPhysicalMods(dataDir)
	if err != nil {
		return nil, err
	}
	mods = ApplyModEnableProfile(dataDir, saveName, mods)
	return applyModDependencyStatus(mods), nil
}

func listPhysicalMods(dataDir string) ([]registry.ModInfo, error) {
	active, err := listModsFromRoot(modsDir(dataDir), true, true)
	if err != nil {
		return nil, err
	}
	disabled, err := listModsFromRoot(disabledModsDir(dataDir), false, false)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	mods := make([]registry.ModInfo, 0, len(active)+len(disabled))
	for _, mod := range active {
		mods = append(mods, mod)
		if mod.FolderName != "" {
			seen[mod.FolderName] = struct{}{}
		}
	}
	for _, mod := range disabled {
		if _, ok := seen[mod.FolderName]; ok {
			continue
		}
		mods = append(mods, mod)
	}
	return mods, nil
}

func listModsFromRoot(root string, enabled bool, includeSMAPIRuntime bool) ([]registry.ModInfo, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read mods dir: %w", err)
	}

	mods := make([]registry.ModInfo, 0, len(entries)+1)
	if includeSMAPIRuntime && hasControlModDir(entries) {
		mods = append(mods, smapiRuntimeModInfo())
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		folderName := e.Name()
		mod := readModInfo(filepath.Join(root, folderName), folderName)
		mod.Enabled = enabled
		mod.CanToggle = !mod.BuiltIn
		if mod.BuiltIn {
			mod.EnableNote = "鍐呯疆缁勪欢涓嶅彲绂佺敤"
		}
		mods = append(mods, mod)
	}
	return mods, nil
}

func hasControlModDir(entries []os.DirEntry) bool {
	for _, e := range entries {
		if e.IsDir() && e.Name() == controlModFolderName {
			return true
		}
	}
	return false
}

func smapiRuntimeModInfo() registry.ModInfo {
	return registry.ModInfo{
		ID:          smapiRuntimeModID,
		UniqueID:    "Pathoschild.SMAPI",
		Name:        "SMAPI",
		Author:      "Pathoschild",
		Description: "The mod loader for Stardew Valley.",
		FolderName:  "SMAPI",
		Enabled:     true,
		CanToggle:   false,
		EnableNote:  "SMAPI runtime cannot be disabled",
		SyncKind:    registry.ModSyncKindClientRequired,
		SyncNote:    "SMAPI is listed for player awareness but is not packaged as a normal mod.",
		BuiltIn:     true,
		UpdateKeys:  []string{"Nexus:2400"},
		NexusModID:  smapiNexusModID,
		NexusURL:    nexusModURL(smapiNexusModID),
	}
}

// readModInfo reads a single mod directory and parses its manifest.json.
func readModInfo(modPath, folderName string) registry.ModInfo {
	info := registry.ModInfo{
		ID:         folderName,
		FolderName: folderName,
		Enabled:    true,
		CanToggle:  true,
	}

	manifestPath := filepath.Join(modPath, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		info.ParseError = "缂哄皯 manifest.json"
		return info
	}

	var m modManifest
	if err := decodeModManifest(data, &m); err != nil {
		info.ParseError = "manifest.json 瑙ｆ瀽澶辫触: " + err.Error()
		return info
	}

	info.UniqueID = m.UniqueID
	info.Name = m.Name
	info.Version = m.Version
	info.Author = m.Author
	info.Description = m.Description
	info.UpdateKeys = m.UpdateKeys
	info.Dependencies = manifestDependencies(m)
	if m.ContentPackFor != nil {
		info.IsContentPack = true
		info.ContentPackFor = strings.TrimSpace(m.ContentPackFor.UniqueID)
	}
	if nexusID, ok := parseNexusModIDFromUpdateKeys(m.UpdateKeys); ok {
		info.NexusModID = nexusID
	}

	if info.UniqueID == "" {
		info.ParseError = "manifest.json 缂哄皯 UniqueID"
	}
	if info.Name == "" {
		info.ParseError = "manifest.json 缂哄皯 Name"
	}

	if isControlModInfo(info) {
		info.BuiltIn = true
		info.CanToggle = false
		info.EnableNote = "Built-in component cannot be disabled"
		info.SyncKind = registry.ModSyncKindServerOnly
		info.SyncNote = "Built-in server control mod; excluded from player sync packs."
	}

	return info
}

func decodeModManifest(data []byte, manifest *modManifest) error {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(data, manifest); err == nil {
		return nil
	}

	normalized := normalizeManifestJSON(data)
	if bytes.Equal(normalized, data) {
		return json.Unmarshal(data, manifest)
	}
	return json.Unmarshal(normalized, manifest)
}

func normalizeManifestJSON(data []byte) []byte {
	return stripJSONTrailingCommas(stripJSONComments(data))
}

func stripJSONComments(data []byte) []byte {
	var out bytes.Buffer
	out.Grow(len(data))
	inString := false
	escaped := false

	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out.WriteByte(c)
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			out.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(data) {
			next := data[i+1]
			if next == '/' {
				i += 2
				for i < len(data) && data[i] != '\n' && data[i] != '\r' {
					i++
				}
				if i < len(data) {
					out.WriteByte(data[i])
				}
				continue
			}
			if next == '*' {
				i += 2
				for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
					if data[i] == '\n' || data[i] == '\r' {
						out.WriteByte(data[i])
					}
					i++
				}
				if i+1 < len(data) {
					i++
				}
				continue
			}
		}
		out.WriteByte(c)
	}
	return out.Bytes()
}

func stripJSONTrailingCommas(data []byte) []byte {
	var out bytes.Buffer
	out.Grow(len(data))
	inString := false
	escaped := false

	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out.WriteByte(c)
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			out.WriteByte(c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(data) && (data[j] == ' ' || data[j] == '\t' || data[j] == '\r' || data[j] == '\n') {
				j++
			}
			if j < len(data) && (data[j] == '}' || data[j] == ']') {
				continue
			}
		}
		out.WriteByte(c)
	}
	return out.Bytes()
}

func manifestDependencies(m modManifest) []registry.ModDependency {
	deps := make([]registry.ModDependency, 0, len(m.Dependencies)+1)
	seen := map[string]struct{}{}
	add := func(dep modManifestDependency, requiredDefault bool) {
		uniqueID := strings.TrimSpace(dep.UniqueID)
		if uniqueID == "" {
			return
		}
		if _, ok := seen[uniqueID]; ok {
			return
		}
		required := requiredDefault
		if dep.IsRequired != nil {
			required = *dep.IsRequired
		}
		deps = append(deps, registry.ModDependency{
			UniqueID:       uniqueID,
			MinimumVersion: strings.TrimSpace(dep.MinimumVersion),
			Required:       required,
		})
		seen[uniqueID] = struct{}{}
	}
	if m.ContentPackFor != nil {
		add(*m.ContentPackFor, true)
	}
	for _, dep := range m.Dependencies {
		add(dep, true)
	}
	return deps
}

// FindModByUniqueID searches the mods directory for a mod with the given UniqueID.
// Returns the folder name if found, or empty string if not found.
func FindModByUniqueID(dataDir, uniqueID string) (string, error) {
	mods, err := listPhysicalMods(dataDir)
	if err != nil {
		return "", err
	}
	for _, m := range mods {
		if m.BuiltIn {
			continue
		}
		if m.UniqueID == uniqueID {
			return m.FolderName, nil
		}
	}
	return "", nil
}

// ValidateModName rejects dangerous mod folder names.
func ValidateModName(name string) error {
	if name == "" {
		return fmt.Errorf("mod 鍚嶇О涓嶈兘涓虹┖")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("mod 鍚嶇О涓嶈兘鏄?%q", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("mod name cannot contain path separators")
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("mod name cannot be an absolute path")
	}
	return nil
}

type uploadModZipOptions struct {
	inferNexusPackageOrigin bool
}

// UploadModZip validates a mod ZIP upload and extracts it to the mods directory.
// Returns the list of imported mod folder names.
func UploadModZip(dataDir, zipPath string) ([]registry.ModInfo, error) {
	return uploadModZip(dataDir, zipPath, uploadModZipOptions{inferNexusPackageOrigin: true})
}

func uploadModZip(dataDir, zipPath string, opts uploadModZipOptions) ([]registry.ModInfo, error) {
	stat, err := os.Stat(zipPath)
	if err != nil {
		return nil, fmt.Errorf("stat upload: %w", err)
	}
	if stat.Size() > maxModZipBytes {
		return nil, fmt.Errorf("鍘嬬缉鍖呰秴杩?%d MB 闄愬埗", maxModZipBytes/1024/1024)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("鎵撳紑 ZIP 澶辫触: %w", err)
	}
	defer func() { _ = zr.Close() }()

	// Security checks.
	if err := validateModZip(zr); err != nil {
		return nil, err
	}

	// Detect mod structure.
	modDirs, err := detectModDirs(zr)
	if err != nil {
		return nil, err
	}

	// Validate each top dir has a manifest.
	root := modsDir(dataDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create mods dir: %w", err)
	}

	// Check folder names before extracting.
	seenFolderNames := map[string]struct{}{}
	for _, dir := range modDirs {
		if err := ValidateModName(dir.FolderName); err != nil {
			return nil, fmt.Errorf("mod 鐩綍鍚嶄笉鍚堟硶: %w", err)
		}
		if _, dup := seenFolderNames[dir.FolderName]; dup {
			return nil, fmt.Errorf("ZIP 鍐?Mod 鐩綍 %q 閲嶅", dir.FolderName)
		}
		seenFolderNames[dir.FolderName] = struct{}{}
	}

	// Extract to temp dir first, then validate, then move atomically.
	td, err := os.MkdirTemp("", "stardew-mod-upload-*")
	if err != nil {
		return nil, fmt.Errorf("鍒涘缓涓存椂鐩綍: %w", err)
	}
	defer func() { _ = os.RemoveAll(td) }()

	if err := extractModZip(zr, td); err != nil {
		return nil, err
	}

	// 鈹€鈹€ Pre-validation: check ALL mods before moving any 鈹€鈹€
	type modCandidate struct {
		sourcePath string
		folderName string
		info       registry.ModInfo
	}
	candidates := make([]modCandidate, 0, len(modDirs))
	seenUniqueIDs := map[string]string{} // uniqueID 鈫?folderName (within this ZIP)

	for _, dir := range modDirs {
		modPath := filepath.Join(td, filepath.FromSlash(dir.SourcePath))
		info := readModInfo(modPath, dir.FolderName)
		if info.ParseError != "" {
			return nil, fmt.Errorf("mod %q 涓嶆槸鍚堟硶鐨?SMAPI Mod: %s", dir.FolderName, info.ParseError)
		}

		// Check for duplicate UniqueID within this ZIP.
		if prev, dup := seenUniqueIDs[info.UniqueID]; dup {
			return nil, fmt.Errorf("ZIP contains duplicate UniqueID %q in %q and %q", info.UniqueID, prev, dir.FolderName)
		}
		seenUniqueIDs[info.UniqueID] = dir.FolderName

		// Check for duplicate UniqueID against already-installed mods.
		existing, err := FindModByUniqueID(dataDir, info.UniqueID)
		if err != nil {
			return nil, fmt.Errorf("妫€鏌ュ凡鏈?Mod 澶辫触: %w", err)
		}
		if existing != "" {
			return nil, fmt.Errorf("UniqueID %q 宸插瓨鍦ㄤ簬 Mod %q 涓?(mod_exists)", info.UniqueID, existing)
		}

		// Check target directory doesn't already exist.
		dest := filepath.Join(root, dir.FolderName)
		if _, err := os.Stat(dest); err == nil {
			return nil, fmt.Errorf("mod folder %q already exists", dir.FolderName)
		}

		candidates = append(candidates, modCandidate{
			sourcePath: filepath.FromSlash(dir.SourcePath),
			folderName: dir.FolderName,
			info:       info,
		})
	}

	// 鈹€鈹€ All checks passed 鈥?move all mods with rollback on failure 鈹€鈹€
	var imported []registry.ModInfo
	var moved []string // tracks successfully moved dest dirs for rollback

	for _, c := range candidates {
		src := filepath.Join(td, c.sourcePath)
		dest := filepath.Join(root, c.folderName)
		if err := os.Rename(src, dest); err != nil {
			// Cross-filesystem fallback.
			if err := copyDir(src, dest); err != nil {
				// Rollback: remove all previously moved directories.
				for _, d := range moved {
					_ = os.RemoveAll(d)
				}
				return nil, fmt.Errorf("瀵煎叆 Mod %q 澶辫触: %w", c.folderName, err)
			}
		}
		moved = append(moved, dest)
		finalInfo := readModInfo(dest, c.folderName)
		imported = append(imported, finalInfo)
	}

	if opts.inferNexusPackageOrigin {
		if err := SaveInferredNexusPackageOrigin(dataDir, imported); err == nil {
			imported = ApplyNexusMetadataToMods(dataDir, imported)
		}
	}

	return imported, nil
}

// validateModZip performs security checks on the ZIP entries.
func validateModZip(zr *zip.ReadCloser) error {
	var totalUncompressed uint64
	for _, f := range zr.File {
		if f.FileInfo().Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("ZIP contains symbolic links")
		}
		name := filepath.ToSlash(f.Name)
		if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
			return fmt.Errorf("ZIP 鍖呭惈缁濆璺緞 %q", f.Name)
		}
		// Trim trailing "/" for directory entries.
		trimmed := strings.TrimSuffix(name, "/")
		for _, seg := range strings.Split(trimmed, "/") {
			if seg == ".." {
				return fmt.Errorf("ZIP 璺緞 %q 鍖呭惈鐩綍绌胯秺 (..)", f.Name)
			}
			if seg == "." {
				return fmt.Errorf("ZIP 璺緞 %q 鍖呭惈鏃犳晥鐨勫綋鍓嶇洰褰曞紩鐢?(.)", f.Name)
			}
			if seg == "" {
				return fmt.Errorf("ZIP 璺緞 %q 鍖呭惈绌鸿矾寰勬", f.Name)
			}
		}
		totalUncompressed += f.UncompressedSize64
		if f.UncompressedSize64 > maxModSingleFile {
			return fmt.Errorf("ZIP 鍐呭崟涓枃浠惰秴杩?%d MB", maxModSingleFile/1024/1024)
		}
		if totalUncompressed > maxModUncompressed {
			return fmt.Errorf("ZIP 瑙ｅ帇鎬诲ぇ灏忚秴杩?%d MB", maxModUncompressed/1024/1024)
		}
	}
	return nil
}

type modZipDir struct {
	SourcePath string
	FolderName string
}

// detectModDirs finds importable SMAPI mod directories in the ZIP.
// Normal SMAPI archives put one or more mod folders at the ZIP root. Some
// Nexus archives wrap those folders in one extra directory; for that common
// shape we strip only that single wrapper and import the child mod folders.
func detectModDirs(zr *zip.ReadCloser) ([]modZipDir, error) {
	topDirs := map[string]bool{}
	hasManifest := map[string]bool{}
	childManifests := map[string]map[string]bool{}
	anyManifest := false

	for _, f := range zr.File {
		name := strings.TrimSuffix(filepath.ToSlash(f.Name), "/")
		if name == "" {
			continue
		}
		parts := strings.Split(name, "/")
		if len(parts) == 0 || parts[0] == "" {
			continue
		}
		topDirs[parts[0]] = true
		// Check if this file is a manifest.json at the top level of a dir.
		if len(parts) >= 2 && pathBaseEqual(name, "manifest.json") {
			anyManifest = true
		}
		if len(parts) == 2 && parts[1] == "manifest.json" {
			hasManifest[parts[0]] = true
		}
		if len(parts) == 3 && parts[2] == "manifest.json" {
			if childManifests[parts[0]] == nil {
				childManifests[parts[0]] = map[string]bool{}
			}
			childManifests[parts[0]][parts[1]] = true
		}
	}

	if len(topDirs) == 0 {
		return nil, fmt.Errorf("ZIP is empty or has no valid files")
	}
	if !anyManifest && isLikelyXNBReplacementZip(zr) {
		return nil, fmt.Errorf("杩欐槸 XNB 鏇挎崲鍖咃紝涓嶆槸 SMAPI Mod锛屼笉鑳戒笂浼犲埌鏈嶅姟鍣?Mods 鐩綍锛涜浣跨敤甯?manifest.json 鐨?SMAPI 鎴?Content Patcher 鐗堟湰")
	}

	// If there's exactly one top dir with manifest.json, treat as single mod.
	if len(topDirs) == 1 {
		for name := range topDirs {
			if hasManifest[name] {
				return []modZipDir{{SourcePath: name, FolderName: name}}, nil
			}
			if children := childManifests[name]; len(children) > 0 {
				childNames := sortedMapKeys(children)
				dirs := make([]modZipDir, 0, len(childNames))
				for _, child := range childNames {
					dirs = append(dirs, modZipDir{
						SourcePath: name + "/" + child,
						FolderName: child,
					})
				}
				return dirs, nil
			}
			// Single dir without manifest 鈥?still try to extract, will fail validation later.
			return []modZipDir{{SourcePath: name, FolderName: name}}, nil
		}
	}

	// Multiple top dirs: each must have a manifest.
	names := sortedMapKeys(topDirs)
	dirs := make([]modZipDir, 0, len(names))
	for _, name := range names {
		if !hasManifest[name] {
			return nil, fmt.Errorf("ZIP 鍖呭惈澶氫釜椤跺眰鐩綍锛屼絾 %q 缂哄皯 manifest.json", name)
		}
		dirs = append(dirs, modZipDir{SourcePath: name, FolderName: name})
	}
	return dirs, nil
}

func sortedMapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func isLikelyXNBReplacementZip(zr *zip.ReadCloser) bool {
	hasXNB := false
	hasGameContentPath := false
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := strings.ToLower(filepath.ToSlash(f.Name))
		if pathBaseEqual(name, "manifest.json") {
			return false
		}
		if strings.HasSuffix(name, ".xnb") {
			hasXNB = true
		}
		if strings.Contains(name, "/characters/") ||
			strings.Contains(name, "/portraits/") ||
			strings.Contains(name, "/content/") ||
			strings.Contains(name, "/maps/") ||
			strings.Contains(name, "/tilesheets/") {
			hasGameContentPath = true
		}
	}
	return hasXNB && hasGameContentPath
}

func pathBaseEqual(name, want string) bool {
	parts := strings.Split(strings.TrimSuffix(filepath.ToSlash(name), "/"), "/")
	return len(parts) > 0 && parts[len(parts)-1] == want
}

// extractModZip extracts the ZIP to destDir with path escape verification.
func extractModZip(zr *zip.ReadCloser, destDir string) error {
	for _, f := range zr.File {
		if f.FileInfo().Mode()&fs.ModeSymlink != 0 {
			continue
		}
		outPath := filepath.Join(destDir, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(filepath.Clean(outPath)+string(os.PathSeparator), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("zip-slip detected for path %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(outPath, 0o755); err != nil {
				return fmt.Errorf("鍒涘缓鐩綍 %s: %w", outPath, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return fmt.Errorf("鍒涘缓鐖剁洰褰?%s: %w", filepath.Dir(outPath), err)
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
		return fmt.Errorf("file %s exceeds size limit after extraction", f.Name)
	}
	return nil
}

// DeleteMod removes a mod folder by UniqueID or folderName.
// Mods imported from the same Nexus package are treated as a bundle: deleting
// any member removes the other folders installed from that package too.
func DeleteMod(dataDir, modID string) error {
	targetFolder, err := ResolveModFolder(dataDir, modID)
	if err != nil {
		return err
	}
	if targetFolder == controlModFolderName {
		return fmt.Errorf("built-in mod %q cannot be deleted", controlModFolderName)
	}
	folders, err := resolveModDeleteFolders(dataDir, targetFolder)
	if err != nil {
		return err
	}
	for _, folder := range folders {
		if err := deleteModFolder(dataDir, folder); err != nil {
			return err
		}
	}
	_ = DeleteInstalledNexusMetadata(dataDir, folders)
	return nil
}

func resolveModDeleteFolders(dataDir, targetFolder string) ([]string, error) {
	mods, err := ListModsWithState(dataDir, "")
	if err != nil {
		return nil, err
	}
	mods = ApplyNexusMetadataToMods(dataDir, mods)
	var target registry.ModInfo
	for _, mod := range mods {
		if mod.FolderName == targetFolder && !mod.BuiltIn {
			target = mod
			break
		}
	}
	if target.FolderName == "" {
		return []string{targetFolder}, nil
	}
	bundleID := modNexusBundleID(target)
	if bundleID <= 0 {
		return []string{targetFolder}, nil
	}
	folders := make([]string, 0, len(mods))
	for _, mod := range mods {
		if mod.BuiltIn {
			continue
		}
		if modNexusBundleID(mod) == bundleID {
			folders = append(folders, mod.FolderName)
		}
	}
	if len(folders) == 0 {
		return []string{targetFolder}, nil
	}
	sort.Strings(folders)
	return folders, nil
}

func modNexusBundleID(mod registry.ModInfo) int {
	if mod.OriginSource == "nexus" && mod.OriginNexusModID > 0 {
		return mod.OriginNexusModID
	}
	return mod.NexusModID
}

func deleteModFolder(dataDir, folderName string) error {
	if folderName == controlModFolderName {
		return fmt.Errorf("built-in mod %q cannot be deleted", controlModFolderName)
	}
	if err := ValidateModName(folderName); err != nil {
		return err
	}
	deleted := false
	for _, root := range []string{modsDir(dataDir), disabledModsDir(dataDir)} {
		ok, err := deleteModFolderFromRoot(root, folderName)
		if err != nil {
			return err
		}
		deleted = deleted || ok
	}
	if !deleted {
		return fmt.Errorf("Mod %q does not exist", folderName)
	}
	return nil
}

func deleteModFolderFromRoot(root, folderName string) (bool, error) {
	targetPath := filepath.Join(root, folderName)
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false, fmt.Errorf("resolve mods root: %w", err)
	}
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return false, fmt.Errorf("resolve mod path: %w", err)
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, fmt.Errorf("invalid mod path %q", folderName)
	}
	if absTarget == absRoot {
		return false, fmt.Errorf("cannot delete mods root")
	}

	info, statErr := os.Stat(absTarget)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return false, nil
		}
		return false, fmt.Errorf("stat mod %q: %w", folderName, statErr)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("mod %q is not a directory", folderName)
	}

	if err := os.RemoveAll(absTarget); err != nil {
		return false, fmt.Errorf("delete mod %q: %w", folderName, err)
	}
	return true, nil
}

// addModDirToZip walks a single mod directory and writes its files into the
// ZIP writer using paths relative to root. Hidden files and temp files
// (trailing "~") are skipped.
func addModDirToZip(w *zip.Writer, root, dirName string) error {
	modPath := filepath.Join(root, dirName)
	return filepath.WalkDir(modPath, func(path string, d fs.DirEntry, walkErr error) error {
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
}

// ExportModsZip creates a ZIP archive of all mods in the mods directory.
// Returns the path to the created ZIP file. Caller owns the file and must clean it up.
func ExportModsZip(dataDir string) (string, error) {
	root := modsDir(dataDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("mods directory does not exist")
		}
		return "", fmt.Errorf("璇诲彇 mods 鐩綍: %w", err)
	}

	// Filter to directories only.
	var dirs []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		}
	}
	if len(dirs) == 0 {
		return "", fmt.Errorf("no installed mods to export")
	}

	// Build a human-readable ZIP name: mod鍚峗浣滆€呭悕.zip for single mod,
	// or stardew-mods-N.zip for multiple mods.
	zipName := buildModsZipName(root, dirs)
	tmpPath := filepath.Join(os.TempDir(), zipName)

	zf, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("鍒涘缓 ZIP 鏂囦欢: %w", err)
	}
	defer func() {
		if err != nil {
			_ = zf.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	w := zip.NewWriter(zf)
	for _, dir := range dirs {
		if err = addModDirToZip(w, root, dir.Name()); err != nil {
			return "", fmt.Errorf("鍐欏叆 Mod %q 澶辫触: %w", dir.Name(), err)
		}
	}

	if err = w.Close(); err != nil {
		return "", fmt.Errorf("鍏抽棴 ZIP: %w", err)
	}
	if err = zf.Close(); err != nil {
		return "", fmt.Errorf("鍏抽棴鏂囦欢: %w", err)
	}

	return tmpPath, nil
}

// buildModsZipName constructs a human-readable ZIP filename for a mods export.
// Single mod: "mod鍚峗浣滆€呭悕.zip"
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
