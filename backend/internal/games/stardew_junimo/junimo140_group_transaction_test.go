package stardew_junimo

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func junimo140TransactionFixture(t *testing.T) (junimo140GroupPlan, junimo140GroupTransaction) {
	t.Helper()
	instances, primary := junimo140GroupFixture(t)
	plan, err := buildJunimo140GroupPlan(instances, primary, "apply_"+strings.Repeat("a", 24))
	if err != nil {
		t.Fatal(err)
	}
	const image = "sdvd/server:1.5.0-preview.140"
	txn := junimo140GroupTransaction{SchemaVersion: 1, PlanSHA256: plan.digest(), ActorID: 1, JobID: "job_" + strings.Repeat("a", 32), Phase: "claimed",
		Recommendation: sjconfig.RuntimeStackRecommendation{StackVersion: "fixed140-fixture", Server: sjconfig.RuntimeStackManifestComponent{Tag: "1.5.0-preview.140", Images: []string{image}, Digests: map[string]string{image: junimo140Digest}}},
		Target:         RuntimeUpdateSelectedPair{Server: RuntimeUpdateSelectedImage{Image: image, ImageID: "sha256:" + strings.Repeat("c", 64), Digest: junimo140Digest}}}
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil || manifest.FullStackUpgrade == nil {
		t.Fatal("fixed migration fixture requires its deferred Auth target", err)
	}
	txn.Recommendation.SteamAuth = manifest.FullStackUpgrade.SteamAuth
	for _, member := range plan.Members {
		txn.Worlds = append(txn.Worlds, junimo140WorldProgress{ChildApplyID: junimo140ChildApplyID(plan.ApplyID, member.ID), Original: junimo140WorldOriginal{
			Instance: storage.Instance{ID: member.ID, DriverID: DriverID, DataDir: member.DataDir, State: storage.InstanceStateStopped},
			Server:   RuntimeUpdateSelectedImage{ImageID: "sha256:" + strings.Repeat("b", 64)}, ServerPresent: true, HasWorld: true,
			EnvSHA256: strings.Repeat("d", 64), ComposeSHA256: strings.Repeat("e", 64)}})
	}
	txn.InputSHA256 = txn.inputDigest()
	if err := validateJunimo140GroupTransaction(plan, txn); err != nil {
		t.Fatal(err)
	}
	return plan, txn
}

