package stardew_junimo

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

const (
	FarmDependencyReady           = "ready"
	FarmDependencyNeedsEnable     = "needs_enable"
	FarmDependencyMissingRequired = "missing_required"
	FarmDependencyConflict        = "conflict"
)

// NewGameModComponent is a safe, path-free description of one installed Mod
// participating in a future new-game selection.
type NewGameModComponent struct {
	Key        string `json:"key"`
	UniqueID   string `json:"uniqueId,omitempty"`
	FolderName string `json:"folderName"`
	Name       string `json:"name,omitempty"`
	Version    string `json:"version,omitempty"`
	PackageKey string `json:"packageKey,omitempty"`
	Enabled    bool   `json:"enabled"`
	Provider   bool   `json:"provider"`
}

// NewGameModSelection describes the transient Mod set needed before a modded
// farm can eventually be created. It is computed from installed manifests and
// is deliberately not persisted as a save profile because no save name exists.
type NewGameModSelection struct {
	FarmTypeID                 string                `json:"farmTypeId"`
	ProviderModKey             string                `json:"providerModKey,omitempty"`
	ProviderModID              string                `json:"providerModId,omitempty"`
	ProviderName               string                `json:"providerName,omitempty"`
	ProviderVersion            string                `json:"providerVersion,omitempty"`
	RequiredModKeys            []string              `json:"requiredModKeys"`
	OptionalDependencyKeys     []string              `json:"optionalDependencyKeys"`
	EnabledModKeys             []string              `json:"enabledModKeys"`
	DisabledRequiredModKeys    []string              `json:"disabledRequiredModKeys"`
	MissingRequiredModKeys     []string              `json:"missingRequiredModKeys"`
	ConflictingProviderModKeys []string              `json:"conflictingProviderModKeys"`
	Components                 []NewGameModComponent `json:"components"`
	ChangedModKeys             []string              `json:"changedModKeys,omitempty"`
	Warnings                   []string              `json:"warnings"`
	Readiness                  string                `json:"readiness"`
	DependenciesReady          bool                  `json:"dependenciesReady"`
}

type NewGameModSelectionError struct {
	Code    string
	Message string
}

func (e *NewGameModSelectionError) Error() string { return e.Message }

func IsNewGameModSelectionError(err error) (*NewGameModSelectionError, bool) {
	var target *NewGameModSelectionError
	return target, errors.As(err, &target)
}

// BuildFarmDependencySelections reuses the installed Mod relationship index
// used by enable/disable cascades. The returned slice matches catalog.Farms.
func BuildFarmDependencySelections(dataDir string, catalog FarmCatalogResult) ([]NewGameModSelection, error) {
	mods, err := listPhysicalMods(dataDir)
	if err != nil {
		return nil, err
	}
	mods = ApplyNexusMetadataToMods(dataDir, mods)
	idx := buildModRelationshipIndex(mods)
	result := make([]NewGameModSelection, 0, len(catalog.Farms))
	for _, farm := range catalog.Farms {
		result = append(result, buildFarmDependencySelection(farm, idx))
	}
	return result, nil
}

