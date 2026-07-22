package main

import (
	"errors"
	"net/http"
	"testing"
)

func TestErrorCodeSpecsCommercialMeta(t *testing.T) {
	t.Parallel()
	resp := errorCodeSpecs()
	if len(resp.Codes) < 7 {
		t.Fatalf("codes len = %d, want >= 7", len(resp.Codes))
	}
	seen := map[errorCode]errorCodeSpec{}
	for _, c := range resp.Codes {
		if c.Code == "" || c.Description == "" || c.Category == "" || c.Remediation == "" {
			t.Fatalf("incomplete spec: %+v", c)
		}
		if c.HTTPStatus == 0 || c.GRPCCode == "" {
			t.Fatalf("missing status mapping: %+v", c)
		}
		seen[c.Code] = c
	}
	exhausted, ok := seen[errorCodeResourceExhausted]
	if !ok || !exhausted.Retryable {
		t.Fatalf("resource_exhausted retryable = %+v", exhausted)
	}
	if exhausted.DashboardPath == "" {
		t.Fatal("resource_exhausted dashboard_path empty")
	}
	bad, ok := seen[errorCodeBadRequest]
	if !ok || bad.Retryable {
		t.Fatalf("bad_request retryable = %+v", bad)
	}
}

func TestAPIErrorResponseAttachesContractMeta(t *testing.T) {
	t.Parallel()
	status, resp := apiErrorResponse(newAPIError(errorCodePermissionDenied, "no grant", errors.New("no grant")))
	if status != http.StatusForbidden {
		t.Fatalf("status = %d", status)
	}
	if resp.Code != errorCodePermissionDenied {
		t.Fatalf("code = %s", resp.Code)
	}
	if resp.Category == "" || resp.Remediation == "" {
		t.Fatalf("meta missing: %+v", resp)
	}
	if resp.Retryable {
		t.Fatal("permission_denied should not be retryable by default")
	}

	_, busy := apiErrorResponse(newAdminHeavyBusyError("flush"))
	if !busy.Retryable || !busy.AdminOpBusy || busy.Op != "flush" {
		t.Fatalf("busy resp = %+v", busy)
	}
	if busy.Remediation == "" {
		t.Fatal("busy remediation empty")
	}
}
