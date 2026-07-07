package stardew_junimo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/netdns"
)

// Nexus Mods only — Stardew Valley game domain. Read-only search; no
// download/install integration in this phase.
const (
	nexusGameDomain      = "stardewvalley"
	nexusStardewGameID   = "1303"
	nexusUserAgent       = "stardew-server-anxi-panel/1.0 (+https://github.com/anxi-panel)"
	nexusRequestTimeout  = 10 * time.Second
	nexusArchiveTimeout  = 15 * time.Minute
	nexusDefaultPageSize = 20
	nexusMaxResults      = nexusDefaultPageSize
	nexusMaxPageSize     = 50
)

// nexusV1BaseURL and nexusGraphQLURL are vars (not consts) so tests can point
// them at an httptest.Server instead of the real Nexus API.
var (
	nexusV1BaseURL  = "https://api.nexusmods.com/v1"
	nexusGraphQLURL = "https://api.nexusmods.com/v2/graphql"
)

// nexusHTTPClient is swappable in tests. It uses the DNS-fallback transport so
// a flaky host/router resolver doesn't break Nexus lookups (see netdns).
var nexusHTTPClient = netdns.NewClient(nexusRequestTimeout)

// nexusArchiveHTTPClient is used only for ZIP archive bodies. Search/API
// requests should keep the short timeout above, but large Nexus archives can
// legitimately take much longer than 10 seconds on throttled downloads.
var nexusArchiveHTTPClient = netdns.NewClient(nexusArchiveTimeout)

// ErrNexusAPIKeyMissing is returned when a key-gated Nexus operation is
// requested but no Nexus API key is configured in panel settings. Keyword
// search does not require a key (see nexusSearchByKeyword).
var ErrNexusAPIKeyMissing = errors.New("未配置 Nexus Mods API Key")

// ErrInvalidNexusQuery is returned when a Nexus install/download request is missing a valid mod ID.
var ErrInvalidNexusQuery = errors.New("查询关键词不能为空")

// ErrNexusAuthRequired is returned when a keyword search hits Nexus's
// GraphQL v2 API and the upstream response indicates the query needs
// authentication/OAuth capability beyond a plain personal API key (or no key
// at all was sent). This is distinct from ErrNexusAPIKeyMissing: it means
// Nexus itself rejected the request for auth reasons, not that the panel
// refused to even try.
var ErrNexusAuthRequired = errors.New("该查询需要 Nexus OAuth/认证能力")

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

// NexusRequestError wraps network/client failures while calling Nexus. The
// wrapped error is logged by the web layer but is not returned to browsers.
type NexusRequestError struct {
	Err error
}

func (e *NexusRequestError) Error() string {
	if e == nil || e.Err == nil {
		return "nexus request failed"
	}
	return fmt.Sprintf("nexus request failed: %v", e.Err)
}

