package stardew_junimo

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	farmCatalogMaxLabelRunes       = 80
	farmCatalogMaxDescriptionRunes = 512
	farmCatalogMaxI18nKeyBytes     = 256
	farmCatalogMaxIconDimension    = 4096
	farmCatalogMaxIconPixels       = 16 * 1024 * 1024
)

type farmCatalogI18nCacheEntry struct {
	values map[string]string
	exists bool
	err    error
}

type farmCatalogIconCacheEntry struct {
	mediaType string
	width     int
	height    int
	err       error
}

type FarmCatalogIconAsset struct {
	Data      []byte
	MediaType string
	Width     int
	Height    int
}

// ReadFarmCatalogIcon revalidates a scanner-produced icon token immediately
// before serving it. Callers must pass an entry from a fresh catalog scan; URL
// or request parameters must never be copied into ProviderFolder or IconFile.
func ReadFarmCatalogIcon(dataDir string, entry FarmCatalogEntry) (FarmCatalogIconAsset, error) {
	if entry.IconFile == "" || entry.ProviderFolder == "" {
		return FarmCatalogIconAsset{}, fmt.Errorf("farm icon is unavailable")
	}
	folder := entry.ProviderFolder
	if folder == "." || folder == ".." || filepath.IsAbs(folder) || filepath.VolumeName(folder) != "" || strings.ContainsAny(folder, "/\\") || filepath.Clean(folder) != folder {
		return FarmCatalogIconAsset{}, fmt.Errorf("farm icon provider is invalid")
	}
	root := modsDir(dataDir)
	if !entry.Enabled {
		root = disabledModsDir(dataDir)
	}
	ctx := farmCatalogScanContext{modRoot: filepath.Join(root, folder), totalRemaining: farmCatalogMaxTotalBytes}
	clean, err := safeFarmCatalogRelativePath(entry.IconFile)
	if err != nil || clean != entry.IconFile {
		return FarmCatalogIconAsset{}, fmt.Errorf("farm icon token is invalid")
	}
	extension := strings.ToLower(filepath.Ext(clean))
	if extension != ".png" && extension != ".jpg" && extension != ".jpeg" && extension != ".webp" {
		return FarmCatalogIconAsset{}, fmt.Errorf("farm icon format is invalid")
	}
	data, err := ctx.readFile(clean)
	if err != nil {
		return FarmCatalogIconAsset{}, fmt.Errorf("farm icon is unavailable")
	}
	mediaType, width, height, err := inspectFarmCatalogIcon(data, extension)
	if err != nil {
		return FarmCatalogIconAsset{}, fmt.Errorf("farm icon is unavailable")
	}
	return FarmCatalogIconAsset{Data: data, MediaType: mediaType, Width: width, Height: height}, nil
}

func normalizeFarmCatalogLanguage(language string) string {
	language = strings.TrimSpace(strings.ReplaceAll(language, "_", "-"))
	if language == "" {
		return defaultFarmCatalogLanguage
	}
	parts := strings.Split(language, "-")
	for _, part := range parts {
		if part == "" {
			return defaultFarmCatalogLanguage
		}
		for _, r := range part {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
				return defaultFarmCatalogLanguage
			}
		}
	}
	parts[0] = strings.ToLower(parts[0])
	if len(parts) > 1 && len(parts[1]) == 2 {
		parts[1] = strings.ToUpper(parts[1])
	}
	return strings.Join(parts, "-")
}

func farmCatalogLanguageCandidates(language string) []string {
	language = normalizeFarmCatalogLanguage(language)
	candidates := []string{language}
	if base, _, ok := strings.Cut(language, "-"); ok {
		candidates = append(candidates, base)
	}
	candidates = append(candidates, "default")
	seen := map[string]bool{}
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		key := strings.ToLower(candidate)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, candidate)
	}
	return result
}

func isStringsUITarget(target string) bool {
	return strings.EqualFold(normalizeFarmCatalogAssetTarget(target), "Strings/UI")
}

