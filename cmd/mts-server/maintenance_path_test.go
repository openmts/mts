package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPMaintenanceOpsReportPath(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	var flush maintenanceResponse
	postJSONWithHeaders(t, server.URL+routeAdminFlush, emptyRequest{}, nil, http.StatusOK, &flush)
	if !flush.OK || flush.Path != routeAdminFlush {
		t.Fatalf("flush = %+v", flush)
	}

	var compact maintenanceResponse
	postJSONWithHeaders(t, server.URL+routeAdminCompact, emptyRequest{}, nil, http.StatusOK, &compact)
	if !compact.OK || compact.Path != routeAdminCompact {
		t.Fatalf("compact = %+v", compact)
	}

	var retention okResponse
	postJSONWithHeaders(t, server.URL+routeAdminRetentionApply, emptyRequest{}, nil, http.StatusOK, &retention)
	if !retention.OK || retention.Path != routeAdminRetentionApply {
		t.Fatalf("retention = %+v", retention)
	}
}
