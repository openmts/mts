package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mts "github.com/openmts/mts"
)

func TestHTTPWriteAndQueryRows(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/write", writeRequest{
		Points: []mts.Point{testPoint()},
		Options: mts.WriteOptions{
			Sync: true,
		},
	}, http.StatusOK, &writeResponse{})

	var response queryRowsResponse
	postJSON(t, server.URL+"/api/v1/query/rows", queryRowsRequest{
		Query: testQuery(),
	}, http.StatusOK, &response)
	if len(response.Rows) != 1 || response.Rows[0].Fields["usage"].Float64 != 0.7 {
		t.Fatalf("rows = %#v, want one usage row", response.Rows)
	}
}

func TestHTTPHealthAndBadRequest(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("Get(healthz) error = %v", err)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("Close(healthz body) error = %v", closeErr)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want 200", resp.StatusCode)
	}

	postJSON(t, server.URL+"/api/v1/write", map[string]any{"bad": true}, http.StatusBadRequest, &errorResponse{})
	resp, err = http.Post(server.URL+"/healthz", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("Post(healthz) error = %v", err)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("Close(healthz post body) error = %v", closeErr)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST health status = %d, want 405", resp.StatusCode)
	}
}

func TestHTTPWriteAndQueryErrors(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/write", writeRequest{
		Points: []mts.Point{{Measurement: " "}},
	}, http.StatusBadRequest, &errorResponse{})
	postRaw(t, server.URL+"/api/v1/query/rows", `{"query":`, http.StatusBadRequest)

	resp, err := http.Get(server.URL + "/api/v1/query/rows")
	if err != nil {
		t.Fatalf("Get(query rows) error = %v", err)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("Close(query rows body) error = %v", closeErr)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET query rows status = %d, want 405", resp.StatusCode)
	}
}

func TestHTTPMaintenanceEndpoints(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	var flushResp maintenanceResponse
	postJSON(t, server.URL+"/api/v1/flush", map[string]any{}, http.StatusOK, &flushResp)
	if !flushResp.OK {
		t.Fatal("flush OK = false, want true")
	}

	var compactResp maintenanceResponse
	postJSON(t, server.URL+"/api/v1/compact", map[string]any{}, http.StatusOK, &compactResp)
	if !compactResp.OK {
		t.Fatal("compact OK = false, want true")
	}

	resp, err := http.Get(server.URL + "/api/v1/flush")
	if err != nil {
		t.Fatalf("Get(flush) error = %v", err)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("Close(flush body) error = %v", closeErr)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET flush status = %d, want 405", resp.StatusCode)
	}

	resp, err = http.Get(server.URL + "/api/v1/compact")
	if err != nil {
		t.Fatalf("Get(compact) error = %v", err)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("Close(compact body) error = %v", closeErr)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET compact status = %d, want 405", resp.StatusCode)
	}
}

func TestHTTPMaintenanceContextErrors(t *testing.T) {
	runtime := openTestRuntime(t)
	for _, tt := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{name: "flush", handler: runtime.handleFlush},
		{name: "compact", handler: runtime.handleCompact},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{}`))).WithContext(ctx)
			response := httptest.NewRecorder()
			tt.handler(response, request)
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", response.Code)
			}
		})
	}
}

func openTestRuntime(t *testing.T) *serverRuntime {
	t.Helper()
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.HTTP.Addr = "127.0.0.1:0"
	cfg.GRPC.Addr = "127.0.0.1:0"
	runtime, err := openRuntime(context.Background(), cfg)
	if err != nil {
		t.Fatalf("openRuntime() error = %v", err)
	}
	t.Cleanup(func() {
		if err := runtime.shutdown(context.Background()); err != nil {
			t.Fatalf("shutdown runtime error = %v", err)
		}
	})
	return runtime
}

func testPoint() mts.Point {
	return mts.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "api-1"},
		Timestamp:   1,
		Fields: map[string]mts.FieldValue{
			"usage": mts.Float64Value(0.7),
		},
	}
}

func testQuery() mts.Query {
	query, err := mts.NewQuery().From("default", "autogen", "cpu").Select("usage").Build()
	if err != nil {
		panic(err)
	}
	return query
}

func postJSON(t *testing.T, url string, req any, wantStatus int, out any) {
	t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal(request) error = %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Post(%s) error = %v", url, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("Close(response body) error = %v", err)
		}
	}()
	if resp.StatusCode != wantStatus {
		t.Fatalf("Post(%s) status = %d, want %d", url, resp.StatusCode, wantStatus)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("Decode(response) error = %v", err)
	}
}

func postRaw(t *testing.T, url string, body string, wantStatus int) {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("Post(%s) error = %v", url, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("Close(response body) error = %v", err)
		}
	}()
	if resp.StatusCode != wantStatus {
		t.Fatalf("Post(%s) status = %d, want %d", url, resp.StatusCode, wantStatus)
	}
}
