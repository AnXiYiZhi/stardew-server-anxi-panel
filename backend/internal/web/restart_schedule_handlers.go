package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry"
	sj "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

const (
	defaultRestartScheduleTimezone = "Asia/Shanghai"
	restartSchedulePollInterval    = 30 * time.Second
	restartScheduleStartupGrace    = 5 * time.Minute
)

type restartScheduleRequest struct {
	Enabled              bool   `json:"enabled"`
	ShutdownTime         string `json:"shutdownTime"`
	StartupTime          string `json:"startupTime"`
	Timezone             string `json:"timezone"`
	WarningMinutes       []int  `json:"warningMinutes"`
	BackupBeforeShutdown bool   `json:"backupBeforeShutdown"`
	SkipIfPlayersOnline  bool   `json:"skipIfPlayersOnline"`
}

type restartScheduleResponse struct {
	Schedule restartScheduleDTO `json:"schedule"`
}

type restartScheduleDTO struct {
	InstanceID           string  `json:"instanceId"`
	Enabled              bool    `json:"enabled"`
	ShutdownTime         string  `json:"shutdownTime"`
	StartupTime          string  `json:"startupTime"`
	Timezone             string  `json:"timezone"`
	WarningMinutes       []int   `json:"warningMinutes"`
	BackupBeforeShutdown bool    `json:"backupBeforeShutdown"`
	SkipIfPlayersOnline  bool    `json:"skipIfPlayersOnline"`
	NextShutdownAt       *string `json:"nextShutdownAt"`
	NextStartupAt        *string `json:"nextStartupAt"`
	LastShutdownAt       *string `json:"lastShutdownAt"`
	LastStartupAt        *string `json:"lastStartupAt"`
	LastStatus           *string `json:"lastStatus"`
	LastMessage          *string `json:"lastMessage"`
	CreatedAt            string  `json:"createdAt,omitempty"`
	UpdatedAt            string  `json:"updatedAt,omitempty"`
}

func (s *server) handleRestartSchedule(w http.ResponseWriter, r *http.Request, instanceID string) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := s.requireAuth(w, r); !ok {
			return
		}
		_, ok := s.loadInstance(w, r, instanceID)
		if !ok {
			return
		}
		schedule, err := s.restartScheduleOrDefault(r.Context(), instanceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "restart_schedule_failed", sanitizeErrorMsg(err, "读取计划重启失败"))
			return
		}
		writeJSON(w, http.StatusOK, restartScheduleResponse{Schedule: makeRestartScheduleDTO(schedule, time.Now())})
	case http.MethodPut:
		actor, ok := s.requireAdmin(w, r)
		if !ok {
			return
		}
		_, ok = s.loadInstance(w, r, instanceID)
		if !ok {
			return
		}
		var req restartScheduleRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		params, err := normalizeRestartScheduleRequest(instanceID, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_restart_schedule", err.Error())
			return
		}
		schedule, err := s.store.UpsertRestartSchedule(r.Context(), params)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "restart_schedule_failed", sanitizeErrorMsg(err, "保存计划重启失败"))
			return
		}
		s.auditLog(r, &actor, "restart_schedule_update", "instance", instanceID, auditMetadata("enabled", fmt.Sprintf("%t", schedule.Enabled)))
		writeJSON(w, http.StatusOK, restartScheduleResponse{Schedule: makeRestartScheduleDTO(schedule, time.Now())})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (s *server) restartScheduleOrDefault(ctx context.Context, instanceID string) (storage.RestartSchedule, error) {
	schedule, err := s.store.GetRestartSchedule(ctx, instanceID)
	if err == nil {
		return schedule, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return storage.RestartSchedule{}, err
	}
	return defaultRestartSchedule(instanceID), nil
}

func defaultRestartSchedule(instanceID string) storage.RestartSchedule {
	return storage.RestartSchedule{
		InstanceID:           instanceID,
		Enabled:              false,
		ShutdownTime:         "04:00",
		StartupTime:          "04:20",
		Timezone:             defaultRestartScheduleTimezone,
		WarningMinutes:       []int{10, 5, 1},
		BackupBeforeShutdown: true,
		SkipIfPlayersOnline:  false,
	}
}

