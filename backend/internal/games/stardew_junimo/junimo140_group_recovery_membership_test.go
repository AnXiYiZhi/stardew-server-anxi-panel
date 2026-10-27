package stardew_junimo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestJunimo140RecoveryAcceptsOnlyOwnedInterruptedRootRestore(t *testing.T) {
	for _, scenario := range []string{"restore missing env", "restore foreign partial env", "missing env without intent", "missing env during forward update", "new peer", "missing peer", "changed root", "missing lease"} {
		t.Run(scenario, func(t *testing.T) {
			plan, txn := junimo140TransactionFixture(t)
			if _, err := claimJunimo140Group(plan); err != nil {
				t.Fatal(err)
			}
			readyJunimo140Transaction(&txn)
			txn.Phase = "rolling_back"
			txn.Worlds[0].RestoreIntent = true
			if scenario == "missing env without intent" {
				txn.Worlds[0].RestoreIntent = false
			}
			if scenario == "missing env during forward update" {
				txn.Phase = "updating_worlds"
			}
			if err := writeJunimo140GroupTransaction(plan, &txn); err != nil {
				t.Fatal(err)
			}
			instances := []storage.Instance{txn.Worlds[0].Original.Instance, txn.Worlds[1].Original.Instance}
			primary, err := plan.primary()
			if err != nil {
				t.Fatal(err)
			}
			caller := registry.Instance{ID: primary.ID, DriverID: DriverID, DataDir: primary.DataDir}
			envPath := filepath.Join(plan.Members[0].DataDir, ".env")
			if err := os.Remove(envPath); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "restore foreign partial env":
				if err := os.WriteFile(envPath, []byte("GAME_DATA_VOLUME=unrelated-volume\nUSER_ID=1234\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "new peer":
				dir := filepath.Join(filepath.Dir(primary.DataDir), "world-new")
				if err := os.Mkdir(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("GAME_DATA_VOLUME="+plan.SourceVolume+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				instances = append(instances, storage.Instance{ID: "new", DriverID: DriverID, DataDir: dir})
			case "missing peer":
				instances = instances[1:]
			case "changed root":
				dir := filepath.Join(filepath.Dir(primary.DataDir), "world-moved")
				if err := os.Mkdir(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				instances[0].DataDir = dir
			case "missing lease":
				if err := os.Remove(junimo140GroupLeasePath(plan.Members[0].DataDir)); err != nil {
					t.Fatal(err)
				}
			}
			_, err = readRecoverableJunimo140Group(instances, caller)
			allowed := scenario == "restore missing env" || scenario == "restore foreign partial env"
			if (err == nil) != allowed {
				t.Fatalf("wrong interrupted restore scope decision: %v", err)
			}
			if scenario != "restore foreign partial env" {
				if _, err := os.Stat(envPath); !os.IsNotExist(err) {
					t.Fatal("membership inspection wrote or recreated environment data")
				}
			}
		})
	}
}
