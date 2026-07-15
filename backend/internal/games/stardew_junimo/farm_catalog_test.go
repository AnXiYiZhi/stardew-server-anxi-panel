package stardew_junimo

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFarmCatalogRecognizesFrontierFarmAuthority(t *testing.T) {
	dataDir := t.TempDir()
	writeFarmCatalogMod(t, dataDir, true, "[CP] Frontier Farm", `{
		"Name":"Frontier Farm", "UniqueID":"FlashShifter.FrontierFarm", "Version":"1.15.11",
		"ContentPackFor":{"UniqueID":"Pathoschild.ContentPatcher"},
		"Dependencies":[{"UniqueID":"FlashShifter.StardewValleyExpandedCP","MinimumVersion":"1.15","IsRequired":true}]
	}`, `{
		"Changes":[
		{
			"Action":"EditData", "Target":"Data/AdditionalFarms",
			"Entries":{"FlashShifter.FrontierFarm/FrontierFarm":{
				"ID":"FrontierFarm", "TooltipStringPath":"Strings/UI:FrontierFarm",
				"MapName":"Farm_FrontierFarm", "IconTexture":"Mods/FlashShifter.FrontierFarm/Icon",
				"WorldMapTexture":"Mods/FlashShifter.FrontierFarm/WorldMap"
			}}
		},
		{"Action":"Load","Target":"Mods/FlashShifter.FrontierFarm/Icon","FromFile":"Assets/Tilesheets/Icon.png"},
		{"Action":"EditData","Target":"Strings/UI","Entries":{"FrontierFarm":"{{i18n:FrontierFarm.Description}}"}}
		]
	}`)
	writeFarmCatalogFile(t, filepath.Join(dataDir, ".local-container", "mods", "[CP] Frontier Farm"), "i18n/zh.json", `{
		"FrontierFarm.Description":"边境农场_一大片与芬吉尔共和国边境接壤的土地……"
	}`)
	writeFarmCatalogFile(t, filepath.Join(dataDir, ".local-container", "mods", "[CP] Frontier Farm"), "i18n/default.json", `{
		"FrontierFarm.Description":"Frontier Farm_An expansive plot of land."
	}`)
	writeFarmCatalogPNG(t, filepath.Join(dataDir, ".local-container", "mods", "[CP] Frontier Farm"), "Assets/Tilesheets/Icon.png", 22, 20)

	result := scanFarmCatalogForTest(t, dataDir)
	if len(result.Farms) != 1 {
		t.Fatalf("farms = %+v, want one", result.Farms)
	}
	farm := result.Farms[0]
	if farm.ID != "FrontierFarm" || farm.ID == farm.EntryKey {
		t.Fatalf("ID=%q EntryKey=%q, want entry ID rather than namespaced key", farm.ID, farm.EntryKey)
	}
	if farm.EntryKey != "FlashShifter.FrontierFarm/FrontierFarm" || farm.MapName != "Farm_FrontierFarm" || farm.TooltipStringPath != "Strings/UI:FrontierFarm" {
		t.Fatalf("farm = %+v", farm)
	}
	if farm.ProviderModID != "FlashShifter.FrontierFarm" || farm.ProviderFolder != "[CP] Frontier Farm" || !farm.Enabled || farm.Confidence != FarmCatalogConfidenceExplicit {
		t.Fatalf("provider metadata = %+v", farm)
	}
	if farm.Label != "边境农场" || farm.Description != "一大片与芬吉尔共和国边境接壤的土地……" {
		t.Fatalf("display metadata = label %q description %q", farm.Label, farm.Description)
	}
	if farm.IconFile != "Assets/Tilesheets/Icon.png" || farm.IconMediaType != "image/png" || farm.IconWidth != 22 || farm.IconHeight != 20 {
		t.Fatalf("icon metadata = %+v", farm)
	}
	if len(result.Mods) != 1 || result.Mods[0].ContentPackFor == nil || result.Mods[0].ContentPackFor.UniqueID != contentPatcherModID || len(result.Mods[0].Dependencies) != 1 {
		t.Fatalf("mods = %+v", result.Mods)
	}
}

