package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPAdminConfigStorageVersionReportPath(t *testing.T) {
	runtime := openTestRuntime(t)
	runtime.config.ConfigPath = writeRuntimeConfig(t, runtime.currentConfig())
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	var validate configValidateResponse
	postJSONWithHeaders(t, server.URL+routeAdminConfigValidate, configValidateRequest{Config: runtime.currentConfig()}, nil, http.StatusOK, &validate)
	if !validate.OK || validate.Path != routeAdminConfigValidate {
		t.Fatalf("validate = %+v", validate)
	}

	var reload reloadConfigResponse
	postJSONWithHeaders(t, server.URL+routeAdminConfigReload, emptyRequest{}, nil, http.StatusOK, &reload)
	if !reload.OK || reload.Path != routeAdminConfigReload {
		t.Fatalf("reload = %+v", reload)
	}

	var storage storageValidateResponse
	postJSONWithHeaders(t, server.URL+routeAdminStorageValidate, emptyRequest{}, nil, http.StatusOK, &storage)
	if storage.Path != routeAdminStorageValidate {
		t.Fatalf("storage validate path = %q", storage.Path)
	}

	var ver versionResponse
	getJSONWithHeaders(t, server.URL+routeAdminVersion, nil, http.StatusOK, &ver)
	if ver.Path != routeAdminVersion || ver.Version == "" {
		t.Fatalf("version = %+v", ver)
	}
}
