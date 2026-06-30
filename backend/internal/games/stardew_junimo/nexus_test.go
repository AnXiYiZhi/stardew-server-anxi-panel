package stardew_junimo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withNexusEndpoints points the package-level Nexus base URLs at a test
// server for the duration of the test, restoring the originals afterward.
func withNexusEndpoints(t *testing.T, server *httptest.Server) {
	t.Helper()
	origV1 := nexusV1BaseURL
	origGraphQL := nexusGraphQLURL
	nexusV1BaseURL = server.URL + "/v1"
	nexusGraphQLURL = server.URL + "/v2/graphql"
	t.Cleanup(func() {
		nexusV1BaseURL = origV1
		nexusGraphQLURL = origGraphQL
		server.Close()
	})
}

// ── API key / query validation ──────────────────────────────────────────────

func TestSearchNexusMods_MissingAPIKey(t *testing.T) {
	t.Setenv("NEXUS_API_KEY", "")
	_, err := SearchNexusMods(context.Background(), "stardew valley expanded")
	if err != ErrNexusAPIKeyMissing {
		t.Fatalf("err = %v, want ErrNexusAPIKeyMissing", err)
	}
}

func TestSearchNexusMods_EmptyQuery(t *testing.T) {
	t.Setenv("NEXUS_API_KEY", "fake-key-123")
	_, err := SearchNexusMods(context.Background(), "   ")
	if err != ErrInvalidNexusQuery {
		t.Fatalf("err = %v, want ErrInvalidNexusQuery", err)
	}
}

// ── result parsing ───────────────────────────────────────────────────────────

func TestSearchNexusMods_IDLookupParsesResult(t *testing.T) {
	t.Setenv("NEXUS_API_KEY", "fake-key-123")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/games/stardewvalley/mods/2400.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mod_id":            2400,
			"name":              "Stardew Valley Expanded",
			"summary":           "A massive expansion mod",
			"version":           "1.13.6",
			"author":            "FlashShifter",
			"picture_url":       "https://example.com/pic.png",
			"mod_downloads":     1000000,
			"endorsement_count": 50000,
			"updated_time":      "2026-01-01T00:00:00.000Z",
		})
	}))
	withNexusEndpoints(t, server)

	resp, err := SearchNexusMods(context.Background(), "2400")
	if err != nil {
		t.Fatalf("SearchNexusMods: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(resp.Results))
	}
	got := resp.Results[0]
	if got.ModID != 2400 || got.Name != "Stardew Valley Expanded" || got.Author != "FlashShifter" {
		t.Errorf("unexpected result: %+v", got)
	}
	if got.EndorsementCount != 50000 || got.DownloadCount != 1000000 {
		t.Errorf("unexpected counts: %+v", got)
	}
	wantURL := "https://www.nexusmods.com/stardewvalley/mods/2400"
	if got.NexusURL != wantURL {
		t.Errorf("NexusURL = %q, want %q", got.NexusURL, wantURL)
	}
}

func TestSearchNexusMods_KeywordSearchParsesResult(t *testing.T) {
	t.Setenv("NEXUS_API_KEY", "fake-key-123")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/graphql" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"mods": map[string]any{
					"nodes": []map[string]any{
						{"modId": 1, "name": "Mod One", "downloads": 10, "endorsements": 1},
						{"modId": 2, "name": "Mod Two", "downloads": 20, "endorsements": 2},
					},
				},
			},
		})
	}))
	withNexusEndpoints(t, server)

	resp, err := SearchNexusMods(context.Background(), "farming")
	if err != nil {
		t.Fatalf("SearchNexusMods: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(resp.Results))
	}
	if resp.Results[0].Name != "Mod One" || resp.Results[1].ModID != 2 {
		t.Errorf("unexpected results: %+v", resp.Results)
	}
	if resp.Query != "farming" {
		t.Errorf("Query = %q, want %q", resp.Query, "farming")
	}
}

func TestSearchNexusMods_ResultsCappedAtMax(t *testing.T) {
	t.Setenv("NEXUS_API_KEY", "fake-key-123")
	nodes := make([]map[string]any, 30)
	for i := range nodes {
		nodes[i] = map[string]any{"modId": i + 1, "name": "Mod"}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"mods": map[string]any{"nodes": nodes}},
		})
	}))
	withNexusEndpoints(t, server)

	resp, err := SearchNexusMods(context.Background(), "mod")
	if err != nil {
		t.Fatalf("SearchNexusMods: %v", err)
	}
	if len(resp.Results) != nexusMaxResults {
		t.Fatalf("len(results) = %d, want %d", len(resp.Results), nexusMaxResults)
	}
}

// ── non-2xx error mapping ────────────────────────────────────────────────────

