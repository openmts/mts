package queryservice_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryanalyzer"
	"github.com/openmts/mts/internal/queryexec"
	"github.com/openmts/mts/internal/querylang"
	"github.com/openmts/mts/internal/queryservice"
)

func TestServiceRejectsWhenAdmissionIsFull(t *testing.T) {
	service := queryservice.New(queryservice.Options{MaxConcurrent: 1}, blockingExecutor{})
	release, err := service.Admit(context.Background())
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	defer release()
	_, err = service.Query(context.Background(), queryservice.Request{Query: model.Query{Measurement: "cpu"}})
	if !errors.Is(err, queryservice.ErrAdmissionRejected) {
		t.Fatalf("Query() error = %v, want admission rejected", err)
	}
}

func TestServiceQueuesWhenAdmissionIsFull(t *testing.T) {
	service := queryservice.New(queryservice.Options{
		MaxConcurrent: 1,
		MaxQueued:     1,
	}, fakeStreamingExecutor{})
	release, err := service.Admit(context.Background())
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := service.Query(context.Background(), queryservice.Request{Query: model.Query{Measurement: "cpu"}})
		done <- err
	}()
	waitForQueued(t, service, 1)
	release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("queued Query() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued Query() did not finish")
	}
	stats := service.Stats()
	if stats.TotalQueued != 1 || stats.TotalAdmitted != 2 || stats.Queued != 0 || stats.Active != 0 {
		t.Fatalf("stats = %#v, want queued=1 admitted=2 and no active/queued", stats)
	}
}

func TestServiceReturnsQueueFullWhenQueueLimitReached(t *testing.T) {
	service := queryservice.New(queryservice.Options{
		MaxConcurrent: 1,
		MaxQueued:     1,
	}, fakeStreamingExecutor{})
	release, err := service.Admit(context.Background())
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := service.Query(context.Background(), queryservice.Request{Query: model.Query{Measurement: "cpu"}})
		done <- err
	}()
	waitForQueued(t, service, 1)
	_, err = service.Query(context.Background(), queryservice.Request{Query: model.Query{Measurement: "cpu"}})
	if !errors.Is(err, queryservice.ErrQueueFull) {
		t.Fatalf("Query() error = %v, want queue full", err)
	}
	release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("queued Query() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued Query() did not finish")
	}
	stats := service.Stats()
	if stats.TotalRejected == 0 {
		t.Fatalf("stats = %#v, want rejected count", stats)
	}
}

