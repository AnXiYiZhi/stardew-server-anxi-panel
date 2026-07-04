package stardew_junimo

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

const fakeNexusAPIKey = "fake-key-123"

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

// Numeric input without a key uses the public GraphQL metadata path with an
// exact gameId + modId filter. The v1 REST ID lookup is only used once a
// Nexus API key is configured.
func TestSearchNexusMods_NumericQueryUsesGraphQLIDLookupWithoutAPIKey(t *testing.T) {
	var hitGraphQL bool
	var capturedBody map[string]any
	var headerSet bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/graphql" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		hitGraphQL = true
		headerSet = r.Header.Get("apikey") != ""
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"mods": map[string]any{
					"nodes": []map[string]any{{"modId": 2400, "name": "Mod 2400"}},
				},
			},
		})
	}))
	withNexusEndpoints(t, server)

	resp, err := SearchNexusMods(context.Background(), "2400", "")
	if err != nil {
		t.Fatalf("SearchNexusMods: %v", err)
	}
	if !hitGraphQL {
		t.Fatal("GraphQL endpoint was not called")
	}
	if headerSet {
		t.Fatal("apikey header was sent for no-key GraphQL ID lookup")
	}
	variables, _ := capturedBody["variables"].(map[string]any)
	filter, _ := variables["filter"].(map[string]any)
	gameID, _ := filter["gameId"].([]any)
	modID, _ := filter["modId"].([]any)
	if len(gameID) != 1 || len(modID) != 1 {
		t.Fatalf("filter = %+v, want gameId and modId entries", filter)
	}
	gameEntry, _ := gameID[0].(map[string]any)
	modEntry, _ := modID[0].(map[string]any)
	if gameEntry["value"] != nexusStardewGameID {
		t.Errorf("gameId value = %v, want %q", gameEntry["value"], nexusStardewGameID)
	}
	if modEntry["value"] != "2400" {
		t.Errorf("modId value = %v, want 2400", modEntry["value"])
	}
	if len(resp.Results) != 1 || resp.Results[0].ModID != 2400 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

// Keyword search (GraphQL v2) is a public read query and must work without
// a Nexus API key configured in the panel.
func TestSearchNexusMods_KeywordSearchWorksWithoutAPIKey(t *testing.T) {
	var receivedHeader string
	headerSet := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader, headerSet = r.Header.Get("apikey"), r.Header.Get("apikey") != ""
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"mods": map[string]any{
					"nodes": []map[string]any{{"modId": 1, "name": "Mod One"}},
				},
			},
		})
	}))
	withNexusEndpoints(t, server)

	resp, err := SearchNexusMods(context.Background(), "farming", "")
	if err != nil {
		t.Fatalf("SearchNexusMods: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(resp.Results))
	}
	if headerSet {
		t.Errorf("apikey header sent as %q, want no apikey header when key is unset", receivedHeader)
	}
}

func TestSearchNexusMods_EmptyQuery(t *testing.T) {
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"mods": map[string]any{
					"totalCount": 120,
					"nodes": []map[string]any{
						{"modId": 2400, "name": "SMAPI", "downloads": 1000000},
					},
				},
			},
		})
	}))
	withNexusEndpoints(t, server)

	resp, err := SearchNexusModsPage(context.Background(), "   ", fakeNexusAPIKey, 2, 20)
	if err != nil {
		t.Fatalf("SearchNexusModsPage: %v", err)
	}
	if len(resp.Results) != 1 || resp.Results[0].ModID != 2400 {
		t.Fatalf("unexpected results: %+v", resp.Results)
	}
	if resp.Query != "" || resp.Page != 2 || resp.PageSize != 20 || resp.Total != 120 || !resp.HasMore {
		t.Fatalf("unexpected response metadata: %+v", resp)
	}

	variables, _ := capturedBody["variables"].(map[string]any)
	filter, _ := variables["filter"].(map[string]any)
	if _, ok := filter["name"]; ok {
		t.Fatalf("default listing filter should not include name: %+v", filter)
	}
	if _, ok := filter["gameDomainName"]; !ok {
		t.Fatalf("default listing filter missing gameDomainName: %+v", filter)
	}
	if variables["offset"] != float64(20) {
		t.Errorf("variables.offset = %v, want 20", variables["offset"])
	}
	if variables["count"] != float64(20) {
		t.Errorf("variables.count = %v, want 20", variables["count"])
	}
	sort, _ := variables["sort"].([]any)
	if len(sort) != 1 {
		t.Fatalf("variables.sort = %+v, want one downloads sort", variables["sort"])
	}
}

