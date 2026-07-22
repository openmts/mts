package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	mts "github.com/openmts/mts"
)

func TestHTTPP0P1RemainingEndpoints(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/admin/databases", databaseRequest{Name: "metrics"}, http.StatusOK, &okResponse{})
	postJSON(t, server.URL+"/api/v1/admin/databases/metrics/retention-policies", retentionPolicyRequest{
		Policy: mts.RetentionPolicy{Name: "rp", Duration: time.Hour},
	}, http.StatusOK, &okResponse{})
	var policies retentionPoliciesResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/databases/metrics/retention-policies", nil, http.StatusOK, &policies)
	if len(policies.Policies) == 0 {
		t.Fatalf("retention policies = %#v, want non-empty", policies.Policies)
	}

	postJSON(t, server.URL+"/api/v1/data/write", writeRequest{Points: []mts.Point{{
		Database:    "metrics",
		Measurement: "mem",
		Tags:        map[string]string{"host": "h1"},
		Timestamp:   10,
		Fields:      map[string]mts.FieldValue{"used": mts.Float64Value(12)},
	}}}, http.StatusOK, &writeResponse{})
	var measurements measurementsResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/data/databases/metrics/measurements", nil, http.StatusOK, &measurements)
	if !slices.Contains(measurements.Measurements, "mem") {
		t.Fatalf("measurements = %#v, want to contain mem", measurements.Measurements)
	}
	var series seriesResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/data/databases/metrics/measurements/mem/series?host=h1", nil, http.StatusOK, &series)
	if len(series.Series) != 1 {
		t.Fatalf("series = %#v, want one", series.Series)
	}

	var schema configSchemaResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/config/schema", nil, http.StatusOK, &schema)
	if len(schema.Fields) == 0 {
		t.Fatal("config schema fields empty")
	}
	postJSON(t, server.URL+"/api/v1/admin/retention/apply", retentionApplyRequest{}, http.StatusOK, &okResponse{})
	var maintenance maintenanceErrorsResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/maintenance/errors", nil, http.StatusOK, &maintenance)
	var memory storageMemoryResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/stats/storage-memory", nil, http.StatusOK, &memory)
	var compaction compactionStatsResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/stats/compaction", nil, http.StatusOK, &compaction)
	var health mts.HealthSnapshot
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/health", nil, http.StatusOK, &health)
	if !health.Healthy {
		t.Fatalf("admin health = %#v, want healthy", health)
	}

	postJSON(t, server.URL+"/api/v1/users", mts.User{Name: "bob"}, http.StatusOK, &okResponse{})
	postJSON(t, server.URL+"/api/v1/users/bob/database-permissions/metrics/admin", emptyRequest{}, http.StatusOK, &okResponse{})
	var grants databasePermissionsResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/users/bob/database-permissions", nil, http.StatusOK, &grants)
	if len(grants.Grants) != 1 {
		t.Fatalf("grants = %#v, want one", grants.Grants)
	}
	var allowed authzDatabaseCheckResponse
	postJSON(t, server.URL+"/api/v1/authz/database/check", authzDatabaseCheckRequest{
		UserName: "bob", Database: "metrics", Permission: mts.DatabasePermissionRead,
	}, http.StatusOK, &allowed)
	if !allowed.Allowed {
		t.Fatal("authz allowed = false, want true")
	}
	putJSON(t, server.URL+"/api/v1/users/bob", mts.User{DisplayName: "Bob"}, http.StatusOK, &okResponse{})
	deleteHTTP(t, server.URL+"/api/v1/users/bob/database-permissions/metrics/admin", http.StatusOK)

	policy := testDownsamplePolicy()
	policy.Name = "coverage_rollup"
	policy.SourceDatabase = "metrics"
	policy.SourceMeasurement = "mem"
	policy.TargetDatabase = "metrics"
	postJSON(t, server.URL+"/api/v1/admin/downsample/policies", policy, http.StatusOK, &okResponse{})
	postJSON(t, server.URL+"/api/v1/admin/downsample/policies/coverage_rollup/disable", emptyRequest{}, http.StatusOK, &okResponse{})
	postJSON(t, server.URL+"/api/v1/admin/downsample/policies/coverage_rollup/enable", emptyRequest{}, http.StatusOK, &okResponse{})
	postJSON(t, server.URL+"/api/v1/admin/downsample/policies/coverage_rollup/reset", downsampleResetRequest{}, http.StatusOK, &okResponse{})
	var statuses downsampleStatusesResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/downsample/statuses", nil, http.StatusOK, &statuses)
	var run downsampleRunResponse
	postJSON(t, server.URL+"/api/v1/admin/downsample/policies/coverage_rollup/run-range", downsampleRangeRequest{StartUnix: 1, EndUnix: int64(time.Hour)}, http.StatusOK, &run)
	postJSON(t, server.URL+"/api/v1/admin/downsample/policies/coverage_rollup/repair", downsampleRangeRequest{StartUnix: 1, EndUnix: int64(time.Hour)}, http.StatusOK, &run)
	deleteHTTP(t, server.URL+"/api/v1/admin/downsample/policies/coverage_rollup", http.StatusOK)
	deleteHTTP(t, server.URL+"/api/v1/admin/databases/metrics", http.StatusOK)
}

