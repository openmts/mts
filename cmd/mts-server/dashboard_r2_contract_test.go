package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	mts "github.com/openmts/mts"
)

func TestHTTPQueryRowsEmbedsStats(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/data/write", writeRequest{
		Points:  []mts.Point{testPoint()},
		Options: mts.WriteOptions{Sync: true},
	}, http.StatusOK, &writeResponse{})

	var response queryRowsResponse
	postJSON(t, server.URL+"/api/v1/data/query/rows", queryRowsRequest{
		Query: testQuery(),
	}, http.StatusOK, &response)
	if len(response.Rows) != 1 {
		t.Fatalf("rows = %#v, want 1", response.Rows)
	}
	if response.Stats.DurationNanos <= 0 && response.Stats.SamplesRead <= 0 && response.Stats.SamplesReturned <= 0 {
		t.Fatalf("expected non-zero query stats in response, got %#v", response.Stats)
	}
}

func TestHTTPLoginReturnsRole(t *testing.T) {
	runtime := openTestRuntime(t)
	runtime.config.Auth.RequireUser = true
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	seedUserWithPassword(t, runtime, mts.User{Name: "role-admin", Role: mts.UserRoleAdmin}, "secret12")
	seedUserWithPassword(t, runtime, mts.User{Name: "role-user", Role: mts.UserRoleUser}, "secret12")

	var adminLogin authTokenResponse
	postJSON(t, server.URL+"/api/v1/auth/login", loginRequest{
		UserName: "role-admin", Password: "secret12", TTLSeconds: 60,
	}, http.StatusOK, &adminLogin)
	if adminLogin.Token.Role != mts.UserRoleAdmin {
		t.Fatalf("admin role = %q, want admin", adminLogin.Token.Role)
	}

	var userLogin authTokenResponse
	postJSON(t, server.URL+"/api/v1/auth/login", loginRequest{
		UserName: "role-user", Password: "secret12", TTLSeconds: 60,
	}, http.StatusOK, &userLogin)
	if userLogin.Token.Role != mts.UserRoleUser {
		t.Fatalf("user role = %q, want user", userLogin.Token.Role)
	}
}
