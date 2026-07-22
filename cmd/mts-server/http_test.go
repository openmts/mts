package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func TestHTTPWriteAndQueryRows(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/data/write", writeRequest{
		Points: []mts.Point{testPoint()},
		Options: mts.WriteOptions{
			Sync: true,
		},
	}, http.StatusOK, &writeResponse{})

	var response queryRowsResponse
	postJSON(t, server.URL+"/api/v1/data/query/rows", queryRowsRequest{
		Query: testQuery(),
	}, http.StatusOK, &response)
	if len(response.Rows) != 1 || response.Rows[0].Fields["usage"].Float64 != 0.7 {
		t.Fatalf("rows = %#v, want one usage row", response.Rows)
	}
}

func TestHTTPRequireUserAuthenticatesBearerToken(t *testing.T) {
	runtime := openTestRuntime(t)
	runtime.config.Auth.RequireUser = true
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	seedUserWithPassword(t, runtime, mts.User{Name: "alice"}, "secret")
	seedDatabasePermission(t, runtime, "alice", "default", mts.DatabasePermissionWrite)

	point := testPoint()
	postJSONWithHeaders(t, server.URL+"/api/v1/data/write", writeRequest{Points: []mts.Point{point}}, map[string]string{
		"X-MTS-User": "alice",
	}, http.StatusUnauthorized, &errorResponse{})
	postJSONWithHeaders(t, server.URL+"/api/v1/data/write", writeRequest{Points: []mts.Point{point}}, map[string]string{
		"Authorization": "Bearer bad-token",
	}, http.StatusUnauthorized, &errorResponse{})

	var login authTokenResponse
	postJSON(t, server.URL+"/api/v1/auth/login", loginRequest{
		UserName:   "alice",
		Password:   "secret",
		TTLSeconds: 60,
	}, http.StatusOK, &login)
	if login.Token.Token == "" {
		t.Fatal("login token is empty")
	}
	postJSONWithHeaders(t, server.URL+"/api/v1/data/write", writeRequest{Points: []mts.Point{point}}, map[string]string{
		"Authorization": "Bearer " + login.Token.Token,
	}, http.StatusOK, &writeResponse{})
}

func TestHTTPRequireUserBootstrapsDefaultAdmin(t *testing.T) {
	runtime := openTestRuntimeRequireUser(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	getJSONWithHeaders(t, server.URL+"/api/v1/users", nil, http.StatusUnauthorized, &errorResponse{})
	adminToken := loginHTTPUser(t, server.URL, "admin", "admin")
	adminHeaders := map[string]string{"Authorization": "Bearer " + adminToken}
	getJSONWithHeaders(t, server.URL+"/api/v1/users", adminHeaders, http.StatusOK, &usersResponse{})

	var userResp userResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/users/admin", adminHeaders, http.StatusOK, &userResp)
	if userResp.User.Name != "admin" || userResp.User.Role != mts.UserRoleAdmin {
		t.Fatalf("admin user = %#v, want admin role", userResp.User)
	}
}

func TestHTTPCreateUserAcceptsInitialPassword(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/users", createUserRequest{
		User:     mts.User{Name: "created-user", Role: mts.UserRoleUser},
		Password: "secret",
	}, http.StatusOK, &okResponse{})

	if token := loginHTTPUser(t, server.URL, "created-user", "secret"); token == "" {
		t.Fatal("login token is empty")
	}
	resp, err := http.Get(server.URL + "/api/v1/users/created-user")
	if err != nil {
		t.Fatalf("Get(user) error = %v", err)
	}
	body, closeBody := readResponseBody(t, resp)
	defer closeBody()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Get(user) status = %d, want 200, body = %s", resp.StatusCode, body)
	}
	if strings.Contains(body, "secret") || strings.Contains(body, `"password"`) {
		t.Fatalf("Get(user) leaked password material: %s", body)
	}
}

func TestHTTPCreateUserRollsBackWhenInitialPasswordInvalid(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/users", createUserRequest{
		User:     mts.User{Name: "rollback-user"},
		Password: " ",
	}, http.StatusUnauthorized, &errorResponse{})

	getJSONWithHeaders(t, server.URL+"/api/v1/users/rollback-user", nil, http.StatusNotFound, &errorResponse{})
}

