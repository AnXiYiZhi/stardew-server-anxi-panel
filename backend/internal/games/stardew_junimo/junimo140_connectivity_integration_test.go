//go:build integration

package stardew_junimo

import (
	"encoding/json"
	"os"
	"testing"
)

func TestJunimo140ConnectivityRealSnapshotOptIn(t *testing.T) {
	path := os.Getenv("ANXI_REAL_JUNIMO140_STATUS")
	if path == "" {
		t.Skip("set ANXI_REAL_JUNIMO140_STATUS to a freshly captured isolated .140 status")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := normalizeJunimo140Connectivity(raw)
	if err != nil {
		t.Fatal(err)
	}
	var world JunimoRuntimeWorldState
	if err := json.Unmarshal(raw, &world); err != nil || world.Version == nil || *world.Version <= 0 {
		t.Fatalf("actual snapshot cursor unavailable: %v", err)
	}
	if result.ConnectionStatusCode == "inviteUnavailable" && (result.InviteCode != "" || result.Status != "invite_unavailable") {
		t.Fatal("unconfigured actual runtime exposed an invitation")
	}
	t.Logf("actual .140 snapshot normalized: status=%s; cursor present", result.Status)
}
