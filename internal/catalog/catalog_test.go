package catalog_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/openmts/mts/internal/catalog"
	"github.com/openmts/mts/internal/model"
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

func TestCatalogMetadataManagementPersists(t *testing.T) {
	dir := t.TempDir()
	cat, err := catalog.Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := cat.CreateDatabase("metrics"); err != nil {
		t.Fatalf("CreateDatabase() error = %v", err)
	}
	if err := cat.CreateRetentionPolicy("metrics", model.RetentionPolicy{Name: "hot"}); err != nil {
		t.Fatalf("CreateRetentionPolicy() error = %v", err)
	}
	point := model.Point{
		Database:        "metrics",
		RetentionPolicy: "hot",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a"},
		Fields:          map[string]model.FieldValue{"usage": model.Float64Value(1)},
	}
	if _, err := cat.ResolvePoint(point); err != nil {
		t.Fatalf("ResolvePoint() error = %v", err)
	}
	if err := cat.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	reopened, err := catalog.Open(dir)
	if err != nil {
		t.Fatalf("Open(reopened) error = %v", err)
	}
	policies, err := reopened.ListRetentionPolicies("metrics")
	if err != nil {
		t.Fatalf("ListRetentionPolicies() error = %v", err)
	}
	if len(policies) != 1 || policies[0].Name != "hot" {
		t.Fatalf("policies = %#v, want hot", policies)
	}
	measurements, err := reopened.ListMeasurements("metrics")
	if err != nil {
		t.Fatalf("ListMeasurements() error = %v", err)
	}
	if len(measurements) != 1 || measurements[0] != "cpu" {
		t.Fatalf("measurements = %#v, want cpu", measurements)
	}
	fields, err := reopened.ListFields("metrics", "cpu")
	if err != nil {
		t.Fatalf("ListFields() error = %v", err)
	}
	if len(fields) != 1 || fields[0].Name != "usage" {
		t.Fatalf("fields = %#v, want usage", fields)
	}
	series, err := reopened.ListSeries("metrics", "cpu", map[string]string{"host": "a"})
	if err != nil {
		t.Fatalf("ListSeries() error = %v", err)
	}
	if len(series) != 1 || series[0].Tags["host"] != "a" {
		t.Fatalf("series = %#v, want host=a", series)
	}
	if err := reopened.DropDatabase("metrics"); err != nil {
		t.Fatalf("DropDatabase() error = %v", err)
	}
	policies, err = reopened.ListRetentionPolicies("metrics")
	if err != nil {
		t.Fatalf("ListRetentionPolicies(after drop) error = %v", err)
	}
	if len(policies) != 0 {
		t.Fatalf("policies after drop = %#v, want none", policies)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("Close(reopened) error = %v", err)
	}
}

func TestCatalogMetadataRejectsInvalidInputsAndCorruption(t *testing.T) {
	dir := t.TempDir()
	cat, err := catalog.Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := cat.CreateDatabase(""); err == nil {
		t.Fatal("CreateDatabase(empty) error = nil, want error")
	}
	if err := cat.CreateRetentionPolicy("", model.RetentionPolicy{Name: "hot"}); err == nil {
		t.Fatal("CreateRetentionPolicy(empty db) error = nil, want error")
	}
	if err := cat.CreateRetentionPolicy("metrics", model.RetentionPolicy{}); err == nil {
		t.Fatal("CreateRetentionPolicy(empty policy) error = nil, want error")
	}
	if policies, err := cat.ListRetentionPolicies("missing"); err != nil || len(policies) != 0 {
		t.Fatalf("ListRetentionPolicies(missing) = %v, %v; want empty nil", policies, err)
	}
	if fields, err := cat.ListFields("missing", "cpu"); err != nil || len(fields) != 0 {
		t.Fatalf("ListFields(missing) = %v, %v; want empty nil", fields, err)
	}
	if series, err := cat.ListSeries("missing", "cpu", nil); err != nil || len(series) != 0 {
		t.Fatalf("ListSeries(missing) = %v, %v; want empty nil", series, err)
	}
	if err := cat.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.bin"), []byte{1, 2, 3}, 0600); err != nil {
		t.Fatalf("WriteFile(metadata corrupt) error = %v", err)
	}
	if _, err := catalog.Open(dir); err == nil {
		t.Fatal("Open(corrupt metadata) error = nil, want error")
	}
}

