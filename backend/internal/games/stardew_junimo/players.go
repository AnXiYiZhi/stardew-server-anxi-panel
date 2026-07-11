package stardew_junimo

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// PlayerInfo describes one player from the best available Stardew runtime source.
type PlayerInfo struct {
	Name                string `json:"name"`
	Role                string `json:"role,omitempty"`
	Location            string `json:"location,omitempty"`
	LocationName        string `json:"locationName,omitempty"`
	LocationDisplayName string `json:"locationDisplayName,omitempty"`
	TileX               *int   `json:"tileX,omitempty"`
	TileY               *int   `json:"tileY,omitempty"`
	PixelX              *int   `json:"pixelX,omitempty"`
	PixelY              *int   `json:"pixelY,omitempty"`
	OnlineFor           string `json:"onlineFor,omitempty"`
	Status              string `json:"status"`
	Source              string `json:"source"`
	UniqueMultiplayerID string `json:"uniqueMultiplayerId,omitempty"`
	IsHost              bool   `json:"isHost,omitempty"`
	Money               *int64 `json:"money,omitempty"`
	FarmIncome          *int64 `json:"farmIncome,omitempty"`
	PersonalIncome      *int64 `json:"personalIncome,omitempty"`
	TotalMoneyEarned    *int64 `json:"totalMoneyEarned,omitempty"`
	WalletMode          string `json:"walletMode,omitempty"`
	LastSeen            string `json:"lastSeen,omitempty"`
	// IsAuthenticated reflects JunimoServer's password-protection state for this
	// player (nil when the control mod doesn't support it or the query failed,
	// which is distinct from "known to be unauthenticated"). Always nil/omitted
	// for offline players; see playerInfoFromCacheItem.
	IsAuthenticated *bool `json:"isAuthenticated,omitempty"`
}

// PlayerEvent is a compact player activity entry derived from roster changes.
type PlayerEvent struct {
	ID                  string `json:"id"`
	Type                string `json:"type"`
	PlayerName          string `json:"playerName"`
	UniqueMultiplayerID string `json:"uniqueMultiplayerId,omitempty"`
	IsHost              bool   `json:"isHost,omitempty"`
	Location            string `json:"location,omitempty"`
	LocationName        string `json:"locationName,omitempty"`
	LocationDisplayName string `json:"locationDisplayName,omitempty"`
	SaveID              string `json:"saveId,omitempty"`
	At                  string `json:"at"`
	Message             string `json:"message"`
}

// PlayersResult is returned by the player status endpoint.
type PlayersResult struct {
	InstanceID   string        `json:"instanceId"`
	State        string        `json:"state"`
	Source       string        `json:"source,omitempty"`
	SaveID       string        `json:"saveId,omitempty"`
	OnlineCount  *int          `json:"onlineCount"`
	MaxPlayers   *int          `json:"maxPlayers"`
	Players      []PlayerInfo  `json:"players"`
	RecentEvents []PlayerEvent `json:"recentEvents,omitempty"`
	RawInfo      string        `json:"rawInfo,omitempty"`
	ParseStatus  string        `json:"parseStatus"`
	Message      string        `json:"message,omitempty"`
	UpdatedAt    string        `json:"updatedAt"`
}

type playerRosterStore interface {
	UpsertPlayerRoster(context.Context, storage.UpsertPlayerRosterParams) error
	ListPlayerRoster(context.Context, string, string) ([]storage.PlayerRosterEntry, error)
	MarkPlayerRosterOfflineExcept(context.Context, string, string, string, []string) error
	ImportPlayerRosterEvents(context.Context, []storage.PlayerRosterEvent) error
	ListPlayerRosterEvents(context.Context, string, string, int) ([]storage.PlayerRosterEvent, error)
}

// ListPlayers returns the best available online-player snapshot for a running
// JunimoServer instance. The StardewAnxiPanel.Control SMAPI mod writes a
// structured players.json file in the mounted control directory; older instances
// without that bridge fall back to the conservative Junimo "info" parser.
func (d *Driver) ListPlayers(ctx context.Context, instance registry.Instance) (*PlayersResult, error) {
	_, durableRoster := d.store.(playerRosterStore)
	result := &PlayersResult{
		InstanceID:  instance.ID,
		State:       instance.State,
		Players:     []PlayerInfo{},
		ParseStatus: "unavailable",
		UpdatedAt:   time.Now().Format(time.RFC3339),
	}
	// 当前存档的人数上限来自新建存档时写入的 server-settings.json，
	// 运行时来源（junimo info）解析出的值仍会覆盖这里的兜底。
	result.MaxPlayers = readServerMaxPlayers(instance.DataDir)
	zero := 0
	if instance.State != storage.InstanceStateRunning {
		result.OnlineCount = &zero
		saveID := latestControlSaveID(instance.DataDir)
		result.SaveID = saveID
		if cached := markCachedPlayersOffline(instance.DataDir, saveID, result.UpdatedAt, !durableRoster); len(cached) > 0 {
			result.Source = "panel_cache"
			result.Players = cached
			result.RecentEvents = recentPlayerEvents(instance.DataDir, saveID)
			result.ParseStatus = "partial"
			result.Message = "服务器未运行，显示已记录玩家名册。"
			return d.persistPlayerRoster(ctx, instance, result), nil
		}
		result.RecentEvents = recentPlayerEvents(instance.DataDir, saveID)
		result.Message = "服务器未运行，暂无已记录玩家。"
		return d.persistPlayerRoster(ctx, instance, result), nil
	}

	if snapshot, ok := readPlayersFromControl(instance.DataDir); ok {
		roster := mergePlayerRoster(instance.DataDir, snapshot.SaveID, snapshot.Players, snapshot.UpdatedAt, !durableRoster)
		result.Source = "smapi_control"
		result.SaveID = snapshot.SaveID
		result.OnlineCount = snapshot.OnlineCount
		result.Players = roster
		result.RecentEvents = recentPlayerEvents(instance.DataDir, snapshot.SaveID)
		result.ParseStatus = "exact"
		result.Message = "已从 StardewAnxiPanel.Control players.json 读取当前在线快照，并合并玩家名册。"
		result.UpdatedAt = snapshot.UpdatedAt
		return d.persistPlayerRoster(ctx, instance, result), nil
	}

	info, err := runCommand(ctx, d, instance, CommandRequest{Command: "info"}, true)
	if err != nil {
		return nil, err
	}
	result.Source = "junimo_info"
	raw := strings.TrimSpace(info.Output)
	if raw == "" {
		raw = strings.TrimSpace(info.Error)
	}
	result.RawInfo = raw

	parsed := ParsePlayersFromInfo(raw)
	result.OnlineCount = parsed.OnlineCount
	if parsed.MaxPlayers != nil {
		result.MaxPlayers = parsed.MaxPlayers
	}
	if len(parsed.Players) > 0 {
		result.Players = mergePlayerRoster(instance.DataDir, "", parsed.Players, result.UpdatedAt, !durableRoster)
		result.RecentEvents = recentPlayerEvents(instance.DataDir, "")
	} else {
		result.Players = parsed.Players
		result.RecentEvents = recentPlayerEvents(instance.DataDir, "")
	}
	result.ParseStatus = parsed.ParseStatus
	result.Message = parsed.Message
	return d.persistPlayerRoster(ctx, instance, result), nil
}

