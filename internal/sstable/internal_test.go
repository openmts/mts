package sstable

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/openmts/mts/internal/codec"
	"github.com/openmts/mts/internal/faultinject"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/storagefs"
)

func writePartForPageTests(root string, level int, id string, columns []model.ColumnData) (PartMeta, error) {
	return WritePartWithOptions(root, level, id, columns, WriteOptions{
		Compression: model.CompressionOptions{ValuePageSamples: testValueBlockPageSamples},
	})
}

func TestBlockReadValidationErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blocks.bin")
	if err := os.WriteFile(path, []byte{1, 2, 3}, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := readBlock(path, blockRef{Offset: 0, Size: 3}); err == nil {
		t.Fatal("readBlock() small frame error = nil, want error")
	}

	frame := make([]byte, 8)
	binary.BigEndian.PutUint32(frame[:4], 99)
	if err := os.WriteFile(path, frame, 0600); err != nil {
		t.Fatalf("WriteFile() mismatch error = %v", err)
	}
	if _, err := readBlock(path, blockRef{Offset: 0, Size: int64(len(frame))}); err == nil {
		t.Fatal("readBlock() length mismatch error = nil, want error")
	}
	if _, err := readBlock(filepath.Join(dir, "missing.bin"), blockRef{}); err == nil {
		t.Fatal("readBlock(missing) error = nil, want error")
	}
	file := mustCreateTestFile(t, filepath.Join(dir, "closed.bin"))
	if err := file.Close(); err != nil {
		t.Fatalf("Close(closed.bin) error = %v", err)
	}
	if _, err := writeBlock(file, []byte("x")); err == nil {
		t.Fatal("writeBlock(closed) error = nil, want error")
	}
	if _, err := newBlockWriter(file); err == nil {
		t.Fatal("newBlockWriter(closed) error = nil, want error")
	}

	readFile := mustCreateTestFile(t, filepath.Join(dir, "read-invalid.bin"))
	if _, err := readBlockFrom(readFile, blockRef{Offset: 0, Size: 3}); err == nil {
		closeErr := readFile.Close()
		t.Fatalf("readBlockFrom(invalid ref) error = nil, want error close = %v", closeErr)
	}
	if err := readFile.Close(); err != nil {
		t.Fatalf("Close(read-invalid.bin) error = %v", err)
	}
}

func TestBlockFramePoolLargeBufferBranch(t *testing.T) {
	frame := borrowBlockFrame(maxPooledBlockFrameBytes + 1)
	if len(frame) != maxPooledBlockFrameBytes+1 {
		t.Fatalf("borrowBlockFrame() len = %d, want %d", len(frame), maxPooledBlockFrameBytes+1)
	}
	releaseBlockFrame(frame)
	small := borrowBlockFrame(16)
	if len(small) != 16 {
		t.Fatalf("borrowBlockFrame(small) len = %d, want 16", len(small))
	}
	releaseBlockFrame(small)
}

func TestCompressionEnabledUsesDefaultMinPageValues(t *testing.T) {
	if compressionEnabled(model.CompressionOptions{Enabled: true}, defaultCompressionMinPageValues-1) {
		t.Fatal("compressionEnabled(default min - 1) = true, want false")
	}
	if !compressionEnabled(model.CompressionOptions{Enabled: true}, defaultCompressionMinPageValues) {
		t.Fatal("compressionEnabled(default min) = false, want true")
	}
}

func TestBlockFramePoolReusesSmallBuffer(t *testing.T) {
	frame := borrowBlockFrame(32)
	if len(frame) != 32 {
		t.Fatalf("borrowBlockFrame() len = %d, want 32", len(frame))
	}
	frame[0] = 7
	releaseBlockFrame(frame)

	reused := borrowBlockFrame(16)
	if len(reused) != 16 {
		t.Fatalf("borrowBlockFrame(reused) len = %d, want 16", len(reused))
	}
	if cap(reused) < 32 {
		t.Fatalf("borrowBlockFrame(reused) cap = %d, want at least 32", cap(reused))
	}
	releaseBlockFrame(reused)
}

func TestBlockPayloadReleaseDoesNotCorruptCopiedData(t *testing.T) {
	dir := t.TempDir()
	file := mustCreateTestFile(t, filepath.Join(dir, "blocks.bin"))
	ref, err := writeBlock(file, []byte("payload"))
	if err != nil {
		t.Fatalf("writeBlock() error = %v", err)
	}
	payload, err := readBlockPayloadFrom(file, ref)
	if err != nil {
		t.Fatalf("readBlockPayloadFrom() error = %v", err)
	}
	copied := append([]byte(nil), payload.Bytes()...)
	payload.Release()
	if string(copied) != "payload" {
		t.Fatalf("copied payload = %q, want payload", string(copied))
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(blocks.bin) error = %v", err)
	}
}

func TestFieldPredicateSampleFilteringCoversNumericStringAndBool(t *testing.T) {
	samples := []model.VersionedSample{
		{Timestamp: 1, Value: model.Float64Value(0.2)},
		{Timestamp: 2, Value: model.Int64Value(7)},
		{Timestamp: 3, Value: model.StringValue("ok")},
		{Timestamp: 4, Value: model.BoolValue(true)},
	}
	query := Query{FieldPredicates: map[uint32][]model.QueryPredicate{
		1: {
			{Kind: model.QueryPredicateFieldGTE, Value: model.Int64Value(1)},
			{Kind: model.QueryPredicateFieldLT, Value: model.Float64Value(10)},
		},
	}}
	filtered := filterSamplesByFieldPredicates(1, append([]model.VersionedSample(nil), samples[:2]...), query)
	if len(filtered) != 1 || filtered[0].Timestamp != 2 {
		t.Fatalf("numeric filtered = %#v, want only timestamp 2", filtered)
	}

	stringQuery := Query{FieldPredicates: map[uint32][]model.QueryPredicate{
		2: {{Kind: model.QueryPredicateFieldEq, Value: model.StringValue("ok")}},
	}}
	filtered = filterSamplesByFieldPredicates(2, append([]model.VersionedSample(nil), samples[2:3]...), stringQuery)
	if len(filtered) != 1 || filtered[0].Timestamp != 3 {
		t.Fatalf("string filtered = %#v, want timestamp 3", filtered)
	}

	boolQuery := Query{FieldPredicates: map[uint32][]model.QueryPredicate{
		3: {{Kind: model.QueryPredicateFieldNe, Value: model.BoolValue(false)}},
	}}
	filtered = filterSamplesByFieldPredicates(3, append([]model.VersionedSample(nil), samples[3:]...), boolQuery)
	if len(filtered) != 1 || filtered[0].Timestamp != 4 {
		t.Fatalf("bool filtered = %#v, want timestamp 4", filtered)
	}

	if compareSampleFieldValue(model.StringValue("x"), model.BoolValue(true)) >= 0 {
		t.Fatal("compareSampleFieldValue(mixed non-numeric) >= 0, want mismatched type sorted before")
	}
	if !sampleMatchesFieldPredicate(model.BoolValue(false), model.QueryPredicate{Kind: model.QueryPredicateTagEq}) {
		t.Fatal("unsupported sample predicate = false, want true for non-field predicate")
	}
}

func TestValuePagePredicatePruningCoversFloatIntBoundaryAndStats(t *testing.T) {
	floatStats := valuePageStats{HasNumeric: true, MinFloat64: 1.5, MaxFloat64: 9.5}
	if !numericPageMayMatchPredicate(floatStats, model.FieldFloat64, model.QueryPredicate{
		Kind:  model.QueryPredicateFieldEq,
		Value: model.Float64Value(5),
	}) {
		t.Fatal("float eq within range = false, want true")
	}
	if numericPageMayMatchPredicate(floatStats, model.FieldFloat64, model.QueryPredicate{
		Kind:  model.QueryPredicateFieldGT,
		Value: model.Float64Value(10),
	}) {
		t.Fatal("float gt outside range = true, want false")
	}

	intStats := valuePageStats{HasNumeric: true, MinInt64: 10, MaxInt64: 20}
	if !numericPageMayMatchPredicate(intStats, model.FieldInt64, model.QueryPredicate{
		Kind:  model.QueryPredicateFieldLTE,
		Value: model.Int64Value(10),
	}) {
		t.Fatal("int lte lower boundary = false, want true")
	}
	if intPageMayMatchPredicate(10, 20, model.QueryPredicate{
		Kind:  model.QueryPredicateFieldLT,
		Value: model.Int64Value(10),
	}) {
		t.Fatal("int lt below lower boundary = true, want false")
	}
	if !numericPageMayMatchPredicate(valuePageStats{}, model.FieldString, model.QueryPredicate{
		Kind:  model.QueryPredicateFieldEq,
		Value: model.StringValue("x"),
	}) {
		t.Fatal("non numeric page match = false, want conservative true")
	}

	page := valuePageRef{MinTime: 10, MaxTime: 20, Stats: intStats}
	query := Query{
		Start: 0,
		End:   30,
		FieldPredicates: map[uint32][]model.QueryPredicate{
			1: {{Kind: model.QueryPredicateFieldGTE, Value: model.Int64Value(15)}},
		},
	}
	if !valuePageMatchesQuery(page, 1, model.FieldInt64, query) {
		t.Fatal("valuePageMatchesQuery(in range) = false, want true")
	}
	query.Start = 21
	query.End = 30
	if valuePageMatchesQuery(page, 1, model.FieldInt64, query) {
		t.Fatal("valuePageMatchesQuery(time skipped) = true, want false")
	}

	refs := []valuePageRef{{MinTime: 1}, {MinTime: 2}, {MinTime: 3}}
	if got := selectBoundaryPageRefs(refs, model.QueryBoundaryFirst); len(got) != 1 || got[0].MinTime != 1 {
		t.Fatalf("first refs = %#v, want first page", got)
	}
	if got := selectBoundaryPageRefs(refs, model.QueryBoundaryLast); len(got) != 1 || got[0].MinTime != 3 {
		t.Fatalf("last refs = %#v, want last page", got)
	}
	if got := selectBoundaryPageRefs(refs, model.QueryBoundaryBoth); len(got) != 2 || got[0].MinTime != 1 || got[1].MinTime != 3 {
		t.Fatalf("both refs = %#v, want first and last page", got)
	}
	floatPredicates := []model.QueryPredicate{
		{Kind: model.QueryPredicateFieldNe, Value: model.Float64Value(2)},
		{Kind: model.QueryPredicateFieldGTE, Value: model.Int64Value(9)},
		{Kind: model.QueryPredicateFieldLT, Value: model.Float64Value(2)},
		{Kind: model.QueryPredicateFieldLTE, Value: model.Int64Value(1)},
		{Kind: model.QueryPredicateTagEq, Value: model.Float64Value(100)},
	}
	wantFloat := []bool{true, true, true, true, true}
	for index, predicate := range floatPredicates {
		if got := floatPageMayMatchPredicate(1, 9, predicate); got != wantFloat[index] {
			t.Fatalf("float predicate %d match=%v, want %v", index, got, wantFloat[index])
		}
	}
	intPredicates := []model.QueryPredicate{
		{Kind: model.QueryPredicateFieldNe, Value: model.Float64Value(15)},
		{Kind: model.QueryPredicateFieldGTE, Value: model.Float64Value(20)},
		{Kind: model.QueryPredicateTagEq, Value: model.Int64Value(100)},
	}
	wantInt := []bool{true, true, true}
	for index, predicate := range intPredicates {
		if got := intPageMayMatchPredicate(10, 20, predicate); got != wantInt[index] {
			t.Fatalf("int predicate %d match=%v, want %v", index, got, wantInt[index])
		}
	}
	if compareSampleString("a", "b") != -1 ||
		compareSampleString("b", "a") != 1 ||
		compareSampleString("a", "a") != 0 {
		t.Fatal("compareSampleString() did not cover less/greater/equal ordering")
	}
	if compareSampleBool(false, true) != -1 ||
		compareSampleBool(true, false) != 1 ||
		compareSampleBool(true, true) != 0 {
		t.Fatal("compareSampleBool() did not cover false/true/equal ordering")
	}
	if floatPageMayMatchPredicate(1, 1, model.QueryPredicate{
		Kind:  model.QueryPredicateFieldNe,
		Value: model.Float64Value(1),
	}) {
		t.Fatal("float ne single-value page matched same value, want false")
	}
	if intPageMayMatchPredicate(10, 10, model.QueryPredicate{
		Kind:  model.QueryPredicateFieldNe,
		Value: model.Int64Value(10),
	}) {
		t.Fatal("int ne single-value page matched same value, want false")
	}
	if got := queryPredicateFloatValue(model.QueryPredicate{Value: model.Int64Value(3)}); got != 3 {
		t.Fatalf("queryPredicateFloatValue(int) = %v, want 3", got)
	}
	if got := queryPredicateIntValue(model.QueryPredicate{Value: model.Float64Value(3.8)}); got != 3 {
		t.Fatalf("queryPredicateIntValue(float) = %d, want truncation to 3", got)
	}
	fullRefs, err := matchingBoundaryPageRefs(nil, refs, true, Query{})
	if err != nil {
		t.Fatalf("matchingBoundaryPageRefs(full range) error = %v", err)
	}
	if len(fullRefs) != len(refs) {
		t.Fatalf("full range refs = %#v, want all refs", fullRefs)
	}
	oneRef := firstAndLastPageRefs(refs[:1])
	if len(oneRef) != 1 || oneRef[0].MinTime != refs[0].MinTime {
		t.Fatalf("firstAndLastPageRefs(single) = %#v, want original single ref", oneRef)
	}
}