func buildFarmDependencySelection(farm FarmCatalogEntry, idx modRelationshipIndex) NewGameModSelection {
	selection := NewGameModSelection{
		FarmTypeID:                 farm.ID,
		ProviderModID:              farm.ProviderModID,
		ProviderName:               farm.ProviderName,
		ProviderVersion:            farm.ProviderVersion,
		RequiredModKeys:            []string{},
		OptionalDependencyKeys:     []string{},
		EnabledModKeys:             []string{},
		DisabledRequiredModKeys:    []string{},
		MissingRequiredModKeys:     []string{},
		ConflictingProviderModKeys: []string{},
		Components:                 []NewGameModComponent{},
		Warnings:                   []string{},
	}
	if farm.Conflict {
		selection.Readiness = FarmDependencyConflict
		for _, source := range farm.ConflictSources {
			selection.ConflictingProviderModKeys = appendUniqueFarmString(selection.ConflictingProviderModKeys, source.ProviderModID)
		}
		sort.Strings(selection.ConflictingProviderModKeys)
		return selection
	}

	seed, ok := idx.byUnique[normalizeModUniqueID(farm.ProviderModID)]
	if !ok {
		seed, ok = idx.byFolder[strings.ToLower(strings.TrimSpace(farm.ProviderFolder))]
	}
	if !ok {
		selection.MissingRequiredModKeys = []string{farm.ProviderModID}
		selection.Warnings = append(selection.Warnings, "provider Mod is no longer installed")
		selection.Readiness = FarmDependencyMissingRequired
		return selection
	}

	selection.ProviderModKey = stableModKey(idx.mods[seed])
	indexes := idx.enableClosure(seed)
	selected := make(map[int]bool, len(indexes))
	missing := map[string]string{}
	optional := map[string]string{}
	for _, i := range indexes {
		selected[i] = true
		mod := idx.mods[i]
		key := stableModKey(mod)
		component := NewGameModComponent{
			Key: key, UniqueID: mod.UniqueID, FolderName: mod.FolderName, Name: mod.Name,
			Version: mod.Version, PackageKey: mod.PackageKey, Enabled: mod.Enabled, Provider: i == seed,
		}
		selection.Components = append(selection.Components, component)
		if i != seed {
			selection.RequiredModKeys = append(selection.RequiredModKeys, key)
		}
		if mod.Enabled {
			selection.EnabledModKeys = append(selection.EnabledModKeys, key)
		} else {
			selection.DisabledRequiredModKeys = append(selection.DisabledRequiredModKeys, key)
		}
	}
	for _, i := range indexes {
		for _, dep := range idx.mods[i].Dependencies {
			depID := strings.TrimSpace(dep.UniqueID)
			if depID == "" {
				continue
			}
			normalized := normalizeModUniqueID(depID)
			if !dep.Required {
				optional[normalized] = depID
				continue
			}
			depIndex, installed := idx.byUnique[normalized]
			if !installed {
				missing[normalized] = depID
				continue
			}
			if !selected[depIndex] {
				// This should not occur because enableClosure follows required
				// edges, but preserve a diagnostic if the index ever diverges.
				selection.Warnings = append(selection.Warnings, "required dependency closure is incomplete: "+depID)
			}
		}
	}
	for _, value := range optional {
		selection.OptionalDependencyKeys = append(selection.OptionalDependencyKeys, value)
	}
	for _, value := range missing {
		selection.MissingRequiredModKeys = append(selection.MissingRequiredModKeys, value)
	}
	sort.Strings(selection.OptionalDependencyKeys)
	sort.Strings(selection.MissingRequiredModKeys)
	sort.Strings(selection.RequiredModKeys)
	sort.Strings(selection.EnabledModKeys)
	sort.Strings(selection.DisabledRequiredModKeys)
	if len(selection.MissingRequiredModKeys) > 0 {
		selection.Readiness = FarmDependencyMissingRequired
	} else if len(selection.DisabledRequiredModKeys) > 0 {
		selection.Readiness = FarmDependencyNeedsEnable
	} else {
		selection.Readiness = FarmDependencyReady
		selection.DependenciesReady = true
	}
	return selection
}

func stableModKey(mod registry.ModInfo) string {
	if uniqueID := strings.TrimSpace(mod.UniqueID); uniqueID != "" {
		return "unique:" + uniqueID
	}
	return "folder:" + mod.FolderName
}

func appendUniqueFarmString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return values
		}
	}
	return append(values, value)
}

type newGameModMover func(dataDir, folderName string, enabled bool) error

// ResolveNewGameModSelection resolves an explicit FarmType (or the legacy
// "modded" compatibility value) to one unambiguous installed provider and its
// required dependency closure. It never changes the filesystem.
func ResolveNewGameModSelection(dataDir, farmTypeID string) (NewGameModSelection, error) {
	farmType, err := NormalizeNewGameFarmType(farmTypeID)
	if err != nil || farmType.Builtin {
		return NewGameModSelection{}, &NewGameModSelectionError{Code: "farm_type_not_installed", Message: "未找到该模组农场"}
	}
	catalog, err := ScanFarmCatalog(dataDir)
	if err != nil {
		return NewGameModSelection{}, fmt.Errorf("scan farm catalog: %w", err)
	}
	selections, err := BuildFarmDependencySelections(dataDir, catalog)
	if err != nil {
		return NewGameModSelection{}, err
	}
	matches := make([]NewGameModSelection, 0, 2)
	for _, selection := range selections {
		if farmType.CompatibilityID || selection.FarmTypeID == farmType.ID {
			matches = append(matches, selection)
		}
	}
	if len(matches) == 0 {
		return NewGameModSelection{}, &NewGameModSelectionError{Code: "farm_type_not_installed", Message: "FarmType 未由已安装 Mod 声明"}
	}
	if len(matches) != 1 || matches[0].Readiness == FarmDependencyConflict {
		return NewGameModSelection{}, &NewGameModSelectionError{Code: "farm_type_conflict", Message: "FarmType 存在多个 provider，请使用无冲突的显式 Id"}
	}
	selection := matches[0]
	if len(selection.MissingRequiredModKeys) > 0 {
		return selection, &NewGameModSelectionError{Code: "farm_dependencies_missing", Message: "模组农场缺少必需依赖"}
	}
	return selection, nil
}