func TestGRPCP0P1RemainingRPCs(t *testing.T) {
	runtime := openTestRuntime(t)
	conn := openBufconnClient(t, runtime)
	defer func() { _ = conn.Close() }()
	ctx := context.Background()

	invokeOK(t, ctx, conn, "CreateDatabase", &databaseRequest{Name: "grpcdb"}, &okResponse{})
	invokeOK(t, ctx, conn, "CreateRetentionPolicy", &grpcRetentionPolicyRequest{
		Database: "grpcdb",
		Policy:   mts.RetentionPolicy{Name: "rp", Duration: time.Hour},
	}, &okResponse{})
	var policies retentionPoliciesResponse
	invokeOK(t, ctx, conn, "ListRetentionPolicies", &databaseRequest{Name: "grpcdb"}, &policies)
	invokeOK(t, ctx, conn, "Write", &writeRequest{Points: []mts.Point{{
		Database:    "grpcdb",
		Measurement: "disk",
		Tags:        map[string]string{"host": "g1"},
		Timestamp:   20,
		Fields:      map[string]mts.FieldValue{"used": mts.Float64Value(9)},
	}}}, &writeResponse{})
	var measurements measurementsResponse
	invokeOK(t, ctx, conn, "ListMeasurements", &metadataRequest{Database: "grpcdb"}, &measurements)
	var series seriesResponse
	invokeOK(t, ctx, conn, "ListSeries", &metadataRequest{Database: "grpcdb", Measurement: "disk"}, &series)
	var stats queryStatsResponse
	invokeOK(t, ctx, conn, "QueryStats", &emptyRequest{}, &stats)
	var schema configSchemaResponse
	invokeOK(t, ctx, conn, "GetConfigSchema", &emptyRequest{}, &schema)
	invokeOK(t, ctx, conn, "ApplyRetention", &retentionApplyRequest{}, &okResponse{})
	var maintenance maintenanceErrorsResponse
	invokeOK(t, ctx, conn, "MaintenanceErrors", &emptyRequest{}, &maintenance)
	var memory storageMemoryResponse
	invokeOK(t, ctx, conn, "StorageMemory", &emptyRequest{}, &memory)
	var compaction compactionStatsResponse
	invokeOK(t, ctx, conn, "CompactionStats", &emptyRequest{}, &compaction)

	invokeOK(t, ctx, conn, "CreateUser", &mts.User{Name: "grpc-bob"}, &okResponse{})
	invokeOK(t, ctx, conn, "UpdateUser", &mts.User{Name: "grpc-bob", DisplayName: "Bob"}, &okResponse{})
	var user userResponse
	invokeOK(t, ctx, conn, "GetUser", &userNameRequest{Name: "grpc-bob"}, &user)
	var users usersResponse
	invokeOK(t, ctx, conn, "ListUsers", &emptyRequest{}, &users)
	perm := databasePermissionRequest{UserName: "grpc-bob", Database: "grpcdb", Permission: mts.DatabasePermissionAdmin}
	invokeOK(t, ctx, conn, "GrantDatabasePermission", &perm, &okResponse{})
	var grants databasePermissionsResponse
	invokeOK(t, ctx, conn, "ListDatabasePermissions", &userNameRequest{Name: "grpc-bob"}, &grants)
	var authz authzDatabaseCheckResponse
	invokeOK(t, ctx, conn, "CheckDatabasePermission", &perm, &authz)
	invokeOK(t, ctx, conn, "RevokeDatabasePermission", &perm, &okResponse{})

	policy := testDownsamplePolicy()
	policy.Name = "grpc_coverage_rollup"
	policy.SourceDatabase = "grpcdb"
	policy.SourceMeasurement = "disk"
	policy.TargetDatabase = "grpcdb"
	invokeOK(t, ctx, conn, "CreateDownsamplePolicy", &policy, &okResponse{})
	var downsamplePolicies downsamplePoliciesResponse
	invokeOK(t, ctx, conn, "ListDownsamplePolicies", &emptyRequest{}, &downsamplePolicies)
	invokeOK(t, ctx, conn, "DisableDownsamplePolicy", &downsamplePolicyRequest{Name: policy.Name}, &okResponse{})
	invokeOK(t, ctx, conn, "EnableDownsamplePolicy", &downsamplePolicyRequest{Name: policy.Name}, &okResponse{})
	invokeOK(t, ctx, conn, "ResetDownsamplePolicy", &grpcDownsampleResetRequest{Name: policy.Name}, &okResponse{})
	var statuses downsampleStatusesResponse
	invokeOK(t, ctx, conn, "DownsamplePolicyStatuses", &emptyRequest{}, &statuses)
	rangeReq := downsamplePolicyRangeRequest{Name: policy.Name, StartUnix: 1, EndUnix: int64(time.Hour)}
	var run downsampleRunResponse
	invokeOK(t, ctx, conn, "RunDownsamplePolicyRange", &rangeReq, &run)
	invokeOK(t, ctx, conn, "RepairDownsamplePolicy", &rangeReq, &run)
	invokeOK(t, ctx, conn, "RunDownsamplePolicy", &rangeReq, &run)
	invokeOK(t, ctx, conn, "DropDownsamplePolicy", &downsamplePolicyRequest{Name: policy.Name}, &okResponse{})
	invokeOK(t, ctx, conn, "DeleteUser", &userNameRequest{Name: "grpc-bob"}, &okResponse{})
	invokeOK(t, ctx, conn, "DropDatabase", &databaseRequest{Name: "grpcdb"}, &okResponse{})
}

