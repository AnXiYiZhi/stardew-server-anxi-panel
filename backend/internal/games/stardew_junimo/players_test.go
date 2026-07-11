package stardew_junimo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestParsePlayersFromInfo_CountAndNames(t *testing.T) {
	raw := `--- Server Info ---
Players: 2 / 4
Online players: Abigail, Sam
Save: JunimoFarm`

	parsed := ParsePlayersFromInfo(raw)
	if parsed.OnlineCount == nil || *parsed.OnlineCount != 2 {
		t.Fatalf("online count = %#v, want 2", parsed.OnlineCount)
	}
	if parsed.MaxPlayers == nil || *parsed.MaxPlayers != 4 {
		t.Fatalf("max players = %#v, want 4", parsed.MaxPlayers)
	}
	if len(parsed.Players) != 2 || parsed.Players[0].Name != "Abigail" || parsed.Players[1].Name != "Sam" {
		t.Fatalf("players = %+v, want Abigail/Sam", parsed.Players)
	}
	if parsed.ParseStatus != "exact" {
		t.Fatalf("parse status = %q, want exact", parsed.ParseStatus)
	}
}

func TestParsePlayersFromInfo_ChineseCountOnly(t *testing.T) {
	parsed := ParsePlayersFromInfo("在线玩家：1/8\n邀请码：ABCDEF12")
	if parsed.OnlineCount == nil || *parsed.OnlineCount != 1 {
		t.Fatalf("online count = %#v, want 1", parsed.OnlineCount)
	}
	if parsed.MaxPlayers == nil || *parsed.MaxPlayers != 8 {
		t.Fatalf("max players = %#v, want 8", parsed.MaxPlayers)
	}
	if len(parsed.Players) != 0 {
		t.Fatalf("players = %+v, want none when names are absent", parsed.Players)
	}
	if parsed.ParseStatus != "partial" {
		t.Fatalf("parse status = %q, want partial", parsed.ParseStatus)
	}
}