func TestRuntimeDoesNotResetExistingDefaultAdminPassword(t *testing.T) {
	ctx := context.Background()
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.HTTP.Addr = "127.0.0.1:0"
	cfg.GRPC.Addr = "127.0.0.1:0"
	cfg.Auth.RequireUser = true
	cfg.Observability.AccessLog = false

	runtime, err := openRuntime(ctx, cfg)
	if err != nil {
		t.Fatalf("openRuntime(first) error = %v", err)
	}
	if err := runtime.engine.ChangePassword(ctx, "admin", "admin", "changed"); err != nil {
		t.Fatalf("ChangePassword(admin) error = %v", err)
	}
	if err := runtime.shutdown(ctx); err != nil {
		t.Fatalf("shutdown(first) error = %v", err)
	}

	reopened, err := openRuntime(ctx, cfg)
	if err != nil {
		t.Fatalf("openRuntime(reopen) error = %v", err)
	}
	t.Cleanup(func() {
		if err := reopened.shutdown(ctx); err != nil {
			t.Fatalf("shutdown(reopened) error = %v", err)
		}
	})
	if _, err := reopened.engine.Authenticate(ctx, mts.Credentials{UserName: "admin", Password: "admin"}, time.Minute); err != mts.ErrInvalidCredentials {
		t.Fatalf("Authenticate(admin/default) error = %v, want ErrInvalidCredentials", err)
	}
	if _, err := reopened.engine.Authenticate(ctx, mts.Credentials{UserName: "admin", Password: "changed"}, time.Minute); err != nil {
		t.Fatalf("Authenticate(admin/changed) error = %v", err)
	}
}

func TestBootstrapDefaultAdminSkipsWhenUserAuthDisabled(t *testing.T) {
	ctx := context.Background()
	runtime := openTestRuntime(t)

	// 密码认证开启时，即使 require_user=false 也应预置 admin，保证 Dashboard 可登录。
	if err := bootstrapDefaultAdmin(ctx, runtime.currentConfig(), runtime.engine); err != nil {
		t.Fatalf("bootstrapDefaultAdmin(password auth enabled) error = %v", err)
	}
	if _, err := runtime.engine.Authenticate(ctx, mts.Credentials{UserName: "admin", Password: "admin"}, time.Minute); err != nil {
		t.Fatalf("Authenticate(admin/admin) error = %v", err)
	}

	// 密码认证关闭时跳过 bootstrap。
	cfg := runtime.currentConfig()
	cfg.Auth.RequireUser = true
	cfg.User.PasswordAuthDisabled = true
	// 清理已有 admin，验证禁用密码认证时不会再创建/修复
	if err := runtime.engine.DeleteUser(ctx, "admin"); err != nil {
		t.Fatalf("DeleteUser(admin) error = %v", err)
	}
	if err := bootstrapDefaultAdmin(ctx, cfg, runtime.engine); err != nil {
		t.Fatalf("bootstrapDefaultAdmin(disabled password auth) error = %v", err)
	}
	if _, ok, err := runtime.engine.GetUser(ctx, "admin"); err != nil || ok {
		t.Fatalf("GetUser(admin) ok=%v err=%v, want missing", ok, err)
	}
}

func TestBootstrapDefaultAdminPromotesExistingAdminUser(t *testing.T) {
	ctx := context.Background()
	runtime := openTestRuntime(t)
	// openTestRuntime 已 bootstrap 默认 admin；先降级为 disabled user 再验证提升逻辑。
	if err := runtime.engine.UpdateUser(ctx, mts.User{Name: "admin", Role: mts.UserRoleUser, Disabled: true}); err != nil {
		t.Fatalf("UpdateUser(admin demote) error = %v", err)
	}
	cfg := runtime.currentConfig()
	cfg.Auth.RequireUser = true
	if err := bootstrapDefaultAdmin(ctx, cfg, runtime.engine); err != nil {
		t.Fatalf("bootstrapDefaultAdmin(existing admin) error = %v", err)
	}
	admin, ok, err := runtime.engine.GetUser(ctx, "admin")
	if err != nil || !ok {
		t.Fatalf("GetUser(admin) ok=%v err=%v", ok, err)
	}
	if admin.Role != mts.UserRoleAdmin || admin.Disabled {
		t.Fatalf("admin = %#v, want enabled admin role", admin)
	}
	// 密码仍保留 bootstrap 初始值
	if _, err := runtime.engine.Authenticate(ctx, mts.Credentials{UserName: "admin", Password: "admin"}, time.Minute); err != nil {
		t.Fatalf("Authenticate(admin/admin) error = %v", err)
	}
}

