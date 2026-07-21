package main

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func TestHTTPAuthSession(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	seedUserWithPassword(t, runtime, mts.User{Name: "session-user", Role: mts.UserRoleUser}, "secret")
	token := loginHTTPUser(t, server.URL, "session-user", "secret")
	headers := map[string]string{"Authorization": "Bearer " + token}

	var session sessionResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/auth/session", headers, http.StatusOK, &session)
	if !session.OK {
		t.Fatalf("session.ok = false")
	}
	if session.UserName != "session-user" {
		t.Fatalf("user_name = %q", session.UserName)
	}
	if session.Role != mts.UserRoleUser {
		t.Fatalf("role = %q", session.Role)
	}
	if session.ExpiresAt.IsZero() || session.ExpiresAt.Before(time.Now()) {
		t.Fatalf("expires_at = %v", session.ExpiresAt)
	}
	if session.RemainingSeconds <= 0 {
		t.Fatalf("remaining_seconds = %d", session.RemainingSeconds)
	}

	getJSONWithHeaders(t, server.URL+"/api/v1/auth/session", nil, http.StatusUnauthorized, &errorResponse{})
	getJSONWithHeaders(t, server.URL+"/api/v1/auth/session", map[string]string{
		"Authorization": "Bearer bad-token",
	}, http.StatusUnauthorized, &errorResponse{})
}

func TestDataSeriesOffsetAndQuery(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/admin/databases", databaseRequest{Name: "seriespage"}, http.StatusOK, &okResponse{})
	for i := 0; i < 5; i++ {
		host := "h" + strconv.Itoa(i)
		postJSON(t, server.URL+"/api/v1/data/write", writeRequest{Points: []mts.Point{{
			Database:    "seriespage",
			Measurement: "cpu",
			Tags:        map[string]string{"host": host, "zone": "z1"},
			Timestamp:   int64(10 + i),
			Fields:      map[string]mts.FieldValue{"usage": mts.Float64Value(float64(i))},
		}}}, http.StatusOK, &writeResponse{})
	}

	var page seriesResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/data/databases/seriespage/measurements/cpu/series?limit=2&offset=2", nil, http.StatusOK, &page)
	if page.Total < 5 {
		t.Fatalf("total = %d, want >= 5", page.Total)
	}
	if len(page.Series) != 2 {
		t.Fatalf("series len = %d, want 2", len(page.Series))
	}
	if page.Offset != 2 || page.Limit != 2 {
		t.Fatalf("offset/limit = %d/%d", page.Offset, page.Limit)
	}
	if !page.Truncated {
		t.Fatalf("expected truncated")
	}

	var filtered seriesResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/data/databases/seriespage/measurements/cpu/series?q=h1", nil, http.StatusOK, &filtered)
	if filtered.Total != 1 || len(filtered.Series) != 1 {
		t.Fatalf("q filter total=%d len=%d", filtered.Total, len(filtered.Series))
	}
	if filtered.Series[0].Tags["host"] != "h1" {
		t.Fatalf("tags = %#v", filtered.Series[0].Tags)
	}
}
