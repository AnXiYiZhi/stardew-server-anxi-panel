package web

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
)

func TestSupportBundleStreamsValidZip(t *testing.T) {
	fake := fakeDockerService{
		versionResult: paneldocker.CommandResult{ExitCode: 0, Stdout: "ok"},
		psResult: paneldocker.ComposePsResult{
			Result:   paneldocker.CommandResult{ExitCode: 0, Stdout: "[]"},
			Services: []paneldocker.ComposeService{{Name: "demo-server-1", Service: "server", State: "running"}},
		},
		logsResult: paneldocker.CommandResult{ExitCode: 0, Stdout: "server log tail STEAM_PASSWORD=super-secret refresh_token=recovery-secret app_ticket=ticket-secret"},
	}
	handler, _, dataRoot, closeStore := newDockerTestHandlerWithStore(t, fake)
	defer closeStore()
	adminCookie := setupDockerAdmin(t, handler)
	instanceDir := filepath.Join(dataRoot, "instances", "stardew")
	if err := os.MkdirAll(filepath.Join(instanceDir, ".local-container", "smapi-update", "recovery", "apply_secret"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instanceDir, ".local-container", "smapi-update", "recovery", "apply_secret", "secret.txt"), []byte("do-not-export-recovery-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(instanceDir, "docker-compose.yml"), []byte("services:\n  server:\n    environment:\n      STEAM_PASSWORD: compose-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	transactionDir := filepath.Join(instanceDir, ".local-container", "control", "new-game-transactions", "tx-secret")
	if err := os.MkdirAll(transactionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(transactionDir, "transaction.json"), []byte(`{"password":"transaction-secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	saveDir := filepath.Join(instanceDir, ".local-container", "saves", "Saves", "PrivateFarm_1")
	if err := os.MkdirAll(saveDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saveDir, "PrivateFarm_1"), []byte(`<SaveGame><secret>save-content-secret</secret></SaveGame>`), 0o600); err != nil {
		t.Fatal(err)
	}

	response, _ := doJSON(t, handler, http.MethodPost, "/api/instances/stardew/support-bundle", nil, adminCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("support bundle returned %d: %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/zip" {
		t.Fatalf("content type = %q, want application/zip", got)
	}
	if got := response.Header().Get("Content-Length"); got != "" {
		t.Fatalf("streamed support bundle should not set Content-Length, got %q", got)
	}

	reader, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if err != nil {
		t.Fatalf("support bundle is not a valid zip: %v", err)
	}
	names := make(map[string]bool, len(reader.File))
	var contents strings.Builder
	for _, file := range reader.File {
		names[file.Name] = true
		if strings.Contains(file.Name, "smapi-update") || strings.Contains(file.Name, "recovery") || strings.Contains(file.Name, "new-game-transactions") || strings.Contains(file.Name, "Saves/") {
			t.Fatalf("support bundle included private recovery entry %q", file.Name)
		}
		rc, openErr := file.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		data, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		contents.Write(data)
	}
	for _, name := range []string{"version.json", "health.json", "instance-state.json", "jobs.json", "audit-logs.json", "compose-ps.json", "server-logs.txt"} {
		if !names[name] {
			t.Fatalf("support bundle missing %s; entries=%v", name, names)
		}
	}
	serialized := strings.ToLower(contents.String())
	for _, secret := range []string{"super-secret", "recovery-secret", "ticket-secret", "do-not-export-recovery-secret", "compose-secret", "transaction-secret", "save-content-secret"} {
		if strings.Contains(serialized, secret) {
			t.Fatalf("support bundle leaked %q", secret)
		}
	}
}