func TestHTTPUserRoleControlsUserManagement(t *testing.T) {
	runtime := openTestRuntime(t)
	runtime.config.Auth.RequireUser = true
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	seedUserWithPassword(t, runtime, mts.User{Name: "admin", Role: mts.UserRoleAdmin}, "admin-secret")
	seedUserWithPassword(t, runtime, mts.User{Name: "alice"}, "alice-secret")
	seedUserWithPassword(t, runtime, mts.User{Name: "bob"}, "bob-secret")
	adminToken := loginHTTPUser(t, server.URL, "admin", "admin-secret")
	aliceToken := loginHTTPUser(t, server.URL, "alice", "alice-secret")

	aliceHeaders := map[string]string{"Authorization": "Bearer " + aliceToken}
	deleteHTTPWithHeaders(t, server.URL+"/api/v1/users/bob", nil, http.StatusUnauthorized)
	deleteHTTPWithHeaders(t, server.URL+"/api/v1/users/bob", aliceHeaders, http.StatusForbidden)
	putJSONWithHeaders(t, server.URL+"/api/v1/users/bob/password", passwordRequest{Password: "next"}, aliceHeaders, http.StatusForbidden, &errorResponse{})
	putJSONWithHeaders(
		t,
		server.URL+"/api/v1/users/bob/database-permissions/default/read",
		emptyRequest{},
		aliceHeaders,
		http.StatusForbidden,
		&errorResponse{},
	)

	adminHeaders := map[string]string{"Authorization": "Bearer " + adminToken}
	putJSONWithHeaders(t, server.URL+"/api/v1/users/bob/password", passwordRequest{Password: "next"}, adminHeaders, http.StatusOK, &okResponse{})
	putJSONWithHeaders(
		t,
		server.URL+"/api/v1/users/bob/database-permissions/default/read",
		emptyRequest{},
		adminHeaders,
		http.StatusOK,
		&okResponse{},
	)
	deleteHTTPWithHeaders(
		t,
		server.URL+"/api/v1/users/bob/database-permissions/default/read",
		aliceHeaders,
		http.StatusForbidden,
	)
	deleteHTTPWithHeaders(t, server.URL+"/api/v1/users/bob", adminHeaders, http.StatusOK)
}

func TestHTTPUserCanOnlyChangeOwnPassword(t *testing.T) {
	runtime := openTestRuntime(t)
	runtime.config.Auth.RequireUser = true
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	seedUserWithPassword(t, runtime, mts.User{Name: "alice"}, "alice-secret")
	seedUserWithPassword(t, runtime, mts.User{Name: "bob"}, "bob-secret")
	aliceToken := loginHTTPUser(t, server.URL, "alice", "alice-secret")
	aliceHeaders := map[string]string{"Authorization": "Bearer " + aliceToken}

	postJSONWithHeaders(
		t,
		server.URL+"/api/v1/auth/password",
		changePasswordRequest{UserName: "bob", OldPassword: "bob-secret", NewPassword: "blocked"},
		aliceHeaders,
		http.StatusForbidden,
		&errorResponse{},
	)
	postJSONWithHeaders(
		t,
		server.URL+"/api/v1/auth/password",
		changePasswordRequest{UserName: "alice", OldPassword: "alice-secret", NewPassword: "alice-next"},
		aliceHeaders,
		http.StatusOK,
		&okResponse{},
	)
	postJSON(t, server.URL+"/api/v1/auth/login", loginRequest{
		UserName:   "alice",
		Password:   "alice-secret",
		TTLSeconds: 60,
	}, http.StatusUnauthorized, &errorResponse{})
	if token := loginHTTPUser(t, server.URL, "alice", "alice-next"); token == "" {
		t.Fatal("login token is empty after password change")
	}
}

func TestHTTPHealthAndBadRequest(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("Get(healthz) error = %v", err)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("Close(healthz body) error = %v", closeErr)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want 200", resp.StatusCode)
	}

	postJSON(t, server.URL+"/api/v1/data/write", map[string]any{"bad": true}, http.StatusBadRequest, &errorResponse{})
	resp, err = http.Post(server.URL+"/healthz", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("Post(healthz) error = %v", err)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("Close(healthz post body) error = %v", closeErr)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST health status = %d, want 405", resp.StatusCode)
	}
}

