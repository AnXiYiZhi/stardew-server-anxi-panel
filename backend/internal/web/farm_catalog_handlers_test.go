package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestFarmTypeCatalogNoModsReturnsEightBuiltins(t *testing.T) {
	handler, _, adminCookie, cleanup := newFarmCatalogTestHandler(t, nil)
	defer cleanup()

	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/saves/farm-types", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("catalog returned %d: %s", response.Code, response.Body.String())
	}
	result := decodeFarmCatalogResponse(t, response.Body.Bytes())
	if len(result.FarmTypes) != 8 {
		t.Fatalf("farmTypes = %+v, want eight builtins", result.FarmTypes)
	}
	for _, farm := range result.FarmTypes {
		if farm.Kind != "builtin" || !farm.Selectable || farm.RequiresRuntimeValidation {
			t.Fatalf("builtin farm = %+v", farm)
		}
	}
}

func TestFarmTypeCatalogSyntheticFrontierAndIconSecurity(t *testing.T) {
	handler, store, adminCookie, cleanup := newFarmCatalogTestHandler(t, nil)
	defer cleanup()
	instance := farmCatalogTestInstance(t, store)
	modRoot := writeFarmCatalogWebFixture(t, instance.DataDir, true, "[CP] Frontier Farm", "FlashShifter.FrontierFarm", "FrontierFarm", "边境农场_一大片土地", true)
	writeWebModManifest(t, instance.DataDir, true, "Content Patcher", map[string]any{
		"Name": "Content Patcher", "UniqueID": "Pathoschild.ContentPatcher", "Version": "2.0.0",
	})

	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/saves/farm-types", nil, adminCookie)
	result := decodeFarmCatalogResponse(t, response.Body.Bytes())
	frontier := findFarmCatalogItem(t, result, "FrontierFarm")
	if frontier.Label != "边境农场" || frontier.Kind != "modded" || frontier.Selectable || !frontier.RequiresRuntimeValidation || !frontier.Enabled {
		t.Fatalf("FrontierFarm = %+v", frontier)
	}
	if frontier.ProviderFolder != "[CP] Frontier Farm" || frontier.IconURL == "" {
		t.Fatalf("provider/icon = %+v", frontier)
	}
	if frontier.DependenciesReady == nil || !*frontier.DependenciesReady || frontier.ModSelection == nil || frontier.ModSelection.Readiness != sj.FarmDependencyReady {
		t.Fatalf("dependency selection = %+v", frontier)
	}

	iconResponse, _ := doJSON(t, handler, http.MethodGet, frontier.IconURL, nil, adminCookie)
	if iconResponse.Code != http.StatusOK || iconResponse.Header().Get("Content-Type") != "image/png" || iconResponse.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("icon response = %d headers=%v body=%s", iconResponse.Code, iconResponse.Header(), iconResponse.Body.String())
	}
	if iconResponse.Body.Len() == 0 || iconResponse.Body.Len() > int(2*1024*1024) {
		t.Fatalf("icon size = %d", iconResponse.Body.Len())
	}

	for _, attack := range []string{
		"/api/instances/stardew/saves/farm-types/..%252fmanifest/icon",
		"/api/instances/stardew/saves/farm-types/C:%255cWindows/icon",
		"/api/instances/stardew/saves/farm-types/FrontierFarm/icon/../../manifest.json",
	} {
		blocked, _ := doJSON(t, handler, http.MethodGet, attack, nil, adminCookie)
		if blocked.Code != http.StatusNotFound {
			t.Fatalf("attack %q returned %d: %s", attack, blocked.Code, blocked.Body.String())
		}
	}

	if err := os.Remove(filepath.Join(modRoot, "Assets", "Icon.png")); err != nil {
		t.Fatal(err)
	}
	deleted, _ := doJSON(t, handler, http.MethodGet, frontier.IconURL, nil, adminCookie)
	if deleted.Code != http.StatusNotFound || bytes.Contains(deleted.Body.Bytes(), []byte(instance.DataDir)) {
		t.Fatalf("deleted icon = %d %s", deleted.Code, deleted.Body.String())
	}
}