// ApplyNewGameModSelectionState makes the physical Mod directories match the
// exact creation set. Required/provider/package components are enabled and all
// unrelated toggleable Mods are disabled. The surrounding new-game transaction
// owns the durable snapshot and rollback.
func ApplyNewGameModSelectionState(dataDir string, selection NewGameModSelection) (NewGameModSelection, error) {
	lock := modProfileLockFor(dataDir)
	lock.Lock()
	defer lock.Unlock()

	allowed := make(map[string]bool, len(selection.Components))
	for _, component := range selection.Components {
		allowed[component.Key] = true
	}
	mods, err := listPhysicalMods(dataDir)
	if err != nil {
		return NewGameModSelection{}, err
	}
	for _, mod := range mods {
		if mod.BuiltIn || isSMAPIRuntimeMod(mod) || isControlModInfo(mod) || isJunimoServerModInfo(mod) {
			continue
		}
		desired := allowed[stableModKey(mod)]
		if desired != mod.Enabled {
			if err := moveModFolder(dataDir, mod.FolderName, desired); err != nil {
				return NewGameModSelection{}, fmt.Errorf("prepare creation Mod set %s: %w", mod.UniqueID, err)
			}
		}
	}

	updated, err := ResolveNewGameModSelection(dataDir, selection.FarmTypeID)
	if err != nil {
		return NewGameModSelection{}, err
	}
	updated.ChangedModKeys = []string{}
	for _, component := range updated.Components {
		if !component.Enabled {
			return NewGameModSelection{}, fmt.Errorf("required component was not enabled: %s", component.Key)
		}
		updated.ChangedModKeys = append(updated.ChangedModKeys, component.Key)
	}
	sort.Strings(updated.ChangedModKeys)
	return updated, nil
}

// PrepareNewGameMods enables the computed provider/dependency/package closure
// without creating a save profile or starting the server.
func PrepareNewGameMods(dataDir, farmTypeID string) (NewGameModSelection, error) {
	return prepareNewGameModsWithMover(dataDir, farmTypeID, moveModFolder)
}

func prepareNewGameModsWithMover(dataDir, farmTypeID string, mover newGameModMover) (NewGameModSelection, error) {
	farmTypeID = strings.TrimSpace(farmTypeID)
	if farmTypeID == "" {
		return NewGameModSelection{}, &NewGameModSelectionError{Code: "farm_type_required", Message: "farmTypeId is required"}
	}
	lock := modProfileLockFor(dataDir)
	lock.Lock()
	defer lock.Unlock()

	selection, err := ResolveNewGameModSelection(dataDir, farmTypeID)
	if err != nil {
		if selectionErr, ok := IsNewGameModSelectionError(err); ok {
			switch selectionErr.Code {
			case "farm_type_not_installed":
				selectionErr.Code = "farm_type_not_found"
			case "farm_dependencies_missing":
				selectionErr.Code = "missing_required_mods"
			}
		}
		return selection, err
	}

	byKey := make(map[string]NewGameModComponent, len(selection.Components))
	for _, component := range selection.Components {
		byKey[component.Key] = component
	}
	moved := make([]NewGameModComponent, 0, len(selection.DisabledRequiredModKeys))
	for _, key := range selection.DisabledRequiredModKeys {
		component, ok := byKey[key]
		if !ok {
			continue
		}
		if err := mover(dataDir, component.FolderName, true); err != nil {
			rollbackErr := rollbackPreparedMods(dataDir, moved, mover)
			if rollbackErr != nil {
				return NewGameModSelection{}, fmt.Errorf("enable %s: %w; rollback failed: %v", component.UniqueID, err, rollbackErr)
			}
			return NewGameModSelection{}, fmt.Errorf("enable %s: %w", component.UniqueID, err)
		}
		moved = append(moved, component)
	}

	updatedCatalog, err := ScanFarmCatalog(dataDir)
	if err != nil {
		return NewGameModSelection{}, rollbackPreparedFailure(dataDir, moved, mover, err)
	}
	updatedSelections, err := BuildFarmDependencySelections(dataDir, updatedCatalog)
	if err != nil {
		return NewGameModSelection{}, rollbackPreparedFailure(dataDir, moved, mover, err)
	}
	for _, updated := range updatedSelections {
		if updated.FarmTypeID == farmTypeID && updated.ProviderModID == selection.ProviderModID {
			for _, component := range moved {
				updated.ChangedModKeys = append(updated.ChangedModKeys, component.Key)
			}
			sort.Strings(updated.ChangedModKeys)
			return updated, nil
		}
	}
	return NewGameModSelection{}, rollbackPreparedFailure(dataDir, moved, mover, fmt.Errorf("prepared farm selection disappeared after update"))
}

func rollbackPreparedFailure(dataDir string, moved []NewGameModComponent, mover newGameModMover, cause error) error {
	if rollbackErr := rollbackPreparedMods(dataDir, moved, mover); rollbackErr != nil {
		return fmt.Errorf("%w; rollback failed: %v", cause, rollbackErr)
	}
	return cause
}

func rollbackPreparedMods(dataDir string, moved []NewGameModComponent, mover newGameModMover) error {
	var rollbackErrors []error
	for i := len(moved) - 1; i >= 0; i-- {
		if err := mover(dataDir, moved[i].FolderName, false); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	return errors.Join(rollbackErrors...)
}