func TestServiceAppliesRequestTimeout(t *testing.T) {
	service := queryservice.New(queryservice.Options{}, contextExecutor{})
	_, err := service.Query(context.Background(), queryservice.Request{
		Query:   model.Query{Measurement: "cpu"},
		Timeout: time.Millisecond,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Query() error = %v, want deadline exceeded", err)
	}
	stats := service.Stats()
	if stats.TotalTimedOut != 1 {
		t.Fatalf("stats = %#v, want one timeout", stats)
	}
}

func TestServiceCachesQueryResultsAndInvalidates(t *testing.T) {
	executor := &countingExecutor{
		result: queryservice.Result{
			Rows: []model.Row{{Timestamp: 1}},
		},
	}
	service := queryservice.New(queryservice.Options{CacheMaxEntries: 2}, executor)
	request := queryservice.Request{Query: model.Query{Measurement: "cpu"}}
	first, err := service.Query(context.Background(), request)
	if err != nil {
		t.Fatalf("Query(first) error = %v", err)
	}
	first.Rows[0].Timestamp = 99
	second, err := service.Query(context.Background(), request)
	if err != nil {
		t.Fatalf("Query(second) error = %v", err)
	}
	if executor.calls != 1 {
		t.Fatalf("executor calls = %d, want 1", executor.calls)
	}
	if second.Rows[0].Timestamp != 1 {
		t.Fatalf("cached row = %#v, want immutable cached copy", second.Rows[0])
	}
	service.InvalidateCache()
	if _, err := service.Query(context.Background(), request); err != nil {
		t.Fatalf("Query(after invalidate) error = %v", err)
	}
	if executor.calls != 2 {
		t.Fatalf("executor calls after invalidate = %d, want 2", executor.calls)
	}
	stats := service.Stats()
	if stats.TotalCacheHits != 1 || stats.TotalCacheMisses != 2 {
		t.Fatalf("cache stats = %#v, want one hit and two misses", stats)
	}
}

func TestServiceAuthorizesTenantAndAuditsRejections(t *testing.T) {
	service := queryservice.New(queryservice.Options{
		AllowedTenants:  []string{"tenant-a"},
		AuditMaxRecords: 4,
	}, fakeStreamingExecutor{})
	_, err := service.Query(context.Background(), queryservice.Request{
		Tenant: "tenant-b",
		User:   "alice",
		Query:  model.Query{Measurement: "cpu"},
	})
	if !errors.Is(err, queryservice.ErrUnauthorized) {
		t.Fatalf("Query() error = %v, want unauthorized", err)
	}
	stats := service.Stats()
	if stats.TotalUnauthorized != 1 || stats.TotalAuditRecords != 1 {
		t.Fatalf("stats = %#v, want unauthorized audit", stats)
	}
	records := service.AuditRecords()
	if len(records) != 1 || records[0].Accepted || records[0].Tenant != "tenant-b" {
		t.Fatalf("audit records = %#v, want rejected tenant-b", records)
	}
}

func TestServiceQueryStreamReleasesAdmissionOnClose(t *testing.T) {
	service := queryservice.New(queryservice.Options{MaxConcurrent: 1}, fakeStreamingExecutor{
		rows: []model.Row{{Timestamp: 1}},
	})
	result, err := service.QueryStream(context.Background(), queryservice.Request{
		Query: model.Query{Measurement: "cpu"},
	})
	if err != nil {
		t.Fatalf("QueryStream() error = %v", err)
	}
	_, err = service.Query(context.Background(), queryservice.Request{Query: model.Query{Measurement: "cpu"}})
	if !errors.Is(err, queryservice.ErrAdmissionRejected) {
		t.Fatalf("Query() while stream open error = %v, want admission rejected", err)
	}
	if err := result.Rows.Close(); err != nil {
		t.Fatalf("Rows.Close() error = %v", err)
	}
	_, err = service.Query(context.Background(), queryservice.Request{Query: model.Query{Measurement: "cpu"}})
	if err != nil {
		t.Fatalf("Query() after stream close error = %v", err)
	}
}

func TestServiceQueryStreamUnsupportedReleasesAdmission(t *testing.T) {
	service := queryservice.New(queryservice.Options{MaxConcurrent: 1}, blockingExecutor{})
	_, err := service.QueryStream(context.Background(), queryservice.Request{
		Query: model.Query{Measurement: "cpu"},
	})
	if !errors.Is(err, queryservice.ErrStreamingUnsupported) {
		t.Fatalf("QueryStream() error = %v, want streaming unsupported", err)
	}
	if _, err := service.Query(context.Background(), queryservice.Request{
		Query: model.Query{Measurement: "cpu"},
	}); err != nil {
		t.Fatalf("Query() after unsupported stream error = %v", err)
	}
}

func TestServiceQueryStreamUnauthorizedAndEmptyResultReleaseAdmission(t *testing.T) {
	service := queryservice.New(queryservice.Options{
		MaxConcurrent:   1,
		AllowedTenants:  []string{"tenant-a"},
		AuditMaxRecords: 2,
	}, fakeStreamingExecutor{})
	_, err := service.QueryStream(context.Background(), queryservice.Request{
		Tenant: "tenant-b",
		Query:  model.Query{Measurement: "cpu"},
	})
	if !errors.Is(err, queryservice.ErrUnauthorized) {
		t.Fatalf("QueryStream(unauthorized) error = %v, want unauthorized", err)
	}
	if stats := service.Stats(); stats.TotalUnauthorized != 1 || stats.TotalAuditRecords != 1 {
		t.Fatalf("stats = %#v, want unauthorized audit", stats)
	}

	empty := queryservice.New(queryservice.Options{MaxConcurrent: 1}, emptyStreamingExecutor{})
	result, err := empty.QueryStream(context.Background(), queryservice.Request{
		Query: model.Query{Measurement: "cpu"},
	})
	if err != nil {
		t.Fatalf("QueryStream(empty) error = %v", err)
	}
	if result.Rows != nil || result.Columns != nil {
		t.Fatalf("empty stream result = %#v, want no streams", result)
	}
	if _, err := empty.Query(context.Background(), queryservice.Request{
		Query: model.Query{Measurement: "cpu"},
	}); err != nil {
		t.Fatalf("Query after empty stream error = %v", err)
	}
}

func TestServiceQueryStreamReleasesAdmissionOnColumnEOF(t *testing.T) {
	service := queryservice.New(queryservice.Options{MaxConcurrent: 1}, fakeStreamingExecutor{
		columns: []model.ColumnSeries{{Timestamps: []int64{1}, Values: []model.FieldValue{model.Int64Value(1)}}},
	})
	result, err := service.QueryStream(context.Background(), queryservice.Request{
		Query: model.Query{Measurement: "cpu"},
	})
	if err != nil {
		t.Fatalf("QueryStream() error = %v", err)
	}
	if !result.Columns.Next() {
		t.Fatalf("Columns.Next() = false err=%v", result.Columns.Err())
	}
	if result.Columns.Next() {
		t.Fatal("Columns.Next(after EOF) = true, want false")
	}
	if err := result.Columns.Err(); err != nil {
		t.Fatalf("Columns.Err() = %v", err)
	}
	if _, err := service.Query(context.Background(), queryservice.Request{
		Query: model.Query{Measurement: "cpu"},
	}); err != nil {
		t.Fatalf("Query() after column stream EOF error = %v", err)
	}
}

func TestCompatExecutorUsesRowsForRawQueriesAndColumnsForAggregates(t *testing.T) {
	reader := fakeReader{
		rows:    []model.Row{{Measurement: "cpu"}},
		columns: []model.ColumnSeries{{Measurement: "cpu"}},
	}
	executor := queryservice.NewCompatExecutor(reader)
	raw, err := executor.Query(context.Background(), model.Query{Measurement: "cpu"})
	if err != nil {
		t.Fatalf("Query(raw) error = %v", err)
	}
	if len(raw.Rows) != 1 || len(raw.Columns) != 0 {
		t.Fatalf("raw result = %#v, want rows only", raw)
	}
	aggregated, err := executor.Query(context.Background(), model.Query{
		Measurement: "cpu",
		Aggregates:  []model.AggregateSpec{{Field: "usage", Function: "avg"}},
	})
	if err != nil {
		t.Fatalf("Query(aggregate) error = %v", err)
	}
	if len(aggregated.Columns) != 1 || len(aggregated.Rows) != 0 {
		t.Fatalf("aggregate result = %#v, want columns only", aggregated)
	}
}

func TestLayeredExecutorRunsAnalyzerAndQuerySpecRows(t *testing.T) {
	reader := &fakeLayeredReader{
		fields: []model.FieldSchema{{Measurement: "cpu", Name: "usage", Type: model.FieldFloat64}},
		rows:   []model.Row{{Measurement: "cpu"}},
	}
	executor := queryservice.NewLayeredExecutor(reader)
	result, err := executor.Query(context.Background(), model.Query{
		Database:    "metrics",
		Measurement: "cpu",
		Fields:      []string{"usage"},
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(result.Rows) != 1 || len(result.Columns) != 0 {
		t.Fatalf("result = %#v, want rows", result)
	}
	if reader.rowsSpec.Measurement != "cpu" || len(reader.rowsSpec.Fields) != 1 {
		t.Fatalf("rows spec = %#v, want cpu usage", reader.rowsSpec)
	}
	if result.LogicalPlanRoot == "" {
		t.Fatalf("logical plan root is empty")
	}
	if len(result.PhysicalOperators) == 0 {
		t.Fatalf("physical operators are empty")
	}
	if len(result.Pushdowns) == 0 {
		t.Fatalf("pushdowns are empty")
	}
	if !containsString(result.Pushdowns, "limit") {
		t.Fatalf("pushdowns = %v, want limit", result.Pushdowns)
	}
	if !containsString(result.PhysicalOperators, "limit") {
		t.Fatalf("physical operators = %v, want limit", result.PhysicalOperators)
	}
}

func TestLayeredExecutorRejectsInvalidFieldBeforeReader(t *testing.T) {
	reader := &fakeLayeredReader{
		fields: []model.FieldSchema{{Measurement: "cpu", Name: "usage", Type: model.FieldFloat64}},
	}
	executor := queryservice.NewLayeredExecutor(reader)
	_, err := executor.Query(context.Background(), model.Query{
		Database:    "metrics",
		Measurement: "cpu",
		Fields:      []string{"missing"},
	})
	if !queryanalyzer.IsCode(err, queryanalyzer.ErrFieldNotFound) {
		t.Fatalf("Query() error = %v, want field not found", err)
	}
	if reader.rowsCalled || reader.columnsCalled {
		t.Fatal("reader was called for invalid query")
	}
}

func TestLayeredExecutorRunsColumnAggregatePath(t *testing.T) {
	reader := &fakeLayeredReader{
		fields:  []model.FieldSchema{{Measurement: "cpu", Name: "usage", Type: model.FieldFloat64}},
		columns: []model.ColumnSeries{{Measurement: "cpu"}},
	}
	executor := queryservice.NewLayeredExecutor(reader)
	result, err := executor.Query(context.Background(), model.Query{
		Database:    "metrics",
		Measurement: "cpu",
		Aggregates:  []model.AggregateSpec{{Field: "usage", Function: "avg"}},
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(result.Columns) != 1 || len(result.Rows) != 0 {
		t.Fatalf("result = %#v, want columns", result)
	}
	if !reader.columnsCalled {
		t.Fatal("column reader was not called")
	}
	if result.LogicalPlanRoot == "" || len(result.PhysicalOperators) == 0 {
		t.Fatalf("result plan metadata = %#v", result)
	}
}

func TestLayeredExecutorQueryStreamRows(t *testing.T) {
	reader := &fakeLayeredReader{
		fields: []model.FieldSchema{{Measurement: "cpu", Name: "usage", Type: model.FieldFloat64}},
		rows: []model.Row{
			{Measurement: "cpu", Timestamp: 1},
			{Measurement: "cpu", Timestamp: 2},
		},
	}
	executor := queryservice.NewLayeredExecutor(reader)
	result, err := executor.QueryStream(context.Background(), model.Query{
		Database:    "metrics",
		Measurement: "cpu",
		Fields:      []string{"usage"},
	})
	if err != nil {
		t.Fatalf("QueryStream() error = %v", err)
	}
	var timestamps []int64
	for result.Rows.Next() {
		timestamps = append(timestamps, result.Rows.Row().Timestamp)
	}
	if err := result.Rows.Err(); err != nil {
		t.Fatalf("Rows.Err() = %v", err)
	}
	if len(timestamps) != 2 || timestamps[0] != 1 || timestamps[1] != 2 {
		t.Fatalf("timestamps = %v, want [1 2]", timestamps)
	}
	if len(result.Profile.Operators) == 0 {
		t.Fatal("profile is empty")
	}
	execute := result.Profile.Operators[len(result.Profile.Operators)-1]
	if execute.ID != "execute" || execute.RowsOut != 2 {
		t.Fatalf("execute profile = %#v, want RowsOut=2", execute)
	}
}

func TestLayeredExecutorQueryStreamColumns(t *testing.T) {
	reader := &fakeLayeredReader{
		fields: []model.FieldSchema{{Measurement: "cpu", Name: "usage", Type: model.FieldFloat64}},
		columns: []model.ColumnSeries{{
			Measurement: "cpu",
			FieldName:   "usage",
			Timestamps:  []int64{1},
			Values:      []model.FieldValue{model.Float64Value(1)},
		}},
	}
	executor := queryservice.NewLayeredExecutor(reader)
	result, err := executor.QueryStream(context.Background(), model.Query{
		Database:    "metrics",
		Measurement: "cpu",
		Aggregates:  []model.AggregateSpec{{Field: "usage", Function: "avg"}},
	})
	if err != nil {
		t.Fatalf("QueryStream() error = %v", err)
	}
	if !result.Columns.Next() {
		t.Fatalf("Columns.Next() = false err=%v", result.Columns.Err())
	}
	if got := result.Columns.Column(); got.FieldName != "usage" {
		t.Fatalf("column = %#v, want usage", got)
	}
	if err := result.Columns.Close(); err != nil {
		t.Fatalf("Columns.Close() error = %v", err)
	}
	if len(result.Profile.Operators) == 0 {
		t.Fatal("profile is empty")
	}
}

func TestLayeredExecutorProfilesSuccessfulRowQuery(t *testing.T) {
	reader := &fakeLayeredReader{
		fields: []model.FieldSchema{{Measurement: "cpu", Name: "usage", Type: model.FieldFloat64}},
		rows: []model.Row{
			{Measurement: "cpu", Fields: map[string]model.FieldValue{"usage": model.Float64Value(1)}},
			{Measurement: "cpu", Fields: map[string]model.FieldValue{"usage": model.Float64Value(2)}},
		},
	}
	executor := queryservice.NewLayeredExecutor(reader)
	result, err := executor.Query(context.Background(), model.Query{
		Database:    "metrics",
		Measurement: "cpu",
		Fields:      []string{"usage"},
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	want := []string{"analyze", "logical_plan", "optimize", "physical_plan", "execute"}
	assertProfileIDs(t, result, want)
	execute := result.Profile.Operators[len(result.Profile.Operators)-1]
	if execute.RowsOut != 2 {
		t.Fatalf("execute RowsOut = %d, want 2", execute.RowsOut)
	}
}

func TestLayeredExecutorProfilesAnalyzeErrors(t *testing.T) {
	reader := &fakeLayeredReader{
		fields: []model.FieldSchema{{Measurement: "cpu", Name: "usage", Type: model.FieldFloat64}},
	}
	executor := queryservice.NewLayeredExecutor(reader)
	result, err := executor.Query(context.Background(), model.Query{
		Database:    "metrics",
		Measurement: "cpu",
		Fields:      []string{"missing"},
	})
	if !queryanalyzer.IsCode(err, queryanalyzer.ErrFieldNotFound) {
		t.Fatalf("Query() error = %v, want field not found", err)
	}
	if len(result.Profile.Operators) != 1 {
		t.Fatalf("profile = %#v, want one analyze entry", result.Profile)
	}
	if result.Profile.Operators[0].ID != "analyze" || result.Profile.Operators[0].Error == "" {
		t.Fatalf("profile = %#v, want analyze error", result.Profile)
	}
}

func TestLayeredExecutorNilReaderReturnsEmptyResult(t *testing.T) {
	executor := queryservice.NewLayeredExecutor(nil)
	result, err := executor.Query(context.Background(), model.Query{Measurement: "cpu"})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(result.Rows) != 0 || len(result.Columns) != 0 {
		t.Fatalf("result = %#v, want empty", result)
	}
}

func TestHTTPHandlerQueryReturnsJSONResult(t *testing.T) {
	service := queryservice.New(queryservice.Options{}, fakeStreamingExecutor{})
	handler := queryservice.NewHTTPHandler(service)
	body := bytes.NewBufferString(`{"query":{"measurement":"cpu"}}`)
	request := httptest.NewRequest(http.MethodPost, "/query", body)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", recorder.Code, recorder.Body.String())
	}
	var response queryservice.QueryResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if !response.OK {
		t.Fatalf("response = %#v, want ok", response)
	}
}

func TestHTTPHandlerQueryStreamReturnsNDJSONRows(t *testing.T) {
	service := queryservice.New(queryservice.Options{}, fakeStreamingExecutor{
		rows: []model.Row{{Timestamp: 1}, {Timestamp: 2}},
	})
	handler := queryservice.NewHTTPHandler(service)
	body := bytes.NewBufferString(`{"query":{"measurement":"cpu"}}`)
	request := httptest.NewRequest(http.MethodPost, "/query/stream", body)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/x-ndjson" {
		t.Fatalf("Content-Type = %q, want application/x-ndjson", contentType)
	}
	lines := strings.Split(strings.TrimSpace(recorder.Body.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %v, want two rows and end", lines)
	}
	var first queryservice.StreamRecord
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("Unmarshal(row) error = %v", err)
	}
	if first.Type != "row" || first.Row == nil || first.Row.Timestamp != 1 {
		t.Fatalf("first record = %#v, want row timestamp 1", first)
	}
	var last queryservice.StreamRecord
	if err := json.Unmarshal([]byte(lines[2]), &last); err != nil {
		t.Fatalf("Unmarshal(end) error = %v", err)
	}
	if last.Type != "end" {
		t.Fatalf("last record = %#v, want end", last)
	}
}

func TestHTTPHandlerQueryStreamReturnsNDJSONColumns(t *testing.T) {
	service := queryservice.New(queryservice.Options{}, fakeStreamingExecutor{
		columns: []model.ColumnSeries{{
			FieldName:  "usage",
			Timestamps: []int64{1},
			Values:     []model.FieldValue{model.Float64Value(1)},
		}},
	})
	handler := queryservice.NewHTTPHandler(service)
	body := bytes.NewBufferString(`{"query":{"measurement":"cpu","aggregates":[{"field":"usage","function":"avg"}]}}`)
	request := httptest.NewRequest(http.MethodPost, "/query/stream", body)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", recorder.Code, recorder.Body.String())
	}
	lines := strings.Split(strings.TrimSpace(recorder.Body.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %v, want one column and end", lines)
	}
	var first queryservice.StreamRecord
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("Unmarshal(column) error = %v", err)
	}
	if first.Type != "column" || first.Column == nil || first.Column.FieldName != "usage" {
		t.Fatalf("first record = %#v, want usage column", first)
	}
}

func TestHTTPHandlerMapsAdmissionError(t *testing.T) {
	service := queryservice.New(queryservice.Options{MaxConcurrent: 1}, fakeStreamingExecutor{})
	release, err := service.Admit(context.Background())
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	defer release()
	handler := queryservice.NewHTTPHandler(service)
	body := bytes.NewBufferString(`{"query":{"measurement":"cpu"}}`)
	request := httptest.NewRequest(http.MethodPost, "/query", body)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d body=%s, want 429", recorder.Code, recorder.Body.String())
	}
	var response queryservice.QueryResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if response.Error == nil || response.Error.Code != queryservice.ErrorCodeAdmissionRejected {
		t.Fatalf("error = %#v, want admission_rejected", response.Error)
	}
}

func TestHTTPHandlerQueryStats(t *testing.T) {
	service := queryservice.New(queryservice.Options{MaxConcurrent: 1}, fakeStreamingExecutor{})
	release, err := service.Admit(context.Background())
	if err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	release()
	handler := queryservice.NewHTTPHandler(service)
	request := httptest.NewRequest(http.MethodGet, "/query/stats", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", recorder.Code, recorder.Body.String())
	}
	var response queryservice.StatsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if !response.OK || response.Stats.TotalAdmitted != 1 {
		t.Fatalf("response = %#v, want admitted stats", response)
	}
}

func TestHTTPHandlerQueryAudit(t *testing.T) {
	service := queryservice.New(queryservice.Options{
		AllowedTenants:  []string{"tenant-a"},
		AuditMaxRecords: 2,
	}, fakeStreamingExecutor{})
	handler := queryservice.NewHTTPHandler(service)
	body := bytes.NewBufferString(`{"tenant":"tenant-b","query":{"measurement":"cpu"}}`)
	request := httptest.NewRequest(http.MethodPost, "/query", body)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s, want 403", recorder.Code, recorder.Body.String())
	}
	auditRequest := httptest.NewRequest(http.MethodGet, "/query/audit", nil)
	auditRecorder := httptest.NewRecorder()
	handler.ServeHTTP(auditRecorder, auditRequest)
	if auditRecorder.Code != http.StatusOK {
		t.Fatalf("audit status = %d body=%s, want 200", auditRecorder.Code, auditRecorder.Body.String())
	}
	var response queryservice.AuditResponse
	if err := json.NewDecoder(auditRecorder.Body).Decode(&response); err != nil {
		t.Fatalf("Decode(audit) error = %v", err)
	}
	if !response.OK || len(response.Records) != 1 || response.Records[0].Code != queryservice.ErrorCodeUnauthorized {
		t.Fatalf("audit response = %#v, want unauthorized record", response)
	}
}

func TestHTTPHandlerRejectsBadRequests(t *testing.T) {
	service := queryservice.New(queryservice.Options{}, fakeStreamingExecutor{})
	handler := queryservice.NewHTTPHandler(service)
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
	}{
		{
			name:   "method",
			method: http.MethodGet,
			path:   "/query",
			status: http.StatusMethodNotAllowed,
		},
		{
			name:   "unknown field",
			method: http.MethodPost,
			path:   "/query",
			body:   `{"query":{"measurement":"cpu"},"extra":true}`,
			status: http.StatusBadRequest,
		},
		{
			name:   "stream method",
			method: http.MethodGet,
			path:   "/query/stream",
			status: http.StatusMethodNotAllowed,
		},
		{
			name:   "audit method",
			method: http.MethodPost,
			path:   "/query/audit",
			status: http.StatusMethodNotAllowed,
		},
		{
			name:   "stats method",
			method: http.MethodPost,
			path:   "/query/stats",
			status: http.StatusMethodNotAllowed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != tt.status {
				t.Fatalf("status = %d body=%s, want %d", recorder.Code, recorder.Body.String(), tt.status)
			}
			var response queryservice.QueryResponse
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if response.OK || response.Error == nil {
				t.Fatalf("response = %#v, want error", response)
			}
		})
	}
}

