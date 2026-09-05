package stardew_junimo

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestSteamCMDRejectedGuardRemainsRetryableAndCanDownload(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := storage.Open(ctx, config.Config{DataDir: dir, DBPath: filepath.Join(dir, "panel.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	state := &fakeStore{instance: storage.Instance{ID: "stardew", DataDir: dir}}
	fake := &fakeDocker{containerLines: []string{
		"Enter Steam Guard code:",
		"That Steam Guard code was invalid.",
		"Enter Steam Guard code:",
		"Logged in OK",
		"Update state downloading, progress: 42",
		"Success! App '413150' fully installed",
		"Success! App '1007' fully installed",
	}}
	manager := jobs.NewManager(db, slog.Default())
	driver := New(fake, slog.Default(), manager, state)
	runner := &installRunner{driver: driver, instance: state.instance, username: "qa-user", password: "qa-pass"}
	done := make(chan error, 1)
	job, err := manager.Start(ctx, jobs.Spec{Type: "stardew_install", TargetType: "instance", TargetID: "stardew", Run: func(ctx context.Context, job *jobs.Context) error {
		err := runner.runSteamCMDFallback(ctx, job, make(chan string, 4))
		done <- err
		return err
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	waitForDriverTestJobStatus(t, db, job.ID, storage.JobStatusSucceeded)
	rejected := false
	downloaded := false
	for _, update := range state.updated {
		if update.DriverPhase == "steamcmd_guard_required" {
			if rejected && !strings.Contains(update.StateMessage, "无效或已过期") {
				t.Fatal("repeated prompt erased rejection")
			}
			if strings.Contains(update.StateMessage, "无效或已过期") {
				rejected = true
			}
		}
		if rejected && update.DriverPhase == "steamcmd_downloading" {
			downloaded = true
		}
	}
	if !rejected || !downloaded {
		t.Fatalf("rejection=%v resumed download=%v", rejected, downloaded)
	}
	if len(fake.removedVolumes) != 0 {
		t.Fatal("retry must preserve authorization volumes")
	}
}

func TestSteamCMDConfirmationAndPrefixedDownloadAdvancePhase(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := storage.Open(ctx, config.Config{DataDir: dir, DBPath: filepath.Join(dir, "panel.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	state := &fakeStore{instance: storage.Instance{ID: "stardew", DataDir: dir}}
	fake := &fakeDocker{containerLines: []string{
		"Waiting for confirmation...",
		"Waiting for confirmation...Retrying...",
		"Waiting for confirmation...OK",
		"Waiting for confirmation...",
		"Waiting for client config...",
		"Waiting for user info...",
		"Waiting for confirmation... Update state (0x61) downloading, progress: 4.28 (30336711 / 708901690)",
		"Waiting for confirmation...OK",
		"Waiting for confirmation...",
		"Waiting for confirmation... Success! App '413150' fully installed",
		"Waiting for confirmation... Success! App '1007' fully installed",
	}}
	manager := jobs.NewManager(db, slog.Default())
	driver := New(fake, slog.Default(), manager, state)
	runner := &installRunner{driver: driver, instance: state.instance, username: "qa-user", password: "qa-pass"}
	done := make(chan error, 1)
	job, err := manager.Start(ctx, jobs.Spec{Type: "stardew_install", TargetType: "instance", TargetID: "stardew", Run: func(ctx context.Context, job *jobs.Context) error {
		err := runner.runSteamCMDFallback(ctx, job, make(chan string, 4))
		done <- err
		return err
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	waitForDriverTestJobStatus(t, db, job.ID, storage.JobStatusSucceeded)
	confirmed, downloaded := false, false
	for _, update := range state.updated {
		if strings.Contains(update.StateMessage, "手机确认已通过") {
			confirmed = true
			if update.DriverPhase != "steamcmd_auth_running" {
				t.Fatal("confirmation did not leave mobile wait")
			}
		}
		if confirmed && update.DriverPhase == "steamcmd_downloading" {
			downloaded = true
		}
		if confirmed && update.DriverPhase == "steamcmd_guard_mobile_required" {
			t.Fatal("stale confirmation prompt regressed phase")
		}
	}
	if !confirmed || !downloaded {
		t.Fatalf("confirmed=%v downloaded=%v", confirmed, downloaded)
	}
}
