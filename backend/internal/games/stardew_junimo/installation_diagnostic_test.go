package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

func prepareInstallationDiagnosticFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services:\n  server:\n    image: example/server\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := installSMAPIMod(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInstallationDiagnosticClassifiesErrorUsingFileEvidence(t *testing.T) {
	tests := []struct {
		name          string
		verifyCode    int
		inspectErr    error
		inspectStderr string
		wantStatus    string
		wantFiles     string
		wantAction    string
		wantRuns      int
	}{
		{name: "runtime error keeps complete installation", wantStatus: "installed", wantFiles: "ok", wantAction: "retry_start", wantRuns: 1},
		{name: "missing required files needs repair", verifyCode: installVerificationMissingExitCode, wantStatus: "incomplete", wantFiles: "missing", wantAction: "repair_install", wantRuns: 1},
		{name: "transient image inspect failure stays unknown", inspectErr: errors.New("docker daemon unavailable"), wantStatus: "unknown", wantFiles: "unknown", wantAction: "diagnose"},
		{name: "explicit missing image needs repair", inspectErr: errors.New("inspect failed"), inspectStderr: "Error: No such image", wantStatus: "incomplete", wantFiles: "unknown", wantAction: "repair_install"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := prepareInstallationDiagnosticFixture(t)
			docker := &fakeDocker{
				verifyCode:    tt.verifyCode,
				inspectErr:    tt.inspectErr,
				inspectResult: paneldocker.CommandResult{Stderr: tt.inspectStderr},
			}
			diagnostic := New(docker, nil, nil, nil).InstallationDiagnostic(context.Background(), registry.Instance{
				ID: "instance-1", DataDir: dataDir, State: storage.InstanceStateError,
			})
			if diagnostic.Status != tt.wantStatus || diagnostic.RequiredFiles != tt.wantFiles || diagnostic.RecommendedAction != tt.wantAction {
				t.Fatalf("diagnostic = %#v, want status=%s files=%s action=%s", diagnostic, tt.wantStatus, tt.wantFiles, tt.wantAction)
			}
			if docker.verifyRuns != tt.wantRuns {
				t.Fatalf("verifier runs = %d, want %d", docker.verifyRuns, tt.wantRuns)
			}
		})
	}
}

func TestInstallationDiagnosticOnlyExplicitlyUninitializedIsNotInstalled(t *testing.T) {
	diagnostic := New(&fakeDocker{
		inspectErr:    errors.New("inspect failed"),
		inspectResult: paneldocker.CommandResult{Stderr: "Error: No such image"},
	}, nil, nil, nil).InstallationDiagnostic(context.Background(), registry.Instance{
		ID: "instance-1", DataDir: t.TempDir(), State: storage.InstanceStateUninitialized,
	})
	if diagnostic.Status != "not_installed" || diagnostic.RecommendedAction != "install" {
		t.Fatalf("diagnostic = %#v, want not_installed/install", diagnostic)
	}
}

func TestInstallationDiagnosticSeparatesRuntimeVersionFromStaticInstall(t *testing.T) {
	dataDir := prepareInstallationDiagnosticFixture(t)
	control := controlDir(dataDir)
	if err := os.MkdirAll(control, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(control, "options.json"), []byte(`{"controlModVersion":"0.0.1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	docker := &fakeDocker{psResult: paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Service: "server", State: "running"}}}}
	diagnostic := New(docker, nil, nil, nil).InstallationDiagnostic(context.Background(), registry.Instance{
		ID: "instance-1", DataDir: dataDir, State: storage.InstanceStateRunning,
	})
	if diagnostic.Control.Static != "match" || diagnostic.Control.Runtime != "mismatch" || diagnostic.Control.ObservedVersion != "0.0.1" {
		t.Fatalf("control diagnostic = %#v", diagnostic.Control)
	}
	if diagnostic.Status != "installed" || diagnostic.RecommendedAction != "diagnose" {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
}
