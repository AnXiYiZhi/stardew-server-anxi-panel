package stardew_junimo

import (
	"path/filepath"
	"testing"
)

func TestDockerHostPathMapsPanelDataToDaemonSource(t *testing.T) {
	containerRoot := filepath.Join(t.TempDir(), "container-data")
	hostRoot := filepath.Join(t.TempDir(), "host-data")
	driver := NewWithOptions(nil, nil, nil, nil, DriverOptions{
		ContainerDataDir: containerRoot,
		HostDataDir:      hostRoot,
	})
	containerPath := filepath.Join(containerRoot, "instances", "stardew", ".local-container", "junimo-mod-sync", "sync-one")
	got, err := driver.dockerHostPath(containerPath)
	if err != nil {
		t.Fatalf("dockerHostPath: %v", err)
	}
	want := filepath.Join(hostRoot, "instances", "stardew", ".local-container", "junimo-mod-sync", "sync-one")
	if got != want {
		t.Fatalf("dockerHostPath = %q, want %q", got, want)
	}
}

func TestDockerHostPathRejectsPathOutsidePanelData(t *testing.T) {
	containerRoot := filepath.Join(t.TempDir(), "container-data")
	driver := NewWithOptions(nil, nil, nil, nil, DriverOptions{
		ContainerDataDir: containerRoot,
		HostDataDir:      filepath.Join(t.TempDir(), "host-data"),
	})
	if _, err := driver.dockerHostPath(filepath.Join(filepath.Dir(containerRoot), "outside")); err == nil {
		t.Fatal("dockerHostPath should reject a path outside the Panel data directory")
	}
}

func TestDockerHostPathKeepsLegacySamePathBehavior(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance", "work")
	got, err := New(nil, nil, nil, nil).dockerHostPath(path)
	if err != nil {
		t.Fatalf("dockerHostPath: %v", err)
	}
	want, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("dockerHostPath = %q, want %q", got, want)
	}
}

func TestDockerHostPathRejectsIncompleteConfiguredMapping(t *testing.T) {
	driver := NewWithOptions(nil, nil, nil, nil, DriverOptions{HostDataDir: filepath.Join(t.TempDir(), "host-data")})
	if _, err := driver.dockerHostPath(filepath.Join(t.TempDir(), "instance", "work")); err == nil {
		t.Fatal("dockerHostPath should reject a host mapping without an absolute container root")
	}
}
