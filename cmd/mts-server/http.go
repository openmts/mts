package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	mts "github.com/openmts/mts"
)

func (r *serverRuntime) httpHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", r.handleHealth)
	mux.HandleFunc("/readyz", r.handleHealth)
	mux.HandleFunc("/metrics", r.handleMetrics)
	mux.HandleFunc("/api/v1/data/write", r.handleWrite)
	mux.HandleFunc("/api/v1/data/write/typed", r.handleWriteTyped)
	mux.HandleFunc("/api/v1/data/query/rows", r.handleQueryRows)
	mux.HandleFunc("/api/v1/data/query/columns", r.handleQueryColumns)
	mux.HandleFunc("/api/v1/data/query/explain", r.handleQueryExplain)
	mux.HandleFunc("/api/v1/data/query/stream", r.handleQueryStream)
	mux.HandleFunc("/api/v1/data/query/stats", r.handleQueryStats)
	mux.HandleFunc("/api/v1/data/databases/", r.handleDataDatabase)
	mux.HandleFunc("/api/v1/users", r.handleUsers)
	mux.HandleFunc("/api/v1/users/", r.handleUserResource)
	mux.HandleFunc("/api/v1/authz/database/check", r.handleAuthzDatabaseCheck)
	mux.HandleFunc("/api/v1/admin/databases", r.handleAdminDatabases)
	mux.HandleFunc("/api/v1/admin/databases/", r.handleAdminDatabaseResource)
	mux.HandleFunc("/api/v1/admin/config", r.handleConfig)
	mux.HandleFunc("/api/v1/admin/config/effective", r.handleConfig)
	mux.HandleFunc("/api/v1/admin/config/schema", r.handleConfigSchema)
	mux.HandleFunc("/api/v1/admin/flush", r.handleFlush)
	mux.HandleFunc("/api/v1/admin/compact", r.handleCompact)
	mux.HandleFunc("/api/v1/admin/retention/apply", r.handleApplyRetention)
	mux.HandleFunc("/api/v1/admin/maintenance/errors", r.handleMaintenanceErrors)
	mux.HandleFunc("/api/v1/admin/stats/storage-memory", r.handleStorageMemory)
	mux.HandleFunc("/api/v1/admin/stats/compaction", r.handleCompactionStats)
	mux.HandleFunc("/api/v1/admin/health", r.handleAdminHealth)
	mux.HandleFunc("/api/v1/admin/downsample/policies", r.handleDownsamplePolicies)
	mux.HandleFunc("/api/v1/admin/downsample/policies/", r.handleDownsamplePolicyResource)
	mux.HandleFunc("/api/v1/admin/downsample/statuses", r.handleDownsampleStatuses)
	return mux
}

func (r *serverRuntime) handleHealth(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodGet) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.health())
}

func decodeHTTPJSON(request *http.Request, value any) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

func writeHTTPJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeAPIError(writer http.ResponseWriter, err error) {
	status, response := apiErrorResponse(err)
	writeHTTPJSON(writer, status, response)
}

func apiErrorResponse(err error) (int, errorResponse) {
	if err == nil {
		return http.StatusInternalServerError, errorResponse{Code: errorCodeInternal}
	}
	code := errorCodeInternal
	statusCode := http.StatusInternalServerError
	var apiErr apiError
	if errors.As(err, &apiErr) {
		code = apiErr.Code
		statusCode = httpStatusForErrorCode(code)
	} else if errors.Is(err, mts.ErrPermissionDenied) {
		code = errorCodePermissionDenied
		statusCode = http.StatusForbidden
	} else if errors.Is(err, mts.ErrUserNotFound) {
		code = errorCodeNotFound
		statusCode = http.StatusNotFound
	} else if looksLikeNotFoundError(err) {
		code = errorCodeNotFound
		statusCode = http.StatusNotFound
	} else if errors.Is(err, mts.ErrUserAlreadyExists) {
		code = errorCodeAlreadyExists
		statusCode = http.StatusConflict
	} else if errors.Is(err, mts.ErrInvalidUser) ||
		errors.Is(err, mts.ErrInvalidPermission) ||
		errors.Is(err, mts.ErrInvalidPrecision) {
		code = errorCodeBadRequest
		statusCode = http.StatusBadRequest
	} else if looksLikeValidationError(err) {
		code = errorCodeBadRequest
		statusCode = http.StatusBadRequest
	}
	return statusCode, errorResponse{OK: false, Code: code, Message: err.Error(), Error: err.Error()}
}

func httpStatusForErrorCode(code errorCode) int {
	switch code {
	case errorCodeBadRequest:
		return http.StatusBadRequest
	case errorCodeUnauthenticated:
		return http.StatusUnauthorized
	case errorCodePermissionDenied:
		return http.StatusForbidden
	case errorCodeNotFound:
		return http.StatusNotFound
	case errorCodeAlreadyExists:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

type apiError struct {
	Code    errorCode
	Message string
	Cause   error
}

func newAPIError(code errorCode, message string, cause error) error {
	return apiError{Code: code, Message: message, Cause: cause}
}

func (e apiError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return string(e.Code)
}

func (e apiError) Unwrap() error { return e.Cause }

func looksLikeNotFoundError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}

func looksLikeValidationError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "invalid") ||
		strings.Contains(message, "must ") ||
		strings.Contains(message, "required") ||
		strings.Contains(message, "incomplete") ||
		strings.Contains(message, "empty")
}

func requireHTTPMethod(writer http.ResponseWriter, request *http.Request, method string) bool {
	if request.Method == method {
		return true
	}
	writeHTTPJSON(writer, http.StatusMethodNotAllowed, errorResponse{
		OK:      false,
		Code:    errorCodeBadRequest,
		Message: "method not allowed",
		Error:   "method not allowed",
	})
	return false
}

func splitPath(path string, prefix string) []string {
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}
