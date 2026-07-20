package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func TestHTTPDeleteAcceptsSnakeCaseJSON(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/data/write", writeRequest{
		Points: []mts.Point{{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "align"},
			Timestamp:   1_700_000_000_000,
			Precision:   mts.PrecisionMillisecond,
			Fields:      map[string]mts.FieldValue{"usage": mts.Float64Value(0.5)},
		}},
		Options: mts.WriteOptions{Sync: true},
	}, http.StatusOK, &writeResponse{})

	raw := []byte(`{"request":{"measurement":"cpu","tags":{"host":"align"},"start_time":1700000000000,"end_time":1700000000000,"precision":"ms"}}`)
	resp, err := http.Post(server.URL+"/api/v1/data/delete", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("delete post error = %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", resp.StatusCode, body)
	}

	var rows queryRowsResponse
	postJSON(t, server.URL+"/api/v1/data/query/rows", queryRowsRequest{
		Query: mts.Query{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "align"},
			StartTime:   1_700_000_000_000,
			EndTime:     1_700_000_000_000,
			Precision:   mts.PrecisionMillisecond,
		},
	}, http.StatusOK, &rows)
	if len(rows.Rows) != 0 {
		t.Fatalf("rows after delete = %#v, want empty", rows.Rows)
	}
}

func TestHTTPListDatabasesReturnsDatabasesField(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/admin/databases", databaseRequest{Name: "metrics"}, http.StatusOK, &okResponse{})
	resp, err := http.Get(server.URL + "/api/v1/admin/databases")
	if err != nil {
		t.Fatalf("get databases error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error = %v", err)
	}
	dbs, _ := payload["databases"].([]any)
	meas, _ := payload["measurements"].([]any)
	if len(dbs) == 0 {
		t.Fatalf("databases field missing/empty: %#v", payload)
	}
	if len(meas) == 0 {
		t.Fatalf("measurements compat field missing/empty: %#v", payload)
	}
}

func TestHTTPDownsampleEnableDisableActions(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	policy := mts.DownsamplePolicy{
		Name:              "align-ds",
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
	postJSON(t, server.URL+"/api/v1/admin/downsample/policies", policy, http.StatusOK, &okResponse{})

	// pause/resume 风格名应失败；enable/disable 成功
	resp, err := http.Post(server.URL+"/api/v1/admin/downsample/policies/align-ds/pause", "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatalf("pause post error = %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("pause should not be accepted")
	}

	postJSON(t, server.URL+"/api/v1/admin/downsample/policies/align-ds/disable", emptyRequest{}, http.StatusOK, &okResponse{})
	postJSON(t, server.URL+"/api/v1/admin/downsample/policies/align-ds/enable", emptyRequest{}, http.StatusOK, &okResponse{})
}

func TestHTTPDataListRetentionPoliciesForReadUser(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/admin/databases", databaseRequest{Name: "rpdb"}, http.StatusOK, &okResponse{})
	postJSON(t, server.URL+"/api/v1/admin/databases/rpdb/retention-policies", retentionPolicyRequest{
		Policy: mts.RetentionPolicy{Name: "short", Duration: time.Hour},
	}, http.StatusOK, &okResponse{})

	seedUserWithPassword(t, runtime, mts.User{Name: "rp-reader", Role: mts.UserRoleUser}, "secret")
	if err := runtime.engine.GrantDatabasePermission(context.Background(), "rp-reader", "rpdb", mts.DatabasePermissionRead); err != nil {
		t.Fatalf("grant read: %v", err)
	}
	token := loginHTTPUser(t, server.URL, "rp-reader", "secret")
	headers := map[string]string{"Authorization": "Bearer " + token}

	// admin path should be forbidden for non-admin
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/databases/rpdb/retention-policies", headers, http.StatusForbidden, &errorResponse{})

	var policies retentionPoliciesResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/data/databases/rpdb/retention-policies", headers, http.StatusOK, &policies)
	names := make([]string, 0, len(policies.Policies))
	for _, p := range policies.Policies {
		names = append(names, p.Name)
	}
	if !containsString(names, "short") && !containsString(names, "autogen") {
		// at least one policy should exist; short was created
		t.Fatalf("data retention policies = %#v, want short (or autogen)", policies.Policies)
	}
	foundShort := false
	for _, p := range policies.Policies {
		if p.Name == "short" {
			foundShort = true
			break
		}
	}
	if !foundShort {
		t.Fatalf("data retention policies = %#v, want short", policies.Policies)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
