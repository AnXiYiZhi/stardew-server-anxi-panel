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

type fakeRuntime struct {
	dockerOK, composeOK bool
	info                ContainerInfo
	inspectErr          error
	helperSpecs         []HelperSpec
	applySpecs          []ApplyHelperSpec
	digestErr           error
	applyStartErr       error
}

func (f *fakeRuntime) Available(context.Context) bool        { return f.dockerOK }
func (f *fakeRuntime) ComposeAvailable(context.Context) bool { return f.composeOK }
func (f *fakeRuntime) InspectContainer(context.Context, string) (ContainerInfo, error) {
	return f.info, f.inspectErr
}
func (f *fakeRuntime) StartHelper(_ context.Context, spec HelperSpec) error {
	f.helperSpecs = append(f.helperSpecs, spec)
	return nil
}
func (f *fakeRuntime) ImageDigest(context.Context, string) (string, error) {
	return "repo@example@sha256:1234", f.digestErr
}
func (f *fakeRuntime) StartApplyHelper(_ context.Context, spec ApplyHelperSpec) error {
	f.applySpecs = append(f.applySpecs, spec)
	return f.applyStartErr
}

func standardContainer(composeFile, dataMount string) ContainerInfo {
	return ContainerInfo{
		ID: "1234567890abcdef", Name: "anxi-panel", Image: "anxiyizhi/stardew-server-anxi-panel:0.1.14",
		Labels: map[string]string{
			labelProject: "anxi-panel", labelService: "panel", labelConfigFiles: composeFile,
			labelWorkingDir: filepath.Dir(composeFile),
		},
		Mounts: []MountInfo{
			{Type: "bind", Source: dockerSocketPath(), Destination: "/var/run/docker.sock"},
			{Type: "bind", Source: dataMount, Destination: "/data"},
		},
	}
}

func dockerSocketPath() string {
	if filepath.Separator == '\\' {
		return `C:\var\run\docker.sock`
	}
	return "/var/run/docker.sock"
}

func TestDockerContractDetectsStandardComposeLabels(t *testing.T) {
	installDir := t.TempDir()
	composeFile := filepath.Join(installDir, "docker-compose.yml")
	dataMount := filepath.Join(t.TempDir(), "data")
	runtime := &fakeRuntime{dockerOK: true, composeOK: true, info: standardContainer(composeFile, dataMount)}
	capability := DetectCapability(context.Background(), runtime, DetectOptions{ContainerRef: "1234567890ab", ContainerDataDir: "/data"})
	if !capability.Supported || capability.Code != CodeSupported {
		t.Fatalf("capability = %+v", capability)
	}
	if capability.ComposeProject != "anxi-panel" || capability.ComposeFile != composeFile || capability.DataMount != dataMount {
		t.Fatalf("unexpected compose detection: %+v", capability)
	}
}

func TestDockerContractRejectsMissingComposeLabels(t *testing.T) {
	info := standardContainer(filepath.Join(t.TempDir(), "docker-compose.yml"), filepath.Join(t.TempDir(), "data"))
	info.Labels = map[string]string{}
	runtime := &fakeRuntime{dockerOK: true, composeOK: true, info: info}
	capability := DetectCapability(context.Background(), runtime, DetectOptions{ContainerRef: "1234567890ab", ContainerDataDir: "/data"})
	if capability.Supported || capability.Code != CodeComposeLabelsMissing {
		t.Fatalf("unsafe deployment was accepted: %+v", capability)
	}
}

func TestDockerContractAcceptsCompleteExplicitFallback(t *testing.T) {
	installDir := t.TempDir()
	composeFile := filepath.Join(installDir, "docker-compose.yml")
	dataMount := filepath.Join(t.TempDir(), "data")
	info := standardContainer(composeFile, dataMount)
	info.Labels = map[string]string{}
	runtime := &fakeRuntime{dockerOK: true, composeOK: true, info: info}
	capability := DetectCapability(context.Background(), runtime, DetectOptions{
		ContainerRef: "1234567890ab", ContainerDataDir: "/data", HostInstallDir: installDir,
		HostComposeFile: composeFile, HostDataDir: dataMount, ComposeProject: "anxi-panel",
	})
	if !capability.Supported {
		t.Fatalf("explicit run.sh fallback rejected: %+v", capability)
	}
}

func TestDockerContractRejectsArbitraryAndMutableImages(t *testing.T) {
	for _, ref := range []string{
		"evil.example.com/attacker/panel:0.1.15",
		"anxiyizhi/stardew-server-anxi-panel:latest",
		"anxiyizhi/stardew-server-anxi-panel:0.1.15@sha256:abcd",
	} {
		if err := ValidateTrustedImage(ref); err == nil {
			t.Fatalf("image %q unexpectedly accepted", ref)
		}
	}
	if err := ValidateTrustedImage("ghcr.io/anxiyizhi/stardew-server-anxi-panel:0.1.15"); err != nil {
		t.Fatalf("trusted exact image rejected: %v", err)
	}
}

