package stardew_junimo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

const nexusMetadataEnrichmentMaxIDs = 20

type nexusInstalledMetadata struct {
	ModID            int    `json:"modId"`
	Name             string `json:"name,omitempty"`
	Summary          string `json:"summary,omitempty"`
	Author           string `json:"author,omitempty"`
	Version          string `json:"version,omitempty"`
	UpdatedAt        string `json:"updatedAt,omitempty"`
	EndorsementCount int    `json:"endorsementCount,omitempty"`
	DownloadCount    int    `json:"downloadCount,omitempty"`
	PictureURL       string `json:"pictureUrl,omitempty"`
	NexusURL         string `json:"nexusUrl,omitempty"`
	PackageKey       string `json:"packageKey,omitempty"`
	PackageName      string `json:"packageName,omitempty"`
}

type nexusMetadataStore struct {
	Mods map[string]nexusInstalledMetadata `json:"mods"`
}

var (
	nexusMetadataLocksMu sync.Mutex
	nexusMetadataLocks   = map[string]*sync.Mutex{}
)

func nexusMetadataFilePath(dataDir string) string {
	return filepath.Join(dataDir, ".local-container", "control", "nexus-mods.json")
}

func nexusMetadataLockFor(dataDir string) *sync.Mutex {
	nexusMetadataLocksMu.Lock()
	defer nexusMetadataLocksMu.Unlock()
	lock, ok := nexusMetadataLocks[dataDir]
	if !ok {
		lock = &sync.Mutex{}
		nexusMetadataLocks[dataDir] = lock
	}
	return lock
}

func loadNexusMetadataStore(dataDir string) (nexusMetadataStore, error) {
	data, err := os.ReadFile(nexusMetadataFilePath(dataDir))
	if os.IsNotExist(err) {
		return nexusMetadataStore{Mods: map[string]nexusInstalledMetadata{}}, nil
	}
	if err != nil {
		return nexusMetadataStore{}, fmt.Errorf("read nexus metadata file: %w", err)
	}
	var store nexusMetadataStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nexusMetadataStore{}, fmt.Errorf("parse nexus metadata file: %w", err)
	}
	if store.Mods == nil {
		store.Mods = map[string]nexusInstalledMetadata{}
	}
	return store, nil
}

func saveNexusMetadataStore(dataDir string, store nexusMetadataStore) error {
	path := nexusMetadataFilePath(dataDir)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create control dir: %w", err)
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal nexus metadata file: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".nexus-mods-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp nexus metadata file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp nexus metadata file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp nexus metadata file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename nexus metadata file: %w", err)
	}
	return nil
}

func SaveInstalledNexusMetadata(dataDir string, imported []registry.ModInfo, result NexusModSearchResult) error {
	return saveInstalledNexusMetadata(dataDir, imported, result, true)
}

func saveInstalledNexusMetadata(dataDir string, imported []registry.ModInfo, result NexusModSearchResult, normalizeResult bool) error {
	if normalizeResult {
		result = normalizeInstalledNexusResult(imported, result)
	}
	if len(imported) == 0 {
		return nil
	}
	packageKey, packageName := installedPackageIdentity(imported, result.Name)
	if result.ModID <= 0 && packageKey == "" {
		return nil
	}
	lock := nexusMetadataLockFor(dataDir)
	lock.Lock()
	defer lock.Unlock()

	store, err := loadNexusMetadataStore(dataDir)
	if err != nil {
		return err
	}
	entry := nexusMetadataFromResult(result)
	entry.PackageKey = packageKey
	entry.PackageName = packageName
	if result.ModID > 0 {
		displayEntry := entry
		displayEntry.PackageKey = ""
		displayEntry.PackageName = ""
		for folderName, existing := range store.Mods {
			if existing.ModID == result.ModID {
				store.Mods[folderName] = mergeNexusMetadata(existing, displayEntry)
			}
		}
	}
	for _, mod := range imported {
		if mod.FolderName == "" {
			continue
		}
		existing := store.Mods[mod.FolderName]
		incoming := entry
		if mod.NexusModID > 0 && mod.NexusModID != result.ModID && len(imported) > 1 {
			// Preserve this component's own Nexus identity while still recording
			// that it belongs to the same physical install package.
			incoming = nexusInstalledMetadata{PackageKey: packageKey, PackageName: packageName}
		}
		if incoming.ModID > 0 && existing.ModID > 0 && existing.ModID != incoming.ModID {
			existing = nexusInstalledMetadata{}
		}
		store.Mods[mod.FolderName] = mergeNexusMetadata(existing, incoming)
	}
	return saveNexusMetadataStore(dataDir, store)
}

