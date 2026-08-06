package stardew_junimo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestGetPlayerModDetailsComparesActualLoadedMods(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Date(2026, 8, 6, 4, 5, 6, 0, time.UTC)
	for _, mod := range []struct {
		folder, uniqueID, name, version string
	}{
		{"RequiredMatch", "Example.RequiredMatch", "Required Match", "1.0"},
		{"RequiredMissing", "Example.RequiredMissing", "Required Missing", "2.0.0"},
		{"VersionMismatch", "Example.VersionMismatch", "Version Mismatch", "2.0.0"},
		{"ServerOnly", "Example.ServerOnly", "Server Only", "1.0.0"},
		{controlModFolderName, controlModUniqueID, "Panel Control", "0.3.0"},
		{junimoServerModFolderName, junimoServerModUniqueID, "JunimoServer", "1.5.0"},
	} {
		writePlayerModManifest(t, dataDir, mod.folder, mod.uniqueID, mod.name, mod.version)
	}
	if err := SetModSyncClassification(dataDir, "ServerOnly", registry.ModSyncKindServerOnly, "test server-only"); err != nil {
		t.Fatalf("classify server-only mod: %v", err)
	}

	writePlayerModOptions(t, dataDir, now, "1.6.15", "4.5.2", []runtimeLoadedMod{
		{UniqueID: "Example.RequiredMatch", Version: "1.0.0"},
		{UniqueID: "Example.RequiredMissing", Version: "2.0.0"},
		{UniqueID: "Example.VersionMismatch", Version: "2.0.0"},
		{UniqueID: "Example.ServerOnly", Version: "1.0.0"},
		{UniqueID: controlModUniqueID, Version: "0.3.0"},
		{UniqueID: junimoServerModUniqueID, Version: "1.5.0"},
	})
	writePlayerModContexts(t, dataDir, controlPlayerModContextsFile{
		SchemaVersion: playerModContextsSchemaVersion,
		UpdatedAt:     now,
		Players: map[string]controlPlayerModContext{
			"123": {
				UniqueMultiplayerID: "123",
				HasSmapi:            true,
				GameVersion:         "1.6.15",
				APIVersion:          "4.5.1",
				Mods: []PlayerReportedMod{
					{UniqueID: "example.requiredmatch", Name: "Client Match", Version: "1.0"},
					{UniqueID: "Example.VersionMismatch", Name: "Client Mismatch", Version: "1.0.0"},
					{UniqueID: "Example.ClientOnly", Name: "Client Only", Version: "3.0.0"},
					{UniqueID: "cJBoK.cHeAtSmEnU", Name: "CJB Cheats Menu", Version: "1.2.3"},
					{UniqueID: playerModSMAPIUniqueID, Name: "SMAPI", Version: "4.5.1"},
					{UniqueID: controlModUniqueID, Name: "Panel Control", Version: "0.3.0"},
					{UniqueID: junimoServerModUniqueID, Name: "JunimoServer", Version: "1.5.0"},
				},
				ContextStatus: PlayerModContextReported,
				ReportedAt:    &now,
				UpdatedAt:     now,
			},
		},
	})

	driver := &Driver{}
	result, err := driver.GetPlayerModDetails(context.Background(), registry.Instance{
		ID: "stardew", DataDir: dataDir, State: storage.InstanceStateRunning,
	}, "123")
	if err != nil {
		t.Fatalf("GetPlayerModDetails returned error: %v", err)
	}
	if result.ContextStatus != PlayerModContextReported || result.Mods == nil || len(result.Mods) != 7 {
		t.Fatalf("context = status %q mods %#v", result.ContextStatus, result.Mods)
	}
	if result.ServerContext == nil || len(result.ServerContext.LoadedMods) != 4 {
		t.Fatalf("server context = %#v, want four user-facing options.json loaded mods", result.ServerContext)
	}
	if result.Comparison.Status != playerModComparisonAvailable {
		t.Fatalf("comparison status = %q", result.Comparison.Status)
	}
	wantSummary := PlayerModComparisonSummary{Match: 1, MissingOnClient: 1, ClientOnly: 2, VersionMismatch: 1}
	if result.Comparison.Summary != wantSummary {
		t.Fatalf("summary = %+v, want %+v; items=%+v", result.Comparison.Summary, wantSummary, result.Comparison.Items)
	}
	if len(result.RiskFlags) != 1 || result.RiskFlags[0] != playerModRiskCJB {
		t.Fatalf("risk flags = %#v, want cjb", result.RiskFlags)
	}

	seenCJB := false
	for _, item := range result.Comparison.Items {
		if item.UniqueID == "Example.ServerOnly" {
			t.Fatalf("server_only mod was falsely reported missing: %+v", item)
		}
		if playerModComparisonIgnores(item.UniqueID) {
			t.Fatalf("panel-owned runtime component leaked into comparison: %+v", item)
		}
		if item.UniqueID == "cJBoK.cHeAtSmEnU" {
			seenCJB = len(item.RiskFlags) == 1 && item.RiskFlags[0] == playerModRiskCJB
		}
	}
	if !seenCJB {
		t.Fatal("case-insensitive official CJB UniqueID was not flagged")
	}
	if len(result.Comparison.Items) == 0 || result.Comparison.Items[0].Result != PlayerModResultClientOnly {
		t.Fatalf("comparison order = %+v, want player extras first", result.Comparison.Items)
	}
}

