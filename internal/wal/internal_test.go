package wal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/openmts/mts/internal/codec"
	"github.com/openmts/mts/internal/model"
)

func TestAppendRejectsUnencodablePayloadAndEmptyReplay(t *testing.T) {
	log, err := Open(t.TempDir(), Options{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	bad := model.ResolvedPoint{
		SeriesID:  1,
		Timestamp: 1,
		WriteSeq:  1,
		Fields: []model.ResolvedField{
			{FieldID: 2, Type: model.FieldType(99), Value: model.FieldValue{Type: model.FieldType(99)}},
		},
	}
	if err := log.Append([]model.ResolvedPoint{bad}, false); err == nil {
		t.Fatal("Append(unsupported field type) error = nil, want encode error")
	}
	if err := log.TruncateAll(); err != nil {
		t.Fatalf("TruncateAll() error = %v", err)
	}
	points, err := log.Replay()
	if err != nil {
		t.Fatalf("Replay() empty error = %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("empty replay count = %d, want 0", len(points))
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestBatchIntervalSyncAndCheckpoint(t *testing.T) {
	dir := t.TempDir()
	log, err := Open(dir, Options{SegmentBytes: 96, BatchInterval: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	record := model.ResolvedPoint{SeriesID: 1, Timestamp: 1, WriteSeq: 1}
	if err := log.Append([]model.ResolvedPoint{record}, false); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	waitForWALTest(t, time.Second, func() bool {
		log.mu.Lock()
		defer log.mu.Unlock()
		return log.pendingRecords == 0 && log.pendingBytes == 0
	})
	if err := log.Append([]model.ResolvedPoint{record}, false); err != nil {
		t.Fatalf("Append(second) error = %v", err)
	}
	if err := log.Checkpoint(); err != nil {
		t.Fatalf("Checkpoint() error = %v", err)
	}
	got, err := log.Replay()
	if err != nil {
		t.Fatalf("Replay() after checkpoint error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("checkpoint replay count = %d, want 0", len(got))
	}
	segments, err := listSegments(dir)
	if err != nil {
		t.Fatalf("listSegments() error = %v", err)
	}
	if len(segments) != 1 || segments[0].number != 2 {
		t.Fatalf("segments = %#v, want only active segment 2", segments)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestWALMetricsSnapshotRecordsAppendSyncCheckpointReplay(t *testing.T) {
	dir := t.TempDir()
	log, err := Open(dir, Options{Sync: true})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	record := model.ResolvedPoint{SeriesID: 1, Timestamp: 1, WriteSeq: 1}
	if err := log.Append([]model.ResolvedPoint{record}, true); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if _, err := log.ReplayRecords(); err != nil {
		t.Fatalf("ReplayRecords() error = %v", err)
	}
	if err := log.Checkpoint(); err != nil {
		t.Fatalf("Checkpoint() error = %v", err)
	}
	snapshot := log.MetricsSnapshot()
	if snapshot.AppendRecords != 1 || snapshot.SyncCount == 0 || snapshot.CheckpointCount != 1 {
		t.Fatalf("MetricsSnapshot() = %#v, want append=1 sync>0 checkpoint=1", snapshot)
	}
	if snapshot.ReplayRecords != 1 || snapshot.SegmentCount != 1 {
		t.Fatalf("MetricsSnapshot() replay/segments = %#v, want replay=1 segment=1", snapshot)
	}
	if snapshot.AppendLatencyNanos == 0 || snapshot.SyncLatencyNanos == 0 {
		t.Fatalf("MetricsSnapshot() latencies = %#v, want non-zero append and sync latency", snapshot)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestWALTombstoneReplayRecords(t *testing.T) {
	dir := t.TempDir()
	log, err := Open(dir, Options{Sync: true})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	tombstone := model.Tombstone{
		SeriesIDs: []uint64{1},
		FieldIDs:  []uint32{2},
		StartTime: 10,
		EndTime:   20,
		WriteSeq:  7,
	}
	if err := log.AppendTombstones([]model.Tombstone{tombstone}, true); err != nil {
		t.Fatalf("AppendTombstones() error = %v", err)
	}
	records, err := log.ReplayRecords()
	if err != nil {
		t.Fatalf("ReplayRecords() error = %v", err)
	}
	if len(records) != 1 || len(records[0].Tombstones) != 1 {
		t.Fatalf("records = %#v, want one tombstone record", records)
	}
	if records[0].Tombstones[0].WriteSeq != tombstone.WriteSeq {
		t.Fatalf("tombstone = %#v, want %#v", records[0].Tombstones[0], tombstone)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestTombstoneDecodeAndSizeBoundaries(t *testing.T) {
	if uvarintSize(0) != 1 || uvarintSize(128) != 2 {
		t.Fatalf("uvarintSize boundaries failed")
	}
	if varintSize(-1) != 1 {
		t.Fatalf("varintSize(-1) = %d, want 1", varintSize(-1))
	}
	if _, err := decodeTombstones(nil); err == nil {
		t.Fatal("decodeTombstones(empty) error = nil, want error")
	}
	payload := []byte{1, 1}
	if _, err := decodeTombstones(payload); err == nil {
		t.Fatal("decodeTombstones(truncated) error = nil, want error")
	}
	reader := newBatchReader([]byte{1})
	if _, err := readUint32s(reader, "bad uint32"); err == nil {
		t.Fatal("readUint32s(truncated) error = nil, want error")
	}
}

func TestReplayRejectsUnsupportedTypeBadPayloadAndSmallLength(t *testing.T) {
	tests := []struct {
		name  string
		frame []byte
	}{
		{name: "unsupported type", frame: encodeFrame(99, nil)},
		{name: "bad payload", frame: encodeFrame(recordWriteBatch, []byte{1})},
		{name: "small length", frame: smallLengthFrame()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "000001.wal"), testSegment(tt.frame), 0600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			log, err := Open(dir, Options{})
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			if _, err := log.Replay(); err == nil {
				t.Fatal("Replay() error = nil, want error")
			}
			if err := log.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
		})
	}
}

func TestWALSegmentHasHeaderAndRejectsUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	log, err := Open(dir, Options{Sync: true})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := log.Append([]model.ResolvedPoint{{SeriesID: 1, Timestamp: 1, WriteSeq: 1}}, true); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "000001.wal"))
	if err != nil {
		t.Fatalf("ReadFile(segment) error = %v", err)
	}
	if len(data) < walSegmentHeaderLen || !bytes.Equal(data[:len(walSegmentMagic)], []byte(walSegmentMagic)) {
		t.Fatalf("segment header prefix = %q, want magic %q", data[:min(len(data), len(walSegmentMagic))], walSegmentMagic)
	}

	data[len(walSegmentMagic)] = 0xff
	if err := os.WriteFile(filepath.Join(dir, "000001.wal"), data, 0600); err != nil {
		t.Fatalf("WriteFile(corrupt format) error = %v", err)
	}
	log, err = Open(dir, Options{})
	if err != nil {
		t.Fatalf("Open(corrupt format) error = %v", err)
	}
	if _, err := log.Replay(); err == nil {
		t.Fatal("Replay(corrupt format) error = nil, want unknown format error")
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close(corrupt format) error = %v", err)
	}
}

func TestWALReplayTruncatesPartialLastRecordAfterHeader(t *testing.T) {
	dir := t.TempDir()
	frame := encodeFrame(recordWriteBatch, mustBatch(t, []model.ResolvedPoint{{SeriesID: 1, Timestamp: 1, WriteSeq: 1}}))
	data := append(testSegment(frame), 1, 2, 3)
	path := filepath.Join(dir, "000001.wal")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile(partial segment) error = %v", err)
	}
	log, err := Open(dir, Options{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	got, err := log.Replay()
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Replay() count = %d, want 1", len(got))
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(segment) error = %v", err)
	}
	if info.Size() != int64(len(testSegment(frame))) {
		t.Fatalf("segment size after truncate = %d, want %d", info.Size(), len(testSegment(frame)))
	}
}

func TestBinaryPayloadDecodeValidationErrors(t *testing.T) {
	if _, err := decodeBatch([]byte{0, 1}); err == nil {
		t.Fatal("decodeBatch(trailing) error = nil, want error")
	}

	reader := newBatchReader([]byte{2})
	if _, err := reader.bool("bad bool"); err == nil {
		t.Fatal("bool(invalid) error = nil, want error")
	}
	if err := newBatchReader([]byte{1}).done("wal test"); err == nil {
		t.Fatal("done(trailing) error = nil, want error")
	}
	if _, err := uint32Value("field id", uint64(^uint32(0))+1); err == nil {
		t.Fatal("uint32Value(overflow) error = nil, want error")
	}
	if _, err := readValuePayload(model.FieldType(99), newBatchReader(nil)); err == nil {
		t.Fatal("readValuePayload(unknown) error = nil, want error")
	}
}

func TestWALBatchDictionaryRoundTripAndSize(t *testing.T) {
	records := []model.ResolvedPoint{
		walDictionaryPointForTest(1, 10, 1),
		walDictionaryPointForTest(1, 11, 2),
		walDictionaryPointForTest(1, 12, 3),
	}
	payload, err := encodeBatch(records)
	if err != nil {
		t.Fatalf("encodeBatch() error = %v", err)
	}
	got, err := decodeBatch(payload)
	if err != nil {
		t.Fatalf("decodeBatch() error = %v", err)
	}
	if !reflect.DeepEqual(got, records) {
		t.Fatalf("decodeBatch() = %#v, want %#v", got, records)
	}
	plainEstimate := 0
	for _, record := range records {
		plainEstimate += estimatePointSize(record)
	}
	if len(payload) >= plainEstimate {
		t.Fatalf("dictionary payload size = %d, want smaller than plain estimate %d", len(payload), plainEstimate)
	}
}

func TestEncodeBatchUsesScratchRefsArena(t *testing.T) {
	records := make([]model.ResolvedPoint, 64)
	for index := range records {
		records[index] = walWidePointForTest(uint64(index + 1))
	}
	allocs := testing.AllocsPerRun(100, func() {
		payload, err := encodeBatch(records)
		if err != nil {
			t.Fatalf("encodeBatch() error = %v", err)
		}
		if len(payload) == 0 {
			t.Fatal("payload is empty")
		}
	})
	if allocs > 80 {
		t.Fatalf("encodeBatch allocs/run = %.2f, want <= 80", allocs)
	}
}

func TestEncodeBatchIntoReusesDestination(t *testing.T) {
	records := []model.ResolvedPoint{
		walDictionaryPointForTest(1, 10, 1),
		walDictionaryPointForTest(2, 11, 2),
	}
	first, err := encodeBatchInto(make([]byte, 0, 1024), records)
	if err != nil {
		t.Fatalf("encodeBatchInto(first) error = %v", err)
	}
	second, err := encodeBatchInto(first[:0], records)
	if err != nil {
		t.Fatalf("encodeBatchInto(second) error = %v", err)
	}
	if len(first) == 0 || len(second) == 0 {
		t.Fatal("encoded payload is empty")
	}
	if &first[:cap(first)][0] != &second[:cap(second)][0] {
		t.Fatal("encodeBatchInto did not reuse destination backing array")
	}
}

func TestEncodeFrameIntoReusesDestination(t *testing.T) {
	payload := []byte{1, 2, 3, 4}
	first := encodeFrameInto(make([]byte, 0, 64), recordWriteBatch, payload)
	second := encodeFrameInto(first[:0], recordWriteBatch, payload)
	if len(first) == 0 || len(second) == 0 {
		t.Fatal("encoded frame is empty")
	}
	if &first[:cap(first)][0] != &second[:cap(second)][0] {
		t.Fatal("encodeFrameInto did not reuse destination backing array")
	}
}

func TestBatchIdentityScratchFallbacks(t *testing.T) {
	first := walWidePointForTest(1)
	second := walWidePointForTest(2)
	second.Tags = map[string]string{"host": "b", "region": "west"}
	identities, refs := batchIdentities([]model.ResolvedPoint{first, second, first})
	if len(identities) != 2 {
		t.Fatalf("identity count = %d, want 2", len(identities))
	}
	if !reflect.DeepEqual(refs, []int{0, 1, 0}) {
		t.Fatalf("refs = %#v, want [0 1 0]", refs)
	}
	if _, ok := lastIdentityRef(second, identities[:1]); ok {
		t.Fatal("lastIdentityRef(mismatch) ok = true, want false")
	}
	key, scratch := identityKeyWithScratch(first, nil)
	if key == "" || len(scratch) == 0 {
		t.Fatalf("identityKeyWithScratch() key=%q scratch len=%d, want populated", key, len(scratch))
	}
}

func TestBatchIdentityUsesSeriesIDFastPathWithCollisionFallback(t *testing.T) {
	first := walWidePointForTest(1)
	second := walWidePointForTest(1)
	second.Tags = map[string]string{"host": "b"}
	identities, refs := batchIdentities([]model.ResolvedPoint{first, second, first, second})
	if len(identities) != 2 {
		t.Fatalf("identity count = %d, want 2", len(identities))
	}
	if !reflect.DeepEqual(refs, []int{0, 1, 0, 1}) {
		t.Fatalf("refs = %#v, want [0 1 0 1]", refs)
	}
}

func TestSeriesRefIndexUsesDenseAndSparseLayouts(t *testing.T) {
	dense := newSeriesRefIndex([]model.ResolvedPoint{{SeriesID: 10}, {SeriesID: 12}})
	if len(dense.dense) != 3 || dense.sparse != nil {
		t.Fatalf("dense index = %#v, want dense span 3", dense)
	}
	dense.setIfAbsent(11, 7)
	ref, ok := dense.lookup(11)
	if !ok || ref != 7 {
		t.Fatalf("dense lookup = %d %t, want 7 true", ref, ok)
	}

	sparse := newSeriesRefIndex([]model.ResolvedPoint{{SeriesID: 1}, {SeriesID: 1000}})
	if sparse.sparse == nil || len(sparse.dense) != 0 {
		t.Fatalf("sparse index = %#v, want sparse map", sparse)
	}
	sparse.setIfAbsent(1000, 9)
	ref, ok = sparse.lookup(1000)
	if !ok || ref != 9 {
		t.Fatalf("sparse lookup = %d %t, want 9 true", ref, ok)
	}
}

func TestWALBatchEmptyBatchRoundTrips(t *testing.T) {
	payload, err := encodeBatch(nil)
	if err != nil {
		t.Fatalf("encodeBatch(nil) error = %v", err)
	}
	got, err := decodeBatch(payload)
	if err != nil {
		t.Fatalf("decodeBatch(empty) error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("decodeBatch(empty) count = %d, want 0", len(got))
	}
}

func TestAppendPointRejectsMismatchedFieldRefs(t *testing.T) {
	_, err := appendPoint(nil, walDictionaryPointForTest(1, 1, 1), 0, nil)
	if err == nil {
		t.Fatal("appendPoint(mismatched refs) error = nil, want error")
	}
}

func TestWALBatchRejectsBadReferences(t *testing.T) {
	records := []model.ResolvedPoint{walDictionaryPointForTest(1, 10, 1)}
	payload, err := encodeBatch(records)
	if err != nil {
		t.Fatalf("encodeBatch() error = %v", err)
	}
	for index := 1; index < len(payload); index++ {
		corrupt := append([]byte(nil), payload...)
		corrupt[index] = 0xff
		if _, err := decodeBatch(corrupt); err != nil {
			return
		}
	}
	t.Fatal("decodeBatch(corrupt refs) error = nil, want error")
}

func TestEncodeTypedBatchIntoDecodesAsResolvedPoints(t *testing.T) {
	batch := model.ResolvedTypedBatch{
		Database:        "db",
		RetentionPolicy: "rp",
		Measurement:     "cpu",
		Tags: []model.TagColumn{
			{Name: "region", Values: []string{"west", "east"}},
			{Name: "host", Values: []string{"a", "b"}},
		},
		Timestamps: []int64{10, 20},
		SeriesIDs:  []uint64{1, 2},
		WriteSeqs:  []uint64{7, 8},
		Fields: []model.ResolvedTypedFieldColumn{
			{
				FieldID:       1,
				Name:          "usage",
				Type:          model.FieldFloat64,
				Float64Values: []float64{1.5, 2.5},
			},
			{
				FieldID:      2,
				Name:         "state",
				Type:         model.FieldString,
				StringValues: []string{"ok", "warn"},
			},
		},
	}

	payload, err := encodeTypedBatchInto(nil, batch, nil)
	if err != nil {
		t.Fatalf("encodeTypedBatchInto() error = %v", err)
	}
	got, err := decodeBatch(payload)
	if err != nil {
		t.Fatalf("decodeBatch(typed payload) error = %v", err)
	}
	want := []model.ResolvedPoint{
		{
			Database:        "db",
			RetentionPolicy: "rp",
			Measurement:     "cpu",
			Tags:            map[string]string{"host": "a", "region": "west"},
			SeriesID:        1,
			Timestamp:       10,
			WriteSeq:        7,
			Fields: []model.ResolvedField{
				{FieldID: 1, FieldName: "usage", Type: model.FieldFloat64, Value: model.Float64Value(1.5)},
				{FieldID: 2, FieldName: "state", Type: model.FieldString, Value: model.StringValue("ok")},
			},
		},
		{
			Database:        "db",
			RetentionPolicy: "rp",
			Measurement:     "cpu",
			Tags:            map[string]string{"host": "b", "region": "east"},
			SeriesID:        2,
			Timestamp:       20,
			WriteSeq:        8,
			Fields: []model.ResolvedField{
				{FieldID: 1, FieldName: "usage", Type: model.FieldFloat64, Value: model.Float64Value(2.5)},
				{FieldID: 2, FieldName: "state", Type: model.FieldString, Value: model.StringValue("warn")},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded typed payload = %#v, want %#v", got, want)
	}
}

func TestAppendTagsFastPathsAndStableMultiTagOrder(t *testing.T) {
	tests := []struct {
		name string
		tags map[string]string
		want []byte
	}{
		{name: "none", tags: nil, want: binary.AppendUvarint(nil, 0)},
		{
			name: "one",
			tags: map[string]string{"host": "a"},
			want: encodedTags("host", "a"),
		},
		{
			name: "many",
			tags: map[string]string{"region": "west", "host": "a"},
			want: encodedTags("host", "a", "region", "west"),
		},
	}
	for _, tt := range tests {
		got := appendTags(nil, tt.tags)
		if !bytes.Equal(got, tt.want) {
			t.Fatalf("%s encoded tags = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func waitForWALTest(t *testing.T, timeout time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not satisfied before timeout")
}

func TestDecodeBatchRejectsTruncatedPayloadPrefixes(t *testing.T) {
	payload, err := encodeBatch([]model.ResolvedPoint{{
		Database:        "db",
		RetentionPolicy: "rp",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a", "zone": "z"},
		SeriesID:        1,
		Timestamp:       10,
		WriteSeq:        2,
		Fields: []model.ResolvedField{
			{FieldID: 1, FieldName: "f", Type: model.FieldFloat64, Value: model.Float64Value(1)},
			{FieldID: 2, FieldName: "i", Type: model.FieldInt64, Value: model.Int64Value(2)},
			{FieldID: 3, FieldName: "s", Type: model.FieldString, Value: model.StringValue("ok")},
			{FieldID: 4, FieldName: "b", Type: model.FieldBool, Value: model.BoolValue(false)},
		},
	}})
	if err != nil {
		t.Fatalf("encodeBatch() error = %v", err)
	}
	for size := 0; size < len(payload); size++ {
		if _, err := decodeBatch(payload[:size]); err == nil {
			t.Fatalf("decodeBatch(prefix %d) error = nil, want error", size)
		}
	}
}

func encodedTags(parts ...string) []byte {
	dst := binary.AppendUvarint(nil, uint64(len(parts)/2))
	for index := 0; index < len(parts); index += 2 {
		dst = codec.AppendString(dst, parts[index])
		dst = codec.AppendString(dst, parts[index+1])
	}
	return dst
}

func walDictionaryPointForTest(seriesID uint64, timestamp int64, writeSeq uint64) model.ResolvedPoint {
	return model.ResolvedPoint{
		Database:        "db",
		RetentionPolicy: "rp",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a", "region": "west"},
		SeriesID:        seriesID,
		Timestamp:       timestamp,
		WriteSeq:        writeSeq,
		Fields: []model.ResolvedField{
			{FieldID: 1, FieldName: "usage", Type: model.FieldFloat64, Value: model.Float64Value(float64(timestamp))},
			{FieldID: 2, FieldName: "state", Type: model.FieldString, Value: model.StringValue("ok")},
			{FieldID: 3, FieldName: "active", Type: model.FieldBool, Value: model.BoolValue(true)},
		},
	}
}

func walWidePointForTest(seq uint64) model.ResolvedPoint {
	fields := make([]model.ResolvedField, 10)
	for index := range fields {
		fieldID := uint32(index + 1)
		fields[index] = model.ResolvedField{
			FieldID:   fieldID,
			FieldName: "field_" + string(rune('a'+index)),
			Type:      model.FieldInt64,
			Value:     model.Int64Value(int64(seq) + int64(index)),
		}
	}
	return model.ResolvedPoint{
		Database:        "db",
		RetentionPolicy: "rp",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a", "region": "west"},
		SeriesID:        seq,
		Timestamp:       int64(seq),
		WriteSeq:        seq,
		Fields:          fields,
	}
}

func TestTruncateAllAfterCloseIsSafe(t *testing.T) {
	log, err := Open(t.TempDir(), Options{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := log.TruncateAll(); err != nil {
		t.Fatalf("TruncateAll() after close error = %v", err)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() final error = %v", err)
	}
}

func TestOpenIgnoresNonSegmentsAndReusesLastSegment(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("ignore"), 0600); err != nil {
		t.Fatalf("WriteFile(non-segment) error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0700); err != nil {
		t.Fatalf("Mkdir(non-segment dir) error = %v", err)
	}
	record := model.ResolvedPoint{SeriesID: 10, Timestamp: 20, WriteSeq: 30}
	if err := os.WriteFile(filepath.Join(dir, "000003.wal"), testSegment(encodeFrame(recordWriteBatch, mustBatch(t, []model.ResolvedPoint{record}))), 0600); err != nil {
		t.Fatalf("WriteFile(segment) error = %v", err)
	}

	log, err := Open(dir, Options{SegmentBytes: 4096})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	another := model.ResolvedPoint{SeriesID: 11, Timestamp: 21, WriteSeq: 31}
	if err := log.Append([]model.ResolvedPoint{another}, true); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	got, err := log.Replay()
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Replay() count = %d, want 2", len(got))
	}
	if got[0].SeriesID != record.SeriesID || got[1].SeriesID != another.SeriesID {
		t.Fatalf("Replay() = %#v, want records from existing and appended segment", got)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestReplayRejectsPartialNonLastSegment(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "000001.wal"), append(testSegment(nil), 1, 2, 3), 0600); err != nil {
		t.Fatalf("WriteFile(partial) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "000002.wal"), testSegment(nil), 0600); err != nil {
		t.Fatalf("WriteFile(empty) error = %v", err)
	}
	log, err := Open(dir, Options{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := log.Replay(); err == nil {
		t.Fatal("Replay() error = nil, want partial non-last segment error")
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestTruncateAllKeepsNonSegmentFiles(t *testing.T) {
	dir := t.TempDir()
	notePath := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(notePath, []byte("keep"), 0600); err != nil {
		t.Fatalf("WriteFile(note) error = %v", err)
	}
	log, err := Open(dir, Options{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := log.TruncateAll(); err != nil {
		t.Fatalf("TruncateAll() error = %v", err)
	}
	got, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("ReadFile(note) error = %v", err)
	}
	if string(got) != "keep" {
		t.Fatalf("note data = %q, want %q", string(got), "keep")
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestInternalPathErrors(t *testing.T) {
	log := &Log{dir: "bad\x00path", opts: Options{SegmentBytes: 1}}
	if err := log.openLastSegment(); err == nil {
		t.Fatal("openLastSegment(invalid) error = nil, want error")
	}
	if err := log.openSegment(1); err == nil {
		t.Fatal("openSegment(invalid) error = nil, want error")
	}
	if err := log.TruncateAll(); err == nil {
		t.Fatal("TruncateAll(invalid) error = nil, want error")
	}
	if _, err := listSegments("bad\x00path"); err == nil {
		t.Fatal("listSegments(invalid) error = nil, want error")
	}
	if _, err := replaySegment("bad\x00path", true); err == nil {
		t.Fatal("replaySegment(invalid) error = nil, want error")
	}
	if err := normalizeReadError(errors.New("boom")); err == nil {
		t.Fatal("normalizeReadError() error = nil, want wrapped error")
	}
}

func mustBatch(t *testing.T, records []model.ResolvedPoint) []byte {
	t.Helper()
	data, err := encodeBatch(records)
	if err != nil {
		t.Fatalf("encodeBatch() error = %v", err)
	}
	return data
}

func smallLengthFrame() []byte {
	frame := make([]byte, 4)
	binary.BigEndian.PutUint32(frame, 1)
	return frame
}

func testSegment(frames ...[]byte) []byte {
	out := appendSegmentHeader(nil)
	for _, frame := range frames {
		out = append(out, frame...)
	}
	return out
}
