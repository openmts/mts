package engine

import (
	"context"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/querylang"
	"github.com/openmts/mts/internal/queryservice"
)

// 权威查询路径是 Engine + queryexec。
// LayeredExecutor 仅作为服务层可选包装，必须与 Engine.QuerySpec* 结果一致。
func TestLayeredExecutorMatchesEngineQueryAuthority(t *testing.T) {
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
			Timestamp:   int64(time.Minute),
			Fields:      map[string]model.FieldValue{"usage": model.Float64Value(0.5)},
		},
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   int64(2 * time.Minute),
			Fields:      map[string]model.FieldValue{"usage": model.Float64Value(1.5)},
		},
	}, model.WriteOptions{Sync: true}); err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("Write() error = %v", err)
	}

	query := model.Query{
		Database:        "default",
		RetentionPolicy: "autogen",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a"},
		Fields:          []string{"usage"},
		StartTime:       0,
		EndTime:         int64(3 * time.Minute),
		Order:           model.QueryOrder{By: model.QueryOrderByTime, Direction: model.QuerySortAsc},
	}

	wantRows, err := eng.QueryRows(ctx, query)
	if err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("QueryRows() error = %v", err)
	}

	layered := queryservice.NewLayeredExecutor(eng)
	got, err := layered.Query(ctx, query)
	if err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("LayeredExecutor.Query() error = %v", err)
	}
	if len(got.Rows) != len(wantRows) {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("layered rows = %d, engine rows = %d", len(got.Rows), len(wantRows))
	}
	for index := range wantRows {
		if got.Rows[index].Timestamp != wantRows[index].Timestamp {
			closeTestEngine(t, ctx, eng)
			t.Fatalf("row[%d] timestamp layered=%d engine=%d", index, got.Rows[index].Timestamp, wantRows[index].Timestamp)
		}
		if got.Rows[index].Fields["usage"].Float64 != wantRows[index].Fields["usage"].Float64 {
			closeTestEngine(t, ctx, eng)
			t.Fatalf("row[%d] value mismatch", index)
		}
	}

	// QuerySpec 入口也必须落到同一权威路径。
	spec, err := querylang.FromModelQuery(query, querylang.Defaults{})
	if err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("FromModelQuery() error = %v", err)
	}
	specRows, err := eng.QuerySpecRows(ctx, spec)
	if err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("QuerySpecRows() error = %v", err)
	}
	if len(specRows) != len(wantRows) {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("QuerySpecRows len = %d, want %d", len(specRows), len(wantRows))
	}
	closeTestEngine(t, ctx, eng)
}