// persistPlayerRoster makes SQLite the durable history while keeping runtime
// JSON and save XML as observation sources. Storage failures are deliberately
// non-fatal: player status remains available even when persistence is degraded.
func (d *Driver) persistPlayerRoster(ctx context.Context, instance registry.Instance, result *PlayersResult) *PlayersResult {
	store, ok := d.store.(playerRosterStore)
	if !ok || result == nil {
		return result
	}
	stableID, baseID, fullID := resolveStableSaveIdentity(instance.DataDir, result.SaveID)
	if stableID == "" {
		return result
	}
	result.SaveID = stableID
	persisted, err := store.ListPlayerRoster(ctx, instance.ID, stableID)
	if err != nil {
		d.logger.Warn("list durable player roster", "instance_id", instance.ID, "save_id", stableID, "error", err)
		return result
	}
	byKey := make(map[string]storage.PlayerRosterEntry, len(persisted))
	for _, entry := range persisted {
		byKey[playerKey(entry.DisplayName, entry.PlayerID)] = entry
	}
	legacyByKey := map[string]playerCacheItem{}
	legacy := readPlayerCache(instance.DataDir)
	if cacheMatchesSave(legacy.SaveID, result.SaveID) {
		for _, item := range legacy.Players {
			legacyByKey[playerKey(item.Name, item.UniqueMultiplayerID)] = item
		}
	}
	seen := make(map[string]bool, len(result.Players))
	onlineIDs := []string{}
	allPersisted := true
	for i := range result.Players {
		player := &result.Players[i]
		id := strings.TrimSpace(player.UniqueMultiplayerID)
		if id == "" {
			id = "name:" + strings.ToLower(strings.TrimSpace(player.Name))
		}
		if player.Status == "online" {
			onlineIDs = append(onlineIDs, id)
		}
		key := playerKey(player.Name, id)
		seen[key] = true
		if old, exists := byKey[key]; exists && player.Status != "online" {
			mergeStoredPlayerFallback(player, old)
		}
		entry := storage.PlayerRosterEntry{
			InstanceID: instance.ID, StableSaveID: stableID, PlayerID: id, DisplayName: player.Name,
			Role: player.Role, IsHost: player.IsHost, LastSeenAt: player.LastSeen,
			Location: player.Location, LocationName: player.LocationName, LocationDisplayName: player.LocationDisplayName,
			TileX: player.TileX, TileY: player.TileY, PixelX: player.PixelX, PixelY: player.PixelY,
			Money: player.Money, FarmIncome: player.FarmIncome, PersonalIncome: player.PersonalIncome,
			TotalMoneyEarned: player.TotalMoneyEarned, WalletMode: player.WalletMode,
			SnapshotSource: player.Source, SnapshotObservedAt: result.UpdatedAt,
		}
		if legacyItem, exists := legacyByKey[playerKey(player.Name, player.UniqueMultiplayerID)]; exists {
			entry.FirstSeenAt = legacyItem.FirstSeen
			entry.LastOnlineAt = legacyItem.LastSeen
		}
		if entry.LastSeenAt == "" {
			entry.LastSeenAt = result.UpdatedAt
		}
		if err := store.UpsertPlayerRoster(ctx, storage.UpsertPlayerRosterParams{Entry: entry, BaseSaveID: baseID, FullSaveID: fullID, Online: player.Status == "online"}); err != nil {
			allPersisted = false
			d.logger.Warn("upsert durable player roster", "instance_id", instance.ID, "save_id", stableID, "player_id", id, "error", err)
		}
	}
	if err := store.MarkPlayerRosterOfflineExcept(ctx, instance.ID, stableID, result.UpdatedAt, onlineIDs); err != nil {
		allPersisted = false
		d.logger.Warn("mark durable player roster offline", "instance_id", instance.ID, "save_id", stableID, "error", err)
	}
	legacyEvents := readPlayerEventsFile(instance.DataDir)
	if cacheMatchesSave(legacyEvents.SaveID, stableID) && len(legacyEvents.Events) > 0 {
		imports := make([]storage.PlayerRosterEvent, 0, len(legacyEvents.Events))
		for _, event := range legacyEvents.Events {
			playerID := strings.TrimSpace(event.UniqueMultiplayerID)
			if playerID == "" {
				playerID = "name:" + strings.ToLower(strings.TrimSpace(event.PlayerName))
			}
			imports = append(imports, storage.PlayerRosterEvent{ID: event.ID, InstanceID: instance.ID, StableSaveID: stableID, Type: event.Type, PlayerID: playerID, PlayerName: event.PlayerName, IsHost: event.IsHost, Location: event.Location, LocationName: event.LocationName, LocationDisplayName: event.LocationDisplayName, OccurredAt: event.At})
		}
		if err := store.ImportPlayerRosterEvents(ctx, imports); err != nil {
			allPersisted = false
			d.logger.Warn("import legacy player events", "instance_id", instance.ID, "save_id", stableID, "error", err)
		}
	}
	for _, entry := range persisted {
		key := playerKey(entry.DisplayName, entry.PlayerID)
		if seen[key] {
			continue
		}
		result.Players = append(result.Players, playerInfoFromRosterEntry(entry))
	}
	sortPlayers(result.Players)
	if events, err := store.ListPlayerRosterEvents(ctx, instance.ID, stableID, maxPlayerEvents); err != nil {
		allPersisted = false
		d.logger.Warn("list durable player events", "instance_id", instance.ID, "save_id", stableID, "error", err)
	} else {
		result.RecentEvents = playerEventsFromRoster(events)
	}
	if allPersisted {
		// players-cache.json was the old historical database. Once its contents
		// have been durably observed, stop treating it as persistent state.
		_ = os.Remove(playerCachePath(instance.DataDir))
		_ = os.Remove(playerEventsPath(instance.DataDir))
	}
	return result
}

