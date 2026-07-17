package updater

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type isolatedDockerExecutor struct {
	ExecDocker
	targetImage     string
	allowedImageRMs map[string]bool
}

func (e isolatedDockerExecutor) Run(ctx context.Context, args ...string) error {
	if len(args) == 2 && args[0] == "pull" && args[1] == e.targetImage {
		return nil
	}
	// Keep the integration project isolated: production performs a
	// label-scoped dangling prune, but a test must not prune host images that
	// belong to another Panel deployment.
	if len(args) >= 2 && args[0] == "image" && args[1] == "prune" {
		return nil
	}
	if len(args) == 3 && args[0] == "image" && args[1] == "rm" && !e.allowedImageRMs[args[2]] {
		return nil
	}
	copyArgs := append([]string(nil), args...)
	if len(copyArgs) > 0 && copyArgs[0] == "compose" && containsArg(copyArgs, "up") {
		for i := range copyArgs {
			if copyArgs[i] == "always" {
				copyArgs[i] = "never"
			}
		}
	}
	return e.ExecDocker.Run(ctx, copyArgs...)
}
func (e isolatedDockerExecutor) Output(ctx context.Context, args ...string) (string, error) {
	return e.ExecDocker.Output(ctx, args...)
}

func TestDockerIntegrationApplyUsesIsolatedComposeProject(t *testing.T) {
	if os.Getenv("PANEL_RUN_DOCKER_UPDATE_TEST") != "1" {
		t.Skip("set PANEL_RUN_DOCKER_UPDATE_TEST=1")
	}
	if err := exec.Command("docker", "version").Run(); err != nil {
		t.Skip("docker unavailable")
	}
	suffix := strconv.FormatInt(time.Now().UnixNano()%1_000_000, 10)
	major := 9000 + time.Now().Nanosecond()%900
	fromVersion := fmt.Sprintf("%d.0.0", major)
	toVersion := fmt.Sprintf("%d.0.1", major)
	oldImage := "ghcr.io/anxiyizhi/stardew-server-anxi-panel:" + fromVersion
	newImage := "ghcr.io/anxiyizhi/stardew-server-anxi-panel:" + toVersion
	historyImage := fmt.Sprintf("ghcr.io/anxiyizhi/stardew-server-anxi-panel:%d.0.2", major)
	protectedImage := fmt.Sprintf("ghcr.io/anxiyizhi/stardew-server-anxi-panel:%d.0.3", major)
	customImage := fmt.Sprintf("panelapply%s/custom:%d.0.4", suffix, major)
	for _, image := range []string{oldImage, newImage, historyImage, protectedImage, customImage} {
		if exec.Command("docker", "image", "inspect", image).Run() == nil {
			t.Skipf("temporary tag collision: %s", image)
		}
	}
	root := t.TempDir()
	buildDir := filepath.Join(root, "image")
	if err := os.MkdirAll(filepath.Join(buildDir, "www", "api"), 0o700); err != nil {
		t.Fatal(err)
	}
	dockerfile := "FROM alpine:3.20\nRUN apk add --no-cache busybox-extras\nCOPY www /www\nLABEL org.opencontainers.image.title=stardew-server-anxi-panel\nHEALTHCHECK --interval=1s --timeout=1s --retries=10 CMD true\nCMD [\"httpd\",\"-f\",\"-p\",\"8090\",\"-h\",\"/www\"]\n"
	if err := os.WriteFile(filepath.Join(buildDir, "Dockerfile"), []byte(dockerfile), 0o600); err != nil {
		t.Fatal(err)
	}
	buildImage := func(tag, version string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(buildDir, "www", "health"), []byte(`{"status":"ok"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(buildDir, "www", "api", "version"), []byte(`{"version":"`+version+`"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("docker", "build", "-q", "-t", tag, buildDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v: %s", tag, err, output)
		}
	}
	buildImage(oldImage, fromVersion)
	buildImage(newImage, toVersion)
	for _, tag := range []string{historyImage, protectedImage, customImage} {
		if output, err := exec.Command("docker", "tag", oldImage, tag).CombinedOutput(); err != nil {
			t.Fatalf("tag isolated image %s: %v: %s", tag, err, output)
		}
	}
	protectedContainer := projectSafeName("panelapply-protected-" + suffix)
	if output, err := exec.Command("docker", "run", "-d", "--name", protectedContainer, protectedImage, "sleep", "3600").CombinedOutput(); err != nil {
		t.Fatalf("start protected image container: %v: %s", err, output)
	}
	defer exec.Command("docker", "rm", "-f", protectedContainer).Run()
	defer exec.Command("docker", "image", "rm", "-f", oldImage, newImage, historyImage, protectedImage, customImage).Run()

	installDir := filepath.Join(root, "deployment")
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(installDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	project := "panelapply" + suffix
	panelContainer := project + "-panel"
	gameContainer := project + "-game"
	composeFile := filepath.Join(installDir, "docker-compose.yml")
	envFile := filepath.Join(installDir, ".env")
	compose := fmt.Sprintf("services:\n  panel:\n    image: ${PANEL_IMAGE}\n    container_name: %s\n  game:\n    image: alpine:3.20\n    container_name: %s\n    command: [\"sleep\",\"3600\"]\n", panelContainer, gameContainer)
	if err := os.WriteFile(composeFile, []byte(compose), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envFile, []byte("PANEL_IMAGE="+oldImage+"\nPANEL_SECRET=isolated-secret-must-not-log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := []string{"compose", "--project-name", project, "--env-file", envFile, "-f", composeFile}
	defer exec.Command("docker", append(base, "down", "--remove-orphans")...).Run()
	if output, err := exec.Command("docker", append(base, "up", "-d")...).CombinedOutput(); err != nil {
		t.Fatalf("initial compose up: %v: %s", err, output)
	}
	gameIDBefore, err := exec.Command("docker", "inspect", "--format", "{{.Id}}", gameContainer).Output()
	if err != nil {
		t.Fatal(err)
	}

	executor := isolatedDockerExecutor{ExecDocker: ExecDocker{}, targetImage: newImage, allowedImageRMs: map[string]bool{oldImage: true, historyImage: true}}
	digest, err := executor.Output(context.Background(), "image", "inspect", "--format", "{{.Id}}", oldImage)
	if err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(dataDir, "updater", "backups", "docker-integration")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "panel.db"), []byte("isolated-db-backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "panel.db"), []byte("isolated-db-current"), 0o600); err != nil {
		t.Fatal(err)
	}
	stateFile := filepath.Join(dataDir, "updater", "apply-status.json")
	stateStore := NewApplyStateStore(stateFile)
	now := time.Now().UTC()
	if err := stateStore.Write(ApplyStatus{UpdateID: "docker-integration", Phase: PhaseBackingUp, FromVersion: fromVersion, ToVersion: toVersion, OriginalImage: oldImage, OriginalDigest: digest, StartedAt: now, UpdatedAt: now, Logs: []LogEntry{}}); err != nil {
		t.Fatal(err)
	}
	if err := RunApply(context.Background(), ApplyOptions{
		FromVersion: fromVersion, TargetVersion: toVersion, CurrentImage: oldImage, OriginalDigest: digest,
		CurrentContainer: panelContainer, ComposeProject: project, ComposeFile: composeFile,
		StateFile: stateFile, BackupDir: backupDir, DatabaseRelative: "panel.db", DataDir: dataDir,
		HealthTimeout: 30 * time.Second, PollInterval: 500 * time.Millisecond, Executor: executor,
	}); err != nil {
		t.Fatal(err)
	}
	status, err := stateStore.Read()
	if err != nil || status.Phase != PhaseSucceeded {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	panelImage, err := exec.Command("docker", "inspect", "--format", "{{.Config.Image}}", panelContainer).Output()
	if err != nil || strings.TrimSpace(string(panelImage)) != newImage {
		t.Fatalf("panel image=%q err=%v", panelImage, err)
	}
	if err := exec.Command("docker", "image", "inspect", oldImage).Run(); err == nil {
		t.Fatalf("old panel image was not removed after successful apply: %s", oldImage)
	}
	if err := exec.Command("docker", "image", "inspect", historyImage).Run(); err == nil {
		t.Fatalf("trusted historical panel image was not removed: %s", historyImage)
	}
	for _, kept := range []string{protectedImage, customImage} {
		if err := exec.Command("docker", "image", "inspect", kept).Run(); err != nil {
			t.Fatalf("protected/custom image was removed: %s: %v", kept, err)
		}
	}
	gameIDAfter, err := exec.Command("docker", "inspect", "--format", "{{.Id}}", gameContainer).Output()
	if err != nil || strings.TrimSpace(string(gameIDAfter)) != strings.TrimSpace(string(gameIDBefore)) {
		t.Fatalf("game container changed: before=%s after=%s err=%v", gameIDBefore, gameIDAfter, err)
	}
	stateJSON, _ := os.ReadFile(stateFile)
	if strings.Contains(string(stateJSON), "isolated-secret-must-not-log") {
		t.Fatal("secret leaked into updater state")
	}
}

func projectSafeName(value string) string {
	return strings.ToLower(strings.ReplaceAll(value, ".", "-"))
}

func TestDockerIntegrationFailedApplyRollsBackWithoutImageCleanup(t *testing.T) {
	if os.Getenv("PANEL_RUN_DOCKER_UPDATE_TEST") != "1" {
		t.Skip("set PANEL_RUN_DOCKER_UPDATE_TEST=1")
	}
	if err := exec.Command("docker", "version").Run(); err != nil {
		t.Skip("docker unavailable")
	}
	suffix := strconv.FormatInt(time.Now().UnixNano()%1_000_000, 10)
	major := 8000 + time.Now().Nanosecond()%900
	fromVersion := fmt.Sprintf("%d.0.0", major)
	toVersion := fmt.Sprintf("%d.0.1", major)
	repository := "ghcr.io/anxiyizhi/stardew-server-anxi-panel:"
	oldImage := repository + fromVersion
	newImage := repository + toVersion
	historyImage := fmt.Sprintf("%s%d.0.2", repository, major)
	for _, image := range []string{oldImage, newImage, historyImage} {
		if exec.Command("docker", "image", "inspect", image).Run() == nil {
			t.Skipf("temporary tag collision: %s", image)
		}
	}

	root := t.TempDir()
	buildDir := filepath.Join(root, "image")
	if err := os.MkdirAll(filepath.Join(buildDir, "www", "api"), 0o700); err != nil {
		t.Fatal(err)
	}
	dockerfilePath := filepath.Join(buildDir, "Dockerfile")
	build := func(tag, version string, healthy bool) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(buildDir, "www", "health"), []byte(`{"status":"ok"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(buildDir, "www", "api", "version"), []byte(`{"version":"`+version+`"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		healthCommand := "true"
		if !healthy {
			healthCommand = "false"
		}
		dockerfile := "FROM alpine:3.20\nRUN apk add --no-cache busybox-extras\nCOPY www /www\nLABEL org.opencontainers.image.title=stardew-server-anxi-panel\nHEALTHCHECK --interval=1s --timeout=1s --retries=2 CMD " + healthCommand + "\nCMD [\"httpd\",\"-f\",\"-p\",\"8090\",\"-h\",\"/www\"]\n"
		if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0o600); err != nil {
			t.Fatal(err)
		}
		if output, err := exec.Command("docker", "build", "-q", "-t", tag, buildDir).CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v: %s", tag, err, output)
		}
	}
	build(oldImage, fromVersion, true)
	build(newImage, toVersion, false)
	if output, err := exec.Command("docker", "tag", oldImage, historyImage).CombinedOutput(); err != nil {
		t.Fatalf("tag history image: %v: %s", err, output)
	}
	defer exec.Command("docker", "image", "rm", "-f", oldImage, newImage, historyImage).Run()

	installDir := filepath.Join(root, "deployment")
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(installDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	project := "panelrollback" + suffix
	panelContainer := project + "-panel"
	composeFile := filepath.Join(installDir, "docker-compose.yml")
	envFile := filepath.Join(installDir, ".env")
	compose := fmt.Sprintf("services:\n  panel:\n    image: ${PANEL_IMAGE}\n    container_name: %s\n", panelContainer)
	if err := os.WriteFile(composeFile, []byte(compose), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envFile, []byte("PANEL_IMAGE="+oldImage+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := []string{"compose", "--project-name", project, "--env-file", envFile, "-f", composeFile}
	defer exec.Command("docker", append(base, "down", "--remove-orphans")...).Run()
	if output, err := exec.Command("docker", append(base, "up", "-d")...).CombinedOutput(); err != nil {
		t.Fatalf("initial compose up: %v: %s", err, output)
	}

	executor := isolatedDockerExecutor{ExecDocker: ExecDocker{}, targetImage: newImage, allowedImageRMs: map[string]bool{oldImage: true, historyImage: true}}
	digest, err := executor.Output(context.Background(), "image", "inspect", "--format", "{{.Id}}", oldImage)
	if err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(dataDir, "updater", "backups", "docker-rollback")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "panel.db"), []byte("rollback-database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "panel.db"), []byte("migrated-database"), 0o600); err != nil {
		t.Fatal(err)
	}
	stateFile := filepath.Join(dataDir, "updater", "apply-status.json")
	store := NewApplyStateStore(stateFile)
	now := time.Now().UTC()
	if err := store.Write(ApplyStatus{UpdateID: "docker-rollback", Phase: PhaseBackingUp, FromVersion: fromVersion, ToVersion: toVersion, OriginalImage: oldImage, OriginalDigest: digest, StartedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	err = RunApply(context.Background(), ApplyOptions{
		FromVersion: fromVersion, TargetVersion: toVersion, CurrentImage: oldImage, OriginalDigest: digest,
		CurrentContainer: panelContainer, ComposeProject: project, ComposeFile: composeFile, StateFile: stateFile,
		BackupDir: backupDir, DatabaseRelative: "panel.db", DataDir: dataDir,
		HealthTimeout: 5 * time.Second, PollInterval: 250 * time.Millisecond, Executor: executor,
	})
	if err == nil {
		t.Fatal("unhealthy target unexpectedly succeeded")
	}
	status, readErr := store.Read()
	if readErr != nil || status.Phase != PhaseFailedRolledBack {
		t.Fatalf("status=%+v err=%v", status, readErr)
	}
	for _, kept := range []string{oldImage, historyImage} {
		if err := exec.Command("docker", "image", "inspect", kept).Run(); err != nil {
			t.Fatalf("failed apply removed rollback/history image %s: %v", kept, err)
		}
	}
	panelImage, inspectErr := exec.Command("docker", "inspect", "--format", "{{.Config.Image}}", panelContainer).Output()
	if inspectErr != nil || strings.TrimSpace(string(panelImage)) != oldImage {
		t.Fatalf("old panel was not restored: image=%q err=%v", panelImage, inspectErr)
	}
	database, err := os.ReadFile(filepath.Join(dataDir, "panel.db"))
	if err != nil || string(database) != "rollback-database" {
		t.Fatalf("database rollback failed: %q err=%v", database, err)
	}
}

func TestDockerIntegrationNewPanelReconcilesPreviousHelperCleanup(t *testing.T) {
	releaseImage := strings.TrimSpace(os.Getenv("ANXI_PANEL_RELEASE_IMAGE"))
	releaseVersion := strings.TrimSpace(os.Getenv("ANXI_PANEL_RELEASE_VERSION"))
	if releaseImage == "" || releaseVersion == "" {
		t.Skip("set ANXI_PANEL_RELEASE_IMAGE and ANXI_PANEL_RELEASE_VERSION")
	}
	if err := ValidateTrustedImage(releaseImage); err != nil {
		t.Fatalf("release image is not a trusted exact tag: %v", err)
	}
	if normalized, err := NormalizeTargetVersion(releaseVersion); err != nil || normalized != releaseVersion {
		t.Fatalf("invalid release version %q", releaseVersion)
	}
	if err := exec.Command("docker", "image", "inspect", releaseImage).Run(); err != nil {
		t.Skipf("release image unavailable: %s", releaseImage)
	}
	suffix := strconv.FormatInt(time.Now().UnixNano()%1_000_000, 10)
	major := 7000 + time.Now().Nanosecond()%900
	oldImage := fmt.Sprintf("ghcr.io/anxiyizhi/stardew-server-anxi-panel:%d.0.0", major)
	if output, err := exec.Command("docker", "tag", "alpine:3.20", oldImage).CombinedOutput(); err != nil {
		t.Fatalf("tag previous-helper image: %v: %s", err, output)
	}
	container := "anxi-panel-reconcile-" + suffix
	protectors := []string{}
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", container).Run()
		for _, protector := range protectors {
			_ = exec.Command("docker", "rm", "-f", protector).Run()
		}
		_ = exec.Command("docker", "image", "rm", "-f", oldImage).Run()
	})

	// Production cleanup includes a label-scoped dangling prune. Protect every
	// pre-existing matching image with a temporary stopped container so this
	// opt-in test cannot mutate the developer's Docker Desktop inventory.
	output, err := exec.Command("docker", "image", "ls", "--no-trunc", "--filter", "label=org.opencontainers.image.title=stardew-server-anxi-panel", "--format", "{{.ID}}").Output()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for index, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		imageID := strings.TrimSpace(line)
		if imageID == "" || seen[imageID] {
			continue
		}
		seen[imageID] = true
		name := fmt.Sprintf("anxi-panel-reconcile-protect-%s-%d", suffix, index)
		if created, createErr := exec.Command("docker", "create", "--name", name, imageID).CombinedOutput(); createErr != nil {
			t.Fatalf("protect pre-existing image %s: %v: %s", imageID, createErr, created)
		}
		protectors = append(protectors, name)
	}

	dataDir := t.TempDir()
	store := NewApplyStateStore(filepath.Join(dataDir, "updater", "apply-status.json"))
	originalID, err := ExecDocker{}.Output(context.Background(), "image", "inspect", "--format", "{{.Id}}", oldImage)
	if err != nil {
		t.Fatal(err)
	}
	selectedID, err := ExecDocker{}.Output(context.Background(), "image", "inspect", "--format", "{{.Id}}", releaseImage)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := store.Write(ApplyStatus{
		UpdateID: "previous-helper-" + suffix, Phase: PhaseSucceeded, Progress: 100,
		FromVersion: fmt.Sprintf("%d.0.0", major), ToVersion: releaseVersion,
		OriginalImage: oldImage, OriginalDigest: originalID, SelectedImage: releaseImage, SelectedDigest: selectedID,
		StartedAt: now, UpdatedAt: now, FinishedAt: &now, Logs: []LogEntry{},
	}); err != nil {
		t.Fatal(err)
	}
	mount := "type=bind,src=" + filepath.Clean(dataDir) + ",dst=/data"
	if started, startErr := exec.Command("docker", "run", "-d", "--name", container,
		"--mount", "type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock",
		"--mount", mount, releaseImage).CombinedOutput(); startErr != nil {
		t.Fatalf("start release panel reconciliation fixture: %v: %s", startErr, started)
	}
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		status, readErr := store.Read()
		if readErr == nil && status.CleanupCompleted {
			break
		}
		if state, _ := exec.Command("docker", "inspect", "--format", "{{.State.Status}}", container).Output(); strings.TrimSpace(string(state)) == "exited" {
			logs, _ := exec.Command("docker", "logs", container).CombinedOutput()
			t.Fatalf("release panel exited before reconciliation: %s", logs)
		}
		time.Sleep(time.Second)
	}
	status, err := store.Read()
	if err != nil || !status.CleanupCompleted {
		logs, _ := exec.Command("docker", "logs", container).CombinedOutput()
		t.Fatalf("cleanup was not reconciled by new Panel: status=%+v err=%v logs=%s", status, err, logs)
	}
	if err := exec.Command("docker", "image", "inspect", oldImage).Run(); err == nil {
		t.Fatalf("new Panel did not remove previous-helper image: %s", oldImage)
	}
	for imageID := range seen {
		if err := exec.Command("docker", "image", "inspect", imageID).Run(); err != nil {
			t.Fatalf("pre-existing Docker Desktop image was removed: %s: %v", imageID, err)
		}
	}
}