func TestP0P1ErrorHelpersAndAuthBranches(t *testing.T) {
	if bearerToken("Basic abc") != "" || bearerToken("Bearer token") != "token" {
		t.Fatal("bearerToken parsing failed")
	}
	if constantTimeEqual("want", "") || constantTimeEqual("want", "got") {
		t.Fatal("constantTimeEqual accepted invalid values")
	}
	statusCode, response := apiErrorResponse(newAPIError(errorCodeAlreadyExists, "exists", nil))
	if statusCode != http.StatusConflict || response.Code != errorCodeAlreadyExists {
		t.Fatalf("api error response = %d %#v", statusCode, response)
	}
	if httpStatusForErrorCode(errorCode("unknown")) != http.StatusInternalServerError {
		t.Fatal("unknown error code should map to 500")
	}
	if grpcCodeForErrorCode(errorCodeUnauthenticated) != codes.Unauthenticated {
		t.Fatal("unauthenticated grpc code mismatch")
	}
	if newAPIError(errorCodeBadRequest, "", mts.ErrInvalidUser).(apiError).Unwrap() == nil {
		t.Fatal("api error unwrap is nil")
	}
	for _, code := range []errorCode{
		errorCodeBadRequest,
		errorCodeUnauthenticated,
		errorCodePermissionDenied,
		errorCodeNotFound,
		errorCodeAlreadyExists,
		errorCodeInternal,
	} {
		if httpStatusForErrorCode(code) == 0 || grpcCodeForErrorCode(code) == codes.OK {
			t.Fatalf("code %s mapped to empty status", code)
		}
	}
	if errorPayload(mts.ErrInvalidUser).Code != errorCodeBadRequest {
		t.Fatal("errorPayload invalid user code mismatch")
	}
	httpReq := httptest.NewRequest(http.MethodGet, "/", nil)
	httpReq.Header.Set(headerAuthorization, "Bearer http-token")
	httpReq.Header.Set(headerAdminToken, "http-admin")
	httpReq.Header.Set(headerUser, "http-user")
	httpSource := httpCredentialSource{request: httpReq}
	if httpSource.Context() != httpReq.Context() ||
		httpSource.Bearer() != "http-token" ||
		httpSource.Value(credentialKeyAdminToken) != "http-admin" ||
		httpSource.Value(credentialKeyUser) != "http-user" {
		t.Fatal("http credential source mismatch")
	}
	grpcCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		strings.ToLower(headerAuthorization), "Bearer grpc-token",
		metadataAdminToken, "grpc-admin",
		metadataUser, "grpc-user",
	))
	grpcSource := grpcCredentialSource{ctx: grpcCtx}
	if grpcSource.Context() != grpcCtx ||
		grpcSource.Bearer() != "grpc-token" ||
		grpcSource.Value(credentialKeyAdminToken) != "grpc-admin" ||
		grpcSource.Value(credentialKeyUser) != "grpc-user" {
		t.Fatal("grpc credential source mismatch")
	}
}

func TestHTTPP0P1AdminAuthAndBadRequestBranches(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	adminPaths := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/v1/admin/config", nil},
		{http.MethodGet, "/api/v1/admin/config/schema", nil},
		{http.MethodPost, "/api/v1/admin/flush", emptyRequest{}},
		{http.MethodPost, "/api/v1/admin/compact", emptyRequest{}},
		{http.MethodPost, "/api/v1/admin/retention/apply", retentionApplyRequest{}},
		{http.MethodGet, "/api/v1/admin/maintenance/errors", nil},
		{http.MethodGet, "/api/v1/admin/stats/storage-memory", nil},
		{http.MethodGet, "/api/v1/admin/stats/compaction", nil},
		{http.MethodGet, "/api/v1/admin/health", nil},
		{http.MethodGet, "/api/v1/admin/downsample/policies", nil},
		{http.MethodGet, "/api/v1/admin/downsample/statuses", nil},
		{http.MethodGet, "/api/v1/users", nil},
		{http.MethodGet, "/api/v1/users/missing", nil},
		{http.MethodPost, "/api/v1/users/missing/database-permissions/default/read", emptyRequest{}},
		{http.MethodDelete, "/api/v1/admin/databases/missing", emptyRequest{}},
		{http.MethodGet, "/api/v1/admin/databases/missing/retention-policies", nil},
		{http.MethodPost, "/api/v1/admin/downsample/policies/missing/enable", emptyRequest{}},
		{http.MethodDelete, "/api/v1/admin/downsample/policies/missing", emptyRequest{}},
	}
	for _, endpoint := range adminPaths {
		t.Run(endpoint.method+endpoint.path, func(t *testing.T) {
			resp := doHTTP(t, endpoint.method, server.URL+endpoint.path, endpoint.body, nil)
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", resp.StatusCode)
			}
		})
	}

	headers := map[string]string{"Authorization": "Bearer test-admin-token"}
	badJSONPaths := []string{
		"/api/v1/data/write",
		"/api/v1/data/write/typed",
		"/api/v1/data/query/rows",
		"/api/v1/data/query/columns",
		"/api/v1/data/query/explain",
		"/api/v1/data/query/stream",
		"/api/v1/admin/databases",
		"/api/v1/admin/retention/apply",
		"/api/v1/admin/downsample/policies",
		"/api/v1/users",
		"/api/v1/authz/database/check",
	}
	for _, path := range badJSONPaths {
		t.Run("bad-json"+path, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, server.URL+path, bytes.NewReader([]byte(`{"bad"`)))
			if err != nil {
				t.Fatalf("NewRequest error = %v", err)
			}
			request.Header.Set("Authorization", headers["Authorization"])
			resp, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatalf("Do error = %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
		})
	}

	postJSONWithHeaders(t, server.URL+"/api/v1/admin/downsample/policies/missing/unknown", emptyRequest{}, headers, http.StatusNotFound, &errorResponse{})
	deleteHTTPWithHeaders(t, server.URL+"/api/v1/admin/downsample/policies/missing", headers, http.StatusOK)
	postJSONWithHeaders(t, server.URL+"/api/v1/users", mts.User{Name: "to-delete"}, headers, http.StatusOK, &okResponse{})
	deleteHTTPWithHeaders(t, server.URL+"/api/v1/users/to-delete", headers, http.StatusOK)
}

