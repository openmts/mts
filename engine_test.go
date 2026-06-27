package mts_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func TestEngineWriteFlushReopenQueryRows(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	opts := mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 2,
	}
	eng := openTestEngine(t, ctx, opts)
	point := mts.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   int64(10),
		Fields: map[string]mts.FieldValue{
			"state": mts.StringValue("ok"),
			"usage": mts.Float64Value(1.5),
		},
	}
	if err := eng.Write(ctx, []mts.Point{point}, mts.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := eng.Flush(ctx); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	closeTestEngine(t, ctx, eng)

	eng = openTestEngine(t, ctx, opts)
	columns, err := eng.QueryColumns(ctx, mts.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		StartTime:   0,
		EndTime:     int64(time.Hour),
	})
	if err != nil {
		t.Fatalf("QueryColumns() error = %v", err)
	}
	if len(columns) != 2 {
		t.Fatalf("column count = %d, want 2", len(columns))
	}
	iter, err := eng.QueryColumnIterator(ctx, mts.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		StartTime:   0,
		EndTime:     int64(time.Hour),
	})
	if err != nil {
		t.Fatalf("QueryColumnIterator() error = %v", err)
	}
	iterCount := 0
	for iter.Next() {
		if iter.Column().SeriesID == 0 {
			t.Fatal("column iterator returned zero column")
		}
		iterCount++
	}
	if err := iter.Err(); err != nil {
		t.Fatalf("column iterator Err() = %v", err)
	}
	if err := iter.Close(); err != nil {
		t.Fatalf("column iterator Close() error = %v", err)
	}
	if iterCount != 2 {
		t.Fatalf("iterator column count = %d, want 2", iterCount)
	}
	rows, err := eng.QueryRows(ctx, mts.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		StartTime:   0,
		EndTime:     int64(time.Hour),
	})
	if err != nil {
		t.Fatalf("QueryRows() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	rowIter, err := eng.QueryRowIterator(ctx, mts.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		StartTime:   0,
		EndTime:     int64(time.Hour),
	})
	if err != nil {
		t.Fatalf("QueryRowIterator() error = %v", err)
	}
	rowCount := 0
	for rowIter.Next() {
		if rowIter.Row().SeriesID == 0 {
			t.Fatal("row iterator returned zero row")
		}
		rowCount++
	}
	if err := rowIter.Err(); err != nil {
		t.Fatalf("row iterator Err() = %v", err)
	}
	if err := rowIter.Close(); err != nil {
		t.Fatalf("row iterator Close() error = %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("iterator row count = %d, want 1", rowCount)
	}
	if rows[0].Fields["usage"].Float64 != 1.5 {
		t.Fatalf("usage = %v, want 1.5", rows[0].Fields["usage"].Float64)
	}
	if rows[0].Fields["state"].String != "ok" {
		t.Fatalf("state = %q, want ok", rows[0].Fields["state"].String)
	}
	closeTestEngine(t, ctx, eng)
}

func TestEngineReplaysUnflushedWAL(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	opts := mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	}
	eng := openTestEngine(t, ctx, opts)
	point := mts.Point{
		Measurement: "mem",
		Timestamp:   int64(15),
		Fields: map[string]mts.FieldValue{
			"used": mts.Int64Value(9),
		},
	}
	if err := eng.Write(ctx, []mts.Point{point}, mts.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	closeTestEngine(t, ctx, eng)

	eng = openTestEngine(t, ctx, opts)
	rows, err := eng.QueryRows(ctx, mts.Query{
		Measurement: "mem",
		StartTime:   0,
		EndTime:     20,
	})
	if err != nil {
		t.Fatalf("QueryRows() after replay error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count after replay = %d, want 1", len(rows))
	}
	if rows[0].Fields["used"].Int64 != 9 {
		t.Fatalf("used = %d, want 9", rows[0].Fields["used"].Int64)
	}
	closeTestEngine(t, ctx, eng)
}

