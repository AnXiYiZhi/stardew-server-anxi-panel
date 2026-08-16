package stardew_junimo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/netdns"
)

const (
	modUpdateEndpoint           = "https://smapi.io/api/v4.0.0/mods"
	modUpdateCacheTTL           = 6 * time.Hour
	modUpdateRequestTimeout     = 15 * time.Second
	modUpdateResponseMaxBytes   = 2 * 1024 * 1024
	modUpdateBatchSize          = 50
	modUpdateFallbackAPIVersion = "4.0.0"
	modUpdatePlatform           = "Linux"
)

var (
	modUpdateHTTPClient = netdns.NewClient(modUpdateRequestTimeout)
	modUpdateServiceURL = modUpdateEndpoint
	modUpdateNow        = time.Now

	modUpdateLocksMu sync.Mutex
	modUpdateLocks   = map[string]*sync.Mutex{}
)

type modUpdateCache struct {
	Fingerprint string                        `json:"fingerprint"`
	Result      registry.ModUpdateCheckResult `json:"result"`
}

type modUpdateCandidate struct {
	Mod registry.ModInfo
}

type smapiModUpdateRequest struct {
	Mods        []smapiModUpdateRequestItem `json:"mods"`
	APIVersion  string                      `json:"apiVersion"`
	GameVersion string                      `json:"gameVersion,omitempty"`
	Platform    string                      `json:"platform"`
}

type smapiModUpdateRequestItem struct {
	ID               string   `json:"id"`
	UpdateKeys       []string `json:"updateKeys"`
	InstalledVersion string   `json:"installedVersion"`
	IsBroken         bool     `json:"isBroken"`
}

type smapiModUpdateResponseItem struct {
	ID              string                   `json:"id"`
	SuggestedUpdate *smapiSuggestedModUpdate `json:"suggestedUpdate"`
	Errors          []string                 `json:"errors"`
}

type smapiSuggestedModUpdate struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

func modUpdateCachePath(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "control", "mod-updates.json")
}

func modUpdateLockFor(dataDir string) *sync.Mutex {
	modUpdateLocksMu.Lock()
	defer modUpdateLocksMu.Unlock()
	lock, ok := modUpdateLocks[dataDir]
	if !ok {
		lock = &sync.Mutex{}
		modUpdateLocks[dataDir] = lock
	}
	return lock
}

// CheckModUpdates asks SMAPI's update service for suggestions for installed
// physical mods. Results are cached per instance and invalidated whenever the
// local manifest inventory changes. The service's suggestedUpdate is the
// authority; the panel intentionally does not implement its own version
// comparator or replace local files automatically.
func (d *Driver) CheckModUpdates(ctx context.Context, instance registry.Instance, force bool) (registry.ModUpdateCheckResult, error) {
	lock := modUpdateLockFor(instance.DataDir)
	lock.Lock()
	defer lock.Unlock()

	activeSaveName := GetActiveSaveName(instance.DataDir)
	mods, err := ListModsWithState(instance.DataDir, activeSaveName)
	if err != nil {
		return registry.ModUpdateCheckResult{}, fmt.Errorf("list mods for update check: %w", err)
	}
	apiVersion, gameVersion := modUpdateRuntimeVersions(instance.DataDir)
	candidates, fingerprint, skipped := buildModUpdateCandidates(mods, apiVersion, gameVersion)
	cache, err := loadModUpdateCache(instance.DataDir)
	if err != nil {
		return registry.ModUpdateCheckResult{}, err
	}

	if !force && cache != nil && cache.Fingerprint == fingerprint && modUpdateCacheFresh(cache.Result.CheckedAt, modUpdateNow()) {
		result := cache.Result
		result.Cached = true
		return normalizeModUpdateResult(result), nil
	}

	if len(candidates) == 0 {
		result := registry.ModUpdateCheckResult{
			Status:        "ok",
			CheckedAt:     modUpdateNow().UTC().Format(time.RFC3339),
			Updates:       []registry.ModUpdateInfo{},
			EligibleCount: 0,
			SkippedCount:  skipped,
		}
		if err := saveModUpdateCache(instance.DataDir, modUpdateCache{Fingerprint: fingerprint, Result: result}); err != nil {
			return registry.ModUpdateCheckResult{}, err
		}
		return result, nil
	}

	updates, err := d.fetchModUpdates(ctx, candidates, apiVersion, gameVersion)
	if err != nil {
		if d != nil && d.logger != nil {
			d.logger.Warn("mod update check failed", "instance_id", instance.ID, "error", err)
		}
		return failedModUpdateResult(cache, len(candidates), skipped), nil
	}

	result := registry.ModUpdateCheckResult{
		Status:        "ok",
		CheckedAt:     modUpdateNow().UTC().Format(time.RFC3339),
		Updates:       updates,
		EligibleCount: len(candidates),
		SkippedCount:  skipped,
	}
	if err := saveModUpdateCache(instance.DataDir, modUpdateCache{Fingerprint: fingerprint, Result: result}); err != nil {
		return registry.ModUpdateCheckResult{}, err
	}
	return result, nil
}