func TestModdedFarmCreationFeatureFlagAllowsReadyExplicitFarmType(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{Addr: ":0", DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel.db"), Secret: "test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	capture := &capturingNewGameDriver{}
	drivers := registry.New()
	if err := drivers.Register(capture); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(Deps{Config: config.Config{DataDir: dataDir, Secret: "test-secret", EnableModdedFarmCreation: true}, Store: store, Registry: drivers})
	setup, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username": "admin", "password": "admin-password", "confirmPassword": "admin-password",
	}, nil)
	if setup.Code != http.StatusOK {
		t.Fatalf("setup = %d: %s", setup.Code, setup.Body.String())
	}
	instance := farmCatalogTestInstance(t, store)
	writeFarmCatalogWebFixture(t, instance.DataDir, true, "[CP] Frontier Farm", "flashshifter.FrontierFarm", "FrontierFarm", "边境农场_说明", false)
	writeWebModManifest(t, instance.DataDir, true, "Content Patcher", map[string]any{
		"Name": "Content Patcher", "UniqueID": "Pathoschild.ContentPatcher", "Version": "2.7.0",
	})

	catalogResponse, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/saves/farm-types", nil, adminCookie)
	if catalogResponse.Code != http.StatusOK {
		t.Fatalf("catalog = %d: %s", catalogResponse.Code, catalogResponse.Body.String())
	}
	catalog := decodeFarmCatalogResponse(t, catalogResponse.Body.Bytes())
	frontier := findFarmCatalogItem(t, catalog, "FrontierFarm")
	if !catalog.ModdedCreationEnabled || !frontier.Selectable || frontier.DependenciesReady == nil || !*frontier.DependenciesReady {
		t.Fatalf("feature-enabled FrontierFarm = %+v; response=%+v", frontier, catalog)
	}

	create, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/custom-new-game", map[string]any{
		"farmName": "Frontier Test", "farmType": "FrontierFarm",
	}, adminCookie)
	if create.Code != http.StatusAccepted {
		t.Fatalf("create = %d: %s", create.Code, create.Body.String())
	}
	if capture.request.NewGameConfig == nil || capture.request.NewGameConfig.FarmType != "FrontierFarm" {
		t.Fatalf("explicit FarmType not passed to lifecycle: %+v", capture.request.NewGameConfig)
	}

	disabledRoot := filepath.Join(instance.DataDir, ".local-container", "mods-disabled")
	if err := os.MkdirAll(disabledRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(
		filepath.Join(instance.DataDir, ".local-container", "mods", "Content Patcher"),
		filepath.Join(disabledRoot, "Content Patcher"),
	); err != nil {
		t.Fatal(err)
	}
	blocked, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/custom-new-game", map[string]any{
		"farmName": "Blocked Frontier", "farmType": "FrontierFarm",
	}, adminCookie)
	if blocked.Code != http.StatusConflict || !strings.Contains(blocked.Body.String(), "farm_dependencies_missing") {
		t.Fatalf("disabled dependency create = %d: %s", blocked.Code, blocked.Body.String())
	}
}

func TestFarmTypeCatalogDisabledConflictAndPartialDamage(t *testing.T) {
	handler, store, adminCookie, cleanup := newFarmCatalogTestHandler(t, nil)
	defer cleanup()
	instance := farmCatalogTestInstance(t, store)
	writeFarmCatalogWebFixture(t, instance.DataDir, false, "Disabled Farm", "Author.Disabled", "DisabledFarm", "禁用农场_说明", false)
	writeFarmCatalogWebFixture(t, instance.DataDir, true, "Conflict A", "Author.A", "SharedFarm", "冲突甲_说明", false)
	writeFarmCatalogWebFixture(t, instance.DataDir, true, "Conflict B", "Author.B", "SharedFarm", "冲突乙_说明", false)
	brokenRoot := filepath.Join(instance.DataDir, ".local-container", "mods", "Broken")
	if err := os.MkdirAll(brokenRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(brokenRoot, "manifest.json"), []byte("broken JSON"), 0o644); err != nil {
		t.Fatal(err)
	}

	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/saves/farm-types", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("catalog returned %d: %s", response.Code, response.Body.String())
	}
	result := decodeFarmCatalogResponse(t, response.Body.Bytes())
	if len(result.FarmTypes) != 11 || len(result.CatalogWarnings) == 0 {
		t.Fatalf("partial result = farms %d warnings %v", len(result.FarmTypes), result.CatalogWarnings)
	}
	disabled := findFarmCatalogItem(t, result, "DisabledFarm")
	if disabled.Enabled || disabled.Selectable {
		t.Fatalf("disabled farm = %+v", disabled)
	}
	sharedCount := 0
	for _, farm := range result.FarmTypes {
		if farm.ID == "SharedFarm" {
			sharedCount++
			if !farm.Conflict || farm.Selectable || farm.IconURL != "" {
				t.Fatalf("conflict farm = %+v", farm)
			}
		}
	}
	if sharedCount != 2 {
		t.Fatalf("shared farm count = %d", sharedCount)
	}
}

