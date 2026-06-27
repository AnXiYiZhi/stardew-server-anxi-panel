package web

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	paneldocker "github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// handleSupportBundle handles POST /api/instances/:id/support-bundle.
// Exports a diagnostic ZIP containing redacted system info, health checks,
// instance state, recent jobs, audit logs, and compose status.
// Only accessible to admin users.
func (s *server) handleSupportBundle(w http.ResponseWriter, r *http.Request, instanceID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	session, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}

	instance, ok := s.loadInstance(w, r, instanceID)
	if !ok {
		return
	}

	ctx := r.Context()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// 1. Version info
	s.addVersionBundle(zw)

	// 2. Health diagnostics
	s.addHealthBundle(ctx, zw)

	// 3. Instance state
	s.addInstanceStateBundle(ctx, zw, instance)

	// 4. Recent jobs summary
	s.addJobsBundle(ctx, zw)

	// 5. Recent audit logs (redacted)
	s.addAuditLogsBundle(ctx, zw)

	// 6. Docker compose ps
	s.addComposePsBundle(ctx, zw, instance.DataDir)

	// 7. Compose config summary (no secrets)
	s.addComposeConfigBundle(zw, instance.DataDir)

	// 8. Server logs tail (redacted)
	s.addServerLogsBundle(ctx, zw, instance.DataDir)

	if err := zw.Close(); err != nil {
		s.logger.Error("failed to close support bundle zip", "error", err)
		writeError(w, http.StatusInternalServerError, "bundle_failed", "诊断包生成失败")
		return
	}

	filename := fmt.Sprintf("support-bundle-%s.zip", time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	w.Write(buf.Bytes())

	s.auditLog(r, &session, "support_bundle_export", "instance", instanceID, "")
}

func (s *server) addVersionBundle(zw *zip.Writer) {
	info := map[string]string{
		"version":   s.config.Version,
		"commit":    s.config.Commit,
		"buildDate": s.config.BuildDate,
		"exportedAt": time.Now().UTC().Format(time.RFC3339),
	}
	writeJSONToZip(zw, "version.json", info)
}

func (s *server) addHealthBundle(ctx context.Context, zw *zip.Writer) {
	var checks []HealthCheck

	dockerOK := func() bool {
		result, err := s.docker.DockerVersion(ctx, "")
		return err == nil && result.ExitCode == 0
	}()
	if dockerOK {
		checks = append(checks, HealthCheck{Name: "docker_daemon", Status: "ok", Message: "Docker 服务正常"})
	} else {
		checks = append(checks, HealthCheck{Name: "docker_daemon", Status: "error", Message: "Docker 服务不可用"})
	}

	composeOK := func() bool {
		result, err := s.docker.ComposeVersion(ctx, "")
		return err == nil && result.ExitCode == 0
	}()
	if composeOK {
		checks = append(checks, HealthCheck{Name: "docker_compose", Status: "ok", Message: "Docker Compose 可用"})
	} else {
		checks = append(checks, HealthCheck{Name: "docker_compose", Status: "error", Message: "Docker Compose 不可用"})
	}

	dataDirOK := s.checkDataDir()
	if dataDirOK {
		checks = append(checks, HealthCheck{Name: "data_dir", Status: "ok", Message: "数据目录可写"})
	} else {
		checks = append(checks, HealthCheck{Name: "data_dir", Status: "error", Message: "数据目录不可写"})
	}

	writeJSONToZip(zw, "health.json", map[string]any{
		"status": "collected",
		"checks": checks,
	})
}

func (s *server) addInstanceStateBundle(ctx context.Context, zw *zip.Writer, instance storage.Instance) {
	stateData := map[string]any{
		"instanceId":   instance.ID,
		"driverId":     instance.DriverID,
		"name":         instance.Name,
		"state":        instance.State,
		"stateMessage": instance.StateMessage.String,
		"driverPhase":  instance.DriverPhase,
		"createdAt":    instance.CreatedAt,
		"updatedAt":    instance.UpdatedAt,
	}
	writeJSONToZip(zw, "instance-state.json", stateData)
}

