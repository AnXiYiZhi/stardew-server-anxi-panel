package stardew_junimo

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestJunimo140ConnectivityUsesCoherentLiveSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name, code, galaxy, steam, wire, status string
		relay                                   bool
	}{
		{"ready", "SABC123", "connected", "connected", "ready", "connection_ready", true},
		{"gog only", "SABC123", "connected", "connected", "steamRelayPending", "steam_relay_pending", false},
		{"gog before steam", "SABC123", "connected", "lost", "steamRelayPending", "steam_relay_pending", false},
		{"lost steam", "SABC123", "connected", "lost", "reconnecting", "reconnecting", true},
		{"recovering galaxy", "SABC123", "recovering", "connected", "reconnecting", "reconnecting", true},
		{"no lobby", "", "", "lost", "inviteUnavailable", "invite_unavailable", false},
		{"steam connecting", "", "down", "lost", "steamSessionDown", "steam_session_down", false},
		{"forming lobby", "", "down", "connected", "starting", "connection_starting", false},
		{"recovering without code", "", "recovering", "connected", "reconnecting", "reconnecting", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"inviteCode": tc.code, "steamRelayReady": tc.relay, "galaxyLobbyState": tc.galaxy, "steamSessionState": tc.steam, "authReadiness": "expiring", "connectionStatusCode": tc.wire})
			result, err := normalizeJunimo140Connectivity(raw)
			if err != nil || result.Status != tc.status || result.InviteCode != tc.code || result.AuthReadiness != "expiring" {
				t.Fatalf("wrong projection: %+v, %v", result, err)
			}
		})
	}
}

func TestJunimo140ConnectivityRefusesPartialUnknownAndPrivateCodes(t *testing.T) {
	base := map[string]any{"inviteCode": "SABC123", "steamRelayReady": true, "galaxyLobbyState": "connected", "steamSessionState": "connected", "authReadiness": "ok", "connectionStatusCode": "ready"}
	for _, tc := range []struct {
		field string
		value any
	}{
		{"steamRelayReady", nil}, {"steamRelayReady", false}, {"galaxyLobbyState", "future"},
		{"steamSessionState", "future"}, {"authReadiness", "future"}, {"connectionStatusCode", "future"},
		{"inviteCode", "GPRIVATE"}, {"inviteCode", "SABC123\nsecret"}, {"inviteCode", ""},
	} {
		t.Run(tc.field+"/"+strings.ReplaceAll(strings.TrimSpace(stringifyConnectivityValue(tc.value)), "\n", ""), func(t *testing.T) {
			input := make(map[string]any)
			for key, value := range base {
				input[key] = value
			}
			input[tc.field] = tc.value
			raw, _ := json.Marshal(input)
			if result, err := normalizeJunimo140Connectivity(raw); err == nil || result.InviteCode != "" {
				t.Fatalf("invalid snapshot accepted: %+v, %v", result, err)
			}
		})
	}
	if _, err := normalizeJunimo140Connectivity([]byte(strings.Repeat(" ", 64*1024+1))); err == nil {
		t.Fatal("unbounded payload accepted")
	}
}

func stringifyConnectivityValue(value any) string { raw, _ := json.Marshal(value); return string(raw) }

func TestJunimo140InviteReadsLiveSignalsWithoutInferringSteamLogin(t *testing.T) {
	dir := t.TempDir()
	enableSteamInviteForLifecycleTest(t, dir)
	if err := sjconfig.UpdateEnvFile(filepath.Join(dir, ".env"), map[string]string{"SERVER_IMAGE": "sdvd/server:1.5.0-preview.140"}); err != nil {
		t.Fatal(err)
	}
	if err := sjconfig.SetSteamAuthLoggedIn(dir, false); err != nil {
		t.Fatal(err)
	}
	raw := `{"inviteCode":"SABC123","steamRelayReady":false,"galaxyLobbyState":"connected","steamSessionState":"connected","authReadiness":"ok","connectionStatusCode":"steamRelayPending"}`
	reads := 0
	fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, stdin string, args ...string) (paneldocker.CommandResult, error) {
		if stdin != "" || len(args) == 0 || args[0] != "curl" || args[len(args)-1] != "http://localhost:8080/status" {
			t.Fatalf("unexpected connectivity command: %v", args)
		}
		reads++
		return paneldocker.CommandResult{Stdout: raw}, nil
	}}
	driver := New(fake, nil, nil, &fakeStore{instance: storage.Instance{ID: "world", DataDir: dir, State: storage.InstanceStateRunning}})
	driver.inviteCodeCache["world"] = inviteCodeCacheEntry{code: "STALE", expiresAt: time.Now().Add(time.Hour)}
	first, err := driver.GetInviteConnection(context.Background(), registry.Instance{ID: "world"})
	if err != nil || first.Status != "steam_relay_pending" || first.InviteCode != "SABC123" {
		t.Fatalf("wrong live readiness: %+v, %v", first, err)
	}
	raw = `{"inviteCode":null,"steamRelayReady":false,"galaxyLobbyState":"down","steamSessionState":"lost","authReadiness":"unavailable","connectionStatusCode":"steamSessionDown"}`
	second, err := driver.GetInviteConnection(context.Background(), registry.Instance{ID: "world"})
	if err != nil || second.Status != "steam_session_down" || second.InviteCode != "" || reads != 2 {
		t.Fatalf("stale snapshot reused: %+v, %v, reads=%d", second, err, reads)
	}
	(&lifecycleRunner{instance: storage.Instance{DataDir: dir}}).markSteamAuthUsableFromInviteCode(nil)
	if sjconfig.SteamAuthLoggedIn(dir) {
		t.Fatal("code generation upgraded persisted Steam auth state")
	}
}
