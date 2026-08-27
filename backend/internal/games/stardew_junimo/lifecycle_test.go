package stardew_junimo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	appconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sjconfig "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func enableSteamInviteForLifecycleTest(t *testing.T, dataDir string) {
	t.Helper()
	if err := sjconfig.SetSteamInviteEnabled(dataDir, true); err != nil {
		t.Fatalf("enable Steam invite fixture: %v", err)
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

func TestSteamInviteWarmupUsesDedicatedPayloadAndSurvivesOrdinaryUpdates(t *testing.T) {
	dataDir := t.TempDir()
	enableSteamInviteForLifecycleTest(t, dataDir)
	original := time.Date(2020, 8, 27, 8, 0, 0, 0, time.UTC)
	payload := mergeSteamInviteWarmupStartedAt(`{"kept":true}`, original)
	store := &fakeStore{instance: storage.Instance{
		ID: "stardew", DataDir: dataDir, State: storage.InstanceStateRunning, DriverPayload: payload,
	}}
	driver := New(nil, slog.Default(), nil, store)

	driver.updatePhase(context.Background(), "stardew", storage.InstanceStateRunning, "ordinary payload update", "running", "")
	unchanged, ok := SteamInviteWarmupStartedAt(store.instance.DriverPayload)
	if !ok || !unchanged.Equal(original) {
		t.Fatalf("ordinary running update changed warmup marker to %v, ok=%v", unchanged, ok)
	}

	store.instance.DriverPayload = mergeInviteCodeInPayload(store.instance.DriverPayload, "LOCAL-CODE")
	unchanged, ok = SteamInviteWarmupStartedAt(store.instance.DriverPayload)
	if !ok || !unchanged.Equal(original) {
		t.Fatalf("invite-code payload write changed warmup marker to %v, ok=%v", unchanged, ok)
	}

	driver.updatePhaseWithSteamInviteWarmup(context.Background(), "stardew", storage.InstanceStateRunning, "new runtime generation", "running", "")
	refreshed, ok := SteamInviteWarmupStartedAt(store.instance.DriverPayload)
	if !ok || !refreshed.After(original) {
		t.Fatalf("explicit runtime generation did not refresh warmup marker: %v, ok=%v", refreshed, ok)
	}
}

func TestGetInviteCodeMissingFileReturnsEmptyWithoutAttachCLI(t *testing.T) {
	var callsMu sync.Mutex
	var calls [][]string
	fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, stdin string, args ...string) (paneldocker.CommandResult, error) {
		callsMu.Lock()
		calls = append(calls, append([]string(nil), args...))
		callsMu.Unlock()
		if stdin != "" {
			t.Fatalf("invite code read sent stdin %q", stdin)
		}
		if !reflect.DeepEqual(args, []string{"sh", "-c", inviteCodeReadScript}) {
			return paneldocker.CommandResult{ExitCode: 1}, errors.New("missing invite file was read with a failing command")
		}
		return paneldocker.CommandResult{ExitCode: 0}, nil
	}}
	instanceDir := t.TempDir()
	enableSteamInviteForLifecycleTest(t, instanceDir)
	store := &fakeStore{instance: storage.Instance{ID: "stardew", DataDir: instanceDir, State: storage.InstanceStateRunning}}
	driver := New(fake, nil, nil, store)

	code, err := driver.GetInviteCode(context.Background(), registry.Instance{ID: "stardew"})
	if err != nil || code != "" {
		t.Fatalf("GetInviteCode() = %q, %v; want empty", code, err)
	}
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], []string{"sh", "-c", inviteCodeReadScript}) {
		t.Fatalf("runtime calls = %#v; attach-cli must never run", calls)
	}
}

func TestGetInviteCodeDisabledDoesNotUseDocker(t *testing.T) {
	fake := &fakeConsoleDocker{execFunc: func(context.Context, string, string, string, ...string) (paneldocker.CommandResult, error) {
		t.Fatal("disabled invite lookup must not call Docker")
		return paneldocker.CommandResult{}, nil
	}}
	store := &fakeStore{instance: storage.Instance{ID: "stardew", DataDir: t.TempDir(), State: storage.InstanceStateRunning}}
	driver := New(fake, nil, nil, store)
	code, err := driver.GetInviteCode(context.Background(), registry.Instance{ID: "stardew"})
	if code != "" || !errors.Is(err, ErrSteamInviteDisabled) {
		t.Fatalf("GetInviteCode disabled = %q, %v", code, err)
	}
}

