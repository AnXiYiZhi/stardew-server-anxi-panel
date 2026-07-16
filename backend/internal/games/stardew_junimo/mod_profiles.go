package stardew_junimo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

const modProfileStoreVersion = 1

type modProfileEntry struct {
	Enabled    bool   `json:"enabled"`
	FolderName string `json:"folderName,omitempty"`
	UniqueID   string `json:"uniqueId,omitempty"`
}

type modProfileSave struct {
	DefaultEnabled bool                       `json:"defaultEnabled"`
	UpdatedAt      string                     `json:"updatedAt,omitempty"`
	Mods           map[string]modProfileEntry `json:"mods"`
}

type modProfileStore struct {
	Version int                       `json:"version"`
	Saves   map[string]modProfileSave `json:"saves"`
}

var (
	modProfileLocksMu sync.Mutex
	modProfileLocks   = map[string]*sync.Mutex{}
)

func modProfileLockFor(dataDir string) *sync.Mutex {
	modProfileLocksMu.Lock()
	defer modProfileLocksMu.Unlock()
	lock, ok := modProfileLocks[dataDir]
	if !ok {
		lock = &sync.Mutex{}
		modProfileLocks[dataDir] = lock
	}
	return lock
}

func modProfileFilePath(dataDir string) string {
	return filepath.Join(controlDir(dataDir), "mod-profiles.json")
}

func loadModProfileStore(dataDir string) (modProfileStore, error) {
	data, err := os.ReadFile(modProfileFilePath(dataDir))
	if os.IsNotExist(err) {
		return modProfileStore{Version: modProfileStoreVersion, Saves: map[string]modProfileSave{}}, nil
	}
	if err != nil {
		return modProfileStore{}, fmt.Errorf("read mod profiles: %w", err)
	}
	var store modProfileStore
	if err := json.Unmarshal(data, &store); err != nil {
		return modProfileStore{}, fmt.Errorf("parse mod profiles: %w", err)
	}
	if store.Version == 0 {
		store.Version = modProfileStoreVersion
	}
	if store.Saves == nil {
		store.Saves = map[string]modProfileSave{}
	}
	for saveName, profile := range store.Saves {
		if profile.Mods == nil {
			profile.Mods = map[string]modProfileEntry{}
			store.Saves[saveName] = profile
		}
	}
	return store, nil
}

func saveModProfileStore(dataDir string, store modProfileStore) error {
	store.Version = modProfileStoreVersion
	if store.Saves == nil {
		store.Saves = map[string]modProfileSave{}
	}
	path := modProfileFilePath(dataDir)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create control dir: %w", err)
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mod profiles: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".mod-profiles-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp mod profiles: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp mod profiles: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp mod profiles: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename mod profiles: %w", err)
	}
	return nil
}

func modProfileKey(mod registry.ModInfo) string {
	if uniqueID := strings.TrimSpace(mod.UniqueID); uniqueID != "" {
		return "unique:" + uniqueID
	}
	return "folder:" + mod.FolderName
}

func modProfileEntryFor(profile modProfileSave, mod registry.ModInfo) (modProfileEntry, bool) {
	if profile.Mods == nil {
		return modProfileEntry{}, false
	}
	keys := []string{modProfileKey(mod), "folder:" + mod.FolderName}
	if strings.TrimSpace(mod.UniqueID) != "" {
		keys = append(keys, "unique:"+strings.TrimSpace(mod.UniqueID))
	}
	for _, key := range keys {
		if entry, ok := profile.Mods[key]; ok {
			return entry, true
		}
	}
	return modProfileEntry{}, false
}

// ApplyModEnableProfile overlays persisted per-save desired state onto a mod list.
func ApplyModEnableProfile(dataDir, saveName string, mods []registry.ModInfo) []registry.ModInfo {
	if strings.TrimSpace(saveName) == "" {
		for i := range mods {
			annotateModToggle(&mods[i])
		}
		return mods
	}
	store, err := loadModProfileStore(dataDir)
	profile, hasProfile := store.Saves[saveName]
	for i := range mods {
		annotateModToggle(&mods[i])
		if mods[i].BuiltIn || err != nil || !hasProfile {
			continue
		}
		if entry, ok := modProfileEntryFor(profile, mods[i]); ok {
			mods[i].Enabled = entry.Enabled
		} else {
			mods[i].Enabled = profile.DefaultEnabled
		}
	}
	return mods
}