func TestEngineWriteTypedBatchFlushReopenQueryRows(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	opts := mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	}
	eng, err := mts.Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	batch := mts.TypedBatch{
		Measurement: "typed",
		Tags: []mts.TagColumn{
			{Name: "host", Values: []string{"a", "a", "b"}},
		},
		Timestamps: []int64{10, 20, 30},
		Fields: []mts.TypedFieldColumn{
			{Name: "usage", Type: mts.FieldFloat64, Float64Values: []float64{1.5, 2.5, 3.5}},
			{Name: "count", Type: mts.FieldInt64, Int64Values: []int64{1, 2, 3}},
			{Name: "state", Type: mts.FieldString, StringValues: []string{"ok", "warn", "ok"}},
			{Name: "active", Type: mts.FieldBool, BoolValues: []bool{true, false, true}},
		},
	}
	if err := eng.WriteTypedBatch(ctx, batch, mts.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("WriteTypedBatch() error = %v", err)
	}
	if err := eng.Flush(ctx); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := mts.Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open(reopen) error = %v", err)
	}
	rows, err := reopened.QueryRows(ctx, mts.Query{
		Measurement: "typed",
		Tags:        map[string]string{"host": "a"},
		StartTime:   0,
		EndTime:     100,
	})
	if err != nil {
		closeErr := reopened.Close(ctx)
		t.Fatalf("QueryRows() error = %v close = %v", err, closeErr)
	}
	if len(rows) != 2 {
		closeErr := reopened.Close(ctx)
		t.Fatalf("row count = %d, want 2 close = %v", len(rows), closeErr)
	}
	if rows[0].Fields["usage"].Float64 != 1.5 || rows[1].Fields["state"].String != "warn" {
		closeErr := reopened.Close(ctx)
		t.Fatalf("rows = %#v, want typed field values close = %v", rows, closeErr)
	}
	if err := reopened.Close(ctx); err != nil {
		t.Fatalf("Close(reopened) error = %v", err)
	}
}

func TestEngineWriteTypedBatchRejectsInvalidColumnLength(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	err = eng.WriteTypedBatch(ctx, mts.TypedBatch{
		Measurement: "typed",
		Timestamps:  []int64{10, 20},
		Fields: []mts.TypedFieldColumn{
			{Name: "usage", Type: mts.FieldFloat64, Float64Values: []float64{1.5}},
		},
	}, mts.WriteOptions{})
	closeErr := eng.Close(ctx)
	if err == nil {
		t.Fatalf("WriteTypedBatch() error = nil, want length error close = %v", closeErr)
	}
	if closeErr != nil {
		t.Fatalf("Close() error = %v", closeErr)
	}
}