func TestFarmCatalogNameFallbackPriority(t *testing.T) {
	tests := []struct {
		name         string
		manifestName string
		currentI18n  string
		defaultI18n  string
		want         string
	}{
		{name: "missing current language uses default", manifestName: "Manifest Farm", defaultI18n: "Default Farm_Default description", want: "Default Farm"},
		{name: "empty current title uses default", manifestName: "Manifest Farm", currentI18n: "_current description", defaultI18n: "Default Farm_Default description", want: "Default Farm"},
		{name: "missing default uses manifest", manifestName: "Manifest Farm", want: "Manifest Farm"},
		{name: "missing manifest uses ID", want: "FallbackFarm"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dataDir := t.TempDir()
			root := writeFarmCatalogMod(t, dataDir, true, "Fallback", farmCatalogCPManifest("Author.Fallback", test.manifestName), farmCatalogMetadataContent("FallbackFarm", "Fallback", "{{i18n:Fallback.Description}}", ""))
			if test.currentI18n != "" {
				data, _ := json.Marshal(map[string]string{"Fallback.Description": test.currentI18n})
				writeFarmCatalogFile(t, root, "i18n/zh.json", string(data))
			}
			if test.defaultI18n != "" {
				data, _ := json.Marshal(map[string]string{"Fallback.Description": test.defaultI18n})
				writeFarmCatalogFile(t, root, "i18n/default.json", string(data))
			}
			result := scanFarmCatalogForLanguageTest(t, dataDir, "zh-CN")
			if len(result.Farms) != 1 || result.Farms[0].Label != test.want {
				t.Fatalf("farms = %+v, want label %q", result.Farms, test.want)
			}
		})
	}
}

func TestFarmCatalogNameResolutionWarnings(t *testing.T) {
	tests := []struct {
		name      string
		stringsUI string
		i18n      map[string]string
		warning   string
	}{
		{name: "i18n token missing", stringsUI: "{{i18n:Missing.Key}}", i18n: map[string]string{"Other.Key": "Other"}, warning: "i18n key"},
		{name: "Strings UI entry missing", stringsUI: "", i18n: map[string]string{"Farm.Description": "Name_Description"}, warning: "Strings/UI entry"},
		{name: "arbitrary Content Patcher token is not executed", stringsUI: "{{Query: unsafe}}", warning: "not a supported exact i18n token"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dataDir := t.TempDir()
			content := farmCatalogMetadataContent("WarnFarm", "Farm", test.stringsUI, "")
			root := writeFarmCatalogMod(t, dataDir, true, "Warnings", farmCatalogCPManifest("Author.Warnings", "Manifest Farm"), content)
			if test.i18n != nil {
				data, _ := json.Marshal(test.i18n)
				writeFarmCatalogFile(t, root, "i18n/zh.json", string(data))
			}
			result := scanFarmCatalogForTest(t, dataDir)
			if len(result.Farms) != 1 || result.Farms[0].Label != "Manifest Farm" {
				t.Fatalf("fallback farm = %+v", result.Farms)
			}
			assertFarmCatalogWarning(t, result, test.warning)
		})
	}
}

func TestFarmCatalogI18nTitleDescriptionSplittingAndLimits(t *testing.T) {
	longTitle := strings.Repeat("名", farmCatalogMaxLabelRunes+20)
	longDescription := strings.Repeat("说", farmCatalogMaxDescriptionRunes+20)
	values := map[string]string{
		"NoUnderscore": "只有标题",
		"Multiple":     "标题_说明_保留_后续",
		"Long":         longTitle + "_说\u0001" + longDescription,
	}
	dataDir := t.TempDir()
	content := `{"Changes":[
		{"Action":"EditData","Target":"Data/AdditionalFarms","Entries":{
			"No":{"ID":"No","TooltipStringPath":"Strings/UI:No"},
			"Multiple":{"ID":"Multiple","TooltipStringPath":"Strings/UI:Multiple"},
			"Long":{"ID":"Long","TooltipStringPath":"Strings/UI:Long"}
		}},
		{"Action":"EditData","Target":"Strings/UI","Entries":{
			"No":"{{i18n:NoUnderscore}}","Multiple":"{{i18n:Multiple}}","Long":"{{i18n:Long}}"
		}}
	]}`
	root := writeFarmCatalogMod(t, dataDir, true, "Splitting", farmCatalogCPManifest("Author.Splitting", "Manifest"), content)
	i18nData, _ := json.Marshal(values)
	writeFarmCatalogFile(t, root, "i18n/zh.json", string(i18nData))
	result := scanFarmCatalogForTest(t, dataDir)
	byID := map[string]FarmCatalogEntry{}
	for _, farm := range result.Farms {
		byID[farm.ID] = farm
	}
	if byID["No"].Label != "只有标题" || byID["No"].Description != "" {
		t.Fatalf("no underscore = %+v", byID["No"])
	}
	if byID["Multiple"].Label != "标题" || byID["Multiple"].Description != "说明_保留_后续" {
		t.Fatalf("multiple underscores = %+v", byID["Multiple"])
	}
	if len([]rune(byID["Long"].Label)) != farmCatalogMaxLabelRunes || len([]rune(byID["Long"].Description)) != farmCatalogMaxDescriptionRunes || strings.ContainsRune(byID["Long"].Description, '\u0001') {
		t.Fatalf("long/control value = label runes %d description runes %d", len([]rune(byID["Long"].Label)), len([]rune(byID["Long"].Description)))
	}
}