func normalizeInstalledNexusResult(imported []registry.ModInfo, result NexusModSearchResult) NexusModSearchResult {
	if len(imported) <= 1 || result.ModID <= 0 {
		return result
	}
	var candidate registry.ModInfo
	for _, mod := range imported {
		if mod.NexusModID <= 0 || mod.NexusModID == result.ModID {
			continue
		}
		if candidate.NexusModID > 0 && candidate.NexusModID != mod.NexusModID {
			return result
		}
		candidate = mod
	}
	if candidate.NexusModID <= 0 {
		return result
	}
	corrected := result
	corrected.ModID = candidate.NexusModID
	corrected.Name = strings.TrimSpace(candidate.Name)
	corrected.Author = strings.TrimSpace(candidate.Author)
	corrected.Version = strings.TrimSpace(candidate.Version)
	corrected.Summary = ""
	corrected.UpdatedAt = ""
	corrected.EndorsementCount = 0
	corrected.DownloadCount = 0
	corrected.PictureURL = ""
	corrected.NexusURL = nexusModURL(candidate.NexusModID)
	return corrected
}

func mergeNexusMetadata(existing, incoming nexusInstalledMetadata) nexusInstalledMetadata {
	if incoming.ModID > 0 {
		existing.ModID = incoming.ModID
	}
	if incoming.Name != "" {
		existing.Name = incoming.Name
	}
	if incoming.Summary != "" {
		existing.Summary = incoming.Summary
	}
	if incoming.Author != "" {
		existing.Author = incoming.Author
	}
	if incoming.Version != "" {
		existing.Version = incoming.Version
	}
	if incoming.UpdatedAt != "" {
		existing.UpdatedAt = incoming.UpdatedAt
	}
	if incoming.EndorsementCount > 0 {
		existing.EndorsementCount = incoming.EndorsementCount
	}
	if incoming.DownloadCount > 0 {
		existing.DownloadCount = incoming.DownloadCount
	}
	if incoming.PictureURL != "" {
		existing.PictureURL = incoming.PictureURL
	}
	if incoming.NexusURL != "" {
		existing.NexusURL = incoming.NexusURL
	}
	if incoming.PackageKey != "" {
		existing.PackageKey = incoming.PackageKey
	}
	if incoming.PackageName != "" {
		existing.PackageName = incoming.PackageName
	}
	return existing
}

func installedPackageIdentity(imported []registry.ModInfo, fallbackName string) (string, string) {
	if len(imported) < 2 {
		return "", ""
	}
	key := strings.TrimSpace(imported[0].PackageKey)
	name := strings.TrimSpace(imported[0].PackageName)
	for _, mod := range imported[1:] {
		if key == "" || strings.TrimSpace(mod.PackageKey) != key {
			key = ""
			break
		}
	}
	if key == "" {
		key = modPackageKey(imported)
	}
	if name == "" {
		name = strings.TrimSpace(fallbackName)
	}
	return key, name
}

func SaveInferredNexusPackageOrigin(dataDir string, imported []registry.ModInfo) error {
	origin, ok := chooseNexusPackageOrigin(imported)
	result := NexusModSearchResult{}
	if ok {
		result = NexusModSearchResult{
			ModID:    origin.NexusModID,
			Name:     origin.Name,
			Author:   origin.Author,
			Version:  origin.Version,
			NexusURL: nexusModURL(origin.NexusModID),
		}
	}
	// This result was derived from the package itself, so don't run the
	// correction intended for a possibly mismatched external batch result.
	return saveInstalledNexusMetadata(dataDir, imported, result, false)
}

func SaveInferredNexusPackageOrigins(dataDir string, groups map[string][]registry.ModInfo) error {
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := SaveInferredNexusPackageOrigin(dataDir, groups[key]); err != nil {
			return err
		}
	}
	return nil
}

func chooseNexusPackageOrigin(imported []registry.ModInfo) (registry.ModInfo, bool) {
	var fallback registry.ModInfo
	for _, mod := range imported {
		if mod.NexusModID <= 0 {
			continue
		}
		if fallback.NexusModID == 0 {
			fallback = mod
		}
		if !strings.HasPrefix(strings.TrimSpace(mod.Name), "[") {
			return mod, true
		}
	}
	if fallback.NexusModID > 0 {
		return fallback, true
	}
	return registry.ModInfo{}, false
}

func DeleteInstalledNexusMetadata(dataDir string, folderNames []string) error {
	if len(folderNames) == 0 {
		return nil
	}
	lock := nexusMetadataLockFor(dataDir)
	lock.Lock()
	defer lock.Unlock()

	store, err := loadNexusMetadataStore(dataDir)
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
	return saveNexusMetadataStore(dataDir, store)
}

func ApplyNexusMetadataToMods(dataDir string, mods []registry.ModInfo) []registry.ModInfo {
	store, err := loadNexusMetadataStore(dataDir)
	if err != nil || len(store.Mods) == 0 {
		return mods
	}
	byModID := map[int]nexusInstalledMetadata{}
	for _, entry := range store.Mods {
		if entry.ModID > 0 {
			byModID[entry.ModID] = mergeNexusMetadata(byModID[entry.ModID], entry)
		}
	}
	for i := range mods {
		entry, ok := store.Mods[mods[i].FolderName]
		if ok && entry.ModID > 0 {
			if richerEntry, hasRicherEntry := byModID[entry.ModID]; hasRicherEntry {
				entry = mergeNexusMetadata(entry, richerEntry)
			}
		}
		if !ok {
			if mods[i].NexusModID > 0 {
				entry, ok = byModID[mods[i].NexusModID]
			}
		}
		if !ok {
			continue
		}
		applyNexusMetadata(&mods[i], entry)
	}
	return mods
}

