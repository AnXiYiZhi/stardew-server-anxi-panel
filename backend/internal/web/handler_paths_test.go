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
