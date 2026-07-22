package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func TestHTTPMetaAndDownsampleMutationsReportPath(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	t.Cleanup(server.Close)

	adminToken := loginHTTPUser(t, server.URL, "admin", "admin")
	headers := map[string]string{"Authorization": "Bearer " + adminToken}

	var createdDB okResponse
	postJSONWithHeaders(t, server.URL+routeAdminDatabases, databaseRequest{Name: "path_db"}, headers, http.StatusOK, &createdDB)
	if !createdDB.OK || createdDB.Path != routeAdminDatabases {
		t.Fatalf("create database = %+v", createdDB)
	}

	var createdRP okResponse
	postJSONWithHeaders(t, server.URL+routeAdminDatabasesPrefix+"path_db/retention-policies", retentionPolicyRequest{
		Policy: mts.RetentionPolicy{Name: "rp_path", Duration: time.Hour},
	}, headers, http.StatusOK, &createdRP)
	if !createdRP.OK || !strings.Contains(createdRP.Path, "/retention-policies") {
		t.Fatalf("create rp = %+v", createdRP)
	}

	policy := mts.DownsamplePolicy{
		Name:              "path_ds",
		SourceDatabase:    "path_db",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "path_db",
		TargetRetention:   "autogen",
		TargetMeasurement: "cpu_1m",
		Interval:          time.Minute,
		RefreshInterval:   time.Minute,
		Lookback:          time.Minute,
		BatchSize:         100,
		Functions:         []mts.DownsampleFunction{{Function: mts.AggregateAvg, Field: "usage", As: "mean_usage"}},
		Enabled:           true,
	}
	var createdDS okResponse
	postJSONWithHeaders(t, server.URL+routeAdminDownsamplePolicies, policy, headers, http.StatusOK, &createdDS)
	if !createdDS.OK || createdDS.Path != routeAdminDownsamplePolicies {
		t.Fatalf("create downsample = %+v", createdDS)
	}

	var listedDS downsamplePoliciesResponse
	getJSONWithHeaders(t, server.URL+routeAdminDownsamplePolicies, headers, http.StatusOK, &listedDS)
	if listedDS.Path != routeAdminDownsamplePolicies {
		t.Fatalf("list downsample path=%q", listedDS.Path)
	}

	var enabled okResponse
	postJSONWithHeaders(t, server.URL+routeAdminDownsamplePrefix+"path_ds/enable", emptyRequest{}, headers, http.StatusOK, &enabled)
	if !enabled.OK || !strings.HasSuffix(enabled.Path, "/enable") {
		t.Fatalf("enable = %+v", enabled)
	}

	var disabled okResponse
	postJSONWithHeaders(t, server.URL+routeAdminDownsamplePrefix+"path_ds/disable", emptyRequest{}, headers, http.StatusOK, &disabled)
	if !disabled.OK || !strings.HasSuffix(disabled.Path, "/disable") {
		t.Fatalf("disable = %+v", disabled)
	}

	var reset okResponse
	postJSONWithHeaders(t, server.URL+routeAdminDownsamplePrefix+"path_ds/reset", map[string]any{
		"reset": map[string]any{"allow_policy_replace": true},
	}, headers, http.StatusOK, &reset)
	if !reset.OK || !strings.HasSuffix(reset.Path, "/reset") {
		t.Fatalf("reset = %+v", reset)
	}

	var statuses downsampleStatusesResponse
	getJSONWithHeaders(t, server.URL+routeAdminDownsampleStatuses, headers, http.StatusOK, &statuses)
	if statuses.Path != routeAdminDownsampleStatuses {
		t.Fatalf("statuses path=%q", statuses.Path)
	}

	var droppedDS okResponse
	del := doHTTP(t, http.MethodDelete, server.URL+routeAdminDownsamplePrefix+"path_ds", emptyRequest{}, headers)
	defer func() { _ = del.Body.Close() }()
	if del.StatusCode != http.StatusOK {
		t.Fatalf("drop ds status=%d", del.StatusCode)
	}
	if err := json.NewDecoder(del.Body).Decode(&droppedDS); err != nil {
		t.Fatalf("decode drop ds: %v", err)
	}
	if !droppedDS.OK || droppedDS.Path != routeAdminDownsamplePrefix+"path_ds" {
		t.Fatalf("drop ds = %+v", droppedDS)
	}

	var droppedDB okResponse
	delDB := doHTTP(t, http.MethodDelete, server.URL+routeAdminDatabasesPrefix+"path_db", emptyRequest{}, headers)
	defer func() { _ = delDB.Body.Close() }()
	if delDB.StatusCode != http.StatusOK {
		t.Fatalf("drop db status=%d", delDB.StatusCode)
	}
	if err := json.NewDecoder(delDB.Body).Decode(&droppedDB); err != nil {
		t.Fatalf("decode drop db: %v", err)
	}
	if !droppedDB.OK || droppedDB.Path != routeAdminDatabasesPrefix+"path_db" {
		t.Fatalf("drop db = %+v", droppedDB)
	}
}
