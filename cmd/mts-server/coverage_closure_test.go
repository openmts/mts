package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	mts "github.com/openmts/mts"
)

func TestHTTPAuthLogoutBranches(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	seedUserWithPassword(t, runtime, mts.User{Name: "logout-user"}, "secret12")
	token := loginHTTPUser(t, server.URL, "logout-user", "secret12")

	postJSON(t, server.URL+"/api/v1/auth/logout", logoutRequest{}, http.StatusUnauthorized, &errorResponse{})
	postJSON(t, server.URL+"/api/v1/auth/logout", logoutRequest{Token: "bad-token"}, http.StatusUnauthorized, &errorResponse{})
	postJSONWithHeaders(
		t,
		server.URL+"/api/v1/auth/logout",
		emptyRequest{},
		map[string]string{"Authorization": "Bearer " + token},
		http.StatusOK,
		&okResponse{},
	)
	postJSONWithHeaders(
		t,
		server.URL+"/api/v1/data/write",
		writeRequest{Points: []mts.Point{testPoint()}},
		map[string]string{"Authorization": "Bearer " + token},
		http.StatusUnauthorized,
		&errorResponse{},
	)
}

func TestGRPCLogoutBranches(t *testing.T) {
	runtime := openTestRuntime(t)
	conn := openBufconnClient(t, runtime)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	}()

	ctx := context.Background()
	seedUserWithPassword(t, runtime, mts.User{Name: "grpc-logout"}, "secret12")
	var login authTokenResponse
	invokeOK(t, ctx, conn, "Login", &loginRequest{UserName: "grpc-logout", Password: "secret12"}, &login)

	err := invokeGRPC(ctx, conn, "Logout", &logoutRequest{}, &okResponse{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("Logout(empty) code = %v, want Unauthenticated, err=%v", status.Code(err), err)
	}
	err = invokeGRPC(ctx, conn, "Logout", &logoutRequest{Token: "bad-token"}, &okResponse{})
	if status.Code(err) != codes.OK {
		t.Fatalf("Logout(bad token) code = %v, want OK, err=%v", status.Code(err), err)
	}
	tokenCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+login.Token.Token))
	invokeOK(t, tokenCtx, conn, "Logout", &logoutRequest{}, &okResponse{})
	err = invokeGRPC(tokenCtx, conn, "Write", &writeRequest{Points: []mts.Point{testPoint()}}, &writeResponse{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("Write(revoked token) code = %v, want Unauthenticated, err=%v", status.Code(err), err)
	}
}

func TestHTTPDashboardRoutes(t *testing.T) {
	handler := dashboardHandler("/")
	for _, tt := range []struct {
		name       string
		path       string
		wantStatus int
		wantCache  bool
	}{
		{name: "root", path: "/", wantStatus: http.StatusOK},
		{name: "spa fallback", path: "/dashboard/users", wantStatus: http.StatusOK},
		{name: "api not found", path: "/api/missing", wantStatus: http.StatusNotFound},
		{name: "asset cache", path: firstDashboardAssetPath(t, ".css"), wantStatus: http.StatusOK, wantCache: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
			if resp.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.Code, tt.wantStatus)
			}
			if tt.wantCache && resp.Header().Get("Cache-Control") == "" {
				t.Fatal("Cache-Control header is empty")
			}
		})
	}
}

func TestHTTPAdminDatabaseAndMetadataBranches(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/admin/databases", databaseRequest{Name: "metrics"}, http.StatusOK, &okResponse{})
	postJSON(t, server.URL+"/api/v1/admin/databases", databaseRequest{Name: ""}, http.StatusBadRequest, &errorResponse{})
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/databases", nil, http.StatusOK, &measurementsResponse{})
	resp := doHTTP(t, http.MethodPatch, server.URL+"/api/v1/admin/databases", emptyRequest{}, nil)
	closeHTTPResponse(t, resp, http.StatusBadRequest)

	postJSON(
		t,
		server.URL+"/api/v1/admin/databases/metrics/retention-policies",
		retentionPolicyRequest{Policy: mts.RetentionPolicy{Name: "short"}},
		http.StatusOK,
		&okResponse{},
	)
	getJSONWithHeaders(
		t,
		server.URL+"/api/v1/admin/databases/metrics/retention-policies",
		nil,
		http.StatusOK,
		&retentionPoliciesResponse{},
	)
	deleteHTTP(t, server.URL+"/api/v1/admin/databases/metrics", http.StatusOK)
	getJSONWithHeaders(
		t,
		server.URL+"/api/v1/admin/databases/metrics/retention-policies",
		nil,
		http.StatusOK,
		&retentionPoliciesResponse{},
	)
}