func annotateModToggle(mod *registry.ModInfo) {
	if mod.BuiltIn || isSMAPIRuntimeMod(*mod) || isControlModInfo(*mod) || isJunimoServerModInfo(*mod) {
		mod.Enabled = true
		mod.CanToggle = false
		if mod.EnableNote == "" {
			mod.EnableNote = "Built-in component cannot be disabled"
		}
		return
	}
	mod.CanToggle = true
}

// ApplyNewSaveDefaultModState moves every non-built-in mod out of the active
// Mods directory before Junimo creates a fresh save whose name is not known yet.
func ApplyNewSaveDefaultModState(dataDir string) error {
	lock := modProfileLockFor(dataDir)
	lock.Lock()
	defer lock.Unlock()

	mods, err := listPhysicalMods(dataDir)
	if err != nil {
		return err
	}
	for _, mod := range mods {
		if !mod.Enabled || mod.BuiltIn || isSMAPIRuntimeMod(mod) || isControlModInfo(mod) || isJunimoServerModInfo(mod) {
			continue
		}
		if err := moveModFolder(dataDir, mod.FolderName, false); err != nil {
			return err
		}
	}
	return nil
}

// EnsureDisabledModProfileForSave records that a newly created/imported save
// should start with all non-built-in mods disabled unless explicitly enabled.
func EnsureDisabledModProfileForSave(dataDir, saveName string) error {
	saveName = strings.TrimSpace(saveName)
	if saveName == "" {
		return fmt.Errorf("save name is required")
	}
	lock := modProfileLockFor(dataDir)
	lock.Lock()
	defer lock.Unlock()

	store, err := loadModProfileStore(dataDir)
	if err != nil {
		return err
	}
	profile := modProfileSave{
		DefaultEnabled: false,
		UpdatedAt:      time.Now().Format(time.RFC3339),
		Mods:           map[string]modProfileEntry{},
	}
	mods, err := listPhysicalMods(dataDir)
	if err != nil {
		return err
	}
	for _, mod := range mods {
		if mod.BuiltIn || isSMAPIRuntimeMod(mod) || isControlModInfo(mod) || isJunimoServerModInfo(mod) {
			continue
		}
		profile.Mods[modProfileKey(mod)] = modProfileEntry{
			Enabled:    false,
			FolderName: mod.FolderName,
			UniqueID:   mod.UniqueID,
		}
	}
	store.Saves[saveName] = profile
	return saveModProfileStore(dataDir, store)
}

// EnsureNewSaveModProfile preserves the physical Mod state used while creating
// a modded farm and force-enables the provider dependency closure recorded by
// the transaction. Unrelated Mods therefore keep their pre-creation state;
// newly installed Mods still default to disabled until explicitly enabled for
// this save.
func EnsureNewSaveModProfile(dataDir, saveName string, enabledModKeys []string) error {
	saveName = strings.TrimSpace(saveName)
	if saveName == "" {
		return fmt.Errorf("save name is required")
	}
	lock := modProfileLockFor(dataDir)
	lock.Lock()
	defer lock.Unlock()

	enabled := make(map[string]bool, len(enabledModKeys))
	for _, key := range enabledModKeys {
		if key = strings.TrimSpace(key); key != "" {
			enabled[key] = true
		}
	}
	store, err := loadModProfileStore(dataDir)
	if err != nil {
		return err
	}
	mods, err := listPhysicalMods(dataDir)
	if err != nil {
		return err
	}
	// Farm preparation only enables required components; it never disables an
	// already active Mod. Merge the current physical state with the required
	// closure so committing the new save cannot silently turn unrelated Mods
	// off on its next start.
	for _, mod := range mods {
		if mod.Enabled {
			enabled[modProfileKey(mod)] = true
		}
	}
	profile := modProfileSave{
		DefaultEnabled: false,
		UpdatedAt:      time.Now().Format(time.RFC3339),
		Mods:           map[string]modProfileEntry{},
	}
	for _, mod := range mods {
		if mod.BuiltIn || isSMAPIRuntimeMod(mod) || isControlModInfo(mod) || isJunimoServerModInfo(mod) {
			continue
		}
		key := modProfileKey(mod)
		profile.Mods[key] = modProfileEntry{
			Enabled: enabled[key], FolderName: mod.FolderName, UniqueID: mod.UniqueID,
		}
	}
	store.Saves[saveName] = profile
	return saveModProfileStore(dataDir, store)
}

