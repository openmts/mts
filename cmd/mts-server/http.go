package main

import (
	"errors"
	"net/http"
	"net/http/pprof"
	"strings"

	mts "github.com/openmts/mts"
	"github.com/openmts/mts/internal/httpjson"
)

func (r *serverRuntime) httpHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(routeHealth, r.handleHealth)
	mux.HandleFunc(routeReady, r.handleHealth)
	mux.HandleFunc(routeMetrics, r.handleMetrics)
	mux.HandleFunc(routeDataWrite, r.handleWrite)
	mux.HandleFunc(routeDataWriteTyped, r.handleWriteTyped)
	mux.HandleFunc(routeDataQueryRows, r.handleQueryRows)
	mux.HandleFunc(routeDataQueryColumns, r.handleQueryColumns)
	mux.HandleFunc(routeDataQueryExplain, r.handleQueryExplain)
	mux.HandleFunc(routeDataQueryStream, r.handleQueryStream)
	mux.HandleFunc(routeDataQueryStats, r.handleQueryStats)
	mux.HandleFunc(routeDataDatabasesPrefix, r.handleDataDatabase)
	mux.HandleFunc(routeAuthLogin, r.handleLogin)
	mux.HandleFunc(routeAuthLogout, r.handleLogout)
	mux.HandleFunc(routeAuthPassword, r.handleChangePassword)
	mux.HandleFunc(routeUsers, r.handleUsers)
	mux.HandleFunc(routeUsersPrefix, r.handleUserResource)
	mux.HandleFunc(routeAuthzDatabaseCheck, r.handleAuthzDatabaseCheck)
	mux.HandleFunc(routeAdminDatabases, r.handleAdminDatabases)
	mux.HandleFunc(routeAdminDatabasesPrefix, r.handleAdminDatabaseResource)
	mux.HandleFunc(routeAdminConfig, r.handleConfig)
	mux.HandleFunc(routeAdminConfigEffective, r.handleConfig)
	mux.HandleFunc(routeAdminConfigSchema, r.handleConfigSchema)
	mux.HandleFunc(routeAdminFlush, r.handleFlush)
	mux.HandleFunc(routeAdminCompact, r.handleCompact)
	mux.HandleFunc(routeAdminRetentionApply, r.handleApplyRetention)
	mux.HandleFunc(routeAdminMaintenanceErrors, r.handleMaintenanceErrors)
	mux.HandleFunc(routeAdminStatsStorageMemory, r.handleStorageMemory)
	mux.HandleFunc(routeAdminStatsCompaction, r.handleCompactionStats)
	mux.HandleFunc(routeAdminHealth, r.handleAdminHealth)
	mux.HandleFunc(routeAdminDownsamplePolicies, r.handleDownsamplePolicies)
	mux.HandleFunc(routeAdminDownsamplePrefix, r.handleDownsamplePolicyResource)
	mux.HandleFunc(routeAdminDownsampleStatuses, r.handleDownsampleStatuses)
	mux.HandleFunc(routeAdminAPISpec, r.handleAPISpec)
	mux.HandleFunc(routeAdminErrorCodes, r.handleErrorCodes)
	mux.HandleFunc(routeAdminConfigValidate, r.handleValidateConfig)
	mux.HandleFunc(routeAdminConfigReload, r.handleReloadConfig)
	mux.HandleFunc(routeAdminStorageValidate, r.handleStorageValidate)
	mux.HandleFunc(routeAdminStorageSnapshot, r.handleStorageSnapshot)
	mux.HandleFunc(routeAdminStorageExport, r.handleStorageExport)
	r.mountPprof(mux)
	mux.HandleFunc(routeRoot, dashboardHandler().ServeHTTP)
	return r.wrapHTTP(mux)
}

func (r *serverRuntime) mountPprof(mux *http.ServeMux) {
	mux.HandleFunc(routePprofPrefix, r.adminHTTPHandler(pprof.Index))
	mux.HandleFunc(routePprofCmdline, r.adminHTTPHandler(pprof.Cmdline))
	mux.HandleFunc(routePprofProfile, r.adminHTTPHandler(pprof.Profile))
	mux.HandleFunc(routePprofSymbol, r.adminHTTPHandler(pprof.Symbol))
	mux.HandleFunc(routePprofTrace, r.adminHTTPHandler(pprof.Trace))
}

