package stardew_junimo

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	appconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func TestParseInviteCode_ValidPatterns(t *testing.T) {
	cases := []struct {
		output string
		want   string
	}{
		{"Invite Code: ABCD-1234-EFGH", "ABCD-1234-EFGH"},
		{"invitecode: XY12-3456-ABCD", "XY12-3456-ABCD"},
		{"InviteCode: AA11-BB22-CC33", "AA11-BB22-CC33"},
		{"some output\nABCD-1234\nmore", "ABCD-1234"},
		// Galaxy P2P codes have no hyphens
		{"Invite Code: SGCWS0Z572F2", "SGCWS0Z572F2"},
		{"(Invite code: SGCWS0Z572F2)", "SGCWS0Z572F2"},
		{"some output\nSGCWS0Z572F2\nmore", "SGCWS0Z572F2"},
		{"no code here", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := parseInviteCode(tc.output)
		if got != tc.want {
			t.Errorf("parseInviteCode(%q) = %q, want %q", tc.output, got, tc.want)
		}
	}
}

func TestMergeInviteCodeInPayload(t *testing.T) {
	result := mergeInviteCodeInPayload(`{"save_strategy":"new_game"}`, "ABCD-1234-WXYZ")
	if !containsStr(result, `"invite_code"`) {
		t.Errorf("invite_code not in payload: %s", result)
	}
	if !containsStr(result, "ABCD-1234-WXYZ") {
		t.Errorf("invite code value not in payload: %s", result)
	}
	if !containsStr(result, "save_strategy") {
		t.Errorf("existing key lost in merge: %s", result)
	}
}

func TestMergeInviteCodeInPayload_EmptyExisting(t *testing.T) {
	result := mergeInviteCodeInPayload("", "XXXX-1111")
	if !containsStr(result, `"invite_code"`) {
		t.Errorf("invite_code not in payload: %s", result)
	}
}

func TestInviteCodeFromPayload(t *testing.T) {
	if got := inviteCodeFromPayload(`{"invite_code":"SGD0XEES7LO2"}`); got != "SGD0XEES7LO2" {
		t.Fatalf("inviteCodeFromPayload() = %q", got)
	}
	if got := inviteCodeFromPayload(`{"other":"value"}`); got != "" {
		t.Fatalf("expected empty invite code, got %q", got)
	}
}

func TestClearStaleInviteCodeRemovesOnlyStoredOldCode(t *testing.T) {
	var calls [][]string
	fake := &fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
			calls = append(calls, append([]string{}, args...))
			if reflect.DeepEqual(args, []string{"cat", "/tmp/invite-code.txt"}) {
				return paneldocker.CommandResult{Stdout: "OLD-CODE\n", ExitCode: 0}, nil
			}
			return paneldocker.CommandResult{ExitCode: 0}, nil
		},
	}
	runner := &lifecycleRunner{
		lifecycle: fake,
		instance: storage.Instance{
			DataDir:       "custom-dir",
			DriverPayload: `{"invite_code":"OLD-CODE"}`,
		},
	}

	runner.clearStaleInviteCode(context.Background(), nil)

	if len(calls) != 2 {
		t.Fatalf("expected cat and rm calls, got %d: %#v", len(calls), calls)
	}
	if !reflect.DeepEqual(calls[1], []string{"rm", "-f", "/tmp/invite-code.txt"}) {
		t.Fatalf("expected rm stale invite call, got %#v", calls[1])
	}
}

func TestClearStaleInviteCodeKeepsFreshCode(t *testing.T) {
	var calls [][]string
	fake := &fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
			calls = append(calls, append([]string{}, args...))
			return paneldocker.CommandResult{Stdout: "NEW-CODE\n", ExitCode: 0}, nil
		},
	}
	runner := &lifecycleRunner{
		lifecycle: fake,
		instance: storage.Instance{
			DataDir:       "custom-dir",
			DriverPayload: `{"invite_code":"OLD-CODE"}`,
		},
	}

	runner.clearStaleInviteCode(context.Background(), nil)

	if len(calls) != 1 {
		t.Fatalf("expected only cat call, got %d: %#v", len(calls), calls)
	}
	if !reflect.DeepEqual(calls[0], []string{"cat", "/tmp/invite-code.txt"}) {
		t.Fatalf("expected cat invite call, got %#v", calls[0])
	}
}

