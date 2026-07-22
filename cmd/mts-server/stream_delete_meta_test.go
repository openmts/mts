package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mts "github.com/openmts/mts"
)

func TestHTTPQueryStreamEndReportsMetaAndAdminOp(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	point := testPoint()
	postJSON(t, server.URL+routeDataWrite, writeRequest{Points: []mts.Point{point}}, http.StatusOK, &writeResponse{})

	for _, format := range []string{"row", "column"} {
		resp, err := postJSONRawWithHeaders(server.URL+routeDataQueryStream, queryStreamRequest{
			Query:  testQuery(),
			Format: format,
		}, nil)
		if err != nil {
			t.Fatalf("stream %s: %v", format, err)
		}
		var end streamRecord
		foundEnd := false
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			var rec streamRecord
			if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
				_ = resp.Body.Close()
				t.Fatalf("decode: %v line=%q", err, sc.Text())
			}
			if rec.Type == streamTypeEnd {
				end = rec
				foundEnd = true
			}
		}
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
		if err := sc.Err(); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("format %s status=%d", format, resp.StatusCode)
		}
		if !foundEnd {
			t.Fatalf("format %s: missing end frame", format)
		}
		if end.Path != routeDataQueryStream {
			t.Fatalf("format %s path=%q", format, end.Path)
		}
		if end.Format != format {
			t.Fatalf("format %s got format=%q", format, end.Format)
		}
		if end.RecordCount < 1 {
			t.Fatalf("format %s record_count=%d", format, end.RecordCount)
		}
		if end.Stats == nil {
			t.Fatalf("format %s missing stats", format)
		}
		if end.Database != "default" || end.Measurement != "cpu" {
			t.Fatalf("format %s scope db=%q meas=%q", format, end.Database, end.Measurement)
		}
		_ = end.AdminOpBusy
	}
}

func TestHTTPDeleteResponseReportsPathScopeAndAdminOp(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	point := testPoint()
	postJSON(t, server.URL+routeDataWrite, writeRequest{Points: []mts.Point{point}}, http.StatusOK, &writeResponse{})

	var del deleteResponse
	postJSON(t, server.URL+routeDataDelete, deleteRequest{Request: mts.DeleteRequest{
		Database:    point.Database,
		Measurement: point.Measurement,
		StartTime:   1,
		EndTime:     1,
	}}, http.StatusOK, &del)
	if !del.OK {
		t.Fatalf("ok=false")
	}
	if del.Path != routeDataDelete {
		t.Fatalf("path=%q", del.Path)
	}
	if del.Measurement != point.Measurement {
		t.Fatalf("measurement=%q", del.Measurement)
	}
	_ = del.AdminOpBusy
}