func TestFarmTypeCatalogScanFailureStillReturnsBuiltins(t *testing.T) {
	scanner := func(string) (sj.FarmCatalogResult, error) {
		return sj.FarmCatalogResult{}, errors.New("synthetic scanner failure with C:\\private\\path")
	}
	handler, _, adminCookie, cleanup := newFarmCatalogTestHandler(t, scanner)
	defer cleanup()
	response, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/saves/farm-types", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("catalog returned %d: %s", response.Code, response.Body.String())
	}
	result := decodeFarmCatalogResponse(t, response.Body.Bytes())
	if len(result.FarmTypes) != 8 || len(result.CatalogWarnings) != 1 || bytes.Contains(response.Body.Bytes(), []byte("private")) {
		t.Fatalf("scan failure response = %s", response.Body.String())
	}
}

func TestFarmTypeCatalogRequiresAdminAndCustomNewGameStillRejectsModded(t *testing.T) {
	handler, _, adminCookie, cleanup := newFarmCatalogTestHandler(t, nil)
	defer cleanup()
	unauthorized, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/saves/farm-types", nil, nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized = %d", unauthorized.Code)
	}
	created, _ := doJSON(t, handler, http.MethodPost, "/api/users", map[string]string{"username": "player", "password": "player-password", "role": "user"}, adminCookie)
	if created.Code != http.StatusCreated {
		t.Fatalf("create player = %d: %s", created.Code, created.Body.String())
	}
	_, playerCookie := doJSON(t, handler, http.MethodPost, "/api/auth/login", map[string]string{"username": "player", "password": "player-password"}, nil)
	forbidden, _ := doJSON(t, handler, http.MethodGet, "/api/instances/stardew/saves/farm-types", nil, playerCookie)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("ordinary user = %d", forbidden.Code)
	}

	create, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/custom-new-game", map[string]any{
		"farmName": "Blocked", "farmType": "FrontierFarm", "startingCabins": 0, "maxPlayers": 10,
		"cabinLayout": "nearby", "profitMargin": "100", "petBreed": 0, "moneyMode": "shared",
	}, adminCookie)
	if create.Code != http.StatusConflict || !bytes.Contains(create.Body.Bytes(), []byte("modded_farm_creation_disabled")) {
		t.Fatalf("modded create = %d: %s", create.Code, create.Body.String())
	}
}

func TestPrepareFarmTypeEnablesRequiredModsWithoutCreatingSave(t *testing.T) {
	handler, store, adminCookie, cleanup := newFarmCatalogTestHandler(t, nil)
	defer cleanup()
	instance := farmCatalogTestInstance(t, store)
	writeFarmCatalogWebFixture(t, instance.DataDir, false, "[CP] Frontier Farm", "flashshifter.FrontierFarm", "FrontierFarm", "边境农场_说明", false)
	writeWebModManifest(t, instance.DataDir, false, "Content Patcher", map[string]any{
		"Name": "Content Patcher", "UniqueID": "Pathoschild.ContentPatcher", "Version": "2.0.0",
	})

	response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/farm-types/prepare", map[string]string{"farmTypeId": "FrontierFarm"}, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("prepare = %d: %s", response.Code, response.Body.String())
	}
	var selection sj.NewGameModSelection
	if err := json.Unmarshal(response.Body.Bytes(), &selection); err != nil {
		t.Fatal(err)
	}
	if !selection.DependenciesReady || selection.Readiness != sj.FarmDependencyReady || len(selection.ChangedModKeys) != 2 {
		t.Fatalf("selection = %+v", selection)
	}
	for _, folder := range []string{"[CP] Frontier Farm", "Content Patcher"} {
		if _, err := os.Stat(filepath.Join(instance.DataDir, ".local-container", "mods", folder)); err != nil {
			t.Fatalf("%s not enabled: %v", folder, err)
		}
	}
	if _, err := os.Stat(filepath.Join(instance.DataDir, ".local-container", "control", "mod-profiles.json")); !os.IsNotExist(err) {
		t.Fatalf("prepare created permanent save profile: %v", err)
	}
}