func TestReadServerMaxPlayers(t *testing.T) {
	dir := t.TempDir()
	if got := readServerMaxPlayers(dir); got != nil {
		t.Fatalf("max players without settings = %#v, want nil", got)
	}

	settingsPath := serverSettingsPath(dir)
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"Server":{"MaxPlayers":12}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readServerMaxPlayers(dir); got == nil || *got != 12 {
		t.Fatalf("max players = %#v, want 12", got)
	}

	if err := os.WriteFile(settingsPath, []byte(`{"Server":{"MaxPlayers":0}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readServerMaxPlayers(dir); got != nil {
		t.Fatalf("max players with zero value = %#v, want nil", got)
	}
}

func TestReadPlayersFromControl(t *testing.T) {
	dir := t.TempDir()
	control := filepath.Join(dir, ".local-container", "control")
	if err := os.MkdirAll(control, 0o755); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	raw := `{
  "updatedAt": "2026-06-30T10:49:18.7314829+00:00",
  "saveId": "test",
  "players": [
    {
      "name": "host",
      "uniqueMultiplayerId": "-7928143696348358209",
      "isHost": true,
      "location": "FarmHouse",
      "locationName": "FarmHouse",
      "locationDisplayName": "Farmhouse",
      "tileX": 9,
      "tileY": 12,
      "pixelX": 576,
      "pixelY": 768,
      "money": 12345,
      "farmIncome": 67890,
      "personalIncome": 3456,
      "totalMoneyEarned": 67890,
      "walletMode": "shared"
    },
    {
      "name": "test",
      "uniqueMultiplayerId": "-1800332298401119618",
      "isHost": false,
      "location": "Farm",
      "money": 234,
      "farmIncome": 67890,
      "personalIncome": 567,
      "totalMoneyEarned": 67890,
      "walletMode": "shared"
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(control, "players.json"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write players.json: %v", err)
	}

	snapshot, ok := readPlayersFromControl(dir)
	if !ok {
		t.Fatal("readPlayersFromControl returned !ok")
	}
	if snapshot.SaveID != "test" {
		t.Fatalf("save id = %q, want test", snapshot.SaveID)
	}
	if snapshot.OnlineCount == nil || *snapshot.OnlineCount != 2 {
		t.Fatalf("online count = %#v, want 2", snapshot.OnlineCount)
	}
	if len(snapshot.Players) != 2 {
		t.Fatalf("players len = %d, want 2", len(snapshot.Players))
	}
	if snapshot.Players[0].Name != "host" || !snapshot.Players[0].IsHost || snapshot.Players[0].Role != "host" {
		t.Fatalf("host player = %+v", snapshot.Players[0])
	}
	if snapshot.Players[0].Location != "FarmHouse" {
		t.Fatalf("host location = %q, want FarmHouse", snapshot.Players[0].Location)
	}
	if snapshot.Players[0].LocationName != "FarmHouse" {
		t.Fatalf("host location name = %q, want FarmHouse", snapshot.Players[0].LocationName)
	}
	if snapshot.Players[0].LocationDisplayName != "Farmhouse" {
		t.Fatalf("host location display name = %q, want Farmhouse", snapshot.Players[0].LocationDisplayName)
	}
	if snapshot.Players[0].TileX == nil || *snapshot.Players[0].TileX != 9 {
		t.Fatalf("host tile x = %#v, want 9", snapshot.Players[0].TileX)
	}
	if snapshot.Players[0].TileY == nil || *snapshot.Players[0].TileY != 12 {
		t.Fatalf("host tile y = %#v, want 12", snapshot.Players[0].TileY)
	}
	if snapshot.Players[0].Money == nil || *snapshot.Players[0].Money != 12345 {
		t.Fatalf("host money = %#v, want 12345", snapshot.Players[0].Money)
	}
	if snapshot.Players[0].FarmIncome == nil || *snapshot.Players[0].FarmIncome != 67890 {
		t.Fatalf("host farm income = %#v, want 67890", snapshot.Players[0].FarmIncome)
	}
	if snapshot.Players[0].PersonalIncome == nil || *snapshot.Players[0].PersonalIncome != 3456 {
		t.Fatalf("host personal income = %#v, want 3456", snapshot.Players[0].PersonalIncome)
	}
	if snapshot.Players[0].WalletMode != "shared" {
		t.Fatalf("host wallet mode = %q, want shared", snapshot.Players[0].WalletMode)
	}
	if snapshot.Players[1].Name != "test" || snapshot.Players[1].IsHost || snapshot.Players[1].Role != "player" {
		t.Fatalf("guest player = %+v", snapshot.Players[1])
	}
}

func TestListPlayersPrefersControlFile(t *testing.T) {
	dir := t.TempDir()
	control := filepath.Join(dir, ".local-container", "control")
	if err := os.MkdirAll(control, 0o755); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "players.json"), []byte(`{
  "updatedAt": "2026-06-30T10:49:18.7314829+00:00",
  "saveId": "test",
  "players": [{"name": "host", "uniqueMultiplayerId": "1", "isHost": true, "money": 99, "farmIncome": 1000, "personalIncome": 101, "totalMoneyEarned": 1000, "walletMode": "separate"}]
}`), 0o644); err != nil {
		t.Fatalf("write players.json: %v", err)
	}

	d := newTestDriver(&fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, _ string, _ ...string) (paneldocker.CommandResult, error) {
			t.Fatal("ListPlayers should not call Junimo info when players.json is available")
			return paneldocker.CommandResult{}, nil
		},
	})
	instance := makeRunningInstance()
	instance.DataDir = dir

	result, err := d.ListPlayers(context.Background(), instance)
	if err != nil {
		t.Fatalf("ListPlayers returned error: %v", err)
	}
	if result.Source != "smapi_control" {
		t.Fatalf("source = %q, want smapi_control", result.Source)
	}
	if result.SaveID != "test" {
		t.Fatalf("save id = %q, want test", result.SaveID)
	}
	if result.OnlineCount == nil || *result.OnlineCount != 1 {
		t.Fatalf("online count = %#v, want 1", result.OnlineCount)
	}
	if len(result.Players) != 1 || result.Players[0].Name != "host" || !result.Players[0].IsHost {
		t.Fatalf("players = %+v, want host", result.Players)
	}
	if result.Players[0].Money == nil || *result.Players[0].Money != 99 {
		t.Fatalf("money = %#v, want 99", result.Players[0].Money)
	}
	if result.Players[0].FarmIncome == nil || *result.Players[0].FarmIncome != 1000 {
		t.Fatalf("farm income = %#v, want 1000", result.Players[0].FarmIncome)
	}
	if result.Players[0].PersonalIncome == nil || *result.Players[0].PersonalIncome != 101 {
		t.Fatalf("personal income = %#v, want 101", result.Players[0].PersonalIncome)
	}
	if result.Players[0].WalletMode != "separate" {
		t.Fatalf("wallet mode = %q, want separate", result.Players[0].WalletMode)
	}
}

func TestReadPlayersFromControlBackfillsLegacyIncomeFields(t *testing.T) {
	dir := t.TempDir()
	control := filepath.Join(dir, ".local-container", "control")
	if err := os.MkdirAll(control, 0o755); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "players.json"), []byte(`{
  "updatedAt": "2026-06-30T10:49:18Z",
  "saveId": "test",
  "players": [{"name": "host", "uniqueMultiplayerId": "1", "isHost": true, "totalMoneyEarned": 101, "walletMode": "separate"}]
}`), 0o644); err != nil {
		t.Fatalf("write players.json: %v", err)
	}

	snapshot, ok := readPlayersFromControl(dir)
	if !ok {
		t.Fatal("readPlayersFromControl returned !ok")
	}
	if len(snapshot.Players) != 1 {
		t.Fatalf("players len = %d, want 1", len(snapshot.Players))
	}
	if snapshot.Players[0].FarmIncome == nil || *snapshot.Players[0].FarmIncome != 101 {
		t.Fatalf("legacy farm income = %#v, want 101", snapshot.Players[0].FarmIncome)
	}
	if snapshot.Players[0].PersonalIncome == nil || *snapshot.Players[0].PersonalIncome != 101 {
		t.Fatalf("legacy personal income = %#v, want 101", snapshot.Players[0].PersonalIncome)
	}
}