func (s *server) addJobsBundle(ctx context.Context, zw *zip.Writer) {
	jobs, err := s.store.ListJobs(ctx, storage.ListJobsFilter{Limit: 20, IsAdmin: true})
	if err != nil {
		writeJSONToZip(zw, "jobs.json", map[string]string{"error": "读取任务列表失败"})
		return
	}

	type jobSummary struct {
		ID           string `json:"id"`
		Type         string `json:"type"`
		Status       string `json:"status"`
		ErrorMessage string `json:"errorMessage,omitempty"`
		CreatedAt    string `json:"createdAt"`
		FinishedAt   string `json:"finishedAt,omitempty"`
	}
summaries := make([]jobSummary, 0, len(jobs))
	for _, j := range jobs {
		errMsg := ""
		if j.ErrorMessage.Valid {
			errMsg = paneldocker.RedactString(j.ErrorMessage.String)
		}
		finishedAt := ""
		if j.FinishedAt.Valid {
			finishedAt = j.FinishedAt.String
		}
		summaries = append(summaries, jobSummary{
			ID:           j.ID,
			Type:         j.Type,
			Status:       j.Status,
			ErrorMessage: errMsg,
			CreatedAt:    j.CreatedAt,
			FinishedAt:   finishedAt,
		})
	}
	writeJSONToZip(zw, "jobs.json", summaries)
}

func (s *server) addAuditLogsBundle(ctx context.Context, zw *zip.Writer) {
	logs, _, err := s.store.ListAuditLogs(ctx, storage.ListAuditLogsParams{Limit: 50, Offset: 0})
	if err != nil {
		writeJSONToZip(zw, "audit-logs.json", map[string]string{"error": "读取审计日志失败"})
		return
	}

	type auditSummary struct {
		ID         int64  `json:"id"`
		Action     string `json:"action"`
		ActorName  string `json:"actorName,omitempty"`
		TargetType string `json:"targetType"`
		TargetID   string `json:"targetId,omitempty"`
		IPAddress  string `json:"ipAddress,omitempty"`
		CreatedAt  string `json:"createdAt"`
	}
summaries := make([]auditSummary, 0, len(logs))
	for _, l := range logs {
		actorName := ""
		if l.ActorName != nil {
			actorName = *l.ActorName
		}
		targetID := ""
		if l.TargetID != nil {
			targetID = *l.TargetID
		}
		ipAddr := ""
		if l.IPAddress != nil {
			ipAddr = *l.IPAddress
		}
		summaries = append(summaries, auditSummary{
			ID:         l.ID,
			Action:     l.Action,
			ActorName:  actorName,
			TargetType: l.TargetType,
			TargetID:   targetID,
			IPAddress:  ipAddr,
			CreatedAt:  l.CreatedAt,
		})
	}
	writeJSONToZip(zw, "audit-logs.json", summaries)
}

func (s *server) addComposePsBundle(ctx context.Context, zw *zip.Writer, dataDir string) {
	result, err := s.docker.ComposePs(ctx, dataDir)
	if err != nil {
		writeJSONToZip(zw, "compose-ps.json", map[string]string{"error": "Compose PS 执行失败"})
		return
	}
	writeJSONToZip(zw, "compose-ps.json", result)
}

func (s *server) addComposeConfigBundle(zw *zip.Writer, dataDir string) {
	composePath := filepath.Join(dataDir, "docker-compose.yml")
	data, err := os.ReadFile(composePath)
	if err != nil {
		writeJSONToZip(zw, "compose-config.json", map[string]string{"note": "docker-compose.yml 不存在或无法读取"})
		return
	}
	// Redact any sensitive values in compose config
	redacted := paneldocker.RedactString(string(data))
	writeStringToZip(zw, "docker-compose.yml", redacted)
}

func (s *server) addServerLogsBundle(ctx context.Context, zw *zip.Writer, dataDir string) {
	result, err := s.docker.ComposeLogs(ctx, dataDir, paneldocker.LogsOptions{
		Service: "server",
		Tail:    200,
	})
	if err != nil {
		writeJSONToZip(zw, "server-logs.txt", map[string]string{"error": "读取服务器日志失败"})
		return
	}
	redacted := paneldocker.RedactString(result.Stdout)
	writeStringToZip(zw, "server-logs.txt", redacted)
}

// writeJSONToZip writes a JSON-serialized object to a zip entry.
func writeJSONToZip(zw *zip.Writer, name string, v any) {
	f, err := zw.Create(name)
	if err != nil {
		return
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

// writeStringToZip writes a plain string to a zip entry.
func writeStringToZip(zw *zip.Writer, name string, content string) {
	f, err := zw.Create(name)
	if err != nil {
		return
	}
	f.Write([]byte(content))
}
