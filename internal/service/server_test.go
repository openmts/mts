package service

import (
	"context"
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

func (f *fakeOps) MetricsSnapshot() []observability.Metric {
	return f.registry.Snapshot()
}

func (f *fakeOps) HealthSnapshot() Health {
	return f.health
}

func TestServerHandlers(t *testing.T) {
	registry := observability.NewRegistry()
	registry.SetGauge("mts_ready", "Ready.", 1)
	ops := &fakeOps{
		registry: registry,
		health:   Health{Healthy: true, Ready: true, Reasons: []string{}},
	}
	server := NewServer(Options{}, ops, ops, func(context.Context) error {
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
	server := NewServer(Options{EnablePprof: true}, nil, ops, func(context.Context) error {
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

	nilCompactServer := NewServer(Options{}, nil, nil, nil)
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
