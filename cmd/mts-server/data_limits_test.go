package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPDataLimitsReportsConfiguredCaps(t *testing.T) {
	runtime := openTestRuntime(t)
	runtime.config.Limits.MaxWritePoints = 123
	runtime.config.Limits.DefaultQueryLimit = 456
	runtime.config.Limits.MaxQueryLimit = 789
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	var resp dataLimitsResponse
	getJSONWithHeaders(t, server.URL+routeDataLimits, nil, http.StatusOK, &resp)
	if resp.MaxWritePoints != 123 || resp.DefaultQueryLimit != 456 || resp.MaxQueryLimit != 789 {
		t.Fatalf("limits = %+v", resp)
	}
	if resp.Path != routeDataLimits {
		t.Fatalf("path = %q", resp.Path)
	}
}
