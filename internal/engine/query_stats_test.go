package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestQueryStatsRecordsShardPartPageAndSampleCounts(t *testing.T) {
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
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   10,
			Fields:      map[string]model.FieldValue{"load": model.Float64Value(1)},
		},
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "b"},
			Timestamp:   int64(time.Hour) + 10,
			Fields:      map[string]model.FieldValue{"load": model.Float64Value(2)},
		},
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Flush() error = %v close = %v", err, closeErr)
	}
	iter, err := eng.QueryColumnIterator(ctx, model.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields:      []string{"load"},
		StartTime:   0,
		EndTime:     int64(time.Hour) - 1,
	})
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QueryColumnIterator() error = %v close = %v", err, closeErr)
	}
	var samples int
	for iter.Next() {
		samples += len(iter.Column().Values)
	}
	if err := iter.Err(); err != nil {
		closeErr := errorsJoin(iter.Close(), eng.Close(ctx))
		t.Fatalf("iterator Err() = %v close = %v", err, closeErr)
	}
	if err := iter.Close(); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("iterator Close() error = %v engine close = %v", err, closeErr)
	}
	if samples != 1 {
		closeErr := eng.Close(ctx)
		t.Fatalf("samples = %d, want 1 close = %v", samples, closeErr)
	}
	stats := eng.QueryStatsSnapshot()
	if stats.CandidateShards != 2 || stats.ShardsScanned != 1 || stats.ShardsSkipped != 1 {
		closeErr := eng.Close(ctx)
		t.Fatalf("shard stats = %#v, want 2 candidates, 1 scanned, 1 skipped close = %v", stats, closeErr)
	}
	if stats.PartsScanned != 1 || stats.SamplesReturned != 1 {
		closeErr := eng.Close(ctx)
		t.Fatalf("query stats = %#v, want one part and one returned sample close = %v", stats, closeErr)
	}
	if stats.IndexRowsRead == 0 || stats.ValueBlocksRead == 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("read stats = %#v, want index and value reads close = %v", stats, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func errorsJoin(errs ...error) error {
	var out error
	for _, err := range errs {
		out = errors.Join(out, err)
	}
	return out
}
