package engine

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestEngineWriteAcrossShardsParallelCorrectness(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 10000,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	const workers = 8
	const perWorker = 50
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			points := make([]model.Point, 0, perWorker)
			for index := 0; index < perWorker; index++ {
				// 交错两个小时分片，强制跨 shard。
				ts := int64(index) + int64(id)*int64(time.Hour/2)
				if index%2 == 1 {
					ts += int64(time.Hour)
				}
				points = append(points, model.Point{
					Measurement: "cpu",
					Tags:        map[string]string{"host": "a"},
					Timestamp:   ts,
					Fields:      map[string]model.FieldValue{"v": model.Float64Value(float64(id*1000 + index))},
				})
			}
			errCh <- eng.Write(ctx, points, model.WriteOptions{})
		}(worker)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			closeTestEngine(t, ctx, eng)
			t.Fatalf("concurrent Write() error = %v", err)
		}
	}

	rows, err := eng.QueryRows(ctx, model.Query{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     int64(10 * time.Hour),
	})
	if err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("QueryRows() error = %v", err)
	}
	if len(rows) != workers*perWorker {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("row count = %d, want %d", len(rows), workers*perWorker)
	}
	closeTestEngine(t, ctx, eng)
}

func TestEngineSingleBatchAcrossShardsRemainsComplete(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1000,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := []model.Point{
		{Measurement: "m", Timestamp: 1, Fields: map[string]model.FieldValue{"v": model.Float64Value(1)}},
		{Measurement: "m", Timestamp: int64(time.Hour) + 1, Fields: map[string]model.FieldValue{"v": model.Float64Value(2)}},
		{Measurement: "m", Timestamp: int64(2*time.Hour) + 1, Fields: map[string]model.FieldValue{"v": model.Float64Value(3)}},
	}
	if err := eng.Write(ctx, points, model.WriteOptions{Sync: true}); err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("Write() error = %v", err)
	}
	rows, err := eng.QueryRows(ctx, model.Query{Measurement: "m", StartTime: 0, EndTime: int64(3 * time.Hour)})
	if err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("QueryRows() error = %v", err)
	}
	if len(rows) != 3 {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("row count = %d, want 3", len(rows))
	}
	closeTestEngine(t, ctx, eng)
}

func TestWriteIngestMetricsTrackParallelBatches(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 1000,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := []model.Point{
		{Measurement: "m", Timestamp: 1, Fields: map[string]model.FieldValue{"v": model.Float64Value(1)}},
		{Measurement: "m", Timestamp: int64(time.Hour) + 1, Fields: map[string]model.FieldValue{"v": model.Float64Value(2)}},
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("Write() error = %v", err)
	}
	_, parallelBatches, parallelShards, parallelErrors := eng.writeIngest.snapshot()
	if parallelBatches != 1 {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("parallelBatches = %d, want 1", parallelBatches)
	}
	if parallelShards < 2 {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("parallelShards = %d, want >= 2", parallelShards)
	}
	if parallelErrors != 0 {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("parallelErrors = %d, want 0", parallelErrors)
	}
	found := false
	for _, metric := range eng.MetricsSnapshot() {
		if metric.Name == "mts_write_parallel_batches_total" && metric.Value >= 1 {
			found = true
			break
		}
	}
	if !found {
		closeTestEngine(t, ctx, eng)
		t.Fatal("mts_write_parallel_batches_total not found in metrics snapshot")
	}
	closeTestEngine(t, ctx, eng)
}

func TestWriteShardBatchesEmptyAndSinglePath(t *testing.T) {
	if err := writeShardBatches(nil, false); err != nil {
		t.Fatalf("nil batches error = %v", err)
	}
	if err := writeTypedShardBatches(model.ResolvedTypedBatch{}, nil, false); err != nil {
		t.Fatalf("nil typed batches error = %v", err)
	}
}

func TestWriteIngestStatsRecord(t *testing.T) {
	var stats writeIngestStats
	stats.record(2, true, nil)
	stats.record(1, false, nil)
	stats.record(3, true, errors.New("boom"))
	batches, parallelBatches, parallelShards, parallelErrors := stats.snapshot()
	if batches != 6 || parallelBatches != 2 || parallelShards != 5 || parallelErrors != 1 {
		t.Fatalf("stats = %d/%d/%d/%d, want 6/2/5/1", batches, parallelBatches, parallelShards, parallelErrors)
	}
}
