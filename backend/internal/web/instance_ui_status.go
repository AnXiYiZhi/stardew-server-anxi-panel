package web

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const expectedControlModVersion = "0.1.0"

type controlStatusSnapshot struct {
	State     string `json:"state"`
	SaveID    string `json:"saveId"`
	UpdatedAt string `json:"updatedAt"`
}
type controlPlayersSnapshot struct {
	SaveID    string `json:"saveId"`
	UpdatedAt string `json:"updatedAt"`
	Players   []struct {
		IsHost bool   `json:"isHost"`
		Status string `json:"status"`
	} `json:"players"`
}

// resolveInstanceUIStatus is the single lifecycle projection consumed by UI.
func (s *server) resolveInstanceUIStatus(ctx context.Context, instance storage.Instance) (string, string) {
	now := time.Now().UTC().Format(time.RFC3339)
	if instance.State == storage.InstanceStateError {
		return "failed", instance.UpdatedAt
	}
	if instance.DriverPhase == "stopping" {
		return "stopping", instance.UpdatedAt
	}
	active, err := s.store.ListActiveJobs(ctx, storage.ListActiveJobsFilter{TargetType: "instance", TargetID: instance.ID, Types: []string{"stardew_lifecycle"}})
	if err == nil && len(active) > 0 && instance.State != storage.InstanceStateRunning {
		return "starting_container", active[len(active)-1].UpdatedAt
	}
	if instance.State == storage.InstanceStateStarting {
		return "starting_container", instance.UpdatedAt
	}
	if instance.State != storage.InstanceStateRunning {
		return "stopped", instance.UpdatedAt
	}
	controlDir := filepath.Join(instance.DataDir, ".local-container", "control")
	var status controlStatusSnapshot
	if !readControlJSON(filepath.Join(controlDir, "status.json"), &status) || status.State != "save-loaded" {
		return "loading_save", firstNonEmpty(status.UpdatedAt, instance.UpdatedAt, now)
	}
	var players controlPlayersSnapshot
	if readControlJSON(filepath.Join(controlDir, "players.json"), &players) {
		for _, player := range players.Players {
			if player.IsHost && (player.Status == "" || strings.EqualFold(player.Status, "online")) {
				return "ready", firstNonEmpty(players.UpdatedAt, status.UpdatedAt, now)
			}
		}
	}
	return "waiting_for_host", firstNonEmpty(players.UpdatedAt, status.UpdatedAt, now)
}

func readControlJSON(path string, target any) bool {
	data, err := os.ReadFile(path)
	return err == nil && json.Unmarshal(data, target) == nil
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type runtimeDiagnostic struct {
	ActiveSaveID             string         `json:"activeSaveId,omitempty"`
	SaveDirectory            string         `json:"saveDirectory,omitempty"`
	CacheSaveID              string         `json:"cacheSaveId,omitempty"`
	CacheMatchesActive       bool           `json:"cacheMatchesActive"`
	ControlModVersion        string         `json:"controlModVersion,omitempty"`
	ExpectedControlMod       string         `json:"expectedControlModVersion"`
	ControlModMatches        bool           `json:"controlModMatches"`
	JunimoStackVersion       string         `json:"junimoStackVersion"`
	JunimoUpdateStatus       string         `json:"junimoUpdateStatus"`
	JunimoUpdateCode         string         `json:"junimoUpdateCode"`
	JunimoUpdateReason       string         `json:"junimoUpdateReason"`
	JunimoUpdateSupported    bool           `json:"junimoUpdateSupported"`
	ServerVersion            string         `json:"serverVersion,omitempty"`
	ExpectedServerVersion    string         `json:"expectedServerVersion"`
	SteamAuthVersion         string         `json:"steamAuthVersion,omitempty"`
	ExpectedSteamAuthVersion string         `json:"expectedSteamAuthVersion"`
	JunimoVersionMatches     bool           `json:"junimoVersionMatches"`
	ContainerToSaveMs        *int64         `json:"containerToSaveMs,omitempty"`
	SaveToHostMs             *int64         `json:"saveToHostMs,omitempty"`
	CommandProtocol          map[string]any `json:"commandProtocol"`
}

func buildRuntimeDiagnostic(instance storage.Instance, status controlStatusSnapshot, players controlPlayersSnapshot) runtimeDiagnostic {
	active := sj.GetActiveSaveName(instance.DataDir)
	d := runtimeDiagnostic{ActiveSaveID: active, ExpectedControlMod: expectedControlModVersion}
	if active != "" {
		d.SaveDirectory = filepath.Join(instance.DataDir, "game-data", "Saves", active)
	}
	// players.json is the live cache identity used by lifecycle/UI diagnostics.
	// The legacy players-cache.json file is intentionally deleted after the
	// durable roster migration and must not be reported as the current cache.
	d.CacheSaveID = players.SaveID
	d.CacheMatchesActive = sameDiagnosticSaveIdentity(players.SaveID, active)
	var manifest struct {
		Version string `json:"Version"`
	}
	readControlJSON(filepath.Join(instance.DataDir, ".local-container", "mods", "StardewAnxiPanel.Control", "manifest.json"), &manifest)
	d.ControlModVersion = manifest.Version
	d.ControlModMatches = manifest.Version == expectedControlModVersion
	stack := sj.InspectRuntimeStack(instance.DataDir, instance.State)
	d.JunimoStackVersion = stack.Recommended.StackVersion
	d.JunimoUpdateStatus = stack.Status
	d.JunimoUpdateCode = stack.Code
	d.JunimoUpdateReason = stack.Reason
	d.JunimoUpdateSupported = stack.Supported
	d.ServerVersion = stack.Current.Server.Tag
	d.ExpectedServerVersion = stack.Recommended.Server.Tag
	d.SteamAuthVersion = stack.Current.SteamAuth.Tag
	d.ExpectedSteamAuthVersion = stack.Recommended.SteamAuth.Tag
	d.JunimoVersionMatches = stack.Status == sjconfig.RuntimeStackStatusUpToDate
	d.ContainerToSaveMs = durationBetween(instance.UpdatedAt, status.UpdatedAt)
	d.SaveToHostMs = durationBetween(status.UpdatedAt, players.UpdatedAt)
	return d
}

func durationBetween(start, end string) *int64 {
	a, errA := time.Parse(time.RFC3339, start)
	b, errB := time.Parse(time.RFC3339, end)
	if errA != nil || errB != nil || b.Before(a) || b.Sub(a) > 30*time.Minute {
		return nil
	}
	value := b.Sub(a).Milliseconds()
	return &value
}

func sameDiagnosticSaveIdentity(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	if left == right {
		return true
	}
	return diagnosticFolderHasBase(left, right) || diagnosticFolderHasBase(right, left)
}

func diagnosticFolderHasBase(folder, base string) bool {
	if !strings.HasPrefix(folder, base+"_") {
		return false
	}
	suffix := strings.TrimPrefix(folder, base+"_")
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