func buildModUpdateCandidates(mods []registry.ModInfo, apiVersion, gameVersion string) ([]modUpdateCandidate, string, int) {
	type fingerprintItem struct {
		UniqueID   string   `json:"uniqueId"`
		Version    string   `json:"version"`
		FolderName string   `json:"folderName"`
		ParseError string   `json:"parseError,omitempty"`
		UpdateKeys []string `json:"updateKeys,omitempty"`
	}

	physical := make([]registry.ModInfo, 0, len(mods))
	for _, mod := range mods {
		if !mod.BuiltIn {
			physical = append(physical, mod)
		}
	}
	sort.Slice(physical, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(physical[i].UniqueID)) + "\x00" + strings.ToLower(physical[i].FolderName)
		right := strings.ToLower(strings.TrimSpace(physical[j].UniqueID)) + "\x00" + strings.ToLower(physical[j].FolderName)
		return left < right
	})

	fingerprintItems := make([]fingerprintItem, 0, len(physical))
	candidates := make([]modUpdateCandidate, 0, len(physical))
	seen := make(map[string]struct{}, len(physical))
	for _, mod := range physical {
		keys := append([]string(nil), mod.UpdateKeys...)
		sort.Strings(keys)
		fingerprintItems = append(fingerprintItems, fingerprintItem{
			UniqueID:   strings.TrimSpace(mod.UniqueID),
			Version:    strings.TrimSpace(mod.Version),
			FolderName: mod.FolderName,
			ParseError: mod.ParseError,
			UpdateKeys: keys,
		})

		uniqueID := strings.TrimSpace(mod.UniqueID)
		version := strings.TrimSpace(mod.Version)
		if mod.ParseError != "" || uniqueID == "" || version == "" || len(keys) == 0 {
			continue
		}
		identity := strings.ToLower(uniqueID)
		if _, exists := seen[identity]; exists {
			continue
		}
		seen[identity] = struct{}{}
		mod.UpdateKeys = keys
		candidates = append(candidates, modUpdateCandidate{Mod: mod})
	}

	encoded, _ := json.Marshal(struct {
		APIVersion  string            `json:"apiVersion"`
		GameVersion string            `json:"gameVersion"`
		Platform    string            `json:"platform"`
		Mods        []fingerprintItem `json:"mods"`
	}{
		APIVersion: apiVersion, GameVersion: gameVersion, Platform: modUpdatePlatform, Mods: fingerprintItems,
	})
	sum := sha256.Sum256(encoded)
	return candidates, hex.EncodeToString(sum[:]), len(physical) - len(candidates)
}

func (d *Driver) fetchModUpdates(ctx context.Context, candidates []modUpdateCandidate, apiVersion, gameVersion string) ([]registry.ModUpdateInfo, error) {
	updates := make([]registry.ModUpdateInfo, 0)
	for start := 0; start < len(candidates); start += modUpdateBatchSize {
		end := start + modUpdateBatchSize
		if end > len(candidates) {
			end = len(candidates)
		}
		batchUpdates, err := d.fetchModUpdateBatch(ctx, candidates[start:end], apiVersion, gameVersion)
		if err != nil {
			return nil, err
		}
		updates = append(updates, batchUpdates...)
	}
	sort.Slice(updates, func(i, j int) bool {
		return strings.ToLower(updates[i].Name) < strings.ToLower(updates[j].Name)
	})
	return updates, nil
}

