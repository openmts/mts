package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mts "github.com/openmts/mts"
)

func TestHTTPUsersAndDoctorReportPath(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	t.Cleanup(server.Close)

	seedUserWithPassword(t, runtime, mts.User{Name: "path_user", Role: mts.UserRoleUser}, "secret12")
	adminToken := loginHTTPUser(t, server.URL, "admin", "admin")
	headers := map[string]string{"Authorization": "Bearer " + adminToken}

	var created okResponse
	postJSONWithHeaders(t, server.URL+routeUsers, createUserRequest{
		User:     mts.User{Name: "path_new", Role: mts.UserRoleUser},
		Password: "secret12",
	}, headers, http.StatusOK, &created)
	if !created.OK || created.Path != routeUsers {
		t.Fatalf("create user = %+v", created)
	}

	var listed usersResponse
	getJSONWithHeaders(t, server.URL+routeUsers, headers, http.StatusOK, &listed)
	if listed.Path != routeUsers {
		t.Fatalf("list users path=%q", listed.Path)
	}

	var updated okResponse
	putJSONWithHeaders(t, server.URL+routeUsersPrefix+"path_user", mts.User{
		Name: "path_user", Role: mts.UserRoleUser, Disabled: true,
	}, headers, http.StatusOK, &updated)
	if !updated.OK || updated.Path != routeUsersPrefix+"path_user" {
		t.Fatalf("update user = %+v", updated)
	}

	var pwd setPasswordResponse
	putJSONWithHeaders(t, server.URL+routeUsersPrefix+"path_user/password", passwordRequest{
		Password: "secret99x",
	}, headers, http.StatusOK, &pwd)
	if !pwd.OK || pwd.Path != routeUsersPrefix+"path_user/password" || pwd.UserName != "path_user" {
		t.Fatalf("set password = %+v", pwd)
	}

	var grants databasePermissionsResponse
	getJSONWithHeaders(t, server.URL+routeUsersPrefix+"path_user/database-permissions", headers, http.StatusOK, &grants)
	if grants.Path != routeUsersPrefix+"path_user/database-permissions" {
		t.Fatalf("list grants path=%q", grants.Path)
	}

	var granted okResponse
	postJSONWithHeaders(t, server.URL+routeUsersPrefix+"path_user/database-permissions/default/read", emptyRequest{}, headers, http.StatusOK, &granted)
	if !granted.OK || !strings.Contains(granted.Path, "/database-permissions/") {
		t.Fatalf("grant = %+v", granted)
	}

	var revoked okResponse
	resp := doHTTP(t, http.MethodDelete, server.URL+routeUsersPrefix+"path_user/database-permissions/default/read", emptyRequest{}, headers)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("revoke status=%d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&revoked); err != nil {
		t.Fatalf("decode revoke: %v", err)
	}
	if !revoked.OK || !strings.Contains(revoked.Path, "/database-permissions/") {
		t.Fatalf("revoke = %+v", revoked)
	}

	var batch batchMutationResponse
	postJSONWithHeaders(t, server.URL+routeUsersBatchDisabled, batchUserDisabledRequest{
		Names:    []string{"path_user"},
		Disabled: false,
	}, headers, http.StatusOK, &batch)
	if !batch.OK || batch.Path != routeUsersBatchDisabled {
		t.Fatalf("batch = %+v", batch)
	}

	req, err := http.NewRequest(http.MethodPost, server.URL+routeUsersBatchDisabled+"?stream=1", strings.NewReader(`{"names":["path_user"],"disabled":true}`))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", contentTypeNDJSON)
	streamResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer func() { _ = streamResp.Body.Close() }()
	if streamResp.StatusCode != http.StatusOK {
		t.Fatalf("stream status=%d", streamResp.StatusCode)
	}
	raw, err := io.ReadAll(streamResp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	var summary batchProgressEvent
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &summary); err != nil {
		t.Fatalf("summary json: %v", err)
	}
	if summary.Type != "summary" || summary.Path != routeUsersBatchDisabled {
		t.Fatalf("stream summary=%#v", summary)
	}

	var deleted okResponse
	delResp := doHTTP(t, http.MethodDelete, server.URL+routeUsersPrefix+"path_new", emptyRequest{}, headers)
	defer func() { _ = delResp.Body.Close() }()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("delete status=%d", delResp.StatusCode)
	}
	if err := json.NewDecoder(delResp.Body).Decode(&deleted); err != nil {
		t.Fatalf("decode delete: %v", err)
	}
	if !deleted.OK || deleted.Path != routeUsersPrefix+"path_new" {
		t.Fatalf("delete = %+v", deleted)
	}

	var doctor doctorResponse
	getJSONWithHeaders(t, server.URL+routeAdminDoctor, headers, http.StatusOK, &doctor)
	if doctor.Path != routeAdminDoctor {
		t.Fatalf("doctor path=%q", doctor.Path)
	}

	var health adminHealthResponse
	getJSONWithHeaders(t, server.URL+routeAdminHealth, headers, http.StatusOK, &health)
	if health.Path != routeAdminHealth {
		t.Fatalf("health path=%q", health.Path)
	}
}