// ── keyword search auth-required mapping ─────────────────────────────────────

func TestSearchNexusMods_KeywordSearchAuthRequired_HTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	withNexusEndpoints(t, server)

	_, err := SearchNexusMods(context.Background(), "farming", "")
	if err != ErrNexusAuthRequired {
		t.Fatalf("err = %v, want ErrNexusAuthRequired", err)
	}
}

func TestSearchNexusMods_KeywordSearchAuthRequired_GraphQLError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]any{{"message": "Unauthenticated: must provide valid auth credentials"}},
		})
	}))
	withNexusEndpoints(t, server)

	_, err := SearchNexusMods(context.Background(), "farming", "")
	if err != ErrNexusAuthRequired {
		t.Fatalf("err = %v, want ErrNexusAuthRequired", err)
	}
}

// Regression guard: a schema error that happens to mention the "author"
// field must not be misclassified as an auth failure just because "author"
// contains the substring "auth". Only real auth/permission keywords should
// trigger ErrNexusAuthRequired.
func TestSearchNexusMods_GraphQLSchemaErrorMentioningAuthorIsNotAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]any{{"message": "Field 'mods' doesn't accept argument 'author'"}},
		})
	}))
	withNexusEndpoints(t, server)

	_, err := SearchNexusMods(context.Background(), "farming", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if err == ErrNexusAuthRequired {
		t.Fatalf("err = ErrNexusAuthRequired, want a generic schema error (message mentions 'author', not auth)")
	}
}

// ── result parsing ───────────────────────────────────────────────────────────

func TestSearchNexusMods_IDLookupParsesResult(t *testing.T) {
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

	resp, err := SearchNexusMods(context.Background(), "2400", fakeNexusAPIKey)
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

// Regression guard: the "mods" GraphQL root field does not accept a
// "gameDomain" argument directly (confirmed via live schema introspection
// against api.nexusmods.com/v2/graphql) — the game and keyword filters must
// both live inside the "filter" (ModsFilter) variable, as
// gameDomainName/name. An earlier version of this client guessed a
// "gameDomain" top-level argument, which Nexus rejected with a GraphQL
// error on every single keyword search. This test locks in the verified
// request shape so that regression can't silently reappear.
func TestSearchNexusMods_KeywordSearchRequestShape(t *testing.T) {
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"mods": map[string]any{"nodes": []map[string]any{}}},
		})
	}))
	withNexusEndpoints(t, server)

	if _, err := SearchNexusMods(context.Background(), "tractor", ""); err != nil {
		t.Fatalf("SearchNexusMods: %v", err)
	}

	variables, _ := capturedBody["variables"].(map[string]any)
	if variables == nil {
		t.Fatalf("request body missing variables: %+v", capturedBody)
	}
	if _, hasTopLevelGameDomain := variables["gameDomain"]; hasTopLevelGameDomain {
		t.Errorf("variables contains top-level %q — the mods field doesn't accept this argument", "gameDomain")
	}
	filter, _ := variables["filter"].(map[string]any)
	if filter == nil {
		t.Fatalf("variables missing filter: %+v", variables)
	}
	gameDomainName, _ := filter["gameDomainName"].([]any)
	if len(gameDomainName) != 1 {
		t.Fatalf("filter.gameDomainName = %+v, want one entry", filter["gameDomainName"])
	}
	entry, _ := gameDomainName[0].(map[string]any)
	if entry["value"] != nexusGameDomain {
		t.Errorf("filter.gameDomainName[0].value = %v, want %q", entry["value"], nexusGameDomain)
	}
	name, _ := filter["name"].([]any)
	if len(name) != 1 {
		t.Fatalf("filter.name = %+v, want one entry", filter["name"])
	}
	nameEntry, _ := name[0].(map[string]any)
	if nameEntry["value"] != "tractor" {
		t.Errorf("filter.name[0].value = %v, want %q", nameEntry["value"], "tractor")
	}
	sort, _ := variables["sort"].([]any)
	if len(sort) != 1 {
		t.Fatalf("variables.sort = %+v, want one downloads sort entry", variables["sort"])
	}
	sortEntry, _ := sort[0].(map[string]any)
	downloadsSort, _ := sortEntry["downloads"].(map[string]any)
	if downloadsSort["direction"] != "DESC" {
		t.Errorf("sort.downloads.direction = %v, want DESC", downloadsSort["direction"])
	}
	if variables["offset"] != float64(0) {
		t.Errorf("variables.offset = %v, want 0", variables["offset"])
	}
	if variables["count"] != float64(nexusDefaultPageSize) {
		t.Errorf("variables.count = %v, want %d", variables["count"], nexusDefaultPageSize)
	}
}

