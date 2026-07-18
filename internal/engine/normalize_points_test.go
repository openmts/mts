package engine

import (
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestNormalizePointsReusesInputWhenComplete(t *testing.T) {
	opts := model.Options{
		DefaultDatabase:        "db",
		DefaultRetentionPolicy: "rp",
	}
	points := []model.Point{{
		Database:        "db",
		RetentionPolicy: "rp",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a"},
		Fields:          map[string]model.FieldValue{"v": model.Float64Value(1)},
	}}
	got := normalizePoints(opts, points)
	if &got[0] != &points[0] {
		t.Fatal("normalizePoints reallocated complete input slice")
	}
}

func TestNormalizePointsCopiesWhenDefaultsNeeded(t *testing.T) {
	opts := model.Options{
		DefaultDatabase:        "db",
		DefaultRetentionPolicy: "rp",
	}
	points := []model.Point{{
		Measurement: "cpu",
		Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
	}}
	got := normalizePoints(opts, points)
	if got[0].Database != "db" || got[0].RetentionPolicy != "rp" {
		t.Fatalf("normalized defaults = %q/%q", got[0].Database, got[0].RetentionPolicy)
	}
	if got[0].Tags == nil {
		t.Fatal("normalized tags is nil")
	}
	if points[0].Database != "" {
		t.Fatal("input slice was mutated")
	}
}
