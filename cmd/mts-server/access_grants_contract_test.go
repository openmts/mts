package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	mts "github.com/openmts/mts"
)

func TestParseAccessGrantsPageRequest(t *testing.T) {
	tests := []struct {
		name           string
		rawQuery       string
		expectedCursor string
		expectedLimit  int
		expectedError  bool
	}{
		{name: "defaults", expectedLimit: 100},
		{name: "minimum", rawQuery: "limit=1", expectedLimit: 1},
		{name: "maximum and cursor", rawQuery: "limit=200&cursor=alice", expectedCursor: "alice", expectedLimit: 200},
		{name: "zero", rawQuery: "limit=0", expectedError: true},
		{name: "over maximum", rawQuery: "limit=201", expectedError: true},
		{name: "not a number", rawQuery: "limit=many", expectedError: true},
		{name: "cursor surrounding whitespace", rawQuery: "cursor=%20alice%20", expectedError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, routeUsersAccessGrants+"?"+test.rawQuery, nil)
			got, err := parseAccessGrantsPageRequest(request)
			if test.expectedError {
				if err == nil {
					t.Fatalf("parseAccessGrantsPageRequest() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseAccessGrantsPageRequest() error = %v", err)
			}
			if got.Cursor != test.expectedCursor || got.Limit != test.expectedLimit {
				t.Fatalf("request = %+v, want cursor=%q limit=%d", got, test.expectedCursor, test.expectedLimit)
			}
		})
	}
}

func TestAccessGrantsContractPaginatesAndPreservesExistingRoutes(t *testing.T) {
	runtime := openTestRuntimeRequireUser(t)
	seedUserWithPassword(t, runtime, mts.User{Name: "alice"}, "secret12")
	seedUserWithPassword(t, runtime, mts.User{Name: "bob", Disabled: true}, "secret34")
	seedDatabasePermission(t, runtime, "alice", "metrics", mts.DatabasePermissionRead)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	adminToken := loginHTTPUser(t, server.URL, "admin", "admin")
	adminHeaders := map[string]string{"Authorization": "Bearer " + adminToken}

	var first accessGrantsResponse
	getJSONWithHeaders(t, server.URL+routeUsersAccessGrants+"?limit=2", adminHeaders, http.StatusOK, &first)
	if first.Path != routeUsersAccessGrants || first.TotalUsers != 3 || first.NextCursor != "alice" {
		t.Fatalf("first page meta = %+v, want path total=3 next=alice", first)
	}
	if len(first.Items) != 2 || first.Items[0].User.Name != "admin" || first.Items[1].User.Name != "alice" {
		t.Fatalf("first items = %#v, want admin/alice", first.Items)
	}
	if len(first.Items[0].Grants) != 0 {
		t.Fatalf("admin grants = %#v, want empty", first.Items[0].Grants)
	}
	if len(first.Items[1].Grants) != 1 || first.Items[1].Grants[0].Permission != mts.DatabasePermissionRead {
		t.Fatalf("alice grants = %#v, want read", first.Items[1].Grants)
	}

	var second accessGrantsResponse
	getJSONWithHeaders(t, server.URL+routeUsersAccessGrants+"?limit=2&cursor=alice", adminHeaders, http.StatusOK, &second)
	if len(second.Items) != 1 || second.Items[0].User.Name != "bob" || !second.Items[0].User.Disabled || second.NextCursor != "" {
		t.Fatalf("second page = %+v, want disabled bob and no next", second)
	}

	getJSONWithHeaders(t, server.URL+routeUsers, adminHeaders, http.StatusOK, &usersResponse{})
	getJSONWithHeaders(t, server.URL+routeUsersPrefix+"alice/database-permissions", adminHeaders, http.StatusOK, &databasePermissionsResponse{})
}

func TestAccessGrantsContractRequiresAdmin(t *testing.T) {
	runtime := openTestRuntimeRequireUser(t)
	seedUserWithPassword(t, runtime, mts.User{Name: "alice"}, "secret12")
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	getJSONWithHeaders(t, server.URL+routeUsersAccessGrants, nil, http.StatusUnauthorized, &errorResponse{})
	userToken := loginHTTPUser(t, server.URL, "alice", "secret12")
	getJSONWithHeaders(t, server.URL+routeUsersAccessGrants, map[string]string{
		"Authorization": "Bearer " + userToken,
	}, http.StatusForbidden, &errorResponse{})

	adminToken := loginHTTPUser(t, server.URL, "admin", "admin")
	getJSONWithHeaders(t, server.URL+routeUsersAccessGrants, map[string]string{
		"Authorization": "Bearer " + adminToken,
	}, http.StatusOK, &accessGrantsResponse{})
}