func (d *Driver) fetchModUpdateBatch(ctx context.Context, candidates []modUpdateCandidate, apiVersion, gameVersion string) ([]registry.ModUpdateInfo, error) {
	requestItems := make([]smapiModUpdateRequestItem, 0, len(candidates))
	byUniqueID := make(map[string]registry.ModInfo, len(candidates))
	for _, candidate := range candidates {
		mod := candidate.Mod
		requestItems = append(requestItems, smapiModUpdateRequestItem{
			ID:               mod.UniqueID,
			UpdateKeys:       mod.UpdateKeys,
			InstalledVersion: mod.Version,
			IsBroken:         false,
		})
		byUniqueID[strings.ToLower(strings.TrimSpace(mod.UniqueID))] = mod
	}
	payload, err := json.Marshal(smapiModUpdateRequest{
		Mods: requestItems, APIVersion: apiVersion, GameVersion: gameVersion, Platform: modUpdatePlatform,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal SMAPI update request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, modUpdateServiceURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create SMAPI update request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	userAgent := "stardew-server-anxi-panel"
	if d != nil && strings.TrimSpace(d.panelVersion) != "" {
		userAgent += "/" + strings.TrimSpace(d.panelVersion)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := modUpdateHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request SMAPI update service: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 32*1024))
		return nil, fmt.Errorf("SMAPI update service returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, modUpdateResponseMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read SMAPI update response: %w", err)
	}
	if len(body) > modUpdateResponseMaxBytes {
		return nil, fmt.Errorf("SMAPI update response exceeds %d bytes", modUpdateResponseMaxBytes)
	}
	var responseItems []smapiModUpdateResponseItem
	if err := json.Unmarshal(body, &responseItems); err != nil {
		return nil, fmt.Errorf("parse SMAPI update response: %w", err)
	}

	updates := make([]registry.ModUpdateInfo, 0, len(responseItems))
	for _, item := range responseItems {
		mod, ok := byUniqueID[strings.ToLower(strings.TrimSpace(item.ID))]
		if !ok || item.SuggestedUpdate == nil {
			continue
		}
		latestVersion := strings.TrimSpace(item.SuggestedUpdate.Version)
		updateURL := strings.TrimSpace(item.SuggestedUpdate.URL)
		if latestVersion == "" || !safeModUpdateURL(updateURL) {
			continue
		}
		updates = append(updates, registry.ModUpdateInfo{
			ID:             mod.ID,
			UniqueID:       mod.UniqueID,
			Name:           firstNonEmpty(mod.Name, mod.FolderName, mod.UniqueID),
			FolderName:     mod.FolderName,
			CurrentVersion: mod.Version,
			LatestVersion:  latestVersion,
			URL:            updateURL,
		})
	}
	return updates, nil
}

func modUpdateRuntimeVersions(dataDir string) (string, string) {
	apiVersion := modUpdateFallbackAPIVersion
	raw, err := os.ReadFile(filepath.Join(controlDir(dataDir), "options.json"))
	if err != nil {
		return apiVersion, ""
	}
	var options struct {
		APIVersion  string `json:"apiVersion"`
		GameVersion string `json:"gameVersion"`
	}
	if json.Unmarshal(raw, &options) != nil {
		return apiVersion, ""
	}
	if actual := strings.TrimSpace(options.APIVersion); actual != "" {
		apiVersion = actual
	}
	return apiVersion, strings.TrimSpace(options.GameVersion)
}

func safeModUpdateURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return false
	}
	return parsed.Scheme == "https" || parsed.Scheme == "http"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return "Mod"
}

func failedModUpdateResult(cache *modUpdateCache, eligible, skipped int) registry.ModUpdateCheckResult {
	result := registry.ModUpdateCheckResult{
		Status:        "error",
		Updates:       []registry.ModUpdateInfo{},
		EligibleCount: eligible,
		SkippedCount:  skipped,
		CheckError:    "暂时无法连接 Mod 更新服务，请稍后重试。",
	}
	if cache != nil {
		result.CheckedAt = cache.Result.CheckedAt
		result.Updates = cache.Result.Updates
		result.Cached = true
	}
	return normalizeModUpdateResult(result)
}

func modUpdateCacheFresh(checkedAt string, now time.Time) bool {
	checked, err := time.Parse(time.RFC3339, checkedAt)
	if err != nil {
		return false
	}
	age := now.UTC().Sub(checked.UTC())
	return age >= 0 && age < modUpdateCacheTTL
}

func normalizeModUpdateResult(result registry.ModUpdateCheckResult) registry.ModUpdateCheckResult {
	if result.Updates == nil {
		result.Updates = []registry.ModUpdateInfo{}
	}
	return result
}

func loadModUpdateCache(dataDir string) (*modUpdateCache, error) {
	data, err := os.ReadFile(modUpdateCachePath(dataDir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read mod update cache: %w", err)
	}
	var cache modUpdateCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("parse mod update cache: %w", err)
	}
	cache.Result = normalizeModUpdateResult(cache.Result)
	return &cache, nil
}

func saveModUpdateCache(dataDir string, cache modUpdateCache) error {
	cache.Result.Cached = false
	cache.Result.CheckError = ""
	cache.Result = normalizeModUpdateResult(cache.Result)
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mod update cache: %w", err)
	}
	if err := atomicWriteRaw(modUpdateCachePath(dataDir), data, 0o644); err != nil {
		return fmt.Errorf("write mod update cache: %w", err)
	}
	return nil
}