func TestPrepareFarmTypeRejectsRunningBusyUnknownConflictAndInjectedBody(t *testing.T) {
	t.Run("running", func(t *testing.T) {
		handler, store, adminCookie, cleanup := newFarmCatalogTestHandler(t, nil)
		defer cleanup()
		instance := farmCatalogTestInstance(t, store)
		if _, err := store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{ID: instance.ID, State: storage.InstanceStateRunning, StateMessage: "running", DriverPhase: "running"}); err != nil {
			t.Fatal(err)
		}
		response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/farm-types/prepare", map[string]string{"farmTypeId": "FrontierFarm"}, adminCookie)
		if response.Code != http.StatusConflict || !bytes.Contains(response.Body.Bytes(), []byte("server_running")) {
			t.Fatalf("running = %d: %s", response.Code, response.Body.String())
		}
	})

	t.Run("lifecycle busy", func(t *testing.T) {
		handler, store, adminCookie, cleanup := newFarmCatalogTestHandler(t, nil)
		defer cleanup()
		job, err := store.CreateJob(context.Background(), storage.CreateJobParams{Type: "stardew_lifecycle", TargetType: "instance", TargetID: storage.DefaultInstanceID})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.StartJob(context.Background(), job.ID); err != nil {
			t.Fatal(err)
		}
		response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/farm-types/prepare", map[string]string{"farmTypeId": "FrontierFarm"}, adminCookie)
		if response.Code != http.StatusConflict || !bytes.Contains(response.Body.Bytes(), []byte("instance_busy")) {
			t.Fatalf("busy = %d: %s", response.Code, response.Body.String())
		}
	})

	t.Run("unknown", func(t *testing.T) {
		handler, _, adminCookie, cleanup := newFarmCatalogTestHandler(t, nil)
		defer cleanup()
		response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/farm-types/prepare", map[string]string{"farmTypeId": "UnknownFarm"}, adminCookie)
		if response.Code != http.StatusNotFound || !bytes.Contains(response.Body.Bytes(), []byte("farm_type_not_found")) {
			t.Fatalf("unknown = %d: %s", response.Code, response.Body.String())
		}
	})

	t.Run("conflict", func(t *testing.T) {
		handler, store, adminCookie, cleanup := newFarmCatalogTestHandler(t, nil)
		defer cleanup()
		instance := farmCatalogTestInstance(t, store)
		writeFarmCatalogWebFixture(t, instance.DataDir, true, "Farm A", "Author.A", "SharedFarm", "甲_说明", false)
		writeFarmCatalogWebFixture(t, instance.DataDir, true, "Farm B", "Author.B", "SharedFarm", "乙_说明", false)
		response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/farm-types/prepare", map[string]string{"farmTypeId": "SharedFarm"}, adminCookie)
		if response.Code != http.StatusConflict || !bytes.Contains(response.Body.Bytes(), []byte("farm_type_conflict")) {
			t.Fatalf("conflict = %d: %s", response.Code, response.Body.String())
		}
	})

	t.Run("body injection", func(t *testing.T) {
		handler, _, adminCookie, cleanup := newFarmCatalogTestHandler(t, nil)
		defer cleanup()
		response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/saves/farm-types/prepare", map[string]any{"farmTypeId": "FrontierFarm", "folders": []string{"C:\\private"}}, adminCookie)
		if response.Code != http.StatusBadRequest || !bytes.Contains(response.Body.Bytes(), []byte("invalid_json")) {
			t.Fatalf("injected body = %d: %s", response.Code, response.Body.String())
		}
	})
}