func TestHTTPWriteAndQueryErrors(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/data/write", writeRequest{
		Points: []mts.Point{{Measurement: " "}},
	}, http.StatusBadRequest, &errorResponse{})
	postRaw(t, server.URL+"/api/v1/data/query/rows", `{"query":`, http.StatusBadRequest)

	resp, err := http.Get(server.URL + "/api/v1/data/query/rows")
	if err != nil {
		t.Fatalf("Get(query rows) error = %v", err)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("Close(query rows body) error = %v", closeErr)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET query rows status = %d, want 405", resp.StatusCode)
	}
}

func TestHTTPLegacyMixedNamespaceIsNotMounted(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	legacyPaths := []string{
		"/api/v1/write",
		"/api/v1/query/rows",
		"/api/v1/flush",
		"/api/v1/compact",
	}
	for _, path := range legacyPaths {
		t.Run(path, func(t *testing.T) {
			resp, err := http.Get(server.URL + path)
			if err != nil {
				t.Fatalf("Get(%s) error = %v", path, err)
			}
			if closeErr := resp.Body.Close(); closeErr != nil {
				t.Fatalf("Close(%s body) error = %v", path, closeErr)
			}
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("GET %s status = %d, want 404", path, resp.StatusCode)
			}
		})
	}
}

func TestHTTPMaintenanceEndpoints(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	var flushResp maintenanceResponse
	postJSON(t, server.URL+"/api/v1/admin/flush", map[string]any{}, http.StatusOK, &flushResp)
	if !flushResp.OK {
		t.Fatal("flush OK = false, want true")
	}

	var compactResp maintenanceResponse
	postJSON(t, server.URL+"/api/v1/admin/compact", map[string]any{}, http.StatusOK, &compactResp)
	if !compactResp.OK {
		t.Fatal("compact OK = false, want true")
	}

	resp, err := http.Get(server.URL + "/api/v1/admin/flush")
	if err != nil {
		t.Fatalf("Get(flush) error = %v", err)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("Close(flush body) error = %v", closeErr)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET flush status = %d, want 405", resp.StatusCode)
	}

	resp, err = http.Get(server.URL + "/api/v1/admin/compact")
	if err != nil {
		t.Fatalf("Get(compact) error = %v", err)
	}
	if closeErr := resp.Body.Close(); closeErr != nil {
		t.Fatalf("Close(compact body) error = %v", closeErr)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET compact status = %d, want 405", resp.StatusCode)
	}
}

