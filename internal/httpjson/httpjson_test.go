package httpjson

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeStrictRejectsUnknownFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"mts","extra":true}`))
	var out struct {
		Name string `json:"name"`
	}
	if err := DecodeStrict(req, &out); err == nil {
		t.Fatal("DecodeStrict() error = nil, want unknown field error")
	}
}

func TestWriteSetsJSONContentTypeAndStatus(t *testing.T) {
	recorder := httptest.NewRecorder()
	Write(recorder, http.StatusAccepted, map[string]bool{"ok": true})
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if body := strings.TrimSpace(recorder.Body.String()); body != `{"ok":true}` {
		t.Fatalf("body = %q, want JSON object", body)
	}
}

func TestWriteRawSetsContentTypeAndStatus(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteRaw(recorder, http.StatusOK, "text/plain", []byte("mts"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/plain" {
		t.Fatalf("Content-Type = %q, want text/plain", got)
	}
	if body := recorder.Body.String(); body != "mts" {
		t.Fatalf("body = %q, want mts", body)
	}
}