func TestComparePlayerModsIgnoresPanelOwnedRuntimeComponents(t *testing.T) {
	serverMods := map[string]PlayerServerMod{
		strings.ToLower(controlModUniqueID):      {UniqueID: controlModUniqueID, Enabled: true, SyncKind: registry.ModSyncKindClientRequired},
		strings.ToLower(junimoServerModUniqueID): {UniqueID: junimoServerModUniqueID, Enabled: true, SyncKind: registry.ModSyncKindClientRequired},
		strings.ToLower(playerModSMAPIUniqueID):  {UniqueID: playerModSMAPIUniqueID, Enabled: true, SyncKind: registry.ModSyncKindClientRequired},
	}
	client := controlPlayerModContext{HasSmapi: true, APIVersion: "0.0.1", Mods: []PlayerReportedMod{
		{UniqueID: strings.ToUpper(controlModUniqueID), Version: "9.9.9"},
		{UniqueID: strings.ToUpper(junimoServerModUniqueID), Version: "9.9.9"},
		{UniqueID: strings.ToUpper(playerModSMAPIUniqueID), Version: "9.9.9"},
	}}
	comparison := comparePlayerMods(PlayerModServerContext{APIVersion: "4.5.2"}, serverMods, client)
	if len(comparison.Items) != 0 || comparison.Summary != (PlayerModComparisonSummary{}) {
		t.Fatalf("panel-owned runtime comparison = %+v, want no items", comparison)
	}
}

func TestGetPlayerModDetailsDistinguishesNullAndReportedEmptyMods(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Date(2026, 8, 6, 4, 5, 6, 0, time.UTC)
	writePlayerModManifest(t, dataDir, "Required", "Example.Required", "Required", "1.0.0")
	writePlayerModOptions(t, dataDir, now, "1.6.15", "4.5.2", []runtimeLoadedMod{{UniqueID: "Example.Required", Version: "1.0.0"}})

	pending := controlPlayerModContextsFile{
		SchemaVersion: 1,
		UpdatedAt:     now,
		Players: map[string]controlPlayerModContext{
			"123": {UniqueMultiplayerID: "123", ContextStatus: PlayerModContextPending, Mods: nil, UpdatedAt: now},
		},
	}
	writePlayerModContexts(t, dataDir, pending)
	driver := &Driver{}
	instance := registry.Instance{ID: "stardew", DataDir: dataDir, State: storage.InstanceStateRunning}
	result, err := driver.GetPlayerModDetails(context.Background(), instance, "123")
	if err != nil {
		t.Fatal(err)
	}
	if result.Mods != nil || result.Comparison.Status != PlayerModComparisonUnavailable || result.Comparison.UnavailableReason != PlayerModContextPending {
		t.Fatalf("pending result = %+v", result)
	}

	reportedAt := now.Add(time.Second)
	pending.Players["123"] = controlPlayerModContext{
		UniqueMultiplayerID: "123", HasSmapi: true, GameVersion: "1.6.15", APIVersion: "4.5.2",
		Mods: []PlayerReportedMod{}, ContextStatus: PlayerModContextReported, ReportedAt: &reportedAt, UpdatedAt: reportedAt,
	}
	writePlayerModContexts(t, dataDir, pending)
	result, err = driver.GetPlayerModDetails(context.Background(), instance, "123")
	if err != nil {
		t.Fatal(err)
	}
	if result.Mods == nil || len(result.Mods) != 0 {
		t.Fatalf("reported empty mods = %#v, want non-nil empty slice", result.Mods)
	}
	if result.Comparison.Status != playerModComparisonAvailable || result.Comparison.Summary.MissingOnClient != 1 {
		t.Fatalf("reported empty comparison = %+v", result.Comparison)
	}
}

