package stardew_junimo

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	playerModContextsSchemaVersion = 1
	maxPlayerModContextsBytes      = 4 * 1024 * 1024
	maxPlayerModContextPlayers     = 512
	maxPlayerReportedMods          = 1024
	maxPlayerMultiplayerIDChars    = 32
	maxPlayerModUniqueIDChars      = 256
	maxPlayerModNameChars          = 256
	maxPlayerModVersionChars       = 64
	maxServerLoadedMods            = 2048

	PlayerModContextReported    = "reported"
	PlayerModContextPending     = "pending"
	PlayerModContextUnavailable = "unavailable"
	PlayerModContextStale       = "stale"

	PlayerModResultMatch             = "match"
	PlayerModResultMissingOnClient   = "missing_on_client"
	PlayerModResultClientOnly        = "client_only"
	PlayerModResultVersionMismatch   = "version_mismatch"
	PlayerModComparisonUnavailable   = "unavailable"
	playerModComparisonAvailable     = "available"
	playerModSMAPIUniqueID           = "Pathoschild.SMAPI"
	playerModRiskCJB                 = "cjb"
	playerModContextFileName         = "player-mod-contexts.json"
	playerModUnavailableNotReported  = "context_not_reported"
	playerModUnavailableFileInvalid  = "context_file_invalid"
	playerModUnavailableNotRunning   = "server_not_running"
	playerModUnavailableServerAbsent = "server_context_unavailable"
)

var cjbUniqueIDs = map[string]struct{}{
	"cjbok.cheatsmenu":  {},
	"cjbok.itemspawner": {},
}

var playerModComparisonIgnoredUniqueIDs = map[string]struct{}{
	strings.ToLower(playerModSMAPIUniqueID):  {},
	strings.ToLower(controlModUniqueID):      {},
	strings.ToLower(junimoServerModUniqueID): {},
}

type controlPlayerModContextsFile struct {
	SchemaVersion int                                `json:"schemaVersion"`
	UpdatedAt     time.Time                          `json:"updatedAt"`
	Players       map[string]controlPlayerModContext `json:"players"`
}

type controlPlayerModContext struct {
	UniqueMultiplayerID string              `json:"uniqueMultiplayerId"`
	HasSmapi            bool                `json:"hasSmapi"`
	GameVersion         string              `json:"gameVersion"`
	APIVersion          string              `json:"apiVersion"`
	Mods                []PlayerReportedMod `json:"mods"`
	ContextStatus       string              `json:"contextStatus"`
	ReportedAt          *time.Time          `json:"reportedAt"`
	UpdatedAt           time.Time           `json:"updatedAt"`
}

// PlayerReportedMod is one bounded entry received through SMAPI's peer context.
type PlayerReportedMod struct {
	UniqueID string `json:"uniqueId"`
	Name     string `json:"name"`
	Version  string `json:"version"`
}