func TestListPlayersMergesControlSnapshotWithCachedOfflinePlayers(t *testing.T) {
	dir := t.TempDir()
	control := filepath.Join(dir, ".local-container", "control")
	if err := os.MkdirAll(control, 0o755); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "players-cache.json"), []byte(`{
  "saveId": "test",
  "updatedAt": "2026-06-30T10:00:00Z",
  "players": [
    {"name": "host", "uniqueMultiplayerId": "1", "isHost": true, "role": "host", "lastSeen": "2026-06-30T10:00:00Z"},
    {"name": "test", "uniqueMultiplayerId": "2", "isHost": false, "role": "player", "lastSeen": "2026-06-30T09:00:00Z"}
  ]
}`), 0o644); err != nil {
		t.Fatalf("write players-cache.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "players.json"), []byte(`{
  "updatedAt": "2026-06-30T10:49:18Z",
  "saveId": "test",
  "players": [{"name": "host", "uniqueMultiplayerId": "1", "isHost": true}]
}`), 0o644); err != nil {
		t.Fatalf("write players.json: %v", err)
	}

	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()
	instance.DataDir = dir
	result, err := d.ListPlayers(context.Background(), instance)
	if err != nil {
		t.Fatalf("ListPlayers returned error: %v", err)
	}
	if result.OnlineCount == nil || *result.OnlineCount != 1 {
		t.Fatalf("online count = %#v, want 1", result.OnlineCount)
	}
	if len(result.Players) != 2 {
		t.Fatalf("players len = %d, want 2: %+v", len(result.Players), result.Players)
	}
	if result.Players[0].Name != "host" || result.Players[0].Status != "online" {
		t.Fatalf("first player = %+v, want host online", result.Players[0])
	}
	if result.Players[1].Name != "test" || result.Players[1].Status != "offline" {
		t.Fatalf("second player = %+v, want test offline", result.Players[1])
	}
}

func TestListPlayersMergesControlSnapshotWithSaveFarmhands(t *testing.T) {
	dir := t.TempDir()
	control := filepath.Join(dir, ".local-container", "control")
	if err := os.MkdirAll(control, 0o755); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	saveFolder := filepath.Join(dir, ".local-container", "saves", "Saves", "test_123")
	if err := os.MkdirAll(saveFolder, 0o755); err != nil {
		t.Fatalf("mkdir save: %v", err)
	}
	saveXML := `<SaveGame>
  <player>
    <name>host</name>
    <UniqueMultiplayerID>1</UniqueMultiplayerID>
    <money>500</money>
    <totalMoneyEarned>1200</totalMoneyEarned>
  </player>
  <farmhands>
    <Farmer>
      <name>test</name>
      <UniqueMultiplayerID>2</UniqueMultiplayerID>
      <money>50</money>
      <totalMoneyEarned>300</totalMoneyEarned>
	  <homeLocation>Cabin</homeLocation>
	  <lastSleepLocation>FarmHouse</lastSleepLocation>
	  <lastSleepPoint><X>12</X><Y>7</Y></lastSleepPoint>
	  <useSeparateWallets>true</useSeparateWallets>
    </Farmer>
  </farmhands>
</SaveGame>`
	if err := os.WriteFile(filepath.Join(saveFolder, "test_123"), []byte(saveXML), 0o644); err != nil {
		t.Fatalf("write save: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "players.json"), []byte(`{
  "updatedAt": "2026-06-30T10:49:18Z",
  "saveId": "test",
  "players": [{"name": "host", "uniqueMultiplayerId": "1", "isHost": true}]
}`), 0o644); err != nil {
		t.Fatalf("write players.json: %v", err)
	}

	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()
	instance.DataDir = dir
	result, err := d.ListPlayers(context.Background(), instance)
	if err != nil {
		t.Fatalf("ListPlayers returned error: %v", err)
	}
	if result.OnlineCount == nil || *result.OnlineCount != 1 {
		t.Fatalf("online count = %#v, want 1", result.OnlineCount)
	}
	if len(result.Players) != 2 {
		t.Fatalf("players len = %d, want host online and test offline: %+v", len(result.Players), result.Players)
	}
	if result.Players[0].Name != "host" || result.Players[0].Status != "online" || !result.Players[0].IsHost {
		t.Fatalf("first player = %+v, want host online", result.Players[0])
	}
	if result.Players[1].Name != "test" || result.Players[1].Status != "offline" || result.Players[1].Source != "save_file" {
		t.Fatalf("second player = %+v, want save_file test offline", result.Players[1])
	}
	if result.Players[1].FarmIncome == nil || *result.Players[1].FarmIncome != 300 {
		t.Fatalf("test farm income = %#v, want 300", result.Players[1].FarmIncome)
	}
	if result.Players[1].PersonalIncome == nil || *result.Players[1].PersonalIncome != 300 {
		t.Fatalf("test personal income = %#v, want 300", result.Players[1].PersonalIncome)
	}
	if result.Players[1].Location != "FarmHouse" || result.Players[1].TileX == nil || *result.Players[1].TileX != 12 || result.Players[1].TileY == nil || *result.Players[1].TileY != 7 {
		t.Fatalf("test saved location = %+v, want FarmHouse (12, 7)", result.Players[1])
	}
}

func TestCacheMatchesSaveAcceptsFolderSuffixIdentity(t *testing.T) {
	for _, tc := range []struct {
		cacheID string
		saveID  string
		want    bool
	}{
		{cacheID: "Farm", saveID: "Farm", want: true},
		{cacheID: "Farm", saveID: "Farm_442923526", want: true},
		{cacheID: "Farm_442923526", saveID: "Farm", want: true},
		{cacheID: "Farm", saveID: "Farm_backup", want: false},
		{cacheID: "Farm", saveID: "Other_442923526", want: false},
	} {
		if got := cacheMatchesSave(tc.cacheID, tc.saveID); got != tc.want {
			t.Fatalf("cacheMatchesSave(%q, %q) = %v, want %v", tc.cacheID, tc.saveID, got, tc.want)
		}
	}
}

func TestMergePlayerCacheFallbackPreservesRuntimeLocation(t *testing.T) {
	runtimeX, savedX := 65, 12
	current := playerCacheItem{Location: "Farm", LocationName: "Farm", TileX: &runtimeX}
	fallback := playerCacheItem{Location: "FarmHouse", LocationName: "FarmHouse", TileX: &savedX}
	merged := mergePlayerCacheFallback(current, fallback)
	if merged.Location != "Farm" || merged.TileX == nil || *merged.TileX != runtimeX {
		t.Fatalf("runtime location was overwritten by save fallback: %+v", merged)
	}
}

func TestListPlayersMigratesLegacyCacheToSQLiteRoster(t *testing.T) {
	dir := t.TempDir()
	control := filepath.Join(dir, ".local-container", "control")
	saveFolder := filepath.Join(dir, ".local-container", "saves", "Saves", "Farm_123")
	if err := os.MkdirAll(control, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(saveFolder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(control, "players-cache.json"), []byte(`{"saveId":"Farm","updatedAt":"2026-07-11T09:00:00Z","players":[{"name":"Guest","uniqueMultiplayerId":"2","lastSeen":"2026-07-11T09:00:00Z","location":"Town"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(control, "players-events.json"), []byte(`{"saveId":"Farm","updatedAt":"2026-07-11T09:00:00Z","events":[{"id":"legacy-left","type":"left","playerName":"Guest","uniqueMultiplayerId":"2","at":"2026-07-11T09:00:00Z"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(control, "players.json"), []byte(`{"saveId":"Farm","updatedAt":"2026-07-11T10:00:00Z","players":[{"name":"Host","uniqueMultiplayerId":"1","isHost":true,"location":"Farm"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	dbDir := t.TempDir()
	store, err := storage.Open(context.Background(), config.Config{DataDir: dbDir, DBPath: filepath.Join(dbDir, "panel.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	instance := makeRunningInstance()
	instance.ID = "roster-test"
	instance.DataDir = dir
	if _, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{ID: instance.ID, DriverID: storage.DefaultDriverID, Name: "test", DataDir: dir}); err != nil {
		t.Fatal(err)
	}
	d := newTestDriver(&fakeConsoleDocker{})
	d.store = store
	result, err := d.ListPlayers(context.Background(), instance)
	if err != nil {
		t.Fatal(err)
	}
	if result.SaveID != "Farm_123" || len(result.Players) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if _, err := os.Stat(filepath.Join(control, "players-cache.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy cache was not retired: %v", err)
	}
	if _, err := os.Stat(filepath.Join(control, "players-events.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy events were not retired: %v", err)
	}
	rows, err := store.ListPlayerRoster(context.Background(), instance.ID, "Farm_123")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("sqlite roster = %+v, want two players", rows)
	}
	events, err := store.ListPlayerRosterEvents(context.Background(), instance.ID, "Farm_123", 10)
	if err != nil || len(events) < 2 {
		t.Fatalf("sqlite events = %+v, err=%v", events, err)
	}

	if err := os.WriteFile(filepath.Join(control, "players.json"), []byte(`{"saveId":"Farm","updatedAt":"2026-07-11T11:00:00Z","players":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := d.ListPlayers(context.Background(), instance)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Players) != 2 || second.Players[0].Status != "offline" || second.Players[1].Status != "offline" {
		t.Fatalf("durable offline roster = %+v", second.Players)
	}
}

func TestListPlayersRecordsRecentPlayerEvents(t *testing.T) {
	dir := t.TempDir()
	control := filepath.Join(dir, ".local-container", "control")
	if err := os.MkdirAll(control, 0o755); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "players.json"), []byte(`{
  "updatedAt": "2026-06-30T10:00:00Z",
  "saveId": "test",
  "players": [{"name": "host", "uniqueMultiplayerId": "1", "isHost": true, "location": "Farm"}]
}`), 0o644); err != nil {
		t.Fatalf("write players.json: %v", err)
	}

	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()
	instance.DataDir = dir
	first, err := d.ListPlayers(context.Background(), instance)
	if err != nil {
		t.Fatalf("first ListPlayers returned error: %v", err)
	}
	if len(first.RecentEvents) != 1 {
		t.Fatalf("first recent events = %+v, want 1", first.RecentEvents)
	}
	if first.RecentEvents[0].Type != "seen" || first.RecentEvents[0].PlayerName != "host" {
		t.Fatalf("first event = %+v, want seen host", first.RecentEvents[0])
	}

	if err := os.WriteFile(filepath.Join(control, "players.json"), []byte(`{
  "updatedAt": "2026-06-30T10:05:00Z",
  "saveId": "test",
  "players": []
}`), 0o644); err != nil {
		t.Fatalf("write second players.json: %v", err)
	}

	second, err := d.ListPlayers(context.Background(), instance)
	if err != nil {
		t.Fatalf("second ListPlayers returned error: %v", err)
	}
	if len(second.Players) != 1 || second.Players[0].Name != "host" || second.Players[0].Status != "offline" {
		t.Fatalf("players after leave = %+v, want cached offline host", second.Players)
	}
	if len(second.RecentEvents) != 2 {
		t.Fatalf("second recent events = %+v, want 2", second.RecentEvents)
	}
	if second.RecentEvents[0].Type != "left" || second.RecentEvents[0].PlayerName != "host" {
		t.Fatalf("latest event = %+v, want left host", second.RecentEvents[0])
	}
	if second.RecentEvents[1].Type != "seen" {
		t.Fatalf("older event = %+v, want seen", second.RecentEvents[1])
	}
}

func TestListPlayersIgnoresCacheFromDifferentSave(t *testing.T) {
	dir := t.TempDir()
	control := filepath.Join(dir, ".local-container", "control")
	if err := os.MkdirAll(control, 0o755); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "players-cache.json"), []byte(`{
  "saveId": "test",
  "updatedAt": "2026-06-30T10:00:00Z",
  "players": [
    {"name": "old-host", "uniqueMultiplayerId": "old-1", "isHost": true, "role": "host", "lastSeen": "2026-06-30T10:00:00Z"},
    {"name": "old-test", "uniqueMultiplayerId": "old-2", "isHost": false, "role": "player", "lastSeen": "2026-06-30T09:00:00Z"}
  ]
}`), 0o644); err != nil {
		t.Fatalf("write players-cache.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "players-events.json"), []byte(`{
  "saveId": "test",
  "updatedAt": "2026-06-30T10:00:00Z",
  "events": [{"id": "old", "type": "left", "playerName": "old-host", "at": "2026-06-30T10:00:00Z", "message": "old-host 离开了服务器。"}]
}`), 0o644); err != nil {
		t.Fatalf("write players-events.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "players.json"), []byte(`{
  "updatedAt": "2026-06-30T10:49:18Z",
  "saveId": "new-save",
  "players": [{"name": "new-host", "uniqueMultiplayerId": "new-1", "isHost": true}]
}`), 0o644); err != nil {
		t.Fatalf("write players.json: %v", err)
	}

	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()
	instance.DataDir = dir
	result, err := d.ListPlayers(context.Background(), instance)
	if err != nil {
		t.Fatalf("ListPlayers returned error: %v", err)
	}
	if result.SaveID != "new-save" {
		t.Fatalf("save id = %q, want new-save", result.SaveID)
	}
	if result.OnlineCount == nil || *result.OnlineCount != 1 {
		t.Fatalf("online count = %#v, want 1", result.OnlineCount)
	}
	if len(result.Players) != 1 || result.Players[0].Name != "new-host" {
		t.Fatalf("players = %+v, want only new-host", result.Players)
	}
	if len(result.RecentEvents) != 1 || result.RecentEvents[0].PlayerName != "new-host" {
		t.Fatalf("recent events = %+v, want only new-host event", result.RecentEvents)
	}
}

func TestListPlayersReturnsCachedRosterWhenStopped(t *testing.T) {
	dir := t.TempDir()
	control := filepath.Join(dir, ".local-container", "control")
	if err := os.MkdirAll(control, 0o755); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "players-cache.json"), []byte(`{
  "saveId": "test",
  "updatedAt": "2026-06-30T10:00:00Z",
  "players": [{"name": "test", "uniqueMultiplayerId": "2", "role": "player", "lastSeen": "2026-06-30T09:00:00Z"}]
}`), 0o644); err != nil {
		t.Fatalf("write players-cache.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(control, "players.json"), []byte(`{
  "updatedAt": "2026-06-30T10:49:18Z",
  "saveId": "test",
  "players": []
}`), 0o644); err != nil {
		t.Fatalf("write players.json: %v", err)
	}

	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeStoppedInstance()
	instance.DataDir = dir
	result, err := d.ListPlayers(context.Background(), instance)
	if err != nil {
		t.Fatalf("ListPlayers returned error: %v", err)
	}
	if result.Source != "panel_cache" {
		t.Fatalf("source = %q, want panel_cache", result.Source)
	}
	if result.SaveID != "test" {
		t.Fatalf("save id = %q, want test", result.SaveID)
	}
	if result.OnlineCount == nil || *result.OnlineCount != 0 {
		t.Fatalf("online count = %#v, want 0", result.OnlineCount)
	}
	if len(result.Players) != 1 || result.Players[0].Name != "test" || result.Players[0].Status != "offline" {
		t.Fatalf("players = %+v, want cached offline test", result.Players)
	}
}

func TestListPlayersRunsInfoCommandWhenControlFileMissing(t *testing.T) {
	step := 0
	d := newTestDriver(&fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, stdinData string, args ...string) (paneldocker.CommandResult, error) {
			step++
			switch step {
			case 1:
				return paneldocker.CommandResult{Stdout: "0 /tmp/server-output.log", ExitCode: 0}, nil
			case 2:
				if stdinData != "info\n" {
					t.Fatalf("stdin = %q, want info newline", stdinData)
				}
				return paneldocker.CommandResult{Stdout: "", ExitCode: 0}, nil
			default:
				return paneldocker.CommandResult{
					Stdout:   "Players: 1/4\nOnline players: Leah\n",
					ExitCode: 0,
				}, nil
			}
		},
	})

	instance := makeRunningInstance()
	instance.DataDir = t.TempDir()
	result, err := d.ListPlayers(context.Background(), instance)
	if err != nil {
		t.Fatalf("ListPlayers returned error: %v", err)
	}
	if result.Source != "junimo_info" {
		t.Fatalf("source = %q, want junimo_info", result.Source)
	}
	if result.OnlineCount == nil || *result.OnlineCount != 1 {
		t.Fatalf("online count = %#v, want 1", result.OnlineCount)
	}
	if len(result.Players) != 1 || result.Players[0].Name != "Leah" {
		t.Fatalf("players = %+v, want Leah", result.Players)
	}
}

func TestReadPlayersFromControlParsesIsAuthenticated(t *testing.T) {
	dir := t.TempDir()
	control := filepath.Join(dir, ".local-container", "control")
	if err := os.MkdirAll(control, 0o755); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	raw := `{
  "updatedAt": "2026-07-10T10:00:00Z",
  "saveId": "test",
  "players": [
    {"name": "host", "uniqueMultiplayerId": "1", "isHost": true, "isAuthenticated": true},
    {"name": "pending", "uniqueMultiplayerId": "2", "isHost": false, "isAuthenticated": false},
    {"name": "legacyMod", "uniqueMultiplayerId": "3", "isHost": false}
  ]
}`
	if err := os.WriteFile(filepath.Join(control, "players.json"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write players.json: %v", err)
	}

	snapshot, ok := readPlayersFromControl(dir)
	if !ok {
		t.Fatal("readPlayersFromControl returned !ok")
	}
	if len(snapshot.Players) != 3 {
		t.Fatalf("players len = %d, want 3", len(snapshot.Players))
	}
	if snapshot.Players[0].IsAuthenticated == nil || !*snapshot.Players[0].IsAuthenticated {
		t.Fatalf("host isAuthenticated = %#v, want true", snapshot.Players[0].IsAuthenticated)
	}
	if snapshot.Players[1].IsAuthenticated == nil || *snapshot.Players[1].IsAuthenticated {
		t.Fatalf("pending isAuthenticated = %#v, want false", snapshot.Players[1].IsAuthenticated)
	}
	if snapshot.Players[2].IsAuthenticated != nil {
		t.Fatalf("legacyMod isAuthenticated = %#v, want nil (field absent from old mod builds)", snapshot.Players[2].IsAuthenticated)
	}
}

func TestListPlayersOfflinePlayersHideIsAuthenticated(t *testing.T) {
	dir := t.TempDir()
	control := filepath.Join(dir, ".local-container", "control")
	if err := os.MkdirAll(control, 0o755); err != nil {
		t.Fatalf("mkdir control: %v", err)
	}
	// First snapshot: "pending" is online and unauthenticated.
	if err := os.WriteFile(filepath.Join(control, "players.json"), []byte(`{
  "updatedAt": "2026-07-10T10:00:00Z",
  "saveId": "test",
  "players": [
    {"name": "host", "uniqueMultiplayerId": "1", "isHost": true, "isAuthenticated": true},
    {"name": "pending", "uniqueMultiplayerId": "2", "isHost": false, "isAuthenticated": false}
  ]
}`), 0o644); err != nil {
		t.Fatalf("write players.json: %v", err)
	}

	d := newTestDriver(&fakeConsoleDocker{})
	instance := makeRunningInstance()
	instance.DataDir = dir
	if _, err := d.ListPlayers(context.Background(), instance); err != nil {
		t.Fatalf("first ListPlayers returned error: %v", err)
	}

	// Second snapshot: "pending" has disconnected and is no longer listed as online.
	if err := os.WriteFile(filepath.Join(control, "players.json"), []byte(`{
  "updatedAt": "2026-07-10T10:05:00Z",
  "saveId": "test",
  "players": [
    {"name": "host", "uniqueMultiplayerId": "1", "isHost": true, "isAuthenticated": true}
  ]
}`), 0o644); err != nil {
		t.Fatalf("rewrite players.json: %v", err)
	}

	result, err := d.ListPlayers(context.Background(), instance)
	if err != nil {
		t.Fatalf("second ListPlayers returned error: %v", err)
	}
	var pending *PlayerInfo
	for i := range result.Players {
		if result.Players[i].Name == "pending" {
			pending = &result.Players[i]
		}
	}
	if pending == nil {
		t.Fatalf("expected offline 'pending' player still present in roster: %+v", result.Players)
	}
	if pending.Status != "offline" {
		t.Fatalf("pending status = %q, want offline", pending.Status)
	}
	if pending.IsAuthenticated != nil {
		t.Fatalf("pending isAuthenticated = %#v, want nil once offline", pending.IsAuthenticated)
	}
}
