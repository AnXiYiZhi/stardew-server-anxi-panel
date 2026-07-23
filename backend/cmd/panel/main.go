package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config"
	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updatecheck"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/updater"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/web"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if cfg.Secret == "" {
		logger.Warn("PANEL_SECRET is not set; configure it before enabling auth features")
	}

	ctx := context.Background()
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := storage.OpenWithOptions(ctx, cfg, storage.OpenOptions{
		OnRepeatedInterrupt: func(count int) {
			logger.Error("repeated SQLITE_INTERRUPT; terminating for Docker recovery", "consecutive_interrupts", count)
			os.Exit(1)
		},
	})
	if err != nil {
		logger.Error("failed to open storage", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("failed to close storage", "error", err)
		}
	}()

	if err := store.Migrate(ctx); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	if _, err := store.EnsureDefaultInstanceState(ctx); err != nil {
		logger.Error("failed to ensure default instance state", "error", err)
		os.Exit(1)
	}
	defaultInstance, err := store.EnsureDefaultInstance(ctx, storage.EnsureDefaultInstanceParams{
		ID:       cfg.DefaultInstanceID,
		DriverID: cfg.DefaultDriverID,
		Name:     "Stardew Valley",
		DataDir:  filepath.Join(cfg.DataDir, "instances", cfg.DefaultInstanceID),
	})
	if err != nil {
		logger.Error("failed to ensure default instance", "error", err)
		os.Exit(1)
	}

	dockerClient := paneldocker.NewClient(paneldocker.Options{Logger: logger})
	jobManager := jobs.NewManager(store, logger)
	driverRegistry := registry.New()
	stardewDriver := stardew_junimo.New(dockerClient, logger, jobManager, store, cfg.Version)
	if err := driverRegistry.Register(stardewDriver); err != nil {
		logger.Error("failed to register stardew driver", "error", err)
		os.Exit(1)
	}
	if err := jobManager.RecoverInterruptedJobs(ctx); err != nil {
		logger.Error("failed to recover interrupted jobs", "error", err)
		os.Exit(1)
	}
	instances, err := store.ListInstances(ctx)
	if err != nil {
		logger.Error("failed to list instances for runtime update recovery", "error", err)
		os.Exit(1)
	}
	for _, instance := range instances {
		if instance.DriverID != stardewDriver.ID() {
			continue
		}
		if err := stardewDriver.RecoverRuntimeUpdateApply(ctx, registry.Instance{ID: instance.ID, DriverID: instance.DriverID, Name: instance.Name, DataDir: instance.DataDir, State: instance.State, StateMessage: instance.StateMessage.String, DriverPhase: instance.DriverPhase, DriverPayload: instance.DriverPayload, CreatedAt: instance.CreatedAt, UpdatedAt: instance.UpdatedAt}); err != nil {
			logger.Error("failed to recover Junimo runtime update", "instance", instance.ID, "error", err)
		}
		if err := stardewDriver.RecoverSMAPIUpdateApply(ctx, registry.Instance{ID: instance.ID, DriverID: instance.DriverID, Name: instance.Name, DataDir: instance.DataDir, State: instance.State, StateMessage: instance.StateMessage.String, DriverPhase: instance.DriverPhase, DriverPayload: instance.DriverPayload, CreatedAt: instance.CreatedAt, UpdatedAt: instance.UpdatedAt}); err != nil {
			logger.Error("failed to recover SMAPI update", "instance", instance.ID, "error", err)
		}
	}
	defaultRegistryInstance := registry.Instance{ID: defaultInstance.ID, DriverID: defaultInstance.DriverID, Name: defaultInstance.Name, DataDir: defaultInstance.DataDir, State: defaultInstance.State, StateMessage: defaultInstance.StateMessage.String, DriverPhase: defaultInstance.DriverPhase, DriverPayload: defaultInstance.DriverPayload, CreatedAt: defaultInstance.CreatedAt, UpdatedAt: defaultInstance.UpdatedAt}
	if defaultInstance.DriverID == stardewDriver.ID() && !stardewDriver.RuntimeUpdateApplyInProgress(defaultRegistryInstance) && !stardewDriver.SMAPIUpdateApplyInProgress(defaultRegistryInstance) {
		if err := stardewDriver.Prepare(ctx, defaultRegistryInstance); err != nil {
			logger.Error("failed to prepare default instance", "instance", defaultInstance.ID, "error", err)
		}
	}
	// Every managed instance participates in the required full-stack follow-up.
	// The coordinator is durable per instance and serializes its own runtime jobs.
	for _, instance := range instances {
		if instance.DriverID != stardewDriver.ID() {
			continue
		}
		stardewDriver.StartRequiredRuntimeUpdate(signalCtx, registry.Instance{ID: instance.ID, DriverID: instance.DriverID, Name: instance.Name, DataDir: instance.DataDir, State: instance.State, StateMessage: instance.StateMessage.String, DriverPhase: instance.DriverPhase, DriverPayload: instance.DriverPayload, CreatedAt: instance.CreatedAt, UpdatedAt: instance.UpdatedAt})
	}

	restartScheduler := web.NewRestartScheduler(web.RestartSchedulerDeps{
		Store:    store,
		Registry: driverRegistry,
		Logger:   logger,
	})
	go restartScheduler.Run(signalCtx)
	commandScheduler := web.NewControlCommandScheduler(web.ControlCommandSchedulerDeps{
		Store: store, Logger: logger,
		RetentionDays: cfg.ControlCommandRetentionDays, RetentionCount: cfg.ControlCommandRetentionCount,
	})
	go commandScheduler.Run(signalCtx)
	updateChecker := updatecheck.New(updatecheck.Options{
		CurrentVersion: cfg.Version,
		Commit:         cfg.Commit,
		BuildDate:      cfg.BuildDate,
		Logger:         logger,
	})
	go updateChecker.Run(signalCtx)
	hostname, _ := os.Hostname()
	panelUpdater := updater.NewService(updater.ServiceOptions{
		Docker: updater.NewDockerCLI(), DataDir: cfg.DataDir, ContainerRef: hostname, ContainerDataDir: cfg.DataDir,
		HostInstallDir: cfg.HostInstallDir, HostComposeFile: cfg.HostComposeFile,
		HostDataDir: cfg.HostDataDir, ComposeProject: cfg.ComposeProject, Logger: logger,
		Database: store, DatabasePath: cfg.DBPath,
	})
	go panelUpdater.ReconcileCompletedImageCleanup(signalCtx, cfg.Version)

	handler, err := web.NewHandlerWithError(web.Deps{
		Config:        cfg,
		Store:         store,
		Logger:        logger,
		Docker:        dockerClient,
		Jobs:          jobManager,
		Registry:      driverRegistry,
		UpdateChecker: updateChecker,
		Updater:       panelUpdater,
	})
	if err != nil {
		logger.Error("failed to initialize HTTP handler", "error", err)
		os.Exit(1)
	}
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("stardew anxi panel listening", "addr", cfg.Addr, "data_dir", cfg.DataDir, "db_path", cfg.DBPath, "version", cfg.Version)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-signalCtx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", "error", err)
	}
}