func TestSearchNexusMods_NonOKStatusMapsToNexusAPIError(t *testing.T) {
	t.Setenv("NEXUS_API_KEY", "fake-key-123")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	withNexusEndpoints(t, server)

	_, err := SearchNexusMods(context.Background(), "999999")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*NexusAPIError)
	if !ok {
		t.Fatalf("err = %T(%v), want *NexusAPIError", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}

func TestSearchNexusMods_NonOKStatusForKeywordSearch(t *testing.T) {
	t.Setenv("NEXUS_API_KEY", "fake-key-123")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	withNexusEndpoints(t, server)

	_, err := SearchNexusMods(context.Background(), "farming")
	apiErr, ok := err.(*NexusAPIError)
	if !ok {
		t.Fatalf("err = %T(%v), want *NexusAPIError", err, err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusInternalServerError)
	}
}

// ── API key never leaks ─────────────────────────────────────────────────────

func TestSearchNexusMods_DoesNotLeakAPIKey(t *testing.T) {
	const secretKey = "super-secret-nexus-key-do-not-leak"
	t.Setenv("NEXUS_API_KEY", secretKey)

	var receivedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("apikey")
		// Simulate a buggy/hostile upstream that echoes request details in
		// its error body — our client must never surface this to the caller.
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid apikey: " + secretKey))
	}))
	withNexusEndpoints(t, server)

	_, err := SearchNexusMods(context.Background(), "1234")
	if err == nil {
		t.Fatal("expected error")
	}
	if receivedHeader != secretKey {
		t.Fatalf("server did not receive apikey header, got %q", receivedHeader)
	}
	if strings.Contains(err.Error(), secretKey) {
		t.Fatalf("error message leaked API key: %v", err)
	}
}

// ── installed matching ───────────────────────────────────────────────────────

func writeManifestWithUpdateKeys(t *testing.T, modsRoot, folderName, uniqueID, version string, updateKeys []string) {
	t.Helper()
	modPath := filepath.Join(modsRoot, folderName)
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatal(err)
	}
	m := modManifest{
		Name:       folderName,
		UniqueID:   uniqueID,
		Version:    version,
		Author:     "Test",
		UpdateKeys: updateKeys,
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modPath, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestApplyNexusInstalledMatch_MatchesByNexusID(t *testing.T) {
	dir := t.TempDir()
	modsRoot := modsDir(dir)
	writeManifestWithUpdateKeys(t, modsRoot, "SVE", "FlashShifter.SVE", "1.13.6", []string{"Nexus:2400"})
	writeManifestWithUpdateKeys(t, modsRoot, "OtherMod", "Some.OtherMod", "0.1.0", []string{"Chucklefish:1234"})

	results := []NexusModSearchResult{
		{ModID: 2400, Name: "Stardew Valley Expanded"},
		{ModID: 9999, Name: "Not Installed"},
	}
	results = ApplyNexusInstalledMatch(dir, results)

	if !results[0].Installed {
		t.Errorf("result[0].Installed = false, want true")
	}
	if results[0].InstalledFolderName != "SVE" {
		t.Errorf("InstalledFolderName = %q, want SVE", results[0].InstalledFolderName)
	}
	if results[0].InstalledVersion != "1.13.6" {
		t.Errorf("InstalledVersion = %q, want 1.13.6", results[0].InstalledVersion)
	}
	if results[1].Installed {
		t.Errorf("result[1].Installed = true, want false (no matching Nexus:ID installed)")
	}
}

func TestApplyNexusInstalledMatch_NoMods(t *testing.T) {
	dir := t.TempDir()
	results := []NexusModSearchResult{{ModID: 1, Name: "X"}}
	got := ApplyNexusInstalledMatch(dir, results)
	if got[0].Installed {
		t.Errorf("Installed = true, want false when no mods directory exists")
	}
}

// ── manifest UpdateKeys parsing ──────────────────────────────────────────────

func TestParseNexusModIDFromUpdateKeys(t *testing.T) {
	cases := []struct {
		name   string
		keys   []string
		wantID int
		wantOK bool
	}{
		{"simple", []string{"Nexus:2400"}, 2400, true},
		{"case insensitive site", []string{"nexus:2400"}, 2400, true},
		{"with subkey suffix", []string{"Nexus:2400:extra"}, 2400, true},
		{"mixed with non-nexus keys", []string{"Chucklefish:1", "Nexus:42"}, 42, true},
		{"no nexus key", []string{"Chucklefish:1234"}, 0, false},
		{"empty", nil, 0, false},
		{"non-numeric id", []string{"Nexus:abc"}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := parseNexusModIDFromUpdateKeys(tc.keys)
			if id != tc.wantID || ok != tc.wantOK {
				t.Errorf("parseNexusModIDFromUpdateKeys(%v) = (%d, %v), want (%d, %v)", tc.keys, id, ok, tc.wantID, tc.wantOK)
			}
		})
	}
}

// readModInfo should populate UpdateKeys/NexusModID on registry.ModInfo.
func TestReadModInfo_PopulatesNexusModID(t *testing.T) {
	dir := t.TempDir()
	writeManifestWithUpdateKeys(t, dir, "SVE", "FlashShifter.SVE", "1.13.6", []string{"Nexus:2400"})
	info := readModInfo(filepath.Join(dir, "SVE"), "SVE")
	if info.NexusModID != 2400 {
		t.Errorf("NexusModID = %d, want 2400", info.NexusModID)
	}
	if len(info.UpdateKeys) != 1 || info.UpdateKeys[0] != "Nexus:2400" {
		t.Errorf("UpdateKeys = %v, want [Nexus:2400]", info.UpdateKeys)
	}
}