func TestMatchingValuePageRefsAndCompressionMemoryEstimates(t *testing.T) {
	payload, err := marshalValuePageIndex(nil, valuePageIndex{
		FieldID:   7,
		FieldType: model.FieldInt64,
		Count:     30,
		Pages: []valuePageRef{
			{
				MinTime: 0,
				MaxTime: 9,
				Stats:   valuePageStats{HasNumeric: true, MinInt64: 1, MaxInt64: 9},
			},
			{
				MinTime: 10,
				MaxTime: 19,
				Stats:   valuePageStats{HasNumeric: true, MinInt64: 10, MaxInt64: 19},
			},
			{
				MinTime: 20,
				MaxTime: 29,
				Stats:   valuePageStats{HasNumeric: true, MinInt64: 20, MaxInt64: 29},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshalValuePageIndex() error = %v", err)
	}
	refs, err := matchingValuePageRefs(payload, Query{
		Start: 0,
		End:   29,
		FieldPredicates: map[uint32][]model.QueryPredicate{
			7: {{Kind: model.QueryPredicateFieldGTE, Value: model.Int64Value(20)}},
		},
	})
	if err != nil {
		t.Fatalf("matchingValuePageRefs() error = %v", err)
	}
	if len(refs) != 1 || refs[0].MinTime != 20 {
		t.Fatalf("refs = %#v, want only last page", refs)
	}
	header := valuePageIndexHeader{count: 30, pageCount: 3}
	if got := matchingValuePageCapacity(header, len(refs)); got != 10 {
		t.Fatalf("matchingValuePageCapacity() = %d, want 10", got)
	}
	if got := matchingValuePageCapacity(valuePageIndexHeader{}, 1); got != 0 {
		t.Fatalf("matchingValuePageCapacity(empty) = %d, want 0", got)
	}

	if estimatePayloadCompressionBytes(payloadCompressionSnappy, 16) <= 16 {
		t.Fatal("snappy compression estimate did not include working memory")
	}
	if estimatePayloadCompressionBytes(payloadCompressionLZ4, 16) <= 16 {
		t.Fatal("lz4 compression estimate did not include working memory")
	}
	if estimatePayloadCompressionBytes(payloadCompressionZSTD, 16) <= 16 {
		t.Fatal("zstd compression estimate did not include working memory")
	}
	if estimatePayloadCompressionBytes(payloadCompressionNone, 16) != 16 {
		t.Fatal("none compression estimate should equal payload bytes")
	}
}

func TestIndexSkipAndErrorWrappingBranches(t *testing.T) {
	payload, err := encodeIndexRowsInto(nil, []indexRow{{
		SeriesID: 1,
		MinTime:  1,
		MaxTime:  2,
		TimeRef:  blockRef{Offset: 1, Size: 2},
		Columns: []columnRef{{
			FieldID:   7,
			FieldType: model.FieldFloat64,
			ValueRef:  blockRef{Offset: 3, Size: 4},
		}},
	}})
	if err != nil {
		t.Fatalf("encodeIndexRowsInto() error = %v", err)
	}
	stream, err := newIndexRowStream(payload)
	if err != nil {
		t.Fatalf("newIndexRowStream() error = %v", err)
	}
	if _, ok, err := stream.nextHeader(); err != nil || !ok {
		t.Fatalf("nextHeader() ok=%v err=%v, want row header", ok, err)
	}
	if err := skipIndexColumnRefs(stream); err != nil {
		t.Fatalf("skipIndexColumnRefs() error = %v", err)
	}
	if err := stream.done(); err != nil {
		t.Fatalf("stream done() error = %v", err)
	}

	writeErr := errors.New("write failed")
	closeErr := errors.New("close failed")
	wrapped := errorsWithClose(writeErr, closeErr)
	if !errors.Is(wrapped, writeErr) || wrapped.Error() == writeErr.Error() {
		t.Fatalf("errorsWithClose() = %v, want wrapped write and close context", wrapped)
	}
	if got := errorsWithClose(writeErr, nil); !errors.Is(got, writeErr) {
		t.Fatalf("errorsWithClose(no close) = %v, want write error", got)
	}
}

func TestSeriesBatchReaderAndQuerySeriesIDsBranches(t *testing.T) {
	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "sst-series-batch", []model.ColumnData{
		columnWithTimestamps(1, 2, 0, 3),
		columnWithTimestamps(2, 2, 10, 3),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	defer func() {
		if err := part.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	reader, err := NewSeriesBatchReader(part, Query{Start: 0, End: 20})
	if err != nil {
		t.Fatalf("NewSeriesBatchReader() error = %v", err)
	}
	if got := reader.SeriesCount(); got != 2 {
		t.Fatalf("SeriesCount() = %d, want 2", got)
	}
	ids := reader.SeriesIDs()
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("SeriesIDs() = %v, want [1 2]", ids)
	}
	ids[0] = 99
	appended := reader.AppendSeriesIDs([]uint64{0})
	if len(appended) != 3 || appended[1] != 1 || appended[2] != 2 {
		t.Fatalf("AppendSeriesIDs() = %v, want [0 1 2]", appended)
	}
	columns, err := reader.QuerySeriesID(1)
	if err != nil {
		t.Fatalf("QuerySeriesID() error = %v", err)
	}
	if len(columns) != 1 || columns[0].SeriesID != 1 {
		t.Fatalf("QuerySeriesID() = %#v, want series 1 column", columns)
	}
	columns, err = reader.QuerySeriesIDs([]uint64{2})
	if err != nil {
		t.Fatalf("QuerySeriesIDs() error = %v", err)
	}
	if len(columns) != 1 || columns[0].SeriesID != 2 {
		t.Fatalf("QuerySeriesIDs() = %#v, want series 2 column", columns)
	}
	columns, err = part.QuerySeriesIDs(Query{Start: 0, End: 20}, []uint64{1})
	if err != nil {
		t.Fatalf("part QuerySeriesIDs() error = %v", err)
	}
	if len(columns) != 1 || columns[0].SeriesID != 1 {
		t.Fatalf("part QuerySeriesIDs() = %#v, want series 1 column", columns)
	}
	emptyReader, err := NewSeriesBatchReader(part, Query{Start: 100, End: 10})
	if err != nil {
		t.Fatalf("NewSeriesBatchReader(empty) error = %v", err)
	}
	if emptyReader.SeriesCount() != 0 || len(emptyReader.SeriesIDs()) != 0 {
		t.Fatalf("empty reader count=%d ids=%v, want empty", emptyReader.SeriesCount(), emptyReader.SeriesIDs())
	}
	if containsSortedSeriesIDOrAll(nil, 9) != true {
		t.Fatal("containsSortedSeriesIDOrAll(nil) = false, want true")
	}
	if containsSortedSeriesIDOrAll([]uint64{1, 3}, 2) {
		t.Fatal("containsSortedSeriesIDOrAll([1,3],2) = true, want false")
	}
	var nilReader *SeriesBatchReader
	if nilReader.SeriesCount() != 0 || len(nilReader.SeriesIDs()) != 0 {
		t.Fatal("nil reader count/ids not empty")
	}
	if got := nilReader.AppendSeriesIDs([]uint64{5}); len(got) != 1 || got[0] != 5 {
		t.Fatalf("nil AppendSeriesIDs() = %v, want original", got)
	}
	if columns, err := nilReader.QuerySeriesID(1); err != nil || len(columns) != 0 {
		t.Fatalf("nil QuerySeriesID() = %#v, %v; want empty nil", columns, err)
	}
	if columns, err := nilReader.QuerySeriesIDs([]uint64{1}); err != nil || len(columns) != 0 {
		t.Fatalf("nil QuerySeriesIDs() = %#v, %v; want empty nil", columns, err)
	}
}

func TestManifestValidateAndCompressionBudgetBranches(t *testing.T) {
	dir := t.TempDir()
	if err := WriteManifest(dir, Manifest{
		Sequence: 10,
		Parts: []PartMeta{{
			ID:    "missing",
			Level: 1,
		}},
	}); err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}
	if _, err := LoadManifestStrict(dir, 11); err == nil {
		t.Fatal("LoadManifestStrict(sequence regression) error = nil, want error")
	}
	if _, err := LoadManifestStrict(dir, 0); err == nil {
		t.Fatal("LoadManifestStrict(missing part) error = nil, want error")
	}
	filePath := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile(not-a-dir) error = %v", err)
	}
	if err := WriteManifest(dir, Manifest{Parts: []PartMeta{{ID: "not-a-dir"}}}); err != nil {
		t.Fatalf("WriteManifest(non-dir ref) error = %v", err)
	}
	if _, err := LoadManifestStrict(dir, 0); err == nil {
		t.Fatal("LoadManifestStrict(non-directory part) error = nil, want error")
	}

	budget := &failingCompressionBudget{err: errors.New("budget exhausted")}
	if _, err := reservePayloadCompressionMemory(budget, payloadCompressionSnappy, 10); err == nil {
		t.Fatal("reservePayloadCompressionMemory(failing budget) error = nil, want error")
	}
	release, err := reservePayloadCompressionMemory(budget, payloadCompressionNone, 10)
	if err != nil {
		t.Fatalf("reservePayloadCompressionMemory(none) error = %v", err)
	}
	release()
	if budget.calls != 1 {
		t.Fatalf("budget calls = %d, want only failing compressed call", budget.calls)
	}
}

func TestValidateBlockRefAndScanSkippedRowBranches(t *testing.T) {
	for _, tt := range []struct {
		name string
		size int64
		ref  blockRef
	}{
		{name: "negative offset", size: 10, ref: blockRef{Offset: -1, Size: 1}},
		{name: "negative size", size: 10, ref: blockRef{Offset: 0, Size: -1}},
		{name: "zero size", size: 10, ref: blockRef{Offset: 0, Size: 0}},
		{name: "overflow", size: 10, ref: blockRef{Offset: 9, Size: 2}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateBlockRefWithinSize(tt.size, tt.ref); err == nil {
				t.Fatal("validateBlockRefWithinSize() error = nil, want error")
			}
		})
	}
	if err := validatePartBlockRef(nil, valuesFile, blockRef{Offset: 0, Size: 1}); err == nil {
		t.Fatal("validatePartBlockRef(nil) error = nil, want error")
	}
	part := &Part{componentSizes: map[string]int64{}}
	if err := validatePartBlockRef(part, valuesFile, blockRef{Offset: 0, Size: 1}); err == nil {
		t.Fatal("validatePartBlockRef(missing size) error = nil, want error")
	}

	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "sst-scan-skip", []model.ColumnData{
		columnWithTimestamps(1, 2, 0, 3),
		columnWithTimestamps(2, 2, 10, 3),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	opened, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	stats := &model.QueryStats{}
	stream, err := opened.ScanColumns(Query{
		Stats: stats,
		Start: 0,
		End:   5,
	})
	if err != nil {
		closeErr := opened.Close()
		t.Fatalf("ScanColumns() error = %v close = %v", err, closeErr)
	}
	for stream.Next() {
		_ = stream.ColumnData()
	}
	if err := stream.Err(); err != nil {
		closeErr := opened.Close()
		t.Fatalf("stream Err() = %v close = %v", err, closeErr)
	}
	if err := stream.Close(); err != nil {
		closeErr := opened.Close()
		t.Fatalf("stream Close() error = %v part close = %v", err, closeErr)
	}
	if stats.IndexRowsRead == 0 || stats.IndexRowsSkipped == 0 {
		closeErr := opened.Close()
		t.Fatalf("stats = %#v, want read and skipped index rows close = %v", stats, closeErr)
	}
	if err := opened.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestPartReadFilesComponentAndTrustedOpenBranches(t *testing.T) {
	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "sst-read-files", []model.ColumnData{
		columnWithTimestamps(1, 2, 0, 2),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPartTrusted(meta.Path)
	if err != nil {
		t.Fatalf("OpenPartTrusted() error = %v", err)
	}
	if got := part.Meta(); got.ID != "sst-read-files" {
		closeErr := part.Close()
		t.Fatalf("Meta().ID = %q, want sst-read-files close = %v", got.ID, closeErr)
	}
	if readFileForComponent(indexFile, part.files) == nil ||
		readFileForComponent(timestampsFile, part.files) == nil ||
		readFileForComponent(valuesFile, part.files) == nil ||
		readFileForComponent("unknown", part.files) != nil {
		closeErr := part.Close()
		t.Fatalf("readFileForComponent() returned unexpected file close = %v", closeErr)
	}
	if size, err := partComponentSize(meta.Path, indexFile, part.files); err != nil || size <= 0 {
		closeErr := part.Close()
		t.Fatalf("partComponentSize(open file) = %d, %v close = %v", size, err, closeErr)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close(second) error = %v", err)
	}
	if err := (*Part)(nil).Close(); err != nil {
		t.Fatalf("nil Part Close() error = %v", err)
	}
	if err := (*partReadFiles)(nil).close(); err != nil {
		t.Fatalf("nil partReadFiles close() error = %v", err)
	}
	if closeFile(nil, "nil") != nil {
		t.Fatal("closeFile(nil) returned error")
	}
	if _, err := partComponentSize(meta.Path, "missing.bin", nil); err == nil {
		t.Fatal("partComponentSize(missing) error = nil, want error")
	}
	componentDir := filepath.Join(meta.Path, "component-dir")
	if err := os.Mkdir(componentDir, 0700); err != nil {
		t.Fatalf("Mkdir(component-dir) error = %v", err)
	}
	if _, err := partComponentSize(meta.Path, "component-dir", nil); err == nil {
		t.Fatal("partComponentSize(directory) error = nil, want error")
	}
	if _, err := openPartReadFiles(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("openPartReadFiles(missing) error = nil, want error")
	}
}

func TestLowLevelEncodingAndPoolBranches(t *testing.T) {
	encoded := appendDeltaOfDeltaTimestamps(nil, []int64{0, 1, 3, 6})
	if len(encoded) == 0 {
		t.Fatal("appendDeltaOfDeltaTimestamps(non-zero dd) returned empty payload")
	}
	blockFramePool.Put(&struct{}{})
	frame := borrowBlockFrameHandle(32)
	if len(frame.buf) != 32 {
		t.Fatalf("borrowBlockFrameHandle(non-frame pool item) len = %d, want 32", len(frame.buf))
	}
	releaseBlockFrameHandle(frame)
	small := borrowBlockFrameHandle(1)
	releaseBlockFrameHandle(small)
	large := borrowBlockFrameHandle(maxPooledBlockFrameBytes + 1)
	releaseBlockFrameHandle(large)

	if _, _, err := writeIndexBlocks(filepath.Join(t.TempDir(), "index.bin"), []indexRow{{
		TimeRef: blockRef{Offset: 0, Size: -1},
	}}, false); err == nil {
		t.Fatal("writeIndexBlocks(invalid row) error = nil, want error")
	}
	if err := writeMetadata(t.TempDir(), metadata{Part: PartMeta{RowsCount: -1}}); err == nil {
		t.Fatal("writeMetadata(invalid metadata) error = nil, want error")
	}
}

func TestInternalScanCancelAndValidateErrorBranches(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stream := &partColumnDataStream{query: Query{Context: ctx}}
	if stream.Next() {
		t.Fatal("Next(cancelled) = true, want false")
	}
	if !errors.Is(stream.Err(), context.Canceled) {
		t.Fatalf("Err() = %v, want context canceled", stream.Err())
	}

	stream = &partColumnDataStream{query: Query{Context: ctx}}
	if stream.loadNextRow() {
		t.Fatal("loadNextRow(cancelled) = true, want false")
	}
	if !errors.Is(stream.Err(), context.Canceled) {
		t.Fatalf("loadNextRow Err() = %v, want context canceled", stream.Err())
	}
	if stream.loadRowColumns(indexRowHeader{}) {
		t.Fatal("loadRowColumns(cancelled) = true, want false")
	}

	part := &Part{
		metadata:       metadata{Components: []string{indexFile}},
		componentSizes: map[string]int64{},
	}
	if err := validateOpenedPart(part, false); err == nil {
		t.Fatal("validateOpenedPart(missing component size) error = nil, want error")
	}
	part = &Part{
		metadata: metadata{Part: PartMeta{ID: "sst"}},
		componentSizes: map[string]int64{
			timestampsFile: 8,
		},
	}
	if err := validateIndexRows(part, []indexRow{{SeriesID: 1, TimeRef: blockRef{Offset: 0, Size: 16}}}); err == nil {
		t.Fatal("validateIndexRows(bad time ref) error = nil, want error")
	}
}

func TestPartQueryIndexRowsFieldPredicatesAndBoundary(t *testing.T) {
	dir := t.TempDir()
	meta, err := writePartForPageTests(dir, 0, "sst-query-index-rows", []model.ColumnData{
		columnWithTimestamps(1, 2, 0, testValueBlockPageSamples*2),
		columnWithTimestamps(1, 3, 0, testValueBlockPageSamples*2),
		columnWithTimestamps(2, 2, 1000, testValueBlockPageSamples),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	defer func() {
		if err := part.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	stats := &model.QueryStats{}
	columns, err := part.Query(Query{
		Stats:    stats,
		Start:    int64(testValueBlockPageSamples),
		End:      int64(testValueBlockPageSamples*2 - 1),
		FieldIDs: map[uint32]struct{}{2: {}},
		FieldPredicates: map[uint32][]model.QueryPredicate{
			2: {{Kind: model.QueryPredicateFieldGTE, Value: model.Float64Value(float64(testValueBlockPageSamples))}},
		},
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(columns) != 1 || columns[0].FieldID != 2 || len(columns[0].Samples) != testValueBlockPageSamples {
		t.Fatalf("columns = %#v, want one filtered field page", columns)
	}
	if stats.IndexRowsRead == 0 || stats.IndexRowsSkipped == 0 {
		t.Fatalf("stats = %#v, want index reads and skips", stats)
	}

	boundary, err := part.Query(Query{
		Start:    0,
		End:      int64(testValueBlockPageSamples*2 - 1),
		FieldIDs: map[uint32]struct{}{2: {}},
		Boundary: model.QueryBoundaryBoth,
	})
	if err != nil {
		t.Fatalf("Query(boundary) error = %v", err)
	}
	if len(boundary) == 0 || len(boundary[0].Samples) != testValueBlockPageSamples*2 {
		t.Fatalf("boundary columns = %#v, want first and last pages", boundary)
	}
	empty, err := part.Query(Query{Start: 10, End: 1})
	if err != nil {
		t.Fatalf("Query(empty range) error = %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("empty range columns = %#v, want none", empty)
	}
}

func TestPartQueryCompressedAllFieldTypes(t *testing.T) {
	dir := t.TempDir()
	const count = 320
	columns := []model.ColumnData{
		compressedQueryColumn(1, 1, model.FieldFloat64, count, func(index int) model.FieldValue {
			return model.Float64Value(float64(index) * 1.5)
		}),
		compressedQueryColumn(1, 2, model.FieldInt64, count, func(index int) model.FieldValue {
			return model.Int64Value(int64(index * 2))
		}),
		compressedQueryColumn(1, 3, model.FieldString, count, func(index int) model.FieldValue {
			if index%2 == 0 {
				return model.StringValue("stable")
			}
			return model.StringValue("active")
		}),
		compressedQueryColumn(1, 4, model.FieldBool, count, func(index int) model.FieldValue {
			return model.BoolValue(index%2 == 0)
		}),
	}
	meta, err := WritePartWithOptions(dir, 0, "sst-compressed-query", columns, WriteOptions{
		Compression: model.CompressionOptions{
			Enabled:       true,
			Timestamp:     "delta-of-delta",
			Float:         "xor",
			Int:           "delta",
			String:        "dictionary",
			Algorithm:     "zstd",
			MinPageValues: 1,
		},
	})
	if err != nil {
		t.Fatalf("WritePartWithOptions() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	defer func() {
		if err := part.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	got, err := part.Query(Query{Start: 10, End: 20})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("column count = %d, want 4", len(got))
	}
	for _, column := range got {
		if len(column.Samples) != 11 {
			t.Fatalf("field %d sample count = %d, want 11", column.FieldID, len(column.Samples))
		}
	}
}

func compressedQueryColumn(
	seriesID uint64,
	fieldID uint32,
	fieldType model.FieldType,
	count int,
	valueAt func(int) model.FieldValue,
) model.ColumnData {
	samples := make([]model.VersionedSample, 0, count)
	for index := range count {
		samples = append(samples, model.VersionedSample{
			Timestamp: int64(index),
			WriteSeq:  uint64(index + 1),
			Value:     valueAt(index),
		})
	}
	return model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   fieldID,
		FieldType: fieldType,
		Samples:   samples,
	}
}

type failingCompressionBudget struct {
	err   error
	calls int
}

func (b *failingCompressionBudget) ReserveCompressionBytes(int64) (func(), error) {
	b.calls++
	return nil, b.err
}

func TestQueryStatsNoopAndSkippedIndexRows(t *testing.T) {
	stats := &model.QueryStats{}
	query := Query{Stats: stats}
	recordIndexRowSkipped(query)
	recordValuePagesSkipped(query, 2)
	recordSamplesRead(query, 3)
	if stats.IndexRowsSkipped != 1 || stats.ValuePagesSkipped != 2 || stats.SamplesRead != 3 {
		t.Fatalf("stats = %#v, want skipped/read counters", stats)
	}
	recordIndexRowSkipped(Query{})
	recordValuePagesSkipped(Query{Stats: stats}, 0)
	recordSamplesRead(Query{Stats: stats}, 0)
	if stats.ValuePagesSkipped != 2 || stats.SamplesRead != 3 {
		t.Fatalf("stats after noop = %#v, want unchanged skip/read counters", stats)
	}
}

func TestBlockWriterWritesSequentialOffsets(t *testing.T) {
	dir := t.TempDir()
	file := mustCreateTestFile(t, filepath.Join(dir, "blocks.bin"))
	writer, err := newBlockWriter(file)
	if err != nil {
		t.Fatalf("newBlockWriter() error = %v", err)
	}
	first, err := writer.write([]byte("first"))
	if err != nil {
		t.Fatalf("writer.write(first) error = %v", err)
	}
	second, err := writer.write([]byte("second"))
	if err != nil {
		t.Fatalf("writer.write(second) error = %v", err)
	}
	if second.Offset != first.Offset+first.Size {
		t.Fatalf("second offset = %d, want %d", second.Offset, first.Offset+first.Size)
	}
	got, err := readBlockFrom(file, second)
	if err != nil {
		t.Fatalf("readBlockFrom(second) error = %v", err)
	}
	if string(got) != "second" {
		t.Fatalf("second payload = %q, want second", string(got))
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(blocks.bin) error = %v", err)
	}
}

func TestPartWriterAddsSeriesIncrementally(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewPartWriter(dir, 1, "sst-stream", WriteOptions{})
	if err != nil {
		t.Fatalf("NewPartWriter() error = %v", err)
	}
	if err := writer.AddSeries([]model.ColumnData{
		{
			SeriesID:  1,
			FieldID:   1,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.Float64Value(1)},
				{Timestamp: 20, WriteSeq: 2, Value: model.Float64Value(2)},
			},
		},
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldString,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.StringValue("ok")},
			},
		},
	}); err != nil {
		abortErr := writer.Abort()
		t.Fatalf("AddSeries(series 1) error = %v abort = %v", err, abortErr)
	}
	if err := writer.AddSeries([]model.ColumnData{
		{
			SeriesID:  2,
			FieldID:   1,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 30, WriteSeq: 3, Value: model.Float64Value(3)},
			},
		},
	}); err != nil {
		abortErr := writer.Abort()
		t.Fatalf("AddSeries(series 2) error = %v abort = %v", err, abortErr)
	}
	meta, err := writer.Close()
	if err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if meta.Level != 1 || meta.SeriesCount != 2 || meta.RowsCount != 4 {
		t.Fatalf("meta = %#v, want level 1, 2 series, 4 rows", meta)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	defer func() {
		if err := part.Close(); err != nil {
			t.Fatalf("Close(part) error = %v", err)
		}
	}()
	columns, err := part.Query(Query{Start: 0, End: 100})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(columns) != 3 {
		t.Fatalf("column count = %d, want 3", len(columns))
	}
	seriesIDs, err := part.SeriesIDs(Query{Start: 0, End: 100})
	if err != nil {
		t.Fatalf("SeriesIDs() error = %v", err)
	}
	if len(seriesIDs) != 2 || seriesIDs[0] != 1 || seriesIDs[1] != 2 {
		t.Fatalf("SeriesIDs() = %v, want [1 2]", seriesIDs)
	}
	if _, err := writer.Close(); err == nil {
		t.Fatal("Close(already closed) error = nil, want error")
	}
	if err := writer.AddSeries([]model.ColumnData{streamTestColumn(3, 1, 30)}); err == nil {
		t.Fatal("AddSeries(already closed) error = nil, want error")
	}
}

func TestPartWriterAbortAndErrorBranches(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewPartWriter(dir, 1, "sst-abort", WriteOptions{})
	if err != nil {
		t.Fatalf("NewPartWriter(abort) error = %v", err)
	}
	if err := writer.Abort(); err != nil {
		t.Fatalf("Abort() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sst-abort")); !os.IsNotExist(err) {
		t.Fatalf("aborted part stat error = %v, want not exist", err)
	}
	if err := writer.Abort(); err != nil {
		t.Fatalf("second Abort() error = %v", err)
	}
	if err := writer.AddSeries([]model.ColumnData{streamTestColumn(1, 1, 10)}); err == nil {
		t.Fatal("AddSeries(closed) error = nil, want error")
	}

	emptyWriter, err := NewPartWriter(dir, 1, "sst-empty-stream", WriteOptions{})
	if err != nil {
		t.Fatalf("NewPartWriter(empty) error = %v", err)
	}
	if _, err := emptyWriter.Close(); err == nil {
		t.Fatal("Close(empty writer) error = nil, want error")
	}
	if _, err := os.Stat(filepath.Join(dir, "sst-empty-stream")); !os.IsNotExist(err) {
		t.Fatalf("empty closed part stat error = %v, want not exist", err)
	}

	badWriter, err := NewPartWriter(dir, 1, "sst-bad-series", WriteOptions{})
	if err != nil {
		t.Fatalf("NewPartWriter(bad series) error = %v", err)
	}
	if err := badWriter.AddSeries([]model.ColumnData{
		streamTestColumn(1, 1, 10),
		streamTestColumn(2, 1, 10),
	}); err == nil {
		abortErr := badWriter.Abort()
		t.Fatalf("AddSeries(mixed series) error = nil, want error abort = %v", abortErr)
	}
	if err := badWriter.Abort(); err != nil {
		t.Fatalf("Abort(bad series) error = %v", err)
	}

	emptyColumnWriter, err := NewPartWriter(dir, 1, "sst-empty-column", WriteOptions{})
	if err != nil {
		t.Fatalf("NewPartWriter(empty column) error = %v", err)
	}
	if err := emptyColumnWriter.AddSeries([]model.ColumnData{
		{SeriesID: 1, FieldID: 1, FieldType: model.FieldFloat64},
	}); err == nil {
		abortErr := emptyColumnWriter.Abort()
		t.Fatalf("AddSeries(empty column) error = nil, want error abort = %v", abortErr)
	}
	if err := emptyColumnWriter.Abort(); err != nil {
		t.Fatalf("Abort(empty column) error = %v", err)
	}

	cleanupWriter, err := NewPartWriter(dir, 1, "sst-close-cleanup", WriteOptions{})
	if err != nil {
		t.Fatalf("NewPartWriter(cleanup) error = %v", err)
	}
	if err := cleanupWriter.AddSeries([]model.ColumnData{streamTestColumn(1, 1, 10)}); err != nil {
		abortErr := cleanupWriter.Abort()
		t.Fatalf("AddSeries(cleanup) error = %v abort = %v", err, abortErr)
	}
	cleanupPath := filepath.Join(dir, "sst-close-cleanup")
	if err := os.Mkdir(filepath.Join(cleanupPath, indexFile), 0700); err != nil {
		abortErr := cleanupWriter.Abort()
		t.Fatalf("Mkdir(index collision) error = %v abort = %v", err, abortErr)
	}
	if _, err := cleanupWriter.Close(); err == nil {
		t.Fatal("Close(index collision) error = nil, want error")
	}
	if _, err := os.Stat(cleanupPath); !os.IsNotExist(err) {
		t.Fatalf("cleanup part stat error = %v, want not exist", err)
	}
}

func TestPartWriterRejectsInvalidRoot(t *testing.T) {
	if _, err := NewPartWriter("bad\x00path", 1, "sst-bad", WriteOptions{}); err == nil {
		t.Fatal("NewPartWriter(invalid root) error = nil, want error")
	}
}

func TestPartSeriesIDsAndUnsupportedBlockFileBranches(t *testing.T) {
	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "sst-series-ids", []model.ColumnData{
		streamTestColumn(1, 1, 10),
		streamTestColumn(2, 1, 20),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	ids, err := part.SeriesIDs(Query{Start: 100, End: 10})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("SeriesIDs(empty range) error = %v close = %v", err, closeErr)
	}
	if len(ids) != 0 {
		closeErr := part.Close()
		t.Fatalf("SeriesIDs(empty range) = %v, want none close = %v", ids, closeErr)
	}
	ids, err = part.SeriesIDs(Query{
		Start:     0,
		End:       100,
		SeriesIDs: map[uint64]struct{}{99: {}},
	})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("SeriesIDs(filtered) error = %v close = %v", err, closeErr)
	}
	if len(ids) != 0 {
		closeErr := part.Close()
		t.Fatalf("SeriesIDs(filtered) = %v, want none close = %v", ids, closeErr)
	}
	if _, err := part.readBlock("unknown.bin", blockRef{}); err == nil {
		closeErr := part.Close()
		t.Fatalf("readBlock(unsupported) error = nil, want error close = %v", closeErr)
	}
	if _, err := part.readBlockPayload("unknown.bin", blockRef{}); err == nil {
		closeErr := part.Close()
		t.Fatalf("readBlockPayload(unsupported) error = nil, want error close = %v", closeErr)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	closedPart := &Part{path: meta.Path}
	if _, err := closedPart.readBlock("missing.bin", blockRef{}); err == nil {
		t.Fatal("readBlock(closed missing file) error = nil, want error")
	}
	if _, err := closedPart.readBlockPayload("missing.bin", blockRef{}); err == nil {
		t.Fatal("readBlockPayload(closed missing file) error = nil, want error")
	}
}

func TestIndexRowStreamSkipsAndReportsUndrainedRows(t *testing.T) {
	encoded, err := encodeIndexRows([]indexRow{
		{
			SeriesID: 1,
			MinTime:  10,
			MaxTime:  20,
			Columns: []columnRef{
				{FieldID: 1, FieldType: model.FieldFloat64, ValueRef: blockRef{Offset: 1, Size: 2}},
			},
		},
	})
	if err != nil {
		t.Fatalf("encodeIndexRows() error = %v", err)
	}
	stream, err := newIndexRowStream(encoded)
	if err != nil {
		t.Fatalf("newIndexRowStream() error = %v", err)
	}
	if err := stream.done(); err == nil {
		t.Fatal("done(undrained) error = nil, want error")
	}
	header, ok, err := stream.nextHeader()
	if err != nil {
		t.Fatalf("nextHeader() error = %v", err)
	}
	if !ok || header.seriesID != 1 {
		t.Fatalf("nextHeader() = %#v ok=%v, want series 1", header, ok)
	}
	if err := stream.skipColumnRefs(); err != nil {
		t.Fatalf("skipColumnRefs() error = %v", err)
	}
	if _, ok, err := stream.nextHeader(); err != nil || ok {
		t.Fatalf("nextHeader(done) ok=%v err=%v, want false nil", ok, err)
	}
	if err := stream.done(); err != nil {
		t.Fatalf("done() error = %v", err)
	}
}

func TestEncodeIndexRowsIntoReusesDestination(t *testing.T) {
	row := indexRow{
		SeriesID: 1,
		MinTime:  10,
		MaxTime:  20,
		TimeRef:  blockRef{Offset: 1, Size: 2},
		Columns: []columnRef{
			{FieldID: 1, FieldType: model.FieldFloat64, ValueRef: blockRef{Offset: 3, Size: 4}},
		},
	}
	dst := make([]byte, 0, 256)
	encoded, err := encodeIndexRowsInto(dst, []indexRow{row})
	if err != nil {
		t.Fatalf("encodeIndexRowsInto() error = %v", err)
	}
	if cap(encoded) != cap(dst) {
		t.Fatalf("encoded cap = %d, want reused cap %d", cap(encoded), cap(dst))
	}
	decoded, err := decodeIndexRows(encoded)
	if err != nil {
		t.Fatalf("decodeIndexRows() error = %v", err)
	}
	if len(decoded) != 1 || decoded[0].SeriesID != row.SeriesID {
		t.Fatalf("decoded rows = %#v, want row %#v", decoded, row)
	}
}

func TestSeriesBatchReaderCachesIndexRows(t *testing.T) {
	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "sst-series-reader", []model.ColumnData{
		streamTestColumn(1, 1, 10),
		streamTestColumn(2, 1, 20),
		streamTestColumn(3, 1, 30),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	reader, err := NewSeriesBatchReader(part, Query{Start: 0, End: 100})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("NewSeriesBatchReader() error = %v close = %v", err, closeErr)
	}
	ids := reader.SeriesIDs()
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		closeErr := part.Close()
		t.Fatalf("SeriesIDs() = %v, want [1 2 3] close = %v", ids, closeErr)
	}
	ids[0] = 99
	if reader.SeriesIDs()[0] != 1 {
		closeErr := part.Close()
		t.Fatalf("SeriesIDs() exposed internal slice close = %v", closeErr)
	}
	if reader.SeriesCount() != 3 {
		closeErr := part.Close()
		t.Fatalf("SeriesCount() = %d, want 3 close = %v", reader.SeriesCount(), closeErr)
	}
	appended := reader.AppendSeriesIDs([]uint64{0})
	if len(appended) != 4 || appended[0] != 0 || appended[1] != 1 || appended[3] != 3 {
		closeErr := part.Close()
		t.Fatalf("AppendSeriesIDs() = %v, want [0 1 2 3] close = %v", appended, closeErr)
	}
	direct, err := part.QuerySeriesIDs(Query{Start: 0, End: 100}, []uint64{1, 3})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("Part.QuerySeriesIDs() error = %v close = %v", err, closeErr)
	}
	if len(direct) != 2 || direct[0].SeriesID != 1 || direct[1].SeriesID != 3 {
		closeErr := part.Close()
		t.Fatalf("Part.QuerySeriesIDs() = %#v, want series 1 and 3 close = %v", direct, closeErr)
	}
	directEmpty, err := part.QuerySeriesIDs(Query{Start: 100, End: 10}, []uint64{1})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("Part.QuerySeriesIDs(empty range) error = %v close = %v", err, closeErr)
	}
	if len(directEmpty) != 0 {
		closeErr := part.Close()
		t.Fatalf("Part.QuerySeriesIDs(empty range) = %d columns, want 0 close = %v", len(directEmpty), closeErr)
	}
	filteredMeta, err := WritePart(dir, 0, "sst-series-reader-fields", []model.ColumnData{
		{
			SeriesID:  1,
			FieldID:   1,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.Float64Value(1)},
			},
		},
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.Float64Value(2)},
			},
		},
	})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("WritePart(filtered fields) error = %v close = %v", err, closeErr)
	}
	filteredPart, err := OpenPart(filteredMeta.Path)
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("OpenPart(filtered fields) error = %v close = %v", err, closeErr)
	}
	stats := filteredPart.resetReadStatsForTest()
	fieldColumns, err := filteredPart.QuerySeriesIDs(Query{
		FieldIDs: map[uint32]struct{}{2: {}},
		Start:    0,
		End:      100,
	}, []uint64{1})
	if err != nil {
		filteredCloseErr := filteredPart.Close()
		closeErr := part.Close()
		t.Fatalf("QuerySeriesIDs(filtered fields) error = %v filtered close = %v close = %v", err, filteredCloseErr, closeErr)
	}
	if len(fieldColumns) != 1 || fieldColumns[0].FieldID != 2 {
		filteredCloseErr := filteredPart.Close()
		closeErr := part.Close()
		t.Fatalf("QuerySeriesIDs(filtered fields) = %#v, want only field 2 filtered close = %v close = %v", fieldColumns, filteredCloseErr, closeErr)
	}
	if stats.TimeBlocksRead != 1 || stats.ValueBlocksRead != 1 {
		filteredCloseErr := filteredPart.Close()
		closeErr := part.Close()
		t.Fatalf("read stats = %#v, want one time block and one value block filtered close = %v close = %v", stats, filteredCloseErr, closeErr)
	}
	stats = filteredPart.resetReadStatsForTest()
	missingFieldColumns, err := filteredPart.QuerySeriesIDs(Query{
		FieldIDs: map[uint32]struct{}{99: {}},
		Start:    0,
		End:      100,
	}, []uint64{1})
	if err != nil {
		filteredCloseErr := filteredPart.Close()
		closeErr := part.Close()
		t.Fatalf("QuerySeriesIDs(missing field) error = %v filtered close = %v close = %v", err, filteredCloseErr, closeErr)
	}
	if len(missingFieldColumns) != 0 {
		filteredCloseErr := filteredPart.Close()
		closeErr := part.Close()
		t.Fatalf("QuerySeriesIDs(missing field) = %#v, want none filtered close = %v close = %v", missingFieldColumns, filteredCloseErr, closeErr)
	}
	if stats.TimeBlocksRead != 0 || stats.ValueBlocksRead != 0 {
		filteredCloseErr := filteredPart.Close()
		closeErr := part.Close()
		t.Fatalf("missing field read stats = %#v, want no data block reads filtered close = %v close = %v", stats, filteredCloseErr, closeErr)
	}
	if err := filteredPart.Close(); err != nil {
		closeErr := part.Close()
		t.Fatalf("Close(filteredPart) error = %v close = %v", err, closeErr)
	}
	if err := os.WriteFile(filepath.Join(meta.Path, indexFile), []byte{0xff}, 0600); err != nil {
		closeErr := part.Close()
		t.Fatalf("WriteFile(corrupt index) error = %v close = %v", err, closeErr)
	}
	if _, err := part.QuerySeriesIDs(Query{Start: 0, End: 100}, []uint64{2}); err == nil {
		closeErr := part.Close()
		t.Fatalf("Part.QuerySeriesIDs(corrupt index) error = nil, want error close = %v", closeErr)
	}
	columns, err := reader.QuerySeriesIDs([]uint64{2, 3})
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("QuerySeriesIDs() error = %v close = %v", err, closeErr)
	}
	if len(columns) != 2 || columns[0].SeriesID != 2 || columns[1].SeriesID != 3 {
		closeErr := part.Close()
		t.Fatalf("QuerySeriesIDs() = %#v, want series 2 and 3 close = %v", columns, closeErr)
	}
	single, err := reader.QuerySeriesID(2)
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("QuerySeriesID() error = %v close = %v", err, closeErr)
	}
	if len(single) != 1 || single[0].SeriesID != 2 {
		closeErr := part.Close()
		t.Fatalf("QuerySeriesID() = %#v, want only series 2 close = %v", single, closeErr)
	}
	missingSingle, err := reader.QuerySeriesID(99)
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("QuerySeriesID(missing) error = %v close = %v", err, closeErr)
	}
	if len(missingSingle) != 0 {
		closeErr := part.Close()
		t.Fatalf("QuerySeriesID(missing) = %#v, want none close = %v", missingSingle, closeErr)
	}
	empty, err := reader.QuerySeriesIDs(nil)
	if err != nil {
		closeErr := part.Close()
		t.Fatalf("QuerySeriesIDs(nil) error = %v close = %v", err, closeErr)
	}
	if len(empty) != 0 {
		closeErr := part.Close()
		t.Fatalf("QuerySeriesIDs(nil) len = %d, want 0 close = %v", len(empty), closeErr)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	emptyReader, err := NewSeriesBatchReader(part, Query{Start: 100, End: 10})
	if err != nil {
		t.Fatalf("NewSeriesBatchReader(empty range) error = %v", err)
	}
	if emptyReader.SeriesCount() != 0 {
		t.Fatalf("empty SeriesCount() = %d, want 0", emptyReader.SeriesCount())
	}
	if got := emptyReader.AppendSeriesIDs(nil); len(got) != 0 {
		t.Fatalf("empty AppendSeriesIDs() = %v, want none", got)
	}
	var nilReader *SeriesBatchReader
	if nilReader.SeriesCount() != 0 {
		t.Fatalf("nil SeriesCount() = %d, want 0", nilReader.SeriesCount())
	}
	if got := nilReader.AppendSeriesIDs([]uint64{7}); len(got) != 1 || got[0] != 7 {
		t.Fatalf("nil AppendSeriesIDs() = %v, want [7]", got)
	}
	if got, err := nilReader.QuerySeriesID(1); err != nil || len(got) != 0 {
		t.Fatalf("nil QuerySeriesID() = %#v err = %v, want empty nil", got, err)
	}
}