func playerEventsFromRoster(events []storage.PlayerRosterEvent) []PlayerEvent {
	result := make([]PlayerEvent, 0, len(events))
	for _, event := range events {
		uniqueID := event.PlayerID
		if strings.HasPrefix(uniqueID, "name:") {
			uniqueID = ""
		}
		item := playerCacheItem{Name: event.PlayerName, UniqueMultiplayerID: uniqueID, IsHost: event.IsHost, Location: event.Location, LocationName: event.LocationName, LocationDisplayName: event.LocationDisplayName}
		converted := newPlayerEvent(event.Type, event.StableSaveID, event.OccurredAt, item)
		converted.ID = event.ID
		result = append(result, converted)
	}
	return result
}

func resolveStableSaveIdentity(dataDir, observedID string) (stableID, baseID, fullID string) {
	observedID = strings.TrimSpace(observedID)
	if folder := resolveRosterSaveFolder(dataDir, observedID); folder != "" {
		fullID = filepath.Base(folder)
	}
	if fullID == "" {
		active := strings.TrimSpace(GetActiveSaveName(dataDir))
		if active != "" && (observedID == "" || sameSaveIdentity(active, observedID)) {
			fullID = active
		}
	}
	stableID = fullID
	if stableID == "" {
		stableID = observedID
	}
	baseID = stableID
	if idx := strings.LastIndex(stableID, "_"); idx > 0 {
		suffix := stableID[idx+1:]
		if suffix != "" && saveFolderHasBaseID(stableID, stableID[:idx]) {
			baseID = stableID[:idx]
		}
	}
	return stableID, baseID, fullID
}

func mergeStoredPlayerFallback(player *PlayerInfo, old storage.PlayerRosterEntry) {
	if player.Location == "" || (player.Source == "save_file" && old.SnapshotSource != "save_file") {
		player.Location = old.Location
		player.LocationName = old.LocationName
		player.LocationDisplayName = old.LocationDisplayName
		player.TileX = old.TileX
		player.TileY = old.TileY
		player.PixelX = old.PixelX
		player.PixelY = old.PixelY
	}
	if player.Money == nil {
		player.Money = old.Money
	}
	if player.FarmIncome == nil {
		player.FarmIncome = old.FarmIncome
	}
	if player.PersonalIncome == nil {
		player.PersonalIncome = old.PersonalIncome
	}
	if player.TotalMoneyEarned == nil {
		player.TotalMoneyEarned = old.TotalMoneyEarned
	}
	if player.WalletMode == "" {
		player.WalletMode = old.WalletMode
	}
	if player.LastSeen == "" {
		player.LastSeen = old.LastOnlineAt
		if player.LastSeen == "" {
			player.LastSeen = old.LastSeenAt
		}
	}
	if player.Source == "save_file" && old.SnapshotSource != "" {
		player.Source = old.SnapshotSource
	}
}

func playerInfoFromRosterEntry(entry storage.PlayerRosterEntry) PlayerInfo {
	uniqueID := entry.PlayerID
	if strings.HasPrefix(uniqueID, "name:") {
		uniqueID = ""
	}
	return PlayerInfo{Name: entry.DisplayName, Role: normalizePlayerRole(entry.Role, entry.IsHost), Location: entry.Location,
		LocationName: entry.LocationName, LocationDisplayName: entry.LocationDisplayName, TileX: entry.TileX, TileY: entry.TileY,
		PixelX: entry.PixelX, PixelY: entry.PixelY, Status: "offline", Source: "sqlite_roster",
		UniqueMultiplayerID: uniqueID, IsHost: entry.IsHost, Money: entry.Money,
		FarmIncome: entry.FarmIncome, PersonalIncome: entry.PersonalIncome, TotalMoneyEarned: entry.TotalMoneyEarned,
		WalletMode: entry.WalletMode, LastSeen: entry.LastOnlineAt}
}