func TestGetInviteCodeConcurrentRequestsUseCacheAndSingleflight(t *testing.T) {
	var catCalls atomic.Int32
	fake := &fakeConsoleDocker{execFunc: func(_ context.Context, _, _, stdin string, args ...string) (paneldocker.CommandResult, error) {
		if stdin != "" || !reflect.DeepEqual(args, []string{"sh", "-c", inviteCodeReadScript}) {
			t.Errorf("unexpected invite runtime call stdin=%q args=%#v", stdin, args)
		}
		catCalls.Add(1)
		time.Sleep(25 * time.Millisecond)
		return paneldocker.CommandResult{Stdout: "SGD7WVVL8CGJ\n", ExitCode: 0}, nil
	}}
	instanceDir := t.TempDir()
	enableSteamInviteForLifecycleTest(t, instanceDir)
	store := &fakeStore{instance: storage.Instance{ID: "stardew", DataDir: instanceDir, State: storage.InstanceStateRunning}}
	driver := New(fake, nil, nil, store)

	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, err := driver.GetInviteCode(context.Background(), registry.Instance{ID: "stardew"})
			if err != nil || code != "SGD7WVVL8CGJ" {
				t.Errorf("GetInviteCode() = %q, %v", code, err)
			}
		}()
	}
	wg.Wait()
	if got := catCalls.Load(); got != 1 {
		t.Fatalf("cat calls = %d, want 1", got)
	}
	if _, err := driver.GetInviteCode(context.Background(), registry.Instance{ID: "stardew"}); err != nil {
		t.Fatal(err)
	}
	if got := catCalls.Load(); got != 1 {
		t.Fatalf("cached cat calls = %d, want 1", got)
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

func TestClearDriverPayloadInviteCodePreservesLifecycleStateAndOtherPayload(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.Open(context.Background(), appconfig.Config{
		DataDir: dataDir,
		DBPath:  filepath.Join(dataDir, "panel.db"),
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	instanceDir := filepath.Join(dataDir, "instances", storage.DefaultInstanceID)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID:       storage.DefaultInstanceID,
		DriverID: storage.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  instanceDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	instance, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID:            instance.ID,
		State:         storage.InstanceStateRunning,
		StateMessage:  "running fixture",
		DriverPhase:   "running",
		DriverPayload: `{"invite_code":"OLD-CODE","kept":true,"steam_invite_warmup_started_at":"2026-08-27T08:00:00Z"}`,
	})
	if err != nil {
		t.Fatalf("seed instance payload: %v", err)
	}
	driver := New(nil, slog.Default(), nil, store)
	driver.inviteCodeCache[instance.ID] = inviteCodeCacheEntry{code: "OLD-CODE", expiresAt: time.Now().Add(time.Minute)}

	driver.clearDriverPayloadInviteCode(context.Background(), instance.ID)

	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("load cleared instance: %v", err)
	}
	if updated.State != storage.InstanceStateRunning || updated.DriverPhase != "running" || updated.StateMessage.String != "running fixture" {
		t.Fatalf("clearing invite code changed lifecycle state: %#v", updated)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(updated.DriverPayload), &payload); err != nil {
		t.Fatalf("parse cleared payload: %v", err)
	}
	if _, ok := payload["invite_code"]; ok || payload["kept"] != true || payload["steam_invite_warmup_started_at"] != "2026-08-27T08:00:00Z" {
		t.Fatalf("cleared payload = %q, want invite removed and unrelated fields preserved", updated.DriverPayload)
	}
	if _, ok := driver.inviteCodeCache[instance.ID]; ok {
		t.Fatal("invite code cache was not cleared")
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
	instanceDir := t.TempDir()
	enableSteamInviteForLifecycleTest(t, instanceDir)
	runner := &lifecycleRunner{
		lifecycle: fake,
		instance: storage.Instance{
			DataDir:       instanceDir,
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
	instanceDir := t.TempDir()
	enableSteamInviteForLifecycleTest(t, instanceDir)
	runner := &lifecycleRunner{
		lifecycle: fake,
		instance: storage.Instance{
			DataDir:       instanceDir,
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

func TestTailServerLogsPreservesSteamAuthCompletedWhileSessionRestores(t *testing.T) {
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

	if !sjconfig.SteamAuthLoggedIn(dir) {
		t.Fatal("transient no-account startup log must not invalidate the saved steam auth session")
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
	enableSteamInviteForLifecycleTest(t, dir)
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
			if reflect.DeepEqual(args, []string{"sh", "-c", inviteCodeReadScript}) {
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
	enableSteamInviteForLifecycleTest(t, dir)
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
			if reflect.DeepEqual(args, []string{"sh", "-c", inviteCodeReadScript}) {
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
	enableSteamInviteForLifecycleTest(t, dir)
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
			if reflect.DeepEqual(args, []string{"sh", "-c", inviteCodeReadScript}) {
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

func TestRestartRejectsSecondActiveRestart(t *testing.T) {
	store := newLifecycleTestStore(t)
	dataDir := t.TempDir()
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: DriverID, Name: "Stardew", DataDir: dataDir,
	})
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	manager := jobs.NewManager(store, slog.Default())
	release := make(chan struct{})
	job, err := manager.Start(context.Background(), jobs.Spec{
		Type: lifecycleJobType, TargetType: "instance", TargetID: instance.ID,
		Payload: restartJobPayload, Run: func(ctx context.Context, _ *jobs.Context) error {
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
	if err != nil {
		t.Fatalf("start first restart fixture: %v", err)
	}
	t.Cleanup(func() { close(release) })

	driver := New(&fakeConsoleDocker{}, slog.Default(), manager, store)
	err = driver.Restart(context.Background(), makeRegistryInstanceForTest(instance))
	if !errors.Is(err, ErrRestartInProgress) {
		t.Fatalf("second Restart error = %v, want ErrRestartInProgress", err)
	}
	active, listErr := manager.Active(context.Background(), storage.ListActiveJobsFilter{
		TargetType: "instance", TargetID: instance.ID, Types: []string{lifecycleJobType},
	})
	if listErr != nil || len(active) != 1 || active[0].ID != job.ID {
		t.Fatalf("active restarts = %#v, err=%v", active, listErr)
	}
}

func TestStartReusesActiveStartWithoutCancelOrSecondRunner(t *testing.T) {
	store := newLifecycleTestStore(t)
	dataDir := t.TempDir()
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: DriverID, Name: "Stardew", DataDir: dataDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	manager := jobs.NewManager(store, slog.Default())
	release := make(chan struct{})
	finished := make(chan struct{})
	job, err := manager.Start(context.Background(), jobs.Spec{
		Type: lifecycleJobType, TargetType: "instance", TargetID: instance.ID, Payload: startJobPayload,
		Run: func(ctx context.Context, _ *jobs.Context) error {
			defer close(finished)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var releaseOnce sync.Once
	releaseJob := func() {
		releaseOnce.Do(func() { close(release) })
		<-finished
		waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
	}
	t.Cleanup(releaseJob)

	driver := New(&fakeConsoleDocker{}, slog.Default(), manager, store)
	reused, err := driver.Start(context.Background(), registry.StartRequest{Instance: makeRegistryInstanceForTest(instance)})
	if err != nil {
		t.Fatalf("retry Start: %v", err)
	}
	if reused.ID != job.ID {
		t.Fatalf("retry job = %q, want existing %q", reused.ID, job.ID)
	}
	select {
	case <-finished:
		t.Fatal("retry Start canceled the active start runner")
	default:
	}
	active, err := manager.Active(context.Background(), storage.ListActiveJobsFilter{
		TargetType: "instance", TargetID: instance.ID, Types: []string{lifecycleJobType},
	})
	if err != nil || len(active) != 1 || active[0].ID != job.ID {
		t.Fatalf("active lifecycle jobs = %#v, err=%v", active, err)
	}
}

func TestStartRejectsDifferentActiveLifecycleWithoutCancel(t *testing.T) {
	store := newLifecycleTestStore(t)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: DriverID, Name: "Stardew", DataDir: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	manager := jobs.NewManager(store, slog.Default())
	release := make(chan struct{})
	finished := make(chan struct{})
	job, err := manager.Start(context.Background(), jobs.Spec{
		Type: lifecycleJobType, TargetType: "instance", TargetID: instance.ID, Payload: stopJobPayload,
		Run: func(ctx context.Context, _ *jobs.Context) error {
			defer close(finished)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		close(release)
		<-finished
	})

	driver := New(&fakeConsoleDocker{}, slog.Default(), manager, store)
	_, err = driver.Start(context.Background(), registry.StartRequest{Instance: makeRegistryInstanceForTest(instance)})
	if !errors.Is(err, ErrLifecycleInProgress) {
		t.Fatalf("Start error = %v, want ErrLifecycleInProgress", err)
	}
	select {
	case <-finished:
		t.Fatal("conflicting Start canceled the active lifecycle runner")
	default:
	}
	active, listErr := manager.Active(context.Background(), storage.ListActiveJobsFilter{
		TargetType: "instance", TargetID: instance.ID, Types: []string{lifecycleJobType},
	})
	if listErr != nil || len(active) != 1 || active[0].ID != job.ID {
		t.Fatalf("active lifecycle jobs = %#v, err=%v", active, listErr)
	}
}

func TestLifecycleRejectsActiveSteamInviteAuthorizationWithoutCancel(t *testing.T) {
	store := newLifecycleTestStore(t)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: DriverID, Name: "Stardew", DataDir: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	manager := jobs.NewManager(store, slog.Default())
	release := make(chan struct{})
	finished := make(chan struct{})
	job, err := manager.Start(context.Background(), jobs.Spec{
		Type: "stardew_steam_auth", TargetType: "instance", TargetID: instance.ID, Exclusive: true,
		Run: func(ctx context.Context, _ *jobs.Context) error {
			defer close(finished)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var releaseOnce sync.Once
	releaseJob := func() {
		releaseOnce.Do(func() { close(release) })
		<-finished
		waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusSucceeded)
	}
	t.Cleanup(releaseJob)

	driver := New(&fakeConsoleDocker{}, slog.Default(), manager, store)
	if _, err := driver.Start(context.Background(), registry.StartRequest{Instance: makeRegistryInstanceForTest(instance)}); !errors.Is(err, ErrLifecycleInProgress) {
		t.Fatalf("Start error = %v, want ErrLifecycleInProgress", err)
	}
	if err := driver.Stop(context.Background(), makeRegistryInstanceForTest(instance)); !errors.Is(err, ErrLifecycleInProgress) {
		t.Fatalf("Stop error = %v, want ErrLifecycleInProgress", err)
	}
	if err := driver.Restart(context.Background(), makeRegistryInstanceForTest(instance)); !errors.Is(err, ErrLifecycleInProgress) {
		t.Fatalf("Restart error = %v, want ErrLifecycleInProgress", err)
	}
	select {
	case <-finished:
		t.Fatal("lifecycle conflict canceled the active Steam invite authorization")
	default:
	}
	active, listErr := manager.Active(context.Background(), storage.ListActiveJobsFilter{
		TargetType: "instance", TargetID: instance.ID, Types: []string{"stardew_steam_auth"},
	})
	if listErr != nil || len(active) != 1 || active[0].ID != job.ID {
		t.Fatalf("active authorization jobs = %#v, err=%v", active, listErr)
	}
	releaseJob()
}

func TestNewGameStartIdempotencySurvivesConcurrentActiveAndTerminalRetries(t *testing.T) {
	_, store, instance, manager := prepareNewGameFailureDeferTest(t)
	var err error
	instance, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: instance.ID, State: storage.InstanceStateAdminCreated, StateMessage: "fixture", DriverPhase: "fixture",
	})
	if err != nil {
		t.Fatal(err)
	}
	releaseVerify := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseVerify) }) }
	t.Cleanup(release)
	var composePsCalls atomic.Int32
	fake := &fakeConsoleDocker{
		composePsFunc: func(context.Context, string) (paneldocker.ComposePsResult, error) {
			composePsCalls.Add(1)
			return paneldocker.ComposePsResult{}, nil
		},
		runContainerFunc: func(ctx context.Context, _ paneldocker.ContainerTTYRunOpts, _ <-chan string, _ func(string)) (int, error) {
			select {
			case <-releaseVerify:
				return 1, errors.New("release idempotency fixture")
			case <-ctx.Done():
				return 1, ctx.Err()
			}
		},
	}
	driver := New(fake, slog.Default(), manager, store)
	cfg := newGameTestConfig("standard")
	request := registry.StartRequest{
		Instance:      makeRegistryInstanceForTest(instance),
		NewGame:       true,
		NewGameConfig: &cfg,
		RequestID:     "request-concurrent-idempotency",
	}

	const callers = 12
	start := make(chan struct{})
	type result struct {
		job *registry.Job
		err error
	}
	results := make(chan result, callers)
	for range callers {
		go func() {
			<-start
			job, err := driver.Start(context.Background(), request)
			results <- result{job: job, err: err}
		}()
	}
	close(start)

	var originalJobID string
	for range callers {
		got := <-results
		if got.err != nil {
			t.Fatalf("concurrent new-game Start: %v", got.err)
		}
		if got.job == nil || got.job.ID == "" {
			t.Fatalf("concurrent new-game Start returned %#v", got.job)
		}
		if originalJobID == "" {
			originalJobID = got.job.ID
		} else if got.job.ID != originalJobID {
			t.Fatalf("concurrent retry job=%q, want original %q", got.job.ID, originalJobID)
		}
	}
	if got := composePsCalls.Load(); got != 1 {
		t.Fatalf("ComposePs calls during concurrent acceptance = %d, want exactly one new request", got)
	}

	conflicting := cfg
	conflicting.FarmerName = "Different Farmer"
	request.NewGameConfig = &conflicting
	if _, err := driver.Start(context.Background(), request); err == nil {
		t.Fatal("same request ID with different config unexpectedly succeeded")
	} else {
		var ownerErr *NewGameOwnerError
		if !errors.As(err, &ownerErr) || ownerErr.Code != "new_game_request_conflict" {
			t.Fatalf("different-config retry error = %v, want new_game_request_conflict", err)
		}
	}

	release()
	waitForDriverTestJobStatus(t, store, originalJobID, storage.JobStatusFailed)
	request.NewGameConfig = &cfg
	replayed, err := driver.Start(context.Background(), request)
	if err != nil {
		t.Fatalf("terminal retry: %v", err)
	}
	if replayed.ID != originalJobID {
		t.Fatalf("terminal retry job=%q, want original %q", replayed.ID, originalJobID)
	}
}

func makeRegistryInstanceForTest(instance storage.Instance) registry.Instance {
	return registry.Instance{
		ID: instance.ID, DriverID: instance.DriverID, Name: instance.Name, DataDir: instance.DataDir,
		State: instance.State, DriverPhase: instance.DriverPhase, DriverPayload: instance.DriverPayload,
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

func TestNewGameSMAPIBundledSyncFailureRollsBackPersistentTransaction(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "stardew")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store := newLifecycleTestStore(t)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: storage.DefaultDriverID, Name: "Stardew Valley", DataDir: dataDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: instance.ID, State: storage.InstanceStateStopped, StateMessage: "stopped", DriverPhase: "stopped",
	})
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeConsoleDocker{runContainerFunc: func(_ context.Context, opts paneldocker.ContainerTTYRunOpts, _ <-chan string, _ func(string)) (int, error) {
		command := strings.Join(opts.Command, " ")
		if strings.Contains(command, "anxi-install-verify") {
			return 0, nil
		}
		if strings.Contains(command, smapiBundledSyncMarker) {
			return smapiBundledSourceMissing, nil
		}
		return 1, fmt.Errorf("unexpected one-shot command: %s", command)
	}}
	manager := jobs.NewManager(store, slog.Default())
	driver := New(fake, slog.Default(), manager, store)
	config := registry.NewGameConfig{
		FarmName: "FirstFarm", FarmType: "standard", StartingCabins: 1, MaxPlayers: 4,
		CabinLayout: "nearby", ProfitMargin: "100", MoneyMode: "shared",
	}
	runner := &lifecycleRunner{
		driver: driver, lifecycle: fake, instance: instance, operation: "start", newGame: true, newGameConfig: &config,
	}
	job, err := manager.Start(context.Background(), jobs.Spec{
		Type: lifecycleJobType, TargetType: "instance", TargetID: instance.ID, Timeout: 5 * time.Second, Run: runner.run,
	})
	if err != nil {
		t.Fatal(err)
	}
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.DriverPhase != "smapi_bundled_sync_failed" {
		t.Fatalf("driver phase = %s, want smapi_bundled_sync_failed", updated.DriverPhase)
	}
	entries, err := os.ReadDir(newGameTransactionsDir(dataDir))
	if err != nil || len(entries) != 1 {
		t.Fatalf("transaction entries = %d, err=%v", len(entries), err)
	}
	record, err := LoadNewGameTransaction(dataDir, entries[0].Name())
	if err != nil {
		t.Fatal(err)
	}
	if record.Stage != newGameStateRolledBack || !record.RollbackCompleted || record.ErrorCode != "smapi_bundled_sync_failed" {
		t.Fatalf("rolled-back transaction = %#v", record)
	}
	if _, err := LoadNewGameOwner(dataDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new-game owner after rollback = %v", err)
	}
}

func TestNewGameFailureDeferPreservesLateCreationProgressWithoutComposeDown(t *testing.T) {
	dataDir, store, instance, manager := prepareNewGameFailureDeferTest(t)
	var downs atomic.Int32
	fake := newGameFailureDeferDocker(t, dataDir)
	fake.composePsFunc = func(context.Context, string) (paneldocker.ComposePsResult, error) {
		if err := os.MkdirAll(filepath.Join(savesDir(dataDir), "Saves", "Late_777"), 0o755); err != nil {
			return paneldocker.ComposePsResult{}, err
		}
		if err := writeGameloaderPointer(dataDir, "Late_777"); err != nil {
			return paneldocker.ComposePsResult{}, err
		}
		return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
			Service: "server", State: "exited", Status: "Exited (1)", ExitCode: 1,
		}}}, nil
	}
	fake.composeDownFunc = func(context.Context, string) (paneldocker.CommandResult, error) {
		downs.Add(1)
		return paneldocker.CommandResult{ExitCode: 0}, nil
	}
	driver := New(fake, slog.Default(), manager, store)
	job := startNewGameFailureDeferJob(t, manager, driver, fake, instance, "request-late-progress")
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)

	if downs.Load() != 0 {
		t.Fatalf("ComposeDown calls = %d, want 0 after late progress", downs.Load())
	}
	owner, err := LoadNewGameOwner(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	record, err := LoadNewGameTransaction(dataDir, owner.TransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if record.Stage != newGameStateUnknown || !record.ProgressObserved || record.ProgressSave != "Late_777" || record.RollbackCompleted {
		t.Fatalf("preserved transaction = %#v", record)
	}
	if _, err := os.Stat(filepath.Join(savesDir(dataDir), "Saves", "Late_777")); err != nil {
		t.Fatalf("late save directory was not preserved: %v", err)
	}
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != storage.InstanceStateError || updated.DriverPhase != "new_game_recovery_required" {
		t.Fatalf("instance after late progress = %#v", updated)
	}
}

func TestNewGameFailureDeferComposeDownFailureKeepsOwnerAndSkipsRollback(t *testing.T) {
	dataDir, store, instance, manager := prepareNewGameFailureDeferTest(t)
	var downs atomic.Int32
	fake := newGameFailureDeferDocker(t, dataDir)
	fake.composePsFunc = func(context.Context, string) (paneldocker.ComposePsResult, error) {
		return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
			Service: "server", State: "exited", Status: "Exited (1)", ExitCode: 1,
		}}}, nil
	}
	fake.composeDownFunc = func(context.Context, string) (paneldocker.CommandResult, error) {
		downs.Add(1)
		return paneldocker.CommandResult{ExitCode: 1, Stderr: "injected down failure"}, errors.New("injected down failure")
	}
	driver := New(fake, slog.Default(), manager, store)
	job := startNewGameFailureDeferJob(t, manager, driver, fake, instance, "request-down-failure")
	waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)

	if downs.Load() != 1 {
		t.Fatalf("ComposeDown calls = %d, want 1", downs.Load())
	}
	owner, err := LoadNewGameOwner(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	record, err := LoadNewGameTransaction(dataDir, owner.TransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if record.Stage != newGameStateRollbackFail || record.RollbackCompleted || !strings.Contains(record.RollbackError, "injected down failure") {
		t.Fatalf("failed-stop transaction = %#v", record)
	}
	if _, err := os.Stat(newGamePendingPath(dataDir)); err != nil {
		t.Fatalf("pending marker was rolled back after unconfirmed stop: %v", err)
	}
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != storage.InstanceStateError || updated.DriverPhase != "new_game_rollback_failed" {
		t.Fatalf("instance after down failure = %#v", updated)
	}
}

func TestNewGameComposeUpPartialFailureMustConfirmDownBeforeRollback(t *testing.T) {
	for _, tc := range []struct {
		name      string
		downErr   error
		wantOwner bool
		wantStage NewGameTransactionState
	}{
		{name: "down confirmed", wantStage: newGameStateRolledBack},
		{name: "down unconfirmed", downErr: errors.New("injected partial-up cleanup failure"), wantOwner: true, wantStage: newGameStateRollbackFail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir, store, instance, manager := prepareNewGameFailureDeferTest(t)
			fake := newGameFailureDeferDocker(t, dataDir)
			fake.composeUpFunc = func(context.Context, string) (paneldocker.CommandResult, error) {
				return paneldocker.CommandResult{ExitCode: 1, Stderr: "server container may already be running"}, errors.New("injected compose up partial failure")
			}
			var downs atomic.Int32
			fake.composeDownFunc = func(context.Context, string) (paneldocker.CommandResult, error) {
				downs.Add(1)
				if tc.downErr != nil {
					return paneldocker.CommandResult{ExitCode: 1, Stderr: tc.downErr.Error()}, tc.downErr
				}
				return paneldocker.CommandResult{ExitCode: 0}, nil
			}
			driver := New(fake, slog.Default(), manager, store)
			job := startNewGameFailureDeferJob(t, manager, driver, fake, instance, "request-partial-up-"+strings.ReplaceAll(tc.name, " ", "-"))
			waitForDriverTestJobStatus(t, store, job.ID, storage.JobStatusFailed)
			if downs.Load() != 1 {
				t.Fatalf("ComposeDown calls = %d, want 1", downs.Load())
			}
			entries, err := os.ReadDir(newGameTransactionsDir(dataDir))
			if err != nil || len(entries) != 1 {
				t.Fatalf("transaction entries = %d, err=%v", len(entries), err)
			}
			record, err := LoadNewGameTransaction(dataDir, entries[0].Name())
			if err != nil {
				t.Fatal(err)
			}
			if record.Stage != tc.wantStage {
				t.Fatalf("stage = %s, want %s; record=%#v", record.Stage, tc.wantStage, record)
			}
			_, ownerErr := LoadNewGameOwner(dataDir)
			if tc.wantOwner && ownerErr != nil {
				t.Fatalf("owner should be retained: %v", ownerErr)
			}
			if !tc.wantOwner && !errors.Is(ownerErr, os.ErrNotExist) {
				t.Fatalf("terminal rollback owner = %v", ownerErr)
			}
		})
	}
}

func TestManualStartRetriesRollbackOnlyAndNeverStartsGame(t *testing.T) {
	dataDir, store, instance, manager := prepareNewGameFailureDeferTest(t)
	oldSettings := []byte(`{"Server":{"MaxPlayers":2}}`)
	newSettings := []byte(`{"Server":{"MaxPlayers":8}}`)
	writeNewGameSnapshotFixture(t, serverSettingsPath(dataDir), oldSettings)
	cfg := newGameTestConfig("standard")
	tx, _, err := beginOrResumeNewGameTransactionWithJobStatus(
		dataDir, cfg, "request-rollback-recovery", "terminated-original-job",
		func(string) (bool, error) { return false, nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	writeNewGameSnapshotFixture(t, serverSettingsPath(dataDir), newSettings)
	writeNewGameTestRawSave(t, dataDir, "RollbackOnly_73", `<SaveGame>`)
	if err := tx.beginRollback(errors.New("original new-game failure"), "new_game_failed", newGameStateFailed); err != nil {
		t.Fatal(err)
	}
	if err := tx.failRollback(errors.New("initial ComposeDown failure")); err == nil {
		t.Fatal("expected initial rollback failure")
	}

	var running atomic.Bool
	running.Store(true)
	var failDown atomic.Bool
	failDown.Store(true)
	var downs, ups, execs atomic.Int32
	fake := &fakeConsoleDocker{
		composePsFunc: func(context.Context, string) (paneldocker.ComposePsResult, error) {
			if !running.Load() {
				return paneldocker.ComposePsResult{}, nil
			}
			return paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{
				Service: "server", State: "running", Status: "Up",
			}}}, nil
		},
		composeDownFunc: func(context.Context, string) (paneldocker.CommandResult, error) {
			downs.Add(1)
			if failDown.Load() {
				return paneldocker.CommandResult{ExitCode: 1, Stderr: "injected down failure"}, errors.New("injected down failure")
			}
			running.Store(false)
			return paneldocker.CommandResult{ExitCode: 0}, nil
		},
		composeUpFunc: func(context.Context, string) (paneldocker.CommandResult, error) {
			ups.Add(1)
			return paneldocker.CommandResult{ExitCode: 0}, nil
		},
		execFunc: func(context.Context, string, string, string, ...string) (paneldocker.CommandResult, error) {
			execs.Add(1)
			return paneldocker.CommandResult{ExitCode: 0}, nil
		},
	}
	driver := New(fake, slog.Default(), manager, store)
	start := func(wantStatus string) {
		t.Helper()
		job, startErr := driver.Start(context.Background(), registry.StartRequest{Instance: registry.Instance{ID: instance.ID}})
		if startErr != nil {
			t.Fatal(startErr)
		}
		waitForDriverTestJobStatus(t, store, job.ID, wantStatus)
	}

	// First manual Start sees a still-running Compose project. Down fails, so no
	// file restore or owner release is allowed.
	start(storage.JobStatusFailed)
	owner, err := LoadNewGameOwner(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := LoadNewGameTransaction(dataDir, owner.TransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Stage != newGameStateRollbackFail || failed.RollbackCompleted {
		t.Fatalf("failed recovery transaction = %#v", failed)
	}
	if got, readErr := os.ReadFile(serverSettingsPath(dataDir)); readErr != nil || string(got) != string(newSettings) {
		t.Fatalf("settings changed while Compose stop was unconfirmed: %q, %v", got, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(savesDir(dataDir), "Saves", "RollbackOnly_73")); statErr != nil {
		t.Fatalf("save moved while Compose stop was unconfirmed: %v", statErr)
	}

	// A later manual Start may retry only the same rollback. Once Down and its
	// post-check succeed, journal replay finishes and leaves the game off.
	failDown.Store(false)
	start(storage.JobStatusSucceeded)
	if ups.Load() != 0 || execs.Load() != 0 {
		t.Fatalf("rollback recovery called forward game operations: ComposeUp=%d exec=%d", ups.Load(), execs.Load())
	}
	if downs.Load() != 2 {
		t.Fatalf("ComposeDown calls = %d, want 2", downs.Load())
	}
	if _, ownerErr := LoadNewGameOwner(dataDir); !errors.Is(ownerErr, os.ErrNotExist) {
		t.Fatalf("owner after complete rollback = %v", ownerErr)
	}
	final, err := LoadNewGameTransaction(dataDir, tx.record.TransactionID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Stage != newGameStateRolledBack || !final.RollbackCompleted {
		t.Fatalf("final transaction = %#v", final)
	}
	if got, readErr := os.ReadFile(serverSettingsPath(dataDir)); readErr != nil || string(got) != string(oldSettings) {
		t.Fatalf("settings not restored after recovery: %q, %v", got, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, ".local-container", "saves-quarantine", "new-game", tx.record.TransactionID, "RollbackOnly_73")); statErr != nil {
		t.Fatalf("new save not quarantined after recovery: %v", statErr)
	}
	updated, err := store.GetInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != storage.InstanceStateStopped || updated.DriverPhase != "new_game_rolled_back" {
		t.Fatalf("instance after rollback-only recovery = %#v", updated)
	}
}

func prepareNewGameFailureDeferTest(t *testing.T) (string, *storage.Store, storage.Instance, *jobs.Manager) {
	t.Helper()
	dataDir := filepath.Join(t.TempDir(), "stardew")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, ".env"), []byte("IMAGE_VERSION=1.5.0-preview.125\nSERVER_IMAGE=sdvd/server:1.5.0-preview.125\nGAME_LANGUAGE=zh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	junimoDir := junimoServerModDir(dataDir)
	if err := os.MkdirAll(junimoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(junimoDir, junimoServerManifestName), []byte(`{"Name":"JunimoServer","Version":"1.5.0-preview.125","UniqueID":"JunimoHost.Server"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(junimoDir, junimoServerAssemblyName), []byte("test dll"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := newLifecycleTestStore(t)
	instance, err := store.EnsureDefaultInstance(context.Background(), storage.EnsureDefaultInstanceParams{
		ID: storage.DefaultInstanceID, DriverID: storage.DefaultDriverID, Name: "Stardew Valley", DataDir: dataDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err = store.UpdateInstanceState(context.Background(), storage.UpdateInstanceStateParams{
		ID: instance.ID, State: storage.InstanceStateStopped, StateMessage: "stopped", DriverPhase: "stopped",
	})
	if err != nil {
		t.Fatal(err)
	}
	return dataDir, store, instance, jobs.NewManager(store, slog.Default())
}

func newGameFailureDeferDocker(t *testing.T, dataDir string) *fakeConsoleDocker {
	t.Helper()
	return &fakeConsoleDocker{
		runContainerFunc: func(_ context.Context, opts paneldocker.ContainerTTYRunOpts, _ <-chan string, lineHandler func(string)) (int, error) {
			command := strings.Join(opts.Command, " ")
			if strings.Contains(command, "anxi-install-verify") {
				return 0, nil
			}
			if strings.Contains(command, smapiBundledSyncMarker) {
				stage := ""
				for _, bind := range opts.Binds {
					if strings.HasSuffix(bind, ":/managed") {
						stage = strings.TrimSuffix(bind, ":/managed")
						break
					}
				}
				if stage == "" {
					return 1, errors.New("managed SMAPI stage bind missing")
				}
				for _, mod := range []struct{ folder, name, id string }{
					{folder: "ConsoleCommands", name: "Console Commands", id: consoleCommandsID},
					{folder: "SaveBackup", name: "Save Backup", id: saveBackupID},
				} {
					dir := filepath.Join(stage, mod.folder)
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return 1, err
					}
					manifest := fmt.Sprintf(`{"Name":%q,"UniqueID":%q,"Version":"1.0.0"}`, mod.name, mod.id)
					if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
						return 1, err
					}
				}
				if lineHandler != nil {
					lineHandler(smapiBundledSyncMarker + ": copy complete")
				}
				return 0, nil
			}
			return 1, fmt.Errorf("unexpected one-shot command for %s: %s", dataDir, command)
		},
		composeUpFunc: func(context.Context, string) (paneldocker.CommandResult, error) {
			return paneldocker.CommandResult{ExitCode: 0}, nil
		},
	}
}

func startNewGameFailureDeferJob(
	t *testing.T,
	manager *jobs.Manager,
	driver *Driver,
	fake *fakeConsoleDocker,
	instance storage.Instance,
	requestID string,
) storage.Job {
	t.Helper()
	config := registry.NewGameConfig{
		FarmName: "FirstFarm", FarmType: "standard", StartingCabins: 1, MaxPlayers: 4,
		CabinLayout: "nearby", ProfitMargin: "100", MoneyMode: "shared",
	}
	runner := &lifecycleRunner{
		driver: driver, lifecycle: fake, instance: instance, operation: "start", newGame: true,
		newGameConfig: &config, newGameRequestID: requestID,
	}
	job, err := manager.Start(context.Background(), jobs.Spec{
		Type: lifecycleJobType, TargetType: "instance", TargetID: instance.ID, Timeout: 10 * time.Second, Run: runner.run,
	})
	if err != nil {
		t.Fatal(err)
	}
	return job
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
