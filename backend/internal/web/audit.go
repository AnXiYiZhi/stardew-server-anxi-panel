package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker"
	"github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage"
)

// auditLog writes an audit log entry. It silently ignores errors to avoid
// breaking the main operation flow.
func (s *server) auditLog(r *http.Request, session *currentSession, action, targetType, targetID, metadata string) {
	params := storage.AuditLogParams{
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Metadata:   metadata,
		IPAddress:  remoteIP(r),
		UserAgent:  userAgent(r),
	}
	if session != nil {
		params.ActorUserID = &session.User.ID
	}
	if err := s.store.CreateAuditLog(r.Context(), params); err != nil {
		s.logger.Error("failed to write audit log", "action", action, "target", targetType+"/"+targetID, "error", err)
	}
}

// sanitizeError converts an internal error into a user-safe message.
// It strips file paths, Docker error details, and internal Go error chains.
// Returns a Chinese message suitable for frontend display.
func sanitizeError(err error, fallback string) string {
	if err == nil {
		return fallback
	}
	msg := err.Error()
	lower := strings.ToLower(msg)

	// Database errors
	if strings.Contains(lower, "sql") || strings.Contains(lower, "database") || strings.Contains(lower, "sqlite") {
		return "数据库操作失败"
	}
	// Docker errors
	if strings.Contains(lower, "docker") || strings.Contains(lower, "compose") || strings.Contains(lower, "container") {
		if strings.Contains(lower, "not found") || strings.Contains(lower, "no such") {
			return "Docker 容器或服务未找到"
		}
		if strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out") {
			return "Docker 操作超时，请稍后重试"
		}
		if strings.Contains(lower, "permission") || strings.Contains(lower, "denied") {
			return "Docker 权限不足，请检查 socket 挂载"
		}
		return "Docker 操作失败"
	}
	// File system errors
	if strings.Contains(lower, "permission denied") {
		return "文件权限不足"
	}
	if strings.Contains(lower, "no such file") || strings.Contains(lower, "not a directory") {
		return "目标文件或目录不存在"
	}
	if strings.Contains(lower, "disk") || strings.Contains(lower, "no space") {
		return "磁盘空间不足"
	}
	// Network errors
	if strings.Contains(lower, "connection refused") || strings.Contains(lower, "network") {
		return "网络连接失败"
	}
	// ZIP/archive errors
	if strings.Contains(lower, "zip") || strings.Contains(lower, "archive") {
		return "压缩包格式错误或已损坏"
	}

	// Redact any sensitive values from the fallback message
	return docker.RedactString(fallback)
}

// sanitizeErrorMessage is like sanitizeError but takes the raw error message string.
func sanitizeErrorMessage(msg string) string {
	return docker.RedactString(msg)
}

// sanitizeErrorMsg returns a user-safe error message with a Chinese prefix.
// The internal error details are stripped; only the prefix is shown to the user.
// If err is nil, returns the prefix with "未知错误".
func sanitizeErrorMsg(err error, prefix string) string {
	if err == nil {
		return prefix + "：未知错误"
	}
	safe := sanitizeError(err, "操作失败")
	return prefix + "：" + safe
}

// auditMetadata creates a JSON metadata string for audit logs.
// It automatically redacts sensitive values.
func auditMetadata(pairs ...string) string {
	if len(pairs) == 0 {
		return "{}"
	}
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		key := pairs[i]
		val := docker.RedactString(pairs[i+1])
		parts = append(parts, fmt.Sprintf("%q:%q", key, val))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// handleAuditLogs handles GET /api/audit-logs.
// Returns paginated audit logs. Admin-only.
func (s *server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}

	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &limit); n != 1 || err != nil || limit <= 0 || limit > 200 {
			limit = 50
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &offset); n != 1 || err != nil || offset < 0 {
			offset = 0
		}
	}

	entries, total, err := s.store.ListAuditLogs(r.Context(), storage.ListAuditLogsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		s.logger.Error("failed to list audit logs", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "服务器内部错误")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"logs":   entries,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
