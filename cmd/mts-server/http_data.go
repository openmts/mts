package main

import (
	"encoding/json"
	"fmt"
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
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToWrite(writeResponse{
		OK:     true,
		Points: len(req.Points),
		Path:   routeDataWrite,
	}))
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
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToWrite(writeResponse{
		OK:     true,
		Points: len(req.Batch.Timestamps),
		Path:   routeDataWriteTyped,
	}))
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
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToQueryRows(queryRowsResponse{
		Rows:     rows,
		Stats:    r.queryStats(),
		RowCount: len(rows),
		Path:     routeDataQueryRows,
	}))
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
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToQueryColumns(queryColumnsResponse{
		Columns:     columns,
		Stats:       r.queryStats(),
		SeriesCount: len(columns),
		Path:        routeDataQueryColumns,
	}))
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
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToQueryExplain(queryExplainResponse{
		Result: result,
		Path:   routeDataQueryExplain,
	}))
}

func (r *serverRuntime) handleQueryStats(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodGet) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.queryStatsPayload())
}

func (r *serverRuntime) handleDataLimits(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodGet) {
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.dataLimitsPayload())
}

func (r *serverRuntime) handleQueryStream(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodPost) {
		return
	}
	var req queryStreamRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	if err := r.authorizeHTTPDatabase(
		request.Context(),
		request,
		req.Query.Database,
		mts.DatabasePermissionRead,
	); err != nil {
		writeAPIError(writer, err)
		return
	}
	format, err := normalizeStreamFormat(req.Format, req.Mode)
	if err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	query, err := r.limitedQuery(req.Query)
	if err != nil {
		writeAPIError(writer, err)
		return
	}
	switch format {
	case streamTypeColumn:
		columns, err := r.engine.QueryColumnIterator(request.Context(), query)
		if err != nil {
			writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
			return
		}
		writer.Header().Set("Content-Type", contentTypeNDJSON)
		writer.WriteHeader(http.StatusOK)
		r.streamOpenedColumns(json.NewEncoder(writer), columns)
	default:
		rows, err := r.engine.QueryRowIterator(request.Context(), query)
		if err != nil {
			writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
			return
		}
		writer.Header().Set("Content-Type", contentTypeNDJSON)
		writer.WriteHeader(http.StatusOK)
		r.streamOpenedRows(json.NewEncoder(writer), rows)
	}
}

func normalizeStreamFormat(format string, mode string) (string, error) {
	value := format
	if value == "" {
		value = mode
	}
	switch value {
	case "", streamTypeRow, "rows":
		return streamTypeRow, nil
	case streamTypeColumn, "columns":
		return streamTypeColumn, nil
	default:
		return "", fmt.Errorf("unsupported stream format %q", value)
	}
}

func (r *serverRuntime) streamOpenedRows(encoder *json.Encoder, rows mts.RowIterator) {
	defer func() { _ = rows.Close() }()
	count := 0
	for rows.Next() {
		row := rows.Row()
		if err := encoder.Encode(streamRecord{Type: streamTypeRow, Row: &row}); err != nil {
			return
		}
		count++
	}
	if err := rows.Err(); err != nil {
		_ = encoder.Encode(streamRecord{Type: streamTypeError, Error: errorPayload(err)})
		return
	}
	_ = encoder.Encode(r.streamEndRecord(streamTypeRow, count))
}

func (r *serverRuntime) streamOpenedColumns(encoder *json.Encoder, columns mts.ColumnIterator) {
	defer func() { _ = columns.Close() }()
	count := 0
	for columns.Next() {
		column := columns.Column()
		if err := encoder.Encode(streamRecord{Type: streamTypeColumn, Column: &column}); err != nil {
			return
		}
		count++
	}
	if err := columns.Err(); err != nil {
		_ = encoder.Encode(streamRecord{Type: streamTypeError, Error: errorPayload(err)})
		return
	}
	_ = encoder.Encode(r.streamEndRecord(streamTypeColumn, count))
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

func (r *serverRuntime) handleWritePointsTyped(writer http.ResponseWriter, request *http.Request) {
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
	if err := r.writePointsAsTyped(request.Context(), req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToWrite(writeResponse{
		OK:     true,
		Points: len(req.Points),
		Path:   routeDataWritePointsTyped,
	}))
}

func (r *serverRuntime) handleDelete(writer http.ResponseWriter, request *http.Request) {
	if !requireHTTPMethod(writer, request, http.MethodPost) {
		return
	}
	var req deleteRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	if err := r.authorizeHTTPDatabase(
		request.Context(),
		request,
		req.Request.Database,
		mts.DatabasePermissionWrite,
	); err != nil {
		writeAPIError(writer, err)
		return
	}
	if err := r.deleteData(request.Context(), req.Request); err != nil {
		writeAPIError(writer, newAPIError(errorCodeBadRequest, err.Error(), err))
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.attachAdminOpToDelete(deleteResponse{
		OK:          true,
		Path:        routeDataDelete,
		Database:    req.Request.Database,
		Measurement: req.Request.Measurement,
	}))
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
