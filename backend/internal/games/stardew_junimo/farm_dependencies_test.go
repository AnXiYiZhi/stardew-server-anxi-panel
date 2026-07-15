package stardew_junimo

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestFarmCatalogDependenciesFrontierClosureAndContentPackDedup(t *testing.T) {
	dir := t.TempDir()
	writeFrontierDependencyFixture(t, dir, frontierDependencyFixtureOptions{})

	catalog, err := ScanFarmCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	selections, err := BuildFarmDependencySelections(dir, catalog)
	if err != nil {
		t.Fatal(err)
	}
	selection := findDependencySelection(t, selections, "FrontierFarm")
	want := []string{
		"unique:FlashShifter.SVE-FTM",
		"unique:FlashShifter.SVECode",
		"unique:FlashShifter.StardewValleyExpandedCP",
		"unique:Pathoschild.ContentPatcher",
	}
	if !slices.Equal(selection.RequiredModKeys, want) {
		t.Fatalf("required closure = %v, want %v", selection.RequiredModKeys, want)
	}
	if selection.ProviderModKey != "unique:flashshifter.FrontierFarm" || selection.Readiness != FarmDependencyReady || !selection.DependenciesReady {
		t.Fatalf("selection = %+v", selection)
	}
	if countString(selection.RequiredModKeys, "unique:Pathoschild.ContentPatcher") != 1 {
		t.Fatalf("Content Patcher was not deduplicated: %v", selection.RequiredModKeys)
	}
	if !slices.Equal(selection.OptionalDependencyKeys, []string{"Author.OptionalDecoration"}) {
		t.Fatalf("optional dependencies = %v", selection.OptionalDependencyKeys)
	}
}

func TestManifestDependenciesForcesContentPackForRequiredAndDeduplicatesCase(t *testing.T) {
	optional := false
	manifest := modManifest{
		ContentPackFor: &modManifestDependency{UniqueID: "Pathoschild.ContentPatcher", IsRequired: &optional},
		Dependencies: []modManifestDependency{
			{UniqueID: "pathoschild.contentpatcher", IsRequired: &optional, MinimumVersion: "2.0.0"},
		},
	}
	dependencies := manifestDependencies(manifest)
	if len(dependencies) != 1 || !dependencies[0].Required || dependencies[0].MinimumVersion != "2.0.0" {
		t.Fatalf("dependencies = %+v", dependencies)
	}
}

func TestFarmCatalogDependenciesCycleTerminates(t *testing.T) {
	dir := t.TempDir()
	writeFrontierDependencyFixture(t, dir, frontierDependencyFixtureOptions{cycle: true})
	catalog, _ := ScanFarmCatalog(dir)
	selections, err := BuildFarmDependencySelections(dir, catalog)
	if err != nil {
		t.Fatal(err)
	}
	selection := findDependencySelection(t, selections, "FrontierFarm")
	if len(selection.Components) != 5 || selection.Readiness != FarmDependencyReady {
		t.Fatalf("cycle selection = %+v", selection)
	}
}

func TestFarmCatalogDependenciesMissingRequiredAndOptionalDoesNotBlock(t *testing.T) {
	dir := t.TempDir()
	writeFrontierDependencyFixture(t, dir, frontierDependencyFixtureOptions{missingSVECode: true})
	catalog, _ := ScanFarmCatalog(dir)
	selections, err := BuildFarmDependencySelections(dir, catalog)
	if err != nil {
		t.Fatal(err)
	}
	selection := findDependencySelection(t, selections, "FrontierFarm")
	if selection.Readiness != FarmDependencyMissingRequired || selection.DependenciesReady {
		t.Fatalf("missing selection = %+v", selection)
	}
	if !slices.Equal(selection.MissingRequiredModKeys, []string{"FlashShifter.SVECode"}) {
		t.Fatalf("missing required = %v", selection.MissingRequiredModKeys)
	}
	if !slices.Equal(selection.OptionalDependencyKeys, []string{"Author.OptionalDecoration"}) {
		t.Fatalf("optional dependency was lost: %v", selection.OptionalDependencyKeys)
	}
}

