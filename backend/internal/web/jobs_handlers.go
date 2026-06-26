package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/auth"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

type jobsResponse struct {
	Jobs []jobResponse `json:"jobs"`
}

type jobDetailResponse struct {
	Job jobResponse `json:"job"`
}

type jobLogsResponse struct {
	Logs []jobLogResponse `json:"logs"`
}

type jobResponse struct {
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	Status       string  `json:"status"`
	TargetType   string  `json:"targetType"`
	TargetID     string  `json:"targetId"`
	CreatedBy    *int64  `json:"createdBy"`
	CreatedAt    string  `json:"createdAt"`
	StartedAt    *string `json:"startedAt"`
	FinishedAt   *string `json:"finishedAt"`
	ErrorMessage *string `json:"errorMessage"`
	UpdatedAt    string  `json:"updatedAt"`
}

type jobLogResponse struct {
	ID        int64  `json:"id"`
	JobID     string `json:"jobId"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	CreatedAt string `json:"createdAt"`
	Sequence  int64  `json:"sequence"`
}

func (s *server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		s.handleClearJobs(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	session, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	jobsList, err := s.jobs.List(r.Context(), storage.ListJobsFilter{
		UserID:  session.User.ID,
		IsAdmin: session.User.Role == auth.RoleAdmin,
		Limit:   50,
	})
	if err != nil {
		s.logger.Error("failed to list jobs", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	response := jobsResponse{Jobs: make([]jobResponse, 0, len(jobsList))}
	for _, job := range jobsList {
		response.Jobs = append(response.Jobs, makeJobResponse(job))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) handleClearJobs(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	count, err := s.jobs.Clear(r.Context())
	if err != nil {
		if errors.Is(err, storage.ErrActiveJobsExist) {
			writeError(w, http.StatusConflict, "active_jobs_exist", "还有任务正在进行，不能清空任务中心")
			return
		}
		s.logger.Error("failed to clear jobs", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	actorID := session.User.ID
	if err := s.store.CreateAuditLog(r.Context(), storage.AuditLogParams{
		ActorUserID: &actorID,
		Action:      "jobs_cleared",
		TargetType:  "jobs",
		TargetID:    "all",
		Metadata:    fmt.Sprintf(`{"count":%d}`, count),
		IPAddress:   remoteIP(r),
		UserAgent:   userAgent(r),
	}); err != nil {
		s.logger.Error("failed to write jobs clear audit", "error", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": count})
}

func (s *server) handleJobByID(w http.ResponseWriter, r *http.Request) {
	value := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	parts := strings.Split(value, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	jobID := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleJobDetail(w, r, jobID)
		return
	}
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	switch parts[1] {
	case "logs":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleJobLogs(w, r, jobID)
	case "stream":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleJobStream(w, r, jobID)
	case "cancel":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.handleCancelJob(w, r, jobID)
	default:
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	}
}

func (s *server) handleJobDetail(w http.ResponseWriter, r *http.Request, jobID string) {
	session, job, ok := s.loadReadableJob(w, r, jobID)
	_ = session
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, jobDetailResponse{Job: makeJobResponse(job)})
}

func (s *server) handleJobLogs(w http.ResponseWriter, r *http.Request, jobID string) {
	_, _, ok := s.loadReadableJob(w, r, jobID)
	if !ok {
		return
	}
	after, ok := parseOptionalIntQuery(w, r, "after", 0)
	if !ok {
		return
	}
	limit, ok := parseOptionalIntQuery(w, r, "limit", 200)
	if !ok {
		return
	}
	if limit > 1000 {
		limit = 1000
	}
	logs, err := s.jobs.Logs(r.Context(), jobID, after, int(limit))
	if err != nil {
		s.logger.Error("failed to list job logs", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	response := jobLogsResponse{Logs: make([]jobLogResponse, 0, len(logs))}
	for _, logLine := range logs {
		response.Logs = append(response.Logs, makeJobLogResponse(logLine))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) handleTestJob(w http.ResponseWriter, r *http.Request, shouldFail bool) {
	session, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	jobType := "test"
	if shouldFail {
		jobType = "test_fail"
	}
	job, err := s.jobs.Start(r.Context(), jobs.Spec{
		Type:       jobType,
		TargetType: "instance",
		TargetID:   storage.DefaultInstanceID,
		CreatedBy:  session.User.ID,
		Timeout:    15 * time.Second,
		Run: func(ctx context.Context, jobContext *jobs.Context) error {
			for i := 1; i <= 5; i++ {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Second):
				}
				if shouldFail && i == 4 {
					_, _ = jobContext.Warn(ctx, "模拟失败任务即将返回错误。")
					_, _ = jobContext.Error(ctx, "模拟失败任务触发预期错误。")
					return errors.New("模拟失败任务已失败")
				}
				_, _ = jobContext.Info(ctx, fmt.Sprintf("模拟任务进度 %d/5", i))
			}
			return nil
		},
	})
	if err != nil {
		s.logger.Error("failed to start test job", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}
	actorID := session.User.ID
	if err := s.store.CreateAuditLog(r.Context(), storage.AuditLogParams{
		ActorUserID: &actorID,
		Action:      "job_created",
		TargetType:  "job",
		TargetID:    job.ID,
		Metadata:    `{"type":"` + jobType + `"}`,
		IPAddress:   remoteIP(r),
		UserAgent:   userAgent(r),
	}); err != nil {
		s.logger.Error("failed to write job audit", "job_id", job.ID, "error", err)
	}
	writeJSON(w, http.StatusAccepted, jobDetailResponse{Job: makeJobResponse(job)})
}

func (s *server) handleCancelJob(w http.ResponseWriter, r *http.Request, jobID string) {
	_, _, ok := s.loadReadableJob(w, r, jobID)
	if !ok {
		return
	}
	writeError(w, http.StatusNotImplemented, "not_implemented", "任务取消机制将在后续里程碑接入")
}

func (s *server) handleJobStream(w http.ResponseWriter, r *http.Request, jobID string) {
	_, job, ok := s.loadReadableJob(w, r, jobID)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "stream_not_supported", "streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	after := int64(0)
	if lastEventID := strings.TrimSpace(r.Header.Get("Last-Event-ID")); lastEventID != "" {
		if parsed, err := strconv.ParseInt(lastEventID, 10, 64); err == nil && parsed > 0 {
			after = parsed
		}
	}
	if queryAfter := strings.TrimSpace(r.URL.Query().Get("after")); queryAfter != "" {
		if parsed, err := strconv.ParseInt(queryAfter, 10, 64); err == nil && parsed >= 0 {
			after = parsed
		}
	}

	logs, err := s.jobs.Logs(r.Context(), jobID, after, 1000)
	if err != nil {
		s.logger.Error("failed to replay job logs", "job_id", jobID, "error", err)
		return
	}
	for _, logLine := range logs {
		writeSSE(w, "log", strconv.FormatInt(logLine.Sequence, 10), makeJobLogResponse(logLine))
	}
	flusher.Flush()

	if isTerminalJobStatus(job.Status) {
		writeSSE(w, "finished", "", makeJobResponse(job))
		flusher.Flush()
		return
	}

	events, unsubscribe := s.jobs.Subscribe(jobID)
	defer unsubscribe()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			writeSSE(w, "ping", "", map[string]bool{"ok": true})
			flusher.Flush()
		case event, ok := <-events:
			if !ok {
				return
			}
			if event.Log != nil {
				writeSSE(w, "log", strconv.FormatInt(event.Log.Sequence, 10), makeJobLogResponse(*event.Log))
			}
			if event.Job != nil && event.Type == jobs.EventFinished {
				writeSSE(w, "finished", "", makeJobResponse(*event.Job))
				flusher.Flush()
				return
			}
			flusher.Flush()
		}
	}
}

func (s *server) loadReadableJob(w http.ResponseWriter, r *http.Request, jobID string) (currentSession, storage.Job, bool) {
	session, ok := s.requireAuth(w, r)
	if !ok {
		return currentSession{}, storage.Job{}, false
	}
	job, err := s.jobs.Get(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job_not_found", "任务不存在")
			return currentSession{}, storage.Job{}, false
		}
		s.logger.Error("failed to load job", "job_id", jobID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return currentSession{}, storage.Job{}, false
	}
	if session.User.Role != auth.RoleAdmin && (!job.CreatedBy.Valid || job.CreatedBy.Int64 != session.User.ID) {
		writeError(w, http.StatusForbidden, "forbidden", "没有权限查看该任务")
		return currentSession{}, storage.Job{}, false
	}
	return session, job, true
}

func writeSSE(w http.ResponseWriter, event string, id string, payload any) {
	if id != "" {
		_, _ = fmt.Fprintf(w, "id: %s\n", id)
	}
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte(`{"error":"encode_failed"}`)
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}

func parseOptionalIntQuery(w http.ResponseWriter, r *http.Request, key string, fallback int64) (int64, bool) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return fallback, true
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		writeError(w, http.StatusBadRequest, "invalid_query", "query parameter must be a non-negative integer")
		return 0, false
	}
	return parsed, true
}

func makeJobResponse(job storage.Job) jobResponse {
	return jobResponse{
		ID:           job.ID,
		Type:         job.Type,
		Status:       job.Status,
		TargetType:   job.TargetType,
		TargetID:     job.TargetID,
		CreatedBy:    nullableInt64(job.CreatedBy),
		CreatedAt:    job.CreatedAt,
		StartedAt:    nullableString(job.StartedAt),
		FinishedAt:   nullableString(job.FinishedAt),
		ErrorMessage: nullableString(job.ErrorMessage),
		UpdatedAt:    job.UpdatedAt,
	}
}

func makeJobLogResponse(logLine storage.JobLog) jobLogResponse {
	return jobLogResponse{
		ID:        logLine.ID,
		JobID:     logLine.JobID,
		Level:     logLine.Level,
		Message:   logLine.Message,
		CreatedAt: logLine.CreatedAt,
		Sequence:  logLine.Sequence,
	}
}

func nullableInt64(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	return &value.Int64
}

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func isTerminalJobStatus(status string) bool {
	return status == storage.JobStatusSucceeded || status == storage.JobStatusFailed || status == storage.JobStatusCanceled
}
