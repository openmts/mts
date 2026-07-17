package engine

import (
	"context"
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