func streamTestColumn(seriesID uint64, fieldID uint32, timestamp int64) model.ColumnData {
	return model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   fieldID,
		FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{
			{Timestamp: timestamp, WriteSeq: 1, Value: model.Float64Value(float64(timestamp))},
		},
	}
}

func TestPartUnknownEncodingsAndSortHelpers(t *testing.T) {
	dir := t.TempDir()
	timePath := filepath.Join(dir, timestampsFile)
	valuePath := filepath.Join(dir, valuesFile)
	timeFile := mustCreateTestFile(t, timePath)
	timeRef, err := writeBlock(timeFile, []byte(`{"encoding":"unknown","timestamps":[1]}`))
	if err != nil {
		t.Fatalf("writeBlock(time) error = %v", err)
	}
	if err := timeFile.Close(); err != nil {
		t.Fatalf("Close(time) error = %v", err)
	}
	valueFile := mustCreateTestFile(t, valuePath)
	valueRef, err := writeBlock(valueFile, []byte(`{"encoding":"unknown","field_id":2,"samples":[]}`))
	if err != nil {
		t.Fatalf("writeBlock(value) error = %v", err)
	}
	if err := valueFile.Close(); err != nil {
		t.Fatalf("Close(value) error = %v", err)
	}

	part := &Part{path: dir}
	if _, err := part.readTimeBlock(timeRef); err == nil {
		t.Fatal("readTimeBlock() unknown encoding error = nil, want error")
	}
	_, err = part.readValueColumn(1, columnRef{FieldID: 2, ValueRef: valueRef}, nil, Query{Start: 0, End: 10})
	if err == nil {
		t.Fatal("readValueColumn() unknown encoding error = nil, want error")
	}

	columns := []model.ColumnData{
		{SeriesID: 2, FieldID: 1},
		{SeriesID: 1, FieldID: 3},
		{SeriesID: 1, FieldID: 2},
	}
	sortColumns(columns)
	if columns[0].SeriesID != 1 || columns[0].FieldID != 2 {
		t.Fatalf("first sorted column = (%d,%d), want (1,2)", columns[0].SeriesID, columns[0].FieldID)
	}
	if !containsSeries(nil, 1) || !containsField(nil, 1) {
		t.Fatal("nil filters should match")
	}
	if rowMatches(indexRow{SeriesID: 1, MinTime: 100, MaxTime: 200}, Query{Start: 0, End: 10}) {
		t.Fatal("rowMatches() matched non-overlapping time range")
	}
	if !rowMatches(indexRow{SeriesID: 1, MinTime: 1, MaxTime: 2}, Query{Start: 0, End: 10}) {
		t.Fatal("rowMatches() did not match overlapping range")
	}
	if partMatches(PartMeta{MinTime: 100, MaxTime: 200}, nil, Query{Start: 0, End: 10}) {
		t.Fatal("partMatches() matched non-overlapping time range")
	}
	if partMatches(PartMeta{MinTime: 0, MaxTime: 10, MinSeriesID: 10, MaxSeriesID: 20}, nil, Query{
		Start:     0,
		End:       10,
		SeriesIDs: map[uint64]struct{}{1: {}},
	}) {
		t.Fatal("partMatches() matched non-overlapping series range")
	}
	if partMatches(PartMeta{MinTime: 0, MaxTime: 10}, []metaIndexRow{{FieldIDs: []uint32{1}}}, Query{
		Start:    0,
		End:      10,
		FieldIDs: map[uint32]struct{}{2: {}},
	}) {
		t.Fatal("partMatches() matched non-overlapping field IDs")
	}
}

