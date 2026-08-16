package stardew_junimo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

func TestCheckModUpdatesUsesCacheAndInvalidatesForManifestChange(t *testing.T) {
	dataDir := t.TempDir()
	createTestModWithManifest(t, modsDir(dataDir), "ContentPatcher", modManifest{
		Name:       "Content Patcher",
		UniqueID:   "Pathoschild.ContentPatcher",
		Version:    "2.4.0",
		Author:     "Pathoschild",
		UpdateKeys: []string{"Nexus:1915"},
	})
	createTestMod(t, modsDir(dataDir), "LocalOnly", "Example.LocalOnly", "Local Only")

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		var body smapiModUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(body.Mods) != 1 || body.Mods[0].ID != "Pathoschild.ContentPatcher" {
			t.Fatalf("request mods = %+v", body.Mods)
		}
		if body.APIVersion != modUpdateFallbackAPIVersion || body.GameVersion != "" || body.Platform != modUpdatePlatform {
			t.Fatalf("request runtime context = api %q game %q platform %q", body.APIVersion, body.GameVersion, body.Platform)
		}
		_ = json.NewEncoder(w).Encode([]smapiModUpdateResponseItem{{
			ID: "Pathoschild.ContentPatcher",
			SuggestedUpdate: &smapiSuggestedModUpdate{
				Version: "2.5.0",
				URL:     "https://www.nexusmods.com/stardewvalley/mods/1915",
			},
		}})
	}))
	defer server.Close()
	restoreModUpdateTestGlobals(t, server)

	driver := New(nil, nil, nil, nil)
	instance := registry.Instance{ID: "stardew", DataDir: dataDir}
	first, err := driver.CheckModUpdates(context.Background(), instance, false)
	if err != nil {
		t.Fatalf("first CheckModUpdates: %v", err)
	}
	if first.Status != "ok" || first.Cached || len(first.Updates) != 1 {
		t.Fatalf("first result = %+v", first)
	}
	if first.EligibleCount != 1 || first.SkippedCount != 1 {
		t.Fatalf("first counts = eligible %d skipped %d", first.EligibleCount, first.SkippedCount)
	}

	second, err := driver.CheckModUpdates(context.Background(), instance, false)
	if err != nil {
		t.Fatalf("second CheckModUpdates: %v", err)
	}
	if !second.Cached || requests.Load() != 1 {
		t.Fatalf("second cached = %v, requests = %d", second.Cached, requests.Load())
	}

	createTestModWithManifest(t, modsDir(dataDir), "ContentPatcher", modManifest{
		Name:       "Content Patcher",
		UniqueID:   "Pathoschild.ContentPatcher",
		Version:    "2.4.1",
		Author:     "Pathoschild",
		UpdateKeys: []string{"Nexus:1915"},
	})
	third, err := driver.CheckModUpdates(context.Background(), instance, false)
	if err != nil {
		t.Fatalf("third CheckModUpdates: %v", err)
	}
	if third.Cached || requests.Load() != 2 {
		t.Fatalf("third cached = %v, requests = %d", third.Cached, requests.Load())
	}
	if third.Updates[0].CurrentVersion != "2.4.1" {
		t.Fatalf("third current version = %q", third.Updates[0].CurrentVersion)
	}
	if _, err := os.Stat(modUpdateCachePath(dataDir)); err != nil {
		t.Fatalf("cache file: %v", err)
	}
}

func TestCheckModUpdatesUsesRuntimeVersionsAndInvalidatesTheirCache(t *testing.T) {
	dataDir := t.TempDir()
	createTestModWithManifest(t, modsDir(dataDir), "ContentPatcher", modManifest{
		Name:       "Content Patcher",
		UniqueID:   "Pathoschild.ContentPatcher",
		Version:    "2.4.0",
		UpdateKeys: []string{"Nexus:1915"},
	})
	optionsPath := filepath.Join(controlDir(dataDir), "options.json")
	if err := os.MkdirAll(filepath.Dir(optionsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeOptions := func(apiVersion, gameVersion string) {
		t.Helper()
		body, err := json.Marshal(map[string]string{"apiVersion": apiVersion, "gameVersion": gameVersion})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(optionsPath, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeOptions("4.1.0", "1.6.14")

	var requests []smapiModUpdateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body smapiModUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requests = append(requests, body)
		_ = json.NewEncoder(w).Encode([]smapiModUpdateResponseItem{})
	}))
	defer server.Close()
	restoreModUpdateTestGlobals(t, server)

	driver := New(nil, nil, nil, nil)
	instance := registry.Instance{ID: "stardew", DataDir: dataDir}
	if _, err := driver.CheckModUpdates(context.Background(), instance, false); err != nil {
		t.Fatal(err)
	}
	writeOptions("4.2.0", "1.6.15")
	if _, err := driver.CheckModUpdates(context.Background(), instance, false); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want cache invalidated by runtime version change", len(requests))
	}
	if requests[0].APIVersion != "4.1.0" || requests[0].GameVersion != "1.6.14" || requests[0].Platform != "Linux" {
		t.Fatalf("first runtime context = %+v", requests[0])
	}
	if requests[1].APIVersion != "4.2.0" || requests[1].GameVersion != "1.6.15" || requests[1].Platform != "Linux" {
		t.Fatalf("second runtime context = %+v", requests[1])
	}
}

