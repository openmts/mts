package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func TestHTTPBatchUpdateUserDisabled(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	t.Cleanup(server.Close)

	seedUserWithPassword(t, runtime, mts.User{Name: "u1", Role: mts.UserRoleUser}, "secret")
	seedUserWithPassword(t, runtime, mts.User{Name: "u2", Role: mts.UserRoleUser}, "secret")
	adminToken := loginHTTPUser(t, server.URL, "admin", "admin")
	headers := map[string]string{"Authorization": "Bearer " + adminToken}

	var resp batchMutationResponse
	postJSONWithHeaders(t, server.URL+routeUsersBatchDisabled, batchUserDisabledRequest{
		Names:    []string{"u1", "u2", "missing", "admin"},
		Disabled: true,
	}, headers, http.StatusOK, &resp)
	if resp.OKCount != 2 {
		t.Fatalf("ok_count=%d want 2; resp=%#v", resp.OKCount, resp)
	}
	if resp.Skip < 2 {
		t.Fatalf("skip_count=%d want >=2; resp=%#v", resp.Skip, resp)
	}

	// 禁用自己应 skip
	self := batchMutationResponse{}
	postJSONWithHeaders(t, server.URL+routeUsersBatchDisabled, batchUserDisabledRequest{
		Names:    []string{"admin"},
		Disabled: true,
	}, headers, http.StatusOK, &self)
	if self.OKCount != 0 || self.Skip != 1 {
		t.Fatalf("self disable resp=%#v", self)
	}
}

func TestHTTPBatchDownsamplePolicies(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	t.Cleanup(server.Close)
	adminToken := loginHTTPUser(t, server.URL, "admin", "admin")
	headers := map[string]string{"Authorization": "Bearer " + adminToken}

	policy := mts.DownsamplePolicy{
		Name:              "batch_ds_a",
		SourceDatabase:    "default",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "default",
		TargetRetention:   "autogen",
		TargetMeasurement: "cpu_1m",
		Interval:          time.Minute,
		RefreshInterval:   time.Minute,
		Lookback:          time.Minute,
		BatchSize:         100,
		Functions:         []mts.DownsampleFunction{{Function: mts.AggregateAvg, Field: "usage", As: "mean_usage"}},
		Enabled:           true,
	}
	postJSONWithHeaders(t, server.URL+routeAdminDownsamplePolicies, policy, headers, http.StatusOK, &okResponse{})
	policy.Name = "batch_ds_b"
	postJSONWithHeaders(t, server.URL+routeAdminDownsamplePolicies, policy, headers, http.StatusOK, &okResponse{})

	var resp batchMutationResponse
	postJSONWithHeaders(t, server.URL+routeAdminDownsampleBatch, batchDownsampleRequest{
		Names:  []string{"batch_ds_a", "batch_ds_b", "missing_ds"},
		Action: "disable",
	}, headers, http.StatusOK, &resp)
	if resp.OKCount != 2 {
		t.Fatalf("ok_count=%d want 2; resp=%#v", resp.OKCount, resp)
	}
	if resp.Skip < 1 {
		t.Fatalf("skip_count=%d want >=1; resp=%#v", resp.Skip, resp)
	}

	// bad action
	postJSONWithHeaders(t, server.URL+routeAdminDownsampleBatch, batchDownsampleRequest{
		Names:  []string{"batch_ds_a"},
		Action: "pause",
	}, headers, http.StatusBadRequest, &errorResponse{})
}

func TestHTTPBatchUserDisabledStream(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	t.Cleanup(server.Close)

	seedUserWithPassword(t, runtime, mts.User{Name: "s1", Role: mts.UserRoleUser}, "secret")
	seedUserWithPassword(t, runtime, mts.User{Name: "s2", Role: mts.UserRoleUser}, "secret")
	adminToken := loginHTTPUser(t, server.URL, "admin", "admin")

	body := `{"names":["s1","s2","missing"],"disabled":true}`
	req, err := http.NewRequest(http.MethodPost, server.URL+routeUsersBatchDisabled+"?stream=1", strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", contentTypeNDJSON)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "ndjson") {
		t.Fatalf("content-type=%q", resp.Header.Get("Content-Type"))
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) < 3 {
		t.Fatalf("lines=%d body=%q", len(lines), raw)
	}
	var summary batchProgressEvent
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &summary); err != nil {
		t.Fatalf("summary json: %v body=%s", err, lines[len(lines)-1])
	}
	if summary.Type != "summary" {
		t.Fatalf("last type=%q want summary", summary.Type)
	}
	if summary.OKCount != 2 {
		t.Fatalf("ok_count=%d want 2 summary=%#v", summary.OKCount, summary)
	}
}