func TestHTTPDashboardSubpathRoutes(t *testing.T) {
	handler := dashboardHandler("/mts/")
	for _, tt := range []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "base root", path: "/mts/", wantStatus: http.StatusOK},
		{name: "spa under base", path: "/mts/users", wantStatus: http.StatusOK},
		{name: "outside base", path: "/other", wantStatus: http.StatusNotFound},
		{name: "asset under base", path: "/mts" + firstDashboardAssetPath(t, ".css"), wantStatus: http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
			if resp.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d body=%s", resp.Code, tt.wantStatus, resp.Body.String())
			}
		})
	}
}

func firstDashboardAssetPath(t *testing.T, suffix string) string {
	t.Helper()
	entries, err := dashboardFS.ReadDir("dashboard-dist/assets")
	if err != nil {
		t.Fatalf("ReadDir(dashboard assets) error = %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), suffix) {
			return "/assets/" + entry.Name()
		}
	}
	t.Fatalf("dashboard asset with suffix %q not found", suffix)
	return ""
}

func TestHTTPDataMetadataBranches(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/data/write", writeRequest{Points: []mts.Point{testPoint()}}, http.StatusOK, &writeResponse{})
	getJSONWithHeaders(
		t,
		server.URL+"/api/v1/data/databases/default/measurements",
		nil,
		http.StatusOK,
		&measurementsResponse{},
	)
	getJSONWithHeaders(
		t,
		server.URL+"/api/v1/data/databases/default/measurements/cpu/fields",
		nil,
		http.StatusOK,
		&fieldsResponse{},
	)
	getJSONWithHeaders(
		t,
		server.URL+"/api/v1/data/databases/default/measurements/cpu/series?host=server-1",
		nil,
		http.StatusOK,
		&seriesResponse{},
	)
	getJSONWithHeaders(
		t,
		server.URL+"/api/v1/data/databases/default/measurements/cpu/unknown",
		nil,
		http.StatusNotFound,
		&errorResponse{},
	)
	resp := doHTTP(t, http.MethodPost, server.URL+"/api/v1/data/databases/default/measurements", emptyRequest{}, nil)
	closeHTTPResponse(t, resp, http.StatusMethodNotAllowed)
}

func TestHTTPUserManagementErrorBranches(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+"/api/v1/users", mts.User{Name: "branch-user"}, http.StatusOK, &okResponse{})
	putJSONWithHeaders(
		t,
		server.URL+"/api/v1/users/branch-user/password",
		map[string]any{"password": 123},
		nil,
		http.StatusBadRequest,
		&errorResponse{},
	)
	putJSONWithHeaders(
		t,
		server.URL+"/api/v1/users/branch-user/password",
		passwordRequest{Password: "secret12"},
		nil,
		http.StatusOK,
		&okResponse{},
	)
	getJSONWithHeaders(
		t,
		server.URL+"/api/v1/users/branch-user/database-permissions",
		nil,
		http.StatusOK,
		&databasePermissionsResponse{},
	)
	resp := doHTTP(t, http.MethodPost, server.URL+"/api/v1/users/branch-user/database-permissions", emptyRequest{}, nil)
	closeHTTPResponse(t, resp, http.StatusBadRequest)
	resp = doHTTP(t, http.MethodPatch, server.URL+"/api/v1/users/branch-user/database-permissions/default/read", emptyRequest{}, nil)
	closeHTTPResponse(t, resp, http.StatusBadRequest)
	postJSON(
		t,
		server.URL+"/api/v1/authz/database/check",
		authzDatabaseCheckRequest{UserName: "branch-user", Database: "default", Permission: mts.DatabasePermissionRead},
		http.StatusOK,
		&authzDatabaseCheckResponse{},
	)
	postJSONWithHeaders(
		t,
		server.URL+"/api/v1/authz/database/check",
		map[string]any{"user_name": 123},
		nil,
		http.StatusBadRequest,
		&errorResponse{},
	)
	resp = doHTTP(t, http.MethodGet, server.URL+"/api/v1/authz/database/check", nil, nil)
	closeHTTPResponse(t, resp, http.StatusMethodNotAllowed)
}

func TestHTTPAdminConfigAndDownsampleBranches(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSONWithHeaders(
		t,
		server.URL+"/api/v1/admin/config/validate",
		map[string]any{"config": "bad"},
		nil,
		http.StatusBadRequest,
		&errorResponse{},
	)
	badConfig := runtime.currentConfig()
	badConfig.DataDir = ""
	postJSON(
		t,
		server.URL+"/api/v1/admin/config/validate",
		configValidateRequest{Config: badConfig},
		http.StatusBadRequest,
		&configValidateResponse{},
	)

	resp := doHTTP(t, http.MethodPatch, server.URL+"/api/v1/admin/downsample/policies", emptyRequest{}, nil)
	closeHTTPResponse(t, resp, http.StatusBadRequest)
	resp = doHTTP(t, http.MethodGet, server.URL+"/api/v1/admin/downsample/policies/missing", nil, nil)
	closeHTTPResponse(t, resp, http.StatusNotFound)
	postJSON(
		t,
		server.URL+"/api/v1/admin/downsample/policies/missing/unknown",
		emptyRequest{},
		http.StatusNotFound,
		&errorResponse{},
	)
	postJSONWithHeaders(
		t,
		server.URL+"/api/v1/admin/downsample/policies/missing/reset",
		map[string]any{"reset": "bad"},
		nil,
		http.StatusBadRequest,
		&errorResponse{},
	)
	getJSONWithHeaders(
		t,
		server.URL+"/api/v1/admin/downsample/statuses",
		nil,
		http.StatusOK,
		&downsampleStatusesResponse{},
	)
}