// EnrichNexusMetadataForMods fills missing Nexus card metadata for installed
// mods by reading SMAPI UpdateKeys ("Nexus:<id>") and querying the public
// GraphQL v2 metadata endpoint. Successful lookups are cached in the same
// sidecar used by panel-driven installs, so later list calls don't need
// another network request.
func EnrichNexusMetadataForMods(ctx context.Context, dataDir string, mods []registry.ModInfo) []registry.ModInfo {
	mods = ApplyNexusMetadataToMods(dataDir, mods)

	store, err := loadNexusMetadataStore(dataDir)
	if err != nil {
		return mods
	}
	modsByNexusID := map[int][]registry.ModInfo{}
	displayMetadataByModID := nexusDisplayMetadataByModID(store)
	for _, mod := range mods {
		if mod.NexusModID <= 0 || hasNexusMetadataForMod(store, displayMetadataByModID, mod) {
			continue
		}
		modsByNexusID[mod.NexusModID] = append(modsByNexusID[mod.NexusModID], mod)
	}
	if len(modsByNexusID) == 0 {
		return mods
	}

	ids := make([]int, 0, len(modsByNexusID))
	for id := range modsByNexusID {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if len(ids) > nexusMetadataEnrichmentMaxIDs {
		ids = ids[:nexusMetadataEnrichmentMaxIDs]
	}

	results, err := nexusGetModsByIDGraphQL(ctx, ids)
	if err != nil || len(results) == 0 {
		return mods
	}
	for _, result := range results {
		matchingMods := modsByNexusID[result.ModID]
		if len(matchingMods) == 0 {
			continue
		}
		if err := SaveInstalledNexusMetadata(dataDir, matchingMods, result); err != nil {
			continue
		}
	}
	return ApplyNexusMetadataToMods(dataDir, mods)
}

func nexusDisplayMetadataByModID(store nexusMetadataStore) map[int]bool {
	byModID := map[int]bool{}
	for _, entry := range store.Mods {
		if entry.ModID > 0 && entryHasDisplayMetadata(entry) {
			byModID[entry.ModID] = true
		}
	}
	return byModID
}

func hasNexusMetadataForMod(store nexusMetadataStore, displayMetadataByModID map[int]bool, mod registry.ModInfo) bool {
	if entry, ok := store.Mods[mod.FolderName]; ok && entryHasDisplayMetadata(entry) {
		return true
	}
	if mod.NexusModID <= 0 {
		return false
	}
	return displayMetadataByModID[mod.NexusModID]
}

func entryHasDisplayMetadata(entry nexusInstalledMetadata) bool {
	return entry.Summary != "" ||
		entry.UpdatedAt != "" ||
		entry.EndorsementCount > 0 ||
		entry.DownloadCount > 0 ||
		entry.PictureURL != ""
}

func nexusMetadataFromResult(result NexusModSearchResult) nexusInstalledMetadata {
	return nexusInstalledMetadata{
		ModID:            result.ModID,
		Name:             result.Name,
		Summary:          result.Summary,
		Author:           result.Author,
		Version:          result.Version,
		UpdatedAt:        result.UpdatedAt,
		EndorsementCount: result.EndorsementCount,
		DownloadCount:    result.DownloadCount,
		PictureURL:       result.PictureURL,
		NexusURL:         result.NexusURL,
	}
}

func applyNexusMetadata(mod *registry.ModInfo, entry nexusInstalledMetadata) {
	mod.PackageKey = entry.PackageKey
	mod.PackageName = entry.PackageName
	ownNexusMetadata := mod.NexusModID > 0 && mod.NexusModID == entry.ModID
	if ownNexusMetadata {
		if mod.NexusURL == "" {
			mod.NexusURL = entry.NexusURL
		}
		if mod.NexusURL == "" {
			mod.NexusURL = nexusModURL(mod.NexusModID)
		}
	} else if mod.NexusModID == 0 && entry.ModID > 0 {
		mod.OriginSource = "nexus"
		mod.OriginNexusModID = entry.ModID
		mod.OriginModName = entry.Name
		mod.OriginModURL = entry.NexusURL
		if mod.OriginModURL == "" {
			mod.OriginModURL = nexusModURL(entry.ModID)
		}
	}
	mod.NexusSummary = entry.Summary
	mod.UpdatedAt = entry.UpdatedAt
	mod.EndorsementCount = entry.EndorsementCount
	mod.DownloadCount = entry.DownloadCount
	mod.PictureURL = entry.PictureURL
}