func TestFarmCatalogDependenciesDisabledProviderAndContentPatcher(t *testing.T) {
	dir := t.TempDir()
	writeFrontierDependencyFixture(t, dir, frontierDependencyFixtureOptions{providerDisabled: true, contentPatcherDisabled: true})
	catalog, _ := ScanFarmCatalog(dir)
	selections, err := BuildFarmDependencySelections(dir, catalog)
	if err != nil {
		t.Fatal(err)
	}
	selection := findDependencySelection(t, selections, "FrontierFarm")
	if selection.Readiness != FarmDependencyNeedsEnable || selection.DependenciesReady {
		t.Fatalf("disabled selection = %+v", selection)
	}
	want := []string{"unique:Pathoschild.ContentPatcher", "unique:flashshifter.FrontierFarm"}
	if !slices.Equal(selection.DisabledRequiredModKeys, want) {
		t.Fatalf("disabled required = %v, want %v", selection.DisabledRequiredModKeys, want)
	}
}

func TestPrepareNewGameModsEnablesClosureWithoutSaveProfile(t *testing.T) {
	dir := t.TempDir()
	writeFrontierDependencyFixture(t, dir, frontierDependencyFixtureOptions{providerDisabled: true, contentPatcherDisabled: true})
	selection, err := PrepareNewGameMods(dir, "FrontierFarm")
	if err != nil {
		t.Fatal(err)
	}
	if selection.Readiness != FarmDependencyReady || !selection.DependenciesReady {
		t.Fatalf("prepared selection = %+v", selection)
	}
	wantChanged := []string{"unique:Pathoschild.ContentPatcher", "unique:flashshifter.FrontierFarm"}
	if !slices.Equal(selection.ChangedModKeys, wantChanged) {
		t.Fatalf("changed = %v, want %v", selection.ChangedModKeys, wantChanged)
	}
	for _, folder := range []string{"[CP] Frontier Farm", "Content Patcher"} {
		if _, err := os.Stat(filepath.Join(modsDir(dir), folder)); err != nil {
			t.Fatalf("%s not enabled: %v", folder, err)
		}
	}
	if _, err := os.Stat(modProfileFilePath(dir)); !os.IsNotExist(err) {
		t.Fatalf("prepare must not create save profile, err=%v", err)
	}
}

func TestApplyNewGameModSelectionStateUsesExactDependencyClosure(t *testing.T) {
	dir := t.TempDir()
	writeFrontierDependencyFixture(t, dir, frontierDependencyFixtureOptions{providerDisabled: true, contentPatcherDisabled: true})
	createTestMod(t, modsDir(dir), "Unrelated Decoration", "Example.Unrelated", "Unrelated Decoration")

	selection, err := ResolveNewGameModSelection(dir, "FrontierFarm")
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := ApplyNewGameModSelectionState(dir, selection)
	if err != nil {
		t.Fatal(err)
	}
	if !prepared.DependenciesReady || prepared.FarmTypeID != "FrontierFarm" {
		t.Fatalf("prepared selection = %+v", prepared)
	}
	for _, component := range prepared.Components {
		if !component.Enabled {
			t.Fatalf("required component remained disabled: %+v", component)
		}
	}
	if _, err := os.Stat(filepath.Join(disabledModsDir(dir), "Unrelated Decoration")); err != nil {
		t.Fatalf("unrelated Mod was not disabled: %v", err)
	}
	if _, err := os.Stat(filepath.Join(modsDir(dir), "Unrelated Decoration")); !os.IsNotExist(err) {
		t.Fatalf("unrelated Mod remained active: %v", err)
	}
}

