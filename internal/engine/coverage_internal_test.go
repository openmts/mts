package engine

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/openmts/mts/internal/catalog"
	"github.com/openmts/mts/internal/memtable"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/queryexec"
	"github.com/openmts/mts/internal/querylang"
	"github.com/openmts/mts/internal/sstable"
)

func TestTypedBatchConversionCoversAllTypesAndSparseRows(t *testing.T) {
	batch := model.ResolvedTypedBatch{
		Database:        "metrics",
		RetentionPolicy: "hot",
		Measurement:     "cpu",
		Timestamps:      []int64{10, 20, 30},
		SeriesIDs:       []uint64{1, 2, 3},
		WriteSeqs:       []uint64{11, 22, 33},
		Tags: []model.TagColumn{
			{Name: "host", Values: []string{"a", "b", "c"}},
		},
		Fields: []model.ResolvedTypedFieldColumn{
			{FieldID: 1, Name: "usage", Type: model.FieldFloat64, Float64Values: []float64{1, 2, 3}},
			{FieldID: 2, Name: "count", Type: model.FieldInt64, Int64Values: []int64{10, 20, 30}},
			{FieldID: 3, Name: "state", Type: model.FieldString, StringValues: []string{"ok", "warn", "bad"}},
			{FieldID: 4, Name: "ready", Type: model.FieldBool, BoolValues: []bool{true, false, true}},
			{FieldID: 5, Name: "unknown", Type: model.FieldType(99)},
		},
	}
	points := resolvedPointsFromTypedBatch(batch, []int{2, 0})
	if len(points) != 2 || points[0].SeriesID != 3 || points[1].SeriesID != 1 {
		t.Fatalf("points = %#v, want sparse rows 2 and 0", points)
	}
	if points[0].Tags["host"] != "c" || points[0].WriteSeq != 33 {
		t.Fatalf("first point = %#v, want host c write seq 33", points[0])
	}
	if points[0].Fields[0].Value.Float64 != 3 ||
		points[0].Fields[1].Value.Int64 != 30 ||
		points[0].Fields[2].Value.String != "bad" ||
		!points[0].Fields[3].Value.Bool ||
		points[0].Fields[4].Value.Type != model.FieldType(99) {
		t.Fatalf("first point fields = %#v, want all typed values", points[0].Fields)
	}
	if tags := typedTagMapAt(nil, 0); tags != nil {
		t.Fatalf("typedTagMapAt(nil) = %#v, want nil", tags)
	}
}

