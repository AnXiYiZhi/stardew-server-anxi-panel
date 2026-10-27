package stardew_junimo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJunimo140ComposeMigratesManagedMountsAndIsIdempotent(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		text := strings.ReplaceAll(strings.ReplaceAll(junimoComposeTemplate, "\r\n", "\n"), "\n", newline)
		got, err := migrateJunimo140ComposeContract(text, true)
		if err != nil {
			t.Fatal(err)
		}
		for _, removed := range []string{":/etc/cont-env.d/", ":/etc/cont-users.d/", ":/etc/cont-groups.d/"} {
			if strings.Contains(got, removed) {
				t.Fatalf("legacy mount retained: %s", removed)
			}
		}
		for _, kept := range []string{"${GAME_DATA_VOLUME}", ":/data/control", ":/data/Mods", ":/data/settings", ":/config/xdg/config/StardewValley", `API_PORT: "8080"`} {
			if !strings.Contains(got, kept) {
				t.Fatalf("Panel runtime contract lost: %s", kept)
			}
		}
		if strings.Count(got, `USER_ID: "${USER_ID:-1000}"`) != 2 || !strings.Contains(got, `SDVD_COMPOSE_REV: "4"`) || !strings.Contains(got, `VALIDATE_ON_BOOT: "false"`) || !strings.Contains(got, `AUTO_DOWNLOAD_ON_BOOT: "false"`) {
			t.Fatal("missing user/group, revision or validation boundary")
		}
		if !strings.Contains(got, `"127.0.0.1:${API_PORT:-8080}:8080/tcp"`) || strings.Contains(got, `"${API_PORT:-8080}:8080/tcp"`) {
			t.Fatal("public .140 status endpoint would be exposed through the host API port")
		}
		again, err := migrateJunimo140ComposeContract(got, true)
		if err != nil || again != got {
			t.Fatalf("migration is not idempotent: %v", err)
		}
		if newline == "\r\n" && strings.Contains(strings.ReplaceAll(got, newline, ""), "\n") {
			t.Fatal("CRLF compose acquired mixed line endings")
		}
	}
}

func TestJunimo140CustomComposeRefusalIsAtomic(t *testing.T) {
	for index, text := range []string{
		strings.Replace(junimoComposeTemplate, "    environment:", "    environment: []", 1),
		strings.Replace(junimoComposeTemplate, ":/etc/cont-env.d/APP_NAME:ro", ":/etc/cont-env.d/APP_NAME:rw", 1),
		strings.Replace(junimoComposeTemplate, "    environment:", "    environment:\n      USER_ID: 0", 1),
		strings.Replace(junimoComposeTemplate, `"${API_PORT:-8080}:8080/tcp"`, `"192.0.2.1:8080:8080/tcp"`, 1),
		strings.Replace(junimoComposeTemplate, `      API_PORT: "8080"`, `      API_PORT: "8081"`, 1),
		strings.Replace(junimoComposeTemplate, `      API_PORT: "8080"`, `      API_PORT_DISABLED: "8080"`, 1),
		strings.Replace(junimoComposeTemplate, `      API_PORT: "8080"`, "      API_PORT: \"8080\"\n      API_PORT: \"8081\"", 1),
	} {
		if text == junimoComposeTemplate {
			t.Fatalf("custom compose case %d did not change the current template", index)
		}
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SERVER_IMAGE=sdvd/server:1.5.0-preview.140\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "docker-compose.yml")
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		changed, err := ensureRuntimeContEnvFix(dir, true)
		if err == nil || changed || !strings.Contains(err.Error(), "junimo140_compose_manual_required") {
			t.Fatalf("custom compose case %d was accepted: changed=%v err=%v", index, changed, err)
		}
		got, _ := os.ReadFile(path)
		if string(got) != text {
			t.Fatal("refused compose was changed")
		}
		if _, err := os.Stat(filepath.Join(dir, serverContUsersDir)); !os.IsNotExist(err) {
			t.Fatal(".140 migration wrote legacy user overrides")
		}
	}
}

func TestJunimo140WithoutInvitesDoesNotEditAuthService(t *testing.T) {
	got, err := migrateJunimo140ComposeContract(junimoComposeTemplate, false)
	if err != nil {
		t.Fatal(err)
	}
	oldStart, oldEnd := composeServiceBounds(junimoComposeTemplate, "steam-auth")
	start, end := composeServiceBounds(got, "steam-auth")
	if got[start:end] != junimoComposeTemplate[oldStart:oldEnd] {
		t.Fatal("disabled auth was migrated")
	}
}