func TestDockerContractHelperArgsCannotInjectShell(t *testing.T) {
	installDir := t.TempDir()
	spec := HelperSpec{
		Name: "anxi-panel-updater-a1b2", RuntimeImage: "anxiyizhi/stardew-server-anxi-panel:0.1.14",
		TargetVersion: "0.1.15;rm -rf /", ComposeProject: "anxi-panel",
		HostInstallDir: installDir, HostComposeFile: filepath.Join(installDir, "docker-compose.yml"),
		DataMount: t.TempDir(), StateFile: "/data/updater/status.json",
	}
	if _, err := BuildHelperArgs(spec); err == nil {
		t.Fatal("malicious target version accepted")
	}
	spec.TargetVersion = "0.1.15"
	args, err := BuildHelperArgs(spec)
	if err != nil {
		t.Fatal(err)
	}
	joined := " " + strings.Join(args, " ") + " "
	for _, forbidden := range []string{" sh ", " bash ", " -c ", " /bin/sh "} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("helper arguments contain shell execution: %q", joined)
		}
	}
	if !strings.Contains(joined, "dst="+installDir+",readonly") || !strings.Contains(joined, " --compose-file "+spec.HostComposeFile+" ") {
		t.Fatalf("helper must preserve the host deployment path in Compose labels: %q", joined)
	}
}

func TestDockerContractApplyHelperPreservesHostComposePath(t *testing.T) {
	installDir := t.TempDir()
	composeFile := filepath.Join(installDir, "docker-compose.yml")
	args, err := BuildApplyHelperArgs(ApplyHelperSpec{
		Name: "anxi-panel-updater-apply-a1b2", RuntimeImage: "anxiyizhi/stardew-server-anxi-panel:0.1.14",
		FromVersion: "0.1.14", TargetVersion: "0.1.15", OriginalDigest: "sha256:old",
		CurrentContainer: "anxi-panel", ComposeProject: "anxi-panel",
		HostInstallDir: installDir, HostComposeFile: composeFile, DataMount: "panel-data",
		StateFile: "/data/updater/apply-status.json", BackupDir: "/data/updater/backups/a1b2",
		DatabaseRelativePath: "panel.db",
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := " " + strings.Join(args, " ") + " "
	if !strings.Contains(joined, "dst="+installDir+" ") || !strings.Contains(joined, " --compose-file "+composeFile+" ") {
		t.Fatalf("apply helper must preserve the host deployment path in recreated labels: %q", joined)
	}
	if strings.Contains(joined, "/deployment") {
		t.Fatalf("apply helper leaked transient compose path into arguments: %q", joined)
	}
}

type recordingExecutor struct{ calls [][]string }

func (r *recordingExecutor) Run(_ context.Context, args ...string) error {
	r.calls = append(r.calls, append([]string(nil), args...))
	if len(args) >= 2 && args[0] == "image" && args[1] == "inspect" {
		return errors.New("not present")
	}
	return nil
}

func TestDockerContractDryRunUsesOnlyNonDestructiveCommandsAndPersistsStatus(t *testing.T) {
	installDir := t.TempDir()
	composeFile := filepath.Join(installDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("services:\n  panel:\n    image: old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stateFile := filepath.Join(t.TempDir(), "updater", "status.json")
	executor := &recordingExecutor{}
	if err := RunDryRun(context.Background(), DryRunOptions{
		TargetVersion: "v0.1.15", CurrentImage: "anxiyizhi/stardew-server-anxi-panel:0.1.14",
		ComposeProject: "anxi-panel", ComposeFile: composeFile, StateFile: stateFile, Executor: executor,
	}); err != nil {
		t.Fatal(err)
	}
	for _, call := range executor.calls {
		if IsDestructiveDockerArgs(call) {
			t.Fatalf("dry-run issued destructive docker command: %#v", call)
		}
	}
	status, err := NewStateStore(stateFile).Read()
	if err != nil {
		t.Fatal(err)
	}
	if status.Phase != "succeeded" || status.TargetImage == "" || len(status.Logs) == 0 {
		t.Fatalf("persisted status = %+v", status)
	}
	// A new store simulates a restarted panel process reading the same state.
	reloaded, err := NewStateStore(stateFile).Read()
	if err != nil || reloaded.ID != status.ID || reloaded.Phase != status.Phase {
		t.Fatalf("reloaded status = %+v, %v", reloaded, err)
	}
}

func TestDockerContractStateStoreAtomicallyReplacesStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "updater", "status.json")
	store := NewStateStore(path)
	first := DryRunStatus{ID: "first", Phase: "running", StartedAt: time.Now(), UpdatedAt: time.Now(), Logs: []LogEntry{}}
	second := DryRunStatus{ID: "second", Phase: "succeeded", StartedAt: time.Now(), UpdatedAt: time.Now(), Logs: []LogEntry{}}
	if err := store.Write(first); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(second); err != nil {
		t.Fatal(err)
	}
	got, err := NewStateStore(path).Read()
	if err != nil || got.ID != "second" {
		t.Fatalf("status = %+v, %v", got, err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".status-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary status files remain: %v, %v", matches, err)
	}
}