func TestFarmCatalogIgnoresFTMFarmTypesAndFarmTypeConditions(t *testing.T) {
	dataDir := t.TempDir()
	writeFarmCatalogMod(t, dataDir, true, "[FTM] Frontier Farm", `{
		"Name":"FTM Frontier", "UniqueID":"FlashShifter.FrontierFarmFTM", "Version":"1",
		"ContentPackFor":{"UniqueID":"Esca.FarmTypeManager"}
	}`, `{"File_Conditions":{"FarmTypes":["FrontierFarm"]}}`)
	writeFarmCatalogMod(t, dataDir, true, "[CP] Conditions", farmCatalogCPManifest("Author.Conditions", "Conditions"), `{
		"Changes":[
			{"Action":"EditMap","Target":"Maps/Farm","When":{"FarmType":"FrontierFarm"}},
			{"Action":"EditData","Target":"Data/Objects","Entries":{"x":{"FarmType":"FrontierFarm","FarmTypes":["FrontierFarm"]}}}
		],
		"FarmType":"FrontierFarm",
		"FarmTypes":["FrontierFarm"]
	}`)

	result := scanFarmCatalogForTest(t, dataDir)
	if len(result.Farms) != 0 {
		t.Fatalf("condition-only files produced farms: %+v", result.Farms)
	}
	if len(result.Mods) != 2 {
		t.Fatalf("mods = %+v", result.Mods)
	}
}

func TestFarmCatalogSupportsIDIdAndSafeEntryKeyFallback(t *testing.T) {
	dataDir := t.TempDir()
	writeFarmCatalogMod(t, dataDir, true, "Fields", farmCatalogCPManifest("Author.Fields", "Fields"), `{
		"Changes":[{"Action":"editdata","Target":" data\\AdditionalFarms/ ","Entries":{
			"Upper":{"ID":"UPPER","Id":"ignored"},
			"Mixed":{"Id":"MixedCase"},
			"Fallback_1":{"MapName":"Farm_Fallback"},
			"Author.Pack/Unsafe":{"MapName":"must-not-register"}
		}}]
	}`)
	result := scanFarmCatalogForTest(t, dataDir)
	if got := farmCatalogIDs(result.Farms); strings.Join(got, ",") != "Fallback_1,MixedCase,UPPER" {
		t.Fatalf("IDs = %v", got)
	}
	for _, farm := range result.Farms {
		if farm.ID == "Fallback_1" && farm.Confidence != FarmCatalogConfidenceEntryKey {
			t.Fatalf("fallback confidence = %q", farm.Confidence)
		}
	}
}

func TestFarmCatalogJSONCBOMCommentsAndTrailingCommas(t *testing.T) {
	dataDir := t.TempDir()
	manifest := "\ufeff{ // manifest comment\n" +
		`"Name":"JSONC", "UniqueID":"Author.JSONC", "Version":"1",` + "\n" +
		`"ContentPackFor":{"UniqueID":"Pathoschild.ContentPatcher",},` + "\n" +
		`/* block */ "Dependencies":[],` + "\n}"
	content := "\ufeff{ /* content */ \"Changes\":[{\n" +
		`"Action":"EditData", "Target":"Data/AdditionalFarms", "Entries":{"Jsonc":{"ID":"JsoncFarm",},},` +
		"\n},],}"
	writeFarmCatalogMod(t, dataDir, true, "JSONC", manifest, content)
	result := scanFarmCatalogForTest(t, dataDir)
	if got := farmCatalogIDs(result.Farms); len(got) != 1 || got[0] != "JsoncFarm" {
		t.Fatalf("farms = %+v warnings=%v", result.Farms, result.Warnings)
	}
}