// EnsureImportedSaveModProfile initializes a profile from the imported save's
// actual XML FarmType. Official saves retain the historical all-third-party-
// disabled policy. A custom farm must resolve to one installed provider and a
// complete dependency closure; those required Mods are enabled in addition to
// the currently active Mod set.
func EnsureImportedSaveModProfile(dataDir, saveName string) error {
	if err := ValidateSaveExists(dataDir, saveName); err != nil {
		return err
	}
	rawFarmType := readWhichFarmFromMainFile(filepath.Join(savesDir(dataDir), "Saves", saveName), saveName)
	farmType, err := NormalizeNewGameFarmType(rawFarmType)
	if err != nil {
		return fmt.Errorf("read imported save FarmType: %w", err)
	}
	if farmType.Builtin {
		return EnsureDisabledModProfileForSave(dataDir, saveName)
	}
	selection, err := ResolveNewGameModSelection(dataDir, farmType.ID)
	if err != nil {
		return fmt.Errorf("resolve imported modded FarmType %q: %w", farmType.ID, err)
	}
	enabledKeys := make([]string, 0, len(selection.Components))
	for _, component := range selection.Components {
		enabledKeys = append(enabledKeys, component.Key)
	}
	return EnsureNewSaveModProfile(dataDir, saveName, enabledKeys)
}

// MarkImportedModsEnabledForSave records newly installed mods as enabled for
// the active save. Install actions are user intent to add the mod now; users can
// still disable it later from the per-save configuration page.
func MarkImportedModsEnabledForSave(dataDir, saveName string, imported []registry.ModInfo) error {
	saveName = strings.TrimSpace(saveName)
	if saveName == "" || len(imported) == 0 {
		return nil
	}
	if err := ValidateSaveExists(dataDir, saveName); err != nil {
		return err
	}

	lock := modProfileLockFor(dataDir)
	lock.Lock()
	defer lock.Unlock()

	store, err := loadModProfileStore(dataDir)
	if err != nil {
		return err
	}
	profile, ok := store.Saves[saveName]
	if !ok {
		profile = modProfileSave{DefaultEnabled: true, Mods: map[string]modProfileEntry{}}
	}
	if profile.Mods == nil {
		profile.Mods = map[string]modProfileEntry{}
	}
	profile.UpdatedAt = time.Now().Format(time.RFC3339)
	for _, mod := range imported {
		if mod.BuiltIn || isSMAPIRuntimeMod(mod) || isControlModInfo(mod) || isJunimoServerModInfo(mod) {
			continue
		}
		profile.Mods[modProfileKey(mod)] = modProfileEntry{
			Enabled:    true,
			FolderName: mod.FolderName,
			UniqueID:   mod.UniqueID,
		}
	}
	store.Saves[saveName] = profile
	if err := saveModProfileStore(dataDir, store); err != nil {
		return err
	}
	return applyModProfileLocked(dataDir, saveName)
}

// ApplyModProfile makes the physical active/disabled directories match the
// selected save profile. Existing saves without a profile keep their current
// physical mod state for backwards compatibility.
func ApplyModProfile(dataDir, saveName string) error {
	saveName = strings.TrimSpace(saveName)
	if saveName == "" {
		return fmt.Errorf("save name is required")
	}
	lock := modProfileLockFor(dataDir)
	lock.Lock()
	defer lock.Unlock()
	return applyModProfileLocked(dataDir, saveName)
}

func applyModProfileLocked(dataDir, saveName string) error {
	store, err := loadModProfileStore(dataDir)
	if err != nil {
		return err
	}
	profile, hasProfile := store.Saves[saveName]
	if !hasProfile {
		return nil
	}
	mods, err := listPhysicalMods(dataDir)
	if err != nil {
		return err
	}
	for _, mod := range mods {
		if mod.BuiltIn || isSMAPIRuntimeMod(mod) || isControlModInfo(mod) || isJunimoServerModInfo(mod) {
			if !mod.Enabled && (mod.FolderName == controlModFolderName || mod.FolderName == junimoServerModFolderName) {
				if err := moveModFolder(dataDir, mod.FolderName, true); err != nil {
					return err
				}
			}
			continue
		}
		desired := profile.DefaultEnabled
		if entry, ok := modProfileEntryFor(profile, mod); ok {
			desired = entry.Enabled
		}
		if desired != mod.Enabled {
			if err := moveModFolder(dataDir, mod.FolderName, desired); err != nil {
				return err
			}
		}
	}
	return nil
}

