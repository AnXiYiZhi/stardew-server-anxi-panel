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
	store, err := storage.Open(ctx, cfg)
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
	stardewDriver := stardew_junimo.New(dockerClient, logger, jobManager, store)
	if err := driverRegistry.Register(stardewDriver); err != nil {
		logger.Error("failed to register stardew driver", "error", err)
		os.Exit(1)
	}
	if defaultInstance.DriverID == stardewDriver.ID() {
		if err := stardewDriver.Prepare(ctx, registry.Instance{
			ID:            defaultInstance.ID,
			DriverID:      defaultInstance.DriverID,
			Name:          defaultInstance.Name,
			DataDir:       defaultInstance.DataDir,
			State:         defaultInstance.State,
			StateMessage:  defaultInstance.StateMessage.String,
			DriverPhase:   defaultInstance.DriverPhase,
			DriverPayload: defaultInstance.DriverPayload,
			CreatedAt:     defaultInstance.CreatedAt,
			UpdatedAt:     defaultInstance.UpdatedAt,
		}); err != nil {
			logger.Error("failed to prepare default instance", "instance", defaultInstance.ID, "error", err)
		}
	}
	if err := jobManager.RecoverInterruptedJobs(ctx); err != nil {
		logger.Error("failed to recover interrupted jobs", "error", err)
		os.Exit(1)
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	restartScheduler := web.NewRestartScheduler(web.RestartSchedulerDeps{
		Store:    store,
		Registry: driverRegistry,
		Logger:   logger,
	})
	go restartScheduler.Run(signalCtx)

	server := &http.Server{
		Addr: cfg.Addr,
		Handler: web.NewHandler(web.Deps{
			Config:   cfg,
			Store:    store,
			Logger:   logger,
			Docker:   dockerClient,
			Jobs:     jobManager,
			Registry: driverRegistry,
		}),
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
