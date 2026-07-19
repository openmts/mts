package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mts "github.com/openmts/mts"
)

// TestCommercialDashboardSmoke 覆盖可商用后台最小闭环：
// SPA 可达 + 安全头 + 登录 + 写/查 API + 管理库列表。
func TestCommercialDashboardSmoke(t *testing.T) {
	runtime := openTestRuntime(t)
	// 显式验证强制改密闭环（openTestRuntime 默认清掉该标记以免干扰其它单测）。
	ctx := context.Background()
	user, ok, err := runtime.engine.GetUser(ctx, "admin")
	if err != nil || !ok {
		t.Fatalf("GetUser(admin) ok=%v err=%v", ok, err)
	}
	user.Metadata = withMustChangePassword(user.Metadata, true)
	if err := runtime.engine.UpdateUser(ctx, user); err != nil {
		t.Fatalf("UpdateUser must_change error = %v", err)
	}
	server := httptest.NewServer(runtime.httpHandler())
	t.Cleanup(server.Close)

	// 1) SPA shell + security headers
	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET / error = %v", err)
	}
	body, closeBody := readResponseBody(t, resp)
	closeBody()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d body=%s", resp.StatusCode, body)
	}
	assertCommercialSecurityHeaders(t, resp.Header)

	// SPA fallback for client routes
	for _, path := range []string{"/query", "/write", "/operations"} {
		resp, err = http.Get(server.URL + path)
		if err != nil {
			t.Fatalf("GET %s error = %v", path, err)
		}
		_, closeBody = readResponseBody(t, resp)
		closeBody()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d", path, resp.StatusCode)
		}
	}

	// 2) health/ready
	for _, path := range []string{"/healthz", "/readyz"} {
		resp, err = http.Get(server.URL + path)
		if err != nil {
			t.Fatalf("GET %s error = %v", path, err)
		}
		_, closeBody = readResponseBody(t, resp)
		closeBody()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d", path, resp.StatusCode)
		}
	}

	// 3) login with bootstrapped admin，并完成强制改密
	var login authTokenResponse
	postJSONWithHeaders(t, server.URL+"/api/v1/auth/login", loginRequest{
		UserName: "admin",
		Password: "admin",
	}, nil, http.StatusOK, &login)
	if !login.MustChangePassword {
		t.Fatal("bootstrap admin should require password change")
	}
	authBoot := map[string]string{"Authorization": "Bearer " + login.Token.Token}
	postJSONWithHeaders(t, server.URL+"/api/v1/auth/password", changePasswordRequest{
		UserName:    "admin",
		OldPassword: "admin",
		NewPassword: "admin-commercial",
	}, authBoot, http.StatusOK, &okResponse{})
	token := loginHTTPUser(t, server.URL, "admin", "admin-commercial")
	auth := map[string]string{"Authorization": "Bearer " + token}

	// 4) write + query (sync)
	postJSONWithHeaders(t, server.URL+"/api/v1/data/write", writeRequest{
		Points: []mts.Point{testPoint()},
		Options: mts.WriteOptions{
			Sync: true,
		},
	}, auth, http.StatusOK, &writeResponse{})

	var rows queryRowsResponse
	postJSONWithHeaders(t, server.URL+"/api/v1/data/query/rows", queryRowsRequest{
		Query: testQuery(),
	}, auth, http.StatusOK, &rows)
	if len(rows.Rows) != 1 {
		t.Fatalf("query rows = %d, want 1", len(rows.Rows))
	}

	// 5) admin list databases
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/databases", auth, http.StatusOK, &measurementsResponse{})

	// 6) flush op (operations surface)
	postJSONWithHeaders(t, server.URL+"/api/v1/admin/flush", map[string]any{}, auth, http.StatusOK, &okResponse{})
}

func assertCommercialSecurityHeaders(t *testing.T, h http.Header) {
	t.Helper()
	if h.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("X-Frame-Options = %q", h.Get("X-Frame-Options"))
	}
	if h.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", h.Get("X-Content-Type-Options"))
	}
	if h.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q", h.Get("Referrer-Policy"))
	}
	csp := h.Get("Content-Security-Policy")
	if csp == "" || !strings.Contains(csp, "default-src 'self'") {
		t.Fatalf("CSP = %q", csp)
	}
	if h.Get("X-Request-ID") == "" {
		t.Fatal("missing X-Request-ID")
	}
}
