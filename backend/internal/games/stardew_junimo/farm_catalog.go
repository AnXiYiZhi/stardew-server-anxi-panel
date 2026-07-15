package stardew_junimo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	farmCatalogMaxFileBytes    int64 = 2 * 1024 * 1024
	farmCatalogMaxTotalBytes   int64 = 16 * 1024 * 1024
	farmCatalogMaxDepth              = 16
	farmCatalogMaxIDBytes            = 128
	contentPatcherModID              = "Pathoschild.ContentPatcher"
	defaultFarmCatalogLanguage       = "zh-CN"
)

type FarmCatalogConfidence string

const (
	FarmCatalogConfidenceExplicit FarmCatalogConfidence = "explicit"
	FarmCatalogConfidenceEntryKey FarmCatalogConfidence = "entry_key_fallback"
)

type FarmCatalogDependency struct {
	UniqueID       string
	MinimumVersion string
	Required       bool
}

type FarmCatalogMod struct {
	Name           string
	UniqueID       string
	Version        string
	ContentPackFor *FarmCatalogDependency
	Dependencies   []FarmCatalogDependency
	Folder         string
	Enabled        bool
	ParseWarnings  []string
}

type FarmCatalogCondition struct {
	Kind       string
	SourceFile string
	When       json.RawMessage
}

type FarmCatalogSource struct {
	ProviderModID   string
	ProviderFolder  string
	ProviderName    string
	ProviderVersion string
	Enabled         bool
}

type FarmCatalogEntry struct {
	ID                string
	Label             string
	Description       string
	EntryKey          string
	MapName           string
	TooltipStringPath string
	IconTexture       string
	WorldMapTexture   string
	IconFile          string
	IconMediaType     string
	IconWidth         int
	IconHeight        int
	ProviderModID     string
	ProviderFolder    string
	ProviderName      string
	ProviderVersion   string
	Enabled           bool
	Confidence        FarmCatalogConfidence
	Conditions        []FarmCatalogCondition
	ParseWarnings     []string
	Conflict          bool
	ConflictSources   []FarmCatalogSource
}

type FarmCatalogConflict struct {
	ID      string
	Sources []FarmCatalogSource
}

type FarmCatalogResult struct {
	Language  string
	Mods      []FarmCatalogMod
	Farms     []FarmCatalogEntry
	Conflicts []FarmCatalogConflict
	Warnings  []string
}

type FarmCatalogOptions struct {
	Language string
}

// ScanFarmCatalog inspects installed Content Patcher packs without requiring a
// running game server. It only recognizes explicit Data/AdditionalFarms edits.
func ScanFarmCatalog(dataDir string) (FarmCatalogResult, error) {
	return ScanFarmCatalogWithOptions(dataDir, FarmCatalogOptions{Language: defaultFarmCatalogLanguage})
}

// ScanFarmCatalogWithOptions applies display-language preferences while
// retaining the same offline-only scan and filesystem safety boundaries.
func ScanFarmCatalogWithOptions(dataDir string, options FarmCatalogOptions) (FarmCatalogResult, error) {
	language := normalizeFarmCatalogLanguage(options.Language)
	result := FarmCatalogResult{
		Language:  language,
		Mods:      []FarmCatalogMod{},
		Farms:     []FarmCatalogEntry{},
		Conflicts: []FarmCatalogConflict{},
		Warnings:  []string{},
	}
	for _, root := range []struct {
		path    string
		enabled bool
	}{
		{path: modsDir(dataDir), enabled: true},
		{path: disabledModsDir(dataDir), enabled: false},
	} {
		if err := scanFarmCatalogRoot(root.path, root.enabled, language, &result); err != nil {
			return result, err
		}
	}
	finalizeFarmCatalog(&result)
	return result, nil
}

