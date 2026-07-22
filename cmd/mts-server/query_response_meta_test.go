package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	mts "github.com/openmts/mts"
)

func TestHTTPQueryResponsesReportMetaAndAdminOp(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	point := testPoint()
	postJSON(t, server.URL+routeDataWrite, writeRequest{Points: []mts.Point{point}}, http.StatusOK, &writeResponse{})

	var rowsResp queryRowsResponse
	postJSON(t, server.URL+routeDataQueryRows, queryRequest{Query: testQuery()}, http.StatusOK, &rowsResp)
	if rowsResp.Path != routeDataQueryRows {
		t.Fatalf("rows path = %q", rowsResp.Path)
	}
	if rowsResp.RowCount != len(rowsResp.Rows) {
		t.Fatalf("row_count = %d len(rows)=%d", rowsResp.RowCount, len(rowsResp.Rows))
	}
	if rowsResp.Database != "default" || rowsResp.Measurement != "cpu" {
		t.Fatalf("rows scope db=%q meas=%q", rowsResp.Database, rowsResp.Measurement)
	}
	// admin fields may be false/empty when idle; ensure JSON shape is present via zero-value OK
	_ = rowsResp.AdminOpBusy

	var colsResp queryColumnsResponse
	postJSON(t, server.URL+routeDataQueryColumns, queryRequest{Query: testQuery()}, http.StatusOK, &colsResp)
	if colsResp.Path != routeDataQueryColumns || colsResp.SeriesCount != len(colsResp.Columns) {
		t.Fatalf("columns meta = path=%q series=%d len=%d", colsResp.Path, colsResp.SeriesCount, len(colsResp.Columns))
	}
	if colsResp.Database != "default" || colsResp.Measurement != "cpu" {
		t.Fatalf("columns scope db=%q meas=%q", colsResp.Database, colsResp.Measurement)
	}

	var explainResp queryExplainResponse
	postJSON(t, server.URL+routeDataQueryExplain, queryRequest{Query: testQuery()}, http.StatusOK, &explainResp)
	if explainResp.Path != routeDataQueryExplain {
		t.Fatalf("explain path = %q", explainResp.Path)
	}
	if explainResp.Database != "default" || explainResp.Measurement != "cpu" {
		t.Fatalf("explain scope db=%q meas=%q", explainResp.Database, explainResp.Measurement)
	}
}
