package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openmts/mts/internal/codec"
	"github.com/openmts/mts/internal/model"
)

func TestDecodeLineRejectsBadCRCAndApplyEntryIgnoresUnknown(t *testing.T) {
	payload, err := encodeWALEntry(walEntry{
		Type:   "series",
		Series: &Series{ID: 1, Measurement: "cpu"},
	})
	if err != nil {
		t.Fatalf("encodeWALEntry() error = %v", err)
	}
	frame := codec.MarshalEnvelope(nil, catalogMagic, 0, payload)
	frame[len(frame)-1] ^= 0xff
	if _, err := decodeLine(frame); err == nil {
		t.Fatal("decodeLine() bad crc error = nil, want error")
	}

	cat := newCatalog(t.TempDir())
	cat.applyEntry(walEntry{Type: "unknown"})
	if len(cat.series) != 0 {
		t.Fatalf("series count = %d, want 0", len(cat.series))
	}
}

func TestSeriesKeyFastPathsAndStableMultiTagOrder(t *testing.T) {
	if got := seriesKey("cpu", nil); got != "cpu" {
		t.Fatalf("seriesKey(no tags) = %q, want cpu", got)
	}
	if got := seriesKey("cpu", map[string]string{"host": "a"}); got != "cpu\xffhost=a" {
		t.Fatalf("seriesKey(one tag) = %q, want cpu\\xffhost=a", got)
	}
	got := seriesKey("cpu", map[string]string{"region": "west", "host": "a"})
	want := "cpu\xffhost=a\xffregion=west"
	if got != want {
		t.Fatalf("seriesKey(multi tag) = %q, want %q", got, want)
	}
}

func TestSeriesKeyMultiTagUsesSingleAllocationClass(t *testing.T) {
	tags := map[string]string{
		"host":   "a",
		"region": "west",
		"rack":   "r1",
	}
	scratch := make([]string, 0, len(tags))
	got, nextScratch := seriesKeyWithScratch("cpu", tags, scratch)
	if got != "cpu\xffhost=a\xffrack=r1\xffregion=west" {
		t.Fatalf("seriesKeyWithScratch() = %q", got)
	}
	if cap(nextScratch) != cap(scratch) {
		t.Fatalf("scratch cap = %d, want %d", cap(nextScratch), cap(scratch))
	}

	allocs := testing.AllocsPerRun(100, func() {
		var key string
		key, scratch = seriesKeyWithScratch("cpu", tags, scratch)
		if key != got {
			t.Fatalf("key = %q, want %q", key, got)
		}
	})
	if allocs > 1 {
		t.Fatalf("multi tag series key allocs/run = %.2f, want <= 1", allocs)
	}
}

func TestResolveSeriesSinglePointMultiTagUsesScratch(t *testing.T) {
	cat := newCatalog(t.TempDir())
	tags := map[string]string{"host": "a", "region": "west", "rack": "r1"}
	cat.applySeries(Series{ID: 1, Measurement: "cpu", Tags: tags})
	allocs := testing.AllocsPerRun(100, func() {
		series, changed, err := cat.resolveSeriesNoSnapshotLocked("cpu", tags)
		if err != nil {
			t.Fatalf("resolveSeriesNoSnapshotLocked() error = %v", err)
		}
		if changed || series.ID != 1 {
			t.Fatalf("series = %#v changed=%v, want existing id 1 unchanged", series, changed)
		}
	})
	if allocs > 1 {
		t.Fatalf("single point multi tag resolve allocs/run = %.2f, want <= 1", allocs)
	}
}

func TestFieldSchemaCachesSortedFieldsAndDetectsConflicts(t *testing.T) {
	cat, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	_, err = cat.ResolvePoint(model.Point{
		Measurement: "cpu",
		Timestamp:   1,
		Fields: map[string]model.FieldValue{
			"z": model.Int64Value(1),
			"a": model.Float64Value(1),
		},
	})
	if err != nil {
		t.Fatalf("ResolvePoint() error = %v", err)
	}
	if got, want := schemaNames(cat.fieldSchemas["cpu"]), []string{"a", "z"}; !equalStrings(got, want) {
		t.Fatalf("field schema names = %v, want %v", got, want)
	}

	resolved, err := cat.ResolvePoints([]model.Point{{
		Measurement: "cpu",
		Timestamp:   2,
		Fields: map[string]model.FieldValue{
			"z": model.Int64Value(2),
			"a": model.Float64Value(2),
		},
	}})
	if err != nil {
		t.Fatalf("ResolvePoints() error = %v", err)
	}
	if got, want := resolvedFieldNames(resolved[0].Fields), []string{"a", "z"}; !equalStrings(got, want) {
		t.Fatalf("resolved field names = %v, want %v", got, want)
	}

	_, err = cat.ResolvePoints([]model.Point{{
		Measurement: "cpu",
		Timestamp:   3,
		Fields: map[string]model.FieldValue{
			"z": model.Int64Value(3),
			"a": model.StringValue("bad"),
		},
	}})
	if err == nil {
		t.Fatal("ResolvePoints(type conflict) error = nil, want conflict")
	}
}

