package steamcmd

import (
	"strings"
	"testing"
)

func TestBuildContainerOptionsSeparatesAccountAndAnonymousApps(t *testing.T) {
	opts, err := BuildContainerOptions(ContainerRequest{
		ImageRef: "steamcmd:qa", TargetVolume: "stardew_game-data",
		LoginVolume: "panel_steamcmd-login", HomeVolume: "panel_steamcmd-home",
		Username: "steam-user", Password: "secret", UseCachedAuth: true,
		Applications: []AppDownload{
			{AppID: 413150, InstallDir: "/data/game"},
			{AppID: 1007, InstallDir: "/data/game/.steam-sdk", Anonymous: true},
		},
	})
	if err != nil {
		t.Fatalf("build shared SteamCMD options: %v", err)
	}
	script := strings.Join(opts.Command, " ")
	if !strings.Contains(script, `+@NoPromptForPassword 1 +force_install_dir /data/game +login "$STEAM_USERNAME" +app_update 413150`) {
		t.Fatalf("cached account command missing: %s", script)
	}
	if !strings.Contains(script, `+login anonymous +app_update 1007`) {
		t.Fatalf("anonymous SDK command missing: %s", script)
	}
	if strings.Contains(script, `"$STEAM_PASSWORD" +app_update 1007`) {
		t.Fatalf("anonymous application received password: %s", script)
	}
}

func TestBuildContainerOptionsRejectsUnsafeManifest(t *testing.T) {
	_, err := BuildContainerOptions(ContainerRequest{
		ImageRef: "steamcmd:qa", TargetVolume: "game_data", LoginVolume: "panel_login", HomeVolume: "panel_home",
		Username: "user", Password: "secret",
		Applications: []AppDownload{{AppID: 1, InstallDir: "/data/game;rm"}},
	})
	if err == nil {
		t.Fatal("unsafe install directory should be rejected")
	}
}
