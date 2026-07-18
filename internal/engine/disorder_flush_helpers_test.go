package engine

import (
	"testing"

	"github.com/openmts/mts/internal/memtable"
	"github.com/openmts/mts/internal/model"
)

func mustNewMemWithOOO(t *testing.T, ordered int, ooo int) *memtable.MemTable {
	t.Helper()
	mt := memtable.New()
	ts := int64(100)
	for i := 0; i < ordered; i++ {
		if err := mt.Apply(model.ResolvedPoint{
			SeriesID:  1,
			Timestamp: ts,
			WriteSeq:  uint64(i + 1),
			Fields: []model.ResolvedField{{
				FieldID: 1,
				Type:    model.FieldFloat64,
				Value:   model.Float64Value(float64(i)),
			}},
		}); err != nil {
			t.Fatalf("Apply(ordered) error = %v", err)
		}
		ts += 10
	}
	for i := 0; i < ooo; i++ {
		if err := mt.Apply(model.ResolvedPoint{
			SeriesID:  1,
			Timestamp: int64(i + 1),
			WriteSeq:  uint64(ordered + i + 1),
			Fields: []model.ResolvedField{{
				FieldID: 1,
				Type:    model.FieldFloat64,
				Value:   model.Float64Value(float64(i)),
			}},
		}); err != nil {
			t.Fatalf("Apply(ooo) error = %v", err)
		}
	}
	return mt
}
