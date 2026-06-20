package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/querylang"
)

func TestEngineWriteTypedBatchFlushReopenAndQuerySpec(t *testing.T) {
	ctx := context.Background()
	opts := model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	batch := model.TypedBatch{
		Measurement: "cpu",
		Timestamps: []int64{
			int64(10 * time.Minute),
			int64(20 * time.Minute),
			int64(70 * time.Minute),
		},
		Tags: []model.TagColumn{
			{Name: "host", Values: []string{"a", "b", "a"}},
			{Name: "region", Values: []string{"west", "west", "east"}},
		},
		Fields: []model.TypedFieldColumn{
			{Name: "usage", Type: model.FieldFloat64, Float64Values: []float64{0.4, 0.9, 0.7}},
			{Name: "count", Type: model.FieldInt64, Int64Values: []int64{4, 9, 7}},
			{Name: "state", Type: model.FieldString, StringValues: []string{"ok", "warn", "ok"}},
			{Name: "ready", Type: model.FieldBool, BoolValues: []bool{true, false, true}},
		},
	}
	if err := eng.WriteTypedBatch(ctx, batch, model.WriteOptions{Sync: true}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("WriteTypedBatch() error = %v close = %v", err, closeErr)
	}
	if got := eng.StorageMemorySnapshot().MemTableBytes; got <= 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("MemTableBytes = %d, want active typed batch memory close = %v", got, closeErr)
	}
	if err := eng.Flush(ctx); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Flush() error = %v close = %v", err, closeErr)
	}
	_ = eng.RecoveryReports(ctx)

	spec, err := querylang.NewBuilder().
		Select("usage", "state").
		From("default", "autogen", "cpu").
		Where(querylang.TagIn("host", "a"), querylang.FieldGTE("usage", model.Float64Value(0.5))).
		OrderByTimeDesc().
		Build()
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("Build() error = %v close = %v", err, closeErr)
	}
	rows, err := eng.QuerySpecRows(ctx, spec)
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QuerySpecRows() error = %v close = %v", err, closeErr)
	}
	if len(rows) != 1 || rows[0].Fields["usage"].Float64 != 0.7 ||
		rows[0].Fields["state"].String != "ok" {
		closeErr := eng.Close(ctx)
		t.Fatalf("rows = %#v, want east host=a usage 0.7 close = %v", rows, closeErr)
	}
	columns, err := eng.QuerySpecColumns(ctx, spec)
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QuerySpecColumns() error = %v close = %v", err, closeErr)
	}
	if len(columns) != 2 {
		closeErr := eng.Close(ctx)
		t.Fatalf("columns = %#v, want usage and state close = %v", columns, closeErr)
	}
	stream, err := eng.QuerySpecRowStream(ctx, spec)
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QuerySpecRowStream() error = %v close = %v", err, closeErr)
	}
	var streamed int
	for stream.Next() {
		streamed++
	}
	if err := stream.Err(); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("stream Err() = %v close = %v", err, closeErr)
	}
	if err := stream.Close(); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("stream Close() error = %v engine close = %v", err, closeErr)
	}
	if streamed != 1 {
		closeErr := eng.Close(ctx)
		t.Fatalf("streamed rows = %d, want 1 close = %v", streamed, closeErr)
	}
	explainColumns, explain, stats, err := eng.QuerySpecWithExplain(ctx, spec)
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QuerySpecWithExplain() error = %v close = %v", err, closeErr)
	}
	if len(explainColumns) != 2 || explain.Measurement != "cpu" || stats.PartsScanned == 0 {
		closeErr := eng.Close(ctx)
		t.Fatalf("explain columns=%#v explain=%#v stats=%#v close=%v", explainColumns, explain, stats, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	eng, err = Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open(reopen) error = %v", err)
	}
	rows, err = eng.QuerySpecRows(ctx, spec)
	if err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("QuerySpecRows(reopen) error = %v close = %v", err, closeErr)
	}
	if len(rows) != 1 || rows[0].Timestamp != int64(70*time.Minute) {
		closeErr := eng.Close(ctx)
		t.Fatalf("reopened rows = %#v, want persisted latest row close = %v", rows, closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close(reopen) error = %v", err)
	}
}

func TestEngineWriteTypedBatchRejectsMemoryAndInvalidInput(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
		StorageMemory: model.StorageMemoryOptions{
			HardSampleLimit: 1,
		},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	batch := model.TypedBatch{
		Measurement: "cpu",
		Timestamps:  []int64{1},
		Fields: []model.TypedFieldColumn{
			{Name: "usage", Type: model.FieldFloat64, Float64Values: []float64{1}},
			{Name: "count", Type: model.FieldInt64, Int64Values: []int64{1}},
		},
	}
	if err := eng.WriteTypedBatch(ctx, batch, model.WriteOptions{}); err == nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("WriteTypedBatch(over hard limit) error = nil, want error close = %v", closeErr)
	}
	if err := eng.WriteTypedBatch(ctx, model.TypedBatch{}, model.WriteOptions{}); err != nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("WriteTypedBatch(empty) error = %v close = %v", err, closeErr)
	}
	invalid := model.TypedBatch{
		Measurement: "cpu",
		Timestamps:  []int64{1},
		Fields:      []model.TypedFieldColumn{{Name: "usage", Type: model.FieldFloat64}},
	}
	if err := eng.WriteTypedBatch(ctx, invalid, model.WriteOptions{}); err == nil {
		closeErr := eng.Close(ctx)
		t.Fatalf("WriteTypedBatch(invalid) error = nil, want validation error close = %v", closeErr)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
