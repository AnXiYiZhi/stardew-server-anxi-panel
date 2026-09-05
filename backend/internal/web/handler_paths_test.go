package web

import "testing"

func TestKnownRequestPathIncludesPlayerModsPage(t *testing.T) {
	if !isKnownRequestPath("/instances/stardew/player-mods") {
		t.Fatal("player Mod detail SPA route should be allowed")
	}
	if isKnownRequestPath("/instances/stardew/player-mods/unexpected") {
		t.Fatal("nested unknown player Mod route should remain rejected")
	}
}

func TestKnownRequestPathIncludesGameLibraryAndDynamicInstancePages(t *testing.T) {
	for _, path := range []string{
		"/games",
		"/games/stardew",
		"/games/stardew/install",
		"/games/stardew/new",
		"/instances/farm-02",
		"/instances/farm-02/overview",
		"/instances/farm_02/settings",
	} {
		if !isKnownRequestPath(path) {
			t.Fatalf("expected SPA path to be allowed: %s", path)
		}
	}

	for _, path := range []string{
		"/games/stardew/unexpected",
		"/instances/-farm/overview",
		"/instances/farm-02/overview/nested",
		"/instances/farm-02/unexpected",
	} {
		if isKnownRequestPath(path) {
			t.Fatalf("expected unknown SPA path to be rejected: %s", path)
		}
	}
}

func TestKnownRequestPathIncludesGameInstallationAPI(t *testing.T) {
	if !isKnownRequestPath("/api/games/stardew/installation") {
		t.Fatal("game installation API should be allowed")
	}
	if isKnownRequestPath("/api/games/stardew/installation/unexpected") {
		t.Fatal("nested unknown game installation API should remain rejected")
	}
}