func TestGroupColumnsSortsUnsortedSamplesWithoutMutatingInput(t *testing.T) {
	columns := []model.ColumnData{
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldFloat64,
			Samples: []model.VersionedSample{
				{Timestamp: 20, WriteSeq: 2, Value: model.Float64Value(2)},
				{Timestamp: 10, WriteSeq: 1, Value: model.Float64Value(1)},
			},
		},
	}
	grouped := groupColumns(columns)
	got := grouped[1][0].Samples
	if len(got) != 2 || got[0].Timestamp != 10 || got[1].Timestamp != 20 {
		t.Fatalf("grouped samples = %#v, want sorted timestamps 10,20", got)
	}
	if columns[0].Samples[0].Timestamp != 20 {
		t.Fatalf("input samples were mutated: %#v", columns[0].Samples)
	}
	if !samplesSorted(got) {
		t.Fatal("samplesSorted(sorted) = false, want true")
	}
}

func TestCollectTimestampsAlignedAndSparseColumns(t *testing.T) {
	aligned := []model.ColumnData{
		{
			Samples: []model.VersionedSample{
				{Timestamp: 10},
				{Timestamp: 20},
			},
		},
		{
			Samples: []model.VersionedSample{
				{Timestamp: 10},
				{Timestamp: 20},
			},
		},
	}
	got := collectTimestamps(aligned)
	if len(got) != 2 || got[0] != 10 || got[1] != 20 {
		t.Fatalf("aligned timestamps = %v, want [10 20]", got)
	}

	sparse := []model.ColumnData{
		{
			Samples: []model.VersionedSample{
				{Timestamp: 20},
				{Timestamp: 40},
			},
		},
		{
			Samples: []model.VersionedSample{
				{Timestamp: 10},
				{Timestamp: 40},
			},
		},
	}
	got = collectTimestamps(sparse)
	if len(got) != 3 || got[0] != 10 || got[1] != 20 || got[2] != 40 {
		t.Fatalf("sparse timestamps = %v, want [10 20 40]", got)
	}
}

