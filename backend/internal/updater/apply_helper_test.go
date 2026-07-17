package updater

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type applyScenarioExecutor struct {
	pullFail, composeFail, unhealthy, versionMismatch, rollbackFail bool
	currentNew                                                      bool
	calls                                                           [][]string
	oldImage, newImage                                              string
	imageList, containerImages                                      string
	imageIDs                                                        map[string]string
}

func (e *applyScenarioExecutor) Run(_ context.Context, args ...string) error {
	e.calls = append(e.calls, append([]string(nil), args...))
	if len(args) > 0 && args[0] == "pull" {
		if e.pullFail {
			return errors.New("pull failed")
		}
		return nil
	}
	if len(args) > 0 && args[0] == "stop" {
		e.currentNew = false
		return nil
	}
	if containsArg(args, "up") {
		if containsArg(args, "always") {
			e.currentNew = true
			if e.composeFail {
				return errors.New("compose failed")
			}
			return nil
		}
		e.currentNew = false
		if e.rollbackFail {
			return errors.New("rollback compose failed")
		}
	}
	return nil
}

func (e *applyScenarioExecutor) Output(_ context.Context, args ...string) (string, error) {
	e.calls = append(e.calls, append([]string(nil), args...))
	joined := strings.Join(args, " ")
	switch {
	case strings.Contains(joined, "image ls --no-trunc"):
		return e.imageList, nil
	case strings.Contains(joined, "container ls --all"):
		return e.containerImages, nil
	case strings.Contains(joined, "image inspect"):
		if id, ok := e.imageIDs[args[len(args)-1]]; ok {
			return id, nil
		}
		if strings.Contains(joined, e.newImage) {
			return e.newImage + "@sha256:new", nil
		}
		return e.oldImage + "@sha256:old", nil
	case strings.Contains(joined, "config --images panel"):
		envPath := argAfter(args, "--env-file")
		data, err := os.ReadFile(envPath)
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PANEL_IMAGE=") {
				return strings.TrimSpace(strings.TrimPrefix(line, "PANEL_IMAGE=")), nil
			}
		}
		return "", errors.New("image missing")
	case strings.Contains(joined, "inspect --format"):
		if e.currentNew && e.unhealthy {
			return "running|unhealthy", nil
		}
		return "running|healthy", nil
	case strings.Contains(joined, "/health"):
		return `{"status":"ok"}`, nil
	case strings.Contains(joined, "/api/version"):
		if e.currentNew {
			if e.versionMismatch {
				return `{"version":"0.1.99"}`, nil
			}
			return `{"version":"0.1.15"}`, nil
		}
		return `{"version":"0.1.14"}`, nil
	default:
		return "", nil
	}
}

func TestApplySuccessCleansTrustedPanelHistoryButProtectsContainersAndCustomImages(t *testing.T) {
	executor := &applyScenarioExecutor{}
	opts, _, _, _ := prepareApplyTest(t, executor)
	repo, _ := trustedRepositoryOf(executor.newImage)
	executor.imageList = strings.Join([]string{
		repo + "|0.1.15|" + executor.newImage + "@sha256:new",
		repo + "|0.1.13|sha256:stale",
		repo + "|0.1.12|sha256:used",
		repo + "|latest|sha256:latest",
		"custom.invalid/panel|0.1.11|sha256:custom",
	}, "\n")
	executor.containerImages = repo + ":0.1.12"
	executor.imageIDs = map[string]string{repo + ":0.1.13": "sha256:stale", repo + ":latest": "sha256:latest"}
	if err := RunApply(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	joined := flattenCalls(executor.calls)
	for _, want := range []string{"image rm " + repo + ":0.1.13", "image rm " + repo + ":latest"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing trusted history cleanup %q: %s", want, joined)
		}
	}
	for _, forbidden := range []string{repo + ":0.1.12", "custom.invalid/panel:0.1.11", executor.newImage} {
		if strings.Contains(joined, "image rm "+forbidden) {
			t.Fatalf("protected/custom image was removed %q: %s", forbidden, joined)
		}
	}
}

func containsArg(args []string, target string) bool {
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}
func argAfter(args []string, target string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == target {
			return args[i+1]
		}
	}
	return ""
}

