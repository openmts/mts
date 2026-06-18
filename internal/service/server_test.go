package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codeberg.org/mts/mts/internal/observability"
)

type fakeOps struct {
	registry *observability.Registry
	health   Health
	compacts int
}

type fakeAuditLogger struct {
	events []AdminAuditEvent
}

func (f *fakeOps) MetricsSnapshot() []observability.Metric {
	return f.registry.Snapshot()
}

func (f *fakeOps) HealthSnapshot() Health {
	return f.health
}

func (f *fakeAuditLogger) LogAdminAction(event AdminAuditEvent) {
	f.events = append(f.events, event)
}

func TestServerHandlers(t *testing.T) {
	registry := observability.NewRegistry()
	registry.SetGauge("mts_ready", "Ready.", 1)
	ops := &fakeOps{
		registry: registry,
		health:   Health{Healthy: true, Ready: true, Reasons: []string{}},
	}
	server := NewServer(Options{AdminTimeout: time.Second}, ops, ops, func(context.Context) error {
		ops.compacts++
		return nil
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	server.server.Handler.ServeHTTP(recorder, request)
	if !strings.Contains(recorder.Body.String(), "mts_ready 1") {
		t.Fatalf("metrics body = %q, want mts_ready", recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	server.server.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("ready status = %d, want 200", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/admin/compact", nil)
	server.server.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || ops.compacts != 1 {
		t.Fatalf("compact status=%d count=%d, want 200 and 1", recorder.Code, ops.compacts)
	}
}

func TestServerHandlersCoverErrorPathsAndPprof(t *testing.T) {
	ops := &fakeOps{
		registry: observability.NewRegistry(),
		health:   Health{Healthy: true, Ready: false, Reasons: []string{"compaction backlog"}},
	}
	server := NewServer(Options{AdminTimeout: time.Second, EnablePprof: true}, nil, ops, func(context.Context) error {
		return errors.New("compact failed")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	server.server.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("nil metrics status = %d, want 200", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	server.server.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d, want 503", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/admin/compact", nil)
	server.server.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("compact get status = %d, want 405", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/admin/compact", nil)
	server.server.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("compact failure status = %d, want 500", recorder.Code)
	}

	nilCompactServer := NewServer(Options{AdminTimeout: time.Second}, nil, nil, nil)
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/admin/compact", nil)
	nilCompactServer.server.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil compact status = %d, want 503", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	server.server.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("pprof status = %d, want 200", recorder.Code)
	}
}

func TestReadyzReturnsStructuredChecks(t *testing.T) {
	ops := &fakeOps{
		registry: observability.NewRegistry(),
		health: Health{
			Healthy: true,
			Ready:   false,
			Reasons: []string{"memory hard limit exceeded"},
			Checks: []HealthCheck{
				{Name: "wal", Status: "ok"},
				{Name: "memory", Status: "failed", Reason: "hard limit exceeded"},
			},
		},
	}
	server := NewServer(Options{AdminTimeout: time.Second}, ops, ops, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	server.server.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d, want 503", recorder.Code)
	}
	var health Health
	if err := json.NewDecoder(recorder.Body).Decode(&health); err != nil {
		t.Fatalf("Decode ready response error = %v", err)
	}
	if len(health.Checks) != 2 || health.Checks[1].Name != "memory" || health.Checks[1].Status != "failed" {
		t.Fatalf("ready checks = %#v, want structured memory failure", health.Checks)
	}
}

func TestAdminCompactRequiresTimeoutAndReturnsTaskStatus(t *testing.T) {
	server := NewServer(Options{}, nil, nil, func(ctx context.Context) error {
		if _, ok := ctx.Deadline(); !ok {
			return errors.New("deadline missing")
		}
		return nil
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/compact", nil)
	server.server.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("compact without timeout status = %d, want 400", recorder.Code)
	}

	server = NewServer(Options{AdminTimeout: time.Second}, nil, nil, func(ctx context.Context) error {
		if _, ok := ctx.Deadline(); !ok {
			return errors.New("deadline missing")
		}
		return nil
	})
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/admin/compact", nil)
	server.server.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("compact status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	var response adminCompactResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Decode compact response error = %v", err)
	}
	if !response.OK || response.TaskID == "" || response.State != "succeeded" || response.DurationMillis < 0 {
		t.Fatalf("compact response = %#v, want task status", response)
	}
}

func TestAdminCompactWritesAuditLog(t *testing.T) {
	audit := &fakeAuditLogger{}
	server := NewServer(
		Options{AdminTimeout: time.Second, AuditLogger: audit},
		nil,
		nil,
		func(context.Context) error { return errors.New("boom") },
	)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/compact", nil)
	server.server.Handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("compact status = %d, want 500", recorder.Code)
	}
	if len(audit.events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(audit.events))
	}
	event := audit.events[0]
	if event.Action != "compact" || event.TaskID == "" || event.State != "failed" || event.Error != "boom" {
		t.Fatalf("audit event = %#v, want failed compact event", event)
	}
}

func TestServerStartAndShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("Close(listener) error = %v", err)
	}

	server := NewServer(Options{Addr: addr}, nil, nil, nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	client := http.Client{Timeout: time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		shutdownErr := server.Shutdown(context.Background())
		t.Fatalf("GET /healthz error = %v shutdown = %v", err, shutdownErr)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	if err := resp.Body.Close(); err != nil {
		shutdownErr := server.Shutdown(context.Background())
		t.Fatalf("Body.Close() error = %v shutdown = %v", err, shutdownErr)
	}
	if resp.StatusCode != http.StatusOK {
		shutdownErr := server.Shutdown(context.Background())
		t.Fatalf("health status = %d, want 200 shutdown = %v", resp.StatusCode, shutdownErr)
	}
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}