func TestHTTPHandlerMapsQueryServiceErrors(t *testing.T) {
	for _, tt := range []struct {
		name   string
		err    error
		status int
		code   queryservice.ErrorCode
	}{
		{name: "queue", err: queryservice.ErrQueueFull, status: http.StatusTooManyRequests, code: queryservice.ErrorCodeQueueFull},
		{name: "streaming", err: queryservice.ErrStreamingUnsupported, status: http.StatusNotImplemented, code: queryservice.ErrorCodeStreamingUnsupported},
		{name: "language", err: queryservice.ErrUnsupportedQueryLanguage, status: http.StatusNotImplemented, code: queryservice.ErrorCodeLanguageUnsupported},
		{name: "distributed", err: queryservice.ErrDistributedUnsupported, status: http.StatusNotImplemented, code: queryservice.ErrorCodeDistributedUnsupported},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service := queryservice.New(queryservice.Options{}, errorExecutor{err: tt.err})
			handler := queryservice.NewHTTPHandler(service)
			request := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(`{"query":{"measurement":"cpu"}}`))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != tt.status {
				t.Fatalf("status = %d body=%s, want %d", recorder.Code, recorder.Body.String(), tt.status)
			}
			var response queryservice.QueryResponse
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if response.Error == nil || response.Error.Code != tt.code {
				t.Fatalf("response = %#v, want code %s", response, tt.code)
			}
		})
	}
}

