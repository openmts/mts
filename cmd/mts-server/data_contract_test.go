package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPDataContractReportsLimitsAndFeatures(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	var resp dataContractResponse
	getJSONWithHeaders(t, server.URL+routeDataContract, nil, http.StatusOK, &resp)
	if resp.Path != routeDataContract {
		t.Fatalf("path=%q", resp.Path)
	}
	if resp.Version != 1 {
		t.Fatalf("version=%d", resp.Version)
	}
	if resp.MaxWritePoints <= 0 && resp.DefaultQueryLimit < 0 {
		t.Fatalf("unexpected limits: %#v", resp)
	}
	wantIDs := map[string]bool{
		"write_accepted_points": false,
		"write_response_mode":   false,
		"query_result_meta":     false,
		"query_stream_end_meta": false,
		"delete_response_meta":  false,
		"data_limits":           false,
	}
	for _, f := range resp.Features {
		if _, ok := wantIDs[f.ID]; ok {
			if !f.Enabled {
				t.Fatalf("feature %s disabled", f.ID)
			}
			wantIDs[f.ID] = true
		}
	}
	for id, seen := range wantIDs {
		if !seen {
			t.Fatalf("missing feature %s", id)
		}
	}
	_ = resp.AdminOpBusy
}