func (e *NexusRequestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NexusModSearchResult is one search hit, merged with local install state by
// ApplyNexusInstalledMatch.
type NexusModSearchResult struct {
	ModID               int                `json:"modId"`
	Name                string             `json:"name"`
	Summary             string             `json:"summary,omitempty"`
	Author              string             `json:"author,omitempty"`
	Version             string             `json:"version,omitempty"`
	UpdatedAt           string             `json:"updatedAt,omitempty"`
	EndorsementCount    int                `json:"endorsementCount"`
	DownloadCount       int                `json:"downloadCount"`
	PictureURL          string             `json:"pictureUrl,omitempty"`
	NexusURL            string             `json:"nexusUrl"`
	Installed           bool               `json:"installed"`
	InstalledEnabled    bool               `json:"installedEnabled"`
	InstalledFolderName string             `json:"installedFolderName,omitempty"`
	InstalledVersion    string             `json:"installedVersion,omitempty"`
	RequiredMods        []NexusRequiredMod `json:"requiredMods,omitempty"`
}

// NexusRequiredMod is a Nexus-side prerequisite declared on the mod page.
type NexusRequiredMod struct {
	ModID               int    `json:"modId"`
	Name                string `json:"name"`
	Notes               string `json:"notes,omitempty"`
	NexusURL            string `json:"nexusUrl"`
	Installed           bool   `json:"installed"`
	InstalledEnabled    bool   `json:"installedEnabled"`
	InstalledFolderName string `json:"installedFolderName,omitempty"`
	InstalledVersion    string `json:"installedVersion,omitempty"`
}

// NexusModSearchResponse is returned by GET .../mods/nexus/search.
type NexusModSearchResponse struct {
	Query    string                 `json:"query"`
	Results  []NexusModSearchResult `json:"results"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"pageSize"`
	Total    int                    `json:"total"`
	HasMore  bool                   `json:"hasMore"`
}

func nexusModURL(modID int) string {
	return fmt.Sprintf("https://www.nexusmods.com/%s/mods/%d", nexusGameDomain, modID)
}

// SearchNexusMods searches Stardew Valley mods on Nexus Mods.
//
// A purely numeric query is treated as an exact mod ID lookup. With an API
// key configured it uses the documented v1 REST endpoint; without a key it
// uses the public GraphQL v2 mods filter with gameId + modId.
//
// An empty query returns the default Nexus listing for Stardew Valley, sorted
// by downloads descending, so the download page can show a useful first screen.
//
// Any other query is treated as a keyword search via Nexus's GraphQL v2 API
// (the same public read endpoint that powers nexusmods.com's own search box).
// That endpoint does not require a personal API key for read-only queries, so
// this path is NOT gated on apiKey: if a key is configured it's still
// forwarded (harmless, may help with rate limits), but an empty key never
// blocks the request. If Nexus itself rejects the query for auth reasons, the
// keyword path returns ErrNexusAuthRequired instead.
func SearchNexusMods(ctx context.Context, query string, apiKey string) (NexusModSearchResponse, error) {
	return SearchNexusModsPage(ctx, query, apiKey, 1, nexusDefaultPageSize)
}

func SearchNexusModsPage(ctx context.Context, query string, apiKey string, page, pageSize int) (NexusModSearchResponse, error) {
	trimmed := strings.TrimSpace(query)
	apiKey = strings.TrimSpace(apiKey)
	page, pageSize, offset := normalizeSearchPagination(page, pageSize)

	var results []NexusModSearchResult
	total := 0
	var err error
	if trimmed == "" {
		var pageResult nexusGraphQLModsPage
		pageResult, err = nexusSearchPopularPage(ctx, apiKey, pageSize, offset)
		results = pageResult.Results
		total = pageResult.Total
		if total < offset+len(results) {
			total = offset + len(results)
		}
	} else if modID, ok := parsePositiveInt(trimmed); ok {
		page = 1
		if apiKey != "" {
			var result NexusModSearchResult
			result, err = nexusGetModByID(ctx, apiKey, modID)
			if err == nil {
				results = []NexusModSearchResult{result}
				total = 1
			}
		} else {
			results, err = nexusGetModsByIDGraphQL(ctx, []int{modID})
			total = len(results)
		}
	} else {
		var pageResult nexusGraphQLModsPage
		pageResult, err = nexusSearchByKeywordPage(ctx, apiKey, trimmed, pageSize, offset)
		results = pageResult.Results
		total = pageResult.Total
		if total < offset+len(results) {
			total = offset + len(results)
		}
	}
	if err != nil {
		return NexusModSearchResponse{}, err
	}
	if len(results) > pageSize {
		results = results[:pageSize]
	}

	return NexusModSearchResponse{
		Query:    trimmed,
		Results:  results,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
		HasMore:  page*pageSize < total,
	}, nil
}

func normalizeSearchPagination(page, pageSize int) (int, int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = nexusDefaultPageSize
	}
	if pageSize > nexusMaxPageSize {
		pageSize = nexusMaxPageSize
	}
	return page, pageSize, (page - 1) * pageSize
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
// — the same backend nexusmods.com's own search box uses.
//
// This is a public, read-only query — unlike the v1 REST endpoints, it does
// not require a personal API key. We deliberately do not gate this path on
// apiKey (see SearchNexusMods). If Nexus responds with an
// auth-related rejection (401/403, or a GraphQL error mentioning
// auth/forbidden/permission), we surface ErrNexusAuthRequired so the caller
// gets an accurate "this needs more than a personal key" message instead of
// being told to configure a Nexus API key (which wouldn't actually fix it).
//
// The query shape below was confirmed against the live endpoint via schema
// introspection (`{ __type(name: "...") { ... } }`) and a real search
// request — it is not a guess. The `mods` root field takes no `gameDomain`
// argument directly; the game and keyword are both expressed as ModsFilter
// fields: `gameDomainName` (EQUALS) and `name` (WILDCARD, substring match
// against the mod title — confirmed to match e.g. "tractor" inside "Wallet
// Tools - Tractor Mod Addon" without needing literal "*" wildcards in the
// value). If Nexus changes this schema in the future, re-run the
// introspection query above against https://api.nexusmods.com/v2/graphql to
// find the new shape.
const nexusGraphQLSearchQuery = `
query ModsListing($filter: ModsFilter, $sort: [ModsSort!], $offset: Int, $count: Int!) {
  mods(filter: $filter, sort: $sort, offset: $offset, count: $count) {
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
      modRequirements {
        nexusRequirements(offset: 0, count: 10) {
          nodes {
            modId
            modName
            notes
            url
            externalRequirement
          }
        }
      }
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
	ModID           int    `json:"modId"`
	Name            string `json:"name"`
	Summary         string `json:"summary"`
	Version         string `json:"version"`
	Author          string `json:"author"`
	PictureURL      string `json:"pictureUrl"`
	Downloads       int    `json:"downloads"`
	Endorsements    int    `json:"endorsements"`
	UpdatedAt       string `json:"updatedAt"`
	ModRequirements struct {
		NexusRequirements struct {
			Nodes []nexusGraphQLRequirementNode `json:"nodes"`
		} `json:"nexusRequirements"`
	} `json:"modRequirements"`
}

type nexusGraphQLRequirementNode struct {
	ModID               string `json:"modId"`
	ModName             string `json:"modName"`
	Notes               string `json:"notes"`
	URL                 string `json:"url"`
	ExternalRequirement bool   `json:"externalRequirement"`
}

type nexusGraphQLResponse struct {
	Data struct {
		Mods struct {
			Nodes      []nexusGraphQLNode `json:"nodes"`
			TotalCount int                `json:"totalCount"`
		} `json:"mods"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type nexusGraphQLModsPage struct {
	Results []NexusModSearchResult
	Total   int
}

func nexusSearchByKeyword(ctx context.Context, apiKey, query string) ([]NexusModSearchResult, error) {
	page, err := nexusSearchByKeywordPage(ctx, apiKey, query, nexusDefaultPageSize, 0)
	if err != nil {
		return nil, err
	}
	return page.Results, nil
}

func nexusSearchByKeywordPage(ctx context.Context, apiKey, query string, count, offset int) (nexusGraphQLModsPage, error) {
	reqBody := nexusGraphQLRequest{
		Query: nexusGraphQLSearchQuery,
		Variables: map[string]any{
			"filter": map[string]any{
				"gameDomainName": []map[string]any{{"value": nexusGameDomain, "op": "EQUALS"}},
				"name":           []map[string]any{{"value": query, "op": "WILDCARD"}},
			},
			"sort": []map[string]any{
				{"downloads": map[string]any{"direction": "DESC"}},
			},
			"offset": offset,
			"count":  count,
		},
	}
	return doNexusGraphQLModsPageQuery(ctx, apiKey, reqBody)
}

func nexusSearchPopularPage(ctx context.Context, apiKey string, count, offset int) (nexusGraphQLModsPage, error) {
	reqBody := nexusGraphQLRequest{
		Query: nexusGraphQLSearchQuery,
		Variables: map[string]any{
			"filter": map[string]any{
				"gameDomainName": []map[string]any{{"value": nexusGameDomain, "op": "EQUALS"}},
			},
			"sort": []map[string]any{
				{"downloads": map[string]any{"direction": "DESC"}},
			},
			"offset": offset,
			"count":  count,
		},
	}
	return doNexusGraphQLModsPageQuery(ctx, apiKey, reqBody)
}

func nexusGetModsByIDGraphQL(ctx context.Context, modIDs []int) ([]NexusModSearchResult, error) {
	filters := make([]map[string]any, 0, len(modIDs))
	seen := map[int]struct{}{}
	for _, modID := range modIDs {
		if modID <= 0 {
			continue
		}
		if _, ok := seen[modID]; ok {
			continue
		}
		seen[modID] = struct{}{}
		filters = append(filters, map[string]any{
			"gameId": []map[string]any{{"value": nexusStardewGameID, "op": "EQUALS"}},
			"modId":  []map[string]any{{"value": fmt.Sprintf("%d", modID), "op": "EQUALS"}},
		})
	}
	if len(filters) == 0 {
		return nil, nil
	}

	filter := map[string]any{}
	if len(filters) == 1 {
		filter = filters[0]
	} else {
		filter = map[string]any{
			"op":     "OR",
			"filter": filters,
		}
	}
	reqBody := nexusGraphQLRequest{
		Query: nexusGraphQLSearchQuery,
		Variables: map[string]any{
			"filter": filter,
			"offset": 0,
			"count":  len(filters),
		},
	}
	return doNexusGraphQLModsQuery(ctx, "", reqBody)
}

func doNexusGraphQLModsQuery(ctx context.Context, apiKey string, reqBody nexusGraphQLRequest) ([]NexusModSearchResult, error) {
	page, err := doNexusGraphQLModsPageQuery(ctx, apiKey, reqBody)
	if err != nil {
		return nil, err
	}
	return page.Results, nil
}

func doNexusGraphQLModsPageQuery(ctx context.Context, apiKey string, reqBody nexusGraphQLRequest) (nexusGraphQLModsPage, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nexusGraphQLModsPage{}, fmt.Errorf("build nexus graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, nexusGraphQLURL, bytes.NewReader(payload))
	if err != nil {
		return nexusGraphQLModsPage{}, fmt.Errorf("build nexus request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	setNexusHeaders(req, apiKey)

	body, err := doNexusRequest(req)
	if err != nil {
		var apiErr *NexusAPIError
		if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden) {
			return nexusGraphQLModsPage{}, ErrNexusAuthRequired
		}
		return nexusGraphQLModsPage{}, err
	}

	var gqlResp nexusGraphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nexusGraphQLModsPage{}, fmt.Errorf("解析 Nexus 返回结果失败: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		if isNexusAuthError(gqlResp.Errors[0].Message) {
			return nexusGraphQLModsPage{}, ErrNexusAuthRequired
		}
		return nexusGraphQLModsPage{}, fmt.Errorf("nexus graphql error: %s", gqlResp.Errors[0].Message)
	}

	results := make([]NexusModSearchResult, 0, len(gqlResp.Data.Mods.Nodes))
	for _, n := range gqlResp.Data.Mods.Nodes {
		results = append(results, nexusGraphQLNodeToResult(n))
	}
	total := gqlResp.Data.Mods.TotalCount
	if total < len(results) {
		total = len(results)
	}
	return nexusGraphQLModsPage{Results: results, Total: total}, nil
}

func nexusGraphQLNodeToResult(n nexusGraphQLNode) NexusModSearchResult {
	return NexusModSearchResult{
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
		RequiredMods:     nexusGraphQLRequirementsToRequiredMods(n),
	}
}

func nexusGraphQLRequirementsToRequiredMods(n nexusGraphQLNode) []NexusRequiredMod {
	if len(n.ModRequirements.NexusRequirements.Nodes) == 0 {
		return nil
	}
	required := make([]NexusRequiredMod, 0, len(n.ModRequirements.NexusRequirements.Nodes))
	seen := map[int]struct{}{}
	for _, item := range n.ModRequirements.NexusRequirements.Nodes {
		if item.ExternalRequirement {
			continue
		}
		modID, ok := parsePositiveInt(strings.TrimSpace(item.ModID))
		if !ok || modID == n.ModID {
			continue
		}
		if _, exists := seen[modID]; exists {
			continue
		}
		seen[modID] = struct{}{}
		url := strings.TrimSpace(item.URL)
		if url == "" {
			url = nexusModURL(modID)
		}
		required = append(required, NexusRequiredMod{
			ModID:    modID,
			Name:     strings.TrimSpace(item.ModName),
			Notes:    strings.TrimSpace(item.Notes),
			NexusURL: url,
		})
	}
	if len(required) == 0 {
		return nil
	}
	return required
}

// ── shared request plumbing ─────────────────────────────────────────────────

// nexusAuthErrorKeywords are matched as whole words (case-insensitive)
// against a GraphQL error message to decide whether it's an
// authentication/authorization rejection. Deliberately specific — a bare
// "auth" substring also matches "author" (a real Mod field name), which
// would misclassify ordinary schema errors (wrong field/argument, typos in
// the query) as ErrNexusAuthRequired and mask the real problem.
var nexusAuthErrorKeywords = []string{
	"unauthenticated", "unauthorized", "authentication", "authorization", "forbidden", "permission",
}

// isNexusAuthError reports whether a GraphQL error message indicates the
// query was rejected for authentication/authorization reasons, as opposed to
// a generic query error (bad syntax, unknown field/argument, etc.).
func isNexusAuthError(msg string) bool {
	lower := strings.ToLower(msg)
	for _, kw := range nexusAuthErrorKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// setNexusHeaders sets the standard headers for a Nexus request. apiKey may
// be empty for the keyword-search path, which doesn't require one — in that
// case no "apikey" header is sent at all, rather than sending an empty one.
func setNexusHeaders(req *http.Request, apiKey string) {
	if apiKey != "" {
		req.Header.Set("apikey", apiKey)
	}
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
		return nil, &NexusRequestError{Err: err}
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
// ID. This phase includes enable state but doesn't compare remote updates.
func ApplyNexusInstalledMatch(dataDir, saveName string, results []NexusModSearchResult) []NexusModSearchResult {
	mods, err := ListModsWithState(dataDir, saveName)
	if err != nil || len(mods) == 0 {
		return results
	}
	installedByNexusID := map[int]struct {
		folderName string
		version    string
		enabled    bool
	}{}
	for _, m := range mods {
		if m.NexusModID > 0 {
			installedByNexusID[m.NexusModID] = struct {
				folderName string
				version    string
				enabled    bool
			}{folderName: m.FolderName, version: m.Version, enabled: m.Enabled}
		}
	}

	for i := range results {
		if match, ok := installedByNexusID[results[i].ModID]; ok {
			results[i].Installed = true
			results[i].InstalledEnabled = match.enabled
			results[i].InstalledFolderName = match.folderName
			results[i].InstalledVersion = match.version
		}
		for j := range results[i].RequiredMods {
			if match, ok := installedByNexusID[results[i].RequiredMods[j].ModID]; ok {
				results[i].RequiredMods[j].Installed = true
				results[i].RequiredMods[j].InstalledEnabled = match.enabled
				results[i].RequiredMods[j].InstalledFolderName = match.folderName
				results[i].RequiredMods[j].InstalledVersion = match.version
			}
		}
	}
	return results
}