func TestSearchNexusMods_KeywordSearchParsesResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/graphql" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"mods": map[string]any{
					"totalCount": 2,
					"nodes": []map[string]any{
						{
							"modId": 1, "name": "Mod One", "downloads": 10, "endorsements": 1,
							"modRequirements": map[string]any{
								"nexusRequirements": map[string]any{
									"nodes": []map[string]any{
										{"modId": "2400", "modName": "SMAPI - Stardew Modding API", "url": "", "notes": ""},
										{"modId": "1", "modName": "Self requirement", "url": "", "notes": ""},
										{"modId": "9999", "modName": "External", "externalRequirement": true},
									},
								},
							},
						},
						{"modId": 2, "name": "Mod Two", "downloads": 20, "endorsements": 2},
					},
				},
			},
		})
	}))
	withNexusEndpoints(t, server)

	resp, err := SearchNexusMods(context.Background(), "farming", fakeNexusAPIKey)
	if err != nil {
		t.Fatalf("SearchNexusMods: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(resp.Results))
	}
	if resp.Results[0].Name != "Mod One" || resp.Results[1].ModID != 2 {
		t.Errorf("unexpected results: %+v", resp.Results)
	}
	if len(resp.Results[0].RequiredMods) != 1 {
		t.Fatalf("RequiredMods = %+v, want one Nexus prerequisite", resp.Results[0].RequiredMods)
	}
	if resp.Results[0].RequiredMods[0].ModID != 2400 || resp.Results[0].RequiredMods[0].Name != "SMAPI - Stardew Modding API" {
		t.Fatalf("RequiredMods[0] = %+v, want SMAPI", resp.Results[0].RequiredMods[0])
	}
	if resp.Results[0].RequiredMods[0].NexusURL != "https://www.nexusmods.com/stardewvalley/mods/2400" {
		t.Fatalf("RequiredMods[0].NexusURL = %q", resp.Results[0].RequiredMods[0].NexusURL)
	}
	if resp.Query != "farming" {
		t.Errorf("Query = %q, want %q", resp.Query, "farming")
	}
	if resp.Page != 1 || resp.PageSize != nexusDefaultPageSize || resp.Total != 2 || resp.HasMore {
		t.Errorf("unexpected paging metadata: %+v", resp)
	}
}

func TestSearchNexusMods_KeywordSearchPaginationRequestShape(t *testing.T) {
	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"mods": map[string]any{
					"totalCount": 410,
					"nodes":      []map[string]any{{"modId": 1915, "name": "Content Patcher"}},
				},
			},
		})
	}))
	withNexusEndpoints(t, server)

	resp, err := SearchNexusModsPage(context.Background(), "Content Patcher", "", 3, 12)
	if err != nil {
		t.Fatalf("SearchNexusModsPage: %v", err)
	}
	variables, _ := capturedBody["variables"].(map[string]any)
	if variables["offset"] != float64(24) {
		t.Errorf("variables.offset = %v, want 24", variables["offset"])
	}
	if variables["count"] != float64(12) {
		t.Errorf("variables.count = %v, want 12", variables["count"])
	}
	if resp.Page != 3 || resp.PageSize != 12 || resp.Total != 410 || !resp.HasMore {
		t.Errorf("unexpected paging metadata: %+v", resp)
	}
}