func SetModEnabledForSave(dataDir, saveName, modID string, enabled bool) (registry.ModInfo, error) {
	saveName = strings.TrimSpace(saveName)
	if saveName == "" {
		return registry.ModInfo{}, fmt.Errorf("save name is required")
	}
	if err := ValidateSaveExists(dataDir, saveName); err != nil {
		return registry.ModInfo{}, err
	}
	target, err := ResolveModInfo(dataDir, modID)
	if err != nil {
		return registry.ModInfo{}, err
	}
	if target.BuiltIn || isSMAPIRuntimeMod(target) || isControlModInfo(target) || isJunimoServerModInfo(target) {
		return registry.ModInfo{}, fmt.Errorf("built-in mod %q cannot be toggled", target.FolderName)
	}

	lock := modProfileLockFor(dataDir)
	lock.Lock()
	defer lock.Unlock()

	store, err := loadModProfileStore(dataDir)
	if err != nil {
		return registry.ModInfo{}, err
	}
	profile, ok := store.Saves[saveName]
	if !ok {
		profile = modProfileSave{DefaultEnabled: true, Mods: map[string]modProfileEntry{}}
	}
	if profile.Mods == nil {
		profile.Mods = map[string]modProfileEntry{}
	}
	profile.UpdatedAt = time.Now().Format(time.RFC3339)
	profile.Mods[modProfileKey(target)] = modProfileEntry{
		Enabled:    enabled,
		FolderName: target.FolderName,
		UniqueID:   target.UniqueID,
	}
	store.Saves[saveName] = profile
	if err := saveModProfileStore(dataDir, store); err != nil {
		return registry.ModInfo{}, err
	}
	if err := applyModProfileLocked(dataDir, saveName); err != nil {
		return registry.ModInfo{}, err
	}
	target.Enabled = enabled
	target.CanToggle = true
	return target, nil
}

// SetModEnabledForSaveCascade updates a save profile using Stardew mod
// relationships instead of a single folder. Enabling pulls in required
// dependencies and same-package members; disabling pushes to installed
// dependents and same-package members. Shared framework dependencies therefore
// get enabled when needed, but are not disabled merely because one dependent
// package was disabled.
func SetModEnabledForSaveCascade(dataDir, saveName, modID string, enabled bool) ([]registry.ModInfo, error) {
	saveName = strings.TrimSpace(saveName)
	if saveName == "" {
		return nil, fmt.Errorf("save name is required")
	}
	if err := ValidateSaveExists(dataDir, saveName); err != nil {
		return nil, err
	}
	if err := ValidateModName(modID); err != nil {
		return nil, err
	}

	lock := modProfileLockFor(dataDir)
	lock.Lock()
	defer lock.Unlock()

	mods, err := listPhysicalMods(dataDir)
	if err != nil {
		return nil, err
	}
	mods = ApplyNexusMetadataToMods(dataDir, mods)
	idx := buildModRelationshipIndex(mods)
	seed, err := idx.resolve(modID)
	if err != nil {
		return nil, err
	}
	if !modCanRelationshipToggle(mods[seed]) {
		return nil, fmt.Errorf("built-in mod %q cannot be toggled", mods[seed].FolderName)
	}
	affectedIndexes := idx.disableClosure(seed)
	if enabled {
		affectedIndexes = idx.enableClosure(seed)
	}

	store, err := loadModProfileStore(dataDir)
	if err != nil {
		return nil, err
	}
	profile, ok := store.Saves[saveName]
	if !ok {
		profile = modProfileSave{DefaultEnabled: true, Mods: map[string]modProfileEntry{}}
	}
	if profile.Mods == nil {
		profile.Mods = map[string]modProfileEntry{}
	}
	profile.UpdatedAt = time.Now().Format(time.RFC3339)
	affectedFolders := map[string]bool{}
	for _, i := range affectedIndexes {
		mod := mods[i]
		if !modCanRelationshipToggle(mod) {
			continue
		}
		profile.Mods[modProfileKey(mod)] = modProfileEntry{
			Enabled:    enabled,
			FolderName: mod.FolderName,
			UniqueID:   mod.UniqueID,
		}
		affectedFolders[mod.FolderName] = true
	}
	store.Saves[saveName] = profile
	if err := saveModProfileStore(dataDir, store); err != nil {
		return nil, err
	}
	if err := applyModProfileLocked(dataDir, saveName); err != nil {
		return nil, err
	}

	updated, err := ListModsWithState(dataDir, saveName)
	if err != nil {
		return nil, err
	}
	updated = ApplyNexusMetadataToMods(dataDir, updated)
	affected := make([]registry.ModInfo, 0, len(affectedFolders))
	for _, mod := range updated {
		if affectedFolders[mod.FolderName] {
			affected = append(affected, mod)
		}
	}
	return affected, nil
}