func scanFarmCatalogRoot(root string, enabled bool, language string, result *FarmCatalogResult) error {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("farm catalog mods directory cannot be read")
	}
	for _, dirEntry := range entries {
		if !dirEntry.IsDir() {
			continue
		}
		folder := dirEntry.Name()
		modRoot := filepath.Join(root, folder)
		ctx := farmCatalogScanContext{modRoot: modRoot, totalRemaining: farmCatalogMaxTotalBytes}
		manifestData, err := ctx.readFile("manifest.json")
		catalogMod := FarmCatalogMod{Folder: folder, Enabled: enabled, Dependencies: []FarmCatalogDependency{}, ParseWarnings: []string{}}
		if err != nil {
			ctx.warn("manifest.json: %v", err)
			catalogMod.ParseWarnings = append(catalogMod.ParseWarnings, ctx.warnings...)
			result.Mods = append(result.Mods, catalogMod)
			result.Warnings = append(result.Warnings, prefixFarmWarnings(folder, ctx.warnings)...)
			continue
		}
		var manifest modManifest
		if err := decodeJSONC(manifestData, &manifest); err != nil {
			ctx.warn("manifest.json parse failed: %v", err)
			catalogMod.ParseWarnings = append(catalogMod.ParseWarnings, ctx.warnings...)
			result.Mods = append(result.Mods, catalogMod)
			result.Warnings = append(result.Warnings, prefixFarmWarnings(folder, ctx.warnings)...)
			continue
		}
		catalogMod.Name = manifest.Name
		catalogMod.UniqueID = manifest.UniqueID
		catalogMod.Version = manifest.Version
		catalogMod.Dependencies = farmCatalogDependencies(manifest.Dependencies)
		if manifest.ContentPackFor != nil {
			dep := farmCatalogDependency(*manifest.ContentPackFor, true)
			catalogMod.ContentPackFor = &dep
		}
		ctx.provider = catalogMod
		if manifest.ContentPackFor != nil && strings.EqualFold(strings.TrimSpace(manifest.ContentPackFor.UniqueID), contentPatcherModID) {
			ctx.scanDocument("content.json", nil, nil, 0)
		}
		baseWarnings := append([]string{}, ctx.warnings...)
		for i := range ctx.farms {
			ctx.farms[i].ParseWarnings = append(ctx.farms[i].ParseWarnings, baseWarnings...)
		}
		ctx.resolveDisplayMetadata(language)
		catalogMod.ParseWarnings = append(catalogMod.ParseWarnings, ctx.warnings...)
		result.Mods = append(result.Mods, catalogMod)
		result.Farms = append(result.Farms, ctx.farms...)
		result.Warnings = append(result.Warnings, prefixFarmWarnings(folder, ctx.warnings)...)
	}
	return nil
}

type farmCatalogScanContext struct {
	modRoot        string
	provider       FarmCatalogMod
	totalRemaining int64
	activeIncludes map[string]bool
	warnings       []string
	farms          []FarmCatalogEntry
	stringsUI      map[string]string
	loadFiles      map[string]string
	i18nCache      map[string]farmCatalogI18nCacheEntry
	iconCache      map[string]farmCatalogIconCacheEntry
}

func (c *farmCatalogScanContext) warn(format string, args ...any) {
	c.warnings = append(c.warnings, fmt.Sprintf(format, args...))
}

func (c *farmCatalogScanContext) readFile(relative string) ([]byte, error) {
	clean, err := safeFarmCatalogRelativePath(relative)
	if err != nil {
		return nil, err
	}
	realRoot, err := filepath.EvalSymlinks(c.modRoot)
	if err != nil {
		return nil, fmt.Errorf("mod root cannot be resolved safely")
	}
	candidate := filepath.Join(c.modRoot, filepath.FromSlash(clean))
	realCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file does not exist: %w", fs.ErrNotExist)
		}
		return nil, fmt.Errorf("file cannot be resolved safely")
	}
	inside, err := filepath.Rel(realRoot, realCandidate)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) || filepath.IsAbs(inside) {
		return nil, fmt.Errorf("path escapes mod root")
	}
	info, err := os.Stat(realCandidate)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file does not exist: %w", fs.ErrNotExist)
		}
		return nil, fmt.Errorf("file metadata cannot be read safely")
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	if info.Size() > farmCatalogMaxFileBytes {
		return nil, fmt.Errorf("file exceeds %d-byte limit", farmCatalogMaxFileBytes)
	}
	if info.Size() > c.totalRemaining {
		return nil, fmt.Errorf("mod scan exceeds %d-byte total limit", farmCatalogMaxTotalBytes)
	}
	f, err := os.Open(realCandidate)
	if err != nil {
		return nil, fmt.Errorf("file cannot be opened safely")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, farmCatalogMaxFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("file cannot be read safely")
	}
	if int64(len(data)) > farmCatalogMaxFileBytes {
		return nil, fmt.Errorf("file exceeds %d-byte limit", farmCatalogMaxFileBytes)
	}
	c.totalRemaining -= int64(len(data))
	return data, nil
}

