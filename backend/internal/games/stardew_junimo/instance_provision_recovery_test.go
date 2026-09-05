package stardew_junimo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type provisionRecoveryDocker struct {
	*fakeDocker
	clone      func() error
	cleanupErr error
	cleaned    []string
}

func (f *provisionRecoveryDocker) ProvisionInstanceGameData(context.Context, string, string, string, string, string, string) error {
	if f.clone != nil {
		return f.clone()
	}
	return nil
}
func (f *provisionRecoveryDocker) CleanupInstanceGameData(_ context.Context, _, _, token string) error {
	f.cleaned = append(f.cleaned, token)
	return f.cleanupErr
}

func TestProvisionOwnsTemplateUntilPublication(t *testing.T) {
	s, template, dir := newInstalledAuthOnlyFixture(t)
	ctx := context.Background()
	root := filepath.Dir(filepath.Dir(dir))
	target, err := s.CreateInstance(ctx, storage.CreateInstanceParams{ID: "copy-world", DriverID: DriverID, Name: "copy", DataDir: filepath.Join(root, "instances", "copy-world"), State: storage.InstanceStateAdminCreated, DriverPhase: "instance_provisioning"})
	if err != nil {
		t.Fatal(err)
	}
	f := &provisionRecoveryDocker{fakeDocker: &fakeDocker{}}
	d := NewWithOptions(f, nil, jobs.NewManager(s, nil), s, DriverOptions{ContainerDataDir: root})
	f.clone = func() error {
		if d.runtimeUpdateMu.TryLock() {
			d.runtimeUpdateMu.Unlock()
			t.Error("copy did not hold mutation lock")
		}
		for _, id := range []string{template.ID, target.ID} {
			if _, err := s.CreateJob(ctx, storage.CreateJobParams{Type: "stardew_install", TargetType: "instance", TargetID: id}); err == nil {
				t.Error("late writer accepted", id)
			}
		}
		if _, err := s.CreateJob(ctx, storage.CreateJobParams{Type: "panel_update", TargetType: "panel", TargetID: "panel"}); err == nil {
			t.Error("global mutation accepted")
		}
		return nil
	}
	_, err = d.ProvisionInstance(ctx, registry.InstanceProvisionRequest{Template: makeRegistryInstanceFromStorage(template), Target: makeRegistryInstanceFromStorage(target), Existing: []registry.Instance{makeRegistryInstanceFromStorage(template)}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetInstance(ctx, target.ID)
	if err != nil || got.DriverPhase != "instance_ready" {
		t.Fatal(got, err)
	}
	if _, err := s.GetInstanceProvision(ctx, target.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatal("published journal remains", err)
	}
	if owned, err := d.RecoverInstanceProvision(ctx, makeRegistryInstanceFromStorage(got)); owned || err != nil {
		t.Fatal("published world rolled back", owned, err)
	}
	if _, err := os.Stat(target.DataDir); err != nil {
		t.Fatal(err)
	}
}

func TestProvisionRestartRollbackAndRetry(t *testing.T) {
	for _, window := range []string{"reservation", "journal", "empty-directory", "owner", "partial-copy", "cleanup-failure", "foreign-owner"} {
		t.Run(window, func(t *testing.T) {
			s, template, dir := newInstalledAuthOnlyFixture(t)
			ctx := context.Background()
			root := filepath.Dir(filepath.Dir(dir))
			target, err := s.CreateInstance(ctx, storage.CreateInstanceParams{ID: "copy-world", DriverID: DriverID, Name: "copy", DataDir: filepath.Join(root, "instances", "copy-world"), State: storage.InstanceStateAdminCreated, DriverPhase: "instance_provisioning"})
			if err != nil {
				t.Fatal(err)
			}
			plan := storage.InstanceProvision{InstanceID: target.ID, TemplateID: template.ID, Token: "0123456789abcdef0123456789abcdef"}
			if window != "reservation" {
				if err = s.BeginInstanceProvision(ctx, plan); err != nil {
					t.Fatal(err)
				}
			}
			if window == "empty-directory" {
				if err = os.Mkdir(target.DataDir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if window == "owner" || window == "partial-copy" || window == "cleanup-failure" || window == "foreign-owner" {
				if err = createProvisionDirectory(target.DataDir, plan.Token); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(target.DataDir, "partial-game-file"), []byte("synthetic"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if window == "foreign-owner" {
				if err = os.WriteFile(filepath.Join(target.DataDir, "owner.json"), []byte("foreign"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			f := &provisionRecoveryDocker{fakeDocker: &fakeDocker{}}
			if window == "cleanup-failure" {
				f.cleanupErr = errors.New("holder busy")
			}
			fresh := NewWithOptions(f, nil, jobs.NewManager(s, nil), s, DriverOptions{ContainerDataDir: root})
			handled, err := fresh.RecoverInstanceProvision(ctx, makeRegistryInstanceFromStorage(target))
			if !handled {
				t.Fatal("orphan missed")
			}
			if window == "foreign-owner" {
				if err == nil {
					t.Fatal("foreign owner removed")
				}
				if _, e := os.Stat(filepath.Join(target.DataDir, "partial-game-file")); e != nil {
					t.Fatal(e)
				}
				return
			}
			if window == "cleanup-failure" {
				if err == nil {
					t.Fatal("failure not retained")
				}
				if _, e := s.GetInstanceProvision(ctx, target.ID); e != nil {
					t.Fatal(e)
				}
				if e := fresh.Prepare(ctx, makeRegistryInstanceFromStorage(target)); e == nil {
					t.Fatal("bootstrap recreated pending world")
				}
				f.cleanupErr = nil
				fresh = NewWithOptions(f, nil, jobs.NewManager(s, nil), s, DriverOptions{ContainerDataDir: root})
				err = fresh.DeleteInstance(ctx, makeRegistryInstanceFromStorage(target), template.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.GetInstance(ctx, target.ID); !errors.Is(err, storage.ErrNotFound) {
				t.Fatal("reservation remains", err)
			}
			if _, err = os.Stat(target.DataDir); !os.IsNotExist(err) {
				t.Fatal("partial directory remains", err)
			}
			if _, err = s.GetInstance(ctx, template.ID); err != nil {
				t.Fatal("template lost", err)
			}
		})
	}
}