func normalizeFarmCatalogAssetTarget(target string) string {
	target = strings.ReplaceAll(strings.TrimSpace(target), "\\", "/")
	return strings.Trim(target, "/")
}

func (c *farmCatalogScanContext) scanStringsUIPatch(source string, fields map[string]json.RawMessage) {
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(fields["Entries"], &entries); err != nil {
		c.warn("%s: Strings/UI Entries must be an object", source)
		return
	}
	if c.stringsUI == nil {
		c.stringsUI = map[string]string{}
	}
	for key, raw := range entries {
		var value string
		if json.Unmarshal(raw, &value) == nil {
			c.stringsUI[key] = value
		}
	}
}

func (c *farmCatalogScanContext) scanLoadPatch(source string, fields map[string]json.RawMessage) {
	var target, fromFile string
	if json.Unmarshal(fields["Target"], &target) != nil || json.Unmarshal(fields["FromFile"], &fromFile) != nil {
		return
	}
	target = normalizeFarmCatalogAssetTarget(target)
	if target == "" {
		return
	}
	if c.loadFiles == nil {
		c.loadFiles = map[string]string{}
	}
	c.loadFiles[strings.ToLower(target)] = fromFile
}

func (c *farmCatalogScanContext) resolveDisplayMetadata(language string) {
	for i := range c.farms {
		c.resolveFarmLabel(i, language)
		c.resolveFarmIcon(i)
	}
}

func (c *farmCatalogScanContext) resolveFarmLabel(index int, language string) {
	farm := &c.farms[index]
	farm.Label = farmCatalogFallbackLabel(c.provider.Name, farm.ID)
	if farm.TooltipStringPath == "" {
		return
	}
	key, ok := farmCatalogTooltipKey(farm.TooltipStringPath)
	if !ok {
		c.warnFarm(index, "unsupported TooltipStringPath %q", farm.TooltipStringPath)
		return
	}
	value, ok := c.stringsUI[key]
	if !ok {
		c.warnFarm(index, "Strings/UI entry %q was not found", key)
		return
	}
	i18nKey, ok := farmCatalogI18nToken(value)
	if !ok {
		c.warnFarm(index, "Strings/UI entry %q is not a supported exact i18n token", key)
		return
	}
	var loadWarnings []string
	for _, locale := range farmCatalogLanguageCandidates(language) {
		values, exists, err := c.loadI18n(locale)
		if err != nil {
			loadWarnings = append(loadWarnings, fmt.Sprintf("i18n/%s.json could not be parsed safely", locale))
			continue
		}
		if !exists {
			continue
		}
		raw, exists := values[i18nKey]
		if !exists {
			continue
		}
		label, description := farmCatalogSplitI18nValue(raw)
		if label == "" {
			continue
		}
		farm.Label = label
		farm.Description = description
		for _, warning := range loadWarnings {
			c.warnFarm(index, "%s", warning)
		}
		return
	}
	for _, warning := range loadWarnings {
		c.warnFarm(index, "%s", warning)
	}
	c.warnFarm(index, "i18n key %q was not found with a usable title", i18nKey)
}

func farmCatalogTooltipKey(value string) (string, bool) {
	prefix, key, ok := strings.Cut(strings.TrimSpace(value), ":")
	if !ok || !strings.EqualFold(strings.TrimSpace(prefix), "Strings/UI") {
		return "", false
	}
	key = strings.TrimSpace(key)
	return key, key != ""
}

func farmCatalogI18nToken(value string) (string, bool) {
	value = strings.TrimSpace(value)
	const prefix = "{{i18n:"
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, "}}") {
		return "", false
	}
	key := strings.TrimSpace(value[len(prefix) : len(value)-2])
	if key == "" || len(key) > farmCatalogMaxI18nKeyBytes || !utf8.ValidString(key) || strings.ContainsAny(key, "{}|") {
		return "", false
	}
	for _, r := range key {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return key, true
}

