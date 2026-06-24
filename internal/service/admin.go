package service

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type adminResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type adminCompactResponse struct {
	OK             bool   `json:"ok"`
	TaskID         string `json:"task_id,omitempty"`
	State          string `json:"state"`
	DurationMillis int64  `json:"duration_ms"`
	Error          string `json:"error,omitempty"`
}

type AdminAuditEvent struct {
	Action          string        `json:"action"`
	TaskID          string        `json:"task_id"`
	State           string        `json:"state"`
	Duration        time.Duration `json:"duration"`
	Error           string        `json:"error,omitempty"`
	StartedUnixNano int64         `json:"started_unix_nano"`
}

func compactHandler(
	timeout time.Duration,
	token string,
	audit AuditLogger,
	compact CompactFunc,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeJSON(writer, http.StatusMethodNotAllowed, adminResponse{OK: false, Error: "method not allowed"})
			return
		}
		if token == "" {
			writeJSON(writer, http.StatusServiceUnavailable, adminResponse{OK: false, Error: "admin auth token required"})
			return
		}
		if !authorizedAdminRequest(request, token) {
			logger.Warn("admin auth failed", "remote_addr", request.RemoteAddr)
			writeJSON(writer, http.StatusUnauthorized, adminResponse{OK: false, Error: "admin unauthorized"})
			return
		}
		if compact == nil {
			writeJSON(writer, http.StatusServiceUnavailable, adminResponse{OK: false, Error: "compact unavailable"})
			return
		}
		if timeout <= 0 && !hasDeadline(request.Context()) {
			writeJSON(writer, http.StatusBadRequest, adminResponse{OK: false, Error: "admin compact timeout required"})
			return
		}
		started := time.Now()
		taskID := newCompactTaskID(started)
		ctx := request.Context()
		cancel := func() {}
		if timeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, timeout)
		}
		defer cancel()
		if err := compact(ctx); err != nil {
			response := failedCompactResponse(taskID, started, err.Error())
			logCompactAudit(audit, response, started)
			writeJSON(writer, http.StatusInternalServerError, response)
			return
		}
		response := succeededCompactResponse(taskID, started)
		logCompactAudit(audit, response, started)
		writeJSON(writer, http.StatusOK, response)
	}
}

func authorizedAdminRequest(request *http.Request, token string) bool {
	candidate := request.Header.Get("X-MTS-Admin-Token")
	if candidate == "" {
		candidate = strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
	}
	if candidate == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(token)) == 1
}

func hasDeadline(ctx context.Context) bool {
	_, ok := ctx.Deadline()
	return ok
}

func newCompactTaskID(started time.Time) string {
	return "compact-" + strconv.FormatInt(started.UnixNano(), 10)
}

func succeededCompactResponse(taskID string, started time.Time) adminCompactResponse {
	return adminCompactResponse{
		OK:             true,
		TaskID:         taskID,
		State:          "succeeded",
		DurationMillis: time.Since(started).Milliseconds(),
	}
}

func failedCompactResponse(taskID string, started time.Time, message string) adminCompactResponse {
	return adminCompactResponse{
		OK:             false,
		TaskID:         taskID,
		State:          "failed",
		DurationMillis: time.Since(started).Milliseconds(),
		Error:          message,
	}
}

func logCompactAudit(audit AuditLogger, response adminCompactResponse, started time.Time) {
	if audit == nil {
		return
	}
	audit.LogAdminAction(AdminAuditEvent{
		Action:          "compact",
		TaskID:          response.TaskID,
		State:           response.State,
		Duration:        time.Duration(response.DurationMillis) * time.Millisecond,
		Error:           response.Error,
		StartedUnixNano: started.UnixNano(),
	})
}
