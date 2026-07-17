package mts

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPublicDeleteRemovesMatchingSeries(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, DefaultOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []Point{
		{Measurement: "cpu", Tags: map[string]string{"host": "a"}, Timestamp: 10, Fields: map[string]FieldValue{"v": Float64Value(1)}},
		{Measurement: "cpu", Tags: map[string]string{"host": "b"}, Timestamp: 20, Fields: map[string]FieldValue{"v": Float64Value(2)}},
	}, WriteOptions{Sync: true}); err != nil {
		_ = eng.Close(ctx)
		t.Fatalf("Write() error = %v", err)
	}
	if err := eng.Delete(ctx, DeleteRequest{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		StartTime:   0,
		EndTime:     100,
	}); err != nil {
		_ = eng.Close(ctx)
		t.Fatalf("Delete() error = %v", err)
	}
	rows, err := eng.QueryRows(ctx, Query{Measurement: "cpu", StartTime: 0, EndTime: 100})
	if err != nil {
		_ = eng.Close(ctx)
		t.Fatalf("QueryRows() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Tags["host"] != "b" {
		_ = eng.Close(ctx)
		t.Fatalf("rows = %#v, want only host=b", rows)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestPublicWriteRejectsCardinalityLimit(t *testing.T) {
	ctx := context.Background()
	opts := DefaultOptions(t.TempDir())
	opts.Cardinality = CardinalityOptions{MaxSeries: 1}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []Point{{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   1,
		Fields:      map[string]FieldValue{"v": Float64Value(1)},
	}}, WriteOptions{}); err != nil {
		_ = eng.Close(ctx)
		t.Fatalf("Write(first) error = %v", err)
	}
	err = eng.Write(ctx, []Point{{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "b"},
		Timestamp:   2,
		Fields:      map[string]FieldValue{"v": Float64Value(1)},
	}}, WriteOptions{})
	if !errors.Is(err, ErrCardinalityLimit) {
		_ = eng.Close(ctx)
		t.Fatalf("Write(second) error = %v, want ErrCardinalityLimit", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestDeleteRequestRejectsEmptyMeasurement(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, DefaultOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	err = eng.Delete(ctx, DeleteRequest{StartTime: 0, EndTime: 1})
	if !errors.Is(err, ErrInvalidOptions) {
		_ = eng.Close(ctx)
		t.Fatalf("Delete() error = %v, want ErrInvalidOptions", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestOptionsValidateCardinality(t *testing.T) {
	opts := DefaultOptions(t.TempDir())
	opts.Cardinality.MaxSeries = -1
	if err := opts.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want invalid max series")
	}
	_ = time.Second
}
