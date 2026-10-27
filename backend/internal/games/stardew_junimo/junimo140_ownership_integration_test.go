//go:build integration

package stardew_junimo

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// The opt-in fixture must be a consistent copy made after a real .140
// GameLoop.Saved receipt, including its complete XML and ownership store.
func TestJunimo140ImportOwnershipStoreRealFixtureOptIn(t *testing.T) {
	dir := os.Getenv("ANXI_REAL_IMPORT_OWNERSHIP_DATA")
	save := os.Getenv("ANXI_REAL_IMPORT_OWNERSHIP_SAVE")
	count := os.Getenv("ANXI_REAL_IMPORT_OWNERSHIP_COUNT")
	if dir == "" || save == "" || count == "" {
		t.Skip("set ANXI_REAL_IMPORT_OWNERSHIP_DATA, ANXI_REAL_IMPORT_OWNERSHIP_SAVE and ANXI_REAL_IMPORT_OWNERSHIP_COUNT")
	}
	expected, err := strconv.Atoi(count)
	if err != nil || expected <= 0 {
		t.Fatal("invalid real ownership fixture role count")
	}
	if err := verifyImportFarmhandOwnershipStore(dir, save, expected); err != nil {
		t.Fatal(err)
	}
}

func TestJunimo140DeletedOwnershipRealFixtureOptIn(t *testing.T) {
	dir := os.Getenv("ANXI_REAL_DELETE_OWNERSHIP_DATA")
	save := os.Getenv("ANXI_REAL_DELETE_OWNERSHIP_SAVE")
	role := os.Getenv("ANXI_REAL_DELETE_OWNERSHIP_ROLE")
	expected := os.Getenv("ANXI_REAL_DELETE_OWNERSHIP_RESULT")
	if dir == "" || save == "" || role == "" || expected == "" {
		t.Skip("set isolated real deletion ownership fixture")
	}
	if expected != "unconfirmed" && expected != "removed" {
		t.Fatal("unknown deletion proof expectation")
	}
	roster, err := readSaveRosterSnapshot(filepath.Join(savesDir(dir), "Saves", save, save))
	if err != nil {
		t.Fatal("actual deletion save cannot be parsed")
	}
	for _, farmer := range roster.farmhands {
		if farmer.UniqueMultiplayerID == role || farmer.UniqueMultiplayerIDFallback == role {
			t.Fatal("real game did not remove the target from saved XML")
		}
	}
	err = verifyFarmhandAbsentOnDisk(dir, save, role)
	if expected == "unconfirmed" {
		if err == nil || err.Error() != "deleted farmhand ownership is still persisted" {
			t.Fatalf("expected stale real ownership refusal, got %v", err)
		}
		t.Log("actual deleted XML plus stale ownership was refused")
	} else {
		if err != nil {
			t.Fatal(err)
		}
		t.Log("actual deleted XML and ownership removal independently confirmed")
	}
}
