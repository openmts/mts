package main

import (
	"encoding/json"
	"net/http"
)

func (r *serverRuntime) httpHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", r.handleHealth)
	mux.HandleFunc("/readyz", r.handleHealth)
	mux.HandleFunc("/api/v1/write", r.handleWrite)
	mux.HandleFunc("/api/v1/query/rows", r.handleQueryRows)
	mux.HandleFunc("/api/v1/flush", r.handleFlush)
	mux.HandleFunc("/api/v1/compact", r.handleCompact)
	return mux
}

func (r *serverRuntime) handleHealth(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeHTTPError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeHTTPJSON(writer, http.StatusOK, r.health())
}

func (r *serverRuntime) handleWrite(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeHTTPError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req writeRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeHTTPError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err := r.write(request.Context(), req); err != nil {
		writeHTTPError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeHTTPJSON(writer, http.StatusOK, writeResponse{OK: true})
}

func (r *serverRuntime) handleQueryRows(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeHTTPError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req queryRowsRequest
	if err := decodeHTTPJSON(request, &req); err != nil {
		writeHTTPError(writer, http.StatusBadRequest, err.Error())
		return
	}
	rows, err := r.queryRows(request.Context(), req)
	if err != nil {
		writeHTTPError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeHTTPJSON(writer, http.StatusOK, queryRowsResponse{Rows: rows})
}

func (r *serverRuntime) handleFlush(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeHTTPError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := r.flush(request.Context()); err != nil {
		writeHTTPError(writer, http.StatusInternalServerError, err.Error())
		return
	}
	writeHTTPJSON(writer, http.StatusOK, maintenanceResponse{OK: true})
}

func (r *serverRuntime) handleCompact(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeHTTPError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	result, err := r.compact(request.Context())
	if err != nil {
		writeHTTPError(writer, http.StatusInternalServerError, err.Error())
		return
	}
	writeHTTPJSON(writer, http.StatusOK, maintenanceResponse{OK: true, Result: result})
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

func writeHTTPError(writer http.ResponseWriter, status int, message string) {
	writeHTTPJSON(writer, status, errorResponse{Error: message})
}