func newFarmCatalogTestHandler(t *testing.T, scanner func(string) (sj.FarmCatalogResult, error)) (http.Handler, *storage.Store, *http.Cookie, func()) {
	t.Helper()
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{Addr: ":0", DataDir: dataDir, DBPath: filepath.Join(dataDir, "panel.db"), Secret: "test-secret", Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	handler := NewHandler(Deps{Config: config.Config{DataDir: dataDir, Secret: "test-secret", Version: "test"}, Store: store, FarmCatalogScanner: scanner})
	setup, adminCookie := doJSON(t, handler, http.MethodPost, "/api/setup/admin", map[string]string{
		"username": "admin", "password": "admin-password", "confirmPassword": "admin-password",
	}, nil)
	if setup.Code != http.StatusOK {
		_ = store.Close()
		t.Fatalf("setup admin = %d: %s", setup.Code, setup.Body.String())
	}
	return handler, store, adminCookie, func() { _ = store.Close() }
}

func farmCatalogTestInstance(t *testing.T, store *storage.Store) storage.Instance {
	t.Helper()
	instance, err := store.GetInstance(context.Background(), storage.DefaultInstanceID)
	if err != nil {
		t.Fatal(err)
	}
	return instance
}

func writeFarmCatalogWebFixture(t *testing.T, dataDir string, enabled bool, folder, providerID, farmID, i18nValue string, withIcon bool) string {
	t.Helper()
	rootName := "mods-disabled"
	if enabled {
		rootName = "mods"
	}
	root := filepath.Join(dataDir, ".local-container", rootName, folder)
	if err := os.MkdirAll(filepath.Join(root, "i18n"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{"Name": folder, "UniqueID": providerID, "Version": "1.0.0", "ContentPackFor": map[string]string{"UniqueID": "Pathoschild.ContentPatcher"}}
	manifestData, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), manifestData, 0o644); err != nil {
		t.Fatal(err)
	}
	entry := map[string]any{"ID": farmID, "TooltipStringPath": "Strings/UI:" + farmID}
	changes := []any{
		map[string]any{"Action": "EditData", "Target": "Data/AdditionalFarms", "Entries": map[string]any{"Provider/" + farmID: entry}},
		map[string]any{"Action": "EditData", "Target": "Strings/UI", "Entries": map[string]string{farmID: "{{i18n:" + farmID + ".Description}}"}},
	}
	if withIcon {
		entry["IconTexture"] = "Mods/Fixture/Icon"
		changes = append(changes, map[string]any{"Action": "Load", "Target": "Mods/Fixture/Icon", "FromFile": "Assets/Icon.png"})
	}
	contentData, _ := json.Marshal(map[string]any{"Changes": changes})
	if err := os.WriteFile(filepath.Join(root, "content.json"), contentData, 0o644); err != nil {
		t.Fatal(err)
	}
	i18nData, _ := json.Marshal(map[string]string{farmID + ".Description": i18nValue})
	if err := os.WriteFile(filepath.Join(root, "i18n", "zh.json"), i18nData, 0o644); err != nil {
		t.Fatal(err)
	}
	if withIcon {
		img := image.NewRGBA(image.Rect(0, 0, 4, 3))
		img.Set(0, 0, color.RGBA{R: 20, G: 120, B: 50, A: 255})
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, img); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "Assets"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "Assets", "Icon.png"), encoded.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func writeWebModManifest(t *testing.T, dataDir string, enabled bool, folder string, manifest map[string]any) {
	t.Helper()
	rootName := "mods-disabled"
	if enabled {
		rootName = "mods"
	}
	root := filepath.Join(dataDir, ".local-container", rootName, folder)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func decodeFarmCatalogResponse(t *testing.T, data []byte) farmTypeCatalogResponse {
	t.Helper()
	var response farmTypeCatalogResponse
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("decode catalog: %v; body=%s", err, data)
	}
	return response
}

func findFarmCatalogItem(t *testing.T, response farmTypeCatalogResponse, id string) farmTypeCatalogItem {
	t.Helper()
	for _, farm := range response.FarmTypes {
		if farm.ID == id {
			return farm
		}
	}
	t.Fatalf("farm %q not found in %+v", id, response.FarmTypes)
	return farmTypeCatalogItem{}
}