func TestHTTPP0P1AdditionalBranches(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	methodCases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/data/write/typed"},
		{http.MethodGet, "/api/v1/data/query/columns"},
		{http.MethodGet, "/api/v1/data/query/explain"},
		{http.MethodPost, "/api/v1/data/query/stats"},
		{http.MethodPost, "/api/v1/admin/config"},
		{http.MethodPost, "/api/v1/admin/config/schema"},
		{http.MethodGet, "/api/v1/admin/flush"},
		{http.MethodGet, "/api/v1/admin/compact"},
		{http.MethodGet, "/api/v1/admin/retention/apply"},
		{http.MethodPost, "/api/v1/admin/maintenance/errors"},
		{http.MethodPost, "/api/v1/admin/stats/storage-memory"},
		{http.MethodPost, "/api/v1/admin/stats/compaction"},
		{http.MethodPost, "/api/v1/admin/health"},
		{http.MethodPost, "/metrics"},
	}
	for _, tt := range methodCases {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			resp := doHTTP(t, tt.method, server.URL+tt.path, emptyRequest{}, nil)
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", resp.StatusCode)
			}
		})
	}

	postJSON(t, server.URL+"/api/v1/users", mts.User{Name: "branch"}, http.StatusOK, &okResponse{})
	var user userResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/users/branch", nil, http.StatusOK, &user)
	if user.User.Name != "branch" {
		t.Fatalf("user = %#v, want branch", user.User)
	}
	postJSON(t, server.URL+"/api/v1/users/branch/database-permissions/default/read", emptyRequest{}, http.StatusOK, &okResponse{})
	resp := doHTTP(t, http.MethodPost, server.URL+"/api/v1/users/branch/database-permissions", emptyRequest{}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("permission collection post status = %d, want 400", resp.StatusCode)
	}
	_ = resp.Body.Close()
	deleteHTTP(t, server.URL+"/api/v1/users/branch", http.StatusOK)
	getJSONWithHeaders(t, server.URL+"/api/v1/users/branch", nil, http.StatusNotFound, &errorResponse{})
	getJSONWithHeaders(t, server.URL+"/api/v1/users/branch/unknown", nil, http.StatusNotFound, &errorResponse{})
	resp = doHTTP(t, http.MethodGet, server.URL+"/api/v1/users", nil, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list users status = %d, want 200", resp.StatusCode)
	}
	_ = resp.Body.Close()

	postJSON(t, server.URL+"/api/v1/admin/databases", databaseRequest{Name: "branchdb"}, http.StatusOK, &okResponse{})
	postJSON(t, server.URL+"/api/v1/admin/databases/branchdb/retention-policies", retentionPolicyRequest{Policy: mts.RetentionPolicy{Name: "short", Duration: time.Hour}}, http.StatusOK, &okResponse{})
	resp = doHTTP(t, http.MethodPut, server.URL+"/api/v1/admin/databases/branchdb/retention-policies", emptyRequest{}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("retention put status = %d, want 400", resp.StatusCode)
	}
	_ = resp.Body.Close()
	resp = doHTTP(t, http.MethodGet, server.URL+"/api/v1/admin/databases/branchdb", nil, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("admin database get status = %d, want 400", resp.StatusCode)
	}
	_ = resp.Body.Close()
	resp = doHTTP(t, http.MethodGet, server.URL+"/api/v1/admin/databases/branchdb/unknown", nil, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("admin database unknown status = %d, want 404", resp.StatusCode)
	}
	_ = resp.Body.Close()
	getJSONWithHeaders(t, server.URL+"/api/v1/data/databases/branchdb/unknown", nil, http.StatusNotFound, &errorResponse{})
	getJSONWithHeaders(t, server.URL+"/api/v1/data/databases/branchdb/measurements/missing/unknown", nil, http.StatusNotFound, &errorResponse{})
	resp = doHTTP(t, http.MethodGet, server.URL+"/api/v1/data/databases/branchdb", nil, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("data db incomplete status = %d, want 400", resp.StatusCode)
	}
	_ = resp.Body.Close()
	deleteHTTP(t, server.URL+"/api/v1/admin/databases/branchdb", http.StatusOK)

	policy := testDownsamplePolicy()
	policy.Name = "branch_rollup"
	postJSON(t, server.URL+"/api/v1/admin/downsample/policies", policy, http.StatusOK, &okResponse{})
	postJSON(t, server.URL+"/api/v1/admin/downsample/policies/branch_rollup/run", downsampleRunRequest{}, http.StatusOK, &downsampleRunResponse{})
	postJSON(t, server.URL+"/api/v1/admin/downsample/policies/branch_rollup/dry-run", downsampleRangeRequest{StartUnix: 1, EndUnix: int64(time.Hour)}, http.StatusOK, &downsampleDryRunResponse{})
	postJSON(t, server.URL+"/api/v1/admin/downsample/policies/missing/run-range", downsampleRangeRequest{StartUnix: 1, EndUnix: int64(time.Hour)}, http.StatusNotFound, &errorResponse{})
	postRaw(t, server.URL+"/api/v1/admin/downsample/policies/branch_rollup/reset", `{"reset":`, http.StatusBadRequest)
	resp = doHTTP(t, http.MethodGet, server.URL+"/api/v1/admin/downsample/policies/branch_rollup/enable", nil, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("downsample action get status = %d, want 400", resp.StatusCode)
	}
	_ = resp.Body.Close()
	deleteHTTP(t, server.URL+"/api/v1/admin/downsample/policies/branch_rollup", http.StatusOK)
	resp = doHTTP(t, http.MethodPatch, server.URL+"/api/v1/admin/downsample/policies", emptyRequest{}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("downsample policies patch status = %d, want 400", resp.StatusCode)
	}
	_ = resp.Body.Close()
}