func (r *serverRuntime) adminHTTPHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		cfg := r.currentConfig()
		if !cfg.Observability.Pprof.Enabled {
			http.NotFound(writer, request)
			return
		}
		if err := r.requireHTTPAdmin(request); err != nil {
			writeAPIError(writer, err)
			return
		}
		next(writer, request)
	}
}

func (r *serverRuntime) handleHealth(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodGet) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.health())
}

func decodeHTTPJSON(request *http.Request, value any) error {
	return httpjson.DecodeStrict(request, value)
}

func writeHTTPJSON(writer http.ResponseWriter, status int, value any) {
	httpjson.Write(writer, status, value)
}

func writeAPIError(writer http.ResponseWriter, err error) {
	status, response := apiErrorResponse(err)
	writeHTTPJSON(writer, status, response)
}

func apiErrorResponse(err error) (int, errorResponse) {
	classified := classifyAPIError(err)
	return httpStatusForErrorCode(classified.Code), errorResponse{
		OK:      false,
		Code:    classified.Code,
		Message: classified.Message,
		Error:   classified.Message,
	}
}

type apiErrorClass struct {
	Code    errorCode
	Message string
}

func classifyAPIError(err error) apiErrorClass {
	if err == nil {
		return apiErrorClass{Code: errorCodeInternal, Message: string(errorCodeInternal)}
	}
	var apiErr apiError
	switch {
	case errors.As(err, &apiErr):
		return apiErrorClass{Code: apiErr.Code, Message: err.Error()}
	case errors.Is(err, mts.ErrPermissionDenied):
		return apiErrorClass{Code: errorCodePermissionDenied, Message: err.Error()}
	case errors.Is(err, mts.ErrNotFound),
		errors.Is(err, mts.ErrUserNotFound):
		return apiErrorClass{Code: errorCodeNotFound, Message: err.Error()}
	case errors.Is(err, mts.ErrUserAlreadyExists):
		return apiErrorClass{Code: errorCodeAlreadyExists, Message: err.Error()}
	case errors.Is(err, mts.ErrInvalidCredentials), errors.Is(err, mts.ErrAuthenticationDisabled):
		return apiErrorClass{Code: errorCodeUnauthenticated, Message: err.Error()}
	case errors.Is(err, mts.ErrResourceExhausted),
		errors.Is(err, mts.ErrCardinalityLimit),
		errors.Is(err, mts.ErrStorageMemoryLimitExceeded),
		errors.Is(err, mts.ErrReadBudgetExceeded),
		errors.Is(err, mts.ErrEngineBusy):
		return apiErrorClass{Code: errorCodeResourceExhausted, Message: err.Error()}
	case errors.Is(err, mts.ErrInvalidOptions),
		errors.Is(err, mts.ErrInvalidUser),
		errors.Is(err, mts.ErrInvalidPermission),
		errors.Is(err, mts.ErrInvalidPrecision),
		errors.Is(err, mts.ErrUnsupported):
		return apiErrorClass{Code: errorCodeBadRequest, Message: err.Error()}
	case looksLikeNotFoundError(err):
		return apiErrorClass{Code: errorCodeNotFound, Message: err.Error()}
	case looksLikeValidationError(err):
		return apiErrorClass{Code: errorCodeBadRequest, Message: err.Error()}
	default:
		return apiErrorClass{Code: errorCodeInternal, Message: err.Error()}
	}
}

func httpStatusForErrorCode(code errorCode) int {
	switch code {
	case errorCodeBadRequest:
		return http.StatusBadRequest
	case errorCodeUnauthenticated:
		return http.StatusUnauthorized
	case errorCodePermissionDenied:
		return http.StatusForbidden
	case errorCodeResourceExhausted:
		return http.StatusTooManyRequests
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
	writer.Header().Set("Allow", method)
	writeHTTPJSON(writer, http.StatusMethodNotAllowed, errorResponse{
		OK:      false,
		Code:    errorCodeBadRequest,
		Message: messageMethodNotAllowed,
		Error:   messageMethodNotAllowed,
	})
	return false
}

func (r *serverRuntime) requireHTTPAdminMethod(
	writer http.ResponseWriter,
	request *http.Request,
	method string,
) bool {
	if !requireHTTPMethod(writer, request, method) {
		return false
	}
	if err := r.requireHTTPAdmin(request); err != nil {
		writeAPIError(writer, err)
		return false
	}
	return true
}

func splitPath(path string, prefix string) []string {
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}
