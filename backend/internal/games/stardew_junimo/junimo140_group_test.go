package stardew_junimo

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func junimo140GroupFixture(t *testing.T) ([]storage.Instance, registry.Instance) {
	t.Helper()
	root := t.TempDir()
	instances := []storage.Instance{}
	for _, id := range []string{"b", "a", "unrelated"} {
		dir := filepath.Join(root, "world-"+id)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		volume := "shared-game-fixture"
		if id == "unrelated" {
			volume = "other-game-fixture"
		}
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("GAME_DATA_VOLUME="+volume+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		instances = append(instances, storage.Instance{ID: id, DriverID: DriverID, DataDir: dir})
	}
	return instances, registry.Instance{ID: instances[0].ID, DriverID: DriverID, DataDir: instances[0].DataDir}
}

func TestJunimo140GroupDiscoversAllSharedWorldsAndStableIdentity(t *testing.T) {
	instances, primary := junimo140GroupFixture(t)
	id := "apply_" + strings.Repeat("a", 24)
	plan, err := buildJunimo140GroupPlan(instances, primary, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Members) != 2 || plan.Members[0].ID != "a" || plan.Members[1].ID != "b" || plan.UID != "1000" || plan.GID != "1000" || plan.SourceVolume != "shared-game-fixture" || plan.StagingVolume != "world-b_anxi-smapi-update-"+strings.Repeat("a", 24) {
		t.Fatalf("wrong group: %+v", plan)
	}
	if plan.Members[0].Archive == plan.Members[1].Archive {
		t.Fatal("world archives collide")
	}
	instances[0], instances[2] = instances[2], instances[0]
	reordered, err := buildJunimo140GroupPlan(instances, primary, id)
	if err != nil || reordered.digest() != plan.digest() {
		t.Fatal("store order changed transaction identity")
	}
}

func TestJunimo140GroupRejectsAmbiguousRootsMembershipAndUIDs(t *testing.T) {
	for _, scenario := range []string{"missing primary", "changed primary", "duplicate id", "nested other game", "different UID", "invalid UID", "unsafe env"} {
		t.Run(scenario, func(t *testing.T) {
			instances, primary := junimo140GroupFixture(t)
			switch scenario {
			case "missing primary":
				instances = instances[1:]
			case "changed primary":
				primary.DataDir = instances[1].DataDir
			case "duplicate id":
				instances[1].ID = instances[0].ID
			case "nested other game":
				nested := filepath.Join(instances[0].DataDir, "other")
				if err := os.Mkdir(nested, 0o755); err != nil {
					t.Fatal(err)
				}
				instances = append(instances, storage.Instance{ID: "other", DriverID: "other-game", DataDir: nested})
			case "different UID", "invalid UID":
				uid := "1200"
				if scenario == "invalid UID" {
					uid = "1000;id"
				}
				if err := os.WriteFile(filepath.Join(instances[1].DataDir, ".env"), []byte("GAME_DATA_VOLUME=shared-game-fixture\nUSER_ID="+uid+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "unsafe env":
				if err := os.Remove(filepath.Join(instances[1].DataDir, ".env")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(instances[1].DataDir, ".env"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := buildJunimo140GroupPlan(instances, primary, "apply_"+strings.Repeat("a", 24)); err == nil {
				t.Fatal("ambiguous group accepted")
			}
			if _, err := os.Stat(junimo140GroupLeasePath(primary.DataDir)); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("discovery created a mutation lease")
			}
		})
	}
}

func TestJunimo140GroupLeaseReleaseRequiresPersistedTerminalForWholeGroup(t *testing.T) {
	instances, primary := junimo140GroupFixture(t)
	plan, err := buildJunimo140GroupPlan(instances, primary, "apply_"+strings.Repeat("a", 24))
	if err != nil {
		t.Fatal(err)
	}
	journal, err := claimJunimo140Group(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range plan.Members {
		lease, err := readJunimo140GroupLease(member.DataDir)
		if err != nil || lease.PlanSHA256 != plan.digest() {
			t.Fatal("peer was not durably claimed")
		}
		if runtime.GOOS != "windows" {
			info, err := os.Stat(junimo140GroupLeasePath(member.DataDir))
			if err != nil || info.Mode().Perm() != 0o600 {
				t.Fatal("private lease permissions are not 0600")
			}
		}
	}
	if releaseJunimo140Group(journal) == nil {
		t.Fatal("active group released")
	}
	journal.Phase = "committed"
	if releaseJunimo140Group(journal) == nil {
		t.Fatal("in-memory-only terminal released the group")
	}
	if err := writeJunimo140GroupJournal(&journal, "committed"); err != nil {
		t.Fatal(err)
	}
	// A foreign lease anywhere must prevent deletion of every matching peer.
	foreign := junimo140GroupLease{SchemaVersion: 1, ApplyID: "apply_" + strings.Repeat("f", 24), PrimaryID: "foreign", PlanSHA256: strings.Repeat("0", 64)}
	if err := writePrivateRuntimeUpdateJSON(junimo140GroupLeasePath(plan.Members[1].DataDir), foreign); err != nil {
		t.Fatal(err)
	}
	if releaseJunimo140Group(journal) == nil {
		t.Fatal("foreign peer lease ignored")
	}
	if _, err := readJunimo140GroupLease(plan.Members[0].DataDir); err != nil {
		t.Fatal("partial release occurred before checking every peer")
	}
	expected := junimo140GroupLease{SchemaVersion: 1, ApplyID: plan.ApplyID, PrimaryID: plan.PrimaryID, PlanSHA256: plan.digest()}
	if err := writePrivateRuntimeUpdateJSON(junimo140GroupLeasePath(plan.Members[1].DataDir), expected); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := releaseJunimo140Group(journal); err != nil {
			t.Fatal(err)
		}
	}
	for _, member := range plan.Members {
		if _, err := os.Lstat(junimo140GroupLeasePath(member.DataDir)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("terminal peer lease remains")
		}
	}
	path, _ := junimo140GroupJournalPath(plan)
	if _, err := os.Stat(path); err != nil {
		t.Fatal("terminal proof was discarded")
	}
}

func TestJunimo140GroupFailedClaimDoesNotStealForeignOwnerOrReuseIdentity(t *testing.T) {
	instances, primary := junimo140GroupFixture(t)
	plan, err := buildJunimo140GroupPlan(instances, primary, "apply_"+strings.Repeat("a", 24))
	if err != nil {
		t.Fatal(err)
	}
	foreignMember := plan.Members[1]
	if err := ensureJunimo140GroupDirectories(foreignMember.DataDir, ".local-container"); err != nil {
		t.Fatal(err)
	}
	foreign := junimo140GroupLease{SchemaVersion: 1, ApplyID: "apply_" + strings.Repeat("f", 24), PrimaryID: "foreign", PlanSHA256: strings.Repeat("0", 64)}
	if err := writePrivateRuntimeUpdateJSON(junimo140GroupLeasePath(foreignMember.DataDir), foreign); err != nil {
		t.Fatal(err)
	}
	journal, err := claimJunimo140Group(plan)
	if err == nil || journal.Phase != "claim_failed" {
		t.Fatal("conflicting group claim succeeded")
	}
	if got, err := readJunimo140GroupLease(foreignMember.DataDir); err != nil || got != foreign {
		t.Fatal("foreign lease changed")
	}
	if _, err := os.Lstat(junimo140GroupLeasePath(plan.Members[0].DataDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed claim left its healthy peer leased")
	}
	if _, err := claimJunimo140Group(plan); err == nil {
		t.Fatal("immutable transaction identity reused")
	}
}

func TestJunimo140GroupPrivateRecordsFailClosed(t *testing.T) {
	instances, primary := junimo140GroupFixture(t)
	plan, err := buildJunimo140GroupPlan(instances, primary, "apply_"+strings.Repeat("a", 24))
	if err != nil {
		t.Fatal(err)
	}
	journal, err := claimJunimo140Group(plan)
	if err != nil {
		t.Fatal(err)
	}
	changed := journal
	changed.Plan = plan
	changed.Plan.SourceVolume = "other-game-fixture"
	if writeJunimo140GroupJournal(&changed, "committed") == nil {
		t.Fatal("changed plan identity accepted")
	}
	path := junimo140GroupLeasePath(plan.Members[0].DataDir)
	for _, raw := range []string{`{broken`, `{"schemaVersion":99}`, `{"schemaVersion":1,"futureSecret":"not accepted"}`, strings.Repeat("x", 2*1024*1024+1)} {
		if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readJunimo140GroupLease(plan.Members[0].DataDir); err == nil {
			t.Fatal("corrupt/unknown record accepted")
		}
	}
	// Strict JSON also refuses a second top-level object.
	lease := junimo140GroupLease{SchemaVersion: 1, ApplyID: plan.ApplyID, PrimaryID: plan.PrimaryID, PlanSHA256: plan.digest()}
	raw, _ := json.Marshal(lease)
	duplicate := strings.Replace(string(raw), `"schemaVersion":1`, `"schemaVersion":1,"schemaVersion":1`, 1)
	if err := os.WriteFile(path, []byte(duplicate), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readJunimo140GroupLease(plan.Members[0].DataDir); err == nil {
		t.Fatal("duplicate identity key accepted")
	}
	aliasDuplicate := strings.Replace(string(raw), `"schemaVersion":1`, `"SchemaVersion":1,"schemaVersion":1`, 1)
	if err := os.WriteFile(path, []byte(aliasDuplicate), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readJunimo140GroupLease(plan.Members[0].DataDir); err == nil {
		t.Fatal("case-alias duplicate identity accepted")
	}
	if err := os.WriteFile(path, append(raw, []byte(` {}`)...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readJunimo140GroupLease(plan.Members[0].DataDir); err == nil {
		t.Fatal("trailing record accepted")
	}
}

func TestJunimo140GroupRecoveryResolvesPeerThroughStoreDuringPartialVolumeSwitch(t *testing.T) {
	instances, primary := junimo140GroupFixture(t)
	plan, err := buildJunimo140GroupPlan(instances, primary, "apply_"+strings.Repeat("a", 24))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := claimJunimo140Group(plan); err != nil {
		t.Fatal(err)
	}
	// One peer points at staging and the other still at the original volume.
	if err := os.WriteFile(filepath.Join(plan.Members[0].DataDir, ".env"), []byte("GAME_DATA_VOLUME="+plan.StagingVolume+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, member := range plan.Members {
		caller := registry.Instance{ID: member.ID, DriverID: DriverID, DataDir: member.DataDir}
		journal, err := readRecoverableJunimo140Group(instances, caller)
		if err != nil || journal.PlanSHA256 != plan.digest() {
			t.Fatalf("partial switch prevented identity recovery: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(plan.Members[0].DataDir, ".env"), []byte("GAME_DATA_VOLUME=unrelated-game\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readRecoverableJunimo140Group(instances, primary); err == nil {
		t.Fatal("changed group membership accepted")
	}
}

func TestJunimo140GroupRecoveryRefusesChangedOrMissingPrimaryAndJournal(t *testing.T) {
	for _, scenario := range []string{"missing primary", "changed root", "new member", "changed journal", "unknown phase"} {
		t.Run(scenario, func(t *testing.T) {
			instances, primary := junimo140GroupFixture(t)
			plan, err := buildJunimo140GroupPlan(instances, primary, "apply_"+strings.Repeat("a", 24))
			if err != nil {
				t.Fatal(err)
			}
			journal, err := claimJunimo140Group(plan)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "missing primary":
				instances = instances[1:]
			case "changed root":
				instances[0].DataDir = instances[2].DataDir
			case "new member":
				if err := os.WriteFile(filepath.Join(instances[2].DataDir, ".env"), []byte("GAME_DATA_VOLUME="+plan.SourceVolume+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "changed journal", "unknown phase":
				if scenario == "changed journal" {
					journal.Plan.SourceVolume = "unrelated-game"
				} else {
					journal.Phase = "future"
				}
				path, _ := junimo140GroupJournalPath(plan)
				if err := writePrivateRuntimeUpdateJSON(path, journal); err != nil {
					t.Fatal(err)
				}
			}
			peer := plan.Members[0]
			if _, err := readRecoverableJunimo140Group(instances, registry.Instance{ID: peer.ID, DriverID: DriverID, DataDir: peer.DataDir}); err == nil {
				t.Fatal("unproven recovery identity accepted")
			}
		})
	}
}
