package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthProbeResponsesIncludePath(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	t.Cleanup(server.Close)

	for _, path := range []string{routeHealth, routeReady} {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatalf("GET %s error = %v", path, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			t.Fatalf("read %s body: %v", path, readErr)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", path, resp.StatusCode, body)
		}
		var payload healthProbeResponse
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode %s: %v body=%s", path, err, body)
		}
		if payload.Path != path {
			t.Fatalf("%s path = %q want %q", path, payload.Path, path)
		}
	}
}