func (c *farmCatalogScanContext) loadI18n(locale string) (map[string]string, bool, error) {
	if c.i18nCache == nil {
		c.i18nCache = map[string]farmCatalogI18nCacheEntry{}
	}
	cacheKey := strings.ToLower(locale)
	if cached, ok := c.i18nCache[cacheKey]; ok {
		return cached.values, cached.exists, cached.err
	}
	data, err := c.readFile("i18n/" + locale + ".json")
	if err != nil {
		entry := farmCatalogI18nCacheEntry{exists: !errors.Is(err, fs.ErrNotExist), err: err}
		if errors.Is(err, fs.ErrNotExist) {
			entry.err = nil
		}
		c.i18nCache[cacheKey] = entry
		return entry.values, entry.exists, entry.err
	}
	var raw map[string]json.RawMessage
	if err := decodeJSONC(data, &raw); err != nil {
		entry := farmCatalogI18nCacheEntry{exists: true, err: err}
		c.i18nCache[cacheKey] = entry
		return nil, true, err
	}
	values := make(map[string]string, len(raw))
	for key, rawValue := range raw {
		var value string
		if json.Unmarshal(rawValue, &value) == nil {
			values[key] = value
		}
	}
	entry := farmCatalogI18nCacheEntry{values: values, exists: true}
	c.i18nCache[cacheKey] = entry
	return values, true, nil
}

func farmCatalogSplitI18nValue(value string) (string, string) {
	value = farmCatalogRemoveControlCharacters(value)
	label, description, found := strings.Cut(value, "_")
	if !found {
		label = value
		description = ""
	}
	label = truncateFarmCatalogRunes(strings.TrimSpace(label), farmCatalogMaxLabelRunes)
	description = truncateFarmCatalogRunes(strings.TrimSpace(description), farmCatalogMaxDescriptionRunes)
	return label, description
}

func farmCatalogFallbackLabel(manifestName, id string) string {
	label := truncateFarmCatalogRunes(strings.TrimSpace(farmCatalogRemoveControlCharacters(manifestName)), farmCatalogMaxLabelRunes)
	if label != "" {
		return label
	}
	return truncateFarmCatalogRunes(strings.TrimSpace(farmCatalogRemoveControlCharacters(id)), farmCatalogMaxLabelRunes)
}

func farmCatalogRemoveControlCharacters(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
}

func truncateFarmCatalogRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func (c *farmCatalogScanContext) resolveFarmIcon(index int) {
	farm := &c.farms[index]
	if strings.TrimSpace(farm.IconTexture) == "" {
		c.warnFarm(index, "IconTexture is empty")
		return
	}
	fromFile, ok := c.loadFiles[strings.ToLower(normalizeFarmCatalogAssetTarget(farm.IconTexture))]
	if !ok {
		c.warnFarm(index, "no Load patch maps IconTexture %q", farm.IconTexture)
		return
	}
	clean, err := safeFarmCatalogRelativePath(fromFile)
	if err != nil {
		c.warnFarm(index, "icon path %q is unsafe", fromFile)
		return
	}
	extension := strings.ToLower(filepath.Ext(clean))
	if extension != ".png" && extension != ".jpg" && extension != ".jpeg" && extension != ".webp" {
		c.warnFarm(index, "icon file %q has unsupported format", clean)
		return
	}
	if c.iconCache == nil {
		c.iconCache = map[string]farmCatalogIconCacheEntry{}
	}
	cacheKey := strings.ToLower(clean)
	icon, cached := c.iconCache[cacheKey]
	if !cached {
		data, readErr := c.readFile(clean)
		if readErr != nil {
			if errors.Is(readErr, fs.ErrNotExist) {
				readErr = fmt.Errorf("file does not exist")
			} else {
				readErr = fmt.Errorf("file is unavailable or unsafe")
			}
			icon.err = readErr
		} else {
			icon.mediaType, icon.width, icon.height, icon.err = inspectFarmCatalogIcon(data, extension)
		}
		c.iconCache[cacheKey] = icon
	}
	if icon.err != nil {
		c.warnFarm(index, "icon file %q rejected: %v", clean, icon.err)
		return
	}
	farm.IconFile = clean
	farm.IconMediaType = icon.mediaType
	farm.IconWidth = icon.width
	farm.IconHeight = icon.height
}