func TestLocalMetadataStoreCanceledBranches(t *testing.T) {
	store, err := OpenLocalMetadataStore(t.TempDir())
	if err != nil {
		t.Fatalf("OpenLocalMetadataStore() error = %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.ResolvePoints(ctx, []model.Point{{Measurement: "cpu"}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ResolvePoints(cancelled) error = %v", err)
	}
	if _, err := store.ResolveTypedBatch(ctx, model.TypedBatch{Measurement: "cpu", Timestamps: []int64{1}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ResolveTypedBatch(cancelled) error = %v", err)
	}
	if _, err := store.ResolveTypedBatchColumns(ctx, model.TypedBatch{Measurement: "cpu", Timestamps: []int64{1}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ResolveTypedBatchColumns(cancelled) error = %v", err)
	}
	if _, err := store.MatchSeries(ctx, "cpu", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("MatchSeries(cancelled) error = %v", err)
	}
	if _, err := store.FieldIDs(ctx, "cpu", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("FieldIDs(cancelled) error = %v", err)
	}
	if _, err := store.Snapshot(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Snapshot(cancelled) error = %v", err)
	}
	if err := store.CreateDatabase(ctx, "metrics"); !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateDatabase(cancelled) error = %v", err)
	}
	if err := store.DropDatabase(ctx, "metrics"); !errors.Is(err, context.Canceled) {
		t.Fatalf("DropDatabase(cancelled) error = %v", err)
	}
	if err := store.CreateRetentionPolicy(ctx, "metrics", model.RetentionPolicy{Name: "hot"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateRetentionPolicy(cancelled) error = %v", err)
	}
	if _, err := store.ListRetentionPolicies(ctx, "metrics"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListRetentionPolicies(cancelled) error = %v", err)
	}
	if _, err := store.ListMeasurements(ctx, "metrics"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListMeasurements(cancelled) error = %v", err)
	}
	if _, err := store.ListFields(ctx, "metrics", "cpu"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListFields(cancelled) error = %v", err)
	}
	if _, err := store.ListSeries(ctx, "metrics", "cpu", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListSeries(cancelled) error = %v", err)
	}
	if err := (*LocalMetadataStore)(nil).Close(); err != nil {
		t.Fatalf("nil LocalMetadataStore Close() error = %v", err)
	}
}

func TestDefaultPartManagerAndRecoveryIssueBranches(t *testing.T) {
	manager := defaultPartManager{}
	dir := t.TempDir()
	columns := []model.ColumnData{{
		SeriesID:  1,
		FieldID:   1,
		FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{{
			Timestamp: 1,
			WriteSeq:  1,
			Value:     model.Float64Value(1),
		}},
	}}
	meta, err := manager.WritePart(dir, 0, "sst", columns, sstable.WriteOptions{})
	if err != nil {
		t.Fatalf("WritePart() error = %v", err)
	}
	reader, err := manager.OpenPart(meta.Path)
	if err != nil {
		t.Fatalf("OpenPart() error = %v", err)
	}
	if _, err := manager.NewSeriesBatchReader(coverageFakePartReader{}, sstable.Query{}); err == nil {
		closeErr := reader.Close()
		t.Fatalf("NewSeriesBatchReader(fake) error = nil, want error close = %v", closeErr)
	}
	batchReader, err := manager.NewSeriesBatchReader(reader, sstable.Query{Start: 0, End: 10})
	if err != nil {
		closeErr := reader.Close()
		t.Fatalf("NewSeriesBatchReader() error = %v close = %v", err, closeErr)
	}
	if batchReader.SeriesCount() != 1 {
		closeErr := reader.Close()
		t.Fatalf("SeriesCount() = %d, want 1 close = %v", batchReader.SeriesCount(), closeErr)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("reader Close() error = %v", err)
	}

	missingErr := os.ErrNotExist
	issue := partOpenRecoveryIssue(sstable.PartMeta{Path: "missing"}, missingErr)
	if issue.Kind != RecoveryIssueMissingPart || !errors.Is(&issue, ErrRecoveryFatal) ||
		!errors.Is(&issue, missingErr) {
		t.Fatalf("missing issue = %#v, want fatal missing wrapping", issue)
	}
	otherIssue := partOpenRecoveryIssue(sstable.PartMeta{Path: "bad"}, errors.New("boom"))
	if otherIssue.Kind != RecoveryIssuePartOpenFailed {
		t.Fatalf("open issue kind = %s, want part open failed", otherIssue.Kind)
	}
	expected := sstable.PartMeta{ID: "a", Level: 1, MinTime: 1, MaxTime: 2, MinSeriesID: 1, MaxSeriesID: 2, RowsCount: 3, SeriesCount: 4, BlockCount: 5, MaxWriteSeq: 6, Path: "p"}
	actual := sstable.PartMeta{ID: "b", Level: 2, MinTime: 3, MaxTime: 4, MinSeriesID: 5, MaxSeriesID: 6, RowsCount: 7, SeriesCount: 8, BlockCount: 9, MaxWriteSeq: 10, Path: "p"}
	mismatch, ok := partMetadataMismatchIssue(expected, actual)
	if !ok || mismatch.Kind != RecoveryIssueMetadataMismatch {
		t.Fatalf("mismatch issue = %#v ok=%v, want metadata mismatch", mismatch, ok)
	}
	if _, ok := partMetadataMismatchIssue(expected, expected); ok {
		t.Fatal("partMetadataMismatchIssue(equal) ok = true, want false")
	}
	report := RecoveryReport{}
	report.Add(issue)
	report.Add(RecoveryIssue{Kind: RecoveryIssueTempRemoved})
	clone := report.Clone()
	clone.Issues[0].Kind = RecoveryIssueTempRemoved
	if report.Issues[0].Kind != RecoveryIssueMissingPart {
		t.Fatal("RecoveryReport.Clone() did not isolate issue slice")
	}
}

func TestQueryPredicatePureBranches(t *testing.T) {
	query := normalizeStructuredQuery(model.Query{
		Tags: map[string]string{},
		Predicates: []model.QueryPredicate{
			{Kind: model.QueryPredicateTimeRange, Start: 1, End: 2},
			{Kind: model.QueryPredicateTagEq, Name: "host", StringValues: []string{"a"}},
		},
	})
	if query.StartTime != 1 || query.EndTime != 2 || query.Tags["host"] != "a" {
		t.Fatalf("normalized query = %#v, want time and tag pushdown", query)
	}
	expr := andExprFromPredicates([]model.QueryPredicate{
		{Kind: model.QueryPredicateFieldGT, Name: "usage"},
		{Kind: model.QueryPredicateTagExists, Name: "host"},
	})
	if expr.Kind != model.QueryExprAnd || len(expr.Children) != 2 {
		t.Fatalf("and expr = %#v, want two children", expr)
	}
	if storageFieldPredicates(model.Query{Expr: model.QueryExpr{Kind: model.QueryExprOr}}, []model.QueryPredicate{{Name: "usage"}}) != nil {
		t.Fatal("storageFieldPredicates(or expr) != nil, want no unsafe pushdown")
	}
	series := map[uint64]catalog.Series{
		1: {ID: 1, Tags: map[string]string{"host": "a", "region": "west"}},
		2: {ID: 2, Tags: map[string]string{"host": "b"}},
	}
	filtered := filterSeriesIDs([]uint64{1, 2}, series, []model.QueryPredicate{
		{Kind: model.QueryPredicateTagIn, Name: "host", StringValues: []string{"a", "c"}},
		{Kind: model.QueryPredicateTagExists, Name: "region"},
	})
	if len(filtered) != 1 || filtered[0] != 1 {
		t.Fatalf("filtered ids = %v, want [1]", filtered)
	}
	if !seriesMatchesTagExpr(series[2].Tags, model.QueryExpr{Kind: model.QueryExprNot}) {
		t.Fatal("empty NOT tag expr = false, want true")
	}
	if seriesMatchesTagExpr(series[2].Tags, model.QueryExpr{Kind: model.QueryExprKind(99)}) {
		t.Fatal("unknown tag expr = true, want false")
	}
	if !tagPredicateMatches(series[2].Tags, model.QueryPredicate{Kind: model.QueryPredicateTagNe, Name: "missing"}) {
		t.Fatal("tag ne missing = false, want true")
	}
	if stringIn("x", []string{"a", "b"}) {
		t.Fatal("stringIn missing = true, want false")
	}
}

func TestStorageMemoryLimiterCoverageBranches(t *testing.T) {
	var nilLimiter *storageMemoryLimiter
	nilSnapshot := nilLimiter.Snapshot(storageMemoryActive{MemTableBytes: 7, WALBytes: 11})
	if nilSnapshot.CurrentBytes != 18 || nilSnapshot.ActiveBytes != 18 ||
		nilSnapshot.MemTableBytes != 7 || nilSnapshot.WALBytes != 11 {
		t.Fatalf("nil limiter snapshot = %#v, want active bytes populated", nilSnapshot)
	}

	limiter := newStorageMemoryLimiter(model.StorageMemoryOptions{
		HardBytesLimit:  10,
		QueryBytesLimit: 3,
	})
	if _, err := limiter.Reserve(storageMemoryWrite, 9, 2); !errors.Is(err, ErrStorageMemoryLimitExceeded) {
		t.Fatalf("Reserve(write over hard limit) error=%v, want memory limit", err)
	}
	if _, err := limiter.Reserve(storageMemoryQuery, 0, 4); !errors.Is(err, ErrStorageMemoryLimitExceeded) {
		t.Fatalf("Reserve(query over op limit) error=%v, want memory limit", err)
	}
	release, err := limiter.Reserve(storageMemoryFlush, 0, 2)
	if err != nil {
		t.Fatalf("Reserve(flush) error = %v", err)
	}
	release()
	release()
	snapshot := limiter.Snapshot(storageMemoryActive{})
	if snapshot.RejectedWrites != 1 || snapshot.RejectedReservations != 1 || snapshot.ReservationBytes != 0 {
		t.Fatalf("snapshot = %#v, want rejected counters and released reservation", snapshot)
	}

	budget := storageCompressionBudget{}
	compressionRelease, err := budget.ReserveCompressionBytes(128)
	if err != nil {
		t.Fatalf("ReserveCompressionBytes(nil memory) error = %v", err)
	}
	compressionRelease()
}

func TestCompactionStatsRecordSkipAndRecoveryIssueBranches(t *testing.T) {
	var recorder compactionStatsRecorder
	recorder.recordSkip("too_few_parts")
	recorder.recordSafeDeleteParts(0)
	stats := recorder.snapshot()
	if stats.Skipped != 1 || stats.LastSkipReason != "too_few_parts" || stats.SafeDeleteParts != 0 {
		t.Fatalf("stats = %#v, want skip reason only", stats)
	}

	if got := (*RecoveryIssue)(nil).Error(); got != "<nil>" {
		t.Fatalf("nil issue Error() = %q, want <nil>", got)
	}
	if got := (*RecoveryIssue)(nil).Unwrap(); got != nil {
		t.Fatalf("nil issue Unwrap() = %#v, want nil", got)
	}
	plain := &RecoveryIssue{Kind: RecoveryIssueTempRemoved}
	if got := plain.Error(); got != string(RecoveryIssueTempRemoved) {
		t.Fatalf("plain Error() = %q, want kind", got)
	}
	cause := errors.New("disk")
	nonFatal := &RecoveryIssue{Kind: RecoveryIssueOrphanRemoveFailed, Err: cause}
	if !errors.Is(nonFatal, cause) || errors.Is(nonFatal, ErrRecoveryFatal) {
		t.Fatalf("nonfatal issue errors.Is cause=%v fatal=%v", errors.Is(nonFatal, cause), errors.Is(nonFatal, ErrRecoveryFatal))
	}
	fatalOnly := &RecoveryIssue{Kind: RecoveryIssueMissingPart, Fatal: true}
	if !errors.Is(fatalOnly, ErrRecoveryFatal) {
		t.Fatalf("fatal issue does not match ErrRecoveryFatal")
	}
	report := RecoveryReport{}
	report.Add(*fatalOnly)
	report.Add(*nonFatal)
	report.Merge(RecoveryReport{Issues: []RecoveryIssue{{Kind: RecoveryIssueTempRemoved}}})
	if !errors.Is(report.FatalError(), ErrRecoveryFatal) {
		t.Fatalf("FatalError() = %v, want ErrRecoveryFatal", report.FatalError())
	}
	if !errors.Is(report.MaintenanceError(), cause) {
		t.Fatalf("MaintenanceError() = %v, want cause", report.MaintenanceError())
	}
}

func TestShardMemoryAndCleanupBranches(t *testing.T) {
	files := &fakeFileOps{err: errors.New("remove failed")}
	shard := &Shard{
		opts: ShardOptions{
			Dir: t.TempDir(),
		},
		deps: shardDeps{
			files: files,
		},
		mem: &fakeMemStore{samples: 2, snapshot: &fakeMemSnapshot{}},
		wal: &fakeWalStore{
			points: []model.ResolvedPoint{{
				Fields: []model.ResolvedField{{FieldID: 1, Type: model.FieldFloat64}},
			}},
			tombstones: []model.Tombstone{{StartTime: 1, EndTime: 2}},
		},
	}
	if got := shard.ApproxMemoryBytes(); got <= 64 {
		t.Fatalf("ApproxMemoryBytes() = %d, want mem + wal bytes", got)
	}
	shard.wal = nil
	if got := shard.ApproxWALMemoryBytes(); got != 0 {
		t.Fatalf("ApproxWALMemoryBytes(nil wal) = %d, want 0", got)
	}
	if err := shard.removeUnreferencedPart("", "ignored"); err != nil {
		t.Fatalf("removeUnreferencedPart(empty) error = %v", err)
	}
	err := shard.removeUnreferencedPart("orphan", "remove orphan failed")
	if !errors.Is(err, files.err) || len(shard.recoveryReport.Issues) != 1 {
		t.Fatalf("removeUnreferencedPart(error) err=%v report=%#v, want recorded issue", err, shard.recoveryReport)
	}
	if shard.maintenanceErr == nil {
		t.Fatal("maintenanceErr = nil, want nonfatal cleanup error")
	}
}

func TestQueryIteratorAndRowAlignmentBranches(t *testing.T) {
	engine := &Engine{}
	iter, err := engine.queryColumnIteratorFromPlan(context.Background(), QueryPlan{Empty: true})
	if err != nil {
		t.Fatalf("queryColumnIteratorFromPlan(empty) error = %v", err)
	}
	if iter.Next() {
		t.Fatalf("empty iterator Next() = true")
	}
	if stats := iter.Stats(); stats.Errors != 0 {
		t.Fatalf("empty iterator stats = %#v, want no error", stats)
	}
	if err := iter.Close(); err != nil {
		t.Fatalf("empty iterator Close() error = %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := engine.queryColumnIteratorFromPlan(canceled, QueryPlan{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("queryColumnIteratorFromPlan(cancelled) error = %v", err)
	}

	if rows, ok := alignedSeriesRows(nil); !ok || len(rows) != 0 {
		t.Fatalf("alignedSeriesRows(nil) rows=%#v ok=%v, want empty aligned", rows, ok)
	}
	if _, ok := alignedSeriesRows([]model.ColumnSeries{{
		Timestamps: []int64{1},
		Values:     nil,
	}}); ok {
		t.Fatal("alignedSeriesRows(mismatched first column) ok = true, want false")
	}
	if _, ok := alignedSeriesRows([]model.ColumnSeries{
		{SeriesID: 1, FieldName: "a", Timestamps: []int64{1}, Values: []model.FieldValue{model.Float64Value(1)}},
		{SeriesID: 1, FieldName: "b", Timestamps: []int64{2}, Values: []model.FieldValue{model.Float64Value(2)}},
	}); ok {
		t.Fatal("alignedSeriesRows(mismatched timestamp) ok = true, want false")
	}
	columns := []model.ColumnSeries{
		{SeriesID: 1, Measurement: "cpu", Tags: map[string]string{"host": "a"}, FieldName: "usage", Timestamps: []int64{1}, Values: []model.FieldValue{model.Float64Value(1)}},
		{SeriesID: 1, Measurement: "cpu", Tags: map[string]string{"host": "a"}, FieldName: "load", Timestamps: []int64{2}, Values: []model.FieldValue{model.Float64Value(2)}},
		{SeriesID: 2, Measurement: "cpu", Tags: map[string]string{"host": "b"}, FieldName: "usage", Timestamps: []int64{1}, Values: []model.FieldValue{model.Float64Value(3)}},
	}
	rows := columnsToRows(columns)
	if len(rows) != 3 || rows[0].SeriesID != 1 || rows[0].Timestamp != 1 ||
		rows[1].SeriesID != 1 || rows[1].Timestamp != 2 || rows[2].SeriesID != 2 {
		t.Fatalf("columnsToRows(fallback) = %#v, want sorted rows", rows)
	}
	if got := estimateFieldValueBytes(model.FieldValue{Type: model.FieldType(99)}); got != 0 {
		t.Fatalf("estimateFieldValueBytes(unknown) = %d, want 0", got)
	}

	if got := queryBoundaryMode(model.Query{
		Aggregates: []model.AggregateSpec{{Function: "first"}, {Function: "last"}},
	}); got != model.QueryBoundaryBoth {
		t.Fatalf("queryBoundaryMode(first,last) = %v, want both", got)
	}
	if got := queryBoundaryMode(model.Query{
		Aggregates: []model.AggregateSpec{{Function: "first"}, {Function: "first"}},
	}); got != model.QueryBoundaryFirst {
		t.Fatalf("queryBoundaryMode(first,first) = %v, want first", got)
	}

	mem := memTableStore{inner: memtable.New()}
	if err := mem.Apply(model.ResolvedPoint{
		SeriesID:  99,
		Timestamp: 9,
		Fields: []model.ResolvedField{{
			FieldID: 1,
			Type:    model.FieldFloat64,
			Value:   model.Float64Value(9),
		}},
	}); err != nil {
		t.Fatalf("memTableStore.Apply() error = %v", err)
	}
	if columns := mem.Query(memtable.Query{Start: 0, End: 10}); len(columns) != 1 {
		t.Fatalf("memTableStore.Query() columns = %#v, want one column", columns)
	}
}

func TestQuerySpecColumnStreamCoverage(t *testing.T) {
	ctx := context.Background()
	opts := model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if err := eng.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	}()
	if err := eng.Write(ctx, []model.Point{{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   1,
		Fields:      map[string]model.FieldValue{"usage": model.Float64Value(1)},
	}}, model.WriteOptions{}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	spec, err := querylang.NewBuilder().
		Select("usage").
		From("default", "autogen", "cpu").
		Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	stream, err := eng.QuerySpecColumnStream(ctx, spec)
	if err != nil {
		t.Fatalf("QuerySpecColumnStream() error = %v", err)
	}
	var columns int
	for stream.Next() {
		column := stream.Column()
		if column.FieldName != "usage" || len(column.Values) != 1 {
			closeErr := stream.Close()
			t.Fatalf("stream column = %#v close=%v, want usage value", column, closeErr)
		}
		columns++
	}
	if err := stream.Err(); err != nil {
		closeErr := stream.Close()
		t.Fatalf("stream Err() = %v close = %v", err, closeErr)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("stream Close() error = %v", err)
	}
	if columns != 1 {
		t.Fatalf("streamed columns = %d, want 1", columns)
	}
}

type coverageFakePartReader struct{}

func (coverageFakePartReader) Close() error { return nil }

func (coverageFakePartReader) Meta() sstable.PartMeta { return sstable.PartMeta{} }

func (coverageFakePartReader) Query(sstable.Query) ([]model.ColumnData, error) { return nil, nil }

func (coverageFakePartReader) ScanColumns(sstable.Query) (queryexec.ColumnDataStream, error) {
	return nil, nil
}

func (coverageFakePartReader) QuerySeriesIDs(sstable.Query, []uint64) ([]model.ColumnData, error) {
	return nil, nil
}

func (coverageFakePartReader) SeriesIDs(sstable.Query) ([]uint64, error) { return nil, nil }