func cloneJunimo140Transaction(t *testing.T, txn junimo140GroupTransaction) junimo140GroupTransaction {
	t.Helper()
	raw, err := json.Marshal(txn)
	if err != nil {
		t.Fatal(err)
	}
	var clone junimo140GroupTransaction
	if err := json.Unmarshal(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func readyJunimo140Transaction(txn *junimo140GroupTransaction) {
	for i := range txn.Worlds {
		txn.Worlds[i].StopIntent = true
		txn.Worlds[i].Stopped = true
		txn.Worlds[i].ArchiveSHA256 = strings.Repeat("f", 64)
	}
	txn.StageCreateIntent = true
	txn.StageCreated = true
	txn.StageReady = true
	txn.WorldChangesStarted = true
	txn.Phase = "updating_worlds"
}

func TestJunimo140TransactionRejectsInconsistentMutationEvidence(t *testing.T) {
	plan, base := junimo140TransactionFixture(t)
	for _, scenario := range []string{"input changed", "wrong target", "child collision", "unknown phase", "early stage", "missing peer backup", "completion without intent", "stop before save", "missing command id", "unknown auth holder scope", "auth without target", "early commit", "partial rollback"} {
		t.Run(scenario, func(t *testing.T) {
			txn := cloneJunimo140Transaction(t, base)
			switch scenario {
			case "input changed":
				txn.Worlds[0].Original.Server.ImageID = "sha256:" + strings.Repeat("f", 64)
			case "wrong target":
				txn.Target.Server.Digest = "sha256:" + strings.Repeat("f", 64)
				txn.InputSHA256 = txn.inputDigest()
			case "child collision":
				txn.Worlds[0].ChildApplyID = plan.ApplyID
			case "unknown phase":
				txn.Phase = "unsafe"
			case "early stage":
				txn.StageCreateIntent = true
			case "missing peer backup":
				readyJunimo140Transaction(&txn)
				txn.Worlds[1].ArchiveSHA256 = ""
			case "completion without intent":
				txn.Worlds[0].Updated = true
			case "stop before save":
				txn.Worlds[0].Original.ServerWasRunning = true
				txn.Worlds[0].StopIntent = true
				txn.InputSHA256 = txn.inputDigest()
			case "missing command id":
				txn.Worlds[0].SaveIntent = true
				txn.Worlds[0].Original.ServerWasRunning = true
				txn.InputSHA256 = txn.inputDigest()
			case "unknown auth holder scope":
				txn.Worlds[0].Original.AuthPresent = true
				txn.Worlds[0].Original.Auth.ImageID = "sha256:" + strings.Repeat("f", 64)
				txn.InputSHA256 = txn.inputDigest()
			case "auth without target":
				txn.Worlds[0].Original.SteamInviteEnabled = true
				txn.InputSHA256 = txn.inputDigest()
			case "early commit":
				readyJunimo140Transaction(&txn)
				txn.Phase = "committed"
				txn.Worlds[0].UpdateIntent = true
				txn.Worlds[0].Updated = true
			case "partial rollback":
				readyJunimo140Transaction(&txn)
				txn.Phase = "rolled_back"
				txn.Worlds[0].RestoreIntent = true
				txn.Worlds[0].Restored = true
			}
			if err := validateJunimo140GroupTransaction(plan, txn); err == nil {
				t.Fatal("unsafe shared transaction accepted")
			}
		})
	}
}

func TestJunimo140TransactionRequiresFullOwnerAndImmutableInputs(t *testing.T) {
	plan, txn := junimo140TransactionFixture(t)
	if err := writeJunimo140GroupTransaction(plan, &txn); err == nil {
		t.Fatal("checkpoint written before exclusive owner claim")
	}
	if _, err := claimJunimo140Group(plan); err != nil {
		t.Fatal(err)
	}
	if err := writeJunimo140GroupTransaction(plan, &txn); err != nil {
		t.Fatal(err)
	}
	loaded, err := readJunimo140GroupTransaction(plan)
	if err != nil || loaded.InputSHA256 != txn.InputSHA256 {
		t.Fatalf("checkpoint failed to roundtrip: %v", err)
	}
	changed := cloneJunimo140Transaction(t, txn)
	changed.Worlds[0].Original.Instance.Name = "changed"
	changed.InputSHA256 = changed.inputDigest()
	if err := writeJunimo140GroupTransaction(plan, &changed); err == nil {
		t.Fatal("captured immutable originals overwritten")
	}
	readyJunimo140Transaction(&txn)
	if err := writeJunimo140GroupTransaction(plan, &txn); err != nil {
		t.Fatal(err)
	}
	regressed := cloneJunimo140Transaction(t, txn)
	regressed.StageCreateIntent = false
	regressed.StageCreated = false
	regressed.StageReady = false
	regressed.WorldChangesStarted = false
	if err := validateJunimo140GroupTransaction(plan, regressed); err != nil {
		t.Fatalf("fixture must remain structurally valid to test evidence regression: %v", err)
	}
	if err := writeJunimo140GroupTransaction(plan, &regressed); err == nil {
		t.Fatal("durable mutation history regressed")
	}
	for i := range txn.Worlds {
		txn.Worlds[i].UpdateIntent = true
		txn.Worlds[i].Updated = true
	}
	txn.Phase = "committed"
	if err := writeJunimo140GroupTransaction(plan, &txn); err != nil {
		t.Fatal(err)
	}
	if err := writeJunimo140GroupTransaction(plan, &txn); err == nil {
		t.Fatal("terminal checkpoint was mutable")
	}
}

func TestJunimo140TransactionPersistsSaveIntentAndCompleteRollback(t *testing.T) {
	plan, txn := junimo140TransactionFixture(t)
	if _, err := claimJunimo140Group(plan); err != nil {
		t.Fatal(err)
	}
	txn.Worlds[0].Original.ServerWasRunning = true
	txn.InputSHA256 = txn.inputDigest()
	txn.Worlds[0].SaveIntent = true
	txn.Worlds[0].SaveCommandID = "cmd_" + strings.Repeat("a", 32)
	txn.Phase = "quiescing"
	if err := writeJunimo140GroupTransaction(plan, &txn); err != nil {
		t.Fatal(err)
	}
	txn.Worlds[0].Saved = true
	if err := writeJunimo140GroupTransaction(plan, &txn); err != nil {
		t.Fatal(err)
	}
	readyJunimo140Transaction(&txn)
	if err := writeJunimo140GroupTransaction(plan, &txn); err != nil {
		t.Fatal(err)
	}
	for i := range txn.Worlds {
		txn.Worlds[i].RestoreIntent = true
		txn.Worlds[i].Restored = true
	}
	txn.Phase = "rolled_back"
	if err := writeJunimo140GroupTransaction(plan, &txn); err != nil {
		t.Fatal(err)
	}
	path, err := junimo140GroupTransactionPath(plan)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || raw[0] != '{' {
		t.Fatal("private record did not start with an object")
	}
	duplicate := `{"SCHEMAVERSION":1,` + string(raw[1:])
	if err := os.WriteFile(path, []byte(duplicate), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readJunimo140GroupTransaction(plan); err == nil {
		t.Fatal("ambiguous private recovery record accepted")
	}
}

func TestJunimo140MembershipFingerprintExcludesOnlyAttemptIdentity(t *testing.T) {
	instances, primary := junimo140GroupFixture(t)
	a, err := buildJunimo140GroupPlan(instances, primary, "apply_"+strings.Repeat("a", 24))
	if err != nil {
		t.Fatal(err)
	}
	b, err := buildJunimo140GroupPlan(instances, primary, "apply_"+strings.Repeat("b", 24))
	if err != nil {
		t.Fatal(err)
	}
	if a.digest() == b.digest() || a.membershipDigest() != b.membershipDigest() {
		t.Fatal("membership and attempt identities were conflated")
	}
	b.UID = "1234"
	if a.membershipDigest() == b.membershipDigest() {
		t.Fatal("ownership change did not invalidate dry-run scope")
	}
}

func TestJunimo140TransactionRejectsUnknownNestedRecommendationFields(t *testing.T) {
	_, txn := junimo140TransactionFixture(t)
	raw, err := json.Marshal(txn)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"server", "steamAuth", "smapi"} {
		t.Run(field, func(t *testing.T) {
			var envelope map[string]any
			if err := json.Unmarshal(raw, &envelope); err != nil {
				t.Fatal(err)
			}
			component := envelope["recommendation"].(map[string]any)[field].(map[string]any)
			component["unknownRecoveryInstruction"] = true
			modified, err := json.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			var restored junimo140GroupTransaction
			if err := json.Unmarshal(modified, &restored); err == nil {
				t.Fatal("component normalization hid an unknown private recovery field")
			}
		})
	}
}
