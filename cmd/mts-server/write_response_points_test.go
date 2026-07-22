package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	mts "github.com/openmts/mts"
)

func TestHTTPWriteResponsesReportAcceptedPoints(t *testing.T) {
	runtime := openTestRuntime(t)
	server := httptest.NewServer(runtime.httpHandler())
	defer server.Close()

	point := testPoint()
	point.Database = "default"
	point.RetentionPolicy = "autogen"
	var writeResp writeResponse
	postJSON(t, server.URL+routeDataWrite, writeRequest{Points: []mts.Point{point}}, http.StatusOK, &writeResp)
	if !writeResp.OK || writeResp.Points != 1 || writeResp.Path != routeDataWrite || writeResp.Mode != "points" {
		t.Fatalf("write resp = %+v", writeResp)
	}
	if writeResp.Database != "default" {
		t.Fatalf("write database = %q want default", writeResp.Database)
	}
	if writeResp.RetentionPolicy != "autogen" {
		t.Fatalf("write retention_policy = %q want autogen", writeResp.RetentionPolicy)
	}

	batch := mts.TypedBatch{
		Database:        "default",
		RetentionPolicy: "autogen",
		Measurement:     point.Measurement,
		Tags: []mts.TagColumn{{
			Name:   "host",
			Values: []string{"api-1"},
		}},
		Timestamps: []int64{point.Timestamp},
		Fields: []mts.TypedFieldColumn{{
			Name:          "usage",
			Type:          mts.FieldFloat64,
			Float64Values: []float64{0.7},
		}},
	}
	writeResp = writeResponse{}
	postJSON(t, server.URL+routeDataWriteTyped, typedWriteRequest{Batch: batch}, http.StatusOK, &writeResp)
	if !writeResp.OK || writeResp.Points != 1 || writeResp.Path != routeDataWriteTyped || writeResp.Mode != "typed" {
		t.Fatalf("typed write resp = %+v", writeResp)
	}
	if writeResp.Database != "default" {
		t.Fatalf("typed database = %q want default", writeResp.Database)
	}
	if writeResp.RetentionPolicy != "autogen" {
		t.Fatalf("typed retention_policy = %q want autogen", writeResp.RetentionPolicy)
	}

	writeResp = writeResponse{}
	postJSON(t, server.URL+routeDataWritePointsTyped, writeRequest{Points: []mts.Point{point}}, http.StatusOK, &writeResp)
	if !writeResp.OK || writeResp.Points != 1 || writeResp.Path != routeDataWritePointsTyped || writeResp.Mode != "points_typed" {
		t.Fatalf("points-typed write resp = %+v", writeResp)
	}
	if writeResp.Database != "default" {
		t.Fatalf("points-typed database = %q want default", writeResp.Database)
	}
	if writeResp.RetentionPolicy != "autogen" {
		t.Fatalf("points-typed retention_policy = %q want autogen", writeResp.RetentionPolicy)
	}
}