func TestPublicWriteCopiesPointInput(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.DefaultOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	tags := map[string]string{"host": "a"}
	fields := map[string]mts.FieldValue{"usage": mts.Float64Value(1)}
	if err := eng.Write(ctx, []mts.Point{{
		Measurement: "cpu",
		Tags:        tags,
		Timestamp:   1,
		Fields:      fields,
	}}, mts.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	tags["host"] = "mutated"
	fields["usage"] = mts.Float64Value(99)
	rows, err := eng.QueryRows(ctx, mts.Query{Measurement: "cpu", StartTime: 0, EndTime: 2})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryRows() error = %v close = %v", err, closeErr)
	}
	if len(rows) != 1 || rows[0].Tags["host"] != "a" || rows[0].Fields["usage"].Float64 != 1 {
		closeErr := eng.Close(ctx)
		t.Fatalf("rows = %#v, want copied input close = %v", rows, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestPublicWriteTypedBatchCopiesInput(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.DefaultOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	tagValues := []string{"a"}
	timestamps := []int64{1}
	fieldValues := []float64{1}
	if err := eng.WriteTypedBatch(ctx, mts.TypedBatch{
		Measurement: "cpu",
		Tags:        []mts.TagColumn{{Name: "host", Values: tagValues}},
		Timestamps:  timestamps,
		Fields: []mts.TypedFieldColumn{
			{Name: "usage", Type: mts.FieldFloat64, Float64Values: fieldValues},
		},
	}, mts.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("WriteTypedBatch() error = %v close = %v", err, closeErr)
	}
	tagValues[0] = "mutated"
	timestamps[0] = 99
	fieldValues[0] = 99
	rows, err := eng.QueryRows(ctx, mts.Query{Measurement: "cpu", StartTime: 0, EndTime: 2})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryRows() error = %v close = %v", err, closeErr)
	}
	if len(rows) != 1 || rows[0].Tags["host"] != "a" || rows[0].Fields["usage"].Float64 != 1 {
		closeErr := eng.Close(ctx)
		t.Fatalf("rows = %#v, want copied typed batch close = %v", rows, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestPublicAPIReturnsErrorsForInvalidPathAndCanceledQuery(t *testing.T) {
	ctx := context.Background()
	if _, err := mts.Open(ctx, mts.Options{Path: "bad\x00path"}); err == nil {
		t.Fatal("Open(invalid path) error = nil, want error")
	}

	eng, err := mts.Open(ctx, mts.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 10,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	query := mts.Query{Measurement: "cpu", StartTime: 0, EndTime: 1}
	if _, err := eng.QueryColumns(canceled, query); !errors.Is(err, context.Canceled) {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryColumns(canceled) error = %v, want context.Canceled close = %v", err, closeErr)
	}
	if _, err := eng.QueryColumnIterator(canceled, query); !errors.Is(err, context.Canceled) {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryColumnIterator(canceled) error = %v, want context.Canceled close = %v", err, closeErr)
	}
	if _, err := eng.QueryRows(canceled, query); !errors.Is(err, context.Canceled) {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryRows(canceled) error = %v, want context.Canceled close = %v", err, closeErr)
	}
	if _, err := eng.QueryRowIterator(canceled, query); !errors.Is(err, context.Canceled) {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryRowIterator(canceled) error = %v, want context.Canceled close = %v", err, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestPublicHealthSnapshotIncludesStructuredChecksAndQueryStatsDetails(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []mts.Point{{
		Measurement: "public",
		Timestamp:   1,
		Fields: map[string]mts.FieldValue{
			"f0": mts.Float64Value(1),
			"f1": mts.Float64Value(2),
		},
	}}, mts.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if _, err := eng.QueryRows(ctx, mts.Query{
		Measurement: "public",
		StartTime:   0,
		EndTime:     10,
		Budget:      mts.QueryBudget{MaxSamples: 1},
	}); !errors.Is(err, mts.ErrReadBudgetExceeded) {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryRows() error = %v, want ErrReadBudgetExceeded close = %v", err, closeErr)
	}
	health := eng.HealthSnapshot()
	if len(health.Checks) == 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("HealthSnapshot().Checks empty close = %v", closeErr)
	}
	stats := eng.QueryStatsSnapshot()
	if stats.DurationNanos == 0 || stats.BudgetErrors == 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryStatsSnapshot() = %#v, want duration and budget errors close = %v", stats, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestPublicQueryWithExplainReturnsPlanAndStats(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.DefaultOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []mts.Point{{
		Measurement: "public",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   1,
		Fields: map[string]mts.FieldValue{
			"usage": mts.Float64Value(1),
		},
	}}, mts.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	query, err := mts.NewQuery().
		Select("usage").
		From("", "", "public").
		Where(mts.TagEq("host", "a")).
		TimeRange(0, 10).
		Build()
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Build() error = %v close = %v", err, closeErr)
	}
	query.Budget = mts.QueryBudget{MaxSamples: 10}
	result, err := eng.QueryWithExplain(ctx, query)
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryWithExplain() error = %v close = %v", err, closeErr)
	}
	if len(result.Columns) != 1 || result.Columns[0].FieldName != "usage" {
		closeErr := eng.Close(ctx)
		t.Fatalf("columns = %#v, want usage column close = %v", result.Columns, closeErr)
	}
	if result.Explain.Measurement != "public" || result.Explain.TagFilters["host"] != "a" {
		closeErr := eng.Close(ctx)
		t.Fatalf("explain = %#v, want public host=a close = %v", result.Explain, closeErr)
	}
	if result.Explain.Budget.MaxSamples != 10 {
		closeErr := eng.Close(ctx)
		t.Fatalf("explain budget = %#v, want max samples 10 close = %v", result.Explain.Budget, closeErr)
	}
	if result.Stats.SamplesReturned == 0 || result.Stats.StartedUnixNanos == 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("stats = %#v, want returned samples and start time close = %v", result.Stats, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestPublicCompactionResultAndMemorySnapshots(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	for index := range 2 {
		if err := eng.Write(ctx, []mts.Point{{
			Measurement: "compact",
			Timestamp:   int64(index),
			Fields:      map[string]mts.FieldValue{"value": mts.Int64Value(int64(index))},
		}}, mts.WriteOptions{}); err != nil {
			closeErr := eng.Close(ctx)
			t.Fatalf("Write(%d) error = %v close = %v", index, err, closeErr)
		}
	}
	memory := eng.StorageMemorySnapshot()
	if memory.RuntimeHeapAllocBytes == 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("StorageMemorySnapshot() = %#v, want runtime heap close = %v", memory, closeErr)
	}
	result, err := eng.CompactWithResult(ctx)
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("CompactWithResult() error = %v close = %v", err, closeErr)
	}
	if result.State == "" || result.InputParts < 2 || result.OutputParts == 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("result = %#v, want affected compaction close = %v", result, closeErr)
	}
	stats := eng.CompactionStatsSnapshot()
	if stats.Total == 0 || stats.LastTask.State == "" || stats.InputParts < result.InputParts {
		closeErr := eng.Close(ctx)
		t.Fatalf("CompactionStatsSnapshot() = %#v result=%#v close = %v", stats, result, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestPublicMetadataAPIs(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 10,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.CreateDatabase(ctx, "meta"); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("CreateDatabase() error = %v close = %v", err, closeErr)
	}
	if err := eng.CreateRetentionPolicy(ctx, "meta", mts.RetentionPolicy{Name: "rp", Duration: time.Hour}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("CreateRetentionPolicy() error = %v close = %v", err, closeErr)
	}
	point := mts.Point{
		Database:        "meta",
		RetentionPolicy: "rp",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a"},
		Timestamp:       1,
		Fields:          map[string]mts.FieldValue{"value": mts.Float64Value(1)},
	}
	if err := eng.Write(ctx, []mts.Point{point}, mts.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	policies, err := eng.ListRetentionPolicies(ctx, "meta")
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("ListRetentionPolicies() error = %v close = %v", err, closeErr)
	}
	if len(policies) == 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("ListRetentionPolicies() empty close = %v", closeErr)
	}
	fields, err := eng.ListFields(ctx, "meta", "cpu")
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("ListFields() error = %v close = %v", err, closeErr)
	}
	if len(fields) != 1 || fields[0].Name != "value" {
		closeErr := eng.Close(ctx)
		t.Fatalf("fields = %#v, want value close = %v", fields, closeErr)
	}
	series, err := eng.ListSeries(ctx, "meta", "cpu", map[string]string{"host": "a"})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("ListSeries() error = %v close = %v", err, closeErr)
	}
	if len(series) != 1 || series[0].Tags["host"] != "a" {
		closeErr := eng.Close(ctx)
		t.Fatalf("series = %#v, want host a close = %v", series, closeErr)
	}
	measurements, err := eng.ListMeasurements(ctx, "meta")
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("ListMeasurements() error = %v close = %v", err, closeErr)
	}
	if len(measurements) != 1 || measurements[0] != "cpu" {
		closeErr := eng.Close(ctx)
		t.Fatalf("measurements = %#v, want cpu close = %v", measurements, closeErr)
	}
	if err := eng.DropDatabase(ctx, "meta"); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("DropDatabase() error = %v close = %v", err, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestEngineCompactionAndRetention(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	opts := mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		Retention:          time.Hour,
		MemTableMaxSamples: 1,
	}
	eng, err := mts.Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	first := mts.Point{
		Measurement: "cpu",
		Timestamp:   10,
		Fields: map[string]mts.FieldValue{
			"value": mts.Float64Value(1),
		},
	}
	second := mts.Point{
		Measurement: "cpu",
		Timestamp:   10,
		Fields: map[string]mts.FieldValue{
			"value": mts.Float64Value(2),
		},
	}
	if err := eng.Write(ctx, []mts.Point{first}, mts.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write(first) error = %v", err)
	}
	if err := eng.Write(ctx, []mts.Point{second}, mts.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write(second) error = %v", err)
	}
	if err := eng.Compact(ctx); err != nil {
		t.Fatalf("Compact() error = %v", err)
	}
	rows, err := eng.QueryRows(ctx, mts.Query{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     20,
	})
	if err != nil {
		t.Fatalf("QueryRows() after compact error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count after compact = %d, want 1", len(rows))
	}
	if rows[0].Fields["value"].Float64 != 2 {
		t.Fatalf("compacted value = %v, want 2", rows[0].Fields["value"].Float64)
	}

	if err := eng.ApplyRetention(ctx, time.Unix(0, int64(2*time.Hour))); err != nil {
		t.Fatalf("ApplyRetention() error = %v", err)
	}
	rows, err = eng.QueryRows(ctx, mts.Query{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     20,
	})
	if err != nil {
		t.Fatalf("QueryRows() after retention error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("row count after retention = %d, want 0", len(rows))
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestEngineMetadataWrappers(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{Path: t.TempDir(), ShardDuration: time.Hour})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.CreateDatabase(ctx, "metrics"); err != nil {
		t.Fatalf("CreateDatabase() error = %v", err)
	}
	if err := eng.CreateRetentionPolicy(ctx, "metrics", mts.RetentionPolicy{Name: "hot"}); err != nil {
		t.Fatalf("CreateRetentionPolicy() error = %v", err)
	}
	point := mts.Point{
		Database:        "metrics",
		RetentionPolicy: "hot",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a"},
		Timestamp:       1,
		Fields:          map[string]mts.FieldValue{"usage": mts.Float64Value(1)},
	}
	if err := eng.Write(ctx, []mts.Point{point}, mts.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	policies, err := eng.ListRetentionPolicies(ctx, "metrics")
	if err != nil {
		t.Fatalf("ListRetentionPolicies() error = %v", err)
	}
	if len(policies) != 1 || policies[0].Name != "hot" {
		t.Fatalf("policies = %#v, want hot", policies)
	}
	measurements, err := eng.ListMeasurements(ctx, "metrics")
	if err != nil {
		t.Fatalf("ListMeasurements() error = %v", err)
	}
	if len(measurements) != 1 || measurements[0] != "cpu" {
		t.Fatalf("measurements = %#v, want cpu", measurements)
	}
	fields, err := eng.ListFields(ctx, "metrics", "cpu")
	if err != nil {
		t.Fatalf("ListFields() error = %v", err)
	}
	if len(fields) != 1 || fields[0].Name != "usage" {
		t.Fatalf("fields = %#v, want usage", fields)
	}
	series, err := eng.ListSeries(ctx, "metrics", "cpu", map[string]string{"host": "a"})
	if err != nil {
		t.Fatalf("ListSeries() error = %v", err)
	}
	if len(series) != 1 || series[0].Tags["host"] != "a" {
		t.Fatalf("series = %#v, want host=a", series)
	}
	if errs := eng.MaintenanceErrors(ctx); len(errs) != 0 {
		t.Fatalf("MaintenanceErrors() = %v, want none", errs)
	}
	if err := eng.DropDatabase(ctx, "metrics"); err != nil {
		t.Fatalf("DropDatabase() error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
