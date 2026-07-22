package main

import (
	"errors"
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

func TestAdminVersionHTTPBusyAndLast(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()
	headers := map[string]string{"Authorization": "Bearer test-admin-token"}

	if err := runtime.tryBeginAdminHeavy("flush"); err != nil {
		t.Fatalf("begin: %v", err)
	}
	var busy versionResponse
	getJSONWithHeaders(t, server.URL+routeAdminVersion, headers, http.StatusOK, &busy)
	if busy.Version == "" || !busy.AdminOpBusy || busy.Op != "flush" || busy.StartedAtUnix <= 0 {
		t.Fatalf("version busy = %+v", busy)
	}
	runtime.finishAdminHeavy(errors.New("version probe fail"))
	var done versionResponse
	getJSONWithHeaders(t, server.URL+routeAdminVersion, headers, http.StatusOK, &done)
	if done.AdminOpBusy {
		t.Fatal("version want not busy after finish")
	}
	if done.Last == nil || done.Last.Op != "flush" || done.Last.OK || done.Last.Error != "version probe fail" {
		t.Fatalf("version last after fail = %+v", done.Last)
	}
}