func normalizeRestartScheduleRequest(instanceID string, req restartScheduleRequest) (storage.UpsertRestartScheduleParams, error) {
	shutdownTime, err := normalizeClockTime(req.ShutdownTime)
	if err != nil {
		return storage.UpsertRestartScheduleParams{}, fmt.Errorf("关闭时间格式必须是 HH:MM")
	}
	startupTime, err := normalizeClockTime(req.StartupTime)
	if err != nil {
		return storage.UpsertRestartScheduleParams{}, fmt.Errorf("开启时间格式必须是 HH:MM")
	}
	timezone := strings.TrimSpace(req.Timezone)
	if timezone == "" {
		timezone = defaultRestartScheduleTimezone
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return storage.UpsertRestartScheduleParams{}, fmt.Errorf("时区无效")
	}
	warnings := normalizeWarningMinutes(req.WarningMinutes)
	if len(warnings) == 0 {
		warnings = []int{10, 5, 1}
	}
	return storage.UpsertRestartScheduleParams{
		InstanceID:           instanceID,
		Enabled:              req.Enabled,
		ShutdownTime:         shutdownTime,
		StartupTime:          startupTime,
		Timezone:             timezone,
		WarningMinutes:       warnings,
		BackupBeforeShutdown: req.BackupBeforeShutdown,
		SkipIfPlayersOnline:  req.SkipIfPlayersOnline,
	}, nil
}

func normalizeClockTime(value string) (string, error) {
	value = strings.TrimSpace(value)
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return "", err
	}
	return parsed.Format("15:04"), nil
}

func normalizeWarningMinutes(values []int) []int {
	seen := map[int]bool{}
	clean := []int{}
	for _, value := range values {
		if value <= 0 || value > 60 || seen[value] {
			continue
		}
		seen[value] = true
		clean = append(clean, value)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(clean)))
	return clean
}

func makeRestartScheduleDTO(schedule storage.RestartSchedule, now time.Time) restartScheduleDTO {
	nextShutdown, nextStartup := nextRestartScheduleWindow(schedule, now)
	dto := restartScheduleDTO{
		InstanceID:           schedule.InstanceID,
		Enabled:              schedule.Enabled,
		ShutdownTime:         schedule.ShutdownTime,
		StartupTime:          schedule.StartupTime,
		Timezone:             schedule.Timezone,
		WarningMinutes:       append([]int(nil), schedule.WarningMinutes...),
		BackupBeforeShutdown: schedule.BackupBeforeShutdown,
		SkipIfPlayersOnline:  schedule.SkipIfPlayersOnline,
		NextShutdownAt:       timeStringPtr(nextShutdown),
		NextStartupAt:        timeStringPtr(nextStartup),
		LastShutdownAt:       nullableString(schedule.LastShutdownAt),
		LastStartupAt:        nullableString(schedule.LastStartupAt),
		LastStatus:           nullableString(schedule.LastStatus),
		LastMessage:          nullableString(schedule.LastMessage),
		CreatedAt:            schedule.CreatedAt,
		UpdatedAt:            schedule.UpdatedAt,
	}
	if !schedule.Enabled {
		dto.NextShutdownAt = nil
		dto.NextStartupAt = nil
	}
	return dto
}

func timeStringPtr(value time.Time) *string {
	if value.IsZero() {
		return nil
	}
	text := value.Format(time.RFC3339)
	return &text
}

func nextRestartScheduleWindow(schedule storage.RestartSchedule, now time.Time) (time.Time, time.Time) {
	loc := scheduleLocation(schedule)
	localNow := now.In(loc)
	shutdownHour, shutdownMinute := parseClockTime(schedule.ShutdownTime)
	startupHour, startupMinute := parseClockTime(schedule.StartupTime)
	year, month, day := localNow.Date()
	shutdown := time.Date(year, month, day, shutdownHour, shutdownMinute, 0, 0, loc)
	startup := time.Date(year, month, day, startupHour, startupMinute, 0, 0, loc)
	if !startup.After(shutdown) {
		startup = startup.Add(24 * time.Hour)
	}
	if !localNow.Before(shutdown) {
		shutdown = shutdown.Add(24 * time.Hour)
		startup = startup.Add(24 * time.Hour)
	}
	return shutdown, startup
}