func TestCatalogSnapshotClonesSeriesTags(t *testing.T) {
	cat, err := catalog.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	point := catalogPoint("cpu", "a", 1)
	resolved, err := cat.ResolvePoint(point)
	if err != nil {
		t.Fatalf("ResolvePoint() error = %v", err)
	}
	snap := cat.Snapshot()
	snap.Series[resolved.SeriesID].Tags["host"] = "changed"
	series, ok := cat.Series(resolved.SeriesID)
	if !ok {
		t.Fatalf("Series(%d) missing", resolved.SeriesID)
	}
	if series.Tags["host"] != "a" {
		t.Fatalf("catalog series tag = %q, want a", series.Tags["host"])
	}
	if err := cat.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
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

func TestCatalogResolvePointsMatchesResolvePoint(t *testing.T) {
	dir := t.TempDir()
	cat, err := catalog.Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	points := []model.Point{
		catalogPoint("cpu", "host-a", 1),
		catalogPoint("cpu", "host-a", 2),
		catalogPoint("cpu", "host-b", 3),
	}
	got, err := cat.ResolvePoints(points)
	if err != nil {
		t.Fatalf("ResolvePoints() error = %v", err)
	}
	if len(got) != len(points) {
		t.Fatalf("ResolvePoints() len = %d, want %d", len(got), len(points))
	}
	if got[0].SeriesID != got[1].SeriesID {
		t.Fatalf("same tags got different series ids: %d vs %d", got[0].SeriesID, got[1].SeriesID)
	}
	if got[0].SeriesID == got[2].SeriesID {
		t.Fatalf("different tags got same series id: %d", got[0].SeriesID)
	}
}

func TestCatalogResolvePointClonesTagsAndResolvePointsBorrowsTags(t *testing.T) {
	cat, err := catalog.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	single := catalogPoint("cpu", "single", 1)
	resolvedSingle, err := cat.ResolvePoint(single)
	if err != nil {
		t.Fatalf("ResolvePoint() error = %v", err)
	}
	single.Tags["host"] = "changed"
	if resolvedSingle.Tags["host"] != "single" {
		t.Fatalf("ResolvePoint() tags alias input: %q", resolvedSingle.Tags["host"])
	}

	batch := catalogPoint("cpu", "batch", 2)
	resolvedBatch, err := cat.ResolvePoints([]model.Point{batch})
	if err != nil {
		t.Fatalf("ResolvePoints() error = %v", err)
	}
	batch.Tags["host"] = "changed"
	if resolvedBatch[0].Tags["host"] != "changed" {
		t.Fatalf("ResolvePoints() cloned tags = %q, want borrowed map", resolvedBatch[0].Tags["host"])
	}

	series, ok := cat.Series(resolvedBatch[0].SeriesID)
	if !ok {
		t.Fatalf("Series(%d) not found", resolvedBatch[0].SeriesID)
	}
	if series.Tags["host"] != "batch" {
		t.Fatalf("series tags changed through borrowed result: %q", series.Tags["host"])
	}
}

func TestCatalogResolveTypedBatchColumnsBorrowsValues(t *testing.T) {
	cat, err := catalog.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	batch := model.TypedBatch{
		Database:        "db",
		RetentionPolicy: "rp",
		Measurement:     "cpu",
		Tags: []model.TagColumn{
			{Name: "host", Values: []string{"a", "b", "a"}},
		},
		Timestamps: []int64{10, 20, 30},
		Fields: []model.TypedFieldColumn{
			{Name: "usage", Type: model.FieldFloat64, Float64Values: []float64{1.1, 2.2, 3.3}},
			{Name: "state", Type: model.FieldString, StringValues: []string{"ok", "warn", "ok"}},
		},
	}

	resolved, err := cat.ResolveTypedBatchColumns(batch)
	if err != nil {
		t.Fatalf("ResolveTypedBatchColumns() error = %v", err)
	}
	if len(resolved.SeriesIDs) != len(batch.Timestamps) {
		t.Fatalf("series ids len = %d, want %d", len(resolved.SeriesIDs), len(batch.Timestamps))
	}
	if resolved.SeriesIDs[0] == 0 || resolved.SeriesIDs[0] != resolved.SeriesIDs[2] {
		t.Fatalf("series ids = %#v, want repeated host to reuse series", resolved.SeriesIDs)
	}
	if len(resolved.Fields) != 2 {
		t.Fatalf("fields len = %d, want 2", len(resolved.Fields))
	}
	if resolved.Fields[0].FieldID == 0 || resolved.Fields[0].Type != model.FieldFloat64 {
		t.Fatalf("first field = %#v, want resolved float field", resolved.Fields[0])
	}
	if &resolved.Fields[0].Float64Values[0] != &batch.Fields[0].Float64Values[0] {
		t.Fatal("float values were copied, want typed resolver to borrow batch column")
	}
	if &resolved.Fields[1].StringValues[0] != &batch.Fields[1].StringValues[0] {
		t.Fatal("string values were copied, want typed resolver to borrow batch column")
	}
}

func TestCatalogResolvePointsRejectsInvalidPoint(t *testing.T) {
	cat, err := catalog.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	_, err = cat.ResolvePoints([]model.Point{
		catalogPoint("cpu", "host-a", 1),
		{Measurement: "", Fields: map[string]model.FieldValue{"v": model.Float64Value(1)}},
	})
	if !errors.Is(err, catalog.ErrEmptyMeasurement) {
		t.Fatalf("ResolvePoints() error = %v, want ErrEmptyMeasurement", err)
	}
	if len(cat.MatchSeries("cpu", nil)) != 0 {
		t.Fatal("ResolvePoints() created series before rejecting invalid batch")
	}
}

func TestCatalogOpenInvalidPathReturnsError(t *testing.T) {
	if _, err := catalog.Open("bad\x00path"); err == nil {
		t.Fatal("Open(invalid) error = nil, want error")
	}
}

func TestCatalogOpenRejectsCorruptSnapshotAndWAL(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "snapshot.bin"), []byte("{"), 0600); err != nil {
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

func catalogPoint(measurement string, host string, timestamp int64) model.Point {
	return model.Point{
		Measurement: measurement,
		Tags:        map[string]string{"host": host},
		Timestamp:   timestamp,
		Fields: map[string]model.FieldValue{
			"active": model.BoolValue(timestamp%2 == 0),
			"usage":  model.Float64Value(float64(timestamp)),
		},
	}
}

func TestResolvePointsReusesFieldTemplateForHomogeneousBatch(t *testing.T) {
	cat, err := catalog.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = cat.Close() }()

	points := make([]model.Point, 0, 8)
	for index := 0; index < 8; index++ {
		points = append(points, model.Point{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   int64(index),
			Fields: map[string]model.FieldValue{
				"f0": model.Float64Value(float64(index)),
				"f1": model.Float64Value(float64(index) + 0.1),
				"f2": model.Float64Value(float64(index) + 0.2),
			},
		})
	}
	resolved, err := cat.ResolvePoints(points)
	if err != nil {
		t.Fatalf("ResolvePoints() error = %v", err)
	}
	if len(resolved) != len(points) {
		t.Fatalf("resolved count = %d, want %d", len(resolved), len(points))
	}
	firstIDs := []uint32{resolved[0].Fields[0].FieldID, resolved[0].Fields[1].FieldID, resolved[0].Fields[2].FieldID}
	for _, point := range resolved[1:] {
		if len(point.Fields) != 3 {
			t.Fatalf("field count = %d, want 3", len(point.Fields))
		}
		for index, field := range point.Fields {
			if field.FieldID != firstIDs[index] {
				t.Fatalf("field %d id = %d, want %d", index, field.FieldID, firstIDs[index])
			}
		}
	}
}