func TestPrepareNewGameModsMoveFailureRollsBack(t *testing.T) {
	dir := t.TempDir()
	writeFrontierDependencyFixture(t, dir, frontierDependencyFixtureOptions{providerDisabled: true, contentPatcherDisabled: true})
	calls := 0
	mover := func(dataDir, folderName string, enabled bool) error {
		if enabled {
			calls++
			if calls == 2 {
				return errors.New("synthetic move failure")
			}
		}
		return moveModFolder(dataDir, folderName, enabled)
	}
	if _, err := prepareNewGameModsWithMover(dir, "FrontierFarm", mover); err == nil {
		t.Fatal("expected move failure")
	}
	for _, folder := range []string{"[CP] Frontier Farm", "Content Patcher"} {
		if _, err := os.Stat(filepath.Join(disabledModsDir(dir), folder)); err != nil {
			t.Fatalf("%s was not rolled back: %v", folder, err)
		}
	}
}

func TestPrepareNewGameModsRejectsUnknownMissingAndConflict(t *testing.T) {
	t.Run("unknown", func(t *testing.T) {
		dir := t.TempDir()
		_, err := PrepareNewGameMods(dir, "UnknownFarm")
		selectionErr, ok := IsNewGameModSelectionError(err)
		if !ok || selectionErr.Code != "farm_type_not_found" {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("missing", func(t *testing.T) {
		dir := t.TempDir()
		writeFrontierDependencyFixture(t, dir, frontierDependencyFixtureOptions{missingSVECode: true})
		_, err := PrepareNewGameMods(dir, "FrontierFarm")
		selectionErr, ok := IsNewGameModSelectionError(err)
		if !ok || selectionErr.Code != "missing_required_mods" {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("conflict", func(t *testing.T) {
		dir := t.TempDir()
		writeFrontierDependencyFixture(t, dir, frontierDependencyFixtureOptions{})
		writeFarmCatalogMod(t, dir, true, "Other Frontier", `{"Name":"Other","UniqueID":"Other.Frontier","Version":"1","ContentPackFor":{"UniqueID":"Pathoschild.ContentPatcher"}}`, `{"Changes":[{"Action":"EditData","Target":"Data/AdditionalFarms","Entries":{"Other":{"ID":"FrontierFarm"}}}]}`)
		_, err := PrepareNewGameMods(dir, "FrontierFarm")
		selectionErr, ok := IsNewGameModSelectionError(err)
		if !ok || selectionErr.Code != "farm_type_conflict" {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestResolveNewGameModSelectionLegacyModdedRequiresExactlyOneFarm(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		_, err := ResolveNewGameModSelection(t.TempDir(), "modded")
		selectionErr, ok := IsNewGameModSelectionError(err)
		if !ok || selectionErr.Code != "farm_type_not_installed" {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("one", func(t *testing.T) {
		dir := t.TempDir()
		writeFrontierDependencyFixture(t, dir, frontierDependencyFixtureOptions{})
		selection, err := ResolveNewGameModSelection(dir, "modded")
		if err != nil {
			t.Fatal(err)
		}
		if selection.FarmTypeID != "FrontierFarm" {
			t.Fatalf("selection = %+v", selection)
		}
	})

	t.Run("multiple", func(t *testing.T) {
		dir := t.TempDir()
		writeFrontierDependencyFixture(t, dir, frontierDependencyFixtureOptions{})
		writeFarmCatalogMod(t, dir, true, "Other Farm", `{"Name":"Other","UniqueID":"Other.Farm","Version":"1","ContentPackFor":{"UniqueID":"Pathoschild.ContentPatcher"}}`, `{"Changes":[{"Action":"EditData","Target":"Data/AdditionalFarms","Entries":{"OtherFarm":{"ID":"OtherFarm"}}}]}`)
		_, err := ResolveNewGameModSelection(dir, "modded")
		selectionErr, ok := IsNewGameModSelectionError(err)
		if !ok || selectionErr.Code != "farm_type_conflict" {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestFarmCatalogDependenciesRealInstanceReadOnly(t *testing.T) {
	dataDir := strings.TrimSpace(os.Getenv("STARDEW_REAL_DATA_DIR"))
	if dataDir == "" {
		t.Skip("STARDEW_REAL_DATA_DIR is not set")
	}
	catalog, err := ScanFarmCatalog(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	selections, err := BuildFarmDependencySelections(dataDir, catalog)
	if err != nil {
		t.Fatal(err)
	}
	selection := findDependencySelection(t, selections, "FrontierFarm")
	ids := make([]string, 0, len(selection.Components))
	for _, component := range selection.Components {
		ids = append(ids, component.UniqueID)
	}
	t.Logf("FrontierFarm readiness=%s components=%s missing=%v disabled=%v optional=%v", selection.Readiness, strings.Join(ids, ","), selection.MissingRequiredModKeys, selection.DisabledRequiredModKeys, selection.OptionalDependencyKeys)
	if selection.ProviderModID == "" || len(selection.Components) == 0 {
		t.Fatalf("invalid real selection: %+v", selection)
	}
}

type frontierDependencyFixtureOptions struct {
	providerDisabled       bool
	contentPatcherDisabled bool
	missingSVECode         bool
	cycle                  bool
}

func writeFrontierDependencyFixture(t *testing.T, dataDir string, options frontierDependencyFixtureOptions) {
	t.Helper()
	providerManifest := `{
  "Name":"Frontier Farm","UniqueID":"flashshifter.FrontierFarm","Version":"1.15.11",
  "ContentPackFor":{"UniqueID":"Pathoschild.ContentPatcher"},
  "Dependencies":[
    {"UniqueID":"Pathoschild.ContentPatcher"},
    {"UniqueID":"FlashShifter.StardewValleyExpandedCP"},
    {"UniqueID":"Author.OptionalDecoration","IsRequired":false}
  ]
}`
	providerContent := `{"Changes":[{"Action":"EditData","Target":"Data/AdditionalFarms","Entries":{"FlashShifter.FrontierFarm/FrontierFarm":{"ID":"FrontierFarm"}}}]}`
	writeFarmCatalogMod(t, dataDir, !options.providerDisabled, "[CP] Frontier Farm", providerManifest, providerContent)

	cpRoot := modsDir(dataDir)
	if options.contentPatcherDisabled {
		cpRoot = disabledModsDir(dataDir)
	}
	createTestModWithManifest(t, cpRoot, "Content Patcher", modManifest{Name: "Content Patcher", UniqueID: "Pathoschild.ContentPatcher", Version: "2.0", Author: "Pathoschild"})
	createTestModWithManifest(t, modsDir(dataDir), "[CP] SVE", modManifest{
		Name: "SVE CP", UniqueID: "FlashShifter.StardewValleyExpandedCP", Version: "1.0", Author: "FlashShifter",
		Dependencies: []modManifestDependency{{UniqueID: "FlashShifter.SVE-FTM"}, {UniqueID: "FlashShifter.SVECode"}},
	})
	createTestModWithManifest(t, modsDir(dataDir), "[FTM] SVE", modManifest{Name: "SVE FTM", UniqueID: "FlashShifter.SVE-FTM", Version: "1.0", Author: "FlashShifter"})
	if !options.missingSVECode {
		codeDeps := []modManifestDependency{}
		if options.cycle {
			codeDeps = append(codeDeps, modManifestDependency{UniqueID: "FlashShifter.StardewValleyExpandedCP"})
		}
		createTestModWithManifest(t, modsDir(dataDir), "SVE Code", modManifest{Name: "SVE Code", UniqueID: "FlashShifter.SVECode", Version: "1.0", Author: "FlashShifter", Dependencies: codeDeps})
	}
}

func findDependencySelection(t *testing.T, selections []NewGameModSelection, farmTypeID string) NewGameModSelection {
	t.Helper()
	for _, selection := range selections {
		if selection.FarmTypeID == farmTypeID {
			return selection
		}
	}
	t.Fatalf("selection %s not found: %+v", farmTypeID, selections)
	return NewGameModSelection{}
}

func countString(values []string, target string) int {
	count := 0
	for _, value := range values {
		if value == target {
			count++
		}
	}
	return count
}
