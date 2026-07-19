package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestApplySecurityHeaders(t *testing.T) {
	h := make(http.Header)
	applySecurityHeaders(h)
	checks := map[string]string{
		"X-Content-Type-Options":     "nosniff",
		"X-Frame-Options":            "DENY",
		"Referrer-Policy":            "no-referrer",
		"Permissions-Policy":         "camera=(), microphone=(), geolocation=()",
		"Cross-Origin-Opener-Policy": "same-origin",
	}
	for k, want := range checks {
		if got := h.Get(k); got != want {
			t.Fatalf("%s = %q, want %q", k, got, want)
		}
	}
	csp := h.Get("Content-Security-Policy")
	for _, part := range []string{"default-src 'self'", "frame-ancestors 'none'", "object-src 'none'", "script-src 'self'"} {
		if !strings.Contains(csp, part) {
			t.Fatalf("CSP missing %q: %q", part, csp)
		}
	}
}

func TestHTTPSecurityHeadersOnHealthz(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz error = %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q", got)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := resp.Header.Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q", got)
	}
	if got := resp.Header.Get("Content-Security-Policy"); got == "" {
		t.Fatal("missing Content-Security-Policy")
	}
	if got := resp.Header.Get("X-Request-ID"); got == "" {
		t.Fatal("missing X-Request-ID")
	}
}
