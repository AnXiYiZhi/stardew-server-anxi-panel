package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// PlayerRosterEntry is the durable player identity and latest observed snapshot.
type PlayerRosterEntry struct {
	InstanceID, StableSaveID, PlayerID, DisplayName     string
	Role, Location, LocationName, LocationDisplayName   string
	IsHost                                              bool
	FirstSeenAt, LastSeenAt, LastOnlineAt               string
	TileX, TileY, PixelX, PixelY                        *int
	Money, FarmIncome, PersonalIncome, TotalMoneyEarned *int64
	WalletMode, SnapshotSource, SnapshotObservedAt      string
	CurrentStatus                                       string
}

type PlayerRosterEvent struct {
	ID, InstanceID, StableSaveID, Type, PlayerID, PlayerName string
	IsHost                                                   bool
	Location, LocationName, LocationDisplayName, OccurredAt  string
}

type UpsertPlayerRosterParams struct {
	Entry                  PlayerRosterEntry
	BaseSaveID, FullSaveID string
	Online                 bool
}

func (s *Store) UpsertPlayerRoster(ctx context.Context, params UpsertPlayerRosterParams) error {
	e := params.Entry
	e.InstanceID = strings.TrimSpace(e.InstanceID)
	e.StableSaveID = strings.TrimSpace(e.StableSaveID)
	e.PlayerID = strings.TrimSpace(e.PlayerID)
	e.DisplayName = strings.TrimSpace(e.DisplayName)
	if e.InstanceID == "" || e.StableSaveID == "" || e.PlayerID == "" || e.DisplayName == "" {
		return fmt.Errorf("player roster identity is incomplete")
	}
	observedAt := strings.TrimSpace(e.SnapshotObservedAt)
	if observedAt == "" {
		observedAt = strings.TrimSpace(e.LastSeenAt)
	}
	if observedAt == "" {
		return fmt.Errorf("player roster observed time is empty")
	}
	firstSeenAt := strings.TrimSpace(e.FirstSeenAt)
	if firstSeenAt == "" {
		firstSeenAt = observedAt
	}
	baseID := strings.TrimSpace(params.BaseSaveID)
	if baseID == "" {
		baseID = e.StableSaveID
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin player roster upsert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var previousStatus string
	lookupErr := tx.QueryRowContext(ctx, `SELECT current_status FROM player_roster WHERE instance_id=? AND stable_save_id=? AND player_id=?`, e.InstanceID, e.StableSaveID, e.PlayerID).Scan(&previousStatus)
	isNew := lookupErr == sql.ErrNoRows
	if lookupErr != nil && lookupErr != sql.ErrNoRows {
		return fmt.Errorf("read previous player status: %w", lookupErr)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO save_identities
(instance_id, stable_save_id, base_save_id, full_save_id, first_seen_at, last_seen_at)
VALUES (?, ?, ?, NULLIF(?, ''), ?, ?)
ON CONFLICT(instance_id, stable_save_id) DO UPDATE SET
base_save_id=excluded.base_save_id,
full_save_id=COALESCE(excluded.full_save_id, save_identities.full_save_id),
last_seen_at=excluded.last_seen_at`, e.InstanceID, e.StableSaveID, baseID, strings.TrimSpace(params.FullSaveID), observedAt, observedAt)
	if err != nil {
		return fmt.Errorf("upsert save identity: %w", err)
	}

	// Replace a legacy name identity once the authoritative multiplayer ID arrives.
	if !strings.HasPrefix(e.PlayerID, "name:") {
		_, err = tx.ExecContext(ctx, `DELETE FROM player_roster WHERE instance_id=? AND stable_save_id=? AND player_id LIKE 'name:%' AND lower(display_name)=lower(?)`, e.InstanceID, e.StableSaveID, e.DisplayName)
		if err != nil {
			return fmt.Errorf("merge temporary player identity: %w", err)
		}
	}
	lastOnline := any(rosterNullString(e.LastOnlineAt))
	if params.Online {
		lastOnline = observedAt
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO player_roster (
instance_id, stable_save_id, player_id, display_name, role, is_host, first_seen_at, last_seen_at, last_online_at,
location, location_name, location_display_name, tile_x, tile_y, pixel_x, pixel_y, money, farm_income,
personal_income, total_money_earned, wallet_mode, snapshot_source, snapshot_observed_at, current_status)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(instance_id, stable_save_id, player_id) DO UPDATE SET
display_name=excluded.display_name, role=excluded.role, is_host=excluded.is_host,
last_seen_at=excluded.last_seen_at, last_online_at=COALESCE(excluded.last_online_at, player_roster.last_online_at),
location=COALESCE(excluded.location, player_roster.location), location_name=COALESCE(excluded.location_name, player_roster.location_name),
location_display_name=COALESCE(excluded.location_display_name, player_roster.location_display_name),
tile_x=COALESCE(excluded.tile_x, player_roster.tile_x), tile_y=COALESCE(excluded.tile_y, player_roster.tile_y),
pixel_x=COALESCE(excluded.pixel_x, player_roster.pixel_x), pixel_y=COALESCE(excluded.pixel_y, player_roster.pixel_y),
money=COALESCE(excluded.money, player_roster.money), farm_income=COALESCE(excluded.farm_income, player_roster.farm_income),
personal_income=COALESCE(excluded.personal_income, player_roster.personal_income),
total_money_earned=COALESCE(excluded.total_money_earned, player_roster.total_money_earned),
wallet_mode=COALESCE(excluded.wallet_mode, player_roster.wallet_mode), snapshot_source=excluded.snapshot_source,
snapshot_observed_at=excluded.snapshot_observed_at, current_status=excluded.current_status`,
		e.InstanceID, e.StableSaveID, e.PlayerID, e.DisplayName, rosterNullString(e.Role), boolInt(e.IsHost), firstSeenAt, observedAt, lastOnline,
		rosterNullString(e.Location), rosterNullString(e.LocationName), rosterNullString(e.LocationDisplayName), e.TileX, e.TileY, e.PixelX, e.PixelY,
		e.Money, e.FarmIncome, e.PersonalIncome, e.TotalMoneyEarned, rosterNullString(e.WalletMode), e.SnapshotSource, observedAt, playerStatus(params.Online))
	if err != nil {
		return fmt.Errorf("upsert player roster: %w", err)
	}
	if params.Online && (isNew || previousStatus != "online") {
		eventType := "joined"
		if isNew {
			eventType = "seen"
		}
		if err := insertPlayerEventTx(ctx, tx, PlayerRosterEvent{InstanceID: e.InstanceID, StableSaveID: e.StableSaveID, Type: eventType, PlayerID: e.PlayerID, PlayerName: e.DisplayName, IsHost: e.IsHost, Location: e.Location, LocationName: e.LocationName, LocationDisplayName: e.LocationDisplayName, OccurredAt: observedAt}); err != nil {
			return err
		}
	} else if !params.Online && previousStatus == "online" {
		if err := insertPlayerEventTx(ctx, tx, PlayerRosterEvent{InstanceID: e.InstanceID, StableSaveID: e.StableSaveID, Type: "left", PlayerID: e.PlayerID, PlayerName: e.DisplayName, IsHost: e.IsHost, Location: e.Location, LocationName: e.LocationName, LocationDisplayName: e.LocationDisplayName, OccurredAt: observedAt}); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit player roster upsert: %w", err)
	}
	return nil
}

func (s *Store) ListPlayerRoster(ctx context.Context, instanceID, stableSaveID string) ([]PlayerRosterEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT instance_id, stable_save_id, player_id, display_name,
COALESCE(role,''), is_host, first_seen_at, last_seen_at, COALESCE(last_online_at,''),
COALESCE(location,''), COALESCE(location_name,''), COALESCE(location_display_name,''), tile_x, tile_y, pixel_x, pixel_y,
money, farm_income, personal_income, total_money_earned, COALESCE(wallet_mode,''), COALESCE(snapshot_source,''), snapshot_observed_at, current_status
FROM player_roster WHERE instance_id=? AND stable_save_id=? ORDER BY is_host DESC, lower(display_name)`, instanceID, stableSaveID)
	if err != nil {
		return nil, fmt.Errorf("list player roster: %w", err)
	}
	defer rows.Close()
	var result []PlayerRosterEntry
	for rows.Next() {
		var e PlayerRosterEntry
		var host int
		var tx, ty, px, py sql.NullInt64
		var money, farm, personal, total sql.NullInt64
		if err := rows.Scan(&e.InstanceID, &e.StableSaveID, &e.PlayerID, &e.DisplayName, &e.Role, &host,
			&e.FirstSeenAt, &e.LastSeenAt, &e.LastOnlineAt, &e.Location, &e.LocationName, &e.LocationDisplayName,
			&tx, &ty, &px, &py, &money, &farm, &personal, &total, &e.WalletMode, &e.SnapshotSource, &e.SnapshotObservedAt, &e.CurrentStatus); err != nil {
			return nil, err
		}
		e.IsHost = host != 0
		e.TileX, e.TileY, e.PixelX, e.PixelY = nullableInt(tx), nullableInt(ty), nullableInt(px), nullableInt(py)
		e.Money, e.FarmIncome, e.PersonalIncome, e.TotalMoneyEarned = nullableInt64(money), nullableInt64(farm), nullableInt64(personal), nullableInt64(total)
		result = append(result, e)
	}
	return result, rows.Err()
}

func (s *Store) MarkPlayerRosterOfflineExcept(ctx context.Context, instanceID, stableSaveID, observedAt string, onlinePlayerIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT player_id, display_name, is_host, COALESCE(location,''), COALESCE(location_name,''), COALESCE(location_display_name,'') FROM player_roster WHERE instance_id=? AND stable_save_id=? AND current_status='online'`, instanceID, stableSaveID)
	if err != nil {
		return err
	}
	online := map[string]bool{}
	for _, id := range onlinePlayerIDs {
		online[id] = true
	}
	var left []PlayerRosterEvent
	for rows.Next() {
		var event PlayerRosterEvent
		var host int
		if err := rows.Scan(&event.PlayerID, &event.PlayerName, &host, &event.Location, &event.LocationName, &event.LocationDisplayName); err != nil {
			_ = rows.Close()
			return err
		}
		if online[event.PlayerID] {
			continue
		}
		event.InstanceID, event.StableSaveID, event.Type = instanceID, stableSaveID, "left"
		event.IsHost, event.OccurredAt = host != 0, observedAt
		left = append(left, event)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, event := range left {
		if _, err := tx.ExecContext(ctx, `UPDATE player_roster SET current_status='offline', last_seen_at=? WHERE instance_id=? AND stable_save_id=? AND player_id=?`, observedAt, instanceID, stableSaveID, event.PlayerID); err != nil {
			return err
		}
		if err := insertPlayerEventTx(ctx, tx, event); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ImportPlayerRosterEvents(ctx context.Context, events []PlayerRosterEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, event := range events {
		if err := insertPlayerEventTx(ctx, tx, event); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListPlayerRosterEvents(ctx context.Context, instanceID, stableSaveID string, limit int) ([]PlayerRosterEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, instance_id, stable_save_id, event_type, player_id, player_name, is_host, COALESCE(location,''), COALESCE(location_name,''), COALESCE(location_display_name,''), occurred_at FROM player_events WHERE instance_id=? AND stable_save_id=? ORDER BY occurred_at DESC, id DESC LIMIT ?`, instanceID, stableSaveID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PlayerRosterEvent
	for rows.Next() {
		var event PlayerRosterEvent
		var host int
		if err := rows.Scan(&event.ID, &event.InstanceID, &event.StableSaveID, &event.Type, &event.PlayerID, &event.PlayerName, &host, &event.Location, &event.LocationName, &event.LocationDisplayName, &event.OccurredAt); err != nil {
			return nil, err
		}
		event.IsHost = host != 0
		result = append(result, event)
	}
	return result, rows.Err()
}

func insertPlayerEventTx(ctx context.Context, tx *sql.Tx, event PlayerRosterEvent) error {
	if event.ID == "" {
		event.ID = fmt.Sprintf("%s-%s-%d", event.Type, strings.ReplaceAll(event.PlayerID, ":", "_"), time.Now().UnixNano())
	}
	_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO player_events (id, instance_id, stable_save_id, event_type, player_id, player_name, is_host, location, location_name, location_display_name, occurred_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, event.ID, event.InstanceID, event.StableSaveID, event.Type, event.PlayerID, event.PlayerName, boolInt(event.IsHost), rosterNullString(event.Location), rosterNullString(event.LocationName), rosterNullString(event.LocationDisplayName), event.OccurredAt)
	if err != nil {
		return fmt.Errorf("insert player event: %w", err)
	}
	return nil
}

func playerStatus(online bool) string {
	if online {
		return "online"
	}
	return "offline"
}

func rosterNullString(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
func nullableInt(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}
func nullableInt64(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}
