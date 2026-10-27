package stardew_junimo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

func TestJunimo140DeleteUsesFixedContainerAPIPort(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("API_PORT=19367\nAPI_KEY=fixture-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, service, stdin string, args ...string) (paneldocker.CommandResult, error) {
		called = true
		if service != "server" || stdin != "" || args[len(args)-1] != "http://localhost:8080/farmhands?playerId=42" {
			t.Fatal("delete used external host port or a different service")
		}
		if !strings.Contains(strings.Join(args, "\n"), "Authorization: Bearer fixture-key") {
			t.Fatal("private delete omitted API authentication")
		}
		return paneldocker.CommandResult{Stdout: "{\"success\":true}\n200"}, nil
	}}
	runner := &farmhandDeleteRunner{instance: registry.Instance{DataDir: dir}, lifecycle: fake, playerID: "42"}
	if err := runner.deleteViaJunimo(context.Background()); err != nil || !called {
		t.Fatalf("fixed internal delete failed: %v", err)
	}
}

func TestJunimo140DeletionRequiresOwnershipRecordAbsentOnDisk(t *testing.T) {
	for _, tc := range []struct {
		name, store string
		wantError   bool
	}{
		{"removed", `{"Farmhands":{}}`, false},
		{"never owned", "", false},
		{"unrelated owner", `{"Farmhands":{"99":{"Platform":"steam","Id":"fixture","Origin":"claim"}}}`, false},
		{"legacy unrelated owner", `{"Farmhands":{"99":{"Platform":"steam","Id":"fixture"}}}`, false},
		{"swallowed owner write", `{"Farmhands":{"42":{"Platform":"steam","Id":"fixture","Origin":"claim"}}}`, true},
		{"stale released marker", `{"Farmhands":{"42":{"Platform":"released","Id":"","Origin":"operator"}}}`, true},
		{"unknown schema", `{"Farmhands":{},"future":true}`, true},
		{"duplicate root", `{"Farmhands":{"42":{"Platform":"steam","Id":"fixture","Origin":"claim"}},"Farmhands":{}}`, true},
		{"noncanonical id", `{"Farmhands":{"042":{"Platform":"steam","Id":"fixture","Origin":"claim"}}}`, true},
		{"corrupt", `{`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, save := t.TempDir(), "Owned_1"
			folder := filepath.Join(savesDir(dir), "Saves", save)
			if err := os.MkdirAll(folder, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SERVER_IMAGE=sdvd/server:1.5.0-preview.140\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(folder, save), []byte(`<SaveGame><farmhands><Farmer><UniqueMultiplayerID>99</UniqueMultiplayerID></Farmer></farmhands></SaveGame>`), 0o600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(folder, "farmhand-ownership.json")
			if tc.store != "" {
				if err := os.WriteFile(path, []byte(tc.store), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := verifyFarmhandAbsentOnDisk(dir, save, "42")
			if (err != nil) != tc.wantError {
				t.Fatalf("ownership deletion verification err=%v wantError=%v", err, tc.wantError)
			}
			if err != nil && strings.Contains(err.Error(), "42") {
				t.Fatal("ownership verification exposed a role identifier")
			}
			if tc.store != "" {
				if after, err := os.ReadFile(path); err != nil || string(after) != tc.store {
					t.Fatal("read-only deletion proof changed ownership data")
				}
			}
		})
	}
}

func TestJunimo140ImportRequiresDurableOwnershipReceipt(t *testing.T) {
	legacy := map[string]string{
		"preSaveAction": importFarmhandUnbindAction, "preSaveActionSaveId": "Owned_1", "saveId": "Owned_1",
		"farmhandCount": "2", "customizedFarmhandCount": "1", "boundFarmhandCount": "0",
	}
	if _, err := importFarmhandUnbindSummaryFromOutcome(CommandOutcome{Details: legacy}, "Owned_1"); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		fields    map[string]string
		wantError bool
	}{
		{"missing", nil, true},
		{"persisted", map[string]string{"farmhandOwnershipRequired": "true", "releasedFarmhandCount": "2", "farmhandOwnershipPersisted": "true"}, false},
		{"memory only", map[string]string{"farmhandOwnershipRequired": "true", "releasedFarmhandCount": "2", "farmhandOwnershipPersisted": "false"}, true},
		{"partial", map[string]string{"farmhandOwnershipRequired": "true", "releasedFarmhandCount": "1", "farmhandOwnershipPersisted": "true"}, true},
		{"invalid count", map[string]string{"farmhandOwnershipRequired": "true", "releasedFarmhandCount": "2.0", "farmhandOwnershipPersisted": "true"}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fields := map[string]string{}
			for key, value := range legacy {
				fields[key] = value
			}
			for key, value := range test.fields {
				fields[key] = value
			}
			_, err := importFarmhandUnbindSummaryFromOutcome(CommandOutcome{Details: fields}, "Owned_1", true)
			if (err != nil) != test.wantError {
				t.Fatalf("ownership receipt err=%v wantError=%v", err, test.wantError)
			}
		})
	}
}

func TestJunimo140ImportOwnershipMatchesEverySavedRole(t *testing.T) {
	dir := t.TempDir()
	save := "Owned_1"
	folder := filepath.Join(savesDir(dir), "Saves", save)
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	xml := `<SaveGame><player><UniqueMultiplayerID>1</UniqueMultiplayerID></player><farmhands><Farmer><UniqueMultiplayerID>42</UniqueMultiplayerID></Farmer><Farmer><UniqueMultiplayerID>-43</UniqueMultiplayerID></Farmer></farmhands></SaveGame>`
	if err := os.WriteFile(filepath.Join(folder, save), []byte(xml), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		json      string
		wantError bool
	}{
		{"released", `{"Farmhands":{"42":{"Platform":"released","Id":"","Origin":"operator"},"-43":{"Platform":"released","Id":"","Origin":"operator"},"99":{"Platform":"steam","Id":"redacted","Origin":"claim"}}}`, false},
		{"wrong roles same count", `{"Farmhands":{"44":{"Platform":"released","Id":"","Origin":"operator"},"45":{"Platform":"released","Id":"","Origin":"operator"}}}`, true},
		{"reclaimed", `{"Farmhands":{"42":{"Platform":"steam","Id":"redacted","Origin":"operator"},"-43":{"Platform":"released","Id":"","Origin":"operator"}}}`, true},
		{"corrupt", `{`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(folder, "farmhand-ownership.json")
			if err := os.WriteFile(path, []byte(test.json), 0o600); err != nil {
				t.Fatal(err)
			}
			err := verifyImportFarmhandOwnershipStore(dir, save, 2)
			if (err != nil) != test.wantError {
				t.Fatalf("ownership store err=%v wantError=%v", err, test.wantError)
			}
			if after, err := os.ReadFile(path); err != nil || string(after) != test.json {
				t.Fatal("ownership verification changed the store")
			}
		})
	}
	if err := verifyImportFarmhandOwnershipStore(dir, "../Owned_1", 2); err == nil {
		t.Fatal("unsafe target accepted")
	}
}
