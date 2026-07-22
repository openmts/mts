package main

import (
	"context"
	"net/http"
	"strings"

	mts "github.com/openmts/mts"
)

func (r *serverRuntime) handleAdminDatabases(writer http.ResponseWriter, request *http.Request) {
	if err := r.requireHTTPAdmin(request); err != nil {
		writeAPIError(writer, err)
		return
	}
	switch request.Method {
	case http.MethodGet:
		databases, err := r.engine.ListDatabases(request.Context())
		if err != nil {
			writeAPIError(writer, err)
			return
		}
		writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToMeasurements(measurementsResponse{
			Databases:    databases,
			Measurements: databases,
			Path:         routeAdminDatabases,
		}))
	case http.MethodPost:
		var req databaseRequest
		if err := decodeHTTPJSON(request, &req); err != nil {
			writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
			return
		}
		if err := r.engine.CreateDatabase(request.Context(), req.Name); err != nil {
			writeAPIError(writer, err)
			return
		}
		r.audit.record(auditEvent{UserName: r.auditUser(request), Action: "create_database", Database: req.Name})
		writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToOK(okResponse{OK: true, Path: routeAdminDatabases}))
	default:
		writeAPIError(writer, newAPIError(errorCodeBadRequest, messageMethodNotAllowed, nil))
	}
}

func (r *serverRuntime) handleAdminDatabaseResource(writer http.ResponseWriter, request *http.Request) {
	if err := r.requireHTTPAdmin(request); err != nil {
		writeAPIError(writer, err)
		return
	}
	parts := splitPath(request.URL.Path, routeAdminDatabasesPrefix)
	if len(parts) == 0 {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "database is required", nil))
		return
	}
	database := parts[0]
	if len(parts) == 1 {
		if request.Method != http.MethodDelete {
			writeAPIError(writer, newAPIError(errorCodeBadRequest, messageMethodNotAllowed, nil))
			return
		}
		if err := r.engine.DropDatabase(request.Context(), database); err != nil {
			writeAPIError(writer, err)
			return
		}
		r.audit.record(auditEvent{UserName: r.auditUser(request), Action: "drop_database", Database: database})
		writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToOK(okResponse{OK: true, Path: request.URL.Path}))
		return
	}
	if len(parts) == 2 && parts[1] == "retention-policies" {
		r.handleRetentionPolicies(writer, request, database)
		return
	}
	writeAPIError(writer, newAPIError(errorCodeNotFound, "metadata resource not found", nil))
}

func (r *serverRuntime) handleRetentionPolicies(
	writer http.ResponseWriter,
	request *http.Request,
	database string,
) {
	switch request.Method {
	case http.MethodPost:
		var req retentionPolicyRequest
		if err := decodeHTTPJSON(request, &req); err != nil {
			writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
			return
		}
		if err := r.engine.CreateRetentionPolicy(request.Context(), database, req.Policy); err != nil {
			writeAPIError(writer, err)
			return
		}
		r.audit.record(auditEvent{UserName: r.auditUser(request), Action: "create_retention_policy", Database: database, Detail: req.Policy.Name})
		writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToOK(okResponse{OK: true, Path: request.URL.Path}))
	case http.MethodGet:
		policies, err := r.engine.ListRetentionPolicies(request.Context(), database)
		if err != nil {
			writeAPIError(writer, err)
			return
		}
		writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToRetentionPolicies(retentionPoliciesResponse{
			Policies: policies,
			Path:     request.URL.Path,
			Database: database,
		}))
	default:
		writeAPIError(writer, newAPIError(errorCodeBadRequest, messageMethodNotAllowed, nil))
	}
}

