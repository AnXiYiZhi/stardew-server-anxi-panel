package stardew_junimo

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type saveRosterSnapshot struct {
	items     []playerCacheItem
	farmhands []saveRosterFarmer
}

func readSaveRosterSnapshot(path string) (*saveRosterSnapshot, error) {
	file, err := os.Open(path)
	if err != nil {
		return &saveRosterSnapshot{}, err
	}
	defer file.Close()
	var parsed saveRosterXML
	if err := xml.NewDecoder(file).Decode(&parsed); err != nil {
		return &saveRosterSnapshot{}, err
	}
	result := &saveRosterSnapshot{farmhands: parsed.Farmhands}
	if item, ok := saveRosterFarmerItem(parsed.Player, true); ok {
		result.items = append(result.items, item)
	}
	for _, farmer := range parsed.Farmhands {
		if item, ok := saveRosterFarmerItem(farmer, false); ok {
			result.items = append(result.items, item)
		}
	}
	return result, nil
}

func rosterSavePath(dataDir, saveID string) string {
	folder := resolveRosterSaveFolder(dataDir, saveID)
	if folder == "" {
		return ""
	}
	return filepath.Join(folder, filepath.Base(folder))
}

func (d *Driver) cachedSaveRoster(ctx context.Context, instance registry.Instance, saveID string) *saveRosterSnapshot {
	path := rosterSavePath(instance.DataDir, saveID)
	if path == "" {
		return &saveRosterSnapshot{}
	}
	key := fileReadKey(path)
	value, err := d.saveRosterReads.get(ctx, instance.ID, key, 24*time.Hour, func(context.Context) (*saveRosterSnapshot, error) {
		result, err := readSaveRosterSnapshot(path)
		if err == nil && key != fileReadKey(path) {
			err = errors.New("save changed while reading roster")
		}
		return result, err
	})
	if err != nil {
		return &saveRosterSnapshot{}
	}
	return value
}

func playerReadKey(instance registry.Instance) string {
	return playerPersistenceKey(instance) + fileReadKey(playerControlFilePath(instance.DataDir),
		filepath.Join(controlDir(instance.DataDir), "player-mod-contexts.json"))
}

func playerPersistenceKey(instance registry.Instance) string {
	// A Control timestamp is a new observation, not a world mutation. Keep
	// the captured coherent snapshot while checking save/config/lifecycle identity.
	saveID := latestControlSaveID(instance.DataDir)
	return instance.State + "\x00" + instance.UpdatedAt + "\x00" + saveID + fileReadKey(
		rosterSavePath(instance.DataDir, saveID),
		serverSettingsPath(instance.DataDir), filepath.Join(instance.DataDir, ".env"))
}

func (d *Driver) cachedLiveServerMaxPlayers(ctx context.Context, instance registry.Instance) *int {
	key := instance.State + instance.UpdatedAt + fileReadKey(serverSettingsPath(instance.DataDir), filepath.Join(instance.DataDir, ".env"))
	value, err := d.playerLimitReads.get(ctx, instance.ID, key, 30*time.Second, func(ctx context.Context) (*int, error) {
		value := readLiveServerMaxPlayers(ctx, d, instance)
		if value == nil {
			return nil, errors.New("live player limit unavailable")
		}
		return value, nil
	})
	if err != nil {
		return nil
	}
	return value
}

func playerControlFilePath(dataDir string) string {
	return filepath.Join(controlDir(dataDir), "players.json")
}

func (d *Driver) finishPlayerRead(ctx context.Context, instance registry.Instance, result *PlayersResult, ownershipHeld ...bool) (*PlayersResult, error) {
	save := d.cachedSaveRoster(ctx, instance, result.SaveID)
	pending := d.pendingCharacterIDs(ctx, instance, result.SaveID, save)
	persist := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if result.readKey != playerPersistenceKey(instance) {
			return errors.New("player inputs changed while reading")
		}
		// Recheck durable instance state after slow external reads. A completed
		// lifecycle operation must not be overwritten by an old online snapshot.
		if store, ok := d.store.(interface {
			GetInstance(context.Context, string) (storage.Instance, error)
		}); ok {
			current, err := store.GetInstance(ctx, instance.ID)
			if err != nil {
				return err
			}
			if result.observedInstanceState != "" && current.State != result.observedInstanceState {
				return errors.New("instance changed while reading players")
			}
			if result.observedInstanceUpdatedAt != "" && current.UpdatedAt != result.observedInstanceUpdatedAt {
				return errors.New("instance generation changed while reading players")
			}
		}
		if _, durable := d.store.(playerRosterStore); !durable {
			if instance.State != storage.InstanceStateRunning {
				result.Players = markCachedPlayersOffline(instance.DataDir, result.SaveID, result.UpdatedAt, true)
			} else {
				online := make([]PlayerInfo, 0, len(result.Players))
				for _, player := range result.Players {
					if player.Status == "online" {
						online = append(online, player)
					}
				}
				result.Players = mergePlayerRoster(instance.DataDir, result.SaveID, online, result.UpdatedAt, true, save)
			}
			result.RecentEvents = recentPlayerEvents(instance.DataDir, result.SaveID)
		}
		result = d.persistPlayerRoster(ctx, instance, result, save, pending)
		return nil
	}
	if len(ownershipHeld) > 0 && ownershipHeld[0] {
		err := persist()
		markPlayerModRiskFlags(instance.DataDir, result.Players)
		return result, err
	}
	err := d.WithMutationOwnership(ctx, instance, persist)
	if err == nil {
		markPlayerModRiskFlags(instance.DataDir, result.Players)
	}
	return result, err
}

func clonePlayersResult(result *PlayersResult) *PlayersResult {
	if result == nil {
		return nil
	}
	// Player lists are small, unlike the world XML; each caller owns its slices
	// and pointer fields and cannot mutate another browser's cached result.
	raw, _ := json.Marshal(result)
	var clone PlayersResult
	_ = json.Unmarshal(raw, &clone)
	return &clone
}