func TestCollectTimestampsSparseOrderedAllocations(t *testing.T) {
	columns := []model.ColumnData{
		columnWithTimestamps(1, 1, 0, 100),
		columnWithTimestamps(1, 2, 50, 100),
		columnWithTimestamps(1, 3, 100, 100),
	}
	allocs := testing.AllocsPerRun(100, func() {
		got := collectTimestamps(columns)
		if len(got) != 200 {
			t.Fatalf("timestamp count = %d, want 200", len(got))
		}
		if got[0] != 0 || got[len(got)-1] != 199 {
			t.Fatalf("timestamp bounds = (%d,%d), want (0,199)", got[0], got[len(got)-1])
		}
	})
	if allocs > 8 {
		t.Fatalf("collectTimestamps ordered allocs/run = %.2f, want <= 8", allocs)
	}
}

func TestCollectTimestampsUnsortedFallbackAndSortedSeriesIDs(t *testing.T) {
	columns := []model.ColumnData{
		{
			Samples: []model.VersionedSample{
				{Timestamp: 30},
				{Timestamp: 10},
			},
		},
		{
			Samples: []model.VersionedSample{
				{Timestamp: 20},
				{Timestamp: 10},
			},
		},
	}
	got := collectTimestamps(columns)
	if len(got) != 3 || got[0] != 10 || got[1] != 20 || got[2] != 30 {
		t.Fatalf("timestamps = %v, want [10 20 30]", got)
	}
	grouped := map[uint64][]model.ColumnData{
		3: nil,
		1: nil,
		2: nil,
	}
	ids := sortedSeriesIDs(grouped)
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Fatalf("series ids = %v, want [1 2 3]", ids)
	}
}

func columnWithTimestamps(seriesID uint64, fieldID uint32, start int64, count int) model.ColumnData {
	samples := make([]model.VersionedSample, 0, count)
	for index := 0; index < count; index++ {
		timestamp := start + int64(index)
		samples = append(samples, model.VersionedSample{
			Timestamp: timestamp,
			WriteSeq:  uint64(index + 1),
			Value:     model.Float64Value(float64(timestamp)),
		})
	}
	return model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   fieldID,
		FieldType: model.FieldFloat64,
		Samples:   samples,
	}
}