func TestSearchNexusMods_ResultsCappedAtMax(t *testing.T) {
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

	resp, err := SearchNexusMods(context.Background(), "mod", fakeNexusAPIKey)
	if err != nil {
		t.Fatalf("SearchNexusMods: %v", err)
	}
	if len(resp.Results) != nexusMaxResults {
		t.Fatalf("len(results) = %d, want %d", len(resp.Results), nexusMaxResults)
	}
}

// ── non-2xx error mapping ────────────────────────────────────────────────────

func TestSearchNexusMods_NonOKStatusMapsToNexusAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	withNexusEndpoints(t, server)

	_, err := SearchNexusMods(context.Background(), "999999", fakeNexusAPIKey)
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	withNexusEndpoints(t, server)

	_, err := SearchNexusMods(context.Background(), "farming", fakeNexusAPIKey)
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

	var receivedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("apikey")
		// Simulate a buggy/hostile upstream that echoes request details in
		// its error body — our client must never surface this to the caller.
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid apikey: " + secretKey))
	}))
	withNexusEndpoints(t, server)

	_, err := SearchNexusMods(context.Background(), "1234", secretKey)
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

func makeNexusModZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	manifest, err := zw.Create("CoolMod/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	_ = json.NewEncoder(manifest).Encode(modManifest{
		Name:        "Cool Mod",
		UniqueID:    "Author.CoolMod",
		Version:     "1.2.3",
		Author:      "Author",
		Description: "From Nexus",
		UpdateKeys:  []string{"Nexus:1234"},
	})
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestInstallNexusMod_DownloadsInstallsAndStoresMetadata(t *testing.T) {
	dataDir := t.TempDir()
	archive := makeNexusModZip(t)
	var serverURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/games/stardewvalley/mods/1234/files.json":
			if r.Header.Get("apikey") != fakeNexusAPIKey {
				t.Fatalf("apikey header = %q, want %q", r.Header.Get("apikey"), fakeNexusAPIKey)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"files": []map[string]any{{
					"file_id":       99,
					"name":          "Cool Mod main file",
					"version":       "1.2.3",
					"category_id":   1,
					"category_name": "MAIN",
				}},
			})
		case "/v1/games/stardewvalley/mods/1234/files/99/download_link.json":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{"URI": serverURL + "/download/cool.zip"}})
		case "/download/cool.zip":
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
			_, _ = w.Write(archive)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	serverURL = server.URL
	withNexusEndpoints(t, server)

	var logs []string
	imported, err := InstallNexusMod(context.Background(), dataDir, fakeNexusAPIKey, NexusModSearchResult{
		ModID:            1234,
		Name:             "Cool Mod",
		Summary:          "A cool Nexus mod",
		Author:           "Author",
		Version:          "1.2.3",
		EndorsementCount: 12,
		DownloadCount:    34,
		PictureURL:       "https://example.com/thumb.png",
		NexusURL:         "https://www.nexusmods.com/stardewvalley/mods/1234",
	}, func(message string) {
		logs = append(logs, message)
	})
	if err != nil {
		t.Fatalf("InstallNexusMod: %v", err)
	}
	if len(imported) != 1 {
		t.Fatalf("len(imported) = %d, want 1", len(imported))
	}
	if imported[0].FolderName != "CoolMod" || imported[0].NexusModID != 1234 {
		t.Fatalf("unexpected imported mod: %+v", imported[0])
	}
	if imported[0].PictureURL != "https://example.com/thumb.png" {
		t.Fatalf("PictureURL = %q, want thumbnail metadata", imported[0].PictureURL)
	}
	if GetModsRestartRequired(dataDir) {
		t.Fatal("restart-required flag should not be set by stopped-server Nexus install")
	}

	mods, err := ListMods(dataDir)
	if err != nil {
		t.Fatalf("ListMods: %v", err)
	}
	mods = ApplyNexusMetadataToMods(dataDir, mods)
	if len(mods) != 1 || mods[0].PictureURL != "https://example.com/thumb.png" {
		t.Fatalf("metadata was not applied to installed list: %+v", mods)
	}

	joinedLogs := strings.Join(logs, "\n")
	for _, want := range []string{"远程压缩包大小：", "下载进度：已下载", "剩余 0 B", "100.0%"} {
		if !strings.Contains(joinedLogs, want) {
			t.Fatalf("download progress logs missing %q in:\n%s", want, joinedLogs)
		}
	}
}

func TestNexusDownloadArchiveRejectsHTMLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html>download page</html>"))
	}))
	defer server.Close()

	var logs []string
	err := nexusDownloadArchive(context.Background(), server.URL+"/download/cool.zip", filepath.Join(t.TempDir(), "cool.zip"), func(message string) {
		logs = append(logs, message)
	})
	if err == nil {
		t.Fatal("nexusDownloadArchive returned nil, want html response error")
	}
	if !strings.Contains(err.Error(), "不是 ZIP 压缩包") {
		t.Fatalf("err = %v, want not-zip message", err)
	}
	if !strings.Contains(strings.Join(logs, "\n"), "远程响应类型：text/html") {
		t.Fatalf("logs = %v, want content-type log", logs)
	}
}

func TestEnrichNexusMetadataForMods_UsesGraphQLAndCaches(t *testing.T) {
	dataDir := t.TempDir()
	modsRoot := modsDir(dataDir)
	writeManifestWithUpdateKeys(t, modsRoot, "SMAPI", "Pathoschild.SMAPI", "4.5.2", []string{"Nexus:2400"})

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/graphql" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("apikey") != "" {
			t.Fatalf("apikey header = %q, want empty", r.Header.Get("apikey"))
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"mods": map[string]any{
					"nodes": []map[string]any{{
						"modId":        2400,
						"name":         "SMAPI - Stardew Modding API",
						"summary":      "The mod loader for Stardew Valley.",
						"version":      "4.5.2",
						"author":       "Pathoschild",
						"pictureUrl":   "https://example.com/smapi.png",
						"downloads":    12345,
						"endorsements": 678,
						"updatedAt":    "2026-03-14T19:07:23Z",
					}},
				},
			},
		})
	}))
	withNexusEndpoints(t, server)

	mods, err := ListMods(dataDir)
	if err != nil {
		t.Fatalf("ListMods: %v", err)
	}
	enriched := EnrichNexusMetadataForMods(context.Background(), dataDir, mods)
	if len(enriched) != 1 {
		t.Fatalf("len(enriched) = %d, want 1", len(enriched))
	}
	if enriched[0].PictureURL != "https://example.com/smapi.png" {
		t.Fatalf("PictureURL = %q, want GraphQL thumbnail", enriched[0].PictureURL)
	}
	if enriched[0].NexusSummary != "The mod loader for Stardew Valley." {
		t.Fatalf("NexusSummary = %q, want GraphQL summary", enriched[0].NexusSummary)
	}
	if enriched[0].DownloadCount != 12345 || enriched[0].EndorsementCount != 678 {
		t.Fatalf("counts not applied: %+v", enriched[0])
	}
	if calls != 1 {
		t.Fatalf("GraphQL calls = %d, want 1", calls)
	}

	mods, err = ListMods(dataDir)
	if err != nil {
		t.Fatalf("ListMods second call: %v", err)
	}
	cached := EnrichNexusMetadataForMods(context.Background(), dataDir, mods)
	if cached[0].PictureURL != "https://example.com/smapi.png" {
		t.Fatalf("cached PictureURL = %q, want sidecar metadata", cached[0].PictureURL)
	}
	if calls != 1 {
		t.Fatalf("GraphQL calls after cached enrichment = %d, want still 1", calls)
	}
}

