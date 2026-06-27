package main

import (
	"encoding/json"
	"net/http"

	mts "github.com/openmts/mts"
)

func (r *serverRuntime) handleWrite(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodPost) {
		return
	}
	var req writeRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	for _, db := range writeRequestDatabases(req) {
		dbName := db
		if dbName == "__default__" {
			dbName = ""
		}
		if err := r.authorizeHTTPDatabase(request.Context(), request, dbName, mts.DatabasePermissionWrite); err != nil {
			writeAPIError(writer, err)
			return
		}
	}
	if err := r.write(request.Context(), req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	writeHTTPJSON(writer, http.StatusOK, writeResponse{OK: true})
}

func (r *serverRuntime) handleWriteTyped(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodPost) {
		return
	}
	var req typedWriteRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	if err := r.authorizeHTTPDatabase(
		request.Context(),
		request,
		req.Batch.Database,
		mts.DatabasePermissionWrite,
	); err != nil {
		writeAPIError(writer, err)
		return
	}
	if err := r.writeTypedBatch(request.Context(), req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	writeHTTPJSON(writer, http.StatusOK, writeResponse{OK: true})
}

func (r *serverRuntime) handleQueryRows(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodPost) {
		return
	}
	req, ok := r.decodeAuthorizedQuery(writer, request)
	if !ok {
		return
	}
	rows, err := r.queryRows(request.Context(), req)
	if err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	writeHTTPJSON(writer, http.StatusOK, queryRowsResponse{Rows: rows})
}

func (r *serverRuntime) handleQueryColumns(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodPost) {
		return
	}
	req, ok := r.decodeAuthorizedQuery(writer, request)
	if !ok {
		return
	}
	columns, err := r.queryColumns(request.Context(), req)
	if err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	writeHTTPJSON(writer, http.StatusOK, queryColumnsResponse{Columns: columns})
}

func (r *serverRuntime) handleQueryExplain(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodPost) {
		return
	}
	req, ok := r.decodeAuthorizedQuery(writer, request)
	if !ok {
		return
	}
	result, err := r.queryWithExplain(request.Context(), req)
	if err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	writeHTTPJSON(writer, http.StatusOK, queryExplainResponse{Result: result})
}

func (r *serverRuntime) handleQueryStats(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodGet) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, queryStatsResponse{Stats: r.queryStats()})
}

func (r *serverRuntime) handleQueryStream(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodPost) {
		return
	}
	req, ok := r.decodeAuthorizedQuery(writer, request)
	if !ok {
		return
	}
	rows, err := r.engine.QueryRowIterator(request.Context(), req.Query)
	if err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	defer func() {
		_ = rows.Close()
	}()
	writer.Header().Set("Content-Type", contentTypeNDJSON)
	writer.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(writer)
	for rows.Next() {
		row := rows.Row()
		if err := encoder.Encode(streamRecord{Type: streamTypeRow, Row: &row}); err != nil {
			return
		}
	}
	if err := rows.Err(); err != nil {
		_ = encoder.Encode(streamRecord{Type: streamTypeError, Error: errorPayload(err)})
		return
	}
	stats := r.queryStats()
	_ = encoder.Encode(streamRecord{Type: streamTypeEnd, Stats: &stats})
}

func (r *serverRuntime) decodeAuthorizedQuery(
	writer http.ResponseWriter,
	request *http.Request,
) (queryRequest, bool) {
	var req queryRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return queryRequest{}, false
	}
	if err := r.authorizeHTTPDatabase(
		request.Context(),
		request,
		req.Query.Database,
		mts.DatabasePermissionRead,
	); err != nil {
		writeAPIError(writer, err)
		return queryRequest{}, false
	}
	return req, true
}

func writeRequestDatabases(req writeRequest) []string {
	seen := make(map[string]struct{})
	var dbs []string
	for _, point := range req.Points {
		db := point.Database
		if db == "" {
			db = "__default__"
		}
		if _, ok := seen[db]; !ok {
			seen[db] = struct{}{}
			dbs = append(dbs, db)
		}
	}
	return dbs
}

func errorPayload(err error) *errorResponse {
	_, response := apiErrorResponse(err)
	return &response
}