func TestHTTPChangePasswordErrorBranches(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	seedUserWithPassword(t, runtime, mts.User{Name: "change-user"}, "secret12")
	token := loginHTTPUser(t, server.URL, "change-user", "secret12")
	headers := map[string]string{"Authorization": "Bearer " + token}

	postJSONWithHeaders(
		t,
		server.URL+"/api/v1/auth/password",
		map[string]any{"user_name": 123},
		headers,
		http.StatusBadRequest,
		&errorResponse{},
	)
	postJSONWithHeaders(
		t,
		server.URL+"/api/v1/auth/password",
		changePasswordRequest{UserName: "change-user", OldPassword: "wrong", NewPassword: "nextpass1"},
		headers,
		http.StatusBadRequest,
		&errorResponse{},
	)
	// 旧密码错误不得撤销当前会话 token
	var session sessionResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/auth/session", headers, http.StatusOK, &session)
	if !session.OK || session.UserName != "change-user" {
		t.Fatalf("session after wrong password = %+v", session)
	}
	resp := doHTTP(t, http.MethodGet, server.URL+"/api/v1/auth/password", nil, headers)
	closeHTTPResponse(t, resp, http.StatusMethodNotAllowed)
}

func TestGRPCDatabaseAndRequestFactoryBranches(t *testing.T) {
	runtime := openTestRuntime(t)
	conn := openBufconnClient(t, runtime)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	}()

	ctx := context.Background()
	invokeOK(t, ctx, conn, "CreateDatabase", &databaseRequest{Name: "grpc-drop"}, &okResponse{})
	invokeOK(t, ctx, conn, "DropDatabase", &databaseRequest{Name: "grpc-drop"}, &okResponse{})
	if got := newGRPCRequest(emptyRequest{}); got != (emptyRequest{}) {
		t.Fatalf("newGRPCRequest(non-pointer) = %#v, want emptyRequest", got)
	}
}

func TestAdminUserBearerRejectsDisabledAndMissingUsers(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	seedUserWithPassword(t, runtime, mts.User{Name: "disabled-admin", Role: mts.UserRoleAdmin}, "secret12")
	token := loginHTTPUser(t, server.URL, "disabled-admin", "secret12")
	ctx := context.Background()
	if err := runtime.engine.UpdateUser(ctx, mts.User{Name: "disabled-admin", Role: mts.UserRoleAdmin, Disabled: true}); err != nil {
		t.Fatalf("UpdateUser(disable admin) error = %v", err)
	}
	headers := map[string]string{"Authorization": "Bearer " + token}
	getJSONWithHeaders(t, server.URL+"/api/v1/users", headers, http.StatusUnauthorized, &errorResponse{})
	if err := runtime.requireUserRole(ctx, "disabled-admin", mts.UserRoleAdmin); err != mts.ErrPermissionDenied {
		t.Fatalf("requireUserRole(disabled) error = %v, want ErrPermissionDenied", err)
	}

	if err := runtime.engine.DeleteUser(ctx, "disabled-admin"); err != nil {
		t.Fatalf("DeleteUser(disabled admin) error = %v", err)
	}
	getJSONWithHeaders(t, server.URL+"/api/v1/users", headers, http.StatusUnauthorized, &errorResponse{})
	if err := runtime.requireUserRole(ctx, "disabled-admin", mts.UserRoleAdmin); err != mts.ErrPermissionDenied {
		t.Fatalf("requireUserRole(missing) error = %v, want ErrPermissionDenied", err)
	}
}

func TestHTTPSmallPublicAdminBranches(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	resp := doHTTP(t, http.MethodGet, server.URL+"/api/v1/auth/login", nil, nil)
	closeHTTPResponse(t, resp, http.StatusMethodNotAllowed)
	postJSONWithHeaders(
		t,
		server.URL+"/api/v1/auth/login",
		map[string]any{"user_name": 123},
		nil,
		http.StatusBadRequest,
		&errorResponse{},
	)
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/config/schema", nil, http.StatusOK, &configSchemaResponse{})
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/error-codes", nil, http.StatusOK, &errorCodesResponse{})
}

func closeHTTPResponse(t *testing.T, resp *http.Response, wantStatus int) {
	t.Helper()
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("Close(response body) error = %v", err)
		}
	}()
	if resp.StatusCode != wantStatus {
		t.Fatalf("status = %d, want %d", resp.StatusCode, wantStatus)
	}
}