func TestCheckModUpdatesKeepsLastSuccessWhenServiceFails(t *testing.T) {
	dataDir := t.TempDir()
	createTestModWithManifest(t, modsDir(dataDir), "TestMod", modManifest{
		Name:       "Test Mod",
		UniqueID:   "Example.TestMod",
		Version:    "1.0.0",
		UpdateKeys: []string{"GitHub:example/test-mod"},
	})

	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if fail.Load() {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode([]smapiModUpdateResponseItem{{
			ID: "Example.TestMod",
			SuggestedUpdate: &smapiSuggestedModUpdate{
				Version: "1.1.0",
				URL:     "https://github.com/example/test-mod/releases",
			},
		}})
	}))
	defer server.Close()
	restoreModUpdateTestGlobals(t, server)

	driver := New(nil, nil, nil, nil)
	instance := registry.Instance{ID: "stardew", DataDir: dataDir}
	first, err := driver.CheckModUpdates(context.Background(), instance, true)
	if err != nil || len(first.Updates) != 1 {
		t.Fatalf("initial result = %+v, err = %v", first, err)
	}
	fail.Store(true)
	failed, err := driver.CheckModUpdates(context.Background(), instance, true)
	if err != nil {
		t.Fatalf("failed check should degrade in-band: %v", err)
	}
	if failed.Status != "error" || !failed.Cached || failed.CheckError == "" || len(failed.Updates) != 1 {
		t.Fatalf("failed result = %+v", failed)
	}
	if failed.CheckedAt != first.CheckedAt {
		t.Fatalf("failed checkedAt = %q, want %q", failed.CheckedAt, first.CheckedAt)
	}
}

func TestCheckModUpdatesFiltersUnsafeSuggestedURL(t *testing.T) {
	dataDir := t.TempDir()
	createTestModWithManifest(t, modsDir(dataDir), "UnsafeMod", modManifest{
		Name:       "Unsafe Mod",
		UniqueID:   "Example.UnsafeMod",
		Version:    "1.0.0",
		UpdateKeys: []string{"Nexus:123"},
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]smapiModUpdateResponseItem{{
			ID: "Example.UnsafeMod",
			SuggestedUpdate: &smapiSuggestedModUpdate{
				Version: "2.0.0",
				URL:     "javascript:alert(1)",
			},
		}})
	}))
	defer server.Close()
	restoreModUpdateTestGlobals(t, server)

	result, err := New(nil, nil, nil, nil).CheckModUpdates(context.Background(), registry.Instance{DataDir: dataDir}, true)
	if err != nil {
		t.Fatalf("CheckModUpdates: %v", err)
	}
	if result.Status != "ok" || len(result.Updates) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestCheckModUpdatesSkipsNetworkWithoutEligibleMods(t *testing.T) {
	dataDir := t.TempDir()
	createTestMod(t, modsDir(dataDir), "LocalOnly", "Example.LocalOnly", "Local Only")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
	defer server.Close()
	restoreModUpdateTestGlobals(t, server)

	result, err := New(nil, nil, nil, nil).CheckModUpdates(context.Background(), registry.Instance{DataDir: dataDir}, false)
	if err != nil {
		t.Fatalf("CheckModUpdates: %v", err)
	}
	if result.Status != "ok" || result.EligibleCount != 0 || result.SkippedCount != 1 || requests.Load() != 0 {
		t.Fatalf("result = %+v, requests = %d", result, requests.Load())
	}
}

func restoreModUpdateTestGlobals(t *testing.T, server *httptest.Server) {
	t.Helper()
	previousURL := modUpdateServiceURL
	previousClient := modUpdateHTTPClient
	previousNow := modUpdateNow
	modUpdateServiceURL = server.URL
	modUpdateHTTPClient = server.Client()
	modUpdateNow = func() time.Time {
		return time.Date(2026, time.August, 16, 6, 0, 0, 0, time.UTC)
	}
	t.Cleanup(func() {
		modUpdateServiceURL = previousURL
		modUpdateHTTPClient = previousClient
		modUpdateNow = previousNow
	})
}
