package memtable

import (
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestMemTableTracksOutOfOrderAndDuplicateSamples(t *testing.T) {
	mt := New()
	base := model.ResolvedPoint{
		SeriesID:  1,
		Timestamp: 10,
		WriteSeq:  1,
		Fields: []model.ResolvedField{{
			FieldID: 1,
			Type:    model.FieldFloat64,
			Value:   model.Float64Value(1),
		}},
	}
	if err := mt.Apply(base); err != nil {
		t.Fatalf("Apply(base) error = %v", err)
	}
	late := base
	late.Timestamp = 20
	late.WriteSeq = 2
	if err := mt.Apply(late); err != nil {
		t.Fatalf("Apply(late) error = %v", err)
	}
	dup := base
	dup.Timestamp = 20
	dup.WriteSeq = 3
	if err := mt.Apply(dup); err != nil {
		t.Fatalf("Apply(dup) error = %v", err)
	}
	ooo := base
	ooo.Timestamp = 5
	ooo.WriteSeq = 4
	if err := mt.Apply(ooo); err != nil {
		t.Fatalf("Apply(ooo) error = %v", err)
	}
	stats := mt.StatsSnapshot()
	if stats.AppendedSamples != 4 {
		t.Fatalf("AppendedSamples = %d, want 4", stats.AppendedSamples)
	}
	if stats.DuplicateSamples != 1 {
		t.Fatalf("DuplicateSamples = %d, want 1", stats.DuplicateSamples)
	}
	if stats.OutOfOrderSamples != 1 {
		t.Fatalf("OutOfOrderSamples = %d, want 1", stats.OutOfOrderSamples)
	}
	if stats.Samples != 4 {
		t.Fatalf("Samples = %d, want 4", stats.Samples)
	}
}
