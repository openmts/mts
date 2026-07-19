package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminVersionHTTP(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	var unauthorized errorResponse
	getJSONWithHeaders(t, server.URL+routeAdminVersion, nil, http.StatusUnauthorized, &unauthorized)

	var body versionResponse
	headers := map[string]string{"Authorization": "Bearer test-admin-token"}
	getJSONWithHeaders(t, server.URL+routeAdminVersion, headers, http.StatusOK, &body)
	if body.Version == "" {
		t.Fatalf("version empty: %+v", body)
	}
	if body.Commit == "" {
		t.Fatalf("commit empty: %+v", body)
	}
	if body.BuiltAt == "" {
		t.Fatalf("built_at empty: %+v", body)
	}
}