func prepareApplyTest(t *testing.T, scenario *applyScenarioExecutor) (ApplyOptions, *ApplyStateStore, string, string) {
	t.Helper()
	dataDir := t.TempDir()
	installDir := t.TempDir()
	composeFile := filepath.Join(installDir, "docker-compose.yml")
	envFile := filepath.Join(installDir, ".env")
	oldImage := "anxiyizhi/stardew-server-anxi-panel:0.1.14"
	newImage := "anxiyizhi/stardew-server-anxi-panel:0.1.15"
	scenario.oldImage, scenario.newImage = oldImage, newImage
	if err := os.WriteFile(composeFile, []byte("services:\n  panel:\n    image: ${PANEL_IMAGE}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envFile, []byte("PANEL_IMAGE="+oldImage+"\nPANEL_SECRET=must-not-log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(dataDir, "updater", "backups", "apply-test")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "panel.db"), []byte("old-database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "panel.db"), []byte("possibly-migrated-database"), 0o600); err != nil {
		t.Fatal(err)
	}
	stateFile := filepath.Join(dataDir, "updater", "apply-status.json")
	store := NewApplyStateStore(stateFile)
	now := time.Now().UTC()
	if err := store.Write(ApplyStatus{UpdateID: "apply-test", Phase: PhaseBackingUp, FromVersion: "0.1.14", ToVersion: "0.1.15", OriginalImage: oldImage, OriginalDigest: oldImage + "@sha256:old", StartedAt: now, UpdatedAt: now, Logs: []LogEntry{}}); err != nil {
		t.Fatal(err)
	}
	return ApplyOptions{
		FromVersion: "0.1.14", TargetVersion: "0.1.15", CurrentImage: oldImage,
		OriginalDigest: oldImage + "@sha256:old", CurrentContainer: "anxi-panel-test",
		ComposeProject: "anxi-panel-test", ComposeFile: composeFile, StateFile: stateFile,
		BackupDir: backupDir, DatabaseRelative: "panel.db", DataDir: dataDir,
		HealthTimeout: 12 * time.Millisecond, PollInterval: time.Millisecond, Executor: scenario,
	}, store, envFile, filepath.Join(dataDir, "panel.db")
}

func TestApplySuccess(t *testing.T) {
	executor := &applyScenarioExecutor{}
	opts, store, envFile, _ := prepareApplyTest(t, executor)
	if err := RunApply(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	status, _ := store.Read()
	if status.Phase != PhaseSucceeded || status.SelectedImage != executor.newImage || status.SelectedDigest == "" || !status.CleanupCompleted {
		t.Fatalf("status=%+v", status)
	}
	env, _ := os.ReadFile(envFile)
	if !strings.Contains(string(env), "PANEL_IMAGE="+executor.newImage) {
		t.Fatalf("env=%s", env)
	}
	assertPanelOnlyCompose(t, executor.calls, opts.ComposeProject)
	joined := flattenCalls(executor.calls)
	if !strings.Contains(joined, "image rm "+executor.oldImage) || !strings.Contains(joined, "image prune --force --filter label=org.opencontainers.image.title=stardew-server-anxi-panel") {
		t.Fatalf("successful apply did not clean old panel images: %s", joined)
	}
}

func TestApplyFailureAndRollbackScenarios(t *testing.T) {
	tests := []struct {
		name                string
		configure           func(*applyScenarioExecutor)
		wantPhase, wantCode string
		restoredDB          bool
	}{
		{"image pull", func(e *applyScenarioExecutor) { e.pullFail = true }, PhaseFailedRolledBack, CodeImagePullFailed, false},
		{"compose recreate", func(e *applyScenarioExecutor) { e.composeFail = true }, PhaseFailedRolledBack, CodeComposeRecreateFailed, true},
		{"new unhealthy", func(e *applyScenarioExecutor) { e.unhealthy = true }, PhaseFailedRolledBack, CodeHealthCheckFailed, true},
		{"version mismatch", func(e *applyScenarioExecutor) { e.versionMismatch = true }, PhaseFailedRolledBack, CodeVersionMismatch, true},
		{"rollback failure", func(e *applyScenarioExecutor) { e.unhealthy = true; e.rollbackFail = true }, PhaseRollbackFailed, CodeRollbackFailed, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &applyScenarioExecutor{}
			tt.configure(executor)
			opts, store, envFile, dbFile := prepareApplyTest(t, executor)
			if err := RunApply(context.Background(), opts); err == nil {
				t.Fatal("expected apply failure")
			}
			status, err := store.Read()
			if err != nil || status.Phase != tt.wantPhase || status.ErrorCode != tt.wantCode {
				t.Fatalf("status=%+v err=%v", status, err)
			}
			env, _ := os.ReadFile(envFile)
			if !strings.Contains(string(env), "PANEL_IMAGE="+executor.oldImage) {
				t.Fatalf("old env not restored: %s", env)
			}
			if tt.restoredDB {
				db, _ := os.ReadFile(dbFile)
				if string(db) != "old-database" {
					t.Fatalf("database not restored: %q", db)
				}
			}
			assertPanelOnlyCompose(t, executor.calls, opts.ComposeProject)
			if strings.Contains(flattenCalls(executor.calls), "image prune") || strings.Contains(flattenCalls(executor.calls), "image rm ") {
				t.Fatalf("failed apply attempted image cleanup: %#v", executor.calls)
			}
		})
	}
}

func flattenCalls(calls [][]string) string {
	lines := make([]string, 0, len(calls))
	for _, call := range calls {
		lines = append(lines, strings.Join(call, " "))
	}
	return strings.Join(lines, "\n")
}

func assertPanelOnlyCompose(t *testing.T, calls [][]string, project string) {
	t.Helper()
	for _, call := range calls {
		if containsArg(call, "steam-auth") || containsArg(call, "server") || argAfter(call, "--project-name") == "stardew" {
			t.Fatalf("game service touched: %#v", call)
		}
		if len(call) > 0 && call[0] == "compose" {
			if argAfter(call, "--project-name") != project {
				t.Fatalf("wrong compose project: %#v", call)
			}
			if containsArg(call, "up") && call[len(call)-1] != "panel" {
				t.Fatalf("non-panel service: %#v", call)
			}
		}
	}
}