// SetAllModsEnabledForSave updates every user-toggleable Mod for one save in a
// single profile write and applies the resulting physical state once. Runtime,
// Control, and other built-in components are intentionally left enabled.
func SetAllModsEnabledForSave(dataDir, saveName string, enabled bool) ([]registry.ModInfo, error) {
	saveName = strings.TrimSpace(saveName)
	if saveName == "" {
		return nil, fmt.Errorf("save name is required")
	}
	if err := ValidateSaveExists(dataDir, saveName); err != nil {
		return nil, err
	}

	lock := modProfileLockFor(dataDir)
	lock.Lock()
	defer lock.Unlock()

	mods, err := listPhysicalMods(dataDir)
	if err != nil {
		return nil, err
	}
	mods = ApplyNexusMetadataToMods(dataDir, mods)
	store, err := loadModProfileStore(dataDir)
	if err != nil {
		return nil, err
	}
	profile, ok := store.Saves[saveName]
	if !ok {
		profile = modProfileSave{DefaultEnabled: true, Mods: map[string]modProfileEntry{}}
	}
	if profile.Mods == nil {
		profile.Mods = map[string]modProfileEntry{}
	}
	profile.UpdatedAt = time.Now().Format(time.RFC3339)
	affectedFolders := map[string]bool{}
	for _, mod := range mods {
		if !modCanRelationshipToggle(mod) || isJunimoServerModInfo(mod) {
			continue
		}
		profile.Mods[modProfileKey(mod)] = modProfileEntry{
			Enabled:    enabled,
			FolderName: mod.FolderName,
			UniqueID:   mod.UniqueID,
		}
		affectedFolders[mod.FolderName] = true
	}
	store.Saves[saveName] = profile
	if err := saveModProfileStore(dataDir, store); err != nil {
		return nil, err
	}
	if err := applyModProfileLocked(dataDir, saveName); err != nil {
		return nil, err
	}

	updated, err := ListModsWithState(dataDir, saveName)
	if err != nil {
		return nil, err
	}
	updated = ApplyNexusMetadataToMods(dataDir, updated)
	affected := make([]registry.ModInfo, 0, len(affectedFolders))
	for _, mod := range updated {
		if affectedFolders[mod.FolderName] {
			affected = append(affected, mod)
		}
	}
	return affected, nil
}

func ResolveModInfo(dataDir, modID string) (registry.ModInfo, error) {
	if err := ValidateModName(modID); err != nil {
		return registry.ModInfo{}, err
	}
	mods, err := listPhysicalMods(dataDir)
	if err != nil {
		return registry.ModInfo{}, err
	}
	for _, mod := range mods {
		if mod.FolderName == modID || mod.UniqueID == modID {
			return mod, nil
		}
	}
	return registry.ModInfo{}, fmt.Errorf("Mod %q does not exist", modID)
}

func moveModFolder(dataDir, folderName string, enabled bool) error {
	if err := ValidateModName(folderName); err != nil {
		return err
	}
	srcRoot := modsDir(dataDir)
	dstRoot := disabledModsDir(dataDir)
	if enabled {
		srcRoot, dstRoot = dstRoot, srcRoot
	}
	src := filepath.Join(srcRoot, folderName)
	dst := filepath.Join(dstRoot, folderName)
	info, err := os.Stat(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat mod %q: %w", folderName, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("mod %q is not a directory", folderName)
	}
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("target mod folder %q already exists", folderName)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat target mod %q: %w", folderName, err)
	}
	if err := os.MkdirAll(dstRoot, 0o755); err != nil {
		return fmt.Errorf("create mod target dir: %w", err)
	}
	if err := os.Rename(src, dst); err != nil {
		if err := copyDir(src, dst); err != nil {
			_ = os.RemoveAll(dst)
			return fmt.Errorf("copy mod %q: %w", folderName, err)
		}
		if err := os.RemoveAll(src); err != nil {
			return fmt.Errorf("remove moved mod %q: %w", folderName, err)
		}
	}
	return nil
}