func safeFarmCatalogRelativePath(value string) (string, error) {
	if value == "" || strings.ContainsRune(value, '\x00') || strings.Contains(value, "{{") {
		return "", fmt.Errorf("include path must be a static local relative path")
	}
	value = strings.ReplaceAll(value, "\\", "/")
	looksLikeWindowsVolume := len(value) >= 2 && ((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z')) && value[1] == ':'
	if strings.HasPrefix(value, "/") || filepath.IsAbs(value) || path.IsAbs(value) || filepath.VolumeName(value) != "" || looksLikeWindowsVolume {
		return "", fmt.Errorf("absolute include path is not allowed")
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("include path escapes mod root")
	}
	return clean, nil
}

func (c *farmCatalogScanContext) scanDocument(relative string, inherited []FarmCatalogCondition, includeStack []string, depth int) {
	if depth > farmCatalogMaxDepth {
		c.warn("%s: include depth exceeds %d", relative, farmCatalogMaxDepth)
		return
	}
	clean, err := safeFarmCatalogRelativePath(relative)
	if err != nil {
		c.warn("%s: %v", relative, err)
		return
	}
	if c.activeIncludes == nil {
		c.activeIncludes = map[string]bool{}
	}
	cycleKey := strings.ToLower(filepath.Clean(clean))
	if c.activeIncludes[cycleKey] {
		c.warn("%s: include cycle detected (%s)", clean, strings.Join(append(includeStack, clean), " -> "))
		return
	}
	c.activeIncludes[cycleKey] = true
	defer delete(c.activeIncludes, cycleKey)

	data, err := c.readFile(clean)
	if err != nil {
		c.warn("%s: %v", clean, err)
		return
	}
	var raw json.RawMessage
	if err := decodeJSONC(data, &raw); err != nil {
		c.warn("%s: JSON parse failed: %v", clean, err)
		return
	}
	patches, err := farmCatalogPatches(raw)
	if err != nil {
		c.warn("%s: %v", clean, err)
		return
	}
	for _, patchRaw := range patches {
		c.scanPatch(clean, patchRaw, inherited, append(includeStack, clean), depth)
	}
}

func farmCatalogPatches(raw json.RawMessage) ([]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty JSON document")
	}
	if trimmed[0] == '[' {
		var patches []json.RawMessage
		if err := json.Unmarshal(trimmed, &patches); err != nil {
			return nil, err
		}
		return patches, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil {
		return nil, err
	}
	if changes, ok := object["Changes"]; ok {
		var patches []json.RawMessage
		if err := json.Unmarshal(changes, &patches); err != nil {
			return nil, fmt.Errorf("Changes must be an array: %w", err)
		}
		return patches, nil
	}
	if _, ok := object["Action"]; ok {
		return []json.RawMessage{trimmed}, nil
	}
	return []json.RawMessage{}, nil
}

func (c *farmCatalogScanContext) scanPatch(source string, raw json.RawMessage, inherited []FarmCatalogCondition, includeStack []string, depth int) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		c.warn("%s: patch is not an object", source)
		return
	}
	var action string
	if err := json.Unmarshal(fields["Action"], &action); err != nil {
		return
	}
	conditions := append([]FarmCatalogCondition{}, inherited...)
	if when, ok := fields["When"]; ok && !bytes.Equal(bytes.TrimSpace(when), []byte("null")) {
		canonical, err := canonicalRawJSON(when)
		if err != nil {
			c.warn("%s: invalid When condition: %v", source, err)
		} else {
			conditions = append(conditions, FarmCatalogCondition{Kind: "patch", SourceFile: source, When: canonical})
		}
	}
	if strings.EqualFold(strings.TrimSpace(action), "Include") {
		var fromFile string
		if err := json.Unmarshal(fields["FromFile"], &fromFile); err != nil {
			c.warn("%s: Include FromFile must be a string", source)
			return
		}
		if len(conditions) > len(inherited) {
			conditions[len(conditions)-1].Kind = "include"
		}
		c.scanDocument(fromFile, conditions, includeStack, depth+1)
		return
	}
	if strings.EqualFold(strings.TrimSpace(action), "Load") {
		c.scanLoadPatch(source, fields)
		return
	}
	if !strings.EqualFold(strings.TrimSpace(action), "EditData") {
		return
	}
	var target string
	if err := json.Unmarshal(fields["Target"], &target); err != nil {
		return
	}
	if isStringsUITarget(target) {
		c.scanStringsUIPatch(source, fields)
		return
	}
	if !isAdditionalFarmsTarget(target) {
		return
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(fields["Entries"], &entries); err != nil {
		c.warn("%s: AdditionalFarms Entries must be an object", source)
		return
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		c.scanFarmEntry(source, key, entries[key], conditions)
	}
}

func (c *farmCatalogScanContext) scanFarmEntry(source, entryKey string, raw json.RawMessage, conditions []FarmCatalogCondition) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		c.warn("%s: AdditionalFarms entry %q must be an object", source, entryKey)
		return
	}
	id, confidence, ok := farmCatalogEntryID(fields, entryKey)
	if !ok {
		c.warn("%s: AdditionalFarms entry %q has no usable ID", source, entryKey)
		return
	}
	if err := validateFarmCatalogID(id); err != nil {
		c.warn("%s: AdditionalFarms entry %q has invalid ID: %v", source, entryKey, err)
		return
	}
	entry := FarmCatalogEntry{
		ID: id, EntryKey: entryKey,
		MapName: farmCatalogString(fields, "MapName"), TooltipStringPath: farmCatalogString(fields, "TooltipStringPath"),
		IconTexture: farmCatalogString(fields, "IconTexture"), WorldMapTexture: farmCatalogString(fields, "WorldMapTexture"),
		ProviderModID: c.provider.UniqueID, ProviderFolder: c.provider.Folder, ProviderName: c.provider.Name,
		ProviderVersion: c.provider.Version, Enabled: c.provider.Enabled, Confidence: confidence,
		Conditions: append([]FarmCatalogCondition{}, conditions...), ParseWarnings: []string{}, ConflictSources: []FarmCatalogSource{},
	}
	c.farms = append(c.farms, entry)
}

