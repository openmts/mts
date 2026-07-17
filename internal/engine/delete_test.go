package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestEngineDeleteRemovesMatchingSamples(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []model.Point{
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   10,
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
		},
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "b"},
			Timestamp:   20,
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(2)},
		},
	}, model.WriteOptions{Sync: true}); err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("Write() error = %v", err)
	}

	if err := eng.Delete(ctx, model.DeleteRequest{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		StartTime:   0,
		EndTime:     100,
	}); err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("Delete() error = %v", err)
	}

	rows, err := eng.QueryRows(ctx, model.Query{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     100,
	})
	if err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("QueryRows() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Tags["host"] != "b" {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("rows = %#v, want only host=b", rows)
	}
	closeTestEngine(t, ctx, eng)
}

func TestEngineDeleteByMeasurementTimeRange(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []model.Point{
		{Measurement: "cpu", Timestamp: 10, Fields: map[string]model.FieldValue{"v": model.Float64Value(1)}},
		{Measurement: "cpu", Timestamp: 50, Fields: map[string]model.FieldValue{"v": model.Float64Value(2)}},
	}, model.WriteOptions{Sync: true}); err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("Write() error = %v", err)
	}
	if err := eng.Delete(ctx, model.DeleteRequest{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     30,
	}); err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("Delete() error = %v", err)
	}
	rows, err := eng.QueryRows(ctx, model.Query{Measurement: "cpu", StartTime: 0, EndTime: 100})
	if err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("QueryRows() error = %v", err)
	}
	if len(rows) != 1 || rows[0].Fields["v"].Float64 != 2 {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("rows = %#v, want only timestamp 50", rows)
	}
	closeTestEngine(t, ctx, eng)
}