// handleDataDatabases 返回当前用户可读的 database 列表（data 面，非 admin）。
func (r *serverRuntime) handleDataDatabases(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodGet) {
		return
	}
	if err := r.requireDataToken(httpCredentialSource{request: request}); err != nil {
		writeAPIError(writer, err)
		return
	}
	userName, err := r.authenticateDataUser(httpCredentialSource{request: request})
	if err != nil {
		writeAPIError(writer, err)
		return
	}
	databases, err := r.listReadableDatabases(request.Context(), userName)
	if err != nil {
		writeAPIError(writer, err)
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToMeasurements(measurementsResponse{
		Databases:    databases,
		Measurements: databases,
		Path:         routeDataDatabases,
	}))
}

func (r *serverRuntime) listReadableDatabases(ctx context.Context, userName string) ([]string, error) {
	databases, err := r.engine.ListDatabases(ctx)
	if err != nil {
		return nil, err
	}
	userName = strings.TrimSpace(userName)
	if userName == "" {
		// 与 authorizeDatabase 一致：无身份且未 RequireUser 时放行全量
		if r.currentConfig().Auth.RequireUser {
			return nil, newAPIError(errorCodeUnauthenticated, "user identity is required", nil)
		}
		return databases, nil
	}
	user, ok, err := r.engine.GetUser(ctx, userName)
	if err != nil {
		return nil, err
	}
	if !ok || user.Disabled {
		return nil, mts.ErrPermissionDenied
	}
	if user.Role == mts.UserRoleAdmin {
		return databases, nil
	}
	out := make([]string, 0, len(databases))
	for _, database := range databases {
		if err := r.engine.CheckUserDatabasePermission(ctx, userName, database, mts.DatabasePermissionRead); err != nil {
			continue
		}
		out = append(out, database)
	}
	return out, nil
}

func (r *serverRuntime) handleDataDatabase(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodGet) {
		return
	}
	parts := splitPath(request.URL.Path, routeDataDatabasesPrefix)
	if len(parts) < 2 {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "metadata path is incomplete", nil))
		return
	}
	database := parts[0]
	if err := r.authorizeHTTPDatabase(
		request.Context(),
		request,
		database,
		mts.DatabasePermissionRead,
	); err != nil {
		writeAPIError(writer, err)
		return
	}
	if len(parts) == 2 && parts[1] == "measurements" {
		measurements, err := r.engine.ListMeasurements(request.Context(), database)
		if err != nil {
			writeAPIError(writer, err)
			return
		}
		writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToMeasurements(measurementsResponse{
			Measurements: measurements,
			Path:         request.URL.Path,
			Database:     database,
		}))
		return
	}
	// data 面只读 RP：有 database read 权限即可，避免非 admin 只能手填
	if len(parts) == 2 && parts[1] == "retention-policies" {
		policies, err := r.engine.ListRetentionPolicies(request.Context(), database)
		if err != nil {
			writeAPIError(writer, err)
			return
		}
		writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToRetentionPolicies(retentionPoliciesResponse{
			Policies: policies,
			Path:     request.URL.Path,
			Database: database,
		}))
		return
	}
	if len(parts) >= 4 && parts[1] == "measurements" {
		measurement := parts[2]
		switch parts[3] {
		case "fields":
			fields, err := r.engine.ListFields(request.Context(), database, measurement)
			if err != nil {
				writeAPIError(writer, err)
				return
			}
			writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToFields(fieldsResponse{
				Fields:      fields,
				Path:        request.URL.Path,
				Database:    database,
				Measurement: measurement,
			}))
		case "series":
			series, err := r.engine.ListSeries(request.Context(), database, measurement, queryTags(request))
			if err != nil {
				writeAPIError(writer, err)
				return
			}
			resp := buildSeriesResponse(series, seriesPageOptions(request))
			resp.Path = request.URL.Path
			resp.Database = database
			resp.Measurement = measurement
			writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToSeries(resp))
		default:
			writeAPIError(writer, newAPIError(errorCodeNotFound, "metadata resource not found", nil))
		}
		return
	}
	writeAPIError(writer, newAPIError(errorCodeNotFound, "metadata resource not found", nil))
}
