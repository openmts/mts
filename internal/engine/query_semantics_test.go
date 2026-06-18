package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestAggregateWindowMergesAcrossShardAndPartBoundaries(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Second,
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := []model.Point{
		aggregatePoint(int64(500*time.Millisecond), 1),
		aggregatePoint(int64(1500*time.Millisecond), 2),
		aggregatePoint(int64(2500*time.Millisecond), 3),
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Flush() error = %v close = %v", err, closeErr)
	}
	columns, err := eng.QueryColumns(ctx, model.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields:      []string{"value"},
		StartTime:   0,
		EndTime:     int64(3 * time.Second),
		Aggregates:  []model.AggregateSpec{{Field: "value", Function: "sum"}},
		Window:      time.Second,
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryColumns() error = %v close = %v", err, closeErr)
	}
	if len(columns) != 1 || len(columns[0].Values) != 3 {
		closeErr := eng.Close(ctx)
		t.Fatalf("aggregate columns = %#v, want one column with three windows close = %v", columns, closeErr)
	}
	for index, want := range []float64{1, 2, 3} {
		if columns[0].Values[index].Float64 != want {
			closeErr := eng.Close(ctx)
			t.Fatalf("window %d value = %v, want %v close = %v", index, columns[0].Values[index].Float64, want, closeErr)
		}
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestQueryDeduplicatesOverlappingLevelsByWriteSeq(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []model.Point{aggregatePoint(10, 1)}, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("first Write() error = %v close = %v", err, closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("first Flush() error = %v close = %v", err, closeErr)
	}
	if err := eng.Write(ctx, []model.Point{aggregatePoint(10, 9)}, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("second Write() error = %v close = %v", err, closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("second Flush() error = %v close = %v", err, closeErr)
	}
	columns, err := eng.QueryColumns(ctx, model.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields:      []string{"value"},
		StartTime:   0,
		EndTime:     20,
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryColumns() error = %v close = %v", err, closeErr)
	}
	if len(columns) != 1 || len(columns[0].Values) != 1 || columns[0].Values[0].Float64 != 9 {
		closeErr := eng.Close(ctx)
		t.Fatalf("dedup columns = %#v, want latest value 9 close = %v", columns, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestQueryTombstoneFiltersOverlappingLevels(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := []model.Point{
		aggregatePoint(1, 1),
		aggregatePoint(2, 2),
		aggregatePoint(3, 3),
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Flush() error = %v close = %v", err, closeErr)
	}
	shard := onlyShardForTest(t, eng)
	if err := shard.DeleteRange(model.Tombstone{StartTime: 2, EndTime: 2}, true); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("DeleteRange() error = %v close = %v", err, closeErr)
	}
	columns, err := eng.QueryColumns(ctx, model.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields:      []string{"value"},
		StartTime:   0,
		EndTime:     10,
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryColumns() error = %v close = %v", err, closeErr)
	}
	if len(columns) != 1 || len(columns[0].Values) != 2 {
		closeErr := eng.Close(ctx)
		t.Fatalf("tombstone columns = %#v, want two remaining samples close = %v", columns, closeErr)
	}
	if columns[0].Timestamps[0] != 1 || columns[0].Timestamps[1] != 3 {
		closeErr := eng.Close(ctx)
		t.Fatalf("timestamps = %v, want [1 3] close = %v", columns[0].Timestamps, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestFirstAggregateUsesBoundaryPageFastPath(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1000,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := make([]model.Point, 0, 768)
	for index := 0; index < 768; index++ {
		points = append(points, aggregatePoint(int64(index), float64(index)))
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Flush() error = %v close = %v", err, closeErr)
	}
	columns, err := eng.QueryColumns(ctx, model.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields:      []string{"value"},
		StartTime:   0,
		EndTime:     767,
		Aggregates:  []model.AggregateSpec{{Field: "value", Function: "first"}},
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryColumns() error = %v close = %v", err, closeErr)
	}
	if len(columns) != 1 || len(columns[0].Values) != 1 || columns[0].Values[0].Float64 != 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("first aggregate = %#v, want first value 0 close = %v", columns, closeErr)
	}
	stats := eng.QueryStatsSnapshot()
	if stats.ValuePagesRead != 1 || stats.ValuePagesSkipped != 2 {
		closeErr := eng.Close(ctx)
		t.Fatalf("query stats = %#v, want first page fast path close = %v", stats, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func aggregatePoint(timestamp int64, value float64) model.Point {
	return model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   timestamp,
		Fields:      map[string]model.FieldValue{"value": model.Float64Value(value)},
	}
}
