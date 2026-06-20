package queryservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryexec"
)

func TestResultCacheClonesColumnsRowsAndEvictsOldest(t *testing.T) {
	cache := newResultCache(1)
	first := Result{
		Columns: []model.ColumnSeries{{
			Tags:       map[string]string{"host": "a"},
			Timestamps: []int64{1},
			Values:     []model.FieldValue{model.Float64Value(1)},
		}},
		Rows: []model.Row{{
			Tags:   map[string]string{"host": "a"},
			Fields: map[string]model.FieldValue{"usage": model.Float64Value(1)},
		}},
		Pushdowns:         []string{"field"},
		PhysicalOperators: []string{"scan"},
	}
	cache.set("first", first)
	first.Columns[0].Tags["host"] = "mutated"
	first.Rows[0].Fields["usage"] = model.Float64Value(99)

	cached, ok := cache.get("first")
	if !ok {
		t.Fatal("cache.get(first) ok = false, want true")
	}
	if cached.Columns[0].Tags["host"] != "a" {
		t.Fatalf("cached column tags = %#v, want immutable host=a", cached.Columns[0].Tags)
	}
	if cached.Rows[0].Fields["usage"].Float64 != 1 {
		t.Fatalf("cached row field = %#v, want immutable value 1", cached.Rows[0].Fields["usage"])
	}
	cached.Columns[0].Timestamps[0] = 100
	again, ok := cache.get("first")
	if !ok {
		t.Fatal("cache.get(first again) ok = false, want true")
	}
	if again.Columns[0].Timestamps[0] != 1 {
		t.Fatalf("cached timestamp = %d, want defensive clone", again.Columns[0].Timestamps[0])
	}

	cache.set("second", Result{Rows: []model.Row{{Timestamp: 2}}})
	if _, ok := cache.get("first"); ok {
		t.Fatal("cache.get(first after eviction) ok = true, want false")
	}
	if _, ok := cache.get("second"); !ok {
		t.Fatal("cache.get(second) ok = false, want true")
	}
	cache.clear()
	if _, ok := cache.get("second"); ok {
		t.Fatal("cache.get(second after clear) ok = true, want false")
	}
}

func TestStaticTenantAuthorizerAndAuditRing(t *testing.T) {
	if authorizer := newStaticTenantAuthorizer(nil); authorizer != nil {
		t.Fatalf("newStaticTenantAuthorizer(nil) = %#v, want nil", authorizer)
	}
	authorizer := newStaticTenantAuthorizer([]string{"", "tenant-a"})
	if err := authorizer.AuthorizeQuery(context.Background(), Principal{Tenant: ""}, model.Query{}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("AuthorizeQuery(empty tenant) error = %v, want unauthorized", err)
	}
	if err := authorizer.AuthorizeQuery(context.Background(), Principal{Tenant: "tenant-b"}, model.Query{}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("AuthorizeQuery(tenant-b) error = %v, want unauthorized", err)
	}
	if err := authorizer.AuthorizeQuery(context.Background(), Principal{Tenant: "tenant-a"}, model.Query{}); err != nil {
		t.Fatalf("AuthorizeQuery(tenant-a) error = %v", err)
	}

	log := newAuditLog(2)
	started := time.Now()
	log.append(newAuditRecord(Request{Tenant: "a", Query: model.Query{Measurement: "cpu"}}, true, "", started))
	log.append(newAuditRecord(Request{Tenant: "b", Query: model.Query{Measurement: "mem"}}, false, ErrorCodeUnauthorized, started))
	log.append(newAuditRecord(Request{Tenant: "c", Query: model.Query{Measurement: "disk"}}, true, "", started))
	records, total := log.snapshot()
	if total != 3 || len(records) != 2 {
		t.Fatalf("audit snapshot total=%d len=%d, want total 3 len 2", total, len(records))
	}
	if records[0].Tenant != "b" || records[1].Tenant != "c" {
		t.Fatalf("audit records = %#v, want last two tenants b/c", records)
	}
}

