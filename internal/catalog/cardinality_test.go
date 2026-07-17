package catalog

import (
	"errors"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestCatalogRejectsSeriesOverLimit(t *testing.T) {
	cat, err := OpenWithOptions(t.TempDir(), Options{Limits: Limits{MaxSeries: 1}})
	if err != nil {
		t.Fatalf("OpenWithOptions() error = %v", err)
	}
	defer func() { _ = cat.Close() }()

	if _, err := cat.ResolvePoint(model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
	}); err != nil {
		t.Fatalf("ResolvePoint(first) error = %v", err)
	}
	_, err = cat.ResolvePoint(model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "b"},
		Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
	})
	if !errors.Is(err, ErrCardinalityLimit) {
		t.Fatalf("ResolvePoint(second) error = %v, want ErrCardinalityLimit", err)
	}
	if cat.CardinalityRejected() != 1 {
		t.Fatalf("CardinalityRejected() = %d, want 1", cat.CardinalityRejected())
	}
}

func TestCatalogRejectsFieldsOverLimit(t *testing.T) {
	cat, err := OpenWithOptions(t.TempDir(), Options{Limits: Limits{MaxFields: 1}})
	if err != nil {
		t.Fatalf("OpenWithOptions() error = %v", err)
	}
	defer func() { _ = cat.Close() }()

	if _, err := cat.ResolvePoint(model.Point{
		Measurement: "cpu",
		Fields:      map[string]model.FieldValue{"a": model.Float64Value(1)},
	}); err != nil {
		t.Fatalf("ResolvePoint(first field) error = %v", err)
	}
	_, err = cat.ResolvePoint(model.Point{
		Measurement: "cpu",
		Fields:      map[string]model.FieldValue{"b": model.Float64Value(1)},
	})
	if !errors.Is(err, ErrCardinalityLimit) {
		t.Fatalf("ResolvePoint(second field) error = %v, want ErrCardinalityLimit", err)
	}
}

func TestCatalogRejectsTagValuesOverLimit(t *testing.T) {
	cat, err := OpenWithOptions(t.TempDir(), Options{Limits: Limits{MaxTagValuesPerKey: 1}})
	if err != nil {
		t.Fatalf("OpenWithOptions() error = %v", err)
	}
	defer func() { _ = cat.Close() }()

	if _, err := cat.ResolvePoint(model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
	}); err != nil {
		t.Fatalf("ResolvePoint(first tag) error = %v", err)
	}
	_, err = cat.ResolvePoint(model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "b"},
		Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
	})
	if !errors.Is(err, ErrCardinalityLimit) {
		t.Fatalf("ResolvePoint(second tag) error = %v, want ErrCardinalityLimit", err)
	}
}

func TestCatalogAllowsExistingSeriesUnderLimit(t *testing.T) {
	cat, err := OpenWithOptions(t.TempDir(), Options{Limits: Limits{MaxSeries: 1}})
	if err != nil {
		t.Fatalf("OpenWithOptions() error = %v", err)
	}
	defer func() { _ = cat.Close() }()

	point := model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
	}
	if _, err := cat.ResolvePoint(point); err != nil {
		t.Fatalf("ResolvePoint(first) error = %v", err)
	}
	if _, err := cat.ResolvePoint(point); err != nil {
		t.Fatalf("ResolvePoint(existing) error = %v", err)
	}
}
