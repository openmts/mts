package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPOpsConfigStatsReportPath(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	var maint maintenanceStatsResponse
	getJSONWithHeaders(t, server.URL+routeAdminStatsMaintenance, nil, http.StatusOK, &maint)
	if maint.Path != routeAdminStatsMaintenance {
		t.Fatalf("maintenance stats path = %q", maint.Path)
	}

	var ops opsStatusResponse
	getJSONWithHeaders(t, server.URL+routeAdminOpsStatus, nil, http.StatusOK, &ops)
	if ops.Path != routeAdminOpsStatus {
		t.Fatalf("ops-status path = %q", ops.Path)
	}

	var mem storageMemoryResponse
	getJSONWithHeaders(t, server.URL+routeAdminStatsStorageMemory, nil, http.StatusOK, &mem)
	if mem.Path != routeAdminStatsStorageMemory {
		t.Fatalf("storage-memory path = %q", mem.Path)
	}

	var compact compactionStatsResponse
	getJSONWithHeaders(t, server.URL+routeAdminStatsCompaction, nil, http.StatusOK, &compact)
	if compact.Path != routeAdminStatsCompaction {
		t.Fatalf("compaction path = %q", compact.Path)
	}

	var errs maintenanceErrorsResponse
	getJSONWithHeaders(t, server.URL+routeAdminMaintenanceErrors, nil, http.StatusOK, &errs)
	if errs.Path != routeAdminMaintenanceErrors {
		t.Fatalf("maintenance errors path = %q", errs.Path)
	}

	var cfg configResponse
	getJSONWithHeaders(t, server.URL+routeAdminConfigEffective, nil, http.StatusOK, &cfg)
	if cfg.Path != routeAdminConfigEffective {
		t.Fatalf("config path = %q", cfg.Path)
	}

	var schema configSchemaResponse
	getJSONWithHeaders(t, server.URL+routeAdminConfigSchema, nil, http.StatusOK, &schema)
	if schema.Path != routeAdminConfigSchema {
		t.Fatalf("config schema path = %q", schema.Path)
	}

	var spec apiSpecResponse
	getJSONWithHeaders(t, server.URL+routeAdminAPISpec, nil, http.StatusOK, &spec)
	if spec.Path != routeAdminAPISpec || spec.Version == "" {
		t.Fatalf("api-spec = %+v", spec)
	}

	var codes errorCodesResponse
	getJSONWithHeaders(t, server.URL+routeAdminErrorCodes, nil, http.StatusOK, &codes)
	if codes.Path != routeAdminErrorCodes || len(codes.Codes) == 0 {
		t.Fatalf("error-codes = %+v", codes)
	}
}
