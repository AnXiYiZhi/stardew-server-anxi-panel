package stardew_junimo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Nexus Mods only — Stardew Valley game domain. Read-only search; no
// download/install integration in this phase.
const (
	nexusGameDomain     = "stardewvalley"
	nexusUserAgent      = "stardew-server-anxi-panel/1.0 (+https://github.com/anxi-panel)"
	nexusRequestTimeout = 10 * time.Second
	nexusMaxResults     = 20
)

// nexusV1BaseURL and nexusGraphQLURL are vars (not consts) so tests can point
// them at an httptest.Server instead of the real Nexus API.
var (
	nexusV1BaseURL  = "https://api.nexusmods.com/v1"
	nexusGraphQLURL = "https://api.nexusmods.com/v2/graphql"
)

// nexusHTTPClient is swappable in tests.
var nexusHTTPClient = &http.Client{Timeout: nexusRequestTimeout}

// ErrNexusAPIKeyMissing is returned when NEXUS_API_KEY is not configured.
var ErrNexusAPIKeyMissing = errors.New("未配置 Nexus Mods API Key")

// ErrInvalidNexusQuery is returned when the search query is empty after trimming.
var ErrInvalidNexusQuery = errors.New("查询关键词不能为空")

// NexusAPIError wraps a non-2xx response from Nexus. Only the status code is
// retained — the response body is never surfaced, so an upstream error page
// can't leak request details (and the API key is never included in it anyway,
// since it's sent as a header, not a query parameter).
type NexusAPIError struct {
	StatusCode int
}

func (e *NexusAPIError) Error() string {
	return fmt.Sprintf("nexus api returned status %d", e.StatusCode)
}

// NexusModSearchResult is one search hit, merged with local install state by
// ApplyNexusInstalledMatch.
type NexusModSearchResult struct {
	ModID               int    `json:"modId"`
	Name                string `json:"name"`
	Summary             string `json:"summary,omitempty"`
	Author              string `json:"author,omitempty"`
	Version             string `json:"version,omitempty"`
	UpdatedAt           string `json:"updatedAt,omitempty"`
	EndorsementCount    int    `json:"endorsementCount"`
	DownloadCount       int    `json:"downloadCount"`
	PictureURL          string `json:"pictureUrl,omitempty"`
	NexusURL            string `json:"nexusUrl"`
	Installed           bool   `json:"installed"`
	InstalledFolderName string `json:"installedFolderName,omitempty"`
	InstalledVersion    string `json:"installedVersion,omitempty"`
}

// NexusModSearchResponse is returned by GET .../mods/nexus/search.
type NexusModSearchResponse struct {
	Query   string                 `json:"query"`
	Results []NexusModSearchResult `json:"results"`
}

// nexusAPIKey reads NEXUS_API_KEY from the environment. Read at call time
// (rather than cached at process start) so tests can use t.Setenv freely.
func nexusAPIKey() string {
	return strings.TrimSpace(os.Getenv("NEXUS_API_KEY"))
}

func nexusModURL(modID int) string {
	return fmt.Sprintf("https://www.nexusmods.com/%s/mods/%d", nexusGameDomain, modID)
}

// SearchNexusMods searches Stardew Valley mods on Nexus Mods.
//
// A purely numeric query is treated as an exact mod ID lookup via the
// official, documented v1 REST endpoint
// (GET /v1/games/{domain}/mods/{id}.json). Any other query is treated as a
// keyword search via Nexus's GraphQL v2 API (the same backend that powers
// nexusmods.com's own search box). The v1 REST shape is well documented and
// stable; the GraphQL keyword-search shape was not independently verified
// against live Nexus docs while building this (see nexusSearchByKeyword) and
// may need field-name adjustments once exercised against a real API key.
func SearchNexusMods(ctx context.Context, query string) (NexusModSearchResponse, error) {
	apiKey := nexusAPIKey()
	if apiKey == "" {
		return NexusModSearchResponse{}, ErrNexusAPIKeyMissing
	}
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return NexusModSearchResponse{}, ErrInvalidNexusQuery
	}

	var results []NexusModSearchResult
	var err error
	if modID, ok := parsePositiveInt(trimmed); ok {
		var result NexusModSearchResult
		result, err = nexusGetModByID(ctx, apiKey, modID)
		if err == nil {
			results = []NexusModSearchResult{result}
		}
	} else {
		results, err = nexusSearchByKeyword(ctx, apiKey, trimmed)
	}
	if err != nil {
		return NexusModSearchResponse{}, err
	}

	if len(results) > nexusMaxResults {
		results = results[:nexusMaxResults]
	}
	return NexusModSearchResponse{Query: trimmed, Results: results}, nil
}

