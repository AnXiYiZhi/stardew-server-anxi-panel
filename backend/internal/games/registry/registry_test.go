package registry

import (
	"context"
	"errors"
	"testing"
)

type fakeDriver struct {
	id   string
	name string
}

func (f fakeDriver) ID() string                                           { return f.id }
func (f fakeDriver) Name() string                                         { return f.name }
func (f fakeDriver) Prepare(ctx context.Context, instance Instance) error { return nil }
func (f fakeDriver) Install(ctx context.Context, req InstallRequest) (*Job, error) {
	return nil, ErrNotImplemented
}
func (f fakeDriver) Start(ctx context.Context, req StartRequest) (*Job, error) {
	return nil, ErrNotImplemented
}
func (f fakeDriver) Stop(ctx context.Context, instance Instance) error    { return ErrNotImplemented }
func (f fakeDriver) Restart(ctx context.Context, instance Instance) error { return ErrNotImplemented }
func (f fakeDriver) Status(ctx context.Context, instance Instance) (*ServerStatus, error) {
	return &ServerStatus{InstanceID: instance.ID}, nil
}
func (f fakeDriver) Logs(ctx context.Context, instance Instance) (<-chan LogLine, error) {
	return nil, ErrNotImplemented
}
func (f fakeDriver) ExecCommand(ctx context.Context, cmd string) (*CommandResult, error) {
	return nil, ErrNotImplemented
}
func (f fakeDriver) ListSaves(ctx context.Context, instance Instance) ([]SaveInfo, error) {
	return nil, ErrNotImplemented
}
func (f fakeDriver) UploadSave(ctx context.Context, file UploadedFile) error {
	return ErrNotImplemented
}
func (f fakeDriver) SelectSave(ctx context.Context, name string) error { return ErrNotImplemented }
func (f fakeDriver) DeleteSave(ctx context.Context, name string) error { return ErrNotImplemented }
func (f fakeDriver) ListMods(ctx context.Context, instance Instance) ([]ModInfo, error) {
	return nil, ErrNotImplemented
}
func (f fakeDriver) UploadMod(ctx context.Context, file UploadedFile) error { return ErrNotImplemented }
func (f fakeDriver) DeleteMod(ctx context.Context, id string) error         { return ErrNotImplemented }

func TestRegistryRegisterGetAndList(t *testing.T) {
	registry := New()
	driver := fakeDriver{id: "demo", name: "Demo"}
	if err := registry.Register(driver); err != nil {
		t.Fatalf("register driver: %v", err)
	}
	loaded, err := registry.Get("demo")
	if err != nil {
		t.Fatalf("get driver: %v", err)
	}
	if loaded.Name() != "Demo" {
		t.Fatalf("unexpected driver name %q", loaded.Name())
	}
	list := registry.List()
	if len(list) != 1 || list[0].ID != "demo" || list[0].Name != "Demo" {
		t.Fatalf("unexpected driver list: %#v", list)
	}
}

func TestRegistryRejectsDuplicateAndUnknownDrivers(t *testing.T) {
	registry := New()
	if err := registry.Register(fakeDriver{id: "demo", name: "Demo"}); err != nil {
		t.Fatalf("register driver: %v", err)
	}
	if err := registry.Register(fakeDriver{id: "demo", name: "Other"}); !errors.Is(err, ErrDriverAlreadyRegistered) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	if _, err := registry.Get("missing"); !errors.Is(err, ErrDriverNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if err := registry.Register(fakeDriver{}); !errors.Is(err, ErrInvalidDriver) {
		t.Fatalf("expected invalid driver, got %v", err)
	}
}
