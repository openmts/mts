package main

import (
	"net/http"
)

func (r *serverRuntime) handleAdminDatabases(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodPost) {
		return
	}
	if err := r.requireHTTPAdmin(request); err != nil {
		writeAPIError(writer, err)
		return
	}
	var req databaseRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	if err := r.engine.CreateDatabase(request.Context(), req.Name); err != nil {
		writeAPIError(writer, err)
		return
	}
	writeHTTPJSON(writer, http.StatusOK, okResponse{OK: true})
}

func (r *serverRuntime) handleAdminDatabaseResource(writer http.ResponseWriter, request *http.Request) {
	if err := r.requireHTTPAdmin(request); err != nil {
		writeAPIError(writer, err)
		return
	}
	parts := splitPath(request.URL.Path, "/api/v1/admin/databases/")
	if len(parts) == 0 {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "database is required", nil))
		return
	}
	database := parts[0]
	if len(parts) == 1 {
		if request.Method != http.MethodDelete {
			writeAPIError(writer, newAPIError(errorCodeBadRequest, "method not allowed", nil))
			return
		}
		if err := r.engine.DropDatabase(request.Context(), database); err != nil {
			writeAPIError(writer, err)
			return
		}
		writeHTTPJSON(writer, http.StatusOK, okResponse{OK: true})
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
		writeHTTPJSON(writer, http.StatusOK, okResponse{OK: true})
	case http.MethodGet:
		policies, err := r.engine.ListRetentionPolicies(request.Context(), database)
		if err != nil {
			writeAPIError(writer, err)
			return
		}
		writeHTTPJSON(writer, http.StatusOK, retentionPoliciesResponse{Policies: policies})
	default:
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "method not allowed", nil))
	}
}

func (r *serverRuntime) handleDataDatabase(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodGet) {
		return
	}
	parts := splitPath(request.URL.Path, "/api/v1/data/databases/")
	if len(parts) < 2 {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, "metadata path is incomplete", nil))
		return
	}
	database := parts[0]
	if err := r.authorizeHTTPDatabase(
		request.Context(),
		request,
		database,
		"read",
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
		writeHTTPJSON(writer, http.StatusOK, measurementsResponse{Measurements: measurements})
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
			writeHTTPJSON(writer, http.StatusOK, fieldsResponse{Fields: fields})
		case "series":
			series, err := r.engine.ListSeries(request.Context(), database, measurement, queryTags(request))
			if err != nil {
				writeAPIError(writer, err)
				return
			}
			writeHTTPJSON(writer, http.StatusOK, seriesResponse{Series: series})
		default:
			writeAPIError(writer, newAPIError(errorCodeNotFound, "metadata resource not found", nil))
		}
		return
	}
	writeAPIError(writer, newAPIError(errorCodeNotFound, "metadata resource not found", nil))
}

func queryTags(request *http.Request) map[string]string {
	values := request.URL.Query()
	tags := make(map[string]string)
	for key, value := range values {
		if len(value) == 0 || key == "" {
			continue
		}
		tags[key] = value[0]
	}
	if len(tags) == 0 {
		return nil
	}
	return tags
}