func TestGetPlayerModDetailsUnavailableBoundaries(t *testing.T) {
	driver := &Driver{}
	dataDir := t.TempDir()
	instance := registry.Instance{ID: "stardew", DataDir: dataDir, State: storage.InstanceStateRunning}
	result, err := driver.GetPlayerModDetails(context.Background(), instance, "123")
	if err != nil {
		t.Fatal(err)
	}
	if result.ContextStatus != PlayerModContextUnavailable || result.Mods != nil || result.Comparison.UnavailableReason != playerModUnavailableNotReported {
		t.Fatalf("missing context result = %+v", result)
	}

	if _, err := driver.GetPlayerModDetails(context.Background(), instance, "not-a-player"); err == nil {
		t.Fatal("invalid player ID was accepted")
	}

	now := time.Now().UTC()
	writePlayerModContexts(t, dataDir, controlPlayerModContextsFile{
		SchemaVersion: 1,
		UpdatedAt:     now,
		Players: map[string]controlPlayerModContext{
			"123": {UniqueMultiplayerID: "123", ContextStatus: PlayerModContextPending, Mods: []PlayerReportedMod{}, UpdatedAt: now},
		},
	})
	result, err = driver.GetPlayerModDetails(context.Background(), instance, "123")
	if err != nil {
		t.Fatal(err)
	}
	if result.Comparison.UnavailableReason != playerModUnavailableFileInvalid || result.Mods != nil {
		t.Fatalf("invalid pending empty array was not rejected: %+v", result)
	}

	writePlayerModContexts(t, dataDir, controlPlayerModContextsFile{
		SchemaVersion: 1,
		UpdatedAt:     now,
		Players: map[string]controlPlayerModContext{
			"123": {
				UniqueMultiplayerID: "123", HasSmapi: true, APIVersion: "4.5.2",
				Mods:          []PlayerReportedMod{{UniqueID: "CJBok.ItemSpawner", Name: "Item Spawner", Version: "2.0.0"}},
				ContextStatus: PlayerModContextStale, ReportedAt: &now, UpdatedAt: now,
			},
		},
	})
	result, err = driver.GetPlayerModDetails(context.Background(), instance, "123")
	if err != nil {
		t.Fatal(err)
	}
	if result.ContextStatus != PlayerModContextStale || result.Mods == nil || result.Comparison.UnavailableReason != PlayerModContextStale {
		t.Fatalf("stale context result = %+v", result)
	}
}

func TestPlayerModRiskFlagsOfficialCJBIDsCaseInsensitive(t *testing.T) {
	for _, uniqueID := range []string{"CJBok.CheatsMenu", "cjbOK.itemSPAWNER"} {
		flags := riskFlagsForPlayerMod(uniqueID)
		if len(flags) != 1 || flags[0] != playerModRiskCJB {
			t.Fatalf("risk flags for %q = %#v, want cjb", uniqueID, flags)
		}
	}
	if flags := riskFlagsForPlayerMod("Example.NotCJB"); len(flags) != 0 {
		t.Fatalf("non-CJB flags = %#v", flags)
	}
}

func TestComparePlayerModsServerOnlyIsNeverMissing(t *testing.T) {
	serverContext := PlayerModServerContext{APIVersion: "4.5.2"}
	serverOnly := PlayerServerMod{
		UniqueID: "Example.ServerOnly", Name: "Server Only", Version: "1.0.0",
		SyncKind: registry.ModSyncKindServerOnly, Enabled: true,
	}
	serverMods := map[string]PlayerServerMod{"example.serveronly": serverOnly}

	missingClient := controlPlayerModContext{HasSmapi: true, APIVersion: "4.5.2", Mods: []PlayerReportedMod{}}
	comparison := comparePlayerMods(serverContext, serverMods, missingClient)
	if comparison.Summary.MissingOnClient != 0 {
		t.Fatalf("server-only mod was counted as missing: %+v", comparison)
	}
	for _, item := range comparison.Items {
		if item.UniqueID == serverOnly.UniqueID {
			t.Fatalf("absent server-only mod should not create a comparison warning: %+v", item)
		}
	}

	reportedClient := controlPlayerModContext{
		HasSmapi:   true,
		APIVersion: "4.5.2",
		Mods:       []PlayerReportedMod{{UniqueID: "example.serveronly", Name: "Client label", Version: "1.0"}},
	}
	comparison = comparePlayerMods(serverContext, serverMods, reportedClient)
	found := false
	for _, item := range comparison.Items {
		if item.UniqueID == serverOnly.UniqueID {
			found = item.Result == PlayerModResultMatch && item.SyncKind == registry.ModSyncKindServerOnly
		}
	}
	if !found {
		t.Fatalf("a reported server-only mod was not retained as an informational match: %+v", comparison)
	}
}

