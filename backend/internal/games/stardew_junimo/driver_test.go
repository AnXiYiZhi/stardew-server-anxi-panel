package stardew_junimo

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

type fakeDocker struct {
	workDir string
	result  paneldocker.ComposePsResult
	err     error
}

func (f *fakeDocker) ComposePs(ctx context.Context, dir string) (paneldocker.ComposePsResult, error) {
	f.workDir = dir
	return f.result, f.err
}

func TestDriverIdentityAndPrepare(t *testing.T) {
	driver := New(nil, nil)
	if driver.ID() != DriverID {
		t.Fatalf("unexpected id %q", driver.ID())
	}
	if driver.Name() != DriverName {
		t.Fatalf("unexpected name %q", driver.Name())
	}
	dataDir := filepath.Join(t.TempDir(), "stardew")
	if err := driver.Prepare(context.Background(), registry.Instance{DataDir: dataDir}); err != nil {
		t.Fatalf("prepare: %v", err)
	}
}

func TestDriverStatusUsesInstanceDataDir(t *testing.T) {
	fake := &fakeDocker{result: paneldocker.ComposePsResult{Services: []paneldocker.ComposeService{{Name: "demo", Service: "server", State: "running"}}}}
	driver := New(fake, nil)
	status, err := driver.Status(context.Background(), registry.Instance{
		ID:          "stardew",
		DriverID:    DriverID,
		Name:        "Stardew Valley",
		DataDir:     "custom-dir",
		State:       "running",
		DriverPhase: "empty",
	})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if fake.workDir != "custom-dir" {
		t.Fatalf("expected custom-dir workdir, got %q", fake.workDir)
	}
	if status.Runtime == nil || len(status.Runtime.Containers) != 1 || status.Runtime.Containers[0].Service != "server" {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func TestDriverUnimplementedMethods(t *testing.T) {
	driver := New(nil, nil)
	if _, err := driver.Install(context.Background(), registry.InstallRequest{}); !errors.Is(err, registry.ErrNotImplemented) {
		t.Fatalf("expected install not implemented, got %v", err)
	}
	if err := driver.DeleteMod(context.Background(), "demo"); !errors.Is(err, registry.ErrNotImplemented) {
		t.Fatalf("expected delete mod not implemented, got %v", err)
	}
}
