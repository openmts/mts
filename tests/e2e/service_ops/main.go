package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/openmts/mts/internal/observability"
	"github.com/openmts/mts/internal/service"
)

type ops struct {
	registry *observability.Registry
	compacts int
	audits   []service.AdminAuditEvent
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
	server := service.NewServer(service.Options{
		AdminTimeout: time.Second,
		EnableAdmin:  true,
		AdminToken:   "secret",
		EnablePprof:  true,
		AuditLogger:  operations,
	}, operations, operations, func(context.Context) error {
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
	if err := assertGET(handler, "/readyz", `"checks":[`); err != nil {
		return err
	}
	if err := assertGET(handler, "/debug/pprof/", "Types of profiles available"); err != nil {
		return err
	}
	response, err := assertCompact(handler)
	if err != nil {
		return err
	}
	if operations.compacts != 1 || len(operations.audits) != 1 {
		return fmt.Errorf("compact count=%d audits=%d, want 1 and 1", operations.compacts, len(operations.audits))
	}
	if response.TaskID == "" || operations.audits[0].TaskID != response.TaskID {
		return fmt.Errorf("compact task id response=%q audit=%q", response.TaskID, operations.audits[0].TaskID)
	}
	return nil
}

func (o *ops) MetricsSnapshot() []observability.Metric {
	return o.registry.Snapshot()
}

func (o *ops) HealthSnapshot() service.Health {
	return service.Health{
		Healthy: true,
		Ready:   true,
		Reasons: []string{},
		Checks: []service.HealthCheck{
			{Name: "wal", Status: "ok"},
			{Name: "manifest", Status: "ok"},
			{Name: "disk", Status: "ok"},
			{Name: "compaction", Status: "ok"},
			{Name: "memory", Status: "ok"},
			{Name: "maintenance", Status: "ok"},
		},
	}
}

func (o *ops) LogAdminAction(event service.AdminAuditEvent) {
	o.audits = append(o.audits, event)
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

func assertCompact(handler http.Handler) (serviceAdminCompactResponse, error) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/compact", nil)
	request.Header.Set("Authorization", "Bearer secret")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		return serviceAdminCompactResponse{}, fmt.Errorf("compact status=%d, want 200", recorder.Code)
	}
	var response serviceAdminCompactResponse
	if err := json.NewDecoder(recorder.Result().Body).Decode(&response); err != nil {
		return serviceAdminCompactResponse{}, fmt.Errorf("decode compact response: %w", err)
	}
	if err := recorder.Result().Body.Close(); err != nil {
		return serviceAdminCompactResponse{}, fmt.Errorf("close compact body: %w", err)
	}
	if !response.OK || response.State != "succeeded" || response.TaskID == "" {
		return serviceAdminCompactResponse{}, fmt.Errorf("compact response=%#v, want succeeded task", response)
	}
	return response, nil
}

type serviceAdminCompactResponse struct {
	OK             bool   `json:"ok"`
	TaskID         string `json:"task_id"`
	State          string `json:"state"`
	DurationMillis int64  `json:"duration_ms"`
	Error          string `json:"error"`
}

func serverHandler(server *service.Server) http.Handler {
	return server.HTTPHandler()
}