func TestFarmCatalogIncludeNestedAndConditions(t *testing.T) {
	dataDir := t.TempDir()
	root := writeFarmCatalogMod(t, dataDir, true, "Includes", farmCatalogCPManifest("Author.Includes", "Includes"), `{
		"Changes":[{"Action":"Include","FromFile":"patches/first.json","When":{"HasMod":"Example.Dependency"}}]
	}`)
	writeFarmCatalogFile(t, root, "patches/first.json", `[
		{"Action":"Include","FromFile":"patches/nested/second.json"},
		{"Action":"EditData","Target":"Data/AdditionalFarms","Entries":{"First":{"ID":"FirstFarm"}}}
	]`)
	writeFarmCatalogFile(t, root, "patches/nested/second.json", `{
		"Action":"EditData","Target":"Data/AdditionalFarms","When":{"Season":"spring"},
		"Entries":{"Second":{"Id":"SecondFarm"}}
	}`)

	result := scanFarmCatalogForTest(t, dataDir)
	if got := strings.Join(farmCatalogIDs(result.Farms), ","); got != "FirstFarm,SecondFarm" {
		t.Fatalf("IDs = %s warnings=%v", got, result.Warnings)
	}
	for _, farm := range result.Farms {
		if len(farm.Conditions) == 0 || farm.Conditions[0].Kind != "include" || !bytes.Contains(farm.Conditions[0].When, []byte("HasMod")) {
			t.Fatalf("include condition not retained: %+v", farm.Conditions)
		}
		if farm.ID == "SecondFarm" && len(farm.Conditions) != 2 {
			t.Fatalf("nested patch conditions = %+v", farm.Conditions)
		}
	}
}

func TestFarmCatalogIncludeCycleIsWarning(t *testing.T) {
	dataDir := t.TempDir()
	root := writeFarmCatalogMod(t, dataDir, true, "Cycle", farmCatalogCPManifest("Author.Cycle", "Cycle"), `{
		"Changes":[{"Action":"Include","FromFile":"a.json"}]
	}`)
	writeFarmCatalogFile(t, root, "a.json", `[{"Action":"Include","FromFile":"b.json"}]`)
	writeFarmCatalogFile(t, root, "b.json", `[{"Action":"Include","FromFile":"a.json"}]`)
	result := scanFarmCatalogForTest(t, dataDir)
	assertFarmCatalogWarning(t, result, "include cycle")
}