func farmCatalogEntryID(fields map[string]json.RawMessage, key string) (string, FarmCatalogConfidence, bool) {
	if raw, ok := fields["ID"]; ok {
		var id string
		if json.Unmarshal(raw, &id) == nil {
			return id, FarmCatalogConfidenceExplicit, true
		}
		return "", FarmCatalogConfidenceExplicit, false
	}
	if raw, ok := fields["Id"]; ok {
		var id string
		if json.Unmarshal(raw, &id) == nil {
			return id, FarmCatalogConfidenceExplicit, true
		}
		return "", FarmCatalogConfidenceExplicit, false
	}
	if isSimpleFarmCatalogID(key) {
		return key, FarmCatalogConfidenceEntryKey, true
	}
	return "", FarmCatalogConfidenceEntryKey, false
}

func isSimpleFarmCatalogID(value string) bool {
	if value == "" || strings.ContainsAny(value, "/\\") {
		return false
	}
	for _, r := range value {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.') {
			return false
		}
	}
	return validateFarmCatalogID(value) == nil
}

func validateFarmCatalogID(id string) error {
	if id == "" {
		return fmt.Errorf("empty")
	}
	if !utf8.ValidString(id) {
		return fmt.Errorf("not valid UTF-8")
	}
	if len(id) > farmCatalogMaxIDBytes {
		return fmt.Errorf("longer than %d bytes", farmCatalogMaxIDBytes)
	}
	for _, r := range id {
		if unicode.IsControl(r) {
			return fmt.Errorf("contains control characters")
		}
	}
	return nil
}

func isAdditionalFarmsTarget(target string) bool {
	normalized := strings.ReplaceAll(strings.TrimSpace(target), "\\", "/")
	normalized = strings.TrimPrefix(path.Clean("/"+normalized), "/")
	return strings.EqualFold(normalized, "Data/AdditionalFarms")
}