// readServerMaxPlayers reads Server.MaxPlayers from the instance's
// server-settings.json. Returns nil when the file is missing, unreadable, or
// the value is not a positive number.
func readServerMaxPlayers(dataDir string) *int {
	data, err := os.ReadFile(serverSettingsPath(dataDir))
	if err != nil {
		return nil
	}
	var parsed struct {
		Server struct {
			MaxPlayers *int `json:"MaxPlayers"`
		} `json:"Server"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil
	}
	if parsed.Server.MaxPlayers == nil || *parsed.Server.MaxPlayers <= 0 {
		return nil
	}
	return parsed.Server.MaxPlayers
}

type playerCacheFile struct {
	SaveID    string            `json:"saveId,omitempty"`
	UpdatedAt string            `json:"updatedAt"`
	Players   []playerCacheItem `json:"players"`
}

type playerCacheItem struct {
	Name                string `json:"name"`
	Role                string `json:"role,omitempty"`
	Location            string `json:"location,omitempty"`
	LocationName        string `json:"locationName,omitempty"`
	LocationDisplayName string `json:"locationDisplayName,omitempty"`
	TileX               *int   `json:"tileX,omitempty"`
	TileY               *int   `json:"tileY,omitempty"`
	PixelX              *int   `json:"pixelX,omitempty"`
	PixelY              *int   `json:"pixelY,omitempty"`
	Source              string `json:"source,omitempty"`
	UniqueMultiplayerID string `json:"uniqueMultiplayerId,omitempty"`
	IsHost              bool   `json:"isHost,omitempty"`
	Money               *int64 `json:"money,omitempty"`
	FarmIncome          *int64 `json:"farmIncome,omitempty"`
	PersonalIncome      *int64 `json:"personalIncome,omitempty"`
	TotalMoneyEarned    *int64 `json:"totalMoneyEarned,omitempty"`
	WalletMode          string `json:"walletMode,omitempty"`
	Status              string `json:"status,omitempty"`
	FirstSeen           string `json:"firstSeen,omitempty"`
	LastSeen            string `json:"lastSeen,omitempty"`
	IsAuthenticated     *bool  `json:"isAuthenticated,omitempty"`
}

type playerEventsFile struct {
	SaveID    string        `json:"saveId,omitempty"`
	UpdatedAt string        `json:"updatedAt"`
	Events    []PlayerEvent `json:"events"`
}

func playerCachePath(dataDir string) string {
	return filepath.Join(controlDir(dataDir), "players-cache.json")
}

func playerEventsPath(dataDir string) string {
	return filepath.Join(controlDir(dataDir), "players-events.json")
}

func offlinePlayersFromCache(dataDir, saveID string) []PlayerInfo {
	cache := readPlayerCache(dataDir)
	byKey := map[string]playerCacheItem{}
	if cacheMatchesSave(cache.SaveID, saveID) {
		for _, item := range cache.Players {
			key := playerKey(item.Name, item.UniqueMultiplayerID)
			if key == "" {
				continue
			}
			byKey[key] = item
		}
	}
	for _, item := range saveRosterItems(dataDir, saveID) {
		key := playerKey(item.Name, item.UniqueMultiplayerID)
		if key == "" {
			continue
		}
		if existing, ok := byKey[key]; ok {
			byKey[key] = mergePlayerCacheFallback(existing, item)
		} else {
			byKey[key] = item
		}
	}
	players := make([]PlayerInfo, 0, len(byKey))
	for _, item := range byKey {
		players = append(players, playerInfoFromCacheItem(item, "offline", offlineRosterSource(item)))
	}
	sortPlayers(players)
	return players
}

func markCachedPlayersOffline(dataDir, saveID, seenAt string, writeLegacyHistory bool) []PlayerInfo {
	if strings.TrimSpace(seenAt) == "" {
		seenAt = time.Now().Format(time.RFC3339)
	}
	cache := readPlayerCache(dataDir)
	if !cacheMatchesSave(cache.SaveID, saveID) {
		return offlinePlayersFromCache(dataDir, saveID)
	}
	changed := false
	events := []PlayerEvent{}
	for i := range cache.Players {
		if strings.EqualFold(strings.TrimSpace(cache.Players[i].Status), "online") {
			cache.Players[i].Status = "offline"
			cache.Players[i].LastSeen = seenAt
			events = append(events, newPlayerEvent("left", saveID, seenAt, cache.Players[i]))
			changed = true
		}
	}
	if changed && writeLegacyHistory {
		cache.UpdatedAt = seenAt
		_ = writePlayerCache(dataDir, cache)
		appendPlayerEvents(dataDir, saveID, events, seenAt)
	}
	return offlinePlayersFromCache(dataDir, saveID)
}

func mergePlayerRoster(dataDir, saveID string, onlinePlayers []PlayerInfo, seenAt string, writeLegacyHistory bool) []PlayerInfo {
	if strings.TrimSpace(seenAt) == "" {
		seenAt = time.Now().Format(time.RFC3339)
	}

	cache := readPlayerCache(dataDir)
	byKey := map[string]playerCacheItem{}
	if cacheMatchesSave(cache.SaveID, saveID) {
		for _, item := range cache.Players {
			key := playerKey(item.Name, item.UniqueMultiplayerID)
			if key == "" {
				continue
			}
			byKey[key] = item
		}
	}
	for _, item := range saveRosterItems(dataDir, saveID) {
		key := playerKey(item.Name, item.UniqueMultiplayerID)
		if key == "" {
			continue
		}
		if existing, ok := byKey[key]; ok {
			byKey[key] = mergePlayerCacheFallback(existing, item)
		} else {
			byKey[key] = item
		}
	}

	onlineKeys := map[string]bool{}
	events := []PlayerEvent{}
	for _, player := range onlinePlayers {
		name := strings.TrimSpace(player.Name)
		if name == "" {
			continue
		}
		key := playerKey(name, player.UniqueMultiplayerID)
		onlineKeys[key] = true

		item := byKey[key]
		previousStatus := strings.TrimSpace(item.Status)
		previousSource := strings.TrimSpace(item.Source)
		isNewPlayer := playerKey(item.Name, item.UniqueMultiplayerID) == ""
		if strings.TrimSpace(item.FirstSeen) == "" {
			item.FirstSeen = seenAt
		}
		item.Name = name
		item.Role = normalizePlayerRole(player.Role, player.IsHost)
		item.Location = player.Location
		item.LocationName = player.LocationName
		item.LocationDisplayName = player.LocationDisplayName
		item.TileX = player.TileX
		item.TileY = player.TileY
		item.PixelX = player.PixelX
		item.PixelY = player.PixelY
		item.Source = player.Source
		item.UniqueMultiplayerID = player.UniqueMultiplayerID
		item.IsHost = player.IsHost
		item.Money = player.Money
		item.FarmIncome = player.FarmIncome
		item.PersonalIncome = player.PersonalIncome
		item.TotalMoneyEarned = player.TotalMoneyEarned
		item.WalletMode = player.WalletMode
		item.IsAuthenticated = player.IsAuthenticated
		item.Status = "online"
		item.LastSeen = seenAt
		switch {
		case isNewPlayer:
			events = append(events, newPlayerEvent("seen", saveID, seenAt, item))
		case strings.EqualFold(previousStatus, "offline") && !strings.EqualFold(previousSource, "save_file"):
			events = append(events, newPlayerEvent("joined", saveID, seenAt, item))
		}
		byKey[key] = item
	}

	cachePlayers := make([]playerCacheItem, 0, len(byKey))
	roster := make([]PlayerInfo, 0, len(byKey))
	for key, item := range byKey {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		status := "offline"
		source := offlineRosterSource(item)
		if onlineKeys[key] {
			status = "online"
			source = item.Source
			if source == "" {
				source = "smapi_control"
			}
		} else if strings.EqualFold(strings.TrimSpace(item.Status), "online") {
			events = append(events, newPlayerEvent("left", saveID, seenAt, item))
			item.Status = "offline"
			item.LastSeen = seenAt
		}
		cachePlayers = append(cachePlayers, item)
		roster = append(roster, playerInfoFromCacheItem(item, status, source))
	}

	sort.Slice(cachePlayers, func(i, j int) bool {
		return strings.ToLower(cachePlayers[i].Name) < strings.ToLower(cachePlayers[j].Name)
	})
	if writeLegacyHistory {
		_ = writePlayerCache(dataDir, playerCacheFile{
			SaveID:    strings.TrimSpace(saveID),
			UpdatedAt: seenAt,
			Players:   cachePlayers,
		})
		appendPlayerEvents(dataDir, saveID, events, seenAt)
	}

	sortPlayers(roster)
	return roster
}

func playerInfoFromCacheItem(item playerCacheItem, status, source string) PlayerInfo {
	name := strings.TrimSpace(item.Name)
	farmIncome, personalIncome := normalizeCachedIncome(item.FarmIncome, item.PersonalIncome, item.TotalMoneyEarned, item.WalletMode)
	// "Pending auth" is a transient online-only concept; never surface a stale
	// authenticated/unauthenticated value for a player that is currently offline.
	isAuthenticated := item.IsAuthenticated
	if status != "online" {
		isAuthenticated = nil
	}
	return PlayerInfo{
		Name:                name,
		Role:                normalizePlayerRole(item.Role, item.IsHost),
		Location:            item.Location,
		LocationName:        item.LocationName,
		LocationDisplayName: item.LocationDisplayName,
		TileX:               item.TileX,
		TileY:               item.TileY,
		PixelX:              item.PixelX,
		PixelY:              item.PixelY,
		Status:              status,
		Source:              source,
		UniqueMultiplayerID: item.UniqueMultiplayerID,
		IsHost:              item.IsHost,
		Money:               item.Money,
		FarmIncome:          farmIncome,
		PersonalIncome:      personalIncome,
		TotalMoneyEarned:    item.TotalMoneyEarned,
		WalletMode:          item.WalletMode,
		LastSeen:            item.LastSeen,
		IsAuthenticated:     isAuthenticated,
	}
}

func offlineRosterSource(item playerCacheItem) string {
	if strings.EqualFold(strings.TrimSpace(item.Source), "save_file") {
		return "save_file"
	}
	return "panel_cache"
}

type saveRosterXML struct {
	XMLName   xml.Name           `xml:"SaveGame"`
	Player    saveRosterFarmer   `xml:"player"`
	Farmhands []saveRosterFarmer `xml:"farmhands>Farmer"`
}

type saveRosterFarmer struct {
	Name                        string `xml:"name"`
	UniqueMultiplayerID         string `xml:"UniqueMultiplayerID"`
	UniqueMultiplayerIDFallback string `xml:"uniqueMultiplayerID"`
	Money                       *int64 `xml:"money"`
	TotalMoneyEarned            *int64 `xml:"totalMoneyEarned"`
	HomeLocation                string `xml:"homeLocation"`
	LastSleepLocation           string `xml:"lastSleepLocation"`
	LastSleepPoint              struct {
		X *int `xml:"X"`
		Y *int `xml:"Y"`
	} `xml:"lastSleepPoint"`
	UseSeparateWallets bool `xml:"useSeparateWallets"`
}

func saveRosterItems(dataDir, saveID string) []playerCacheItem {
	saveFolder := resolveRosterSaveFolder(dataDir, saveID)
	if saveFolder == "" {
		return []playerCacheItem{}
	}
	saveName := filepath.Base(saveFolder)
	raw, err := os.ReadFile(filepath.Join(saveFolder, saveName))
	if err != nil || len(raw) == 0 {
		return []playerCacheItem{}
	}
	var parsed saveRosterXML
	if err := xml.Unmarshal(raw, &parsed); err != nil || parsed.XMLName.Local != "SaveGame" {
		return []playerCacheItem{}
	}

	items := make([]playerCacheItem, 0, 1+len(parsed.Farmhands))
	if item, ok := saveRosterFarmerItem(parsed.Player, true); ok {
		items = append(items, item)
	}
	for _, farmer := range parsed.Farmhands {
		if item, ok := saveRosterFarmerItem(farmer, false); ok {
			items = append(items, item)
		}
	}
	return items
}

func saveRosterFarmerItem(farmer saveRosterFarmer, isHost bool) (playerCacheItem, bool) {
	name := strings.TrimSpace(farmer.Name)
	if name == "" {
		return playerCacheItem{}, false
	}
	uniqueID := strings.TrimSpace(farmer.UniqueMultiplayerID)
	if uniqueID == "" {
		uniqueID = strings.TrimSpace(farmer.UniqueMultiplayerIDFallback)
	}
	role := "player"
	if isHost {
		role = "host"
	}
	location := strings.TrimSpace(farmer.LastSleepLocation)
	if location == "" {
		location = strings.TrimSpace(farmer.HomeLocation)
	}
	walletMode := "shared"
	var personalIncome *int64
	if farmer.UseSeparateWallets {
		walletMode = "separate"
		personalIncome = farmer.TotalMoneyEarned
	}
	return playerCacheItem{
		Name:                name,
		Role:                role,
		Location:            location,
		LocationName:        location,
		TileX:               farmer.LastSleepPoint.X,
		TileY:               farmer.LastSleepPoint.Y,
		Source:              "save_file",
		UniqueMultiplayerID: uniqueID,
		IsHost:              isHost,
		Money:               farmer.Money,
		FarmIncome:          farmer.TotalMoneyEarned,
		PersonalIncome:      personalIncome,
		TotalMoneyEarned:    farmer.TotalMoneyEarned,
		WalletMode:          walletMode,
		Status:              "offline",
	}, true
}

// mergePlayerCacheFallback fills fields that the runtime cache has never seen
// from the save file without replacing more precise last-observed runtime data.
func mergePlayerCacheFallback(current, fallback playerCacheItem) playerCacheItem {
	if current.Location == "" {
		current.Location = fallback.Location
	}
	if current.LocationName == "" {
		current.LocationName = fallback.LocationName
	}
	if current.LocationDisplayName == "" {
		current.LocationDisplayName = fallback.LocationDisplayName
	}
	if current.TileX == nil {
		current.TileX = fallback.TileX
	}
	if current.TileY == nil {
		current.TileY = fallback.TileY
	}
	if current.Money == nil {
		current.Money = fallback.Money
	}
	if current.FarmIncome == nil {
		current.FarmIncome = fallback.FarmIncome
	}
	if current.PersonalIncome == nil {
		current.PersonalIncome = fallback.PersonalIncome
	}
	if current.TotalMoneyEarned == nil {
		current.TotalMoneyEarned = fallback.TotalMoneyEarned
	}
	if current.WalletMode == "" {
		current.WalletMode = fallback.WalletMode
	}
	return current
}

func resolveRosterSaveFolder(dataDir, saveID string) string {
	activeSave := strings.TrimSpace(GetActiveSaveName(dataDir))
	if activeSave != "" {
		if folder := findRosterSaveFolder(dataDir, activeSave); folder != "" {
			return folder
		}
	}
	return findRosterSaveFolder(dataDir, saveID)
}

func findRosterSaveFolder(dataDir, saveID string) string {
	saveID = strings.TrimSpace(saveID)
	if saveID == "" {
		return ""
	}
	dirs, err := listSaveDirs(dataDir)
	if err != nil || len(dirs) == 0 {
		return ""
	}
	sort.Strings(dirs)
	for _, name := range dirs {
		if name == saveID {
			return filepath.Join(savesDir(dataDir), "Saves", name)
		}
	}
	for _, name := range dirs {
		if strings.HasPrefix(name, saveID+"_") {
			return filepath.Join(savesDir(dataDir), "Saves", name)
		}
	}
	return ""
}

func cacheMatchesSave(cacheSaveID, currentSaveID string) bool {
	cacheSaveID = strings.TrimSpace(cacheSaveID)
	currentSaveID = strings.TrimSpace(currentSaveID)
	if currentSaveID == "" {
		return true
	}
	if cacheSaveID == "" {
		return false
	}
	return sameSaveIdentity(cacheSaveID, currentSaveID)
}

func sameSaveIdentity(left, right string) bool {
	if left == right {
		return true
	}
	return saveFolderHasBaseID(left, right) || saveFolderHasBaseID(right, left)
}

func saveFolderHasBaseID(folderID, baseID string) bool {
	if baseID == "" || !strings.HasPrefix(folderID, baseID+"_") {
		return false
	}
	suffix := strings.TrimPrefix(folderID, baseID+"_")
	if suffix == "" {
		return false
	}
	for _, ch := range suffix {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func normalizeCachedIncome(farmIncome, personalIncome, totalMoneyEarned *int64, walletMode string) (*int64, *int64) {
	if farmIncome == nil {
		farmIncome = totalMoneyEarned
	}
	if personalIncome == nil && strings.EqualFold(strings.TrimSpace(walletMode), "separate") {
		personalIncome = totalMoneyEarned
	}
	return farmIncome, personalIncome
}

const maxPlayerEvents = 50

func newPlayerEvent(eventType, saveID, at string, player playerCacheItem) PlayerEvent {
	name := strings.TrimSpace(player.Name)
	message := ""
	switch eventType {
	case "joined":
		message = fmt.Sprintf("%s 加入了服务器。", name)
	case "left":
		message = fmt.Sprintf("%s 离开了服务器。", name)
	default:
		eventType = "seen"
		message = fmt.Sprintf("首次记录玩家 %s 在线。", name)
	}
	key := playerKey(name, player.UniqueMultiplayerID)
	if key == "" {
		key = "name:" + strings.ToLower(name)
	}
	return PlayerEvent{
		ID:                  fmt.Sprintf("%s:%s:%s", eventType, key, strings.TrimSpace(at)),
		Type:                eventType,
		PlayerName:          name,
		UniqueMultiplayerID: strings.TrimSpace(player.UniqueMultiplayerID),
		IsHost:              player.IsHost,
		Location:            strings.TrimSpace(player.Location),
		LocationName:        strings.TrimSpace(player.LocationName),
		LocationDisplayName: strings.TrimSpace(player.LocationDisplayName),
		SaveID:              strings.TrimSpace(saveID),
		At:                  strings.TrimSpace(at),
		Message:             message,
	}
}

func recentPlayerEvents(dataDir, saveID string) []PlayerEvent {
	eventsFile := readPlayerEventsFile(dataDir)
	if !eventLogMatchesSave(eventsFile.SaveID, saveID) {
		return []PlayerEvent{}
	}
	events := make([]PlayerEvent, 0, len(eventsFile.Events))
	for i := len(eventsFile.Events) - 1; i >= 0; i-- {
		event := eventsFile.Events[i]
		if strings.TrimSpace(event.PlayerName) == "" || strings.TrimSpace(event.At) == "" {
			continue
		}
		events = append(events, event)
	}
	return events
}

func appendPlayerEvents(dataDir, saveID string, events []PlayerEvent, updatedAt string) {
	if len(events) == 0 {
		return
	}
	cleaned := make([]PlayerEvent, 0, len(events))
	for _, event := range events {
		if strings.TrimSpace(event.PlayerName) == "" || strings.TrimSpace(event.At) == "" {
			continue
		}
		cleaned = append(cleaned, event)
	}
	if len(cleaned) == 0 {
		return
	}

	eventsFile := readPlayerEventsFile(dataDir)
	if !eventLogMatchesSave(eventsFile.SaveID, saveID) {
		eventsFile = playerEventsFile{}
	}
	eventsFile.SaveID = strings.TrimSpace(saveID)
	eventsFile.UpdatedAt = strings.TrimSpace(updatedAt)
	eventsFile.Events = append(eventsFile.Events, cleaned...)
	if len(eventsFile.Events) > maxPlayerEvents {
		eventsFile.Events = eventsFile.Events[len(eventsFile.Events)-maxPlayerEvents:]
	}
	_ = writePlayerEventsFile(dataDir, eventsFile)
}

func eventLogMatchesSave(logSaveID, currentSaveID string) bool {
	logSaveID = strings.TrimSpace(logSaveID)
	currentSaveID = strings.TrimSpace(currentSaveID)
	if currentSaveID == "" {
		return logSaveID == ""
	}
	if logSaveID == "" {
		return false
	}
	return logSaveID == currentSaveID
}

func readPlayerEventsFile(dataDir string) playerEventsFile {
	raw, err := os.ReadFile(playerEventsPath(dataDir))
	if err != nil {
		return playerEventsFile{}
	}
	var events playerEventsFile
	if err := json.Unmarshal(raw, &events); err != nil {
		return playerEventsFile{}
	}
	return events
}

func writePlayerEventsFile(dataDir string, events playerEventsFile) error {
	if err := os.MkdirAll(controlDir(dataDir), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(playerEventsPath(dataDir), raw, 0o644)
}

func latestControlSaveID(dataDir string) string {
	if snapshot, ok := readPlayersFromControl(dataDir); ok {
		return snapshot.SaveID
	}
	raw, err := os.ReadFile(filepath.Join(controlDir(dataDir), "status.json"))
	if err != nil {
		return ""
	}
	var status struct {
		SaveID string `json:"saveId"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		return ""
	}
	return strings.TrimSpace(status.SaveID)
}

