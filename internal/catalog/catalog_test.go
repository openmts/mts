package catalog_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/mts/mts/internal/catalog"
	"codeberg.org/mts/mts/internal/model"
)

func TestCatalogResolveReopenAndTypeConflict(t *testing.T) {
	dir := t.TempDir()
	cat, err := catalog.Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	point := model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields: map[string]model.FieldValue{
			"usage": model.Float64Value(1),
		},
	}
	resolved, err := cat.ResolvePoint(point)
	if err != nil {
		t.Fatalf("ResolvePoint() error = %v", err)
	}
	if resolved.SeriesID == 0 {
		t.Fatal("SeriesID = 0, want non-zero")
	}
	if len(resolved.Fields) != 1 {
		t.Fatalf("field count = %d, want 1", len(resolved.Fields))
	}
	if err := cat.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	cat, err = catalog.Open(dir)
	if err != nil {
		t.Fatalf("Open() after close error = %v", err)
	}
	reopened, err := cat.ResolvePoint(point)
	if err != nil {
		t.Fatalf("ResolvePoint() after reopen error = %v", err)
	}
	if reopened.SeriesID != resolved.SeriesID {
		t.Fatalf("SeriesID after reopen = %d, want %d", reopened.SeriesID, resolved.SeriesID)
	}
	if reopened.Fields[0].FieldID != resolved.Fields[0].FieldID {
		t.Fatalf("FieldID after reopen = %d, want %d", reopened.Fields[0].FieldID, resolved.Fields[0].FieldID)
	}

	point.Fields["usage"] = model.StringValue("bad")
	if _, err := cat.ResolvePoint(point); err == nil {
		t.Fatal("ResolvePoint() type conflict error = nil, want error")
	}
	if err := cat.Close(); err != nil {
		t.Fatalf("Close() after reopen error = %v", err)
	}
}

func TestCatalogPersistenceIsBinary(t *testing.T) {
	dir := t.TempDir()
	cat, err := catalog.Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	point := model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields:      map[string]model.FieldValue{"usage": model.Float64Value(1)},
	}
	if _, err := cat.ResolvePoint(point); err != nil {
		t.Fatalf("ResolvePoint() error = %v", err)
	}
	if err := cat.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	for _, name := range []string{"catalog.wal", "snapshot.bin"} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", name, err)
		}
		if bytes.Contains(data, []byte(`{"`)) {
			t.Fatalf("%s contains JSON marker", name)
		}
	}
	reopened, err := catalog.Open(dir)
	if err != nil {
		t.Fatalf("Open() after binary persistence error = %v", err)
	}
	if _, err := reopened.ResolvePoint(point); err != nil {
		t.Fatalf("ResolvePoint() after reopen error = %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("Close() reopened error = %v", err)
	}
}

func TestCatalogMatchSeriesByExactTags(t *testing.T) {
	cat, err := catalog.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	points := []model.Point{
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a", "region": "west"},
			Fields:      map[string]model.FieldValue{"usage": model.Float64Value(1)},
		},
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "b", "region": "east"},
			Fields:      map[string]model.FieldValue{"usage": model.Float64Value(2)},
		},
	}
	for _, point := range points {
		if _, err := cat.ResolvePoint(point); err != nil {
			t.Fatalf("ResolvePoint() error = %v", err)
		}
	}

	matches := cat.MatchSeries("cpu", map[string]string{"host": "a"})
	if len(matches) != 1 {
		t.Fatalf("match count = %d, want 1", len(matches))
	}
	series, ok := cat.Series(matches[0])
	if !ok {
		t.Fatalf("Series(%d) not found", matches[0])
	}
	if series.Tags["host"] != "a" {
		t.Fatalf("host tag = %q, want %q", series.Tags["host"], "a")
	}
	if len(cat.MatchSeries("cpu", map[string]string{"host": "missing"})) != 0 {
		t.Fatal("MatchSeries() returned rows for missing tag")
	}
	if len(cat.MatchSeries("mem", nil)) != 0 {
		t.Fatal("MatchSeries() returned rows for missing measurement")
	}
	ids := cat.FieldIDs("cpu", []string{"usage"})
	if len(ids) != 1 {
		t.Fatalf("FieldIDs() count = %d, want 1", len(ids))
	}
	for id := range ids {
		field, ok := cat.Field(id)
		if !ok {
			t.Fatalf("Field(%d) not found", id)
		}
		if field.Name != "usage" {
			t.Fatalf("field name = %q, want usage", field.Name)
		}
	}
	allIDs := cat.FieldIDs("cpu", nil)
	if len(allIDs) != 1 {
		t.Fatalf("all FieldIDs() count = %d, want 1", len(allIDs))
	}
	if len(cat.FieldIDs("cpu", []string{"missing"})) != 0 {
		t.Fatal("FieldIDs() returned id for missing field")
	}
	if len(cat.FieldIDs("mem", nil)) != 0 {
		t.Fatal("FieldIDs() returned id for missing measurement")
	}
	if _, ok := cat.Series(999); ok {
		t.Fatal("Series(999) found, want missing")
	}
}

func TestCatalogRejectsInvalidPoint(t *testing.T) {
	cat, err := catalog.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	if _, err := cat.ResolvePoint(model.Point{Fields: map[string]model.FieldValue{"v": model.BoolValue(true)}}); err == nil {
		t.Fatal("ResolvePoint() empty measurement error = nil, want error")
	}
	if _, err := cat.ResolvePoint(model.Point{Measurement: "cpu"}); err == nil {
		t.Fatal("ResolvePoint() empty fields error = nil, want error")
	}
}

func TestCatalogOpenInvalidPathReturnsError(t *testing.T) {
	if _, err := catalog.Open("bad\x00path"); err == nil {
		t.Fatal("Open(invalid) error = nil, want error")
	}
}

func TestCatalogOpenRejectsCorruptSnapshotAndWAL(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "snapshot.json"), []byte("{"), 0600); err != nil {
		t.Fatalf("WriteFile(snapshot) error = %v", err)
	}
	if _, err := catalog.Open(dir); err == nil {
		t.Fatal("Open(corrupt snapshot) error = nil, want error")
	}

	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "catalog.wal"), []byte("{bad\n"), 0600); err != nil {
		t.Fatalf("WriteFile(wal) error = %v", err)
	}
	if _, err := catalog.Open(dir); err == nil {
		t.Fatal("Open(corrupt wal) error = nil, want error")
	}
}

func TestCatalogCloseTwiceAndMatchAllTags(t *testing.T) {
	cat, err := catalog.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	point := model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields:      map[string]model.FieldValue{"usage": model.Float64Value(1)},
	}
	if _, err := cat.ResolvePoint(point); err != nil {
		t.Fatalf("ResolvePoint() error = %v", err)
	}
	if len(cat.MatchSeries("cpu", nil)) != 1 {
		t.Fatal("MatchSeries() with nil tags did not match existing series")
	}
	if err := cat.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := cat.Close(); err != nil {
		t.Fatalf("Close() second error = %v", err)
	}
}