func TestHTTPMaintenanceContextErrors(t *testing.T) {
	runtime := openTestRuntime(t)
	for _, tt := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{name: "flush", handler: runtime.handleFlush},
		{name: "compact", handler: runtime.handleCompact},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{}`))).WithContext(ctx)
			response := httptest.NewRecorder()
			tt.handler(response, request)
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", response.Code)
			}
		})
	}
}

func TestHTTPP0P1DataMetadataUsersAdminAndDownsample(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	adminHeaders := map[string]string{}
	user := mts.User{Name: "alice", DisplayName: "Alice"}
	postJSONWithHeaders(t, server.URL+"/api/v1/users", user, adminHeaders, http.StatusOK, &okResponse{})
	postJSONWithHeaders(t, server.URL+"/api/v1/users/alice/database-permissions/default/write", map[string]any{}, adminHeaders, http.StatusOK, &okResponse{})
	postJSONWithHeaders(t, server.URL+"/api/v1/users/alice/database-permissions/default/read", map[string]any{}, adminHeaders, http.StatusOK, &okResponse{})

	var userResp userResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/users/alice", adminHeaders, http.StatusOK, &userResp)
	if userResp.User.Name != "alice" {
		t.Fatalf("user = %#v, want alice", userResp.User)
	}

	batch := mts.TypedBatch{
		Measurement: "cpu",
		Tags: []mts.TagColumn{{
			Name:   "host",
			Values: []string{"api-1", "api-1"},
		}},
		Timestamps: []int64{1, 2},
		Fields: []mts.TypedFieldColumn{{
			Name:          "usage",
			Type:          mts.FieldFloat64,
			Float64Values: []float64{0.7, 0.9},
		}},
	}
	dataHeaders := map[string]string{"X-MTS-User": "alice"}
	postJSONWithHeaders(t, server.URL+"/api/v1/data/write/typed", typedWriteRequest{
		Batch:   batch,
		Options: mts.WriteOptions{Sync: true},
	}, dataHeaders, http.StatusOK, &writeResponse{})

	var columnsResp queryColumnsResponse
	postJSONWithHeaders(t, server.URL+"/api/v1/data/query/columns", queryRequest{Query: testQuery()}, dataHeaders, http.StatusOK, &columnsResp)
	if len(columnsResp.Columns) != 1 || len(columnsResp.Columns[0].Timestamps) != 2 {
		t.Fatalf("columns = %#v, want one column with two points", columnsResp.Columns)
	}

	var explainResp queryExplainResponse
	postJSONWithHeaders(t, server.URL+"/api/v1/data/query/explain", queryRequest{Query: testQuery()}, dataHeaders, http.StatusOK, &explainResp)
	if len(explainResp.Result.Columns) == 0 || explainResp.Result.Explain.Measurement != "cpu" {
		t.Fatalf("explain response = %#v, want cpu explain with columns", explainResp)
	}

	streamResp, err := postJSONRawWithHeaders(server.URL+"/api/v1/data/query/stream", queryRequest{Query: testQuery()}, dataHeaders)
	if err != nil {
		t.Fatalf("query stream error = %v", err)
	}
	streamBody, closeStream := readResponseBody(t, streamResp)
	defer closeStream()
	if streamResp.StatusCode != http.StatusOK || !strings.Contains(streamBody, `"type":"row"`) {
		t.Fatalf("stream status/body = %d %s, want row stream", streamResp.StatusCode, streamBody)
	}

	columnStreamResp, err := postJSONRawWithHeaders(server.URL+"/api/v1/data/query/stream", queryStreamRequest{
		Query:  testQuery(),
		Format: "column",
	}, dataHeaders)
	if err != nil {
		t.Fatalf("column stream error = %v", err)
	}
	columnStreamBody, closeColumnStream := readResponseBody(t, columnStreamResp)
	defer closeColumnStream()
	if columnStreamResp.StatusCode != http.StatusOK || !strings.Contains(columnStreamBody, `"type":"column"`) {
		t.Fatalf("column stream status/body = %d %s, want column stream", columnStreamResp.StatusCode, columnStreamBody)
	}

	postJSONWithHeaders(t, server.URL+"/api/v1/data/write/points-typed", writeRequest{
		Points: []mts.Point{{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   3,
			Fields:      map[string]mts.FieldValue{"usage": mts.Float64Value(1.1)},
		}},
		Options: mts.WriteOptions{Sync: true},
	}, dataHeaders, http.StatusOK, &writeResponse{})

	postJSONWithHeaders(t, server.URL+"/api/v1/data/delete", deleteRequest{
		Request: mts.DeleteRequest{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			StartTime:   3,
			EndTime:     3,
		},
	}, dataHeaders, http.StatusOK, &okResponse{})

	var maintenanceStatsResp maintenanceStatsResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/stats/maintenance", adminHeaders, http.StatusOK, &maintenanceStatsResp)

	var opsStatusResp opsStatusResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/ops-status", adminHeaders, http.StatusOK, &opsStatusResp)
	if opsStatusResp.AdminOpBusy {
		t.Fatal("ops-status admin_op_busy want false when idle")
	}

	postJSONWithHeaders(t, server.URL+"/api/v1/admin/flush", emptyRequest{}, adminHeaders, http.StatusOK, &okResponse{})
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/ops-status", adminHeaders, http.StatusOK, &opsStatusResp)
	if opsStatusResp.AdminOpBusy {
		t.Fatal("ops-status after flush want not busy")
	}
	if opsStatusResp.Last == nil || opsStatusResp.Last.Op != "flush" || !opsStatusResp.Last.OK {
		t.Fatalf("ops-status last after flush = %+v", opsStatusResp.Last)
	}

	var statsResp queryStatsResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/data/query/stats", dataHeaders, http.StatusOK, &statsResp)
	if statsResp.Stats.SamplesReturned == 0 {
		t.Fatalf("query stats = %#v, want returned samples", statsResp.Stats)
	}

	var fieldsResp fieldsResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/data/databases/default/measurements/cpu/fields", dataHeaders, http.StatusOK, &fieldsResp)
	if len(fieldsResp.Fields) != 1 || fieldsResp.Fields[0].Name != "usage" {
		t.Fatalf("fields = %#v, want usage field", fieldsResp.Fields)
	}

	var cfgResp configResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/config/effective", adminHeaders, http.StatusOK, &cfgResp)
	if cfgResp.Config.DataDir == "" {
		t.Fatalf("config response = %#v, want data dir", cfgResp)
	}

	metricsResp, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatalf("Get(metrics) error = %v", err)
	}
	metricsBody, closeMetrics := readResponseBody(t, metricsResp)
	defer closeMetrics()
	if metricsResp.StatusCode != http.StatusOK || !strings.Contains(metricsBody, "mts_health_ready") {
		t.Fatalf("metrics status/body = %d %s, want mts metrics", metricsResp.StatusCode, metricsBody)
	}

	policy := testDownsamplePolicy()
	postJSONWithHeaders(t, server.URL+"/api/v1/admin/downsample/policies", policy, adminHeaders, http.StatusOK, &okResponse{})
	var policiesResp downsamplePoliciesResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/downsample/policies", adminHeaders, http.StatusOK, &policiesResp)
	if len(policiesResp.Policies) != 1 || policiesResp.Policies[0].Name != policy.Name {
		t.Fatalf("policies = %#v, want rollup_cpu", policiesResp.Policies)
	}
	postJSONWithHeaders(t, server.URL+"/api/v1/admin/downsample/policies/rollup_cpu/dry-run", downsampleRangeRequest{
		StartUnix: 1,
		EndUnix:   int64(time.Hour),
	}, adminHeaders, http.StatusOK, &downsampleDryRunResponse{})
}

func TestHTTPAdminAuth(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	var response errorResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/users", nil, http.StatusUnauthorized, &response)
	if response.Code != errorCodeUnauthenticated {
		t.Fatalf("auth error = %#v, want unauthenticated", response)
	}
}

func clearTestMustChangePassword(t *testing.T, runtime *serverRuntime) {
	t.Helper()
	if err := runtime.clearMustChangePassword(context.Background(), defaultSystemAdminName); err != nil {
		t.Fatalf("clearTestMustChangePassword error = %v", err)
	}
}

func TestHTTPOpsStatusLastErrorAfterFailedHeavy(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	if err := runtime.tryBeginAdminHeavy("compact"); err != nil {
		t.Fatalf("begin heavy: %v", err)
	}
	runtime.finishAdminHeavy(errors.New("disk full"))

	var opsStatusResp opsStatusResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/ops-status", nil, http.StatusOK, &opsStatusResp)
	if opsStatusResp.AdminOpBusy {
		t.Fatal("ops-status want not busy after failed heavy")
	}
	if opsStatusResp.Last == nil || opsStatusResp.Last.Op != "compact" || opsStatusResp.Last.OK || opsStatusResp.Last.Error != "disk full" {
		t.Fatalf("ops-status last after error = %+v", opsStatusResp.Last)
	}
	if opsStatusResp.Last.FinishedAtUnix <= 0 {
		t.Fatalf("finished_at_unix missing: %+v", opsStatusResp.Last)
	}

	// maintenance stats should mirror last
	var maint maintenanceStatsResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/stats/maintenance", nil, http.StatusOK, &maint)
	if maint.Last == nil || maint.Last.Op != "compact" || maint.Last.OK || maint.Last.Error != "disk full" {
		t.Fatalf("maintenance last after error = %+v", maint.Last)
	}

	var maintErrs maintenanceErrorsResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/maintenance/errors", nil, http.StatusOK, &maintErrs)
	if maintErrs.Last == nil || maintErrs.Last.Op != "compact" || maintErrs.Last.OK || maintErrs.Last.Error != "disk full" {
		t.Fatalf("maintenance/errors last after error = %+v", maintErrs.Last)
	}
}

func TestHTTPMaintenanceErrorsBusyAndLast(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	if err := runtime.tryBeginAdminHeavy("flush"); err != nil {
		t.Fatalf("begin: %v", err)
	}
	var busyResp maintenanceErrorsResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/maintenance/errors", nil, http.StatusOK, &busyResp)
	if !busyResp.AdminOpBusy || busyResp.Op != "flush" || busyResp.StartedAtUnix <= 0 {
		t.Fatalf("busy maintenance/errors = %+v", busyResp)
	}
	runtime.finishAdminHeavy(errors.New("wal fsync failed"))
	var doneResp maintenanceErrorsResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/maintenance/errors", nil, http.StatusOK, &doneResp)
	if doneResp.AdminOpBusy {
		t.Fatal("want not busy after finish")
	}
	if doneResp.Last == nil || doneResp.Last.Op != "flush" || doneResp.Last.OK || doneResp.Last.Error != "wal fsync failed" {
		t.Fatalf("last after fail = %+v", doneResp.Last)
	}
}

func TestHTTPAdminHealthBusyAndLast(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	if err := runtime.tryBeginAdminHeavy("compact"); err != nil {
		t.Fatalf("begin: %v", err)
	}
	var busy adminHealthResponse
	getJSONWithHeaders(t, server.URL+routeAdminHealth, nil, http.StatusOK, &busy)
	if !busy.Health.Healthy || !busy.AdminOpBusy || busy.Op != "compact" || busy.StartedAtUnix <= 0 {
		t.Fatalf("health busy = %+v", busy)
	}
	runtime.finishAdminHeavy(errors.New("health probe fail"))
	var done adminHealthResponse
	getJSONWithHeaders(t, server.URL+routeAdminHealth, nil, http.StatusOK, &done)
	if done.AdminOpBusy {
		t.Fatal("health want not busy after finish")
	}
	if done.Last == nil || done.Last.Op != "compact" || done.Last.OK || done.Last.Error != "health probe fail" {
		t.Fatalf("health last after fail = %+v", done.Last)
	}
}

func openTestRuntime(t *testing.T) *serverRuntime {
	t.Helper()
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.HTTP.Addr = "127.0.0.1:0"
	cfg.GRPC.Addr = "127.0.0.1:0"
	cfg.Observability.AccessLog = false
	runtime, err := openRuntime(context.Background(), cfg)
	if err != nil {
		t.Fatalf("openRuntime() error = %v", err)
	}
	t.Cleanup(func() {
		if err := runtime.shutdown(context.Background()); err != nil {
			t.Fatalf("shutdown runtime error = %v", err)
		}
	})
	clearTestMustChangePassword(t, runtime)
	return runtime
}

func openTestRuntimeWithAdminToken(t *testing.T) *serverRuntime {
	t.Helper()
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.HTTP.Addr = "127.0.0.1:0"
	cfg.GRPC.Addr = "127.0.0.1:0"
	cfg.Auth.AdminToken = "test-admin-token"
	cfg.Observability.AccessLog = false
	runtime, err := openRuntime(context.Background(), cfg)
	if err != nil {
		t.Fatalf("openRuntime() error = %v", err)
	}
	t.Cleanup(func() {
		if err := runtime.shutdown(context.Background()); err != nil {
			t.Fatalf("shutdown runtime error = %v", err)
		}
	})
	clearTestMustChangePassword(t, runtime)
	return runtime
}

func openTestRuntimeRequireUser(t *testing.T) *serverRuntime {
	t.Helper()
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.HTTP.Addr = "127.0.0.1:0"
	cfg.GRPC.Addr = "127.0.0.1:0"
	cfg.Auth.RequireUser = true
	cfg.Observability.AccessLog = false
	runtime, err := openRuntime(context.Background(), cfg)
	if err != nil {
		t.Fatalf("openRuntime() error = %v", err)
	}
	t.Cleanup(func() {
		if err := runtime.shutdown(context.Background()); err != nil {
			t.Fatalf("shutdown runtime error = %v", err)
		}
	})
	clearTestMustChangePassword(t, runtime)
	return runtime
}

func testDownsamplePolicy() mts.DownsamplePolicy {
	return mts.DownsamplePolicy{
		Name:              "rollup_cpu",
		SourceDatabase:    "default",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "default",
		TargetRetention:   "autogen",
		TargetMeasurement: "cpu_1h",
		Interval:          time.Hour,
		RefreshInterval:   time.Hour,
		Lookback:          time.Hour,
		BatchSize:         100,
		Functions: []mts.DownsampleFunction{{
			Function: mts.AggregateAvg,
			Field:    "usage",
			As:       "usage_avg",
		}},
		GroupByTags: []string{"host"},
		Enabled:     true,
	}
}

func testPoint() mts.Point {
	return mts.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "api-1"},
		Timestamp:   1,
		Fields: map[string]mts.FieldValue{
			"usage": mts.Float64Value(0.7),
		},
	}
}

func testQuery() mts.Query {
	query, err := mts.NewQuery().From("default", "autogen", "cpu").Select("usage").Build()
	if err != nil {
		panic(err)
	}
	return query
}

func postJSON(t *testing.T, url string, req any, wantStatus int, out any) {
	t.Helper()
	postJSONWithHeaders(t, url, req, nil, wantStatus, out)
}

func postJSONWithHeaders(
	t *testing.T,
	url string,
	req any,
	headers map[string]string,
	wantStatus int,
	out any,
) {
	t.Helper()
	resp, err := postJSONRawWithHeaders(url, req, headers)
	if err != nil {
		t.Fatalf("Post(%s) error = %v", url, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("Close(response body) error = %v", err)
		}
	}()
	if resp.StatusCode != wantStatus {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Post(%s) status = %d, want %d, body = %s", url, resp.StatusCode, wantStatus, string(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("Decode(response) error = %v", err)
	}
}

func postJSONRawWithHeaders(url string, req any, headers map[string]string) (*http.Response, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return http.DefaultClient.Do(request)
}

func loginHTTPUser(t *testing.T, baseURL string, userName string, password string) string {
	t.Helper()
	var login authTokenResponse
	postJSON(t, baseURL+"/api/v1/auth/login", loginRequest{
		UserName:   userName,
		Password:   password,
		TTLSeconds: 60,
	}, http.StatusOK, &login)
	if login.Token.Token == "" {
		t.Fatal("login token is empty")
	}
	return login.Token.Token
}

func seedUserWithPassword(t *testing.T, runtime *serverRuntime, user mts.User, password string) {
	t.Helper()
	ctx := context.Background()
	if _, ok, err := runtime.engine.GetUser(ctx, user.Name); err != nil {
		t.Fatalf("GetUser(seed %s) error = %v", user.Name, err)
	} else if !ok {
		if err := runtime.engine.CreateUser(ctx, user); err != nil {
			t.Fatalf("CreateUser(seed %s) error = %v", user.Name, err)
		}
	} else if err := runtime.engine.UpdateUser(ctx, user); err != nil {
		t.Fatalf("UpdateUser(seed %s) error = %v", user.Name, err)
	}
	if err := runtime.engine.SetPassword(ctx, user.Name, password); err != nil {
		t.Fatalf("SetPassword(seed %s) error = %v", user.Name, err)
	}
}

func seedDatabasePermission(
	t *testing.T,
	runtime *serverRuntime,
	userName string,
	database string,
	permission mts.DatabasePermission,
) {
	t.Helper()
	if err := runtime.engine.GrantDatabasePermission(context.Background(), userName, database, permission); err != nil {
		t.Fatalf("GrantDatabasePermission(seed %s %s %s) error = %v", userName, database, permission, err)
	}
}

func getJSONWithHeaders(
	t *testing.T,
	url string,
	headers map[string]string,
	wantStatus int,
	out any,
) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("NewRequest(%s) error = %v", url, err)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Get(%s) error = %v", url, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("Close(response body) error = %v", err)
		}
	}()
	if resp.StatusCode != wantStatus {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Get(%s) status = %d, want %d, body = %s", url, resp.StatusCode, wantStatus, string(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("Decode(response) error = %v", err)
	}
}

func readResponseBody(t *testing.T, resp *http.Response) (string, func()) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll(response body) error = %v", err)
	}
	return string(body), func() {
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("Close(response body) error = %v", err)
		}
	}
}

func postRaw(t *testing.T, url string, body string, wantStatus int) {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("Post(%s) error = %v", url, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("Close(response body) error = %v", err)
		}
	}()
	if resp.StatusCode != wantStatus {
		t.Fatalf("Post(%s) status = %d, want %d", url, resp.StatusCode, wantStatus)
	}
}