func TestPartQueryPrunesValueBlocksByField(t *testing.T) {
	dir := t.TempDir()
	columns := []model.ColumnData{
		columnWithField(1, 1, model.Float64Value(1)),
		columnWithField(1, 2, model.Int64Value(2)),
		columnWithField(1, 3, model.StringValue("skip")),
	}
	meta, err := writePartForPageTests(dir, 0, "sst-000001", columns)
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	stats := part.resetReadStatsForTest()
	got, err := part.Query(Query{
		SeriesIDs: map[uint64]struct{}{1: {}},
		FieldIDs:  map[uint32]struct{}{2: {}},
		Start:     0,
		End:       10,
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 || got[0].FieldID != 2 {
		t.Fatalf("Query() = %#v, want only field 2", got)
	}
	if stats.ValueBlocksRead != 1 {
		t.Fatalf("ValueBlocksRead = %d, want 1", stats.ValueBlocksRead)
	}
}

func TestPartQueryReadsOnlyMatchingValuePages(t *testing.T) {
	dir := t.TempDir()
	columns := []model.ColumnData{
		columnWithTimestamps(1, 2, 0, testValueBlockPageSamples*3),
	}
	meta, err := writePartForPageTests(dir, 0, "sst-000001", columns)
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	stats := part.resetReadStatsForTest()
	got, err := part.Query(Query{
		SeriesIDs: map[uint64]struct{}{1: {}},
		FieldIDs:  map[uint32]struct{}{2: {}},
		Start:     int64(testValueBlockPageSamples + 10),
		End:       int64(testValueBlockPageSamples + 10),
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 || len(got[0].Samples) != 1 {
		t.Fatalf("query result = %#v, want one sample", got)
	}
	if stats.ValuePagesRead != 1 {
		t.Fatalf("value pages read = %d, want 1", stats.ValuePagesRead)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestPartQueryPrunesValuePagesByNumericFieldPredicate(t *testing.T) {
	dir := t.TempDir()
	columns := []model.ColumnData{
		columnWithTimestamps(1, 2, 0, testValueBlockPageSamples*3),
	}
	meta, err := writePartForPageTests(dir, 0, "sst-000001", columns)
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	stats := part.resetReadStatsForTest()
	got, err := part.Query(Query{
		SeriesIDs: map[uint64]struct{}{1: {}},
		FieldIDs:  map[uint32]struct{}{2: {}},
		Start:     0,
		End:       int64(testValueBlockPageSamples*3 - 1),
		FieldPredicates: map[uint32][]model.QueryPredicate{
			2: {
				{
					Kind:  model.QueryPredicateFieldGT,
					Name:  "value",
					Value: model.Float64Value(float64(testValueBlockPageSamples*2 + 100)),
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 || len(got[0].Samples) == 0 {
		t.Fatalf("query result = %#v, want matching samples", got)
	}
	if stats.ValuePagesRead != 1 {
		t.Fatalf("value pages read = %d, want 1", stats.ValuePagesRead)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestPartReadBlockRejectsUnsupportedOpenFileName(t *testing.T) {
	part := &Part{files: &partReadFiles{}}
	if _, err := part.readBlock("unknown.bin", blockRef{}); err == nil {
		t.Fatal("readBlock(unknown) error = nil, want error")
	}
	if _, err := part.readBlockPayload("unknown.bin", blockRef{}); err == nil {
		t.Fatal("readBlockPayload(unknown) error = nil, want error")
	}
}

func TestPartReadBlockUsesOpenFiles(t *testing.T) {
	dir := t.TempDir()
	indexFileHandle, indexRef := writeSingleBlockFileForTest(t, filepath.Join(dir, indexFile), []byte("index"))
	timeFileHandle, timeRef := writeSingleBlockFileForTest(t, filepath.Join(dir, timestampsFile), []byte("time"))
	valueFileHandle, valueRef := writeSingleBlockFileForTest(t, filepath.Join(dir, valuesFile), []byte("value"))
	part := &Part{files: &partReadFiles{
		index:      indexFileHandle,
		timestamps: timeFileHandle,
		values:     valueFileHandle,
	}}
	tests := []struct {
		name string
		file string
		ref  blockRef
		want string
	}{
		{name: "index", file: indexFile, ref: indexRef, want: "index"},
		{name: "timestamps", file: timestampsFile, ref: timeRef, want: "time"},
		{name: "values", file: valuesFile, ref: valueRef, want: "value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := part.readBlock(tt.file, tt.ref)
			if err != nil {
				t.Fatalf("readBlock(%s) error = %v", tt.file, err)
			}
			if string(got) != tt.want {
				t.Fatalf("readBlock(%s) = %q, want %q", tt.file, string(got), tt.want)
			}
		})
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func writeSingleBlockFileForTest(t *testing.T, path string, payload []byte) (*os.File, blockRef) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("OpenFile(%s) error = %v", path, err)
	}
	ref, err := writeBlock(file, payload)
	if err != nil {
		closeErr := file.Close()
		t.Fatalf("writeBlock(%s) error = %v close = %v", path, err, closeErr)
	}
	if _, err := file.Seek(0, 0); err != nil {
		closeErr := file.Close()
		t.Fatalf("Seek(%s) error = %v close = %v", path, err, closeErr)
	}
	return file, ref
}

func TestWritePartWithCompressionOptionsRoundTrips(t *testing.T) {
	dir := t.TempDir()
	column := columnWithTimestamps(1, 2, 0, 64)
	meta, err := WritePartWithOptions(dir, 0, "sst-000001", []model.ColumnData{column}, WriteOptions{
		Compression: model.CompressionOptions{Enabled: true, MinPageValues: 1},
	})
	if err != nil {
		t.Fatalf("WritePartWithOptions() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	got, err := part.Query(Query{
		SeriesIDs: map[uint64]struct{}{1: {}},
		FieldIDs:  map[uint32]struct{}{2: {}},
		Start:     10,
		End:       12,
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 || len(got[0].Samples) != 3 {
		t.Fatalf("compressed query result = %#v, want one column with 3 samples", got)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestWritePartWithSyncReturnsSyncError(t *testing.T) {
	fs := faultinject.NewFS()
	fs.FailNext(faultinject.OpSync, os.ErrPermission)
	restore := storagefs.SetFaultController(fs)
	defer restore()

	_, err := WritePartWithOptions(
		t.TempDir(),
		0,
		"sst-sync",
		[]model.ColumnData{columnWithTimestamps(1, 2, 0, 4)},
		WriteOptions{Sync: true},
	)
	if err == nil {
		t.Fatal("WritePartWithOptions(Sync:true) error = nil, want sync error")
	}
}

func TestWritePartWithPayloadCompressionAlgorithmsRoundTrip(t *testing.T) {
	algorithms := []string{"snappy", "lz4", "zstd"}
	for _, algorithm := range algorithms {
		t.Run(algorithm, func(t *testing.T) {
			dir := t.TempDir()
			columns := payloadCompressionColumns()
			meta, err := WritePartWithOptions(dir, 0, "sst-000001", columns, WriteOptions{
				Compression: model.CompressionOptions{
					Enabled:       true,
					MinPageValues: 1,
					Algorithm:     algorithm,
				},
			})
			if err != nil {
				t.Fatalf("WritePartWithOptions(%s) error = %v", algorithm, err)
			}
			part, err := OpenPart(meta.Path)
			if err != nil {
				t.Fatalf("OpenPart(%s) error = %v", algorithm, err)
			}
			got, err := part.Query(Query{Start: 10, End: 12})
			if err != nil {
				closeErr := part.Close()
				t.Fatalf("Query(%s) error = %v close = %v", algorithm, err, closeErr)
			}
			if err := part.Close(); err != nil {
				t.Fatalf("Close(%s) error = %v", algorithm, err)
			}
			assertPayloadCompressionQuery(t, got)
		})
	}
}

func TestWritePartWithPayloadCompressionReducesValuesFileSize(t *testing.T) {
	columns := payloadCompressionColumns()
	plainDir := t.TempDir()
	plainMeta, err := WritePartWithOptions(plainDir, 0, "sst-plain", columns, WriteOptions{
		Compression: model.CompressionOptions{Enabled: true, MinPageValues: 1},
	})
	if err != nil {
		t.Fatalf("WritePartWithOptions(plain) error = %v", err)
	}
	zstdDir := t.TempDir()
	zstdMeta, err := WritePartWithOptions(zstdDir, 0, "sst-zstd", columns, WriteOptions{
		Compression: model.CompressionOptions{
			Enabled:       true,
			MinPageValues: 1,
			Algorithm:     "zstd",
		},
	})
	if err != nil {
		t.Fatalf("WritePartWithOptions(zstd) error = %v", err)
	}
	plainSize := fileSizeForTest(t, filepath.Join(plainMeta.Path, valuesFile))
	zstdSize := fileSizeForTest(t, filepath.Join(zstdMeta.Path, valuesFile))
	if zstdSize >= plainSize {
		t.Fatalf("zstd values.bin size = %d, want smaller than plain %d", zstdSize, plainSize)
	}
}

func payloadCompressionColumns() []model.ColumnData {
	count := 512
	return []model.ColumnData{
		payloadCompressionColumn(1, 1, model.FieldFloat64, count, func(index int) model.FieldValue {
			return model.Float64Value(42.25)
		}),
		payloadCompressionColumn(1, 2, model.FieldInt64, count, func(index int) model.FieldValue {
			return model.Int64Value(1_000_000 + int64(index%4))
		}),
		payloadCompressionColumn(1, 3, model.FieldString, count, func(index int) model.FieldValue {
			return model.StringValue("payload-compression-value-payload-compression-value")
		}),
		payloadCompressionColumn(1, 4, model.FieldBool, count, func(index int) model.FieldValue {
			return model.BoolValue(index%2 == 0)
		}),
	}
}

func payloadCompressionColumn(
	seriesID uint64,
	fieldID uint32,
	fieldType model.FieldType,
	count int,
	value func(index int) model.FieldValue,
) model.ColumnData {
	samples := make([]model.VersionedSample, count)
	for index := range count {
		samples[index] = model.VersionedSample{
			Timestamp: int64(index),
			WriteSeq:  uint64(index + 1),
			Value:     value(index),
		}
	}
	return model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   fieldID,
		FieldType: fieldType,
		Samples:   samples,
	}
}

func assertPayloadCompressionQuery(t *testing.T, got []model.ColumnData) {
	t.Helper()
	if len(got) != 4 {
		t.Fatalf("column count = %d, want 4", len(got))
	}
	for _, column := range got {
		if len(column.Samples) != 3 {
			t.Fatalf("field %d sample count = %d, want 3", column.FieldID, len(column.Samples))
		}
		if column.Samples[0].Timestamp != 10 || column.Samples[2].Timestamp != 12 {
			t.Fatalf("field %d samples = %#v, want timestamps 10..12", column.FieldID, column.Samples)
		}
	}
}

func fileSizeForTest(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}
	return info.Size()
}

func TestOpenPartQueriesWithAlreadyOpenedBlockFiles(t *testing.T) {
	dir := t.TempDir()
	meta, err := writePartForPageTests(dir, 0, "sst-000001", []model.ColumnData{
		columnWithField(1, 2, model.Float64Value(42)),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}

	for _, name := range []string{indexFile, timestampsFile, valuesFile} {
		path := filepath.Join(meta.Path, name)
		if err := os.Rename(path, path+".moved"); err != nil {
			t.Fatalf("Rename(%s) error = %v", name, err)
		}
	}

	got, err := part.Query(Query{Start: 0, End: 10})
	if err != nil {
		t.Fatalf("Query() after block files moved error = %v", err)
	}
	if len(got) != 1 || len(got[0].Samples) != 1 || got[0].Samples[0].Value.Float64 != 42 {
		t.Fatalf("Query() = %#v, want retained open file data", got)
	}
}

func TestOpenPartRejectsOutOfBoundsMetadataRefs(t *testing.T) {
	dir := t.TempDir()
	meta, err := writePartForPageTests(dir, 0, "sst-000001", []model.ColumnData{
		columnWithField(1, 2, model.Float64Value(42)),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	decoded, err := loadPartMetadata(meta.Path)
	if err != nil {
		t.Fatalf("loadPartMetadata() error = %v", err)
	}
	decoded.IndexRef.Offset = fileSizeForTest(t, filepath.Join(meta.Path, indexFile)) + 1
	if err := writeMetadata(meta.Path, decoded); err != nil {
		t.Fatalf("writeMetadata() error = %v", err)
	}
	if _, err := OpenPart(meta.Path); err == nil {
		t.Fatal("OpenPart(out-of-bounds metadata refs) error = nil, want error")
	}
}

func TestWritePartEmbedsComponentSizes(t *testing.T) {
	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "sst-sizes", []model.ColumnData{
		columnWithField(1, 2, model.Float64Value(42)),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	decoded, err := loadPartMetadata(meta.Path)
	if err != nil {
		t.Fatalf("loadPartMetadata() error = %v", err)
	}
	if len(decoded.ComponentSizes) == 0 {
		t.Fatal("ComponentSizes empty, want embedded sizes")
	}
	for _, name := range metadataComponents(nil) {
		if name == metadataFile {
			continue
		}
		size, ok := decoded.ComponentSizes[name]
		if !ok {
			t.Fatalf("missing component size for %s", name)
		}
		if name != stringsFile && size <= 0 {
			t.Fatalf("component %s size = %d, want > 0", name, size)
		}
	}
	part, err := OpenPartTrusted(meta.Path)
	if err != nil {
		t.Fatalf("OpenPartTrusted() error = %v", err)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestOpenPartRejectsMissingComponent(t *testing.T) {
	dir := t.TempDir()
	meta, err := writePartForPageTests(dir, 0, "sst-000001", []model.ColumnData{
		columnWithField(1, 2, model.Float64Value(42)),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	if err := os.Remove(filepath.Join(meta.Path, stringsFile)); err != nil {
		t.Fatalf("Remove(strings) error = %v", err)
	}
	if _, err := OpenPart(meta.Path); err == nil {
		t.Fatal("OpenPart(missing strings component) error = nil, want error")
	}
}

func TestOpenPartRejectsComponentChecksumCorruption(t *testing.T) {
	dir := t.TempDir()
	meta, err := writePartForPageTests(dir, 0, "sst-000001", []model.ColumnData{
		columnWithTimestamps(1, 2, 0, 300),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	path := filepath.Join(meta.Path, valuesFile)
	file, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("OpenFile(values) error = %v", err)
	}
	if _, err := file.WriteAt([]byte{0xff}, 16); err != nil {
		closeErr := file.Close()
		t.Fatalf("WriteAt(values) error = %v close = %v", err, closeErr)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(values) error = %v", err)
	}
	if _, err := OpenPart(meta.Path); err == nil {
		t.Fatal("OpenPart(corrupt values component) error = nil, want checksum error")
	}
}

func TestOpenPartTrustedSkipsDeepValueValidation(t *testing.T) {
	dir := t.TempDir()
	meta, err := writePartForPageTests(dir, 0, "sst-000001", []model.ColumnData{
		columnWithTimestamps(1, 2, 0, 300),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	path := filepath.Join(meta.Path, valuesFile)
	file, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("OpenFile(values) error = %v", err)
	}
	if _, err := file.WriteAt([]byte{0xff}, 16); err != nil {
		closeErr := file.Close()
		t.Fatalf("WriteAt(values) error = %v close = %v", err, closeErr)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(values) error = %v", err)
	}
	if _, err := OpenPart(meta.Path); err == nil {
		t.Fatal("OpenPart(corrupt values component) error = nil, want checksum error")
	}
	part, err := OpenPartTrusted(meta.Path)
	if err != nil {
		t.Fatalf("OpenPartTrusted(corrupt values component) error = %v, want trusted warm open", err)
	}
	if closeErr := part.Close(); closeErr != nil {
		t.Fatalf("trusted Close() error = %v", closeErr)
	}
}

func TestPartQueryFallsBackToPathAfterClose(t *testing.T) {
	dir := t.TempDir()
	meta, err := writePartForPageTests(dir, 0, "sst-000001", []model.ColumnData{
		columnWithField(1, 2, model.Float64Value(42)),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	got, err := part.Query(Query{Start: 0, End: 10})
	if err != nil {
		t.Fatalf("Query() after Close error = %v", err)
	}
	if len(got) != 1 || got[0].Samples[0].Value.Float64 != 42 {
		t.Fatalf("Query() after Close = %#v, want value 42", got)
	}
}

func TestSSTableSmallHelpersAndOpenPartFilesErrors(t *testing.T) {
	if valuePageCount(0, testValueBlockPageSamples) != 0 {
		t.Fatalf("valuePageCount(0, testValueBlockPageSamples) = %d, want 0", valuePageCount(0, testValueBlockPageSamples))
	}
	if _, err := uint32Value("too large", uint64(^uint32(0))+1); err == nil {
		t.Fatal("uint32Value(overflow) error = nil, want error")
	}
	row := indexRow{
		SeriesID: 10,
		MinTime:  5,
		MaxTime:  10,
		Columns:  []columnRef{{FieldID: 2}},
	}
	if rowMatches(row, Query{SeriesIDs: map[uint64]struct{}{11: {}}, Start: 0, End: 20}) {
		t.Fatal("rowMatches(series mismatch) = true, want false")
	}
	if rowMatches(row, Query{Start: 20, End: 30}) {
		t.Fatal("rowMatches(time mismatch) = true, want false")
	}
	header := indexRowHeader{
		seriesID: 10,
		minTime:  5,
		maxTime:  10,
	}
	if rowHeaderMatches(header, Query{SeriesIDs: map[uint64]struct{}{11: {}}, Start: 0, End: 20}) {
		t.Fatal("rowHeaderMatches(series mismatch) = true, want false")
	}
	if rowHeaderMatches(header, Query{Start: 20, End: 30}) {
		t.Fatal("rowHeaderMatches(time mismatch) = true, want false")
	}
	if _, err := openPartFiles("bad\x00path"); err == nil {
		t.Fatal("openPartFiles(invalid) error = nil, want error")
	}
	pageHeader := valuePageIndexHeader{count: 10, pageCount: 2}
	if got := matchingValuePageCapacity(pageHeader, 0); got != 0 {
		t.Fatalf("matchingValuePageCapacity(no match) = %d, want 0", got)
	}
	if got := matchingValuePageCapacity(pageHeader, 1); got != 5 {
		t.Fatalf("matchingValuePageCapacity(one match) = %d, want 5", got)
	}
	pagePayload, err := marshalValuePageIndex(nil, valuePageIndex{
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Count:     10,
		Pages: []valuePageRef{
			{MinTime: 0, MaxTime: 4, Ref: blockRef{Offset: 1, Size: 2}},
			{MinTime: 5, MaxTime: 9, Ref: blockRef{Offset: 3, Size: 4}},
		},
	})
	if err != nil {
		t.Fatalf("marshalValuePageIndex() error = %v", err)
	}
	gotHeader, matches, err := matchingValuePageIndexHeader(pagePayload, Query{Start: 4, End: 6})
	if err != nil {
		t.Fatalf("matchingValuePageIndexHeader() error = %v", err)
	}
	if gotHeader.fieldID != 1 || gotHeader.pageCount != 2 || matches != 2 {
		t.Fatalf("page header = %#v matches=%d, want field 1 page count 2 matches 2", gotHeader, matches)
	}
	if _, _, err := matchingValuePageIndexHeader(pagePayload[:len(pagePayload)-1], Query{Start: 0, End: 10}); err == nil {
		t.Fatal("matchingValuePageIndexHeader(truncated) error = nil, want error")
	}
	column, err := (*Part)(nil).readValuePagesFromIndexPayload(7, pagePayload, nil, Query{Start: 20, End: 30})
	if err != nil {
		t.Fatalf("readValuePagesFromIndexPayload(no match) error = %v", err)
	}
	if column.SeriesID != 7 || column.FieldID != 1 || len(column.Samples) != 0 {
		t.Fatalf("no-match column = %#v, want empty series 7 field 1", column)
	}
}

func TestValuePageIndexFullRangeUsesSinglePass(t *testing.T) {
	pagePayload, err := marshalValuePageIndex(nil, valuePageIndex{
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Count:     12,
		Pages: []valuePageRef{
			{MinTime: 0, MaxTime: 3, Ref: blockRef{Offset: 1, Size: 2}},
			{MinTime: 4, MaxTime: 7, Ref: blockRef{Offset: 3, Size: 4}},
			{MinTime: 8, MaxTime: 11, Ref: blockRef{Offset: 5, Size: 6}},
		},
	})
	if err != nil {
		t.Fatalf("marshalValuePageIndex() error = %v", err)
	}
	reads := 0
	valuePageRefReadHook = func() {
		reads++
	}
	t.Cleanup(func() {
		valuePageRefReadHook = nil
	})
	header, pages, fullRange, matches, err := scanValuePageIndexCoverage(pagePayload, Query{Start: 0, End: 11})
	if err != nil {
		t.Fatalf("scanValuePageIndexCoverage() error = %v", err)
	}
	if !fullRange || matches != 3 || len(pages) != 3 {
		t.Fatalf("fullRange=%v matches=%d pages=%d, want true 3 3", fullRange, matches, len(pages))
	}
	if header.count != 12 {
		t.Fatalf("header count = %d, want 12", header.count)
	}
	if reads != 3 {
		t.Fatalf("value page refs read = %d, want 3", reads)
	}
}

func TestPartSeriesIndexReadsOnlyMatchingIndexRows(t *testing.T) {
	dir := t.TempDir()
	meta, err := WritePart(dir, 0, "sst-series-index", []model.ColumnData{
		streamTestColumn(1, 1, 10),
		streamTestColumn(2, 1, 20),
		streamTestColumn(3, 1, 30),
		streamTestColumn(4, 1, 40),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	defer func() {
		if err := part.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()

	stats := part.resetReadStatsForTest()
	got, err := part.QuerySeriesIDs(Query{Start: 0, End: 100}, []uint64{3})
	if err != nil {
		t.Fatalf("QuerySeriesIDs() error = %v", err)
	}
	if len(got) != 1 || got[0].SeriesID != 3 {
		t.Fatalf("QuerySeriesIDs() = %#v, want only series 3", got)
	}
	if stats.IndexRowsRead != 1 {
		t.Fatalf("IndexRowsRead = %d, want 1", stats.IndexRowsRead)
	}

	stats = part.resetReadStatsForTest()
	missing, err := part.QuerySeriesIDs(Query{Start: 0, End: 100}, []uint64{99})
	if err != nil {
		t.Fatalf("QuerySeriesIDs(missing) error = %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("QuerySeriesIDs(missing) = %#v, want empty", missing)
	}
	if stats.IndexRowsRead != 0 {
		t.Fatalf("missing IndexRowsRead = %d, want 0", stats.IndexRowsRead)
	}
}

func TestPartCloseIsIdempotentAndNilSafe(t *testing.T) {
	if err := (*Part)(nil).Close(); err != nil {
		t.Fatalf("nil Part Close() error = %v", err)
	}

	dir := t.TempDir()
	meta, err := writePartForPageTests(dir, 0, "sst-000001", []model.ColumnData{
		columnWithField(1, 2, model.Float64Value(1)),
	})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	part, err := OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := part.Close(); err != nil {
		t.Fatalf("Close() second error = %v", err)
	}
}

func TestOpenPartReadFilesReportsMissingFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := openPartReadFiles(dir); err == nil {
		t.Fatal("openPartReadFiles(missing index) error = nil, want error")
	}
	if err := os.WriteFile(filepath.Join(dir, indexFile), nil, 0600); err != nil {
		t.Fatalf("WriteFile(index) error = %v", err)
	}
	if _, err := openPartReadFiles(dir); err == nil {
		t.Fatal("openPartReadFiles(missing timestamps) error = nil, want error")
	}
	if err := os.WriteFile(filepath.Join(dir, timestampsFile), nil, 0600); err != nil {
		t.Fatalf("WriteFile(timestamps) error = %v", err)
	}
	if _, err := openPartReadFiles(dir); err == nil {
		t.Fatal("openPartReadFiles(missing values) error = nil, want error")
	}
}

func TestOpenPartClosesReadFilesOnBadMetaIndex(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{indexFile, timestampsFile, valuesFile} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}
	metaIndexRef, err := writeBinaryBlock(filepath.Join(dir, metaindexFile), []byte("{"), false)
	if err != nil {
		t.Fatalf("writeBinaryBlock(metaindex) error = %v", err)
	}
	meta := metadata{
		Part:         PartMeta{ID: "bad-metaindex"},
		MetaIndexRef: metaIndexRef,
	}
	if err := writeMetadata(dir, meta); err != nil {
		t.Fatalf("writeMetadata() error = %v", err)
	}
	if _, err := OpenPart(dir); err == nil {
		t.Fatal("OpenPart(bad metaindex) error = nil, want error")
	}
}

func TestPartReadBlockRejectsUnknownFile(t *testing.T) {
	part := &Part{files: &partReadFiles{}}
	if _, err := part.readBlock("unknown.bin", blockRef{}); err == nil {
		t.Fatal("readBlock(unknown file) error = nil, want error")
	}
	if err := closeFile(nil, "nil"); err != nil {
		t.Fatalf("closeFile(nil) error = %v", err)
	}
}

func TestValidateBlockRefWithinSize(t *testing.T) {
	if err := validateBlockRefWithinSize(100, blockRef{Offset: 10, Size: 20}); err != nil {
		t.Fatalf("validateBlockRefWithinSize(valid) error = %v", err)
	}
	for _, item := range []struct {
		name string
		size int64
		ref  blockRef
	}{
		{name: "negative offset", size: 100, ref: blockRef{Offset: -1, Size: 1}},
		{name: "negative size", size: 100, ref: blockRef{Offset: 0, Size: -1}},
		{name: "zero size", size: 100, ref: blockRef{Offset: 0, Size: 0}},
		{name: "past end", size: 100, ref: blockRef{Offset: 101, Size: 1}},
		{name: "spans end", size: 100, ref: blockRef{Offset: 99, Size: 2}},
	} {
		t.Run(item.name, func(t *testing.T) {
			if err := validateBlockRefWithinSize(item.size, item.ref); err == nil {
				t.Fatal("validateBlockRefWithinSize() error = nil, want error")
			}
		})
	}
}

func columnWithField(seriesID uint64, fieldID uint32, value model.FieldValue) model.ColumnData {
	return model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   fieldID,
		FieldType: value.Type,
		Samples: []model.VersionedSample{
			{Timestamp: 5, WriteSeq: 1, Value: value},
		},
	}
}

func TestPartDecodeErrors(t *testing.T) {
	dir := t.TempDir()
	timeFile := mustCreateTestFile(t, filepath.Join(dir, timestampsFile))
	timeRef, err := writeBlock(timeFile, []byte("{"))
	if err != nil {
		t.Fatalf("writeBlock(time) error = %v", err)
	}
	if err := timeFile.Close(); err != nil {
		t.Fatalf("Close(time) error = %v", err)
	}
	valueFile := mustCreateTestFile(t, filepath.Join(dir, valuesFile))
	valueRef, err := writeBlock(valueFile, []byte("{"))
	if err != nil {
		t.Fatalf("writeBlock(value) error = %v", err)
	}
	if err := valueFile.Close(); err != nil {
		t.Fatalf("Close(value) error = %v", err)
	}
	part := &Part{path: dir}
	if _, err := part.readTimeBlock(timeRef); err == nil {
		t.Fatal("readTimeBlock(bad json) error = nil, want error")
	}
	_, err = part.readValueColumn(1, columnRef{FieldID: 2, ValueRef: valueRef}, nil, Query{Start: 0, End: 10})
	if err == nil {
		t.Fatal("readValueColumn(bad json) error = nil, want error")
	}
	missingPart := &Part{path: filepath.Join(dir, "missing")}
	if _, err := missingPart.readTimeBlock(blockRef{}); err == nil {
		t.Fatal("readTimeBlock(missing file) error = nil, want error")
	}
	filtered := columnFromBlock(1, filterValueBlock(valueBlock{
		FieldID:   2,
		FieldType: model.FieldInt64,
		Samples: []model.VersionedSample{
			{Timestamp: 100, WriteSeq: 1, Value: model.Int64Value(1)},
		},
	}, Query{Start: 0, End: 10}))
	if len(filtered.Samples) != 0 {
		t.Fatalf("filtered sample count = %d, want 0", len(filtered.Samples))
	}
}

func TestBinaryEncodingValidationErrors(t *testing.T) {
	if _, err := unmarshalTimeBlock([]byte{99, 0}); err == nil {
		t.Fatal("unmarshalTimeBlock(unknown) error = nil, want error")
	}
	if _, err := unmarshalValueBlockWithTimestamps([]byte{99, 0}, nil, Query{Start: 0, End: 1}); err == nil {
		t.Fatal("unmarshalValueBlockWithTimestamps(unknown) error = nil, want error")
	}
	if _, err := marshalValueBlockWithTimestamps(nil, model.ColumnData{
		FieldType: model.FieldType(99),
		Samples:   []model.VersionedSample{{Timestamp: 1, Value: model.FieldValue{Type: model.FieldType(99)}}},
	}, []int64{1}); err == nil {
		t.Fatal("marshalValueBlockWithTimestamps(unknown) error = nil, want error")
	}
	reader := newBlockReader([]byte{1})
	if err := reader.done("sstable test"); err == nil {
		t.Fatal("blockReader.done(trailing) error = nil, want error")
	}
	if _, err := newBlockReader([]byte{0xff, 0xff, 0xff, 0xff, 0x1f}).uint32("overflow"); err == nil {
		t.Fatal("blockReader.uint32(overflow) error = nil, want error")
	}
	if _, err := readBlockRef(newBlockReader([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 1})); err == nil {
		t.Fatal("readBlockRef(overflow) error = nil, want error")
	}
	if _, err := appendBlockRef(nil, blockRef{Offset: -1}); err == nil {
		t.Fatal("appendBlockRef(negative) error = nil, want error")
	}
}

func TestManifestNormalizeAndMissingFile(t *testing.T) {
	manifest := normalizeManifest(Manifest{Parts: []PartMeta{
		{ID: "b", Level: 1},
		{ID: "a", Level: 0},
	}})
	if manifest.Parts[0].ID != "a" {
		t.Fatalf("first manifest part = %q, want a", manifest.Parts[0].ID)
	}
	if normalizeManifest(Manifest{}).Parts == nil {
		t.Fatal("normalizeManifest() Parts = nil, want empty slice")
	}
	dir := t.TempDir()
	loaded, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest(missing) error = %v", err)
	}
	if loaded.Parts == nil || len(loaded.Parts) != 0 {
		t.Fatalf("LoadManifest(missing) parts = %#v, want empty slice", loaded.Parts)
	}
}

func TestMetadataEncodingValidationErrors(t *testing.T) {
	if _, err := encodeMetadata(metadata{Part: PartMeta{RowsCount: -1}}); err == nil {
		t.Fatal("encodeMetadata(negative count) error = nil, want error")
	}
	if _, err := encodeMetadata(metadata{IndexRef: blockRef{Offset: -1}}); err == nil {
		t.Fatal("encodeMetadata(negative ref) error = nil, want error")
	}
	if _, err := encodeIndexRows([]indexRow{{TimeRef: blockRef{Size: -1}}}); err == nil {
		t.Fatal("encodeIndexRows(negative ref) error = nil, want error")
	}
	if _, err := encodeMetaIndexRows([]metaIndexRow{{IndexRef: blockRef{Offset: -1}}}); err == nil {
		t.Fatal("encodeMetaIndexRows(negative ref) error = nil, want error")
	}
	if _, err := decodeMetadata([]byte{1}); err == nil {
		t.Fatal("decodeMetadata(short) error = nil, want error")
	}
	if _, err := decodeIndexRows([]byte{1}); err == nil {
		t.Fatal("decodeIndexRows(short) error = nil, want error")
	}
	if _, err := decodeMetaIndexRows([]byte{1}); err == nil {
		t.Fatal("decodeMetaIndexRows(short) error = nil, want error")
	}
	block := timeBlockFrom([]int64{1, 2})
	if block.Encoding != "plain-int64" || block.MinTime != 1 || block.MaxTime != 2 {
		t.Fatalf("timeBlockFrom() = %#v, want current metadata fields", block)
	}
}

func TestSSTableBinaryDecodersRejectTruncatedPrefixes(t *testing.T) {
	metaPayload, err := encodeMetadata(metadata{
		Part:         PartMeta{ID: "sst", RowsCount: 1, SeriesCount: 1, BlockCount: 1},
		IndexRef:     blockRef{Offset: 1, Size: 2},
		MetaIndexRef: blockRef{Offset: 3, Size: 4},
	})
	if err != nil {
		t.Fatalf("encodeMetadata() error = %v", err)
	}
	assertDecoderRejectsPrefixes(t, metaPayload, func(data []byte) error {
		_, err := decodeMetadata(data)
		return err
	})

	indexPayload, err := encodeIndexRows([]indexRow{{
		SeriesID: 1,
		MinTime:  1,
		MaxTime:  2,
		TimeRef:  blockRef{Offset: 1, Size: 2},
		Columns:  []columnRef{{FieldID: 1, FieldType: model.FieldFloat64, ValueRef: blockRef{Offset: 3, Size: 4}}},
	}})
	if err != nil {
		t.Fatalf("encodeIndexRows() error = %v", err)
	}
	assertDecoderRejectsPrefixes(t, indexPayload, func(data []byte) error {
		_, err := decodeIndexRows(data)
		return err
	})

	metaIndexPayload, err := encodeMetaIndexRows([]metaIndexRow{{
		MinSeriesID: 1,
		MaxSeriesID: 2,
		MinTime:     1,
		MaxTime:     2,
		FieldIDs:    []uint32{1, 2},
		IndexRef:    blockRef{Offset: 1, Size: 2},
	}})
	if err != nil {
		t.Fatalf("encodeMetaIndexRows() error = %v", err)
	}
	assertDecoderRejectsPrefixes(t, metaIndexPayload, func(data []byte) error {
		_, err := decodeMetaIndexRows(data)
		return err
	})

	timePayload := marshalTimeBlock(nil, []int64{1, 2, 4})
	assertDecoderRejectsPrefixes(t, timePayload, func(data []byte) error {
		_, err := unmarshalTimeBlock(data)
		return err
	})

	valueColumn := columnWithField(1, 1, model.StringValue("abc"))
	valuePayload, err := marshalValueBlockWithTimestamps(nil, valueColumn, sampleTimestamps(valueColumn.Samples))
	if err != nil {
		t.Fatalf("marshalValueBlockWithTimestamps() error = %v", err)
	}
	assertDecoderRejectsPrefixes(t, valuePayload, func(data []byte) error {
		_, err := unmarshalValueBlockWithTimestamps(data, sampleTimestamps(valueColumn.Samples), Query{Start: 0, End: 100})
		return err
	})
}

func TestSSTableEnvelopePayloadDecodersRejectTruncatedInnerPayload(t *testing.T) {
	metaPayload, err := encodeMetadata(metadata{
		Part:         PartMeta{ID: "sst", RowsCount: 1, SeriesCount: 1, BlockCount: 1},
		IndexRef:     blockRef{Offset: 1, Size: 2},
		MetaIndexRef: blockRef{Offset: 3, Size: 4},
	})
	if err != nil {
		t.Fatalf("encodeMetadata() error = %v", err)
	}
	assertEnvelopePayloadPrefixes(t, metaPayload, partMagic, func(data []byte) error {
		_, err := decodeMetadata(data)
		return err
	})

	indexPayload, err := encodeIndexRows([]indexRow{{
		SeriesID: 1,
		MinTime:  1,
		MaxTime:  2,
		TimeRef:  blockRef{Offset: 1, Size: 2},
		Columns:  []columnRef{{FieldID: 1, FieldType: model.FieldFloat64, ValueRef: blockRef{Offset: 3, Size: 4}}},
	}})
	if err != nil {
		t.Fatalf("encodeIndexRows() error = %v", err)
	}
	assertEnvelopePayloadPrefixes(t, indexPayload, indexMagic, func(data []byte) error {
		_, err := decodeIndexRows(data)
		return err
	})

	metaIndexPayload, err := encodeMetaIndexRows([]metaIndexRow{{
		MinSeriesID: 1,
		MaxSeriesID: 2,
		MinTime:     1,
		MaxTime:     2,
		FieldIDs:    []uint32{1, 2},
		IndexRef:    blockRef{Offset: 1, Size: 2},
	}})
	if err != nil {
		t.Fatalf("encodeMetaIndexRows() error = %v", err)
	}
	assertEnvelopePayloadPrefixes(t, metaIndexPayload, metaIndexMagic, func(data []byte) error {
		_, err := decodeMetaIndexRows(data)
		return err
	})
}

func assertEnvelopePayloadPrefixes(t *testing.T, frame []byte, magic codec.Magic, decode func([]byte) error) {
	t.Helper()
	env, err := codec.UnmarshalEnvelope(frame, magic)
	if err != nil {
		t.Fatalf("UnmarshalEnvelope() error = %v", err)
	}
	for size := 0; size < len(env.Payload); size++ {
		prefixFrame := codec.MarshalEnvelope(nil, magic, env.Flags, env.Payload[:size])
		if err := decode(prefixFrame); err == nil {
			t.Fatalf("decode(inner prefix %d/%d) error = nil, want error", size, len(env.Payload))
		}
	}
}

func assertDecoderRejectsPrefixes(t *testing.T, payload []byte, decode func([]byte) error) {
	t.Helper()
	for size := 0; size < len(payload); size++ {
		if err := decode(payload[:size]); err == nil {
			t.Fatalf("decode(prefix %d/%d) error = nil, want error", size, len(payload))
		}
	}
}

func TestPartQueryRejectsBadLazyIndexBlock(t *testing.T) {
	dir := t.TempDir()
	indexFileHandle := mustCreateTestFile(t, filepath.Join(dir, indexFile))
	indexRef, err := writeBlock(indexFileHandle, []byte("{"))
	if err != nil {
		t.Fatalf("writeBlock(index) error = %v", err)
	}
	if err := indexFileHandle.Close(); err != nil {
		t.Fatalf("Close(index) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, timestampsFile), nil, 0600); err != nil {
		t.Fatalf("WriteFile(timestamps) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, valuesFile), nil, 0600); err != nil {
		t.Fatalf("WriteFile(values) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, stringsFile), nil, 0600); err != nil {
		t.Fatalf("WriteFile(strings) error = %v", err)
	}
	metaIndexPayload, err := encodeMetaIndexRows([]metaIndexRow{
		{
			MinSeriesID: 1,
			MaxSeriesID: 1,
			MinTime:     0,
			MaxTime:     10,
			FieldIDs:    []uint32{1},
			IndexRef:    indexRef,
		},
	})
	if err != nil {
		t.Fatalf("encodeMetaIndexRows() error = %v", err)
	}
	metaIndexRef, err := writeBinaryBlock(filepath.Join(dir, metaindexFile), metaIndexPayload, false)
	if err != nil {
		t.Fatalf("writeBinaryBlock(metaindex) error = %v", err)
	}
	seriesIndexPayload, err := encodeSeriesIndexRows([]seriesIndexRow{
		{
			SeriesID: 1,
			MinTime:  0,
			MaxTime:  10,
			FieldIDs: []uint32{1},
			IndexRef: indexRef,
		},
	})
	if err != nil {
		t.Fatalf("encodeSeriesIndexRows() error = %v", err)
	}
	seriesIndexRef, err := writeBinaryBlock(filepath.Join(dir, seriesIndexFile), seriesIndexPayload, false)
	if err != nil {
		t.Fatalf("writeBinaryBlock(series index) error = %v", err)
	}
	meta := metadata{
		Part:           PartMeta{ID: "bad-index", MinTime: 0, MaxTime: 10, MinSeriesID: 1, MaxSeriesID: 1},
		IndexRef:       indexRef,
		MetaIndexRef:   metaIndexRef,
		SeriesIndexRef: seriesIndexRef,
	}
	if err := writeMetadata(dir, meta); err != nil {
		t.Fatalf("writeMetadata() error = %v", err)
	}
	if _, err := OpenPart(dir); err == nil {
		t.Fatal("OpenPart(bad lazy index block) error = nil, want error")
	}
}

func TestPartWriterPathErrors(t *testing.T) {
	if _, err := writeBinaryBlock(filepath.Join("bad\x00path", "index.bin"), []byte{1}, false); err == nil {
		t.Fatal("writeBinaryBlock(invalid) error = nil, want error")
	}
	if err := writeMetadata("bad\x00path", metadata{}); err == nil {
		t.Fatal("writeMetadata(invalid) error = nil, want error")
	}
	if err := ensureStringsFile("bad\x00path", false); err == nil {
		t.Fatal("ensureStringsFile(invalid) error = nil, want error")
	}
	if _, err := openWritable("bad\x00path"); err == nil {
		t.Fatal("openWritable(invalid) error = nil, want error")
	}
	meta := newMetadata(0, "bad")
	if err := writePartIndexes("bad\x00path", &meta, nil, false); err == nil {
		t.Fatal("writePartIndexes(invalid) error = nil, want error")
	}
}

func TestWritePartPropagatesUnsupportedValueType(t *testing.T) {
	columns := []model.ColumnData{
		{
			SeriesID:  1,
			FieldID:   2,
			FieldType: model.FieldType(99),
			Samples: []model.VersionedSample{
				{Timestamp: 10, WriteSeq: 1, Value: model.FieldValue{Type: model.FieldType(99)}},
			},
		},
	}
	if _, err := WritePart(t.TempDir(), 0, "sst-bad-type", columns); err == nil {
		t.Fatal("WritePart(unsupported type) error = nil, want encode error")
	}
}

func TestOpenPartFilesAndCloseErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, valuesFile), 0700); err != nil {
		t.Fatalf("Mkdir(values.bin) error = %v", err)
	}
	if _, err := openPartFiles(dir); err == nil {
		t.Fatal("openPartFiles() error = nil, want values open error")
	}
	timestamps := mustCreateTestFile(t, filepath.Join(t.TempDir(), "timestamps.bin"))
	values := mustCreateTestFile(t, filepath.Join(t.TempDir(), "values.bin"))
	if err := timestamps.Close(); err != nil {
		t.Fatalf("Close(timestamps) error = %v", err)
	}
	if err := values.Close(); err != nil {
		t.Fatalf("Close(values) error = %v", err)
	}
	files := &partFiles{timestamps: timestamps, values: values}
	if err := files.close(false); err == nil {
		t.Fatal("partFiles.close() error = nil, want error")
	}
}

func mustCreateTestFile(t *testing.T, path string) *os.File {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	return file
}
