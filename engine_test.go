package mts_test

import (
	"context"
	"testing"
	"time"

	mts "codeberg.org/mts/mts"
)

func TestEngineWriteFlushReopenQueryRows(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	opts := mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 2,
	}
	eng, err := mts.Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
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
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	eng, err = mts.Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() after close error = %v", err)
	}
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
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() after query error = %v", err)
	}
}

func TestEngineReplaysUnflushedWAL(t *testing.T) {
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
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	eng, err = mts.Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() after close error = %v", err)
	}
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
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() after replay query error = %v", err)
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
