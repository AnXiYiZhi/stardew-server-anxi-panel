package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

var junimoUniversalInviteCode = regexp.MustCompile(`^S[0-9A-Z]{1,32}$`)

// This response is one live upstream snapshot. File/payload invite codes do
// not prove relay readiness and must never upgrade a persisted auth session.
func normalizeJunimo140Connectivity(raw []byte) (registry.InviteCodeResult, error) {
	var snapshot struct {
		InviteCode           string `json:"inviteCode"`
		SteamRelayReady      *bool  `json:"steamRelayReady"`
		GalaxyLobbyState     string `json:"galaxyLobbyState"`
		SteamSessionState    string `json:"steamSessionState"`
		AuthReadiness        string `json:"authReadiness"`
		ConnectionStatusCode string `json:"connectionStatusCode"`
	}
	invalid := errors.New("Junimo connectivity snapshot is unavailable or inconsistent")
	if len(raw) > 64*1024 || json.Unmarshal(raw, &snapshot) != nil || snapshot.SteamRelayReady == nil {
		return registry.InviteCodeResult{}, invalid
	}
	if snapshot.InviteCode != "" && !junimoUniversalInviteCode.MatchString(snapshot.InviteCode) {
		return registry.InviteCodeResult{}, invalid
	}
	switch snapshot.GalaxyLobbyState {
	case "", "down", "recovering", "connected":
	default:
		return registry.InviteCodeResult{}, invalid
	}
	if snapshot.SteamSessionState != "lost" && snapshot.SteamSessionState != "connected" {
		return registry.InviteCodeResult{}, invalid
	}
	switch snapshot.AuthReadiness {
	case "":
		// No sidecar is configured in IP-only mode; the upstream nullable
		// signal is absent until there is an auth health classification.
		snapshot.AuthReadiness = "unknown"
	case "unknown", "ok", "expiring", "unavailable":
	default:
		return registry.InviteCodeResult{}, invalid
	}
	// Mirror the fixed upstream ConnectionStatus.Compute table and check its
	// derived field instead of trusting an unrelated/stale code file.
	code := "starting"
	if snapshot.InviteCode != "" {
		switch {
		case snapshot.GalaxyLobbyState != "connected":
			code = "reconnecting"
		case !*snapshot.SteamRelayReady:
			code = "steamRelayPending"
		case snapshot.SteamSessionState != "connected":
			code = "reconnecting"
		default:
			code = "ready"
		}
	} else {
		switch {
		case snapshot.GalaxyLobbyState == "":
			code = "inviteUnavailable"
		case snapshot.SteamSessionState != "connected":
			code = "steamSessionDown"
		case snapshot.GalaxyLobbyState == "recovering":
			code = "reconnecting"
		}
	}
	if code != snapshot.ConnectionStatusCode {
		return registry.InviteCodeResult{}, invalid
	}
	status := map[string]string{
		"ready": "connection_ready", "steamRelayPending": "steam_relay_pending",
		"reconnecting": "reconnecting", "starting": "connection_starting",
		"steamSessionDown": "steam_session_down", "inviteUnavailable": "invite_unavailable",
	}[code]
	return registry.InviteCodeResult{SteamInviteEnabled: true, Status: status,
		InviteCode: snapshot.InviteCode, ConnectionStatusCode: code, AuthReadiness: snapshot.AuthReadiness}, nil
}

func (r *lifecycleRunner) readJunimo140Connectivity(ctx context.Context) (registry.InviteCodeResult, error) {
	raw, err := readJunimoAPI(ctx, r.lifecycle, r.instance.DataDir, "/status")
	if err != nil {
		return registry.InviteCodeResult{}, err
	}
	return normalizeJunimo140Connectivity(raw)
}

// GetInviteConnection preserves the legacy endpoint contract while providing
// live, coherent .140 readiness. It does not persist code or auth state.
func (d *Driver) GetInviteConnection(ctx context.Context, instance registry.Instance) (registry.InviteCodeResult, error) {
	stored, err := d.store.GetInstance(ctx, instance.ID)
	if err != nil {
		return registry.InviteCodeResult{}, err
	}
	contract, err := readConsoleRuntimeContract(stored.DataDir)
	if err != nil {
		return registry.InviteCodeResult{}, err
	}
	if contract.fifo != junimo140InputFIFO {
		code, err := d.GetInviteCode(ctx, instance)
		status := "generating"
		if code != "" && code != "n/a" {
			status = "ready"
		}
		return registry.InviteCodeResult{SteamInviteEnabled: true, Status: status, InviteCode: code}, err
	}
	if !sjconfig.SteamInviteEnabled(stored.DataDir) {
		return registry.InviteCodeResult{}, ErrSteamInviteDisabled
	}
	if stored.DriverPhase == importMaintenancePhase {
		return registry.InviteCodeResult{}, &ImportTransactionError{Code: ImportErrorBusy, Message: "invite codes are unavailable during save import maintenance"}
	}
	lifecycle, ok := d.docker.(LifecycleDockerService)
	if !ok {
		return registry.InviteCodeResult{}, errors.New("Docker lifecycle service is unavailable")
	}
	runner := &lifecycleRunner{driver: d, lifecycle: lifecycle, instance: stored}
	return runner.readJunimo140Connectivity(ctx)
}