func readPlayerCache(dataDir string) playerCacheFile {
	raw, err := os.ReadFile(playerCachePath(dataDir))
	if err != nil {
		return playerCacheFile{}
	}
	var cache playerCacheFile
	if err := json.Unmarshal(raw, &cache); err != nil {
		return playerCacheFile{}
	}
	return cache
}

func writePlayerCache(dataDir string, cache playerCacheFile) error {
	if err := os.MkdirAll(controlDir(dataDir), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(playerCachePath(dataDir), raw, 0o644)
}

func playerKey(name, uniqueID string) string {
	if strings.TrimSpace(uniqueID) != "" {
		return "id:" + strings.TrimSpace(uniqueID)
	}
	if strings.TrimSpace(name) != "" {
		return "name:" + strings.ToLower(strings.TrimSpace(name))
	}
	return ""
}

func normalizePlayerRole(role string, isHost bool) string {
	if isHost {
		return "host"
	}
	role = strings.TrimSpace(role)
	if role == "" {
		return "player"
	}
	return role
}

func sortPlayers(players []PlayerInfo) {
	sort.SliceStable(players, func(i, j int) bool {
		if players[i].Status != players[j].Status {
			return players[i].Status == "online"
		}
		if players[i].IsHost != players[j].IsHost {
			return players[i].IsHost
		}
		return strings.ToLower(players[i].Name) < strings.ToLower(players[j].Name)
	})
}

type controlPlayersSnapshot struct {
	UpdatedAt   string
	SaveID      string
	OnlineCount *int
	Players     []PlayerInfo
}

type controlPlayersFile struct {
	UpdatedAt string `json:"updatedAt"`
	SaveID    string `json:"saveId"`
	Players   []struct {
		Name                string `json:"name"`
		UniqueMultiplayerID string `json:"uniqueMultiplayerId"`
		IsHost              bool   `json:"isHost"`
		Location            string `json:"location"`
		LocationName        string `json:"locationName"`
		LocationDisplayName string `json:"locationDisplayName"`
		TileX               *int   `json:"tileX"`
		TileY               *int   `json:"tileY"`
		PixelX              *int   `json:"pixelX"`
		PixelY              *int   `json:"pixelY"`
		Money               *int64 `json:"money"`
		FarmIncome          *int64 `json:"farmIncome"`
		PersonalIncome      *int64 `json:"personalIncome"`
		TotalMoneyEarned    *int64 `json:"totalMoneyEarned"`
		WalletMode          string `json:"walletMode"`
		IsAuthenticated     *bool  `json:"isAuthenticated"`
	} `json:"players"`
}

func readPlayersFromControl(dataDir string) (controlPlayersSnapshot, bool) {
	raw, err := os.ReadFile(filepath.Join(controlDir(dataDir), "players.json"))
	if err != nil {
		return controlPlayersSnapshot{}, false
	}

	var file controlPlayersFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return controlPlayersSnapshot{}, false
	}

	players := make([]PlayerInfo, 0, len(file.Players))
	seen := map[string]bool{}
	for _, player := range file.Players {
		name := strings.TrimSpace(player.Name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if player.UniqueMultiplayerID != "" {
			key = player.UniqueMultiplayerID
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		role := "player"
		if player.IsHost {
			role = "host"
		}
		farmIncome := player.FarmIncome
		if farmIncome == nil {
			farmIncome = player.TotalMoneyEarned
		}
		personalIncome := player.PersonalIncome
		if personalIncome == nil && strings.EqualFold(strings.TrimSpace(player.WalletMode), "separate") {
			personalIncome = player.TotalMoneyEarned
		}
		players = append(players, PlayerInfo{
			Name:                name,
			Role:                role,
			Location:            strings.TrimSpace(player.Location),
			LocationName:        strings.TrimSpace(player.LocationName),
			LocationDisplayName: strings.TrimSpace(player.LocationDisplayName),
			TileX:               player.TileX,
			TileY:               player.TileY,
			PixelX:              player.PixelX,
			PixelY:              player.PixelY,
			Status:              "online",
			Source:              "smapi_control",
			UniqueMultiplayerID: player.UniqueMultiplayerID,
			IsHost:              player.IsHost,
			Money:               player.Money,
			FarmIncome:          farmIncome,
			PersonalIncome:      personalIncome,
			TotalMoneyEarned:    player.TotalMoneyEarned,
			WalletMode:          strings.TrimSpace(player.WalletMode),
			IsAuthenticated:     player.IsAuthenticated,
		})
	}

	count := len(players)
	updatedAt := strings.TrimSpace(file.UpdatedAt)
	if updatedAt == "" {
		updatedAt = time.Now().Format(time.RFC3339)
	}
	return controlPlayersSnapshot{
		UpdatedAt:   updatedAt,
		SaveID:      strings.TrimSpace(file.SaveID),
		OnlineCount: &count,
		Players:     players,
	}, true
}

// ParsedPlayers is the test-friendly output of ParsePlayersFromInfo.
type ParsedPlayers struct {
	OnlineCount *int
	MaxPlayers  *int
	Players     []PlayerInfo
	ParseStatus string
	Message     string
}

var playerCountPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:online\s+players?|connected\s+players?|players?|player\s+count|playercount)\s*[:：]\s*(\d+)\s*/\s*(\d+)`),
	regexp.MustCompile(`(?i)(?:online\s+players?|connected\s+players?|players?|player\s+count|playercount)\s*[:：]\s*(\d+)\s+(?:of|/)\s+(\d+)`),
	regexp.MustCompile(`(?i)(?:在线玩家|玩家数|当前玩家)\s*[:：]\s*(\d+)\s*/\s*(\d+)`),
	regexp.MustCompile(`(?i)(?:online\s+players?|connected\s+players?|players?|player\s+count|playercount|在线玩家|玩家数|当前玩家)\s*[:：]\s*(\d+)\b`),
}

var maxPlayersPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:max\s+players?|maximum\s+players?|player\s+limit|capacity)\s*[:：]\s*(\d+)`),
	regexp.MustCompile(`(?i)(?:最大玩家|玩家上限|最大人数)\s*[:：]\s*(\d+)`),
}

