//go:build integration

package stardew_junimo

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
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

func sjCleanupRealGroup(ctx context.Context, driver *Driver, client *paneldocker.Client, plan junimo140GroupPlan, root, primaryProject, sourceVolume, sourceApplyID, sourceImageID, targetImageID string) error {
	snapshots, err := driver.snapshotJunimo140Group(ctx, client, plan)
	if err != nil {
		return err
	}
	allowed := map[string][]string{}
	for _, member := range plan.Members {
		allowed[member.Project+"/server"] = []string{sourceImageID, targetImageID}
		allowed[member.Project+"/steam-auth"] = nil
	}
	if err := client.RuntimeStopJunimo140Services(ctx, root, snapshots, allowed); err != nil {
		return err
	}
	snapshots, err = driver.snapshotJunimo140Group(ctx, client, plan)
	if err != nil {
		return err
	}
	if err := client.RuntimeRemoveJunimo140Services(ctx, root, snapshots, allowed); err != nil {
		return err
	}
	for _, volume := range []struct{ name, owner string }{{plan.StagingVolume, plan.ApplyID}, {sourceVolume, sourceApplyID}} {
		if err := client.RuntimeRemoveJunimo140Staging(ctx, root, primaryProject, volume.name, volume.owner); err != nil {
			return err
		}
	}
	for _, member := range plan.Members {
		for _, kind := range []string{"volume", "network"} {
			out, err := exec.CommandContext(ctx, "docker", kind, "ls", "--quiet", "--filter", "label=com.docker.compose.project="+member.Project).Output()
			if err != nil {
				return err
			}
			for _, name := range strings.Fields(string(out)) {
				raw, err := exec.CommandContext(ctx, "docker", kind, "inspect", name).Output()
				var records []struct {
					Name       string
					Labels     map[string]string
					Containers map[string]any
				}
				if err != nil || json.Unmarshal(raw, &records) != nil || len(records) != 1 || records[0].Labels["com.docker.compose.project"] != member.Project || len(records[0].Containers) > 0 {
					return errors.New("cleanup project ownership unknown")
				}
				if kind == "volume" {
					holders, err := exec.CommandContext(ctx, "docker", "container", "ls", "--all", "--quiet", "--filter", "volume="+records[0].Name).Output()
					if err != nil || strings.TrimSpace(string(holders)) != "" {
						return errors.New("cleanup project volume still held")
					}
				}
				if out, err := exec.CommandContext(ctx, "docker", kind, "rm", records[0].Name).CombinedOutput(); err != nil {
					return fmt.Errorf("cleanup project resource: %w: %s", err, out)
				}
			}
		}
	}
	return nil
}

