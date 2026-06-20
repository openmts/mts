package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/querylang"
)

func TestQuerySpecRowsUsesStructuredMainPath(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []model.Point{queryBuilderPoint("a", 1, 0.8, 55)}, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Flush() error = %v close = %v", err, closeErr)
	}
	spec, err := querylang.NewBuilder().
		Select("usage").
		From("default", "autogen", "cpu").
		Where(querylang.FieldGT("usage", model.Float64Value(0.5))).
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	rows, err := eng.QuerySpecRows(ctx, spec)
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QuerySpecRows() error = %v close = %v", err, closeErr)
	}
	if len(rows) != 1 || rows[0].Fields["usage"].Float64 != 0.8 {
		closeErr := eng.Close(ctx)
		t.Fatalf("rows = %#v, want usage row close = %v", rows, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