func TestGRPCP0P1AdminAuthBranches(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	conn := openBufconnClient(t, runtime)
	defer func() { _ = conn.Close() }()
	ctx := context.Background()
	methods := []struct {
		name string
		req  any
	}{
		{"CreateUser", &mts.User{Name: "x"}},
		{"UpdateUser", &mts.User{Name: "x"}},
		{"GetUser", &userNameRequest{Name: "x"}},
		{"ListUsers", &emptyRequest{}},
		{"DeleteUser", &userNameRequest{Name: "x"}},
		{"CreateDatabase", &databaseRequest{Name: "x"}},
		{"GetConfig", &emptyRequest{}},
		{"GetConfigSchema", &emptyRequest{}},
		{"Flush", &emptyRequest{}},
		{"Compact", &emptyRequest{}},
		{"ApplyRetention", &retentionApplyRequest{}},
		{"MaintenanceErrors", &emptyRequest{}},
		{"StorageMemory", &emptyRequest{}},
		{"CompactionStats", &emptyRequest{}},
		{"CreateDownsamplePolicy", &mts.DownsamplePolicy{}},
		{"ListDownsamplePolicies", &emptyRequest{}},
		{"DownsamplePolicyStatuses", &emptyRequest{}},
		{"GrantDatabasePermission", &databasePermissionRequest{}},
		{"RevokeDatabasePermission", &databasePermissionRequest{}},
		{"ListDatabasePermissions", &userNameRequest{}},
		{"CreateRetentionPolicy", &grpcRetentionPolicyRequest{}},
		{"ListRetentionPolicies", &databaseRequest{}},
		{"EnableDownsamplePolicy", &downsamplePolicyRequest{}},
		{"DisableDownsamplePolicy", &downsamplePolicyRequest{}},
		{"DropDownsamplePolicy", &downsamplePolicyRequest{}},
		{"ResetDownsamplePolicy", &grpcDownsampleResetRequest{}},
		{"RunDownsamplePolicyRange", &downsamplePolicyRangeRequest{}},
		{"RepairDownsamplePolicy", &downsamplePolicyRangeRequest{}},
		{"DryRunDownsamplePolicy", &downsamplePolicyRangeRequest{}},
	}
	for _, method := range methods {
		t.Run(method.name, func(t *testing.T) {
			err := invokeGRPC(ctx, conn, method.name, method.req, &okResponse{})
			if status.Code(err) != codes.Unauthenticated {
				t.Fatalf("error = %v, want unauthenticated", err)
			}
		})
	}
}