func TestReadPlayerModContextBoundsAndDeduplicatesUntrustedFields(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Date(2026, 8, 6, 5, 6, 7, 0, time.UTC)
	longID := strings.Repeat("a", maxPlayerModUniqueIDChars+20)
	longName := "\t" + strings.Repeat("名", maxPlayerModNameChars+20) + "\r\n"
	longVersion := strings.Repeat("v", maxPlayerModVersionChars+20) + "\x00"
	writePlayerModContexts(t, dataDir, controlPlayerModContextsFile{
		SchemaVersion: playerModContextsSchemaVersion,
		UpdatedAt:     now,
		Players: map[string]controlPlayerModContext{
			"100": {
				UniqueMultiplayerID: "100", HasSmapi: true, GameVersion: longVersion, APIVersion: "4.5.2",
				Mods: []PlayerReportedMod{
					{UniqueID: longID, Name: longName, Version: longVersion},
					{UniqueID: strings.ToUpper(longID), Name: "duplicate", Version: "9.9.9"},
					{UniqueID: "\r\n\x00", Name: "empty ID", Version: "1.0.0"},
				},
				ContextStatus: PlayerModContextReported, ReportedAt: &now, UpdatedAt: now,
			},
		},
	})

	context, found, reason := readPlayerModContext(dataDir, "100")
	if !found || reason != "" {
		t.Fatalf("bounded context was rejected: found=%v reason=%q", found, reason)
	}
	if len(context.Mods) != 1 {
		t.Fatalf("normalized mods = %#v, want one case-insensitive unique entry", context.Mods)
	}
	mod := context.Mods[0]
	if utf8.RuneCountInString(mod.UniqueID) != maxPlayerModUniqueIDChars ||
		utf8.RuneCountInString(mod.Name) != maxPlayerModNameChars ||
		utf8.RuneCountInString(mod.Version) != maxPlayerModVersionChars ||
		strings.ContainsAny(mod.Name+mod.Version, "\r\n\x00") {
		t.Fatalf("untrusted fields were not bounded and stripped: %+v", mod)
	}
	if utf8.RuneCountInString(context.GameVersion) != maxPlayerModVersionChars {
		t.Fatalf("game version was not bounded: %q", context.GameVersion)
	}

	excessive := make([]PlayerReportedMod, maxPlayerReportedMods+1)
	for index := range excessive {
		excessive[index] = PlayerReportedMod{UniqueID: fmt.Sprintf("Example.Excess.%d", index)}
	}
	writePlayerModContexts(t, dataDir, controlPlayerModContextsFile{
		SchemaVersion: playerModContextsSchemaVersion,
		UpdatedAt:     now,
		Players: map[string]controlPlayerModContext{
			"100": {
				UniqueMultiplayerID: "100", HasSmapi: true, APIVersion: "4.5.2", Mods: excessive,
				ContextStatus: PlayerModContextReported, ReportedAt: &now, UpdatedAt: now,
			},
		},
	})
	if _, found, reason := readPlayerModContext(dataDir, "100"); found || reason != playerModUnavailableFileInvalid {
		t.Fatalf("excessive context = found %v reason %q, want invalid", found, reason)
	}
}

func TestPlayerModPeerEventHandlersStayReadOnly(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("embedded", "smapi-mod-src", "ModEntry.cs"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "private void OnPeerContextReceived")
	end := strings.Index(text, "private void OnGameLaunched")
	if start < 0 || end <= start {
		t.Fatal("could not locate the player Mod peer event handler block")
	}
	handlers := text[start:end]
	for _, forbidden := range []string{"Game1.server", ".kick(", ".ban(", "commandDir", "PlayerCommand"} {
		if strings.Contains(handlers, forbidden) {
			t.Fatalf("player Mod peer handlers contain forbidden management path %q", forbidden)
		}
	}
	for _, required := range []string{
		"PlayerModContextLifecycle.Report",
		"PlayerModContextLifecycle.Connect",
		"PlayerModContextLifecycle.Disconnect",
		"WritePlayerModContexts",
	} {
		if !strings.Contains(handlers, required) {
			t.Fatalf("player Mod peer handlers no longer contain required read-only path %q", required)
		}
	}
}

func writePlayerModManifest(t *testing.T, dataDir, folder, uniqueID, name, version string) {
	t.Helper()
	dir := filepath.Join(modsDir(dataDir), folder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]string{
		"Name": name, "UniqueID": uniqueID, "Version": version, "Author": "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writePlayerModOptions(t *testing.T, dataDir string, generatedAt time.Time, gameVersion, apiVersion string, mods []runtimeLoadedMod) {
	t.Helper()
	path := runtimeOptionsPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(playerModRuntimeOptions{
		SchemaVersion: runtimeFarmCatalogSchemaVersion,
		Source:        "smapi-runtime",
		GeneratedAt:   generatedAt,
		GameVersion:   gameVersion,
		APIVersion:    apiVersion,
		LoadedMods:    mods,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writePlayerModContexts(t *testing.T, dataDir string, value controlPlayerModContextsFile) {
	t.Helper()
	path := playerModContextsPath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