func TestFarmCatalogRejectsIncludeTraversalAndSymlinkEscape(t *testing.T) {
	dataDir := t.TempDir()
	root := writeFarmCatalogMod(t, dataDir, true, "Traversal", farmCatalogCPManifest("Author.Traversal", "Traversal"), `{
		"Changes":[{"Action":"Include","FromFile":"../outside.json"}]
	}`)
	writeFarmCatalogFile(t, filepath.Dir(root), "outside.json", `[]`)
	result := scanFarmCatalogForTest(t, dataDir)
	assertFarmCatalogWarning(t, result, "escapes mod root")

	if runtime.GOOS == "windows" {
		target := filepath.Join(dataDir, "external.json")
		if err := os.WriteFile(target, []byte(`[]`), 0o644); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, "linked.json")
		if err := os.Symlink(target, link); err != nil {
			t.Logf("symlink unavailable, traversal assertion already covered: %v", err)
			return
		}
		if err := os.WriteFile(filepath.Join(root, "content.json"), []byte(`{"Changes":[{"Action":"Include","FromFile":"linked.json"}]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		result = scanFarmCatalogForTest(t, dataDir)
		assertFarmCatalogWarning(t, result, "escapes mod root")
	}
}

func TestFarmCatalogOversizeAndBrokenJSONAreWarnings(t *testing.T) {
	dataDir := t.TempDir()
	largeRoot := writeFarmCatalogMod(t, dataDir, true, "Large", farmCatalogCPManifest("Author.Large", "Large"), "{}")
	large := bytes.Repeat([]byte{' '}, int(farmCatalogMaxFileBytes)+1)
	if err := os.WriteFile(filepath.Join(largeRoot, "content.json"), large, 0o644); err != nil {
		t.Fatal(err)
	}
	writeFarmCatalogMod(t, dataDir, true, "Broken", farmCatalogCPManifest("Author.Broken", "Broken"), `{"Changes":[`)
	result := scanFarmCatalogForTest(t, dataDir)
	assertFarmCatalogWarning(t, result, "exceeds")
	assertFarmCatalogWarning(t, result, "JSON parse failed")
}

func TestFarmCatalogRejectsInvalidFarmIDs(t *testing.T) {
	dataDir := t.TempDir()
	longID := strings.Repeat("x", farmCatalogMaxIDBytes+1)
	writeFarmCatalogMod(t, dataDir, true, "Invalid IDs", farmCatalogCPManifest("Author.InvalidIDs", "Invalid IDs"), `{
		"Changes":[{"Action":"EditData","Target":"Data/AdditionalFarms","Entries":{
			"Control":{"ID":"bad\u0001id"},
			"Long":{"ID":"`+longID+`"}
		}}]
	}`)
	invalidUTF8Root := writeFarmCatalogMod(t, dataDir, true, "Invalid UTF8", farmCatalogCPManifest("Author.InvalidUTF8", "Invalid UTF8"), "{}")
	invalidUTF8 := append([]byte(`{"Changes":[{"Action":"EditData","Target":"Data/AdditionalFarms","Entries":{"Bad":{"ID":"`), 0xff)
	invalidUTF8 = append(invalidUTF8, []byte(`"}}}]}`)...)
	if err := os.WriteFile(filepath.Join(invalidUTF8Root, "content.json"), invalidUTF8, 0o644); err != nil {
		t.Fatal(err)
	}

	result := scanFarmCatalogForTest(t, dataDir)
	if len(result.Farms) != 0 {
		t.Fatalf("invalid IDs produced farms: %+v", result.Farms)
	}
	assertFarmCatalogWarning(t, result, "control characters")
	assertFarmCatalogWarning(t, result, "longer than")
	assertFarmCatalogWarning(t, result, "not valid UTF-8")
}

func TestFarmCatalogDuplicateDeclarationsAndCrossModConflicts(t *testing.T) {
	dataDir := t.TempDir()
	root := writeFarmCatalogMod(t, dataDir, true, "Provider A", farmCatalogCPManifest("Author.A", "A"), `{
		"Changes":[
			{"Action":"Include","FromFile":"same.json"},
			{"Action":"Include","FromFile":"same.json"}
		]
	}`)
	writeFarmCatalogFile(t, root, "same.json", `[{"Action":"EditData","Target":"Data/AdditionalFarms","Entries":{"A":{"ID":"SharedFarm","MapName":"Farm_A"}}}]`)
	writeFarmCatalogMod(t, dataDir, false, "Provider B", farmCatalogCPManifest("Author.B", "B"), `{
		"Changes":[{"Action":"EditData","Target":"Data/AdditionalFarms","Entries":{"B":{"ID":"SharedFarm","MapName":"Farm_B"}}}]
	}`)

	result := scanFarmCatalogForTest(t, dataDir)
	if len(result.Farms) != 2 {
		t.Fatalf("farms = %+v, want duplicate in A removed and B retained", result.Farms)
	}
	if len(result.Conflicts) != 1 || result.Conflicts[0].ID != "SharedFarm" || len(result.Conflicts[0].Sources) != 2 {
		t.Fatalf("conflicts = %+v", result.Conflicts)
	}
	for _, farm := range result.Farms {
		if !farm.Conflict || len(farm.ConflictSources) != 2 {
			t.Fatalf("farm conflict state = %+v", farm)
		}
	}
	if !result.Farms[0].Enabled || result.Farms[1].Enabled {
		t.Fatalf("enabled ordering/state = %+v", result.Farms)
	}
}

func TestFarmCatalogEnabledDisabledAndMissingDirectories(t *testing.T) {
	missing := scanFarmCatalogForTest(t, t.TempDir())
	if missing.Mods == nil || missing.Farms == nil || missing.Conflicts == nil || missing.Warnings == nil {
		t.Fatalf("missing directory result must use non-nil empty lists: %#v", missing)
	}

	dataDir := t.TempDir()
	writeFarmCatalogMod(t, dataDir, true, "Enabled", farmCatalogCPManifest("Author.Enabled", "Enabled"), farmCatalogSingleFarmContent("EnabledFarm"))
	writeFarmCatalogMod(t, dataDir, false, "Disabled", farmCatalogCPManifest("Author.Disabled", "Disabled"), farmCatalogSingleFarmContent("DisabledFarm"))
	result := scanFarmCatalogForTest(t, dataDir)
	states := map[string]bool{}
	for _, farm := range result.Farms {
		states[farm.ID] = farm.Enabled
	}
	if !states["EnabledFarm"] || states["DisabledFarm"] {
		t.Fatalf("states = %#v", states)
	}
}

func TestFarmCatalogRejectsUnsafeOrInvalidIconsWithoutDroppingFarm(t *testing.T) {
	tests := []struct {
		name     string
		fromFile string
		setup    func(t *testing.T, dataDir, modRoot string)
		warning  string
	}{
		{
			name: "path traversal", fromFile: "../outside.png", warning: "unsafe",
			setup: func(t *testing.T, dataDir, modRoot string) {
				writeFarmCatalogPNG(t, filepath.Dir(modRoot), "outside.png", 1, 1)
			},
		},
		{
			name: "fake PNG", fromFile: "Assets/Icon.png", warning: "unrecognized image header",
			setup: func(t *testing.T, dataDir, modRoot string) {
				writeFarmCatalogFile(t, modRoot, "Assets/Icon.png", "not a PNG")
			},
		},
		{
			name: "SVG rejected", fromFile: "Assets/Icon.svg", warning: "unsupported format",
			setup: func(t *testing.T, dataDir, modRoot string) {
				writeFarmCatalogFile(t, modRoot, "Assets/Icon.svg", `<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
			},
		},
		{
			name: "missing image", fromFile: "Assets/Missing.png", warning: "does not exist",
			setup: func(t *testing.T, dataDir, modRoot string) {},
		},
		{
			name: "file too large", fromFile: "Assets/Huge.png", warning: "unavailable or unsafe",
			setup: func(t *testing.T, dataDir, modRoot string) {
				data := append([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, bytes.Repeat([]byte{0}, int(farmCatalogMaxFileBytes))...)
				writeFarmCatalogBytes(t, modRoot, "Assets/Huge.png", data)
			},
		},
		{
			name: "dimensions too large", fromFile: "Assets/Wide.png", warning: "dimensions",
			setup: func(t *testing.T, dataDir, modRoot string) {
				writeFarmCatalogPNG(t, modRoot, "Assets/Wide.png", farmCatalogMaxIconDimension+1, 1)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dataDir := t.TempDir()
			content := farmCatalogMetadataContent("IconFarm", "", "", test.fromFile)
			root := writeFarmCatalogMod(t, dataDir, true, "Icons", farmCatalogCPManifest("Author.Icons", "Icon Farm"), content)
			test.setup(t, dataDir, root)
			result := scanFarmCatalogForTest(t, dataDir)
			if len(result.Farms) != 1 || result.Farms[0].ID != "IconFarm" {
				t.Fatalf("icon error dropped farm: %+v", result.Farms)
			}
			if result.Farms[0].IconFile != "" {
				t.Fatalf("unsafe icon exposed: %+v", result.Farms[0])
			}
			for _, warning := range result.Warnings {
				if strings.Contains(warning, dataDir) {
					t.Fatalf("warning exposes host path: %q", warning)
				}
			}
			assertFarmCatalogWarning(t, result, test.warning)
		})
	}
}

func TestFarmCatalogRejectsIconSymlinkEscape(t *testing.T) {
	dataDir := t.TempDir()
	root := writeFarmCatalogMod(t, dataDir, true, "Symlink Icon", farmCatalogCPManifest("Author.SymlinkIcon", "Symlink Icon"), farmCatalogMetadataContent("SymlinkFarm", "", "", "Assets/Icon.png"))
	writeFarmCatalogPNG(t, dataDir, "external.png", 2, 2)
	if err := os.MkdirAll(filepath.Join(root, "Assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dataDir, "external.png"), filepath.Join(root, "Assets", "Icon.png")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	result := scanFarmCatalogForTest(t, dataDir)
	if len(result.Farms) != 1 || result.Farms[0].IconFile != "" {
		t.Fatalf("symlink icon result = %+v", result.Farms)
	}
	assertFarmCatalogWarning(t, result, "unavailable or unsafe")
}

func TestFarmCatalogIconHeaderFormats(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	var jpegData bytes.Buffer
	if err := jpeg.Encode(&jpegData, img, nil); err != nil {
		t.Fatal(err)
	}
	mediaType, width, height, err := inspectFarmCatalogIcon(jpegData.Bytes(), ".jpg")
	if err != nil || mediaType != "image/jpeg" || width != 3 || height != 2 {
		t.Fatalf("JPEG inspection = %q %dx%d err=%v", mediaType, width, height, err)
	}

	webp := []byte{
		'R', 'I', 'F', 'F', 22, 0, 0, 0, 'W', 'E', 'B', 'P',
		'V', 'P', '8', 'X', 10, 0, 0, 0,
		0, 0, 0, 0, 1, 0, 0, 2, 0, 0,
	}
	mediaType, width, height, err = inspectFarmCatalogIcon(webp, ".webp")
	if err != nil || mediaType != "image/webp" || width != 2 || height != 3 {
		t.Fatalf("WebP inspection = %q %dx%d err=%v", mediaType, width, height, err)
	}
}

func farmCatalogCPManifest(id, name string) string {
	return `{"Name":"` + name + `","UniqueID":"` + id + `","Version":"1.0.0","ContentPackFor":{"UniqueID":"Pathoschild.ContentPatcher"}}`
}

func farmCatalogMetadataContent(id, tooltipKey, stringsValue, iconFromFile string) string {
	entry := map[string]any{"ID": id}
	changes := []any{}
	if tooltipKey != "" {
		entry["TooltipStringPath"] = "Strings/UI:" + tooltipKey
	}
	if iconFromFile != "" {
		entry["IconTexture"] = "Mods/Test/Icon"
	}
	changes = append(changes, map[string]any{
		"Action": "EditData", "Target": "Data/AdditionalFarms", "Entries": map[string]any{"Entry": entry},
	})
	if tooltipKey != "" && stringsValue != "" {
		changes = append(changes, map[string]any{
			"Action": "EditData", "Target": "Strings/UI", "Entries": map[string]any{tooltipKey: stringsValue},
		})
	}
	if iconFromFile != "" {
		changes = append(changes, map[string]any{
			"Action": "Load", "Target": "Mods/Test/Icon", "FromFile": iconFromFile,
		})
	}
	data, _ := json.Marshal(map[string]any{"Changes": changes})
	return string(data)
}

func farmCatalogSingleFarmContent(id string) string {
	return `{"Changes":[{"Action":"EditData","Target":"Data/AdditionalFarms","Entries":{"Entry":{"ID":"` + id + `"}}}]}`
}

func writeFarmCatalogMod(t *testing.T, dataDir string, enabled bool, folder, manifest, content string) string {
	t.Helper()
	rootName := "mods-disabled"
	if enabled {
		rootName = "mods"
	}
	root := filepath.Join(dataDir, ".local-container", rootName, folder)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if content != "" {
		if err := os.WriteFile(filepath.Join(root, "content.json"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func writeFarmCatalogFile(t *testing.T, root, relative, content string) {
	t.Helper()
	name := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeFarmCatalogBytes(t *testing.T, root, relative string, content []byte) {
	t.Helper()
	name := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeFarmCatalogPNG(t *testing.T, root, relative string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{R: 0x4a, G: 0x8b, B: 0x57, A: 0xff})
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	writeFarmCatalogBytes(t, root, relative, data.Bytes())
}

func scanFarmCatalogForTest(t *testing.T, dataDir string) FarmCatalogResult {
	t.Helper()
	result, err := ScanFarmCatalog(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func scanFarmCatalogForLanguageTest(t *testing.T, dataDir, language string) FarmCatalogResult {
	t.Helper()
	result, err := ScanFarmCatalogWithOptions(dataDir, FarmCatalogOptions{Language: language})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func farmCatalogIDs(farms []FarmCatalogEntry) []string {
	ids := make([]string, 0, len(farms))
	for _, farm := range farms {
		ids = append(ids, farm.ID)
	}
	return ids
}

func assertFarmCatalogWarning(t *testing.T, result FarmCatalogResult, contains string) {
	t.Helper()
	for _, warning := range result.Warnings {
		if strings.Contains(warning, contains) {
			return
		}
	}
	t.Fatalf("warnings %v do not contain %q", result.Warnings, contains)
}