func TestUpsertFieldSchemaInsertsAndUpdatesInNameOrder(t *testing.T) {
	cat := newCatalog(t.TempDir())
	cat.applyField(Field{ID: 1, Measurement: "cpu", Name: "z", Type: model.FieldInt64})
	cat.applyField(Field{ID: 2, Measurement: "cpu", Name: "a", Type: model.FieldFloat64})
	cat.applyField(Field{ID: 3, Measurement: "cpu", Name: "m", Type: model.FieldString})
	cat.applyField(Field{ID: 4, Measurement: "cpu", Name: "m", Type: model.FieldBool})

	schema := cat.fieldSchemas["cpu"]
	if got, want := schemaNames(schema), []string{"a", "m", "z"}; !equalStrings(got, want) {
		t.Fatalf("field schema names = %v, want %v", got, want)
	}
	if schema[1].ID != 4 || schema[1].Type != model.FieldBool {
		t.Fatalf("updated field = %#v, want id 4 bool", schema[1])
	}
}

func TestSingleTagSeriesIndexResolvesExistingSeries(t *testing.T) {
	cat, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	point := model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   1,
		Fields:      map[string]model.FieldValue{"value": model.Float64Value(1)},
	}
	first, err := cat.ResolvePoint(point)
	if err != nil {
		t.Fatalf("ResolvePoint() error = %v", err)
	}
	delete(cat.seriesByKey, seriesKey("cpu", point.Tags))

	second, err := cat.ResolvePoints([]model.Point{point})
	if err != nil {
		t.Fatalf("ResolvePoints() error = %v", err)
	}
	if second[0].SeriesID != first.SeriesID {
		t.Fatalf("SeriesID = %d, want %d", second[0].SeriesID, first.SeriesID)
	}
	if len(cat.series) != 1 {
		t.Fatalf("series count = %d, want 1", len(cat.series))
	}
}

func TestResolvePointsCachesRepeatedMultiTagSeries(t *testing.T) {
	cat, err := Open(t.TempDir())
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
			Timestamp:   1,
			Fields:      map[string]model.FieldValue{"usage": model.Float64Value(1)},
		},
		{
			Measurement: "cpu",
			Tags:        map[string]string{"region": "west", "host": "a"},
			Timestamp:   2,
			Fields:      map[string]model.FieldValue{"usage": model.Float64Value(2)},
		},
	}
	resolved, err := cat.ResolvePoints(points)
	if err != nil {
		t.Fatalf("ResolvePoints() error = %v", err)
	}
	if resolved[0].SeriesID != resolved[1].SeriesID {
		t.Fatalf("series ids = %d,%d want same", resolved[0].SeriesID, resolved[1].SeriesID)
	}
	if len(cat.series) != 1 {
		t.Fatalf("series count = %d, want 1", len(cat.series))
	}
	again, err := cat.ResolvePoints(points)
	if err != nil {
		t.Fatalf("ResolvePoints(again) error = %v", err)
	}
	if again[0].SeriesID != resolved[0].SeriesID || again[1].SeriesID != resolved[0].SeriesID {
		t.Fatalf("again series ids = %d,%d want %d", again[0].SeriesID, again[1].SeriesID, resolved[0].SeriesID)
	}
}

