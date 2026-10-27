package stardew_junimo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestRuntimeUpdateRepairPlanUsesVerifiedInspection(t *testing.T) {
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil || manifest.FullStackUpgrade == nil {
		t.Fatalf("full-stack target unavailable: %v", err)
	}
	manifest.Server = manifest.FullStackUpgrade.Server
	manifest.SteamAuth = manifest.FullStackUpgrade.SteamAuth
	manifest.StackVersion = "verified-instance-stack"
	for _, test := range []struct {
		name, phase, code string
		attempts          int
		corrupt           bool
	}{
		{name: "no journal"},
		{name: "stale succeeded", phase: RuntimeUpdateApplySucceeded},
		{name: "stale safe rollback", phase: RuntimeUpdateApplyFailedRolledBack},
		{name: "active journal", phase: RuntimeUpdateApplyRollingBack, code: "runtime_update_in_progress"},
		{name: "unverifiable rollback", phase: RuntimeUpdateApplyRollbackFailed, code: "recovery_state_uncertain"},
		{name: "exhausted rollback", phase: RuntimeUpdateApplyRollbackFailed, attempts: 3, code: "runtime_repair_exhausted"},
		{name: "corrupt journal", corrupt: true, code: "recovery_state_uncertain"},
	} {
		t.Run(test.name, func(t *testing.T) {
			instance := registry.Instance{ID: "confirmed", DataDir: t.TempDir(), State: storage.InstanceStateStopped}
			env := strings.Join([]string{
				"IMAGE_VERSION=" + manifest.Server.Tag,
				"SERVER_IMAGE=" + manifest.Server.TrustedCandidates[0],
				"SERVER_IMAGE_CANDIDATES=" + strings.Join(manifest.Server.TrustedCandidates, ","),
				"STEAM_SERVICE_IMAGE=" + manifest.SteamAuth.TrustedCandidates[0],
				"STEAM_SERVICE_IMAGE_CANDIDATES=" + strings.Join(manifest.SteamAuth.TrustedCandidates, ","),
				"STEAM_INVITE_ENABLED=true", "STEAM_AUTH_COMPLETED=true",
			}, "\n") + "\n"
			if err := os.WriteFile(filepath.Join(instance.DataDir, ".env"), []byte(env), 0o600); err != nil {
				t.Fatal(err)
			}
			inspection := sjconfig.InspectRuntimeStackWithManifest(instance.DataDir, true, true, manifest)
			if inspection.Status != sjconfig.RuntimeStackStatusUpToDate {
				t.Fatalf("verified instance fixture is not current: %+v", inspection)
			}
			global := InspectManagedRuntimeStack(instance.DataDir, instance.State)
			if global.Status == sjconfig.RuntimeStackStatusUpToDate || global.Recommended.StackVersion == inspection.Recommended.StackVersion {
				t.Fatal("global recommendation did not demonstrate the instance exception")
			}
			if test.phase != "" || test.corrupt {
				status := RuntimeUpdateApplyStatus{ApplyID: "apply_" + strings.Repeat("a", 24), Phase: test.phase, RepairAttempts: test.attempts}
				if err := writeRuntimeUpdateApplyStatus(instance.DataDir, status); err != nil {
					t.Fatal(err)
				}
				if test.corrupt {
					if err := os.WriteFile(runtimeUpdateApplyStatusPath(instance.DataDir), []byte("{broken"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			plan := DetectRuntimeUpdateRepairPlanWithInspection(instance, inspection)
			if test.code == "" {
				if plan != nil {
					t.Fatalf("verified current runtime fabricated a repair: %+v", plan)
				}
				return
			}
			if plan == nil || plan.Code != test.code || plan.ActionAvailable {
				t.Fatalf("journal priority was discarded: %+v", plan)
			}
		})
	}
}

func TestRuntimeUpdateRepairPlanUsesSuppliedDiagnosis(t *testing.T) {
	manifest, err := sjconfig.BuiltInRuntimeStackManifest()
	if err != nil {
		t.Fatal(err)
	}
	instance := registry.Instance{ID: "diagnosed", DataDir: t.TempDir(), State: storage.InstanceStateStopped}
	env := strings.Join([]string{
		"IMAGE_VERSION=" + manifest.Server.Tag,
		"SERVER_IMAGE=" + manifest.Server.TrustedCandidates[0],
		"SERVER_IMAGE_CANDIDATES=" + strings.Join(manifest.Server.TrustedCandidates, ","),
		"STEAM_SERVICE_IMAGE=" + manifest.SteamAuth.TrustedCandidates[0],
		"STEAM_SERVICE_IMAGE_CANDIDATES=" + strings.Join(manifest.SteamAuth.TrustedCandidates, ","),
		"STEAM_INVITE_ENABLED=true", "STEAM_AUTH_COMPLETED=true",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(instance.DataDir, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	global := InspectManagedRuntimeStack(instance.DataDir, instance.State)
	if global.Repairable || DetectRuntimeUpdateRepairPlan(instance) != nil {
		t.Fatal("global catalog unexpectedly supplied the caller's repair diagnosis")
	}
	diagnosis := global
	diagnosis.Repairable = true
	diagnosis.RepairCode = "repairable/legacy_candidates"
	diagnosis.RepairReason = "The caller verified this instance's legacy candidate configuration."
	plan := DetectRuntimeUpdateRepairPlanWithInspection(instance, diagnosis)
	if plan == nil || plan.Code != diagnosis.RepairCode || plan.Detection != diagnosis.RepairReason || !plan.ActionAvailable || plan.Action != "repair" {
		t.Fatalf("caller diagnosis was ignored in favor of the global catalog: %+v", plan)
	}
}