func TestJunimo140SharedGroupRetireRolledBackFixtureOptIn(t *testing.T) {
	name := os.Getenv("ANXI_REAL_JUNIMO140_RETIRE")
	if name == "" {
		t.Skip("explicit isolated rolled-back fixture identity required")
	}
	identity := strings.TrimPrefix(name, "group-real-")
	if len(identity) != 24 || name != "group-real-"+identity {
		t.Fatal("invalid isolated retirement identity")
	}
	if _, err := hex.DecodeString(identity); err != nil {
		t.Fatal(err)
	}
	repo, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(repo, "tmp", "junimo140-20261015", name)
	raw, err := os.ReadFile(filepath.Join(root, "proof.json"))
	if err != nil {
		t.Fatal(err)
	}
	var proof map[string]any
	if json.Unmarshal(raw, &proof) != nil || proof["identity"] != identity || proof["phase"] != "failed_resources_retained" || proof["cleaned"] == true {
		t.Fatal("fixture is not a retained failed transaction")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	store, err := storage.Open(ctx, appconfig.Config{DataDir: root, DBPath: filepath.Join(root, "panel.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	jobID, _ := proof["jobId"].(string)
	preApply := proof["entryPoint"] == "real_panel_http" && proof["panelProcessStopped"] == true && jobID == "" && proof["webApplyId"] == nil
	if !preApply {
		job, err := store.GetJob(ctx, jobID)
		if err != nil || job.Status != "failed" {
			t.Fatal("fixture parent job is not terminal failed", err)
		}
	}
	members, err := store.ListInstances(ctx)
	if err != nil || len(members) != 3 {
		t.Fatal("fixture membership changed", err)
	}
	primary := registry.Instance{ID: "a", DriverID: DriverID, DataDir: filepath.Join(root, "j140-real-"+identity+"-a")}
	applyID, _ := proof["applyId"].(string)
	plan, err := buildJunimo140GroupPlan(members, primary, applyID)
	if err != nil || plan.digest() != proof["planSha256"] {
		t.Fatal("fixture immutable scope changed", err)
	}
	var txn junimo140GroupTransaction
	if !preApply {
		txn, err = readJunimo140GroupTransaction(plan)
		if err == nil && os.Getenv("ANXI_REAL_JUNIMO140_RECOVER_PREBACKUP") == "1" && txn.Phase == "rollback_failed" && !txn.StageCreateIntent && !txn.WorldChangesStarted {
			client := paneldocker.NewClient(paneldocker.Options{DockerPath: "docker"})
			driver := New(client, slog.Default(), nil, store)
			if err := driver.RecoverRuntimeUpdateApply(ctx, primary); err != nil {
				t.Fatal("recover partial pre-backup fixture", err)
			}
			txn, err = readJunimo140GroupTransaction(plan)
			proof["preBackupRecoveryPassed"] = err == nil && txn.Phase == "rolled_back"
		}
		if err != nil || txn.Phase != "rolled_back" || txn.StageCreateIntent && !txn.StageRemoved {
			t.Fatal("fixture rollback is not proven complete", err)
		}
	}
	for i, member := range plan.Members {
		if member.DataDir != filepath.Join(root, "j140-real-"+identity+"-"+member.ID) || rejectJunimo140GroupLease(member.DataDir) != nil {
			t.Fatal("fixture restored ownership or original config changed")
		}
		if !preApply && validateJunimo140OriginalFiles(member, txn.Worlds[i]) != nil {
			t.Fatal("restored fixture config changed")
		}
	}
	client := paneldocker.NewClient(paneldocker.Options{DockerPath: "docker"})
	driver := New(client, slog.Default(), nil, store)
	value := func(key string) string { result, _ := proof[key].(string); return result }
	if preApply {
		active, err := jobs.NewManager(store, slog.Default()).Active(ctx, storage.ListActiveJobsFilter{})
		if err != nil || len(active) != 0 {
			t.Fatal("pre-apply fixture has active jobs", err)
		}
		status, err := driver.RuntimeUpdateApplyStatus(primary)
		if err != nil || status.ApplyID != "" || status.JobID != "" {
			t.Fatal("pre-apply fixture gained an apply owner", err)
		}
		journal, err := junimo140GroupJournalPath(plan)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(journal); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("pre-apply fixture gained a journal")
		}
		snapshots, err := driver.snapshotJunimo140Group(ctx, client, plan)
		if err != nil || len(snapshots) != 2 {
			t.Fatal("pre-apply fixture source scope changed", err)
		}
		for _, snapshot := range snapshots {
			if snapshot.ImageID != value("sourceImageId") || snapshot.GameVolume != value("sourceVolume") {
				t.Fatal("pre-apply fixture source identity changed")
			}
		}
	}
	if err := sjCleanupRealGroup(ctx, driver, client, plan, root, filepath.Base(primary.DataDir), value("sourceVolume"), value("sourceApplyId"), value("sourceImageId"), value("targetImageId")); err != nil {
		t.Fatal(err)
	}
	proof["cleaned"] = true
	proof["retirement"] = "Parent failed, durable rolled_back and original config hashes proved; exact owned resources removed, all host recovery materials retained."
	if preApply {
		proof["retirement"] = "Unsubmitted Web fixture, child stopped, no apply owner/journal/leases, exact source images and game volume proved; only owned resources removed, host evidence retained."
	}
	if err := writePrivateRuntimeUpdateJSON(filepath.Join(root, "proof.json"), proof); err != nil {
		t.Fatal(err)
	}
}