func TestStreamProtocolWritesErrorsAndColumnEnd(t *testing.T) {
	rowErr := errors.New("row stream failed")
	var rows bytes.Buffer
	writeRowStreamRecords(json.NewEncoder(&rows), StreamResult{
		Rows: errorRowStream{err: rowErr},
	})
	rowPayload := strings.TrimSpace(rows.String())
	if !strings.Contains(rowPayload, `"type":"error"`) ||
		!strings.Contains(rowPayload, rowErr.Error()) {
		t.Fatalf("row stream payload = %q, want error record", rowPayload)
	}

	columnErr := errors.New("column stream failed")
	var columns bytes.Buffer
	writeColumnStreamRecords(json.NewEncoder(&columns), StreamResult{
		Columns: errorColumnStream{err: columnErr},
	})
	columnPayload := strings.TrimSpace(columns.String())
	if !strings.Contains(columnPayload, `"type":"error"`) ||
		!strings.Contains(columnPayload, columnErr.Error()) {
		t.Fatalf("column stream payload = %q, want error record", columnPayload)
	}

	var empty bytes.Buffer
	writeColumnStreamRecords(json.NewEncoder(&empty), StreamResult{})
	if !strings.Contains(empty.String(), `"type":"end"`) {
		t.Fatalf("empty column payload = %q, want end record", empty.String())
	}
}

func TestReleaseStreamsHandleNilInnerAndDoubleClose(t *testing.T) {
	releases := 0
	rows := &releaseRowStream{release: func() { releases++ }}
	if rows.Next() {
		t.Fatal("nil row stream Next() = true, want false")
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("nil row stream Close() error = %v", err)
	}
	if releases != 1 {
		t.Fatalf("row releases = %d, want once", releases)
	}

	releases = 0
	columns := &releaseColumnStream{
		inner:   queryexec.NewSliceColumnSeriesStream([]model.ColumnSeries{{FieldName: "usage"}}),
		release: func() { releases++ },
	}
	if !columns.Next() {
		t.Fatalf("column Next() = false err=%v", columns.Err())
	}
	if got := columns.Column().FieldName; got != "usage" {
		t.Fatalf("column field = %q, want usage", got)
	}
	if err := columns.Close(); err != nil {
		t.Fatalf("column Close() error = %v", err)
	}
	if err := columns.Close(); err != nil {
		t.Fatalf("column Close(second) error = %v", err)
	}
	if releases != 1 {
		t.Fatalf("column releases = %d, want once", releases)
	}
}

func TestServiceInternalSmallBranches(t *testing.T) {
	if key, ok := cacheKey(Request{Query: model.Query{Measurement: "cpu", Cursor: "cursor"}}); ok || key != "" {
		t.Fatalf("cacheKey(cursor) = %q %v, want disabled cache", key, ok)
	}
	service := New(Options{QueuePollInterval: 5 * time.Millisecond}, nil)
	if got := service.queuePollInterval(); got != 5*time.Millisecond {
		t.Fatalf("queuePollInterval() = %v, want configured interval", got)
	}
	defaultService := New(Options{}, nil)
	if got := defaultService.queuePollInterval(); got != time.Millisecond {
		t.Fatalf("default queuePollInterval() = %v, want 1ms", got)
	}

	row := &releaseRowStream{}
	if got := row.Row(); got.Timestamp != 0 {
		t.Fatalf("nil release row Row() = %#v, want zero", got)
	}
	if err := row.Err(); err != nil {
		t.Fatalf("nil release row Err() = %v, want nil", err)
	}
	column := &releaseColumnStream{}
	if got := column.Column(); got.FieldName != "" {
		t.Fatalf("nil release column Column() = %#v, want zero", got)
	}
	if err := column.Err(); err != nil {
		t.Fatalf("nil release column Err() = %v, want nil", err)
	}
}

type errorRowStream struct {
	err error
}

func (s errorRowStream) Next() bool {
	return false
}

func (s errorRowStream) Row() model.Row {
	return model.Row{}
}

func (s errorRowStream) Err() error {
	return s.err
}

func (s errorRowStream) Close() error {
	return nil
}

type errorColumnStream struct {
	err error
}

func (s errorColumnStream) Next() bool {
	return false
}

func (s errorColumnStream) Column() model.ColumnSeries {
	return model.ColumnSeries{}
}

func (s errorColumnStream) Err() error {
	return s.err
}

func (s errorColumnStream) Close() error {
	return nil
}
