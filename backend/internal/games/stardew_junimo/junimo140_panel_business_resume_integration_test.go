//go:build integration

package stardew_junimo

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// Resume only the unsubmitted import boundary. The committed shared upgrade
// and the successful farmhand deletion are not executed again.
func TestJunimo140PanelBusinessResumeBeforeImportOptIn(t *testing.T) {
	name := os.Getenv("ANXI_REAL_JUNIMO140_RESUME_PANEL")
	if name == "" {
		t.Skip("explicit retained pre-import fixture identity required")
	}
	identity := strings.TrimPrefix(name, "group-real-")
	if len(identity) != 24 || name != "group-real-"+identity {
		t.Fatal("invalid retained Panel fixture name")
	}
	if _, err := hex.DecodeString(identity); err != nil {
		t.Fatal(err)
	}
	repo, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(repo, "tmp", "junimo140-20261015", name)
	if err := plainJunimoGroupRoot(root); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "proof.json"))
	if err != nil {
		t.Fatal(err)
	}
	var proof map[string]any
	if json.Unmarshal(raw, &proof) != nil || proof["identity"] != identity || proof["phase"] != "failed_resources_retained" || proof["cleaned"] == true || proof["panelProcessStopped"] != true {
		t.Fatal("retained Panel fixture is not stopped and failed")
	}
	business, ok := proof["panelBusiness"].(map[string]any)
	if !ok || business["offlineRoleDeleted"] != true {
		t.Fatal("resume requires the completed deletion evidence")
	}
	var failedImport *ImportJournal
	var completedAsIs *ImportJournal
	var completedSwap *ImportJournal
	cleanedImport := false
	if operation, ok := business["importOperationId"].(string); ok && operation != "" {
		failed, err := LoadImportJournal(filepath.Join(root, "j140-real-"+identity+"-b"), operation)
		if err == nil && failed.InstanceID == "b" && failed.JobID == business["importJobId"] && business["passed"] == true && business["phase"] == "completed" && failed.HostHandling == "swap_host_to" && failed.Stage == ImportStageCompleted && failed.UpstreamConfirmed && failed.FarmhandUnbindVerified && failed.DurableGameLoopSaved && failed.DurableSaveCommandID != "" && failed.DurableSaveAfter != nil {
			completedSwap = &failed
			asIsOperation, _ := business["asIsOperationId"].(string)
			asIs, loadErr := LoadImportJournal(filepath.Join(root, "j140-real-"+identity+"-b"), asIsOperation)
			if loadErr != nil || !sjRealAsIsImportComplete(asIs) || asIs.JobID != business["asIsJobId"] || business["asIsImportCompleted"] != true {
				t.Fatal("completed swap requires its successful as-is predecessor", loadErr)
			}
			completedAsIs = &asIs
		} else if err == nil && failed.InstanceID == "b" && failed.JobID == business["importJobId"] && sjRealAsIsImportComplete(failed) {
			completedAsIs = &failed
		} else if err != nil || failed.InstanceID != "b" || failed.JobID != business["importJobId"] || failed.Stage != ImportStageBackupCreated || failed.UpstreamSubmitted || failed.UpstreamConfirmed || failed.PhaseAFIFOWriteAttempted || failed.MaintenanceStarted || failed.MaintenanceRecoveryState != "snapshot_restored" || failed.RecoveryState != "safe_to_resume_or_cleanup" {
			t.Fatal("resume refuses an uncertain or submitted import", err)
		} else {
			failedImport = &failed
		}
	} else if business["asIsImportCompleted"] == true {
		operation, _ := business["asIsOperationId"].(string)
		loaded, err := LoadImportJournal(filepath.Join(root, "j140-real-"+identity+"-b"), operation)
		if err != nil || !sjRealAsIsImportComplete(loaded) || loaded.JobID != business["asIsJobId"] {
			t.Fatal("completed as-is evidence changed", err)
		}
		completedAsIs = &loaded
	} else if business["phase"] != "uploading_real_save" || business["importJobId"] != nil {
		t.Fatal("resume refuses an unverified pre-import boundary")
	} else if previous, ok := proof["previousPreSubmitImport"].(map[string]any); ok {
		operation, _ := previous["operationId"].(string)
		job, _ := previous["jobId"].(string)
		sjProveWebFailedImportCleanup(t, filepath.Join(root, "j140-real-"+identity+"-b"), operation, job)
		cleanedImport = true
	}
	value := func(key string) string { result, _ := proof[key].(string); return result }
	applyID := value("webApplyId")
	if !junimo140GroupApplyID.MatchString(applyID) || applyID != value("applyId") {
		t.Fatal("retained committed apply identity changed")
	}
	primaryDir := filepath.Join(root, "j140-real-"+identity+"-a")
	var journal junimo140GroupJournal
	if err := readJunimo140GroupJSON(filepath.Join(primaryDir, ".local-container", "junimo140-group", applyID, "manifest.json"), &journal); err != nil {
		t.Fatal(err)
	}
	plan := journal.Plan
	if plan.ApplyID != applyID || plan.PrimaryID != "a" || len(plan.Members) != 3 || journal.PlanSHA256 != plan.digest() || plan.digest() != value("planSha256") || plan.SourceVolume != value("sourceVolume") || plan.StagingVolume != value("stageVolume") {
		t.Fatal("retained committed plan changed")
	}
	for _, member := range plan.Members {
		if member.ID != "a" && member.ID != "b" && member.ID != "c" {
			t.Fatal("foreign member in retained fixture")
		}
		if member.Project != "j140-real-"+identity+"-"+member.ID || member.DataDir != filepath.Join(root, member.Project) || rejectJunimo140GroupLease(member.DataDir) != nil {
			t.Fatal("retained member ownership changed")
		}
		allowedOperation := ""
		if member.ID == "b" && failedImport != nil {
			allowedOperation = failedImport.OperationID
		}
		if busy, err := HasUnfinishedImportTransactionOtherThan(member.DataDir, allowedOperation); err != nil || busy {
			t.Fatal("retained member has an unfinished import", err)
		}
	}
	txn, err := readJunimo140GroupTransaction(plan)
	if err != nil || txn.Phase != "committed" {
		t.Fatal("shared group is not durably committed", err)
	}
	for _, world := range txn.Worlds {
		if !world.Updated {
			t.Fatal("shared group was not fully accepted")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	store, err := storage.Open(ctx, appconfig.Config{DataDir: root, DBPath: filepath.Join(root, "panel.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manager := jobs.NewManager(store, slog.Default())
	active, err := manager.Active(ctx, storage.ListActiveJobsFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(active) > 0 && completedAsIs != nil {
		probeClient := paneldocker.NewClient(paneldocker.Options{DockerPath: "docker"})
		probeDriver := New(probeClient, slog.Default(), manager, store)
		job, operation, found, err := probeDriver.findActiveLifecycleJob(ctx, "b")
		if err != nil || !found || operation != "stop" || len(active) != 1 || active[0].ID != job.ID {
			t.Fatal("retained fixture has an unowned active job", err)
		}
		if err := manager.RecoverInterruptedJobs(ctx); err != nil {
			t.Fatal(err)
		}
		proof["interruptedFixtureStopRecovered"] = true
		active, err = manager.Active(ctx, storage.ListActiveJobsFilter{})
	}
	if err != nil || len(active) != 0 {
		t.Fatal("retained fixture still has active jobs", err)
	}
	if failedImport != nil {
		job, err := store.GetJob(ctx, failedImport.JobID)
		if err != nil || job.Type != SaveImportJobType || job.Status != storage.JobStatusFailed {
			t.Fatal("pre-submit import job is not durably failed", err)
		}
	}
	if completedAsIs != nil {
		job, err := store.GetJob(ctx, completedAsIs.JobID)
		if err != nil || job.Type != SaveImportJobType || job.Status != storage.JobStatusSucceeded {
			t.Fatal("as-is import is not durably successful", err)
		}
	}
	if completedSwap != nil {
		job, err := store.GetJob(ctx, completedSwap.JobID)
		if err != nil || job.Type != SaveImportJobType || job.Status != storage.JobStatusSucceeded {
			t.Fatal("swap import is not durably successful", err)
		}
	}
	instances, err := store.ListInstances(ctx)
	if err != nil || len(instances) != 3 {
		t.Fatal("retained fixture store membership changed", err)
	}
	for _, instance := range instances {
		if instance.DataDir != filepath.Join(root, "j140-real-"+identity+"-"+instance.ID) || instance.DriverID != DriverID {
			t.Fatal("retained store path escaped fixture")
		}
	}
	client := paneldocker.NewClient(paneldocker.Options{DockerPath: "docker"})
	driver := New(client, slog.Default(), manager, store)
	checkRuntimes := func(allowMissingStoppedPeer, allowImportedPeerRunning bool) {
		t.Helper()
		snapshots, err := driver.snapshotJunimo140Group(ctx, client, plan)
		if err != nil || len(snapshots) != 2 && !(allowMissingStoppedPeer && len(snapshots) == 1) {
			t.Fatal("retained game scope changed", err)
		}
		primaryCount := 0
		for _, snapshot := range snapshots {
			wantRunning := snapshot.Project == filepath.Base(primaryDir)
			runningMatches := snapshot.Running == wantRunning || allowImportedPeerRunning && snapshot.Project == "j140-real-"+identity+"-b"
			if snapshot.ImageID != value("targetImageId") || snapshot.GameVolume != plan.StagingVolume || !runningMatches {
				t.Fatal("retained target or running policy changed")
			}
			if snapshot.Project == filepath.Base(primaryDir) {
				primaryCount++
			}
		}
		if primaryCount != 1 {
			t.Fatal("retained primary runtime is missing or ambiguous")
		}
	}
	checkRuntimes(failedImport != nil || cleanedImport, completedAsIs != nil && completedSwap == nil)
	archivePath := filepath.Join(root, "panel-business-first-failure.json")
	if archived, err := os.ReadFile(archivePath); os.IsNotExist(err) {
		if err := os.WriteFile(archivePath, raw, 0600); err != nil {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	} else {
		var first map[string]any
		if json.Unmarshal(archived, &first) != nil || first["identity"] != identity || first["webApplyId"] != applyID || first["planSha256"] != plan.digest() || first["phase"] != "failed_resources_retained" || first["panelProcessStopped"] != true {
			t.Fatal("immutable first failure archive identity changed")
		}
		previous, ok := first["panelBusiness"].(map[string]any)
		if !ok || previous["phase"] != "uploading_real_save" || previous["offlineRoleDeleted"] != true || previous["importJobId"] != nil {
			t.Fatal("first failure archive is not the pre-submit boundary")
		}
	}
	completed := false
	defer func() {
		if completed {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cleanupCancel()
			if err := sjCleanupRealGroup(cleanupCtx, driver, client, plan, root, filepath.Base(primaryDir), value("sourceVolume"), value("sourceApplyId"), value("sourceImageId"), value("targetImageId")); err != nil {
				t.Error(err)
				return
			}
			proof["cleaned"] = true
			proof["phase"] = "committed"
		} else {
			proof["phase"] = "failed_resources_retained"
		}
		if err := writePrivateRuntimeUpdateJSON(filepath.Join(root, "proof.json"), proof); err != nil {
			t.Error(err)
		}
	}()
	if completedSwap != nil {
		cold, err := store.GetInstance(ctx, "c")
		if err != nil || cold.State != storage.InstanceStateSaveRequired {
			t.Fatal("completed business changed the cold template state", err)
		}
		entries, err := os.ReadDir(filepath.Join(savesDir(cold.DataDir), "Saves"))
		if err != nil || len(entries) != 0 {
			t.Fatal("completed business created a cold farm", err)
		}
		proof["completedBusinessFinalAudit"] = map[string]any{"passed": true, "auditedAt": time.Now().UTC(), "bothSuccessfulJobsAndJournals": true, "runningPolicyAndSharedVolume": true, "coldTemplatePreserved": true, "businessReplayed": false}
		completed = true
		return
	}
	password := sjSeedRealPanelAdmin(t, ctx, store)
	if failedImport != nil {
		failurePath := filepath.Join(root, "panel-business-maintenance-"+failedImport.OperationID+".json")
		if _, err := os.Stat(failurePath); !os.IsNotExist(err) {
			t.Fatal("pre-submit failure archive already exists or cannot be inspected", err)
		}
		if err := os.WriteFile(failurePath, raw, 0600); err != nil {
			t.Fatal(err)
		}
		proof["previousPreSubmitImport"] = map[string]any{"operationId": failedImport.OperationID, "jobId": failedImport.JobID, "stage": failedImport.Stage, "noFIFOOrUpstreamSubmission": true, "snapshotRestored": true}
		delete(business, "importJobId")
		delete(business, "importOperationId")
		delete(business, "importRetrySameOwner")
		delete(business, "failedPreSubmitAutoRecoveredViaWeb")
		business["phase"] = "uploading_real_save"
	}
	t.Setenv("ANXI_REAL_JUNIMO140_PANEL_RESUME", "1")
	primary := registry.Instance{ID: "a", DriverID: DriverID, DataDir: primaryDir, State: storage.InstanceStateRunning}
	proof["resumedAfterImportContractFix"] = true
	_, actualPlan := sjRunRealGroupViaPanelHTTP(t, ctx, repo, root, password, primary, proof, &plan)
	if actualPlan.digest() != journal.PlanSHA256 {
		t.Fatal("resume changed the committed group")
	}
	checkRuntimes(false, false)
	cold, err := store.GetInstance(ctx, "c")
	if err != nil || cold.State != storage.InstanceStateSaveRequired {
		t.Fatal("resume changed the cold template state", err)
	}
	entries, err := os.ReadDir(filepath.Join(savesDir(cold.DataDir), "Saves"))
	if err != nil || len(entries) != 0 {
		t.Fatal("resume created a cold farm", err)
	}
	completed = !t.Failed()
}
