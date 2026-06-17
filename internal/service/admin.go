package service

import (
	"context"
	"net/http"
	"time"
)

type adminResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func compactHandler(timeout time.Duration, compact CompactFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeJSON(writer, http.StatusMethodNotAllowed, adminResponse{OK: false, Error: "method not allowed"})
			return
		}
		if compact == nil {
			writeJSON(writer, http.StatusServiceUnavailable, adminResponse{OK: false, Error: "compact unavailable"})
			return
		}
		ctx := request.Context()
		cancel := func() {}
		if timeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, timeout)
		}
		defer cancel()
		if err := compact(ctx); err != nil {
			writeJSON(writer, http.StatusInternalServerError, adminResponse{OK: false, Error: err.Error()})
			return
		}
		writeJSON(writer, http.StatusOK, adminResponse{OK: true})
	}
}