var playerListLinePattern = regexp.MustCompile(`(?i)^\s*(?:online\s+players?|connected\s+players?|players\s+online|player\s+list|players|在线玩家列表|在线玩家|玩家列表)\s*[:：]\s*(.*)$`)

// ParsePlayersFromInfo extracts counts and obvious player names from Junimo's
// "info" output without guessing beyond explicit labels.
func ParsePlayersFromInfo(raw string) ParsedPlayers {
	parsed := ParsedPlayers{
		Players:     []PlayerInfo{},
		ParseStatus: "unavailable",
	}
	raw = strings.TrimSpace(stripControlChars(raw))
	if raw == "" {
		parsed.Message = "Junimo info 未返回内容。"
		return parsed
	}

	for _, pattern := range playerCountPatterns {
		if m := pattern.FindStringSubmatch(raw); len(m) >= 2 {
			if n, err := strconv.Atoi(m[1]); err == nil {
				parsed.OnlineCount = &n
			}
			if len(m) >= 3 {
				if max, err := strconv.Atoi(m[2]); err == nil {
					parsed.MaxPlayers = &max
				}
			}
			break
		}
	}
	if parsed.MaxPlayers == nil {
		for _, pattern := range maxPlayersPatterns {
			if m := pattern.FindStringSubmatch(raw); len(m) >= 2 {
				if max, err := strconv.Atoi(m[1]); err == nil {
					parsed.MaxPlayers = &max
				}
				break
			}
		}
	}

	parsed.Players = parsePlayerNames(raw)
	if parsed.OnlineCount == nil && len(parsed.Players) > 0 {
		count := len(parsed.Players)
		parsed.OnlineCount = &count
	}

	switch {
	case parsed.OnlineCount != nil && len(parsed.Players) > 0:
		parsed.ParseStatus = "exact"
		parsed.Message = "已从 Junimo info 输出解析在线人数和玩家名。"
	case parsed.OnlineCount != nil:
		parsed.ParseStatus = "partial"
		parsed.Message = "已从 Junimo info 输出解析在线人数；当前输出未包含玩家名。"
	case len(parsed.Players) > 0:
		parsed.ParseStatus = "partial"
		parsed.Message = "已从 Junimo info 输出解析玩家名；当前输出未包含人数上限。"
	default:
		parsed.ParseStatus = "partial"
		parsed.Message = "Junimo info 输出暂未包含可识别的在线玩家字段。"
	}
	return parsed
}

