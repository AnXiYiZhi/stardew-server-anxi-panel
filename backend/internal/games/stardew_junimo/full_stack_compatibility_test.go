package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
)

type fullStackReceiptDocker struct{ *junimo140SaveWindowDocker }

func (f *fullStackReceiptDocker) RuntimeImageInspect(context.Context, string, string) (paneldocker.RuntimeImageMetadata, error) {
	return paneldocker.RuntimeImageMetadata{ID: "sha256:" + strings.Repeat("b", 64), Digest: "sha256:10f438581d741fc146ce710cbe20099475ac68908e99f565cf449f0b8192ccf6"}, nil
}

func fullStackRollbackReceiptFixture(t *testing.T) (*Driver, fullStackOperation, registry.Instance) {
	t.Helper()
	driver, op, _ := fullStackTestOperation(t)
	driver.panelVersion = op.Public.Panel.Version
	driver.panelIdentity.Store(&op.Public.Panel)
	plan, txn := junimo140TransactionFixture(t)
	txn.ActorID, txn.Recommendation = op.Public.ActorID, op.Target
	base := &junimo140CoordinatorDocker{plan: plan}
	for i, member := range plan.Members {
		if err := os.WriteFile(filepath.Join(member.DataDir, "docker-compose.yml"), []byte("original compose\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := sjconfig.UpdateEnvFile(filepath.Join(member.DataDir, ".env"), map[string]string{"SERVER_IMAGE": "sdvd/server:1.5.0-preview.125", "SERVER_IMAGE_CANDIDATES": "sdvd/server:1.5.0-preview.125", "IMAGE_VERSION": "1.5.0-preview.125", "STEAM_INVITE_ENABLED": "false"}); err != nil {
			t.Fatal(err)
		}
		dir := smapiModDir(member.DataDir)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "StardewAnxiPanel.Control.dll"), []byte("synthetic archived old Control for metadata-policy regression"), 0600); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(map[string]string{"Version": "0.3.8"})
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		junimo := filepath.Join(member.DataDir, ".local-container", "mods", "JunimoServer", "JunimoServer.dll")
		if err := os.MkdirAll(filepath.Dir(junimo), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(junimo, []byte("synthetic archived Junimo for metadata-policy regression"), 0600); err != nil {
			t.Fatal(err)
		}
		original := &txn.Worlds[i].Original
		original.Current.Server.Tag = "1.5.0-preview.125"
		original.Server.Digest = "sha256:10f438581d741fc146ce710cbe20099475ac68908e99f565cf449f0b8192ccf6"
		original.ServerContainerID = strings.Repeat(string(rune('a'+i)), 64)
		original.EnvSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(member.DataDir, ".env"))
		original.ComposeSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(member.DataDir, "docker-compose.yml"))
		original.ControlDLLSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(dir, "StardewAnxiPanel.Control.dll"))
		original.ControlManifestSHA256, _ = runtimeRecoveryFileSHA256(filepath.Join(dir, "manifest.json"))
		original.JunimoDLLSHA256, _ = runtimeRecoveryFileSHA256(junimo)
		base.services = append(base.services, paneldocker.RuntimeJunimo140ServiceSnapshot{Project: member.Project, Service: "server", RuntimeServiceMetadata: paneldocker.RuntimeServiceMetadata{ContainerID: original.ServerContainerID, ImageID: original.Server.ImageID}, GameVolume: plan.SourceVolume})
	}
	txn.InputSHA256 = txn.inputDigest()
	journal, err := claimJunimo140Group(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJunimo140GroupTransaction(plan, &txn); err != nil {
		t.Fatal(err)
	}
	original := cloneJunimo140Transaction(t, txn)
	fake := &fullStackReceiptDocker{&junimo140SaveWindowDocker{base}}
	driver.docker = fake
	if err := driver.rollbackJunimo140Group(context.Background(), fake, &journal, &txn, false); err != nil {
		t.Fatal(err)
	}
	op.Groups = []fullStackPreparedGroup{{Plan: plan, Original: original}}
	view := FullStackGroupStatus{PrimaryID: plan.PrimaryID, ScopeSHA256: plan.membershipDigest(), ApplyID: plan.ApplyID, Phase: RuntimeUpdateApplyFailedRolledBack}
	for _, member := range plan.Members {
		view.Members = append(view.Members, member.ID)
	}
	op.Public.Groups = []FullStackGroupStatus{view}
	op.Public.PlanSHA256 = op.digest()
	now := time.Now().UTC()
	op.Public.AuthorizedAt = &now
	op.Public.Phase = "failed_safe"
	if err := driver.writeFullStackOperation(op); err != nil {
		t.Fatal(err)
	}
	if err := driver.writeFullStackCompatibility(op, 0); err != nil {
		t.Fatal(err)
	}
	member := plan.Members[0]
	return driver, op, registry.Instance{ID: member.ID, DriverID: DriverID, DataDir: member.DataDir, State: txn.Worlds[0].Original.Instance.State}
}