func TestAppendEntryLockedReturnsWriteError(t *testing.T) {
	cat, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	cat.mu.Lock()
	if err := cat.wal.Close(); err != nil {
		cat.mu.Unlock()
		t.Fatalf("Close(wal) error = %v", err)
	}
	err = cat.appendEntryLocked(walEntry{
		Type:   "series",
		Series: &Series{ID: 1, Measurement: "cpu"},
	})
	cat.wal = nil
	cat.mu.Unlock()
	if err == nil {
		t.Fatal("appendEntryLocked() error = nil, want write error")
	}
	if err := cat.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestCatalogDefersSnapshotAndReplaysWALBeforeClose(t *testing.T) {
	dir := t.TempDir()
	cat, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	points := []model.Point{
		{Measurement: "cpu", Tags: map[string]string{"host": "a"}, Timestamp: 1, Fields: map[string]model.FieldValue{"v": model.Float64Value(1)}},
		{Measurement: "cpu", Tags: map[string]string{"host": "b"}, Timestamp: 2, Fields: map[string]model.FieldValue{"v": model.Float64Value(2)}},
	}
	resolved, err := cat.ResolvePoints(points)
	if err != nil {
		closeErr := cat.Close()
		t.Fatalf("ResolvePoints() error = %v close = %v", err, closeErr)
	}
	if _, err := os.Stat(cat.snapshotPath()); !os.IsNotExist(err) {
		closeErr := cat.Close()
		t.Fatalf("snapshot stat before checkpoint = %v, want not exist close = %v", err, closeErr)
	}
	if err := cat.wal.Close(); err != nil {
		t.Fatalf("Close(wal crash simulation) error = %v", err)
	}
	cat.wal = nil

	replayed, err := Open(dir)
	if err != nil {
		t.Fatalf("Open(replay) error = %v", err)
	}
	replayedResolved, err := replayed.ResolvePoints(points)
	if err != nil {
		closeErr := replayed.Close()
		t.Fatalf("ResolvePoints(replayed) error = %v close = %v", err, closeErr)
	}
	if replayedResolved[0].SeriesID != resolved[0].SeriesID || replayedResolved[1].SeriesID != resolved[1].SeriesID {
		closeErr := replayed.Close()
		t.Fatalf("replayed series ids = %d,%d want %d,%d close = %v",
			replayedResolved[0].SeriesID,
			replayedResolved[1].SeriesID,
			resolved[0].SeriesID,
			resolved[1].SeriesID,
			closeErr,
		)
	}
	if err := replayed.Close(); err != nil {
		t.Fatalf("Close(replayed) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "snapshot.bin")); err != nil {
		t.Fatalf("snapshot stat after Close() error = %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "catalog.wal"))
	if err != nil {
		t.Fatalf("wal stat after checkpoint error = %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("wal size after checkpoint = %d, want 0", info.Size())
	}
}

func TestCatalogCheckpointAndFieldResolveBranches(t *testing.T) {
	cat, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	cat.mu.Lock()
	if err := cat.checkpointSnapshotLocked(false); err != nil {
		cat.mu.Unlock()
		t.Fatalf("checkpoint clean error = %v", err)
	}
	cat.snapshotDirtyRecords = 1
	if err := cat.checkpointSnapshotLocked(false); err != nil {
		cat.mu.Unlock()
		t.Fatalf("checkpoint below threshold error = %v", err)
	}
	if cat.snapshotDirtyRecords != 1 {
		cat.mu.Unlock()
		t.Fatalf("dirty records after below-threshold checkpoint = %d, want 1", cat.snapshotDirtyRecords)
	}
	cat.mu.Unlock()

	point := model.Point{
		Measurement: "cpu",
		Timestamp:   1,
		Fields:      map[string]model.FieldValue{"usage": model.Float64Value(1)},
	}
	if _, err := cat.ResolvePoint(point); err != nil {
		t.Fatalf("ResolvePoint() error = %v", err)
	}
	cat.mu.Lock()
	field, changed, err := cat.resolveFieldNoSnapshotLocked("cpu", "usage", model.FieldFloat64)
	if err != nil {
		cat.mu.Unlock()
		t.Fatalf("resolve existing field error = %v", err)
	}
	if changed || field.Name != "usage" {
		cat.mu.Unlock()
		t.Fatalf("resolve existing field changed=%t field=%#v, want unchanged usage", changed, field)
	}
	_, _, err = cat.resolveFieldNoSnapshotLocked("cpu", "usage", model.FieldString)
	cat.mu.Unlock()
	if err == nil {
		t.Fatal("resolve conflicting field error = nil, want error")
	}
	if err := cat.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	walNil := newCatalog(t.TempDir())
	walNil.applySeries(Series{ID: 1, Measurement: "cpu"})
	walNil.snapshotDirtyRecords = 1
	if err := walNil.checkpointSnapshotLocked(true); err != nil {
		t.Fatalf("checkpoint with nil wal error = %v", err)
	}
	if walNil.snapshotDirtyRecords != 0 {
		t.Fatalf("dirty records after nil-wal checkpoint = %d, want 0", walNil.snapshotDirtyRecords)
	}
}

func TestEnsureMetadataBranches(t *testing.T) {
	cat := newCatalog(t.TempDir())
	if cat.ensureMetadataLocked("", "hot") {
		t.Fatal("ensureMetadataLocked(empty database) changed metadata")
	}
	if !cat.ensureMetadataLocked("metrics", "") {
		t.Fatal("ensureMetadataLocked(new database) changed = false, want true")
	}
	if cat.ensureMetadataLocked("metrics", "") {
		t.Fatal("ensureMetadataLocked(existing database) changed = true, want false")
	}
	if !cat.ensureMetadataLocked("metrics", "hot") {
		t.Fatal("ensureMetadataLocked(new policy) changed = false, want true")
	}
	if cat.ensureMetadataLocked("metrics", "hot") {
		t.Fatal("ensureMetadataLocked(existing policy) changed = true, want false")
	}
}

func schemaNames(fields []Field) []string {
	names := make([]string, len(fields))
	for index, field := range fields {
		names[index] = field.Name
	}
	return names
}

func resolvedFieldNames(fields []model.ResolvedField) []string {
	names := make([]string, len(fields))
	for index, field := range fields {
		names[index] = field.FieldName
	}
	return names
}

func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func TestCatalogBinaryDecodeValidationErrors(t *testing.T) {
	if _, err := encodeWALEntry(walEntry{Type: "series"}); err == nil {
		t.Fatal("encodeWALEntry(missing series) error = nil, want error")
	}
	if _, err := encodeWALEntry(walEntry{Type: "unknown"}); err == nil {
		t.Fatal("encodeWALEntry(unknown) error = nil, want error")
	}
	if _, err := decodeWALFrame(codec.MarshalEnvelope(nil, catalogMagic, 0, []byte{99})); err == nil {
		t.Fatal("decodeWALFrame(unknown record) error = nil, want error")
	}
	if _, err := decodeSnapshot(codec.MarshalEnvelope(nil, catalogMagic, 0, []byte{1})); err == nil {
		t.Fatal("decodeSnapshot(truncated) error = nil, want error")
	}
	if !validFieldType(model.FieldBool) || validFieldType(model.FieldType(99)) {
		t.Fatal("validFieldType() returned unexpected result")
	}
	if _, err := uint32Value("field", uint64(^uint32(0))+1); err == nil {
		t.Fatal("uint32Value(overflow) error = nil, want error")
	}
	reader := newPayloadReader([]byte{1})
	if err := reader.done("catalog test"); err == nil {
		t.Fatal("payloadReader.done(trailing) error = nil, want error")
	}
}

func TestCatalogBinaryDecodersRejectTruncatedPrefixes(t *testing.T) {
	snapshotPayload := encodeSnapshot(snapshot{
		NextSeriesID: 2,
		NextFieldID:  2,
		Series:       []Series{{ID: 1, Measurement: "cpu", Tags: map[string]string{"host": "a"}}},
		Fields:       []Field{{ID: 1, Measurement: "cpu", Name: "value", Type: model.FieldFloat64}},
	})
	for size := 0; size < len(snapshotPayload); size++ {
		if _, err := decodeSnapshot(snapshotPayload[:size]); err == nil {
			t.Fatalf("decodeSnapshot(prefix %d) error = nil, want error", size)
		}
	}

	entryPayload, err := encodeWALEntry(walEntry{
		Type:   "series",
		Series: &Series{ID: 1, Measurement: "cpu", Tags: map[string]string{"host": "a"}},
	})
	if err != nil {
		t.Fatalf("encodeWALEntry() error = %v", err)
	}
	frame := codec.MarshalEnvelope(nil, catalogMagic, 0, entryPayload)
	for size := 0; size < len(frame); size++ {
		if _, err := decodeWALFrame(frame[:size]); err == nil {
			t.Fatalf("decodeWALFrame(prefix %d) error = nil, want error", size)
		}
	}
}

func TestCatalogPayloadDecodersRejectTruncatedInnerPayload(t *testing.T) {
	snapshotFrame := encodeSnapshot(snapshot{
		NextSeriesID: 2,
		NextFieldID:  2,
		Series:       []Series{{ID: 1, Measurement: "cpu", Tags: map[string]string{"host": "a"}}},
		Fields:       []Field{{ID: 1, Measurement: "cpu", Name: "value", Type: model.FieldFloat64}},
	})
	env, err := codec.UnmarshalEnvelope(snapshotFrame, catalogMagic)
	if err != nil {
		t.Fatalf("UnmarshalEnvelope(snapshot) error = %v", err)
	}
	for size := 0; size < len(env.Payload); size++ {
		frame := codec.MarshalEnvelope(nil, catalogMagic, 0, env.Payload[:size])
		if _, err := decodeSnapshot(frame); err == nil {
			t.Fatalf("decodeSnapshot(inner prefix %d) error = nil, want error", size)
		}
	}

	entryPayload, err := encodeWALEntry(walEntry{
		Type:  "field",
		Field: &Field{ID: 1, Measurement: "cpu", Name: "value", Type: model.FieldFloat64},
	})
	if err != nil {
		t.Fatalf("encodeWALEntry(field) error = %v", err)
	}
	for size := 0; size < len(entryPayload); size++ {
		frame := codec.MarshalEnvelope(nil, catalogMagic, 0, entryPayload[:size])
		if _, err := decodeWALFrame(frame); err == nil {
			t.Fatalf("decodeWALFrame(inner prefix %d) error = nil, want error", size)
		}
	}
}
