package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	headerRequestID  = "X-Request-ID"
	contextRequestID = contextKey("request_id")
)

type contextKey string

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *serverRuntime) wrapHTTP(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		cfg := r.currentConfig()
		requestID := request.Header.Get(headerRequestID)
		if strings.TrimSpace(requestID) == "" {
			requestID = r.nextRequestID()
		}
		writer.Header().Set(headerRequestID, requestID)
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		ctx := context.WithValue(request.Context(), contextRequestID, requestID)
		if timeout := time.Duration(cfg.Limits.RequestTimeout); timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		request = request.WithContext(ctx)
		if cfg.Limits.MaxRequestBodyBytes > 0 && request.Body != nil {
			request.Body = http.MaxBytesReader(writer, request.Body, cfg.Limits.MaxRequestBodyBytes)
		}
		httpSem := r.httpSem
		if !r.acquireHTTP(writer, httpSem) {
			return
		}
		defer releaseHTTP(httpSem)
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: writer, status: http.StatusOK}
		handler.ServeHTTP(recorder, request)
		duration := time.Since(start)
		r.metrics.observe("http", httpRoute(request.URL.Path), strconv.Itoa(recorder.status), duration)
		if cfg.Observability.AccessLog {
			r.currentLogger().InfoContext(ctx, "http request",
				"request_id", requestID,
				"method", request.Method,
				"path", request.URL.Path,
				"status", recorder.status,
				"duration", duration.String(),
			)
		}
	})
}

func (r *serverRuntime) acquireHTTP(writer http.ResponseWriter, sem chan struct{}) bool {
	if sem == nil {
		return true
	}
	select {
	case sem <- struct{}{}:
		return true
	default:
		writeHTTPJSON(writer, http.StatusTooManyRequests, errorResponse{
			OK:      false,
			Code:    errorCodeResourceExhausted,
			Message: "too many concurrent http requests",
			Error:   "too many concurrent http requests",
		})
		return false
	}
}

func releaseHTTP(sem chan struct{}) {
	if sem != nil {
		<-sem
	}
}

func acquireGRPC(sem chan struct{}) bool {
	if sem == nil {
		return true
	}
	select {
	case sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func releaseGRPC(sem chan struct{}) {
	if sem != nil {
		<-sem
	}
}

func (r *serverRuntime) nextRequestID() string {
	return fmt.Sprintf("mts-%d", r.requestSeq.Add(1))
}

func requestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(contextRequestID).(string)
	return value
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func httpRoute(path string) string {
	if strings.TrimSpace(path) == "" {
		return "/"
	}
	return path
}