func TestFullStackRollbackCompatibilityIsScopedAndSurvivesRestart(t *testing.T) {
	driver, op, instance := fullStackRollbackReceiptFixture(t)
	if !driver.acceptsFullStackCompatibility(instance) {
		t.Fatal("restored source did not receive bounded compatibility")
	}
	restarted := NewWithOptions(driver.docker, nil, nil, nil, DriverOptions{PanelVersion: op.Public.Panel.Version, PanelIdentity: &op.Public.Panel, ContainerDataDir: driver.containerDataDir})
	if err := restarted.requireCurrentRuntimeStack(instance); err != nil {
		effective := restarted.EffectiveRuntimeInspection(instance)
		t.Fatal("restored .125 became locked by the new Panel requirement", err, effective.Status, effective.Code, effective.Reason)
	}
	if !restarted.acceptsFullStackCompatibility(instance) {
		t.Fatal("second process lost compatibility")
	}
	if err := installSMAPIMod(instance.DataDir); err != nil {
		t.Fatal(err)
	}
	if !restarted.acceptsFullStackCompatibility(instance) {
		t.Fatal("known Panel Control convergence invalidated the restored server exception")
	}
	if err := restarted.requireCurrentRuntimeStack(instance); err != nil {
		t.Fatal(err)
	}
	other := op.Public.Panel
	other.ImageID = "sha256:" + strings.Repeat("e", 64)
	restarted.panelIdentity.Store(&other)
	if restarted.acceptsFullStackCompatibility(instance) {
		t.Fatal("another image with the same version inherited the exception")
	}
	restarted.panelIdentity.Store(&op.Public.Panel)
	restarted.panelVersion = "0.7.4"
	if restarted.acceptsFullStackCompatibility(instance) {
		t.Fatal("another Panel version inherited the old exception")
	}
}

func TestFullStackRollbackCompatibilityRejectsChangedScopeAndArtifacts(t *testing.T) {
	for _, scenario := range []string{"wrong member", "wrong operation", "changed volume", "enabled authentication", "changed dll", "changed manifest", "changed Junimo dll", "changed image receipt"} {
		t.Run(scenario, func(t *testing.T) {
			driver, _, instance := fullStackRollbackReceiptFixture(t)
			switch scenario {
			case "wrong member":
				instance.ID = "other"
			case "wrong operation", "changed image receipt":
				path := fullStackCompatibilityPath(instance.DataDir)
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var receipt fullStackCompatibility
				if json.Unmarshal(raw, &receipt) != nil {
					t.Fatal("invalid test receipt")
				}
				if scenario == "wrong operation" {
					receipt.OperationID = "stack_" + strings.Repeat("e", 32)
				} else {
					receipt.ServerImageID = "sha256:" + strings.Repeat("e", 64)
				}
				if err := writePrivateRuntimeUpdateJSON(path, receipt); err != nil {
					t.Fatal(err)
				}
			case "changed volume":
				if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{"GAME_DATA_VOLUME": "different_scope"}); err != nil {
					t.Fatal(err)
				}
			case "enabled authentication":
				if err := sjconfig.UpdateEnvFile(filepath.Join(instance.DataDir, ".env"), map[string]string{"STEAM_INVITE_ENABLED": "true"}); err != nil {
					t.Fatal(err)
				}
			case "changed dll":
				if err := os.WriteFile(filepath.Join(smapiModDir(instance.DataDir), "StardewAnxiPanel.Control.dll"), []byte("unknown changed Control"), 0600); err != nil {
					t.Fatal(err)
				}
			case "changed manifest":
				if err := os.WriteFile(filepath.Join(smapiModDir(instance.DataDir), "manifest.json"), []byte("{}"), 0600); err != nil {
					t.Fatal(err)
				}
			case "changed Junimo dll":
				if err := os.WriteFile(filepath.Join(instance.DataDir, ".local-container", "mods", "JunimoServer", "JunimoServer.dll"), []byte("changed runtime"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if driver.acceptsFullStackCompatibility(instance) {
				t.Fatal("changed or forged compatibility was accepted")
			}
		})
	}
}

func TestFullStackUntouchedReceiptRequiresOriginalScopeAndNoMutationJournal(t *testing.T) {
	driver, op, instance := fullStackRollbackReceiptFixture(t)
	path, err := junimo140GroupTransactionPath(op.Groups[0].Plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	txn := op.Groups[0].Original
	txn.Phase = "untouched"
	if err := driver.writeFullStackCompatibilityReceipt(op, 0, txn); err != nil {
		t.Fatal(err)
	}
	if !driver.acceptsFullStackCompatibility(instance) {
		t.Fatal("own untouched source lost bounded compatibility")
	}
	other := instance
	other.ID = "not-in-scope"
	if driver.acceptsFullStackCompatibility(other) {
		t.Fatal("untouched exception escaped confirmed membership")
	}
	txn.Phase = "claimed"
	if err := writePrivateRuntimeUpdateJSON(path, txn); err != nil {
		t.Fatal(err)
	}
	if driver.acceptsFullStackCompatibility(instance) {
		t.Fatal("untouched receipt bypassed an in-flight mutation journal")
	}
}

func TestFullStackRecoveryCannotUseAnotherGrant(t *testing.T) {
	driver, op, instance := fullStackRollbackReceiptFixture(t)
	for _, actor := range []int64{0, 42} {
		if _, err := driver.StartFullStackRecovery(context.Background(), actor, op.Public.OperationID, op.Public.PlanSHA256); err == nil {
			t.Fatal("another identity reused the recovery authorization")
		}
	}
	if _, err := driver.StartFullStackRecovery(context.Background(), 41, "stack_"+strings.Repeat("d", 32), op.Public.PlanSHA256); err == nil {
		t.Fatal("another operation reused the grant")
	}
	if driver.InstanceFullStackMaintenance(instance.ID) != nil {
		t.Fatal("proven terminal rollback retained maintenance")
	}
	if _, err := driver.FullStackUpgradeStatus(); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}
