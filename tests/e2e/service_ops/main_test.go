package main

import (
	"net/http"
	"testing"
)

func TestMainSmoke(t *testing.T) {
	main()
}

func TestAssertGETRejectsStatusAndBody(t *testing.T) {
	statusHandler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "bad", http.StatusTeapot)
	})
	if err := assertGET(statusHandler, "/bad", "bad"); err == nil {
		t.Fatal("assertGET(status) error = nil, want error")
	}
	bodyHandler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("missing"))
	})
	if err := assertGET(bodyHandler, "/body", "expected"); err == nil {
		t.Fatal("assertGET(body) error = nil, want error")
	}
}

func TestAssertCompactRejectsBadResponses(t *testing.T) {
	statusHandler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "bad", http.StatusTeapot)
	})
	if _, err := assertCompact(statusHandler); err == nil {
		t.Fatal("assertCompact(status) error = nil, want error")
	}
	decodeHandler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("{"))
	})
	if _, err := assertCompact(decodeHandler); err == nil {
		t.Fatal("assertCompact(decode) error = nil, want error")
	}
	responseHandler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"ok":false,"state":"failed"}`))
	})
	if _, err := assertCompact(responseHandler); err == nil {
		t.Fatal("assertCompact(response) error = nil, want error")
	}
}
