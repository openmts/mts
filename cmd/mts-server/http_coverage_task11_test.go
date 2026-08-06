package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mts "github.com/openmts/mts"
	"github.com/openmts/mts/internal/storagecheck"
)

func TestHTTPErrorContractsTask11(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	server := httptest.NewServer(runtime.httpHandler())
	t.Cleanup(server.Close)
	headers := map[string]string{"Authorization": "Bearer test-admin-token"}

	cases := []struct {
		method string
		path   string
		body   any
		status int
	}{
		{method: http.MethodPost, path: routeDataLimits, body: emptyRequest{}, status: http.StatusMethodNotAllowed},
		{method: http.MethodPost, path: routeDataContract, body: emptyRequest{}, status: http.StatusMethodNotAllowed},
		{method: http.MethodGet, path: routeDataWritePointsTyped, body: emptyRequest{}, status: http.StatusMethodNotAllowed},
		{method: http.MethodGet, path: routeDataDelete, body: emptyRequest{}, status: http.StatusMethodNotAllowed},
		{method: http.MethodPatch, path: routeAdminStorageSnapshots, body: emptyRequest{}, status: http.StatusBadRequest},
		{method: http.MethodDelete, path: routeAdminStorageSnapshots, body: emptyRequest{}, status: http.StatusBadRequest},
		{method: http.MethodDelete, path: routeAdminStorageSnapshots + "?name=missing.json", body: emptyRequest{}, status: http.StatusBadRequest},
		{method: http.MethodPost, path: routeAdminStorageDataSnapshot, body: "{", status: http.StatusBadRequest},
		{method: http.MethodPost, path: routeAdminStorageRestoreDrill, body: "{", status: http.StatusBadRequest},
		{method: http.MethodPost, path: routeUsersBatchDisabled, body: batchUserDisabledRequest{}, status: http.StatusBadRequest},
		{method: http.MethodPost, path: routeAdminDownsampleBatch, body: batchDownsampleRequest{Names: []string{"x"}, Action: "pause"}, status: http.StatusBadRequest},
		{method: http.MethodPost, path: routeDataQueryStream, body: queryStreamRequest{Format: "invalid"}, status: http.StatusUnauthorized},
		{method: http.MethodGet, path: routeDataDatabasesPrefix + "incomplete", body: emptyRequest{}, status: http.StatusBadRequest},
	}
	for _, item := range cases {
		resp := doHTTP(t, item.method, server.URL+item.path, item.body, headers)
		closeHTTPResponse(t, resp, item.status)
	}
	resp := doHTTP(t, http.MethodPost, server.URL+routeDataQueryStream, queryStreamRequest{Format: "invalid"}, nil)
	closeHTTPResponse(t, resp, http.StatusBadRequest)
}

func TestHTTPBatchDownsampleStreamTask11(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	policy := testDownsamplePolicy()
	policy.Name = "task11-stream-policy"
	if err := runtime.engine.CreateDownsamplePolicy(context.Background(), policy); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	body, err := json.Marshal(batchDownsampleRequest{Names: []string{policy.Name, "missing"}, Action: "disable"})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, routeAdminDownsampleBatch+"?stream=1", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-admin-token")
	req.Header.Set("Accept", contentTypeNDJSON)
	recorder := httptest.NewRecorder()
	runtime.httpHandler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"type":"summary"`) {
		t.Fatalf("stream response status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestHTTPPprofDisabledReturnsNotFoundTask11(t *testing.T) {
	runtime := openTestRuntime(t)
	req := httptest.NewRequest(http.MethodGet, routePprofPrefix, nil)
	recorder := httptest.NewRecorder()

	runtime.httpHandler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("pprof status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestStorageAndMiddlewareHelpersTask11(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"data-snapshot-1", "data-snapshot-2", "restore-drill-3", "other"} {
		if err := os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatalf("Mkdir(%s) error = %v", name, err)
		}
	}
	latest, err := latestDataSnapshotPath(root)
	if err != nil || filepath.Base(latest) != "data-snapshot-2" {
		t.Fatalf("latestDataSnapshotPath() = %q, %v", latest, err)
	}
	empty := t.TempDir()
	if _, err := latestDataSnapshotPath(empty); err == nil {
		t.Fatal("latestDataSnapshotPath(empty) error = nil")
	}
	missing := filepath.Join(t.TempDir(), "missing")
	if _, err := latestDataSnapshotPath(missing); err == nil {
		t.Fatal("latestDataSnapshotPath(missing) error = nil")
	}

	base := httptest.NewRecorder()
	recorder := &statusRecorder{ResponseWriter: base}
	if recorder.Unwrap() != base {
		t.Fatal("Unwrap() did not return underlying writer")
	}

	report := storagecheck.Report{Issues: []storagecheck.Issue{
		{Severity: storagecheck.SeverityFatal},
		{Severity: storagecheck.SeverityWarn},
		{Severity: storagecheck.SeverityFatal},
	}}
	if countRestoreDrillFatals(report) != 2 {
		t.Fatalf("countRestoreDrillFatals() = %d, want 2", countRestoreDrillFatals(report))
	}
	values := []struct {
		value mts.FieldValue
		want  string
	}{
		{value: mts.StringValue("x"), want: "x"},
		{value: mts.Int64Value(2), want: "2"},
		{value: mts.Float64Value(2.5), want: "2.5"},
		{value: mts.BoolValue(true), want: "true"},
		{value: mts.BoolValue(false), want: "false"},
		{value: mts.FieldValue{}, want: ""},
	}
	for _, item := range values {
		if got := fieldString(item.value); got != item.want {
			t.Fatalf("fieldString(%#v) = %q, want %q", item.value, got, item.want)
		}
	}
}

func TestWriteScopeHelpersTask11(t *testing.T) {
	first := testPoint()
	first.Database = "one"
	first.RetentionPolicy = "rp"
	second := testPoint()
	second.Database = "two"
	second.RetentionPolicy = "other"
	if got := writePrimaryDatabase(writeRequest{Points: []mts.Point{first}}); got != "one" {
		t.Fatalf("writePrimaryDatabase(single) = %q", got)
	}
	if got := writePrimaryDatabase(writeRequest{Points: []mts.Point{first, second}}); got != "" {
		t.Fatalf("writePrimaryDatabase(multiple) = %q", got)
	}
	if got := writePrimaryRetention(writeRequest{}); got != "" {
		t.Fatalf("writePrimaryRetention(empty) = %q", got)
	}
	if got := writePrimaryRetention(writeRequest{Points: []mts.Point{first}}); got != "rp" {
		t.Fatalf("writePrimaryRetention(single) = %q", got)
	}
	if got := writePrimaryRetention(writeRequest{Points: []mts.Point{first, second}}); got != "" {
		t.Fatalf("writePrimaryRetention(mixed) = %q", got)
	}
}
