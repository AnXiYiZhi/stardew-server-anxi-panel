package stardew_junimo

import (
	"context"
	"errors"
	"log/slog"
	"os"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
)

const (
	DriverID   = "stardew_junimo"
	DriverName = "Stardew Valley / JunimoServer"
)

type DockerService interface {
	ComposePs(ctx context.Context, dir string) (paneldocker.ComposePsResult, error)
}

type Driver struct {
	docker DockerService
	logger *slog.Logger
}

func New(docker DockerService, logger *slog.Logger) *Driver {
	if logger == nil {
		logger = slog.Default()
	}
	return &Driver{docker: docker, logger: logger}
}

func (d *Driver) ID() string {
	return DriverID
}

func (d *Driver) Name() string {
	return DriverName
}

func (d *Driver) Prepare(ctx context.Context, instance registry.Instance) error {
	if instance.DataDir == "" {
		return errors.New("instance data dir is empty")
	}
	return os.MkdirAll(instance.DataDir, 0o755)
}

func (d *Driver) Status(ctx context.Context, instance registry.Instance) (*registry.ServerStatus, error) {
	status := &registry.ServerStatus{
		InstanceID:   instance.ID,
		DriverID:     instance.DriverID,
		DriverName:   d.Name(),
		Name:         instance.Name,
		State:        instance.State,
		StateMessage: instance.StateMessage,
		DriverPhase:  instance.DriverPhase,
	}
	if d.docker == nil {
		status.Warnings = append(status.Warnings, registry.StatusWarning{Code: "runtime_unavailable", Message: "Docker runtime status is unavailable"})
		return status, nil
	}
	ps, err := d.docker.ComposePs(ctx, instance.DataDir)
	if err != nil {
		d.logger.Debug("stardew compose ps unavailable", "instance", instance.ID, "error", err)
		status.Warnings = append(status.Warnings, registry.StatusWarning{Code: "runtime_unavailable", Message: "Docker runtime status is unavailable"})
		return status, nil
	}
	containers := make([]registry.ContainerSummary, 0, len(ps.Services))
	for _, service := range ps.Services {
		containers = append(containers, registry.ContainerSummary{
			Name:    service.Name,
			Service: service.Service,
			State:   service.State,
			Status:  service.Status,
			Health:  service.Health,
		})
	}
	status.Runtime = &registry.RuntimeStatus{Containers: containers}
	return status, nil
}

func (d *Driver) Install(ctx context.Context, req registry.InstallRequest) (*registry.Job, error) {
	return nil, registry.ErrNotImplemented
}

func (d *Driver) Start(ctx context.Context, req registry.StartRequest) (*registry.Job, error) {
	return nil, registry.ErrNotImplemented
}

func (d *Driver) Stop(ctx context.Context, instance registry.Instance) error {
	return registry.ErrNotImplemented
}

func (d *Driver) Restart(ctx context.Context, instance registry.Instance) error {
	return registry.ErrNotImplemented
}

func (d *Driver) Logs(ctx context.Context, instance registry.Instance) (<-chan registry.LogLine, error) {
	return nil, registry.ErrNotImplemented
}

func (d *Driver) ExecCommand(ctx context.Context, cmd string) (*registry.CommandResult, error) {
	return nil, registry.ErrNotImplemented
}

func (d *Driver) ListSaves(ctx context.Context, instance registry.Instance) ([]registry.SaveInfo, error) {
	return nil, registry.ErrNotImplemented
}

func (d *Driver) UploadSave(ctx context.Context, file registry.UploadedFile) error {
	return registry.ErrNotImplemented
}

func (d *Driver) SelectSave(ctx context.Context, name string) error {
	return registry.ErrNotImplemented
}

func (d *Driver) DeleteSave(ctx context.Context, name string) error {
	return registry.ErrNotImplemented
}

func (d *Driver) ListMods(ctx context.Context, instance registry.Instance) ([]registry.ModInfo, error) {
	return nil, registry.ErrNotImplemented
}

func (d *Driver) UploadMod(ctx context.Context, file registry.UploadedFile) error {
	return registry.ErrNotImplemented
}

func (d *Driver) DeleteMod(ctx context.Context, id string) error {
	return registry.ErrNotImplemented
}
