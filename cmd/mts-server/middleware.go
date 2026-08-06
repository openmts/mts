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

// Flush 透传底层 Flusher，保证 NDJSON 批处理进度可边写边推。
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap 供 http.ResponseController 等识别底层 ResponseWriter。
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// applySecurityHeaders 设置可商用后台默认安全响应头（API + Dashboard 静态资源共用）。
// enableHSTS 仅在确认 TLS 终止（本机 TLS 或受信边缘 HTTPS）时启用。
func applySecurityHeaders(header http.Header, enableHSTS bool) {
	if header == nil {
		return
	}
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	header.Set("Cross-Origin-Opener-Policy", "same-origin")
	// SPA 产物为同域静态资源；style 可能含构建期注入，保留 self。
	header.Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; form-action 'self'")
	if enableHSTS {
		header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
}

func (r *serverRuntime) wrapHTTP(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		cfg := r.currentConfig()
		requestID := request.Header.Get(headerRequestID)
		if strings.TrimSpace(requestID) == "" {
			requestID = r.nextRequestID()
		}
		writer.Header().Set(headerRequestID, requestID)
		applySecurityHeaders(writer.Header(), cfg.HTTP.TLS.Enabled)
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
		if !r.acquireHTTP(writer) {
			return
		}
		defer r.httpLimiter.release()
		// 默认 bootstrap 密码强制改密：对已认证用户拦截业务 API。
		if strings.HasPrefix(request.URL.Path, "/api/") {
			if userName, authErr := r.authenticateDataUser(httpCredentialSource{request: request}); authErr == nil {
				if gateErr := r.enforcePasswordChangeGate(request.Context(), userName, request.URL.Path); gateErr != nil {
					writeAPIError(writer, gateErr)
					return
				}
			}
		}
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

func (r *serverRuntime) acquireHTTP(writer http.ResponseWriter) bool {
	if r.httpLimiter.tryAcquire() {
		return true
	}
	writeHTTPJSON(writer, http.StatusTooManyRequests, errorResponse{
		OK:      false,
		Code:    errorCodeResourceExhausted,
		Message: "too many concurrent http requests",
		Error:   "too many concurrent http requests",
	})
	return false
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