func TestHTTPP0P1ErrorBranches(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/users", mts.User{Name: "err-user"}, http.StatusOK, &okResponse{})
	postJSON(t, server.URL+"/api/v1/users", mts.User{Name: "err-user"}, http.StatusConflict, &errorResponse{})
	putJSON(t, server.URL+"/api/v1/users/missing", mts.User{DisplayName: "missing"}, http.StatusNotFound, &errorResponse{})
	deleteHTTP(t, server.URL+"/api/v1/users/missing", http.StatusNotFound)
	resp := doHTTP(t, http.MethodPatch, server.URL+"/api/v1/users/err-user", emptyRequest{}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("user patch status = %d, want 400", resp.StatusCode)
	}
	_ = resp.Body.Close()
	resp = doHTTP(t, http.MethodPost, server.URL+"/api/v1/users/err-user/unknown", emptyRequest{}, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("user unknown status = %d, want 404", resp.StatusCode)
	}
	_ = resp.Body.Close()
	resp = doHTTP(t, http.MethodPost, server.URL+"/api/v1/users/err-user/database-permissions/default", emptyRequest{}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("permission incomplete status = %d, want 400", resp.StatusCode)
	}
	_ = resp.Body.Close()
	postJSON(t, server.URL+"/api/v1/users/missing/database-permissions/default/read", emptyRequest{}, http.StatusNotFound, &errorResponse{})
	getJSONWithHeaders(t, server.URL+"/api/v1/users/missing/database-permissions", nil, http.StatusNotFound, &errorResponse{})
	postJSON(t, server.URL+"/api/v1/authz/database/check", authzDatabaseCheckRequest{
		UserName: "err-user", Database: "default", Permission: mts.DatabasePermissionRead,
	}, http.StatusOK, &authzDatabaseCheckResponse{})

	postJSON(t, server.URL+"/api/v1/admin/databases", databaseRequest{Name: "errdb"}, http.StatusOK, &okResponse{})
	postRaw(t, server.URL+"/api/v1/admin/databases/errdb/retention-policies", `{"policy":`, http.StatusBadRequest)
	resp = doHTTP(t, http.MethodPost, server.URL+"/api/v1/data/databases/errdb/measurements", emptyRequest{}, nil)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("data metadata post status = %d, want 405", resp.StatusCode)
	}
	_ = resp.Body.Close()

	postRaw(t, server.URL+"/api/v1/data/write/typed", `{"batch":`, http.StatusBadRequest)
	postJSON(t, server.URL+"/api/v1/data/write/typed", typedWriteRequest{Batch: mts.TypedBatch{
		Database:    "errdb",
		Measurement: "cpu",
		Timestamps:  []int64{1},
		Fields: []mts.TypedFieldColumn{{
			Name:          "usage",
			Type:          mts.FieldFloat64,
			Float64Values: []float64{},
		}},
	}}, http.StatusBadRequest, &errorResponse{})
	badQuery := queryRequest{Query: mts.Query{Measurement: "cpu", Precision: mts.TimePrecision("bad")}}
	postJSON(t, server.URL+"/api/v1/data/query/columns", badQuery, http.StatusBadRequest, &errorResponse{})
	postJSON(t, server.URL+"/api/v1/data/query/explain", badQuery, http.StatusBadRequest, &errorResponse{})
	postJSON(t, server.URL+"/api/v1/data/query/stream", badQuery, http.StatusBadRequest, &errorResponse{})

	postJSON(t, server.URL+"/api/v1/admin/downsample/policies", mts.DownsamplePolicy{Name: "bad"}, http.StatusBadRequest, &errorResponse{})
	postJSON(t, server.URL+"/api/v1/admin/downsample/policies/missing/disable", emptyRequest{}, http.StatusNotFound, &errorResponse{})
	postJSON(t, server.URL+"/api/v1/admin/downsample/policies/missing/repair", downsampleRangeRequest{StartUnix: 1, EndUnix: int64(time.Hour)}, http.StatusNotFound, &errorResponse{})
	deleteHTTP(t, server.URL+"/api/v1/admin/databases/errdb", http.StatusOK)
}

func TestHTTPP0P1PermissionAndMetadataBranches(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/admin/databases", databaseRequest{Name: "securedb"}, http.StatusOK, &okResponse{})
	postJSON(t, server.URL+"/api/v1/users", mts.User{Name: "readonly"}, http.StatusOK, &okResponse{})
	postJSON(t, server.URL+"/api/v1/users/readonly/database-permissions/securedb/read", emptyRequest{}, http.StatusOK, &okResponse{})

	postJSONWithHeaders(t, server.URL+"/api/v1/data/write", writeRequest{Points: []mts.Point{{
		Database:    "securedb",
		Measurement: "cpu",
		Tags:        map[string]string{"host": "s1"},
		Timestamp:   1,
		Fields:      map[string]mts.FieldValue{"usage": mts.Float64Value(1)},
	}}}, map[string]string{"X-MTS-User": "readonly"}, http.StatusForbidden, &errorResponse{})
	postJSON(t, server.URL+"/api/v1/data/write", writeRequest{Points: []mts.Point{{
		Database:    "securedb",
		Measurement: "cpu",
		Tags:        map[string]string{"host": "s1"},
		Timestamp:   1,
		Fields:      map[string]mts.FieldValue{"usage": mts.Float64Value(1)},
	}}}, http.StatusOK, &writeResponse{})

	querySpec, err := mts.NewQuery().From("securedb", "autogen", "cpu").Select("usage").Build()
	if err != nil {
		t.Fatalf("NewQuery error = %v", err)
	}
	query := queryRequest{Query: querySpec}
	postJSONWithHeaders(t, server.URL+"/api/v1/data/query/columns", query, map[string]string{"X-MTS-User": "missing"}, http.StatusForbidden, &errorResponse{})
	getJSONWithHeaders(t, server.URL+"/api/v1/data/databases/securedb/measurements/cpu/series?host=&=bad", nil, http.StatusOK, &seriesResponse{})

	var denied authzDatabaseCheckResponse
	postJSON(t, server.URL+"/api/v1/authz/database/check", authzDatabaseCheckRequest{
		UserName: "readonly", Database: "securedb", Permission: mts.DatabasePermissionAdmin,
	}, http.StatusOK, &denied)
	if denied.Allowed {
		t.Fatal("denied.Allowed = true, want false")
	}
	postJSON(t, server.URL+"/api/v1/authz/database/check", authzDatabaseCheckRequest{
		UserName: "missing", Database: "securedb", Permission: mts.DatabasePermissionRead,
	}, http.StatusOK, &authzDatabaseCheckResponse{})
}

