package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	mts "github.com/openmts/mts"
)

func TestHTTPMetaListResponsesReportPathAndScope(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	postJSON(t, server.URL+routeDataWrite, writeRequest{Points: []mts.Point{testPoint()}}, http.StatusOK, &writeResponse{})

	var dbs measurementsResponse
	getJSONWithHeaders(t, server.URL+routeDataDatabases, nil, http.StatusOK, &dbs)
	if dbs.Path != routeDataDatabases {
		t.Fatalf("databases path=%q", dbs.Path)
	}

	var meas measurementsResponse
	getJSONWithHeaders(t, server.URL+routeDataDatabasesPrefix+"default/measurements", nil, http.StatusOK, &meas)
	if meas.Path == "" || meas.Database != "default" {
		t.Fatalf("measurements path=%q db=%q", meas.Path, meas.Database)
	}

	var fields fieldsResponse
	getJSONWithHeaders(t, server.URL+routeDataDatabasesPrefix+"default/measurements/cpu/fields", nil, http.StatusOK, &fields)
	if fields.Path == "" || fields.Database != "default" || fields.Measurement != "cpu" {
		t.Fatalf("fields path=%q db=%q meas=%q", fields.Path, fields.Database, fields.Measurement)
	}

	var series seriesResponse
	getJSONWithHeaders(t, server.URL+routeDataDatabasesPrefix+"default/measurements/cpu/series", nil, http.StatusOK, &series)
	if series.Path == "" || series.Database != "default" || series.Measurement != "cpu" {
		t.Fatalf("series path=%q db=%q meas=%q", series.Path, series.Database, series.Measurement)
	}
	if series.Total < 1 {
		t.Fatalf("series total=%d", series.Total)
	}
}
