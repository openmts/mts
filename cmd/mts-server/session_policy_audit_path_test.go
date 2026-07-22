package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mts "github.com/openmts/mts"
)

func TestHTTPSessionPolicyAuthzAuditReportPath(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	t.Cleanup(server.Close)

	var policy passwordPolicyResponse
	getJSONWithHeaders(t, server.URL+routeAuthPasswordPolicy, nil, http.StatusOK, &policy)
	if !policy.OK || policy.Path != routeAuthPasswordPolicy {
		t.Fatalf("password policy = %+v", policy)
	}

	adminToken := loginHTTPUser(t, server.URL, "admin", "admin")
	headers := map[string]string{"Authorization": "Bearer " + adminToken}

	// explicit login response path
	var login authTokenResponse
	postJSONWithHeaders(t, server.URL+routeAuthLogin, loginRequest{UserName: "admin", Password: "admin"}, nil, http.StatusOK, &login)
	if login.Path != routeAuthLogin || strings.TrimSpace(login.Token.Token) == "" {
		t.Fatalf("login = path=%q token=%q", login.Path, login.Token.Token)
	}

	var session sessionResponse
	getJSONWithHeaders(t, server.URL+routeAuthSession, headers, http.StatusOK, &session)
	if !session.OK || session.Path != routeAuthSession {
		t.Fatalf("session = %+v", session)
	}

	var authz authzDatabaseCheckResponse
	postJSONWithHeaders(t, server.URL+routeAuthzDatabaseCheck, map[string]any{
		"user_name":  "admin",
		"database":   "default",
		"permission": "read",
	}, headers, http.StatusOK, &authz)
	if authz.Path != routeAuthzDatabaseCheck {
		t.Fatalf("authz = %+v", authz)
	}

	var adminAudit auditListResponse
	getJSONWithHeaders(t, server.URL+routeAdminAudit, headers, http.StatusOK, &adminAudit)
	if adminAudit.Path != routeAdminAudit {
		t.Fatalf("admin audit path=%q", adminAudit.Path)
	}

	var selfAudit userAuditResponse
	getJSONWithHeaders(t, server.URL+routeUsersPrefix+"admin/audit", headers, http.StatusOK, &selfAudit)
	if !strings.HasSuffix(selfAudit.Path, "/audit") {
		t.Fatalf("self audit path=%q", selfAudit.Path)
	}

	seedUserWithPassword(t, runtime, mts.User{Name: "path_reader", Role: mts.UserRoleUser}, "secret12")
	var login2 authTokenResponse
	postJSONWithHeaders(t, server.URL+routeAuthLogin, loginRequest{UserName: "path_reader", Password: "secret12"}, nil, http.StatusOK, &login2)
	if login2.Path != routeAuthLogin {
		t.Fatalf("user login path=%q", login2.Path)
	}
}
