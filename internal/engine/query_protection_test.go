package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryexec"
)

func TestNormalizeQueryAppliesDefaultBudgetProtection(t *testing.T) {
	opts := model.Options{
		DefaultDatabase:        "default",
		DefaultRetentionPolicy: "autogen",
		QueryProtection: model.QueryProtectionOptions{
			DefaultMaxSamples: 100,
			DefaultLimit:      10,
		},
	}
	query := normalizeQuery(opts, model.Query{Measurement: "cpu"})
	if query.Budget.MaxSamples != 100 {
		t.Fatalf("Budget.MaxSamples = %d, want 100", query.Budget.MaxSamples)
	}
	if query.Limit != 10 {
		t.Fatalf("Limit = %d, want 10", query.Limit)
	}
	// 显式设置不被覆盖
	query = normalizeQuery(opts, model.Query{
		Measurement: "cpu",
		Limit:       3,
		Budget:      model.QueryBudget{MaxSamples: 7},
	})
	if query.Budget.MaxSamples != 7 || query.Limit != 3 {
		t.Fatalf("explicit protection overwritten: limit=%d samples=%d", query.Limit, query.Budget.MaxSamples)
	}
}

func TestQueryRowsUsesDefaultBudgetProtection(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
		QueryProtection: model.QueryProtectionOptions{
			DefaultMaxSamples: 2,
		},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := make([]model.Point, 0, 5)
	for i := 0; i < 5; i++ {
		points = append(points, model.Point{
			Measurement: "cpu",
			Timestamp:   int64(i + 1),
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(float64(i))},
		})
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		_ = eng.Close(ctx)
		t.Fatalf("Write() error = %v", err)
	}
	_, err = eng.QueryRows(ctx, model.Query{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     100,
	})
	if !errors.Is(err, queryexec.ErrReadBudgetExceeded) {
		_ = eng.Close(ctx)
		t.Fatalf("QueryRows() error = %v, want ErrReadBudgetExceeded", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