func currentRestartScheduleWindow(schedule storage.RestartSchedule, now time.Time) (time.Time, time.Time) {
	loc := scheduleLocation(schedule)
	localNow := now.In(loc)
	shutdownHour, shutdownMinute := parseClockTime(schedule.ShutdownTime)
	startupHour, startupMinute := parseClockTime(schedule.StartupTime)
	year, month, day := localNow.Date()
	shutdown := time.Date(year, month, day, shutdownHour, shutdownMinute, 0, 0, loc)
	if localNow.Before(shutdown) {
		shutdown = shutdown.Add(-24 * time.Hour)
	}
	startup := time.Date(shutdown.Year(), shutdown.Month(), shutdown.Day(), startupHour, startupMinute, 0, 0, loc)
	if !startup.After(shutdown) {
		startup = startup.Add(24 * time.Hour)
	}
	return shutdown, startup
}

func parseClockTime(value string) (int, int) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, 0
	}
	return parsed.Hour(), parsed.Minute()
}

func scheduleLocation(schedule storage.RestartSchedule) *time.Location {
	if loc, err := time.LoadLocation(schedule.Timezone); err == nil {
		return loc
	}
	loc, _ := time.LoadLocation(defaultRestartScheduleTimezone)
	return loc
}

type RestartSchedulerDeps struct {
	Store    *storage.Store
	Registry *registry.Registry
	Logger   *slog.Logger
}

type RestartScheduler struct {
	store        *storage.Store
	registry     *registry.Registry
	logger       *slog.Logger
	warningMutex sync.Mutex
	warningsSent map[string]bool
}

func NewRestartScheduler(deps RestartSchedulerDeps) *RestartScheduler {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &RestartScheduler{
		store:        deps.Store,
		registry:     deps.Registry,
		logger:       logger,
		warningsSent: map[string]bool{},
	}
}

func (s *RestartScheduler) Run(ctx context.Context) {
	if s == nil || s.store == nil || s.registry == nil {
		return
	}
	ticker := time.NewTicker(restartSchedulePollInterval)
	defer ticker.Stop()
	s.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *RestartScheduler) tick(ctx context.Context) {
	schedules, err := s.store.ListEnabledRestartSchedules(ctx)
	if err != nil {
		s.logger.Warn("list restart schedules failed", "error", err)
		return
	}
	now := time.Now()
	for _, schedule := range schedules {
		if err := s.processSchedule(ctx, schedule, now); err != nil {
			s.logger.Warn("process restart schedule failed", "instance", schedule.InstanceID, "error", err)
		}
	}
}

func (s *RestartScheduler) processSchedule(ctx context.Context, schedule storage.RestartSchedule, now time.Time) error {
	instance, err := s.store.GetInstance(ctx, schedule.InstanceID)
	if err != nil {
		return err
	}
	driver, err := s.registry.Get(instance.DriverID)
	if err != nil {
		return err
	}
	instance = s.reconcile(ctx, driver, instance)
	nextShutdown, _ := nextRestartScheduleWindow(schedule, now)
	s.sendDueWarnings(ctx, driver, instance, schedule, now, nextShutdown)

	shutdown, startup := currentRestartScheduleWindow(schedule, now)
	nowLocal := now.In(scheduleLocation(schedule))
	if !nowLocal.Before(shutdown) && nowLocal.Before(startup) && shouldRunScheduleAction(schedule.LastShutdownAt, shutdown) {
		return s.runShutdown(ctx, driver, instance, schedule, shutdown)
	}
	if !nowLocal.Before(startup) && nowLocal.Sub(startup) <= restartScheduleStartupGrace && shouldRunScheduleAction(schedule.LastStartupAt, startup) {
		return s.runStartup(ctx, driver, instance, schedule, startup)
	}
	return nil
}

