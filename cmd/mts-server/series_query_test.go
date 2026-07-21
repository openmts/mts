package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	mts "github.com/openmts/mts"
)

func TestBuildSeriesResponseOffsetAndQuery(t *testing.T) {
	series := []mts.Series{
		{ID: 1, Measurement: "cpu", Tags: map[string]string{"host": "h0"}},
		{ID: 2, Measurement: "cpu", Tags: map[string]string{"host": "h1"}},
		{ID: 3, Measurement: "mem", Tags: map[string]string{"host": "h2"}},
		{ID: 4, Measurement: "cpu", Tags: map[string]string{"host": "h3"}},
	}
	resp := buildSeriesResponse(series, seriesPageOpts{Limit: 1, Offset: 1, Query: "cpu"})
	if resp.Total != 3 {
		t.Fatalf("total = %d, want 3", resp.Total)
	}
	if len(resp.Series) != 1 || resp.Series[0].Tags["host"] != "h1" {
		t.Fatalf("page = %#v", resp.Series)
	}
	if !resp.Truncated || resp.Offset != 1 || resp.Limit != 1 {
		t.Fatalf("meta = %#v", resp)
	}
}

func TestSeriesPageOptionsParsing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/series?limit=2&offset=3&q=host", nil)
	opts := seriesPageOptions(req)
	if opts.Limit != 2 || opts.Offset != 3 || opts.Query != "host" {
		t.Fatalf("opts = %#v", opts)
	}
}
