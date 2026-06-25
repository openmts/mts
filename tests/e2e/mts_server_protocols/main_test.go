package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	mts "github.com/openmts/mts"
)

func TestRun(t *testing.T) {
	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}
}

func TestMainFunction(t *testing.T) {
	main()
}

func TestRunWithDirRejectsInvalidRepoRoot(t *testing.T) {
	if err := runWithDir(filepath.Join("bad\x00path")); err == nil {
		t.Fatal("runWithDir(invalid dir) error = nil, want error")
	}
}

func TestAssertRowsRejectsUnexpectedRows(t *testing.T) {
	if err := assertRows(nil, "cpu", "host", 0.1); err == nil {
		t.Fatal("assertRows(nil) error = nil, want error")
	}
	wrongIdentity := []mts.Row{{Measurement: "mem", Tags: map[string]string{"host": "a"}}}
	if err := assertRows(wrongIdentity, "cpu", "a", 0.1); err == nil {
		t.Fatal("assertRows(wrong identity) error = nil, want error")
	}
	wrongValue := []mts.Row{{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields:      map[string]mts.FieldValue{"usage": mts.Float64Value(0.2)},
	}}
	if err := assertRows(wrongValue, "cpu", "a", 0.1); err == nil {
		t.Fatal("assertRows(wrong value) error = nil, want error")
	}
}

func TestHTTPHelpersRejectBadResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/healthz" {
			http.Error(writer, "bad", http.StatusServiceUnavailable)
			return
		}
		_, _ = writer.Write([]byte("{"))
	}))
	defer server.Close()
	addr := server.Listener.Addr().String()
	client := http.Client{}
	if err := assertHTTPHealth(client, addr, "/healthz"); err == nil {
		t.Fatal("assertHTTPHealth(status) error = nil, want error")
	}
	if err := postHTTPJSON(client, addr, "/write", emptyRequest{}, &writeResponse{}); err == nil {
		t.Fatal("postHTTPJSON(decode) error = nil, want error")
	}
}