func TestGRPCP0P1AdminTokenSuccessBranches(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	conn := openBufconnClient(t, runtime)
	defer func() { _ = conn.Close() }()
	ctx := metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer test-admin-token"),
	)
	invokeOK(t, ctx, conn, "CreateUser", &mts.User{Name: "token-user"}, &okResponse{})
	invokeOK(t, ctx, conn, "UpdateUser", &mts.User{Name: "token-user", DisplayName: "Token"}, &okResponse{})
	invokeOK(t, ctx, conn, "GetUser", &userNameRequest{Name: "token-user"}, &userResponse{})
	invokeOK(t, ctx, conn, "ListUsers", &emptyRequest{}, &usersResponse{})
	invokeOK(t, ctx, conn, "CreateDatabase", &databaseRequest{Name: "tokendb"}, &okResponse{})
	invokeOK(t, ctx, conn, "CreateRetentionPolicy", &grpcRetentionPolicyRequest{Database: "tokendb", Policy: mts.RetentionPolicy{Name: "rp", Duration: time.Hour}}, &okResponse{})
	invokeOK(t, ctx, conn, "ListRetentionPolicies", &databaseRequest{Name: "tokendb"}, &retentionPoliciesResponse{})
	invokeOK(t, ctx, conn, "GetConfig", &emptyRequest{}, &configResponse{})
	invokeOK(t, ctx, conn, "Flush", &emptyRequest{}, &maintenanceResponse{})
	invokeOK(t, ctx, conn, "Compact", &emptyRequest{}, &maintenanceResponse{})
	perm := databasePermissionRequest{UserName: "token-user", Database: "tokendb", Permission: mts.DatabasePermissionAdmin}
	invokeOK(t, ctx, conn, "GrantDatabasePermission", &perm, &okResponse{})
	invokeOK(t, ctx, conn, "ListDatabasePermissions", &userNameRequest{Name: "token-user"}, &databasePermissionsResponse{})
	invokeOK(t, ctx, conn, "CheckDatabasePermission", &perm, &authzDatabaseCheckResponse{})
	invokeOK(t, ctx, conn, "RevokeDatabasePermission", &perm, &okResponse{})
	policy := testDownsamplePolicy()
	policy.Name = "token_rollup"
	invokeOK(t, ctx, conn, "CreateDownsamplePolicy", &policy, &okResponse{})
	invokeOK(t, ctx, conn, "DisableDownsamplePolicy", &downsamplePolicyRequest{Name: policy.Name}, &okResponse{})
	invokeOK(t, ctx, conn, "EnableDownsamplePolicy", &downsamplePolicyRequest{Name: policy.Name}, &okResponse{})
	invokeOK(t, ctx, conn, "ResetDownsamplePolicy", &grpcDownsampleResetRequest{Name: policy.Name}, &okResponse{})
	invokeOK(t, ctx, conn, "RunDownsamplePolicyRange", &downsamplePolicyRangeRequest{Name: policy.Name, StartUnix: 1, EndUnix: int64(time.Hour)}, &downsampleRunResponse{})
	invokeOK(t, ctx, conn, "RepairDownsamplePolicy", &downsamplePolicyRangeRequest{Name: policy.Name, StartUnix: 1, EndUnix: int64(time.Hour)}, &downsampleRunResponse{})
	invokeOK(t, ctx, conn, "DryRunDownsamplePolicy", &downsamplePolicyRangeRequest{Name: policy.Name, StartUnix: 1, EndUnix: int64(time.Hour)}, &downsampleDryRunResponse{})
	invokeOK(t, ctx, conn, "DropDownsamplePolicy", &downsamplePolicyRequest{Name: policy.Name}, &okResponse{})
	invokeOK(t, ctx, conn, "DeleteUser", &userNameRequest{Name: "token-user"}, &okResponse{})
	invokeOK(t, ctx, conn, "DropDatabase", &databaseRequest{Name: "tokendb"}, &okResponse{})
}