func inspectFarmCatalogIcon(data []byte, extension string) (string, int, int, error) {
	var mediaType string
	var width, height int
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}):
		if extension != ".png" {
			return "", 0, 0, fmt.Errorf("file header does not match extension")
		}
		config, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || format != "png" {
			return "", 0, 0, fmt.Errorf("invalid PNG header")
		}
		mediaType, width, height = "image/png", config.Width, config.Height
	case len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff:
		if extension != ".jpg" && extension != ".jpeg" {
			return "", 0, 0, fmt.Errorf("file header does not match extension")
		}
		config, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || format != "jpeg" {
			return "", 0, 0, fmt.Errorf("invalid JPEG header")
		}
		mediaType, width, height = "image/jpeg", config.Width, config.Height
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		if extension != ".webp" {
			return "", 0, 0, fmt.Errorf("file header does not match extension")
		}
		var err error
		width, height, err = farmCatalogWebPDimensions(data)
		if err != nil {
			return "", 0, 0, err
		}
		mediaType = "image/webp"
	default:
		return "", 0, 0, fmt.Errorf("unrecognized image header")
	}
	if width <= 0 || height <= 0 || width > farmCatalogMaxIconDimension || height > farmCatalogMaxIconDimension || int64(width)*int64(height) > farmCatalogMaxIconPixels {
		return "", 0, 0, fmt.Errorf("image dimensions %dx%d exceed limits", width, height)
	}
	return mediaType, width, height, nil
}

func farmCatalogWebPDimensions(data []byte) (int, int, error) {
	for offset := 12; offset+8 <= len(data); {
		chunkType := string(data[offset : offset+4])
		chunkLength := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		start := offset + 8
		end := start + chunkLength
		if chunkLength < 0 || end < start || end > len(data) {
			return 0, 0, fmt.Errorf("invalid WebP chunk length")
		}
		chunk := data[start:end]
		switch chunkType {
		case "VP8X":
			if len(chunk) < 10 {
				return 0, 0, fmt.Errorf("invalid WebP VP8X header")
			}
			return 1 + littleEndianUint24(chunk[4:7]), 1 + littleEndianUint24(chunk[7:10]), nil
		case "VP8L":
			if len(chunk) < 5 || chunk[0] != 0x2f {
				return 0, 0, fmt.Errorf("invalid WebP VP8L header")
			}
			width := 1 + int(chunk[1]) + (int(chunk[2]&0x3f) << 8)
			height := 1 + (int(chunk[2] >> 6)) + (int(chunk[3]) << 2) + (int(chunk[4]&0x0f) << 10)
			return width, height, nil
		case "VP8 ":
			if len(chunk) < 10 || !bytes.Equal(chunk[3:6], []byte{0x9d, 0x01, 0x2a}) {
				return 0, 0, fmt.Errorf("invalid WebP VP8 header")
			}
			width := int(binary.LittleEndian.Uint16(chunk[6:8]) & 0x3fff)
			height := int(binary.LittleEndian.Uint16(chunk[8:10]) & 0x3fff)
			return width, height, nil
		}
		offset = end + (chunkLength & 1)
	}
	return 0, 0, fmt.Errorf("WebP image dimensions were not found")
}

func littleEndianUint24(data []byte) int {
	return int(data[0]) | int(data[1])<<8 | int(data[2])<<16
}

func (c *farmCatalogScanContext) warnFarm(index int, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	c.farms[index].ParseWarnings = append(c.farms[index].ParseWarnings, message)
	c.warnings = append(c.warnings, fmt.Sprintf("farm %s: %s", c.farms[index].ID, message))
}
