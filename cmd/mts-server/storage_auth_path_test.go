package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPStorageAndAuthReportPath(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	t.Cleanup(server.Close)

	adminToken := loginHTTPUser(t, server.URL, "admin", "admin")
	headers := map[string]string{"Authorization": "Bearer " + adminToken}

	var snap storageSnapshotResponse
	postJSONWithHeaders(t, server.URL+routeAdminStorageSnapshot, emptyRequest{}, headers, http.StatusOK, &snap)
	if !snap.OK || snap.Path == "" {
		t.Fatalf("create snapshot = %+v", snap)
	}

	var listed storageSnapshotsResponse
	getJSONWithHeaders(t, server.URL+routeAdminStorageSnapshots, headers, http.StatusOK, &listed)
	if listed.Path != routeAdminStorageSnapshots {
		t.Fatalf("list snapshots path=%q", listed.Path)
	}
	if len(listed.Snapshots) == 0 {
		t.Fatal("expected at least one snapshot")
	}
	name := listed.Snapshots[0].Name

	var deleted okResponse
	resp := doHTTP(t, http.MethodDelete, server.URL+routeAdminStorageSnapshots+"?name="+name, emptyRequest{}, headers)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete status=%d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&deleted); err != nil {
		t.Fatalf("decode delete: %v", err)
	}
	if !deleted.OK || deleted.Path != routeAdminStorageSnapshots {
		t.Fatalf("delete = %+v", deleted)
	}

	var dataListed storageDataSnapshotsResponse
	getJSONWithHeaders(t, server.URL+routeAdminStorageDataSnapshots, headers, http.StatusOK, &dataListed)
	if dataListed.Path != routeAdminStorageDataSnapshots {
		t.Fatalf("data snapshots path=%q", dataListed.Path)
	}

	var export storageExportResponse
	getJSONWithHeaders(t, server.URL+routeAdminStorageExport, headers, http.StatusOK, &export)
	if export.Path != routeAdminStorageExport {
		t.Fatalf("export path=%q", export.Path)
	}

	var logout okResponse
	postJSONWithHeaders(t, server.URL+routeAuthLogout, emptyRequest{}, headers, http.StatusOK, &logout)
	if !logout.OK || logout.Path != routeAuthLogout {
		t.Fatalf("logout = %+v", logout)
	}
}