func TestGRPCP0P1ErrorMappingBranches(t *testing.T) {
	runtime := openTestRuntime(t)
	conn := openBufconnClient(t, runtime)
	defer func() { _ = conn.Close() }()
	ctx := context.Background()

	invokeOK(t, ctx, conn, "CreateUser", &mts.User{Name: "grpc-error"}, &okResponse{})
	cases := []struct {
		name string
		req  any
		code codes.Code
	}{
		{name: "CreateUser", req: &mts.User{Name: "grpc-error"}, code: codes.AlreadyExists},
		{name: "UpdateUser", req: &mts.User{Name: "missing"}, code: codes.NotFound},
		{name: "GetUser", req: &userNameRequest{Name: "missing"}, code: codes.NotFound},
		{name: "DeleteUser", req: &userNameRequest{Name: "missing"}, code: codes.NotFound},
		{name: "GrantDatabasePermission", req: &databasePermissionRequest{UserName: "missing", Database: "db", Permission: mts.DatabasePermissionRead}, code: codes.NotFound},
		{name: "CreateDatabase", req: &databaseRequest{}, code: codes.InvalidArgument},
		{name: "CreateDownsamplePolicy", req: &mts.DownsamplePolicy{Name: "bad"}, code: codes.InvalidArgument},
		{name: "EnableDownsamplePolicy", req: &downsamplePolicyRequest{Name: "missing"}, code: codes.NotFound},
		{name: "RunDownsamplePolicyRange", req: &downsamplePolicyRangeRequest{Name: "missing", StartUnix: 1, EndUnix: 2}, code: codes.NotFound},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := invokeGRPC(ctx, conn, tt.name, tt.req, &okResponse{})
			if status.Code(err) != tt.code {
				t.Fatalf("code = %v, want %v, err = %v", status.Code(err), tt.code, err)
			}
		})
	}
}

func TestP0P1DirectHelperBranches(t *testing.T) {
	runtime := openTestRuntime(t)
	if _, err := runtime.queryWithExplain(canceledContext(), queryRequest{Query: testQuery()}); err == nil {
		t.Fatal("queryWithExplain canceled error = nil")
	}
	if err := runtime.writeTypedBatch(canceledContext(), typedWriteRequest{}); err == nil {
		t.Fatal("writeTypedBatch canceled error = nil")
	}
	if _, err := runtime.queryColumns(canceledContext(), queryRequest{}); err == nil {
		t.Fatal("queryColumns canceled error = nil")
	}
	if err := runtime.applyRetention(canceledContext(), retentionApplyRequest{}); err == nil {
		t.Fatal("applyRetention canceled error = nil")
	}
	if runtime.maintenanceErrors(context.Background()) == nil {
		t.Fatal("maintenanceErrors nil")
	}
	if unixNanosOrNow(0).IsZero() || unixNanosOrNow(1).UnixNano() != 1 {
		t.Fatal("unixNanosOrNow mismatch")
	}
	if intMetric(0) != "0" || intMetric(42) != "1" {
		t.Fatal("intMetric mismatch")
	}
	if parts := splitPath("/api/v1/users", "/api/v1/users"); parts != nil {
		t.Fatalf("splitPath empty = %#v, want nil", parts)
	}
	if (&grpcService{}).runtime != nil {
		t.Fatal("empty grpcService runtime should be nil")
	}
	var marker grpcServiceServer = &grpcService{}
	marker.mtsServer()
	if (apiError{Code: errorCodeBadRequest, Cause: mts.ErrInvalidUser}).Error() != mts.ErrInvalidUser.Error() {
		t.Fatal("apiError cause string mismatch")
	}
	if (apiError{Code: errorCodeInternal}).Error() != string(errorCodeInternal) {
		t.Fatal("apiError code string mismatch")
	}
	if status.Code(grpcErrorPlain(newAPIError(errorCodeInternal, "internal", nil))) != codes.Internal {
		t.Fatal("grpc internal code mismatch")
	}
	if status.Code(grpcErrorPlain(errors.New("plain failure"))) != codes.Internal {
		t.Fatal("grpc plain error code mismatch")
	}
}

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func deleteHTTP(t *testing.T, url string, wantStatus int) {
	t.Helper()
	deleteHTTPWithHeaders(t, url, nil, wantStatus)
}

func deleteHTTPWithHeaders(t *testing.T, url string, headers map[string]string, wantStatus int) {
	t.Helper()
	request, err := http.NewRequest(http.MethodDelete, url, bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("NewRequest(delete) error = %v", err)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Delete(%s) error = %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != wantStatus {
		t.Fatalf("Delete(%s) status = %d, want %d", url, resp.StatusCode, wantStatus)
	}
}

func doHTTP(t *testing.T, method string, url string, body any, headers map[string]string) *http.Response {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Marshal request error = %v", err)
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("NewRequest error = %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Do request error = %v", err)
	}
	return resp
}

func putJSON(t *testing.T, url string, req any, wantStatus int, out any) {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal(put) error = %v", err)
	}
	request, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest(put) error = %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Put(%s) error = %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != wantStatus {
		t.Fatalf("Put(%s) status = %d, want %d", url, resp.StatusCode, wantStatus)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("Decode(put response) error = %v", err)
	}
}

func invokeOK(t *testing.T, ctx context.Context, conn *grpc.ClientConn, method string, in any, out any) {
	t.Helper()
	if err := invokeGRPC(ctx, conn, method, in, out); err != nil {
		t.Fatalf("Invoke %s error = %v", method, err)
	}
}
