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
	targetImage string
}

func (e isolatedDockerExecutor) Run(ctx context.Context, args ...string) error {
	if len(args) == 2 && args[0] == "pull" && args[1] == e.targetImage {
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
	for _, image := range []string{oldImage, newImage} {
		if exec.Command("docker", "image", "inspect", image).Run() == nil {
			t.Skipf("temporary tag collision: %s", image)
		}
	}
	root := t.TempDir()
	buildDir := filepath.Join(root, "image")
	if err := os.MkdirAll(filepath.Join(buildDir, "www", "api"), 0o700); err != nil {
		t.Fatal(err)
	}
	dockerfile := "FROM alpine:3.20\nRUN apk add --no-cache busybox-extras\nCOPY www /www\nHEALTHCHECK --interval=1s --timeout=1s --retries=10 CMD true\nCMD [\"httpd\",\"-f\",\"-p\",\"8090\",\"-h\",\"/www\"]\n"
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
	defer exec.Command("docker", "image", "rm", "-f", oldImage, newImage).Run()

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

	executor := isolatedDockerExecutor{ExecDocker: ExecDocker{}, targetImage: newImage}
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
	gameIDAfter, err := exec.Command("docker", "inspect", "--format", "{{.Id}}", gameContainer).Output()
	if err != nil || strings.TrimSpace(string(gameIDAfter)) != strings.TrimSpace(string(gameIDBefore)) {
		t.Fatalf("game container changed: before=%s after=%s err=%v", gameIDBefore, gameIDAfter, err)
	}
	stateJSON, _ := os.ReadFile(stateFile)
	if strings.Contains(string(stateJSON), "isolated-secret-must-not-log") {
		t.Fatal("secret leaked into updater state")
	}
}