func TestEnrichNexusMetadataForMods_FillsBuiltInSMAPIRuntime(t *testing.T) {
	dataDir := t.TempDir()
	modsRoot := modsDir(dataDir)
	createTestMod(t, modsRoot, controlModFolderName, "AnXiYiZhi.StardewAnxiPanel.Control", "Panel Control")

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/graphql" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"mods": map[string]any{
					"nodes": []map[string]any{{
						"modId":      2400,
						"name":       "SMAPI - Stardew Modding API",
						"summary":    "The mod loader for Stardew Valley.",
						"version":    "4.5.2",
						"author":     "Pathoschild",
						"pictureUrl": "https://example.com/smapi-runtime.png",
					}},
				},
			},
		})
	}))
	withNexusEndpoints(t, server)

	mods, err := ListMods(dataDir)
	if err != nil {
		t.Fatalf("ListMods: %v", err)
	}
	enriched := EnrichNexusMetadataForMods(context.Background(), dataDir, mods)
	if len(enriched) != 2 {
		t.Fatalf("len(enriched) = %d, want SMAPI runtime plus control mod", len(enriched))
	}
	smapi := enriched[0]
	if !smapi.BuiltIn || smapi.UniqueID != "Pathoschild.SMAPI" {
		t.Fatalf("first mod should be built-in SMAPI runtime: %+v", smapi)
	}
	if smapi.PictureURL != "https://example.com/smapi-runtime.png" {
		t.Fatalf("SMAPI PictureURL = %q, want GraphQL thumbnail", smapi.PictureURL)
	}
	if calls != 1 {
		t.Fatalf("GraphQL calls = %d, want 1", calls)
	}
}

func TestApplyNexusMetadataToMods_MergesFolderEntryWithRicherSameModID(t *testing.T) {
	dataDir := t.TempDir()
	if err := saveNexusMetadataStore(dataDir, nexusMetadataStore{
		Mods: map[string]nexusInstalledMetadata{
			"MultipleConstructionOrders": {
				ModID:      47289,
				Name:       "Multiple Construction Orders",
				Summary:    "Allows Robin to accept multiple construction orders.",
				PictureURL: "https://example.com/mco.png",
				NexusURL:   "https://www.nexusmods.com/stardewvalley/mods/47289",
			},
			"[CP] MultipleConstructionOrders": {
				ModID:    47289,
				Name:     "Multiple Construction Orders",
				NexusURL: "https://www.nexusmods.com/stardewvalley/mods/47289",
			},
		},
	}); err != nil {
		t.Fatalf("saveNexusMetadataStore: %v", err)
	}

	mods := ApplyNexusMetadataToMods(dataDir, []registry.ModInfo{{
		ID:         "[CP] MultipleConstructionOrders",
		FolderName: "[CP] MultipleConstructionOrders",
	}})
	if len(mods) != 1 {
		t.Fatalf("len(mods) = %d, want 1", len(mods))
	}
	if mods[0].PictureURL != "https://example.com/mco.png" {
		t.Fatalf("PictureURL = %q, want richer same-modID thumbnail", mods[0].PictureURL)
	}
	if mods[0].OriginSource != "nexus" || mods[0].OriginNexusModID != 47289 {
		t.Fatalf("origin = %q/%d, want nexus/47289", mods[0].OriginSource, mods[0].OriginNexusModID)
	}
}

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

func TestParseNexusNXMURL(t *testing.T) {
	ticket, err := ParseNexusNXMURL("nxm://stardewvalley/mods/1234/files/99?key=abc123&expires=1782897483&user_id=42")
	if err != nil {
		t.Fatalf("ParseNexusNXMURL: %v", err)
	}
	if ticket.ModID != 1234 || ticket.FileID != 99 || ticket.Key != "abc123" || ticket.Expires != "1782897483" {
		t.Fatalf("ticket = %+v, want mod/file/key/expires", ticket)
	}
}

func TestInstallNexusModWithTicket_UsesNXMKeyAndExpires(t *testing.T) {
	dataDir := t.TempDir()
	archive := makeNexusModZip(t)
	var serverURL string
	var sawTicket bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/games/stardewvalley/mods/1234/files/99/download_link.json":
			if r.Header.Get("apikey") != fakeNexusAPIKey {
				t.Fatalf("apikey header = %q, want %q", r.Header.Get("apikey"), fakeNexusAPIKey)
			}
			if r.URL.Query().Get("key") != "abc123" || r.URL.Query().Get("expires") != "1782897483" {
				t.Fatalf("ticket query = %q", r.URL.RawQuery)
			}
			sawTicket = true
			_ = json.NewEncoder(w).Encode([]map[string]any{{"URI": serverURL + "/download/cool.zip"}})
		case "/download/cool.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(archive)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	serverURL = server.URL
	withNexusEndpoints(t, server)

	imported, err := InstallNexusModWithTicket(context.Background(), dataDir, fakeNexusAPIKey, NexusModSearchResult{
		ModID:    1234,
		Name:     "Cool Nexus Mod",
		NexusURL: "https://www.nexusmods.com/stardewvalley/mods/1234",
	}, NexusDownloadTicket{ModID: 1234, FileID: 99, Key: "abc123", Expires: "1782897483"}, nil)
	if err != nil {
		t.Fatalf("InstallNexusModWithTicket: %v", err)
	}
	if !sawTicket {
		t.Fatal("download_link endpoint was not called with NXM ticket")
	}
	if len(imported) != 1 || imported[0].FolderName != "CoolMod" {
		t.Fatalf("imported = %+v, want CoolMod", imported)
	}
}

