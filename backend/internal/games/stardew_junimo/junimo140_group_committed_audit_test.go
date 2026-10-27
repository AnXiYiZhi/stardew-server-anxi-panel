//go:build integration

package stardew_junimo

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	appconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestJunimo140AuditCommittedTwoRunningFixtureOptIn(t *testing.T) {
	if os.Getenv("ANXI_REAL_JUNIMO140_AUDIT_COMMITTED") != "1" {
		t.Skip("explicit retained committed fixture audit required")
	}
	const identity = "6c86a7cc266735eb8f955fe3"
	repo, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(repo, "tmp", "junimo140-20261015", "group-real-"+identity)
	raw, err := os.ReadFile(filepath.Join(root, "proof.json"))
	if err != nil {
		t.Fatal(err)
	}
	var proof map[string]any
	if json.Unmarshal(raw, &proof) != nil || proof["identity"] != identity || proof["phase"] != "failed_resources_retained" || proof["cleaned"] == true {
		t.Fatal("retained proof mismatch")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	store, err := storage.Open(ctx, appconfig.Config{DataDir: root, DBPath: filepath.Join(root, "panel.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	instances, err := store.ListInstances(ctx)
	if err != nil || len(instances) != 3 {
		t.Fatal("membership changed")
	}
	primary := registry.Instance{ID: "a", DriverID: DriverID, DataDir: filepath.Join(root, "j140-real-"+identity+"-a")}
	id, _ := proof["applyId"].(string)
	// Recover the immutable original plan, since committed members now bind
	// staging. Never reconstruct a new confirmation from current bindings.
	raw, err = os.ReadFile(filepath.Join(primary.DataDir, ".local-container", "junimo140-group", id, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var journal junimo140GroupJournal
	if json.Unmarshal(raw, &journal) != nil || journal.Plan.digest() != proof["planSha256"] {
		t.Fatal("journal identity mismatch")
	}
	plan := journal.Plan
	txn, err := readJunimo140GroupTransaction(plan)
	if err != nil || txn.Phase != "committed" {
		t.Fatal("transaction did not commit", err)
	}
	client := paneldocker.NewClient(paneldocker.Options{DockerPath: "docker"})
	driver := New(client, nil, nil, store)
	snapshots, err := driver.snapshotJunimo140Group(ctx, client, plan)
	if err != nil || len(snapshots) != 2 {
		t.Fatal("runtime scope changed", err)
	}
	for _, snapshot := range snapshots {
		if snapshot.ImageID != txn.Target.Server.ImageID || snapshot.GameVolume != plan.StagingVolume || !snapshot.Running {
			t.Fatal("target running selection mismatch")
		}
	}
	for i, member := range plan.Members {
		if !txn.Worlds[i].Updated || rejectJunimo140GroupLease(member.DataDir) != nil {
			t.Fatal("member completion/lease mismatch")
		}
		current, err := store.GetInstance(ctx, member.ID)
		if err != nil || current.DriverPayload != txn.Worlds[i].Original.Instance.DriverPayload {
			t.Fatal("original long-lived payload changed")
		}
		want := storage.InstanceStateRunning
		if member.ID == "c" {
			want = storage.InstanceStateSaveRequired
		}
		if current.State != want {
			t.Fatal("original selection/cold state changed")
		}
	}
	value := func(key string) string { v, _ := proof[key].(string); return v }
	if err := sjCleanupRealGroup(ctx, driver, client, plan, root, filepath.Base(primary.DataDir), value("sourceVolume"), value("sourceApplyId"), value("sourceImageId"), value("targetImageId")); err != nil {
		t.Fatal(err)
	}
	audit := map[string]any{"passed": true, "parentTestFailed": true, "transaction": "committed", "twoRunningWorlds": true, "originalPayloadsPreserved": true, "coldTemplatePreserved": true, "cleaned": true, "utc": time.Now().UTC()}
	if err := writePrivateRuntimeUpdateJSON(filepath.Join(root, "post-commit-audit.json"), audit); err != nil {
		t.Fatal(err)
	}
	proof["cleaned"], proof["postCommitAuditPassed"] = true, true
	if err := writePrivateRuntimeUpdateJSON(filepath.Join(root, "proof.json"), proof); err != nil {
		t.Fatal(err)
	}
}
