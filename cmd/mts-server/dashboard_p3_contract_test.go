package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func TestHTTPAdminAuditFilters(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	runtime.audit.record(auditEvent{UserName: "alice", Action: "login", Time: time.Unix(1_700_000_000, 0).UTC()})
	runtime.audit.record(auditEvent{UserName: "bob", Action: "flush", Time: time.Unix(1_700_000_100, 0).UTC()})
	runtime.audit.record(auditEvent{UserName: "alice", Action: "create_database", Database: "metrics", Time: time.Unix(1_700_000_200, 0).UTC()})

	headers := map[string]string{"X-MTS-Admin-Token": runtime.config.Auth.AdminToken}
	var all auditListResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/audit", headers, http.StatusOK, &all)
	if all.Total < 3 {
		t.Fatalf("total=%d events=%#v", all.Total, all.Events)
	}
	var filtered auditListResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/audit?user_name=alice&action=create", headers, http.StatusOK, &filtered)
	if len(filtered.Events) != 1 || filtered.Events[0].Action != "create_database" {
		t.Fatalf("filtered=%#v", filtered.Events)
	}
}

func TestHTTPStorageSnapshotListDelete(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()
	headers := map[string]string{"X-MTS-Admin-Token": runtime.config.Auth.AdminToken}

	var created storageSnapshotResponse
	postJSONWithHeaders(t, server.URL+"/api/v1/admin/storage/snapshot", emptyRequest{}, headers, http.StatusOK, &created)
	if !created.OK || created.Path == "" {
		t.Fatalf("snapshot=%#v", created)
	}
	var listed storageSnapshotsResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/storage/snapshots", headers, http.StatusOK, &listed)
	if len(listed.Snapshots) < 1 {
		t.Fatalf("snapshots=%#v", listed.Snapshots)
	}
	name := filepath.Base(created.Path)
	deleteHTTPWithHeaders(t, server.URL+"/api/v1/admin/storage/snapshots?name="+name, headers, http.StatusOK)
	if _, err := os.Stat(created.Path); !os.IsNotExist(err) {
		t.Fatalf("snapshot file still exists: %v", err)
	}
	_ = mts.User{}
}

func TestHTTPAuditPersistsToInternal(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()
	headers := map[string]string{"X-MTS-Admin-Token": runtime.config.Auth.AdminToken}

	runtime.audit.record(auditEvent{
		UserName: "persist-user",
		Action:   "flush",
		Detail:   "p3",
		Time:     time.Unix(1_700_100_000, 0).UTC(),
	})
	// 等待异步 persist
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.audit.engine == nil {
			break
		}
		// 直接清空内存环，只依赖持久化读回
		runtime.audit.mu.Lock()
		runtime.audit.events = nil
		runtime.audit.mu.Unlock()
		var listed auditListResponse
		getJSONWithHeaders(t, server.URL+"/api/v1/admin/audit?user_name=persist-user&action=flush", headers, http.StatusOK, &listed)
		if len(listed.Events) > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	// 最终再查一次给出失败信息
	var listed auditListResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/audit?user_name=persist-user", headers, http.StatusOK, &listed)
	if len(listed.Events) == 0 {
		t.Fatalf("expected persisted audit events, got %#v", listed)
	}
}

func TestHTTPAPISpecHasNamespaces(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()
	headers := map[string]string{"X-MTS-Admin-Token": runtime.config.Auth.AdminToken}
	var spec apiSpecResponse
	getJSONWithHeaders(t, server.URL+"/api/v1/admin/api-spec", headers, http.StatusOK, &spec)
	if len(spec.Namespaces) == 0 {
		t.Fatalf("api-spec namespaces empty: %#v", spec)
	}
}

func TestHTTPAuthzSelfCheckAllowsCurrentUser(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	seedUserWithPassword(t, runtime, mts.User{Name: "authz-self", Role: mts.UserRoleUser}, "secret12")
	if err := runtime.engine.GrantDatabasePermission(context.Background(), "authz-self", "default", mts.DatabasePermissionRead); err != nil {
		t.Fatalf("grant error = %v", err)
	}
	token := loginHTTPUser(t, server.URL, "authz-self", "secret12")
	headers := map[string]string{"Authorization": "Bearer " + token}

	var allowed authzDatabaseCheckResponse
	postJSONWithHeaders(t, server.URL+"/api/v1/authz/database/check", authzDatabaseCheckRequest{
		Database: "default", Permission: mts.DatabasePermissionRead,
	}, headers, http.StatusOK, &allowed)
	if !allowed.Allowed {
		t.Fatal("self read check allowed=false")
	}

	// 禁止检查他人
	postJSONWithHeaders(t, server.URL+"/api/v1/authz/database/check", authzDatabaseCheckRequest{
		UserName: "admin", Database: "default", Permission: mts.DatabasePermissionRead,
	}, headers, http.StatusForbidden, &errorResponse{})
}