func parsePositiveInt(s string) (int, bool) {
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// ── v1 REST: exact mod-by-id lookup ─────────────────────────────────────────

// nexusV1Mod mirrors the documented v1 REST mod object
// (GET /v1/games/{domain}/mods/{id}.json).
type nexusV1Mod struct {
	ModID            int    `json:"mod_id"`
	Name             string `json:"name"`
	Summary          string `json:"summary"`
	Version          string `json:"version"`
	Author           string `json:"author"`
	PictureURL       string `json:"picture_url"`
	ModDownloads     int    `json:"mod_downloads"`
	EndorsementCount int    `json:"endorsement_count"`
	UpdatedTime      string `json:"updated_time"`
}

func nexusGetModByID(ctx context.Context, apiKey string, modID int) (NexusModSearchResult, error) {
	url := fmt.Sprintf("%s/games/%s/mods/%d.json", nexusV1BaseURL, nexusGameDomain, modID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return NexusModSearchResult{}, fmt.Errorf("build nexus request: %w", err)
	}
	setNexusHeaders(req, apiKey)

	body, err := doNexusRequest(req)
	if err != nil {
		return NexusModSearchResult{}, err
	}

	var m nexusV1Mod
	if err := json.Unmarshal(body, &m); err != nil {
		return NexusModSearchResult{}, fmt.Errorf("解析 Nexus 返回结果失败: %w", err)
	}
	return nexusV1ModToResult(m), nil
}

func nexusV1ModToResult(m nexusV1Mod) NexusModSearchResult {
	return NexusModSearchResult{
		ModID:            m.ModID,
		Name:             m.Name,
		Summary:          m.Summary,
		Author:           m.Author,
		Version:          m.Version,
		UpdatedAt:        m.UpdatedTime,
		EndorsementCount: m.EndorsementCount,
		DownloadCount:    m.ModDownloads,
		PictureURL:       m.PictureURL,
		NexusURL:         nexusModURL(m.ModID),
	}
}

// ── GraphQL v2: keyword search ──────────────────────────────────────────────
//
// NOTE: Nexus's v1 REST API has no keyword full-text search endpoint (only
// exact-id lookup, recently-updated listings, and md5 lookup are documented).
// Keyword search here calls the GraphQL v2 API at api.nexusmods.com/v2/graphql
// — the same backend nexusmods.com's own search box uses — with a query
// modeled on Nexus's publicly known "ModsListing"-style listing operations.
// The exact field names were not verified against a live API key while
// building this; if results stop parsing once NEXUS_API_KEY is configured
// for real, check the response shape against api-docs.nexusmods.com /
// graphql.nexusmods.com and adjust nexusGraphQLSearchQuery and
// nexusGraphQLResponse accordingly.
const nexusGraphQLSearchQuery = `
query ModsListing($gameDomain: String!, $filter: ModsFilter, $count: Int!) {
  mods(gameDomain: $gameDomain, filter: $filter, count: $count) {
    nodes {
      modId
      name
      summary
      version
      author
      pictureUrl
      downloads
      endorsements
      updatedAt
    }
    totalCount
  }
}
`

type nexusGraphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type nexusGraphQLNode struct {
	ModID        int    `json:"modId"`
	Name         string `json:"name"`
	Summary      string `json:"summary"`
	Version      string `json:"version"`
	Author       string `json:"author"`
	PictureURL   string `json:"pictureUrl"`
	Downloads    int    `json:"downloads"`
	Endorsements int    `json:"endorsements"`
	UpdatedAt    string `json:"updatedAt"`
}

type nexusGraphQLResponse struct {
	Data struct {
		Mods struct {
			Nodes []nexusGraphQLNode `json:"nodes"`
		} `json:"mods"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func nexusSearchByKeyword(ctx context.Context, apiKey, query string) ([]NexusModSearchResult, error) {
	reqBody := nexusGraphQLRequest{
		Query: nexusGraphQLSearchQuery,
		Variables: map[string]any{
			"gameDomain": nexusGameDomain,
			"filter":     map[string]any{"search": query},
			"count":      nexusMaxResults,
		},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("build nexus graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, nexusGraphQLURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build nexus request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	setNexusHeaders(req, apiKey)

	body, err := doNexusRequest(req)
	if err != nil {
		return nil, err
	}

	var gqlResp nexusGraphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("解析 Nexus 返回结果失败: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("nexus graphql error: %s", gqlResp.Errors[0].Message)
	}

	results := make([]NexusModSearchResult, 0, len(gqlResp.Data.Mods.Nodes))
	for _, n := range gqlResp.Data.Mods.Nodes {
		results = append(results, NexusModSearchResult{
			ModID:            n.ModID,
			Name:             n.Name,
			Summary:          n.Summary,
			Author:           n.Author,
			Version:          n.Version,
			UpdatedAt:        n.UpdatedAt,
			EndorsementCount: n.Endorsements,
			DownloadCount:    n.Downloads,
			PictureURL:       n.PictureURL,
			NexusURL:         nexusModURL(n.ModID),
		})
	}
	return results, nil
}

// ── shared request plumbing ─────────────────────────────────────────────────

func setNexusHeaders(req *http.Request, apiKey string) {
	req.Header.Set("apikey", apiKey)
	req.Header.Set("User-Agent", nexusUserAgent)
	req.Header.Set("Accept", "application/json")
}

// doNexusRequest executes a request and returns the response body for a 2xx
// status. Non-2xx responses are converted to *NexusAPIError without ever
// reading the response body into an error message (so nothing upstream —
// including any echoed request details — can leak via our error path).
func doNexusRequest(req *http.Request) ([]byte, error) {
	resp, err := nexusHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nexus 请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, &NexusAPIError{StatusCode: resp.StatusCode}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 Nexus 响应失败: %w", err)
	}
	return body, nil
}

// ── installed-mod matching ──────────────────────────────────────────────────

// ApplyNexusInstalledMatch marks each search result as installed when a
// locally installed mod's manifest UpdateKeys resolves to the same Nexus mod
// ID. Only an installed flag is set in this phase — no update comparison.
func ApplyNexusInstalledMatch(dataDir string, results []NexusModSearchResult) []NexusModSearchResult {
	mods, err := ListMods(dataDir)
	if err != nil || len(mods) == 0 {
		return results
	}
	installedByNexusID := map[int]struct {
		folderName string
		version    string
	}{}
	for _, m := range mods {
		if m.NexusModID > 0 {
			installedByNexusID[m.NexusModID] = struct {
				folderName string
				version    string
			}{folderName: m.FolderName, version: m.Version}
		}
	}

	for i := range results {
		if match, ok := installedByNexusID[results[i].ModID]; ok {
			results[i].Installed = true
			results[i].InstalledFolderName = match.folderName
			results[i].InstalledVersion = match.version
		}
	}
	return results
}
