package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestProductionMetricsIncludeMemTableOrderCounters(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := []model.Point{
		{Measurement: "cpu", Timestamp: 10, Fields: map[string]model.FieldValue{"v": model.Float64Value(1)}},
		{Measurement: "cpu", Timestamp: 20, Fields: map[string]model.FieldValue{"v": model.Float64Value(2)}},
		{Measurement: "cpu", Timestamp: 20, Fields: map[string]model.FieldValue{"v": model.Float64Value(3)}},
		{Measurement: "cpu", Timestamp: 5, Fields: map[string]model.FieldValue{"v": model.Float64Value(4)}},
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		_ = eng.Close(ctx)
		t.Fatalf("Write() error = %v", err)
	}
	metrics := eng.productionMetricsSnapshot()
	if metrics.MemTableAppended < 4 {
		_ = eng.Close(ctx)
		t.Fatalf("MemTableAppended = %d, want >=4", metrics.MemTableAppended)
	}
	if metrics.MemTableDuplicates < 1 {
		_ = eng.Close(ctx)
		t.Fatalf("MemTableDuplicates = %d, want >=1", metrics.MemTableDuplicates)
	}
	if metrics.MemTableOutOfOrder < 1 {
		_ = eng.Close(ctx)
		t.Fatalf("MemTableOutOfOrder = %d, want >=1", metrics.MemTableOutOfOrder)
	}
	found := map[string]bool{}
	for _, metric := range eng.MetricsSnapshot() {
		switch metric.Name {
		case "mts_memtable_out_of_order_samples_total",
			"mts_memtable_duplicate_samples_total",
			"mts_memtable_appended_samples_total":
			found[metric.Name] = true
		}
	}
	for name := range map[string]struct{}{
		"mts_memtable_out_of_order_samples_total": {},
		"mts_memtable_duplicate_samples_total":    {},
		"mts_memtable_appended_samples_total":     {},
	} {
		if !found[name] {
			_ = eng.Close(ctx)
			t.Fatalf("missing metric %s", name)
		}
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