func decodeJSONC(data []byte, target any) error {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	normalized := normalizeManifestJSON(data)
	if !utf8.Valid(normalized) {
		return fmt.Errorf("JSON is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(normalized))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func canonicalRawJSON(raw json.RawMessage) (json.RawMessage, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func farmCatalogString(fields map[string]json.RawMessage, key string) string {
	var value string
	_ = json.Unmarshal(fields[key], &value)
	return value
}

func farmCatalogDependency(dep modManifestDependency, requiredDefault bool) FarmCatalogDependency {
	required := requiredDefault
	if dep.IsRequired != nil {
		required = *dep.IsRequired
	}
	return FarmCatalogDependency{UniqueID: dep.UniqueID, MinimumVersion: dep.MinimumVersion, Required: required}
}

func farmCatalogDependencies(deps []modManifestDependency) []FarmCatalogDependency {
	result := make([]FarmCatalogDependency, 0, len(deps))
	for _, dep := range deps {
		result = append(result, farmCatalogDependency(dep, true))
	}
	return result
}

func prefixFarmWarnings(folder string, warnings []string) []string {
	result := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		result = append(result, fmt.Sprintf("%s: %s", folder, warning))
	}
	return result
}

func finalizeFarmCatalog(result *FarmCatalogResult) {
	result.Farms = deduplicateFarmCatalogEntries(result.Farms)
	sort.SliceStable(result.Mods, func(i, j int) bool {
		if result.Mods[i].Enabled != result.Mods[j].Enabled {
			return result.Mods[i].Enabled
		}
		return strings.ToLower(result.Mods[i].Folder) < strings.ToLower(result.Mods[j].Folder)
	})
	sort.SliceStable(result.Farms, func(i, j int) bool {
		return farmCatalogEntrySortKey(result.Farms[i]) < farmCatalogEntrySortKey(result.Farms[j])
	})
	byID := map[string][]int{}
	for i := range result.Farms {
		byID[result.Farms[i].ID] = append(byID[result.Farms[i].ID], i)
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		indexes := byID[id]
		providerSet := map[string]FarmCatalogSource{}
		for _, index := range indexes {
			source := farmCatalogSource(result.Farms[index])
			providerSet[source.ProviderModID+"\x00"+source.ProviderFolder] = source
		}
		if len(providerSet) < 2 {
			continue
		}
		sources := make([]FarmCatalogSource, 0, len(providerSet))
		for _, source := range providerSet {
			sources = append(sources, source)
		}
		sort.Slice(sources, func(i, j int) bool {
			return farmCatalogSourceSortKey(sources[i]) < farmCatalogSourceSortKey(sources[j])
		})
		result.Conflicts = append(result.Conflicts, FarmCatalogConflict{ID: id, Sources: append([]FarmCatalogSource{}, sources...)})
		for _, index := range indexes {
			result.Farms[index].Conflict = true
			result.Farms[index].ConflictSources = append([]FarmCatalogSource{}, sources...)
		}
	}
	sort.Strings(result.Warnings)
}

func deduplicateFarmCatalogEntries(entries []FarmCatalogEntry) []FarmCatalogEntry {
	seen := map[string]bool{}
	result := make([]FarmCatalogEntry, 0, len(entries))
	for _, entry := range entries {
		data, _ := json.Marshal(struct {
			ID, Label, Description, EntryKey, MapName, Tooltip, Icon, World, IconFile, IconMediaType, ProviderID, ProviderFolder, ProviderName, ProviderVersion string
			IconWidth, IconHeight                                                                                                                               int
			Enabled                                                                                                                                             bool
			Confidence                                                                                                                                          FarmCatalogConfidence
			Conditions                                                                                                                                          []FarmCatalogCondition
		}{entry.ID, entry.Label, entry.Description, entry.EntryKey, entry.MapName, entry.TooltipStringPath, entry.IconTexture, entry.WorldMapTexture, entry.IconFile, entry.IconMediaType,
			entry.ProviderModID, entry.ProviderFolder, entry.ProviderName, entry.ProviderVersion,
			entry.IconWidth, entry.IconHeight, entry.Enabled, entry.Confidence, entry.Conditions})
		key := string(data)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, entry)
	}
	return result
}

func farmCatalogSource(entry FarmCatalogEntry) FarmCatalogSource {
	return FarmCatalogSource{ProviderModID: entry.ProviderModID, ProviderFolder: entry.ProviderFolder, ProviderName: entry.ProviderName, ProviderVersion: entry.ProviderVersion, Enabled: entry.Enabled}
}

func farmCatalogEntrySortKey(entry FarmCatalogEntry) string {
	return entry.ID + "\x00" + strings.ToLower(entry.ProviderModID) + "\x00" + strings.ToLower(entry.ProviderFolder) + "\x00" + entry.EntryKey
}

func farmCatalogSourceSortKey(source FarmCatalogSource) string {
	return strings.ToLower(source.ProviderModID) + "\x00" + strings.ToLower(source.ProviderFolder)
}
