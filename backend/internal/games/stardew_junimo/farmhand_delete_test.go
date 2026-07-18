package stardew_junimo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

func TestSplitCurlResponsePreservesJSONAndStatus(t *testing.T) {
	body, status, err := splitCurlResponse(paneldocker.CommandResult{Stdout: `{"success":false,"error":"currently online"}` + "\n200\n", ExitCode: 0})
	if err != nil || status != "200" || body != `{"success":false,"error":"currently online"}` {
		t.Fatalf("body=%q status=%q err=%v", body, status, err)
	}
}

func TestVerifyFarmhandAbsentOnDisk(t *testing.T) {
	dataDir := t.TempDir()
	saveName := "Farm_1"
	saveDir := filepath.Join(dataDir, ".local-container", "saves", "Saves", saveName)
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	withFarmhand := `<SaveGame><player><name>Host</name><UniqueMultiplayerID>1</UniqueMultiplayerID></player><farmhands><Farmer><name>Alice</name><UniqueMultiplayerID>42</UniqueMultiplayerID></Farmer></farmhands></SaveGame>`
	path := filepath.Join(saveDir, saveName)
	if err := os.WriteFile(path, []byte(withFarmhand), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyFarmhandAbsentOnDisk(dataDir, saveName, "42"); err == nil {
		t.Fatal("expected present farmhand verification to fail")
	}
	withoutFarmhand := `<SaveGame><player><name>Host</name><UniqueMultiplayerID>1</UniqueMultiplayerID></player><farmhands></farmhands></SaveGame>`
	if err := os.WriteFile(path, []byte(withoutFarmhand), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyFarmhandAbsentOnDisk(dataDir, saveName, "42"); err != nil {
		t.Fatalf("absent farmhand verification failed: %v", err)
	}
}

func TestMarkSaveCharacterCapabilities(t *testing.T) {
	dataDir := t.TempDir()
	saveName := "Farm_1"
	saveDir := filepath.Join(dataDir, ".local-container", "saves", "Saves", saveName)
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `<SaveGame><player><name>Host</name><UniqueMultiplayerID>1</UniqueMultiplayerID></player><farmhands><Farmer><name>Alice</name><UniqueMultiplayerID>42</UniqueMultiplayerID></Farmer></farmhands></SaveGame>`
	if err := os.WriteFile(filepath.Join(saveDir, saveName), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	players := []PlayerInfo{
		{Name: "Host", UniqueMultiplayerID: "1", IsHost: true, Status: "online"},
		{Name: "Alice", UniqueMultiplayerID: "42", Status: "offline"},
		{Name: "History", UniqueMultiplayerID: "99", Status: "offline", Source: "sqlite_roster"},
	}
	markSaveCharacterCapabilities(dataDir, saveName, players)
	if !players[0].SaveCharacterPresent || players[0].CanDeleteCharacter || players[0].DeleteCharacterBlockReason != "host_not_supported" {
		t.Fatalf("host capability = %+v", players[0])
	}
	if !players[1].SaveCharacterPresent || !players[1].CanDeleteCharacter {
		t.Fatalf("offline farmhand capability = %+v", players[1])
	}
	if players[2].SaveCharacterPresent || players[2].CanDeleteCharacter || players[2].DeleteCharacterBlockReason != "character_not_in_save" {
		t.Fatalf("history-only capability = %+v", players[2])
	}
}

func TestCountOtherOnlineHumansAllowsOtherPlayersButRejectsTarget(t *testing.T) {
	players := []PlayerInfo{
		{UniqueMultiplayerID: "host", IsHost: true, Status: "online"},
		{UniqueMultiplayerID: "target", Status: "offline"},
		{UniqueMultiplayerID: "other-a", Status: "online"},
		{UniqueMultiplayerID: "other-b", Status: "online"},
	}
	count, err := countOtherOnlineHumans(players, "target")
	if err != nil || count != 2 {
		t.Fatalf("expected two allowed online humans, count=%d err=%v", count, err)
	}
	players[1].Status = "online"
	_, err = countOtherOnlineHumans(players, "target")
	var commandErr *CommandError
	if !errors.As(err, &commandErr) || commandErr.Code != "farmhand_online" {
		t.Fatalf("expected farmhand_online, got %v", err)
	}
}