func assertProfileIDs(t *testing.T, result queryservice.Result, want []string) {
	t.Helper()
	if len(result.Profile.Operators) != len(want) {
		t.Fatalf("profile IDs = %#v, want %v", result.Profile.Operators, want)
	}
	for index, id := range want {
		if result.Profile.Operators[index].ID != id {
			t.Fatalf("profile[%d].ID = %q, want %q", index, result.Profile.Operators[index].ID, id)
		}
		if result.Profile.Operators[index].Duration < 0 {
			t.Fatalf("profile[%d].Duration = %v, want non-negative", index, result.Profile.Operators[index].Duration)
		}
	}
}

func waitForQueued(t *testing.T, service *queryservice.Service, want int64) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := service.Stats().Queued; got == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("queued = %d, want %d", service.Stats().Queued, want)
}

type blockingExecutor struct{}

func (blockingExecutor) Query(context.Context, model.Query) (queryservice.Result, error) {
	return queryservice.Result{}, nil
}

type contextExecutor struct{}

func (contextExecutor) Query(ctx context.Context, _ model.Query) (queryservice.Result, error) {
	<-ctx.Done()
	return queryservice.Result{}, ctx.Err()
}

type countingExecutor struct {
	result queryservice.Result
	calls  int
}

func (e *countingExecutor) Query(context.Context, model.Query) (queryservice.Result, error) {
	e.calls++
	return e.result, nil
}