type restartStateReconciler interface {
	ReconcileState(ctx context.Context, instance storage.Instance) (storage.Instance, error)
}

func (s *RestartScheduler) reconcile(ctx context.Context, driver registry.GameDriver, instance storage.Instance) storage.Instance {
	reconciler, ok := driver.(restartStateReconciler)
	if !ok {
		return instance
	}
	updated, err := reconciler.ReconcileState(ctx, instance)
	if err != nil {
		s.logger.Warn("restart schedule state reconcile failed", "instance", instance.ID, "error", err)
		return instance
	}
	return updated
}

func (s *RestartScheduler) sendDueWarnings(ctx context.Context, driver registry.GameDriver, instance storage.Instance, schedule storage.RestartSchedule, now, shutdown time.Time) {
	if instance.State != storage.InstanceStateRunning {
		return
	}
	until := shutdown.Sub(now.In(scheduleLocation(schedule)))
	for _, minute := range schedule.WarningMinutes {
		window := time.Duration(minute) * time.Minute
		if until <= 0 || until > window || until < window-restartSchedulePollInterval {
			continue
		}
		key := fmt.Sprintf("%s:%s:%d", schedule.InstanceID, shutdown.Format(time.RFC3339), minute)
		if s.warningAlreadySent(key) {
			continue
		}
		message := fmt.Sprintf("[Panel] 服务器将在 %d 分钟后进入计划维护，届时会自动关闭。", minute)
		if err := s.sendSay(ctx, driver, instance, message); err != nil {
			s.logger.Warn("restart schedule warning failed", "instance", instance.ID, "minute", minute, "error", err)
			continue
		}
		s.markWarningSent(key)
	}
}

func (s *RestartScheduler) warningAlreadySent(key string) bool {
	s.warningMutex.Lock()
	defer s.warningMutex.Unlock()
	return s.warningsSent[key]
}

func (s *RestartScheduler) markWarningSent(key string) {
	s.warningMutex.Lock()
	defer s.warningMutex.Unlock()
	s.warningsSent[key] = true
	if len(s.warningsSent) > 256 {
		for k := range s.warningsSent {
			delete(s.warningsSent, k)
			if len(s.warningsSent) <= 128 {
				break
			}
		}
	}
}

type restartSaySender interface {
	SendSay(ctx context.Context, instance registry.Instance, message string) (*sj.CommandRunResult, error)
}

func (s *RestartScheduler) sendSay(ctx context.Context, driver registry.GameDriver, instance storage.Instance, message string) error {
	sender, ok := driver.(restartSaySender)
	if !ok {
		return nil
	}
	_, err := sender.SendSay(ctx, makeRegistryInstance(instance), message)
	return err
}

func (s *RestartScheduler) runShutdown(ctx context.Context, driver registry.GameDriver, instance storage.Instance, schedule storage.RestartSchedule, scheduledAt time.Time) error {
	actionAt := scheduledAt.Format(time.RFC3339)
	if schedule.SkipIfPlayersOnline {
		players, err := s.onlinePlayerCount(ctx, driver, instance)
		if err == nil && players > 0 {
			return s.store.MarkRestartScheduleAction(ctx, schedule.InstanceID, "shutdown", actionAt, "skipped_players_online", fmt.Sprintf("有 %d 名玩家在线，已跳过本次计划关闭。", players))
		}
	}
	if instance.State != storage.InstanceStateRunning && instance.State != storage.InstanceStateStarting {
		return s.store.MarkRestartScheduleAction(ctx, schedule.InstanceID, "shutdown", actionAt, "skipped_not_running", "服务器未运行，计划关闭无需执行。")
	}
	if schedule.BackupBeforeShutdown {
		if backupName, err := backupActiveSave(instance); err != nil {
			return s.store.MarkRestartScheduleAction(ctx, schedule.InstanceID, "shutdown", actionAt, "backup_failed", sanitizeErrorMsg(err, "计划关闭前备份失败"))
		} else if backupName != "" {
			s.logger.Info("restart schedule backup created", "instance", instance.ID, "backup", backupName)
		}
	}
	_ = s.sendSay(ctx, driver, instance, "[Panel] 服务器正在进入计划维护，马上关闭。")
	if err := driver.Stop(ctx, makeRegistryInstance(instance)); err != nil {
		_ = s.store.MarkRestartScheduleAction(ctx, schedule.InstanceID, "shutdown", actionAt, "failed", err.Error())
		return err
	}
	return s.store.MarkRestartScheduleAction(ctx, schedule.InstanceID, "shutdown", actionAt, "shutdown_queued", "计划关闭任务已提交。")
}