func parsePlayerNames(raw string) []PlayerInfo {
	names := make([]string, 0)
	lines := strings.Split(raw, "\n")
	collectingList := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			collectingList = false
			continue
		}
		if m := playerListLinePattern.FindStringSubmatch(trimmed); len(m) == 2 {
			value := strings.TrimSpace(m[1])
			if isCountLike(value) {
				collectingList = false
				continue
			}
			if value == "" {
				collectingList = true
				continue
			}
			names = append(names, splitPlayerNames(value)...)
			collectingList = false
			continue
		}
		if collectingList {
			if looksLikeHeader(trimmed) {
				collectingList = false
				continue
			}
			names = append(names, splitPlayerNames(trimmed)...)
		}
	}

	seen := map[string]bool{}
	players := make([]PlayerInfo, 0, len(names))
	for _, name := range names {
		name = normalizePlayerName(name)
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		players = append(players, PlayerInfo{
			Name:   name,
			Status: "online",
			Source: "junimo_info",
		})
	}
	return players
}

func splitPlayerNames(value string) []string {
	replacer := strings.NewReplacer("、", ",", "，", ",", ";", ",", "|", ",")
	value = replacer.Replace(value)
	parts := strings.Split(value, ",")
	if len(parts) == 1 && strings.Contains(value, " - ") {
		parts = strings.Split(value, " - ")
	}
	return parts
}

func normalizePlayerName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "-")
	name = strings.TrimPrefix(name, "*")
	name = strings.TrimPrefix(name, "•")
	name = strings.TrimSpace(name)
	name = strings.Trim(name, `"'[]()`)
	lower := strings.ToLower(name)
	if name == "" ||
		lower == "none" ||
		lower == "no players" ||
		lower == "n/a" ||
		lower == "null" ||
		name == "无" ||
		name == "暂无" {
		return ""
	}
	if isCountLike(name) {
		return ""
	}
	return name
}

func isCountLike(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return regexp.MustCompile(`^\d+\s*(?:/|of)\s*\d+$`).MatchString(strings.ToLower(value)) ||
		regexp.MustCompile(`^\d+$`).MatchString(value)
}

func looksLikeHeader(line string) bool {
	if strings.Contains(line, ":") || strings.Contains(line, "：") {
		return true
	}
	return strings.HasPrefix(line, "---")
}
