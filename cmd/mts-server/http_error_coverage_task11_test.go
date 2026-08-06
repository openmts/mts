package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func TestHTTPHandlersPropagateCancelledContext(t *testing.T) {
	runtime := openTestRuntime(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name    string
		method  string
		path    string
		body    any
		handler http.HandlerFunc
	}{
		{name: "points typed", method: http.MethodPost, path: routeDataWritePointsTyped, body: writeRequest{Points: []mts.Point{testPoint()}}, handler: runtime.handleWritePointsTyped},
		{name: "delete", method: http.MethodPost, path: routeDataDelete, body: deleteRequest{Request: mts.DeleteRequest{Measurement: "cpu"}}, handler: runtime.handleDelete},
		{name: "query stream", method: http.MethodPost, path: routeDataQueryStream, body: queryStreamRequest{Query: testQuery()}, handler: runtime.handleQueryStream},
		{name: "data databases", method: http.MethodGet, path: routeDataDatabases, body: nil, handler: runtime.handleDataDatabases},
		{name: "measurements", method: http.MethodGet, path: routeDataDatabasesPrefix + "default/measurements", body: nil, handler: runtime.handleDataDatabase},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			req := newCancelledRequest(t, ctx, item.method, item.path, item.body)
			resp := httptest.NewRecorder()
			item.handler(resp, req)
			if resp.Code < http.StatusBadRequest {
				t.Fatalf("status = %d, want error; body=%q", resp.Code, resp.Body.String())
			}
		})
	}
}

func TestHTTPAdminAndUserErrorBranchesTask11(t *testing.T) {
	runtime := openTestRuntime(t)

	passwordCases := []struct {
		name string
		body any
	}{
		{name: "invalid json", body: "{"},
		{name: "short password", body: passwordRequest{Password: "x"}},
		{name: "missing user", body: passwordRequest{Password: "secret12"}},
	}
	for _, item := range passwordCases {
		t.Run(item.name, func(t *testing.T) {
			req := requestWithBody(t, http.MethodPut, routeUsersPrefix+"missing/password", item.body)
			resp := httptest.NewRecorder()
			runtime.handleUserPassword(resp, req, "missing")
			if resp.Code < http.StatusBadRequest {
				t.Fatalf("status = %d, want error", resp.Code)
			}
		})
	}

	if err := os.Chmod(runtime.currentConfig().DataDir, 0000); err != nil {
		t.Fatalf("Chmod(data dir) error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(runtime.currentConfig().DataDir, 0700); err != nil {
			t.Errorf("restore data dir mode error = %v", err)
		}
	})

	backup := filepath.Join(t.TempDir(), "backup-file")
	if err := os.WriteFile(backup, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile(backup) error = %v", err)
	}
	runtime.config.Backup.Dir = backup
	for _, handler := range []http.HandlerFunc{runtime.handleListStorageSnapshots, runtime.handleListStorageDataSnapshots} {
		req := httptest.NewRequest(http.MethodGet, routeAdminStorageSnapshots, nil)
		resp := httptest.NewRecorder()
		handler(resp, req)
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("snapshot list status = %d, want 500; body=%q", resp.Code, resp.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, routeAdminAudit+"?since_unix=1&until_unix=2&limit=3", nil)
	resp := httptest.NewRecorder()
	runtime.handleListAudit(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("handleListAudit() status = %d", resp.Code)
	}
}

func TestOptionalJSONAndGRPCMarkerTask11(t *testing.T) {
	var target storageDataSnapshotRequest
	if err := decodeOptionalHTTPJSON(httptest.NewRequest(http.MethodPost, "/", nil), &target); err != nil {
		t.Fatalf("decodeOptionalHTTPJSON(nil) error = %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(nil))
	req.ContentLength = 0
	if err := decodeOptionalHTTPJSON(req, &target); err != nil {
		t.Fatalf("decodeOptionalHTTPJSON(empty) error = %v", err)
	}

	service := &grpcService{}
	service.mtsServer()
	if _, ok := errorCodeMetaByCode(errorCode("unknown")); ok {
		t.Fatal("errorCodeMetaByCode(unknown) ok = true")
	}
	if got := normalizeDashboardBase("dashboard"); got != "/dashboard/" {
		t.Fatalf("normalizeDashboardBase() = %q", got)
	}
	if got := normalizeDashboardBase("/dashboard"); got != "/dashboard/" {
		t.Fatalf("normalizeDashboardBase(slash) = %q", got)
	}
	if got, ok := stripDashboardBase("/anything", "/"); !ok || got != "/anything" {
		t.Fatalf("stripDashboardBase(root) = %q, %v", got, ok)
	}
	if got, ok := stripDashboardBase("/dashboard", "/dashboard/"); !ok || got != "/" {
		t.Fatalf("stripDashboardBase(exact) = %q, %v", got, ok)
	}
	if got, ok := stripDashboardBase("/dashboard/users", "/dashboard/"); !ok || got != "/users" {
		t.Fatalf("stripDashboardBase(child) = %q, %v", got, ok)
	}
	if got, ok := stripDashboardBase("/other", "/dashboard/"); ok || got != "" {
		t.Fatalf("stripDashboardBase(missing) = %q, %v", got, ok)
	}
	if got := remainingSecondsUntil(time.Now().Add(-time.Second)); got != 0 {
		t.Fatalf("remainingSecondsUntil(expired) = %d", got)
	}
	policy := applyDownsamplePolicyRequestDefaults(mts.DownsamplePolicy{
		SourceDatabase:    "default",
		SourceMeasurement: "cpu",
		Interval:          time.Minute,
		Lookback:          -time.Second,
	})
	if policy.Lookback != policy.Interval || policy.TargetMeasurement != "cpu_ds" || policy.BatchSize != 100 {
		t.Fatalf("applyDownsamplePolicyRequestDefaults() = %#v", policy)
	}
}

func newCancelledRequest(t *testing.T, ctx context.Context, method string, path string, body any) *http.Request {
	t.Helper()
	req := requestWithBody(t, method, path, body)
	return req.WithContext(ctx)
}

func requestWithBody(t *testing.T, method string, path string, body any) *http.Request {
	t.Helper()
	if body == nil {
		return httptest.NewRequest(method, path, nil)
	}
	if raw, ok := body.(string); ok {
		return httptest.NewRequest(method, path, bytes.NewBufferString(raw))
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal(request) error = %v", err)
	}
	return httptest.NewRequest(method, path, bytes.NewReader(data))
}
