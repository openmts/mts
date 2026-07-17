package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openmts/mts/internal/catalog"
	"github.com/openmts/mts/internal/model"
)

func TestEngineWriteRejectsSeriesCardinalityLimit(t *testing.T) {
	ctx := context.Background()
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
		Cardinality: model.CardinalityOptions{
			MaxSeries: 1,
		},
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Write(ctx, []model.Point{{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   1,
		Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
	}}, model.WriteOptions{}); err != nil {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("Write(first) error = %v", err)
	}
	err = eng.Write(ctx, []model.Point{{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "b"},
		Timestamp:   2,
		Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
	}}, model.WriteOptions{})
	if !errors.Is(err, catalog.ErrCardinalityLimit) {
		closeTestEngine(t, ctx, eng)
		t.Fatalf("Write(second) error = %v, want ErrCardinalityLimit", err)
	}
	closeTestEngine(t, ctx, eng)
}
