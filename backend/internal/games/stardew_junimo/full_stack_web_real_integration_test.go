//go:build integration

package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updater"
)

// The host browser owns every update/confirmation request. This fixture only
// prepares authorized, independent real game copies, observes the production
// Docker Panel coordinator, injects one explicitly selected local fault, and
// verifies/cleans its exact resources after browser acceptance.
func sjRunFullStackActualWeb(t *testing.T, ctx context.Context, d *Driver, client *paneldocker.Client, instances []registry.Instance, proof map[string]any) junimo140GroupPlan {
	t.Helper()
	const evidence = "/evidence"
	if os.Getenv("ANXI_REAL_FULL_STACK_ISOLATED_DOCKER") != "1" || d.containerDataDir != "/data" {
		t.Fatal("real Web observer requires the isolated task data scope")
	}
	ready := map[string]any{"ready": true, "instances": []string{"a", "b", "c"}, "running": []string{"a", "b"}, "cold": "c", "sourceVolume": proof["sourceVolume"], "sourceImageId": proof["sourceImageId"], "targetImageId": proof["targetImageId"], "sourceControlSha256": proof["sourceControlSha256"]}
	if err := writePrivateRuntimeUpdateJSON(filepath.Join(evidence, "game-ready.json"), ready); err != nil {
		t.Fatal(err)
	}
	want := "succeeded"
	fault := os.Getenv("ANXI_REAL_FULL_STACK_FAULT")
	if fault != "" && fault != "target-exit" && fault != "panel-interruption" {
		t.Fatal("unknown real full-stack fault")
	}
	if fault != "" {
		want = "failed_safe"
	}
	injected := false
	var op fullStackOperation
	phases := map[string]string{}
	for {
		current, err := d.readFullStackOperation()
		if err == nil {
			op = current
			if _, seen := phases[op.Public.Phase]; !seen {
				phases[op.Public.Phase] = time.Now().UTC().Format(time.RFC3339Nano)
			}
			if len(op.Groups) >= 1 && op.Public.AuthorizedAt != nil && !injected {
				group := op.Groups[0]
				if fault == "panel-interruption" {
					txn, readErr := readJunimo140GroupTransaction(group.Plan)
					if readErr == nil && txn.Phase == "preparing_game" {
						out, restartErr := exec.CommandContext(ctx, "docker", "restart", "j140-full-stack-panel").CombinedOutput()
						if restartErr != nil {
							t.Fatalf("owned Panel interruption: %v: %s", restartErr, out)
						}
						injected = true
					}
				} else if fault == "target-exit" {
					snapshots, snapshotErr := d.snapshotJunimo140Group(ctx, client, group.Plan)
					if snapshotErr != nil {
						t.Fatal(snapshotErr)
					}
					for _, snapshot := range snapshots {
						if snapshot.Project == filepath.Base(instances[1].DataDir) && snapshot.Service == "server" && snapshot.Running && snapshot.ImageID == group.Original.Target.Server.ImageID {
							out, stopErr := exec.CommandContext(ctx, "docker", "stop", "--timeout", "2", snapshot.ContainerID).CombinedOutput()
							if stopErr != nil {
								t.Fatalf("owned target exit: %v: %s", stopErr, out)
							}
							injected = true
						}
					}
				}
			}
			if fullStackTerminal(op.Public.Phase) && op.Public.AuthorizedAt != nil {
				break
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal("invalid production operation", err)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(300 * time.Millisecond):
		}
	}
	if op.Public.Phase != want || len(op.Groups) < 1 || len(op.Groups) > 2 || fault != "" && !injected {
		t.Fatalf("production full-stack terminal=%s code=%s error=%s faultInjected=%v", op.Public.Phase, op.Public.ErrorCode, op.Public.Error, injected)
	}
	group := op.Groups[0]
	identity, inspectErr := updater.NewDockerCLI().InspectRunningPanel(ctx, "j140-full-stack-panel", op.Public.Panel.Version)
	if inspectErr != nil || !identity.SameContent(op.Public.Panel) {
		t.Fatal("real Panel differs from confirmed content", inspectErr)
	}
	d.panelIdentity.Store(&identity)
	d.panelVersion = identity.Version
	transactions := []map[string]any{}
	verifiedMembers := 0
	for groupIndex, group := range op.Groups {
		txn, err := readJunimo140GroupTransaction(group.Plan)
		if errors.Is(err, os.ErrNotExist) && want == "failed_safe" && op.Public.Groups[groupIndex].Phase == "unchanged" {
			txn, err = group.Original, nil
			txn.Phase = "untouched"
		}
		if err != nil {
			t.Fatal(err)
		}
		if want == "succeeded" {
			if err := d.verifyFullStackGroupTerminal(ctx, client, group); err != nil {
				t.Fatal("actual game terminal acceptance", err)
			}
		} else if txn.Phase != "rolled_back" && txn.Phase != "untouched" || txn.StageCreateIntent && !txn.StageRemoved {
			t.Fatal("whole group rollback has no cleaned durable terminal")
		}
		for i, member := range group.Plan.Members {
			verifiedMembers++
			instance, err := d.store.(junimo140InstanceLister).ListInstances(ctx)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, row := range instance {
				if row.ID == member.ID {
					found = true
					wantState := storage.InstanceStateRunning
					if member.ID == "c" {
						wantState = storage.InstanceStateSaveRequired
					}
					if fault == "panel-interruption" && member.ID != "c" {
						wantState = storage.InstanceStateStopped
					}
					if row.State != wantState || row.DriverPayload != `{"fixture":"shared-real"}` {
						t.Fatalf("original state/payload lost for %s: state=%s payload=%s", member.ID, row.State, row.DriverPayload)
					}
				}
			}
			var registered registry.Instance
			for _, candidate := range instances {
				if candidate.ID == member.ID {
					registered = candidate
				}
			}
			if !found || rejectJunimo140GroupLease(member.DataDir) != nil || !d.acceptsFullStackCompatibility(registered) || d.requireCurrentRuntimeStack(registered) != nil {
				t.Fatal("terminal member remained fenced or lost bounded runtime compatibility", member.ID)
			}
			if want != "succeeded" && validateJunimo140OriginalFiles(member, txn.Worlds[i]) != nil {
				t.Fatal("rollback changed original configuration or mods", member.ID)
			}
		}
		transactions = append(transactions, map[string]any{"applyId": group.Plan.ApplyID, "phase": txn.Phase, "members": len(group.Plan.Members)})
	}
	if verifiedMembers != 3 {
		t.Fatal("incomplete real-world acceptance scope")
	}
	audit := map[string]any{"passed": true, "operation": op.Public, "transactionPhase": transactions[0]["phase"], "groups": transactions, "fault": fault, "faultInjected": injected, "phases": phases, "gameAcceptance": true, "leasesReleased": true, "boundedCompatibility": true}
	if err := writePrivateRuntimeUpdateJSON(filepath.Join(evidence, "game-terminal-audit.json"), audit); err != nil {
		t.Fatal(err)
	}
	for {
		raw, err := os.ReadFile(filepath.Join(evidence, "browser-audit-done.json"))
		var done struct{ Passed bool }
		if err == nil && json.Unmarshal(raw, &done) == nil && done.Passed {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("browser acceptance did not finish", ctx.Err())
		case <-time.After(time.Second):
		}
	}
	proof["phase"], proof["fullStackOperationId"], proof["fullStackTarget"] = "verified", op.Public.OperationID, op.Public.Panel
	proof["fullStackFault"], proof["fullStackResult"] = fault, op.Public.Phase
	return group.Plan
}