// PlayerServerMod is an actually loaded server mod enriched with panel metadata.
type PlayerServerMod struct {
	UniqueID string `json:"uniqueId"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	SyncKind string `json:"syncKind"`
	Enabled  bool   `json:"enabled"`
	BuiltIn  bool   `json:"builtIn,omitempty"`
}

// PlayerModServerContext identifies the SMAPI runtime snapshot used as the
// authoritative server side of a player comparison.
type PlayerModServerContext struct {
	GameVersion string            `json:"gameVersion,omitempty"`
	APIVersion  string            `json:"apiVersion,omitempty"`
	GeneratedAt string            `json:"generatedAt"`
	LoadedMods  []PlayerServerMod `json:"loadedMods"`
}

// PlayerModComparisonItem describes one comparable user-facing UniqueID.
// Panel-owned runtime components are excluded from comparison items.
type PlayerModComparisonItem struct {
	Result        string   `json:"result"`
	UniqueID      string   `json:"uniqueId"`
	Name          string   `json:"name"`
	ServerVersion string   `json:"serverVersion,omitempty"`
	ClientVersion string   `json:"clientVersion,omitempty"`
	SyncKind      string   `json:"syncKind,omitempty"`
	RiskFlags     []string `json:"riskFlags"`
}

type PlayerModComparisonSummary struct {
	Match           int `json:"match"`
	MissingOnClient int `json:"missingOnClient"`
	ClientOnly      int `json:"clientOnly"`
	VersionMismatch int `json:"versionMismatch"`
}

type PlayerModComparison struct {
	Status            string                     `json:"status"`
	UnavailableReason string                     `json:"unavailableReason,omitempty"`
	Items             []PlayerModComparisonItem  `json:"items"`
	Summary           PlayerModComparisonSummary `json:"summary"`
}

// PlayerModDetailsResult is returned by GET .../players/:id/mods.
type PlayerModDetailsResult struct {
	InstanceID          string                  `json:"instanceId"`
	UniqueMultiplayerID string                  `json:"uniqueMultiplayerId"`
	HasSmapi            bool                    `json:"hasSmapi"`
	GameVersion         string                  `json:"gameVersion,omitempty"`
	APIVersion          string                  `json:"apiVersion,omitempty"`
	Mods                []PlayerReportedMod     `json:"mods"`
	ContextStatus       string                  `json:"contextStatus"`
	ReportedAt          *string                 `json:"reportedAt"`
	UpdatedAt           string                  `json:"updatedAt,omitempty"`
	ServerContext       *PlayerModServerContext `json:"serverContext"`
	Comparison          PlayerModComparison     `json:"comparison"`
	RiskFlags           []string                `json:"riskFlags"`
	Message             string                  `json:"message,omitempty"`
}

type playerModRuntimeOptions struct {
	SchemaVersion int                `json:"schemaVersion"`
	Source        string             `json:"source"`
	GeneratedAt   time.Time          `json:"generatedAt"`
	GameVersion   string             `json:"gameVersion"`
	APIVersion    string             `json:"apiVersion"`
	LoadedMods    []runtimeLoadedMod `json:"loadedMods"`
}

func playerModContextsPath(dataDir string) string {
	return filepath.Join(controlDir(dataDir), playerModContextFileName)
}

// GetPlayerModDetails compares one player's last SMAPI peer context with the
// server process's actual options.json.loadedMods snapshot. It is read-only and
// never performs a moderation action.
func (d *Driver) GetPlayerModDetails(ctx context.Context, instance registry.Instance, uniqueMultiplayerID string) (*PlayerModDetailsResult, error) {
	playerID := normalizePlayerMultiplayerID(uniqueMultiplayerID)
	if playerID == "" {
		return nil, &CommandError{Code: "invalid_player", Message: "无效的玩家标识"}
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	result := &PlayerModDetailsResult{
		InstanceID:          instance.ID,
		UniqueMultiplayerID: playerID,
		Mods:                nil,
		ContextStatus:       PlayerModContextUnavailable,
		Comparison: PlayerModComparison{
			Status:            PlayerModComparisonUnavailable,
			UnavailableReason: playerModUnavailableNotReported,
			Items:             []PlayerModComparisonItem{},
		},
		RiskFlags: []string{},
	}

	serverContext, serverMods, serverReason := readPlayerModServerContext(instance.DataDir)
	if serverContext != nil {
		result.ServerContext = serverContext
	}

	clientContext, found, clientReason := readPlayerModContext(instance.DataDir, playerID)
	if found {
		result.HasSmapi = clientContext.HasSmapi
		result.GameVersion = clientContext.GameVersion
		result.APIVersion = clientContext.APIVersion
		result.Mods = clientContext.Mods
		result.ContextStatus = clientContext.ContextStatus
		result.ReportedAt = formatPlayerModTime(clientContext.ReportedAt)
		if !clientContext.UpdatedAt.IsZero() {
			result.UpdatedAt = clientContext.UpdatedAt.UTC().Format(time.RFC3339Nano)
		}
		result.RiskFlags = playerModRiskFlags(clientContext.Mods)
	}

	switch {
	case instance.State != storage.InstanceStateRunning:
		result.Comparison.UnavailableReason = playerModUnavailableNotRunning
	case !found:
		result.Comparison.UnavailableReason = clientReason
	case clientContext.ContextStatus != PlayerModContextReported:
		result.Comparison.UnavailableReason = clientContext.ContextStatus
	case !clientContext.HasSmapi || clientContext.Mods == nil || clientContext.APIVersion == "":
		result.Comparison.UnavailableReason = PlayerModContextUnavailable
	case serverContext == nil:
		result.Comparison.UnavailableReason = serverReason
	default:
		result.Comparison = comparePlayerMods(*serverContext, serverMods, clientContext)
	}
	if result.Comparison.Status == PlayerModComparisonUnavailable {
		result.Message = playerModUnavailableMessage(result.Comparison.UnavailableReason)
	} else {
		result.Message = "已使用服务器实际加载的 Mod 清单完成比较"
	}

	return result, nil
}

func readPlayerModContext(dataDir, playerID string) (controlPlayerModContext, bool, string) {
	file, found, reason := readPlayerModContextsFile(dataDir)
	if !found {
		return controlPlayerModContext{}, false, reason
	}
	entry, ok := file.Players[playerID]
	if !ok {
		return controlPlayerModContext{}, false, playerModUnavailableNotReported
	}
	return normalizeControlPlayerModContext(entry, playerID)
}

func readPlayerModContextsFile(dataDir string) (controlPlayerModContextsFile, bool, string) {
	path := playerModContextsPath(dataDir)
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return controlPlayerModContextsFile{}, false, playerModUnavailableNotReported
	}
	if err != nil || !stat.Mode().IsRegular() || stat.Size() > maxPlayerModContextsBytes {
		return controlPlayerModContextsFile{}, false, playerModUnavailableFileInvalid
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return controlPlayerModContextsFile{}, false, playerModUnavailableFileInvalid
	}
	var file controlPlayerModContextsFile
	if err := json.Unmarshal(data, &file); err != nil || file.SchemaVersion != playerModContextsSchemaVersion || len(file.Players) > maxPlayerModContextPlayers {
		return controlPlayerModContextsFile{}, false, playerModUnavailableFileInvalid
	}
	return file, true, ""
}

func normalizeControlPlayerModContext(entry controlPlayerModContext, playerID string) (controlPlayerModContext, bool, string) {
	if normalizePlayerMultiplayerID(entry.UniqueMultiplayerID) != playerID || !validPlayerModContextStatus(entry.ContextStatus) {
		return controlPlayerModContext{}, false, playerModUnavailableFileInvalid
	}
	if len(entry.Mods) > maxPlayerReportedMods {
		return controlPlayerModContext{}, false, playerModUnavailableFileInvalid
	}
	entry.GameVersion = normalizePlayerModText(entry.GameVersion, maxPlayerModVersionChars)
	entry.APIVersion = normalizePlayerModText(entry.APIVersion, maxPlayerModVersionChars)
	if entry.Mods != nil {
		entry.Mods = normalizePlayerReportedMods(entry.Mods)
	}
	if (entry.ContextStatus == PlayerModContextPending || entry.ContextStatus == PlayerModContextUnavailable) && entry.Mods != nil {
		return controlPlayerModContext{}, false, playerModUnavailableFileInvalid
	}
	if entry.ContextStatus == PlayerModContextReported && (!entry.HasSmapi || entry.Mods == nil || entry.ReportedAt == nil) {
		return controlPlayerModContext{}, false, playerModUnavailableFileInvalid
	}
	return entry, true, ""
}

func markPlayerModRiskFlags(dataDir string, players []PlayerInfo) {
	for i := range players {
		players[i].ModRiskFlags = nil
	}
	file, found, _ := readPlayerModContextsFile(dataDir)
	if !found {
		return
	}
	for i := range players {
		playerID := normalizePlayerMultiplayerID(players[i].UniqueMultiplayerID)
		if playerID == "" {
			continue
		}
		entry, ok := file.Players[playerID]
		if !ok {
			continue
		}
		context, valid, _ := normalizeControlPlayerModContext(entry, playerID)
		if !valid {
			continue
		}
		players[i].ModRiskFlags = playerModRiskFlags(context.Mods)
	}
}

func readPlayerModServerContext(dataDir string) (*PlayerModServerContext, map[string]PlayerServerMod, string) {
	path := runtimeOptionsPath(dataDir)
	stat, err := os.Stat(path)
	if err != nil || !stat.Mode().IsRegular() || stat.Size() > maxRuntimeOptionsBytes {
		return nil, nil, playerModUnavailableServerAbsent
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, playerModUnavailableServerAbsent
	}
	var options playerModRuntimeOptions
	if err := json.Unmarshal(data, &options); err != nil ||
		options.SchemaVersion < runtimeFarmCatalogSchemaVersion ||
		options.Source != "smapi-runtime" || options.GeneratedAt.IsZero() ||
		len(options.LoadedMods) > maxServerLoadedMods ||
		normalizePlayerModText(options.APIVersion, maxPlayerModVersionChars) == "" {
		return nil, nil, playerModUnavailableServerAbsent
	}

	metadataByID := map[string]registry.ModInfo{}
	if physical, err := listPhysicalMods(dataDir); err == nil {
		physical = ApplyModSyncClassification(dataDir, physical)
		for _, mod := range physical {
			key := strings.ToLower(strings.TrimSpace(mod.UniqueID))
			if key == "" {
				continue
			}
			if _, exists := metadataByID[key]; !exists {
				metadataByID[key] = mod
			}
		}
	}

	loadedByID := map[string]PlayerServerMod{}
	for _, raw := range options.LoadedMods {
		uniqueID := normalizePlayerModText(raw.UniqueID, maxPlayerModUniqueIDChars)
		if uniqueID == "" || playerModComparisonIgnores(uniqueID) {
			continue
		}
		key := strings.ToLower(uniqueID)
		if _, exists := loadedByID[key]; exists {
			continue
		}
		serverMod := PlayerServerMod{
			UniqueID: uniqueID,
			Name:     uniqueID,
			Version:  normalizePlayerModText(raw.Version, maxPlayerModVersionChars),
			SyncKind: registry.ModSyncKindUnknown,
			Enabled:  true,
		}
		if metadata, ok := metadataByID[key]; ok {
			if name := normalizePlayerModText(metadata.Name, maxPlayerModNameChars); name != "" {
				serverMod.Name = name
			}
			if registry.ValidModSyncKind(metadata.SyncKind) {
				serverMod.SyncKind = metadata.SyncKind
			}
			serverMod.BuiltIn = metadata.BuiltIn
		}
		loadedByID[key] = serverMod
	}

	loaded := make([]PlayerServerMod, 0, len(loadedByID))
	for _, mod := range loadedByID {
		loaded = append(loaded, mod)
	}
	sort.Slice(loaded, func(i, j int) bool {
		return strings.ToLower(loaded[i].UniqueID) < strings.ToLower(loaded[j].UniqueID)
	})
	context := &PlayerModServerContext{
		GameVersion: normalizePlayerModText(options.GameVersion, maxPlayerModVersionChars),
		APIVersion:  normalizePlayerModText(options.APIVersion, maxPlayerModVersionChars),
		GeneratedAt: options.GeneratedAt.UTC().Format(time.RFC3339Nano),
		LoadedMods:  loaded,
	}
	return context, loadedByID, ""
}

func comparePlayerMods(_ PlayerModServerContext, serverMods map[string]PlayerServerMod, client controlPlayerModContext) PlayerModComparison {
	comparison := PlayerModComparison{
		Status: playerModComparisonAvailable,
		Items:  []PlayerModComparisonItem{},
	}
	clientByID := map[string]PlayerReportedMod{}
	for _, mod := range client.Mods {
		if playerModComparisonIgnores(mod.UniqueID) {
			continue
		}
		key := strings.ToLower(mod.UniqueID)
		if _, exists := clientByID[key]; !exists {
			clientByID[key] = mod
		}
	}

	for key, serverMod := range serverMods {
		if playerModComparisonIgnores(serverMod.UniqueID) {
			continue
		}
		clientMod, present := clientByID[key]
		if !present {
			if serverMod.Enabled && serverMod.SyncKind == registry.ModSyncKindClientRequired {
				comparison.Items = append(comparison.Items, newPlayerModComparisonItem(PlayerModResultMissingOnClient, serverMod, PlayerReportedMod{}))
			}
			continue
		}
		delete(clientByID, key)
		result := PlayerModResultMatch
		if normalizeRuntimeModVersion(serverMod.Version) != normalizeRuntimeModVersion(clientMod.Version) {
			result = PlayerModResultVersionMismatch
		}
		comparison.Items = append(comparison.Items, newPlayerModComparisonItem(result, serverMod, clientMod))
	}

	for _, clientMod := range clientByID {
		comparison.Items = append(comparison.Items, newPlayerModComparisonItem(PlayerModResultClientOnly, PlayerServerMod{}, clientMod))
	}

	sort.Slice(comparison.Items, func(i, j int) bool {
		left, right := comparison.Items[i], comparison.Items[j]
		if playerModResultPriority(left.Result) != playerModResultPriority(right.Result) {
			return playerModResultPriority(left.Result) < playerModResultPriority(right.Result)
		}
		return strings.ToLower(left.UniqueID) < strings.ToLower(right.UniqueID)
	})
	for _, item := range comparison.Items {
		switch item.Result {
		case PlayerModResultMatch:
			comparison.Summary.Match++
		case PlayerModResultMissingOnClient:
			comparison.Summary.MissingOnClient++
		case PlayerModResultClientOnly:
			comparison.Summary.ClientOnly++
		case PlayerModResultVersionMismatch:
			comparison.Summary.VersionMismatch++
		}
	}
	return comparison
}

func newPlayerModComparisonItem(result string, server PlayerServerMod, client PlayerReportedMod) PlayerModComparisonItem {
	uniqueID := server.UniqueID
	name := server.Name
	if uniqueID == "" {
		uniqueID = client.UniqueID
		name = client.Name
	}
	if name == "" {
		name = uniqueID
	}
	return PlayerModComparisonItem{
		Result:        result,
		UniqueID:      uniqueID,
		Name:          name,
		ServerVersion: server.Version,
		ClientVersion: client.Version,
		SyncKind:      server.SyncKind,
		RiskFlags:     riskFlagsForPlayerMod(client.UniqueID),
	}
}

func normalizePlayerReportedMods(raw []PlayerReportedMod) []PlayerReportedMod {
	seen := make(map[string]struct{}, len(raw))
	mods := make([]PlayerReportedMod, 0, min(len(raw), maxPlayerReportedMods))
	for _, item := range raw {
		uniqueID := normalizePlayerModText(item.UniqueID, maxPlayerModUniqueIDChars)
		key := strings.ToLower(uniqueID)
		if uniqueID == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		mods = append(mods, PlayerReportedMod{
			UniqueID: uniqueID,
			Name:     normalizePlayerModText(item.Name, maxPlayerModNameChars),
			Version:  normalizePlayerModText(item.Version, maxPlayerModVersionChars),
		})
	}
	sort.Slice(mods, func(i, j int) bool {
		return strings.ToLower(mods[i].UniqueID) < strings.ToLower(mods[j].UniqueID)
	})
	return mods
}

func normalizePlayerMultiplayerID(raw string) string {
	value := normalizePlayerModText(raw, maxPlayerMultiplayerIDChars)
	if value == "" {
		return ""
	}
	start := 0
	if value[0] == '-' {
		start = 1
	}
	if start == len(value) {
		return ""
	}
	for i := start; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return ""
		}
	}
	return value
}

func normalizePlayerModText(raw string, maxRunes int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || maxRunes <= 0 {
		return ""
	}
	result := make([]rune, 0, min(len([]rune(raw)), maxRunes))
	for _, r := range raw {
		if unicode.IsControl(r) {
			continue
		}
		if len(result) >= maxRunes {
			break
		}
		result = append(result, r)
	}
	return strings.TrimSpace(string(result))
}

func validPlayerModContextStatus(status string) bool {
	switch status {
	case PlayerModContextReported, PlayerModContextPending, PlayerModContextUnavailable, PlayerModContextStale:
		return true
	default:
		return false
	}
}

func riskFlagsForPlayerMod(uniqueID string) []string {
	if _, ok := cjbUniqueIDs[strings.ToLower(strings.TrimSpace(uniqueID))]; ok {
		return []string{playerModRiskCJB}
	}
	return []string{}
}

func playerModComparisonIgnores(uniqueID string) bool {
	_, ignored := playerModComparisonIgnoredUniqueIDs[strings.ToLower(strings.TrimSpace(uniqueID))]
	return ignored
}

func playerModRiskFlags(mods []PlayerReportedMod) []string {
	for _, mod := range mods {
		if len(riskFlagsForPlayerMod(mod.UniqueID)) > 0 {
			return []string{playerModRiskCJB}
		}
	}
	return []string{}
}

func playerModResultPriority(result string) int {
	switch result {
	case PlayerModResultClientOnly:
		return 0
	case PlayerModResultMissingOnClient:
		return 1
	case PlayerModResultVersionMismatch:
		return 2
	case PlayerModResultMatch:
		return 3
	default:
		return 4
	}
}

func formatPlayerModTime(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339Nano)
	return &formatted
}

func playerModUnavailableMessage(reason string) string {
	switch reason {
	case playerModUnavailableNotRunning:
		return "服务器未运行，无法确认玩家当前 Mod 上下文"
	case PlayerModContextPending:
		return "玩家已连接，正在等待 SMAPI 上下文"
	case PlayerModContextStale:
		return "玩家已断开，最后一次 Mod 上下文已过期"
	case PlayerModContextUnavailable, playerModUnavailableNotReported:
		return "客户端没有提供可比较的 SMAPI Mod 清单"
	case playerModUnavailableFileInvalid:
		return "玩家 Mod 上下文文件无效"
	case playerModUnavailableServerAbsent:
		return "服务器实际加载的 Mod 上下文暂不可用"
	default:
		return fmt.Sprintf("玩家 Mod 比较不可用：%s", reason)
	}
}