type fakeStreamingExecutor struct {
	rows    []model.Row
	columns []model.ColumnSeries
}

type emptyStreamingExecutor struct{}

func (emptyStreamingExecutor) Query(context.Context, model.Query) (queryservice.Result, error) {
	return queryservice.Result{}, nil
}

func (emptyStreamingExecutor) QueryStream(context.Context, model.Query) (queryservice.StreamResult, error) {
	return queryservice.StreamResult{}, nil
}

type errorExecutor struct {
	err error
}

func (e errorExecutor) Query(context.Context, model.Query) (queryservice.Result, error) {
	return queryservice.Result{}, e.err
}

func (f fakeStreamingExecutor) Query(context.Context, model.Query) (queryservice.Result, error) {
	return queryservice.Result{}, nil
}

func (f fakeStreamingExecutor) QueryStream(
	context.Context,
	model.Query,
) (queryservice.StreamResult, error) {
	if len(f.columns) > 0 {
		return queryservice.StreamResult{
			Columns: queryexec.NewSliceColumnSeriesStream(append([]model.ColumnSeries(nil), f.columns...)),
		}, nil
	}
	return queryservice.StreamResult{
		Rows: queryexec.NewSliceRowStream(append([]model.Row(nil), f.rows...)),
	}, nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

type fakeReader struct {
	rows    []model.Row
	columns []model.ColumnSeries
}

func (f fakeReader) QueryColumns(context.Context, model.Query) ([]model.ColumnSeries, error) {
	return append([]model.ColumnSeries(nil), f.columns...), nil
}

func (f fakeReader) QueryRows(context.Context, model.Query) ([]model.Row, error) {
	return append([]model.Row(nil), f.rows...), nil
}

type fakeLayeredReader struct {
	fields        []model.FieldSchema
	rows          []model.Row
	columns       []model.ColumnSeries
	rowsSpec      querylang.QuerySpec
	rowsCalled    bool
	columnsCalled bool
}

func (f *fakeLayeredReader) ListFields(context.Context, string, string) ([]model.FieldSchema, error) {
	return append([]model.FieldSchema(nil), f.fields...), nil
}

func (f *fakeLayeredReader) QuerySpecRows(
	_ context.Context,
	spec querylang.QuerySpec,
) ([]model.Row, error) {
	f.rowsCalled = true
	f.rowsSpec = spec
	return append([]model.Row(nil), f.rows...), nil
}

func (f *fakeLayeredReader) QuerySpecRowStream(
	_ context.Context,
	spec querylang.QuerySpec,
) (queryexec.RowStream, error) {
	f.rowsCalled = true
	f.rowsSpec = spec
	return queryexec.NewSliceRowStream(append([]model.Row(nil), f.rows...)), nil
}

func (f *fakeLayeredReader) QuerySpecColumnStream(
	context.Context,
	querylang.QuerySpec,
) (queryexec.ColumnStream, error) {
	f.columnsCalled = true
	return queryexec.NewSliceColumnSeriesStream(append([]model.ColumnSeries(nil), f.columns...)), nil
}

func (f *fakeLayeredReader) QuerySpecWithExplain(
	_ context.Context,
	_ querylang.QuerySpec,
) ([]model.ColumnSeries, model.QueryExplain, model.QueryStats, error) {
	f.columnsCalled = true
	return append([]model.ColumnSeries(nil), f.columns...), model.QueryExplain{}, model.QueryStats{}, nil
}
