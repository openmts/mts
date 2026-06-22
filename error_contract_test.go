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
