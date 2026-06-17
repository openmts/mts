package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"

	"codeberg.org/mts/mts/internal/observability"
	"codeberg.org/mts/mts/internal/service"
)

type ops struct {
	registry *observability.Registry
	compacts int
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("service_ops failed: %v", err)
	}
	log.Print("service_ops passed")
}

func run() error {
	operations := &ops{registry: observability.NewRegistry()}
	operations.registry.SetGauge("mts_ready", "Ready.", 1)
	server := service.NewServer(service.Options{}, operations, operations, func(context.Context) error {
		operations.compacts++
		return nil
	})
	handler := serverHandler(server)
	if err := assertGET(handler, "/metrics", "mts_ready 1"); err != nil {
		return err
	}
	if err := assertGET(handler, "/healthz", `"healthy":true`); err != nil {
		return err
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/compact", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || operations.compacts != 1 {
		return fmt.Errorf("compact status=%d count=%d, want 200 and 1", recorder.Code, operations.compacts)
	}
	return nil
}

func (o *ops) MetricsSnapshot() []observability.Metric {
	return o.registry.Snapshot()
}

func (o *ops) HealthSnapshot() service.Health {
	return service.Health{Healthy: true, Ready: true, Reasons: []string{}}
}

func assertGET(handler http.Handler, path string, contains string) error {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	handler.ServeHTTP(recorder, request)
	body, err := io.ReadAll(recorder.Result().Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if err := recorder.Result().Body.Close(); err != nil {
		return fmt.Errorf("close response body: %w", err)
	}
	if recorder.Code != http.StatusOK {
		return fmt.Errorf("%s status=%d, want 200", path, recorder.Code)
	}
	if !strings.Contains(string(body), contains) {
		return fmt.Errorf("%s body=%q, want %q", path, string(body), contains)
	}
	return nil
}

func serverHandler(server *service.Server) http.Handler {
	return server.HTTPHandler()
}
