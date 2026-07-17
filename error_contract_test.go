package mts_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func TestPublicErrorCategories(t *testing.T) {
	ctx := context.Background()
	engine, err := mts.Open(ctx, mts.DefaultOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := engine.Write(ctx, []mts.Point{{
		Measurement: "cpu",
		Timestamp:   1,
		Fields:      map[string]mts.FieldValue{"usage": mts.Float64Value(1)},
	}}, mts.WriteOptions{}); err != nil {
		closeErr := engine.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if _, err := engine.RunDownsamplePolicy(ctx, "missing", time.Now()); !errors.Is(err, mts.ErrNotFound) {
		closeErr := engine.Close(ctx)
		t.Fatalf("RunDownsamplePolicy(missing) error = %v, want ErrNotFound close = %v", err, closeErr)
	}
	query, err := mts.NewQuery().
		From("", "", "cpu").
		Aggregate("unsupported", "usage").
		Build()
	if err != nil {
		closeErr := engine.Close(ctx)
		t.Fatalf("Build() error = %v close = %v", err, closeErr)
	}
	if _, err := engine.QueryColumns(ctx, query); !errors.Is(err, mts.ErrUnsupported) {
		closeErr := engine.Close(ctx)
		t.Fatalf("QueryColumns(unsupported) error = %v, want ErrUnsupported close = %v", err, closeErr)
	}
	if err := engine.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestPublicErrorMapsResourceExhaustedSentinels(t *testing.T) {
	ctx := context.Background()
	opts := mts.DefaultOptions(t.TempDir())
	opts.Cardinality = mts.CardinalityOptions{MaxSeries: 1}
	eng, err := mts.Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []mts.Point{{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   1,
		Fields:      map[string]mts.FieldValue{"v": mts.Float64Value(1)},
	}}, mts.WriteOptions{}); err != nil {
		_ = eng.Close(ctx)
		t.Fatalf("Write(first) error = %v", err)
	}
	err = eng.Write(ctx, []mts.Point{{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "b"},
		Timestamp:   2,
		Fields:      map[string]mts.FieldValue{"v": mts.Float64Value(1)},
	}}, mts.WriteOptions{})
	if !errors.Is(err, mts.ErrCardinalityLimit) || !errors.Is(err, mts.ErrResourceExhausted) {
		_ = eng.Close(ctx)
		t.Fatalf("Write(second) error = %v, want cardinality+resource exhausted", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
