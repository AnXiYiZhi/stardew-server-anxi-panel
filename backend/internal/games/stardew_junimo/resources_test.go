package stardew_junimo

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

func TestResourceStorageUsesSelectedWorldVolume(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "world-a")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("GAME_DATA_VOLUME=anxi-resource-selected\n"), 0600); err != nil {
		t.Fatal(err)
	}
	driver := &Driver{}
	plan, err := driver.ResourceStorage(registry.Instance{ID: "world-a", DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan.Directories, []string{dir}) || !reflect.DeepEqual(plan.Volumes, []string{"anxi-resource-selected", "world-a_steam-session"}) || !reflect.DeepEqual(plan.SharedVolumes, []string{"world-a_steamcmd-login", "world-a_steamcmd-home"}) {
		t.Fatalf("resource ownership diverged from runtime bindings: %+v", plan)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("GAME_DATA_VOLUME=../invalid\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := driver.ResourceStorage(registry.Instance{ID: "world-a", DataDir: dir}); err == nil {
		t.Fatal("invalid runtime binding accepted")
	}
}