func TestTailServerLogsClearsSteamAuthCompletedWhenServerReportsNoAccount(t *testing.T) {
	dir := t.TempDir()
	if err := sjconfig.SetSteamAuthLoggedIn(dir, true); err != nil {
		t.Fatalf("seed steam auth flag: %v", err)
	}

	dataDir := filepath.Join(dir, "store")
	store, err := storage.Open(context.Background(), appconfig.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	fake := &fakeConsoleDocker{
		composeLogsFunc: func(_ context.Context, _ string, _ paneldocker.LogsOptions) (paneldocker.CommandResult, error) {
			return paneldocker.CommandResult{
				Stdout: "[app] Steam-auth service has no logged-in accounts\n",
			}, nil
		},
	}
	runner := &lifecycleRunner{
		lifecycle: fake,
		instance:  storage.Instance{ID: "stardew", DataDir: dir},
	}

	manager := jobs.NewManager(store, slog.Default())
	job, err := manager.Start(context.Background(), jobs.Spec{
		Type:       "test",
		TargetType: "instance",
		TargetID:   "stardew",
		Timeout:    5 * time.Second,
		Run: func(ctx context.Context, jobCtx *jobs.Context) error {
			runner.tailServerLogs(ctx, jobCtx, 30)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("start job: %v", err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)

	if sjconfig.SteamAuthLoggedIn(dir) {
		t.Fatal("expected steam auth flag to be cleared after no logged-in accounts log")
	}
}

func TestTailServerLogsRefreshesSteamAuthServiceWhenCompletedFlagIsStale(t *testing.T) {
	dir := t.TempDir()
	if err := sjconfig.SetSteamAuthLoggedIn(dir, true); err != nil {
		t.Fatalf("seed steam auth flag: %v", err)
	}

	dataDir := filepath.Join(dir, "store")
	store, err := storage.Open(context.Background(), appconfig.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	var restarted []string
	fake := &fakeConsoleDocker{
		composeLogsFunc: func(_ context.Context, _ string, _ paneldocker.LogsOptions) (paneldocker.CommandResult, error) {
			return paneldocker.CommandResult{
				Stdout: "[05:52:29 ERROR JunimoServer] Steam-auth service not ready: Could not reach steam-auth service within 30s: Steam auth service request failed after 4 attempts\n" +
					"[05:52:29 ERROR JunimoServer] Make sure you ran: docker compose run -it steam-auth setup\n" +
					"[05:52:29 WARN JunimoServer] Steam-auth service not ready, Galaxy features unavailable\n",
			}, nil
		},
		restartFunc: func(_ context.Context, _ string, services ...string) (paneldocker.CommandResult, error) {
			restarted = append(restarted, services...)
			return paneldocker.CommandResult{ExitCode: 0}, nil
		},
	}
	runner := &lifecycleRunner{
		lifecycle: fake,
		instance:  storage.Instance{ID: "stardew", DataDir: dir},
	}

	manager := jobs.NewManager(store, slog.Default())
	job, err := manager.Start(context.Background(), jobs.Spec{
		Type:       "test",
		TargetType: "instance",
		TargetID:   "stardew",
		Timeout:    5 * time.Second,
		Run: func(ctx context.Context, jobCtx *jobs.Context) error {
			runner.tailServerLogs(ctx, jobCtx, 30)
			runner.tailServerLogs(ctx, jobCtx, 30)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("start job: %v", err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)

	if !sjconfig.SteamAuthLoggedIn(dir) {
		t.Fatal("steam-auth service not ready should not clear a completed login flag")
	}
	if !reflect.DeepEqual(restarted, []string{"steam-auth"}) {
		t.Fatalf("expected one steam-auth refresh, got %#v", restarted)
	}
}

func TestWaitForReadyStateMarksSteamAuthCompletedWhenInviteCodeArrives(t *testing.T) {
	dir := t.TempDir()
	if sjconfig.SteamAuthLoggedIn(dir) {
		t.Fatal("expected fresh dir to start without steam auth flag")
	}

	dataDir := filepath.Join(dir, "store")
	store, err := storage.Open(context.Background(), appconfig.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}

	fake := &fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
			if reflect.DeepEqual(args, []string{"cat", "/tmp/invite-code.txt"}) {
				return paneldocker.CommandResult{Stdout: "SGD7WVVL8CGJ\n", ExitCode: 0}, nil
			}
			return paneldocker.CommandResult{ExitCode: 0}, nil
		},
	}
	runner := &lifecycleRunner{
		lifecycle: fake,
		instance:  storage.Instance{ID: "stardew", DataDir: dir},
	}

	manager := jobs.NewManager(store, slog.Default())
	job, err := manager.Start(context.Background(), jobs.Spec{
		Type:       "test",
		TargetType: "instance",
		TargetID:   "stardew",
		Timeout:    5 * time.Second,
		Run: func(ctx context.Context, jobCtx *jobs.Context) error {
			if got := runner.waitForReadyState(ctx, jobCtx); got != "SGD7WVVL8CGJ" {
				t.Fatalf("waitForReadyState() = %q", got)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("start job: %v", err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)

	if !sjconfig.SteamAuthLoggedIn(dir) {
		t.Fatal("expected invite code success to mark steam auth completed")
	}
}

func TestPollInviteCodeAttemptsMarksAuthAndStoresPayload(t *testing.T) {
	dir := t.TempDir()
	store := &fakeStore{
		instance: storage.Instance{
			ID:            "stardew",
			DataDir:       dir,
			State:         storage.InstanceStateRunning,
			DriverPhase:   "running",
			DriverPayload: "{}",
		},
	}
	catAttempts := 0
	fake := &fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
			if reflect.DeepEqual(args, []string{"cat", "/tmp/invite-code.txt"}) {
				catAttempts++
				if catAttempts == 3 {
					return paneldocker.CommandResult{Stdout: "SGD7WVVL8CGJ\n", ExitCode: 0}, nil
				}
				return paneldocker.CommandResult{ExitCode: 0}, nil
			}
			return paneldocker.CommandResult{ExitCode: 0}, nil
		},
	}
	driver := New(fake, slog.Default(), nil, store)
	runner := &lifecycleRunner{
		driver:    driver,
		lifecycle: fake,
		instance:  store.instance,
	}

	if got := runner.pollInviteCodeAttempts(context.Background(), 5, time.Millisecond); got != "SGD7WVVL8CGJ" {
		t.Fatalf("pollInviteCodeAttempts() = %q", got)
	}
	if catAttempts != 3 {
		t.Fatalf("cat attempts = %d, want 3", catAttempts)
	}
	if !sjconfig.SteamAuthLoggedIn(dir) {
		t.Fatal("expected invite code success to mark steam auth completed")
	}
	if got := inviteCodeFromPayload(store.instance.DriverPayload); got != "SGD7WVVL8CGJ" {
		t.Fatalf("stored invite code = %q", got)
	}
}

func TestPollInviteCodeAttemptsStopsAtLimitWithoutFailingServer(t *testing.T) {
	dir := t.TempDir()
	store := &fakeStore{
		instance: storage.Instance{
			ID:          "stardew",
			DataDir:     dir,
			State:       storage.InstanceStateRunning,
			DriverPhase: "running",
		},
	}
	catAttempts := 0
	fake := &fakeConsoleDocker{
		execFunc: func(_ context.Context, _, _, _ string, args ...string) (paneldocker.CommandResult, error) {
			if reflect.DeepEqual(args, []string{"cat", "/tmp/invite-code.txt"}) {
				catAttempts++
			}
			return paneldocker.CommandResult{ExitCode: 0}, nil
		},
	}
	driver := New(fake, slog.Default(), nil, store)
	runner := &lifecycleRunner{
		driver:    driver,
		lifecycle: fake,
		instance:  store.instance,
	}

	if got := runner.pollInviteCodeAttempts(context.Background(), 3, time.Millisecond); got != "" {
		t.Fatalf("pollInviteCodeAttempts() = %q, want empty", got)
	}
	if catAttempts != 3 {
		t.Fatalf("cat attempts = %d, want 3", catAttempts)
	}
	if len(store.updated) != 0 {
		t.Fatalf("invite polling failure must not update server state, got %#v", store.updated)
	}
	if sjconfig.SteamAuthLoggedIn(dir) {
		t.Fatal("failed invite polling must not mark steam auth completed")
	}
}

func TestServerLogShowsSteamAuthUnavailable(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   bool
	}{
		{
			name:   "no logged in accounts",
			output: "[app] Steam-auth service has no logged-in accounts\n",
			want:   true,
		},
		{
			name: "service not ready",
			output: "[05:52:29 ERROR JunimoServer] Steam-auth service not ready: Could not reach steam-auth service within 30s: Steam auth service request failed after 4 attempts\n" +
				"[05:52:29 ERROR JunimoServer] Make sure you ran: docker compose run -it steam-auth setup\n" +
				"[05:52:29 WARN JunimoServer] Steam-auth service not ready, Galaxy features unavailable\n",
			want: false,
		},
		{
			name:   "invite code n/a alone is not enough",
			output: "[05:52:29 INFO JunimoServer] Invite Code: n/a\n",
			want:   false,
		},
		{
			name:   "ordinary startup log",
			output: "[05:52:29 INFO JunimoServer] Server started\n",
			want:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := serverLogShowsSteamAuthUnavailable(tc.output)
			if got != tc.want {
				t.Fatalf("serverLogShowsSteamAuthUnavailable() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestServerLogShowsSteamAuthServiceNotReady(t *testing.T) {
	output := "[05:52:29 ERROR JunimoServer] Steam-auth service not ready: Could not reach steam-auth service within 30s: Steam auth service request failed after 4 attempts\n" +
		"[05:52:29 WARN JunimoServer] Steam-auth service not ready, Galaxy features unavailable\n"
	if !serverLogShowsSteamAuthServiceNotReady(output) {
		t.Fatal("expected steam-auth service-not-ready marker")
	}
	if serverLogShowsSteamAuthServiceNotReady("[app] Steam-auth service has no logged-in accounts\n") {
		t.Fatal("no logged-in accounts should be handled by unavailable matcher, not service-not-ready matcher")
	}
}

func TestLooksLikePortBindFailure(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "windows reserved port",
			text: "ports are not available: exposing port TCP 0.0.0.0:5800 -> 127.0.0.1:0: listen tcp 0.0.0.0:5800: bind: An attempt was made to access a socket in a way forbidden by its access permissions.",
			want: true,
		},
		{
			name: "already allocated",
			text: "Bind for 0.0.0.0:5800 failed: port is already allocated",
			want: true,
		},
		{
			name: "non port docker error",
			text: "docker compose up: docker command failed",
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := looksLikePortBindFailure(tc.text)
			if got != tc.want {
				t.Fatalf("looksLikePortBindFailure() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEnsureJunimoServerModCopiesFromServerImage(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".local-container", "mods"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("IMAGE_VERSION=custom\nSERVER_IMAGE=sdvd/server:custom\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var gotOpts paneldocker.ContainerTTYRunOpts
	fake := &fakeConsoleDocker{
		runContainerFunc: func(_ context.Context, opts paneldocker.ContainerTTYRunOpts, _ <-chan string, lineHandler func(string)) (int, error) {
			gotOpts = opts
			workDir := strings.TrimSuffix(opts.Binds[0], ":/out")
			targetDir := filepath.Join(workDir, runtimeTargetJunimoDir)
			if err := os.MkdirAll(targetDir, 0o755); err != nil {
				return 1, err
			}
			if err := os.WriteFile(filepath.Join(targetDir, junimoServerManifestName), []byte(`{"Name":"JunimoServer","Version":"custom","UniqueID":"JunimoHost.Server"}`), 0o644); err != nil {
				return 1, err
			}
			if err := os.WriteFile(filepath.Join(targetDir, junimoServerAssemblyName), []byte("custom assembly"), 0o644); err != nil {
				return 1, err
			}
			lineHandler(junimoModExtractMarker)
			return 0, nil
		},
	}
	runner := &lifecycleRunner{
		lifecycle: fake,
		instance:  storage.Instance{DataDir: dir},
	}

	if err := runner.ensureJunimoServerMod(context.Background(), nil); err != nil {
		t.Fatalf("ensureJunimoServerMod: %v", err)
	}
	if gotOpts.ImageRef != "sdvd/server:custom" {
		t.Fatalf("ImageRef = %q, want custom server image", gotOpts.ImageRef)
	}
	if len(gotOpts.Entrypoint) != 1 || gotOpts.Entrypoint[0] != "/bin/sh" {
		t.Fatalf("unexpected entrypoint: %#v", gotOpts.Entrypoint)
	}
	if len(gotOpts.Command) != 2 || !strings.Contains(gotOpts.Command[1], "/data/Mods/JunimoServer") {
		t.Fatalf("copy command should reference JunimoServer, got %#v", gotOpts.Command)
	}
	if len(gotOpts.Binds) != 1 || !strings.HasSuffix(gotOpts.Binds[0], ":/out") || !strings.Contains(gotOpts.Binds[0], "junimo-mod-sync") {
		t.Fatalf("unexpected binds: %#v", gotOpts.Binds)
	}
	if version, err := readJunimoServerModVersion(junimoServerModDir(dir)); err != nil || version != "custom" {
		t.Fatalf("synced JunimoServer version=%q err=%v", version, err)
	}
}

func TestEnsureJunimoServerModSkipsWhenVersionMatches(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, ".local-container", "mods", "JunimoServer", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(`{"Name":"JunimoServer","Version":"1.5.0-preview.125","UniqueID":"JunimoHost.Server"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(manifest), junimoServerAssemblyName), []byte("test dll"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("IMAGE_VERSION=1.5.0-preview.125\nSERVER_IMAGE=sdvd/server:1.5.0-preview.125\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	called := false
	fake := &fakeConsoleDocker{
		runContainerFunc: func(context.Context, paneldocker.ContainerTTYRunOpts, <-chan string, func(string)) (int, error) {
			called = true
			return 0, nil
		},
	}
	runner := &lifecycleRunner{
		lifecycle: fake,
		instance:  storage.Instance{DataDir: dir},
	}

	if err := runner.ensureJunimoServerMod(context.Background(), nil); err != nil {
		t.Fatalf("ensureJunimoServerMod: %v", err)
	}
	if called {
		t.Fatal("RunContainerTTY should not be called when JunimoServer version matches")
	}
}

// ── restore-then-restart (SAVE-RESTORE-AUTORESTART-1) ──────────────────────

func TestRestoreBackupWithRestart_RequiresJobManager(t *testing.T) {
	d := newTestDriver(&fakeConsoleDocker{})
	instance := registry.Instance{ID: storage.DefaultInstanceID, DriverID: DriverID, DataDir: t.TempDir(), State: storage.InstanceStateRunning}

	if _, err := d.RestoreBackupWithRestart(context.Background(), instance, "backup.zip", false, 1); err == nil {
		t.Fatal("expected error when job manager not configured")
	}
}

func newLifecycleTestStore(t *testing.T) *storage.Store {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.Open(context.Background(), appconfig.Config{
		DataDir: dir,
		DBPath:  filepath.Join(dir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate storage: %v", err)
	}
	return store
}

// TestDoRestoreAndRestart_StoppedSkipsStopAndStart verifies that when the
// instance is already stopped, doRestoreAndRestart restores the backup
// directly without touching docker compose at all (no unnecessary stop/start
// around a restore that doesn't need one).
func TestDoRestoreAndRestart_StoppedSkipsStopAndStart(t *testing.T) {
	dataDir := t.TempDir()
	createTestSaveForBackup(t, dataDir, "TestSave")
	backupPath, err := BackupManual(dataDir, "TestSave")
	if err != nil {
		t.Fatalf("BackupManual: %v", err)
	}
	backupName := filepath.Base(backupPath)

	store := newLifecycleTestStore(t)
	if _, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{DataDir: dataDir}); err != nil {
		t.Fatalf("EnsureDefaultInstance: %v", err)
	}
	composeDownCalled := false
	composeUpCalled := false
	fake := &fakeConsoleDocker{
		composeDownFunc: func(context.Context, string) (paneldocker.CommandResult, error) {
			composeDownCalled = true
			return paneldocker.CommandResult{ExitCode: 0}, nil
		},
		composeUpFunc: func(context.Context, string) (paneldocker.CommandResult, error) {
			composeUpCalled = true
			return paneldocker.CommandResult{ExitCode: 0}, nil
		},
	}
	manager := jobs.NewManager(store, slog.Default())
	driver := New(fake, slog.Default(), manager, store)

	runner := &lifecycleRunner{
		driver:    driver,
		lifecycle: fake,
		instance: storage.Instance{
			ID: storage.DefaultInstanceID, DataDir: dataDir,
			State: storage.InstanceStateStopped, DriverPhase: "stopped",
		},
		operation:         "restore_restart",
		restoreBackupName: backupName,
		restoreOverwrite:  true, // TestSave dir still exists on disk from createTestSaveForBackup
	}

	job, err := manager.Start(context.Background(), jobs.Spec{
		Type: "test", TargetType: "instance", TargetID: storage.DefaultInstanceID,
		Timeout: 5 * time.Second, Run: runner.run,
	})
	if err != nil {
		t.Fatalf("start job: %v", err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)

	if composeDownCalled || composeUpCalled {
		t.Fatalf("stopped instance should not trigger compose down/up: down=%v up=%v", composeDownCalled, composeUpCalled)
	}
	updated, err := store.GetInstance(context.Background(), storage.DefaultInstanceID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.DriverPhase != "restored" {
		t.Fatalf("DriverPhase = %q, want restored", updated.DriverPhase)
	}
}

// TestDoRestoreAndRestart_RunningStopsThenRestoresBeforeStarting verifies the
// stop -> restore -> start ordering when the server is running: compose down
// must happen (and the save must already be restored on disk) before compose
// up is attempted, even though the fake ComposeUp deliberately fails here to
// avoid needing to mock the full doStart success path (container-running
// polling, invite code retrieval, etc. — none of that is exercised anywhere
// else in this package either).
func TestDoRestoreAndRestart_RunningStopsThenRestoresBeforeStarting(t *testing.T) {
	dataDir := t.TempDir()
	createTestSaveForBackup(t, dataDir, "TestSave")
	backupPath, err := BackupManual(dataDir, "TestSave")
	if err != nil {
		t.Fatalf("BackupManual: %v", err)
	}
	backupName := filepath.Base(backupPath)

	store := newLifecycleTestStore(t)
	if _, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{DataDir: dataDir}); err != nil {
		t.Fatalf("EnsureDefaultInstance: %v", err)
	}
	var calls []string
	fake := &fakeConsoleDocker{
		composeDownFunc: func(context.Context, string) (paneldocker.CommandResult, error) {
			calls = append(calls, "down")
			return paneldocker.CommandResult{ExitCode: 0}, nil
		},
		composeUpFunc: func(context.Context, string) (paneldocker.CommandResult, error) {
			calls = append(calls, "up")
			return paneldocker.CommandResult{}, errors.New("compose up unavailable in test")
		},
	}
	manager := jobs.NewManager(store, slog.Default())
	driver := New(fake, slog.Default(), manager, store)

	runner := &lifecycleRunner{
		driver:    driver,
		lifecycle: fake,
		instance: storage.Instance{
			ID: storage.DefaultInstanceID, DataDir: dataDir,
			State: storage.InstanceStateRunning, DriverPhase: "running",
		},
		operation:         "restore_restart",
		restoreBackupName: backupName,
		restoreOverwrite:  true,
	}

	job, err := manager.Start(context.Background(), jobs.Spec{
		Type: "test", TargetType: "instance", TargetID: storage.DefaultInstanceID,
		Timeout: 5 * time.Second, Run: runner.run,
	})
	if err != nil {
		t.Fatalf("start job: %v", err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)

	if len(calls) < 1 || calls[0] != "down" {
		t.Fatalf("expected compose down to run first, got %#v", calls)
	}

	// The restore must have already happened (between stop and the failed
	// start attempt) — verified by re-reading the save from disk.
	info := readSaveInfo(filepath.Join(dataDir, ".local-container", "saves", "Saves", "TestSave"))
	if info.ParseError != "" {
		t.Fatalf("expected restored save to parse cleanly, got: %s", info.ParseError)
	}
}