func TestApplyNexusInstalledMatch_MatchesByNexusID(t *testing.T) {
	dir := t.TempDir()
	modsRoot := modsDir(dir)
	writeManifestWithUpdateKeys(t, modsRoot, "SVE", "FlashShifter.SVE", "1.13.6", []string{"Nexus:2400"})
	writeManifestWithUpdateKeys(t, modsRoot, "OtherMod", "Some.OtherMod", "0.1.0", []string{"Chucklefish:1234"})

	results := []NexusModSearchResult{
		{ModID: 2400, Name: "Stardew Valley Expanded"},
		{ModID: 5555, Name: "Needs SVE", RequiredMods: []NexusRequiredMod{{ModID: 2400, Name: "Stardew Valley Expanded"}}},
		{ModID: 9999, Name: "Not Installed"},
	}
	results = ApplyNexusInstalledMatch(dir, "", results)

	if !results[0].Installed {
		t.Errorf("result[0].Installed = false, want true")
	}
	if !results[0].InstalledEnabled {
		t.Errorf("result[0].InstalledEnabled = false, want true")
	}
	if results[0].InstalledFolderName != "SVE" {
		t.Errorf("InstalledFolderName = %q, want SVE", results[0].InstalledFolderName)
	}
	if results[0].InstalledVersion != "1.13.6" {
		t.Errorf("InstalledVersion = %q, want 1.13.6", results[0].InstalledVersion)
	}
	if !results[1].RequiredMods[0].Installed {
		t.Errorf("required mod install state was not applied: %+v", results[1].RequiredMods[0])
	}
	if !results[1].RequiredMods[0].InstalledEnabled {
		t.Errorf("required mod enabled state was not applied: %+v", results[1].RequiredMods[0])
	}
	if results[1].RequiredMods[0].InstalledFolderName != "SVE" {
		t.Errorf("required InstalledFolderName = %q, want SVE", results[1].RequiredMods[0].InstalledFolderName)
	}
	if results[2].Installed {
		t.Errorf("result[1].Installed = true, want false (no matching Nexus:ID installed)")
	}
}

func TestApplyNexusInstalledMatch_NoMods(t *testing.T) {
	dir := t.TempDir()
	results := []NexusModSearchResult{{ModID: 1, Name: "X"}}
	got := ApplyNexusInstalledMatch(dir, "", results)
	if got[0].Installed {
		t.Errorf("Installed = true, want false when no mods directory exists")
	}
}

func TestApplyNexusInstalledMatch_IncludesDisabledMods(t *testing.T) {
	dir := t.TempDir()
	disabledRoot := disabledModsDir(dir)
	writeManifestWithUpdateKeys(t, disabledRoot, "DisabledMod", "Author.Disabled", "2.0.0", []string{"Nexus:5555"})

	results := []NexusModSearchResult{{ModID: 5555, Name: "Disabled Mod"}}
	results = ApplyNexusInstalledMatch(dir, "", results)

	if !results[0].Installed {
		t.Fatalf("Installed = false, want true for mod in disabled directory")
	}
	if results[0].InstalledEnabled {
		t.Fatalf("InstalledEnabled = true, want false for disabled mod")
	}
	if results[0].InstalledFolderName != "DisabledMod" {
		t.Fatalf("InstalledFolderName = %q, want DisabledMod", results[0].InstalledFolderName)
	}
	if results[0].InstalledVersion != "2.0.0" {
		t.Fatalf("InstalledVersion = %q, want 2.0.0", results[0].InstalledVersion)
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
