package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestEngineUsesLocalMetadataStoreAcrossRestart(t *testing.T) {
	ctx := context.Background()
	opts := model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1,
	}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, ok := eng.metadata.(*LocalMetadataStore); !ok {
		t.Fatalf("metadata store type = %T, want *LocalMetadataStore", eng.metadata)
	}
	point := model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   10,
		Fields: map[string]model.FieldValue{
			"usage": model.Float64Value(42),
		},
	}
	if err := eng.Write(ctx, []model.Point{point}, model.WriteOptions{Sync: true}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open(restart) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(ctx); err != nil {
			t.Fatalf("Close(restart) error = %v", err)
		}
	}()
	if _, ok := reopened.metadata.(*LocalMetadataStore); !ok {
		t.Fatalf("reopened metadata store type = %T, want *LocalMetadataStore", reopened.metadata)
	}
	rows, err := reopened.QueryRows(ctx, model.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		StartTime:   0,
		EndTime:     20,
	})
	if err != nil {
		t.Fatalf("QueryRows(restart) error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	if rows[0].Fields["usage"].Float64 != 42 {
		t.Fatalf("usage = %v, want 42", rows[0].Fields["usage"].Float64)
	}
}