func backupActiveSave(instance storage.Instance) (string, error) {
	saveName := sj.GetActiveSaveName(instance.DataDir)
	if saveName == "" {
		return "", nil
	}
	if err := sj.ValidateSaveExists(instance.DataDir, saveName); err != nil {
		return "", err
	}
	backupPath, err := sj.BackupSave(instance.DataDir, saveName)
	if err != nil {
		return "", err
	}
	return filepath.Base(backupPath), nil
}

type restartPlayerLister interface {
	ListPlayers(ctx context.Context, instance registry.Instance) (*sj.PlayersResult, error)
}

func (s *RestartScheduler) onlinePlayerCount(ctx context.Context, driver registry.GameDriver, instance storage.Instance) (int, error) {
	lister, ok := driver.(restartPlayerLister)
	if !ok {
		return 0, nil
	}
	result, err := lister.ListPlayers(ctx, makeRegistryInstance(instance))
	if err != nil {
		return 0, err
	}
	if result.OnlineCount != nil {
		return *result.OnlineCount, nil
	}
	count := 0
	for _, player := range result.Players {
		if player.Status == "online" {
			count++
		}
	}
	return count, nil
}

func (s *RestartScheduler) runStartup(ctx context.Context, driver registry.GameDriver, instance storage.Instance, schedule storage.RestartSchedule, scheduledAt time.Time) error {
	actionAt := scheduledAt.Format(time.RFC3339)
	if instance.State == storage.InstanceStateRunning || instance.State == storage.InstanceStateStarting {
		return s.store.MarkRestartScheduleAction(ctx, schedule.InstanceID, "startup", actionAt, "skipped_already_running", "服务器已在运行，计划开启无需执行。")
	}
	saves, err := driver.ListSaves(ctx, makeRegistryInstance(instance))
	if err != nil {
		_ = s.store.MarkRestartScheduleAction(ctx, schedule.InstanceID, "startup", actionAt, "failed", err.Error())
		return err
	}
	if len(saves) == 0 {
		return s.store.MarkRestartScheduleAction(ctx, schedule.InstanceID, "startup", actionAt, "skipped_save_required", "没有可启动的存档，已跳过计划开启。")
	}
	activeName := sj.GetActiveSaveName(instance.DataDir)
	if activeName == "" {
		return s.store.MarkRestartScheduleAction(ctx, schedule.InstanceID, "startup", actionAt, "skipped_active_save_required", "没有选中的激活存档，已跳过计划开启。")
	}
	if err := sj.ValidateSaveExists(instance.DataDir, activeName); err != nil {
		return s.store.MarkRestartScheduleAction(ctx, schedule.InstanceID, "startup", actionAt, "skipped_active_save_missing", "激活存档不存在，已跳过计划开启。")
	}
	job, err := driver.Start(ctx, registry.StartRequest{Instance: makeRegistryInstance(instance)})
	if err != nil {
		_ = s.store.MarkRestartScheduleAction(ctx, schedule.InstanceID, "startup", actionAt, "failed", err.Error())
		return err
	}
	payload, _ := json.Marshal(map[string]string{"jobId": job.ID})
	return s.store.MarkRestartScheduleAction(ctx, schedule.InstanceID, "startup", actionAt, "startup_queued", string(payload))
}

func shouldRunScheduleAction(lastValue sql.NullString, scheduledAt time.Time) bool {
	if !lastValue.Valid || strings.TrimSpace(lastValue.String) == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, lastValue.String)
	if err != nil {
		return true
	}
	return last.Before(scheduledAt)
}
