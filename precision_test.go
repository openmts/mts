package mts_test

import (
	"context"
	"errors"
	"math"
	"slices"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func TestPrecisionConvertsPointWriteAndQueryRows(t *testing.T) {
	ctx := context.Background()
	eng := openPrecisionEngine(t)
	defer closePrecisionEngine(t, eng)

	const timestampMillis int64 = 1_700_000_000_123
	err := eng.Write(ctx, []mts.Point{{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   timestampMillis,
		Precision:   mts.PrecisionMillisecond,
		Fields:      map[string]mts.FieldValue{"usage": mts.Float64Value(0.7)},
	}}, mts.WriteOptions{})
	if err != nil {
		t.Fatalf("Write(point ms) error = %v", err)
	}

	rows, err := eng.QueryRows(ctx, mts.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		StartTime:   timestampMillis,
		EndTime:     timestampMillis + 1,
		Precision:   mts.PrecisionMillisecond,
	})
	if err != nil {
		t.Fatalf("QueryRows(ms) error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("row count = %d, want 1", len(rows))
	}
	if rows[0].Timestamp != timestampMillis {
		t.Fatalf("row timestamp = %d, want %d", rows[0].Timestamp, timestampMillis)
	}

	rows, err = eng.QueryRows(ctx, mts.Query{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		StartTime:   timestampMillis * int64(time.Millisecond),
		EndTime:     (timestampMillis + 1) * int64(time.Millisecond),
	})
	if err != nil {
		t.Fatalf("QueryRows(default ns) error = %v", err)
	}
	if len(rows) != 1 || rows[0].Timestamp != timestampMillis*int64(time.Millisecond) {
		t.Fatalf("default ns rows = %#v, want timestamp %d", rows, timestampMillis*int64(time.Millisecond))
	}
}

func TestPrecisionConvertsTypedBatchAndQueryColumns(t *testing.T) {
	ctx := context.Background()
	eng := openPrecisionEngine(t)
	defer closePrecisionEngine(t, eng)

	err := eng.WriteTypedBatch(ctx, mts.TypedBatch{
		Measurement: "typed",
		Precision:   mts.PrecisionSecond,
		Tags: []mts.TagColumn{
			{Name: "host", Values: []string{"a", "a"}},
		},
		Timestamps: []int64{10, 11},
		Fields: []mts.TypedFieldColumn{
			{Name: "usage", Type: mts.FieldFloat64, Float64Values: []float64{1.5, 2.5}},
		},
	}, mts.WriteOptions{})
	if err != nil {
		t.Fatalf("WriteTypedBatch(seconds) error = %v", err)
	}

	columns, err := eng.QueryColumns(ctx, mts.Query{
		Measurement: "typed",
		Tags:        map[string]string{"host": "a"},
		StartTime:   10,
		EndTime:     12,
		Precision:   mts.PrecisionSecond,
	})
	if err != nil {
		t.Fatalf("QueryColumns(seconds) error = %v", err)
	}
	if len(columns) != 1 {
		t.Fatalf("column count = %d, want 1", len(columns))
	}
	if !slices.Equal(columns[0].Timestamps, []int64{10, 11}) {
		t.Fatalf("column timestamps = %v, want [10 11]", columns[0].Timestamps)
	}
}

func TestPrecisionConvertsIteratorsAndExplain(t *testing.T) {
	ctx := context.Background()
	eng := openPrecisionEngine(t)
	defer closePrecisionEngine(t, eng)

	err := eng.Write(ctx, []mts.Point{{
		Measurement: "iter",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   42,
		Precision:   mts.PrecisionSecond,
		Fields:      map[string]mts.FieldValue{"usage": mts.Float64Value(4.2)},
	}}, mts.WriteOptions{})
	if err != nil {
		t.Fatalf("Write(point seconds) error = %v", err)
	}
	query := mts.Query{
		Measurement: "iter",
		Tags:        map[string]string{"host": "a"},
		StartTime:   42_000,
		EndTime:     42_001,
		Precision:   mts.PrecisionMillisecond,
	}

	rowIter, err := eng.QueryRowIterator(ctx, query)
	if err != nil {
		t.Fatalf("QueryRowIterator(ms) error = %v", err)
	}
	if !rowIter.Next() {
		err = rowIter.Err()
		closeErr := rowIter.Close()
		t.Fatalf("row iterator Next() = false, err=%v close=%v", err, closeErr)
	}
	if got := rowIter.Row().Timestamp; got != 42_000 {
		closeErr := rowIter.Close()
		t.Fatalf("row iterator timestamp = %d, want 42000 close=%v", got, closeErr)
	}
	if rowIter.Next() {
		closeErr := rowIter.Close()
		t.Fatalf("row iterator returned extra row close=%v", closeErr)
	}
	if err := rowIter.Err(); err != nil {
		closeErr := rowIter.Close()
		t.Fatalf("row iterator Err() = %v close=%v", err, closeErr)
	}
	if err := rowIter.Close(); err != nil {
		t.Fatalf("row iterator Close() error = %v", err)
	}

	columnIter, err := eng.QueryColumnIterator(ctx, query)
	if err != nil {
		t.Fatalf("QueryColumnIterator(ms) error = %v", err)
	}
	if !columnIter.Next() {
		err = columnIter.Err()
		closeErr := columnIter.Close()
		t.Fatalf("column iterator Next() = false, err=%v close=%v", err, closeErr)
	}
	column := columnIter.Column()
	if !slices.Equal(column.Timestamps, []int64{42_000}) {
		closeErr := columnIter.Close()
		t.Fatalf("column timestamps = %v, want [42000] close=%v", column.Timestamps, closeErr)
	}
	if err := columnIter.Close(); err != nil {
		t.Fatalf("column iterator Close() error = %v", err)
	}

	result, err := eng.QueryWithExplain(ctx, query)
	if err != nil {
		t.Fatalf("QueryWithExplain(ms) error = %v", err)
	}
	if len(result.Columns) != 1 || !slices.Equal(result.Columns[0].Timestamps, []int64{42_000}) {
		t.Fatalf("explain columns = %#v, want timestamp 42000", result.Columns)
	}
	if result.Stats.StartedUnixNanos != 0 && result.Stats.StartedUnixNanos < int64(time.Second) {
		t.Fatalf("query stats StartedUnixNanos = %d, want nanosecond clock value", result.Stats.StartedUnixNanos)
	}
}

func TestPrecisionRejectsUnsupportedAndOverflow(t *testing.T) {
	ctx := context.Background()
	eng := openPrecisionEngine(t)
	defer closePrecisionEngine(t, eng)

	err := eng.Write(ctx, []mts.Point{{
		Measurement: "bad",
		Timestamp:   1,
		Precision:   mts.TimePrecision("minute"),
		Fields:      map[string]mts.FieldValue{"usage": mts.Float64Value(1)},
	}}, mts.WriteOptions{})
	if !errors.Is(err, mts.ErrInvalidPrecision) {
		t.Fatalf("Write(invalid precision) error = %v, want ErrInvalidPrecision", err)
	}

	err = eng.Write(ctx, []mts.Point{{
		Measurement: "bad",
		Timestamp:   math.MaxInt64,
		Precision:   mts.PrecisionSecond,
		Fields:      map[string]mts.FieldValue{"usage": mts.Float64Value(1)},
	}}, mts.WriteOptions{})
	if !errors.Is(err, mts.ErrInvalidPrecision) {
		t.Fatalf("Write(overflow precision) error = %v, want ErrInvalidPrecision", err)
	}

	_, err = eng.QueryRows(ctx, mts.Query{
		Measurement: "bad",
		StartTime:   1,
		EndTime:     2,
		Precision:   mts.TimePrecision("minute"),
	})
	if !errors.Is(err, mts.ErrInvalidPrecision) {
		t.Fatalf("QueryRows(invalid precision) error = %v, want ErrInvalidPrecision", err)
	}
}

func openPrecisionEngine(t *testing.T) *mts.Engine {
	t.Helper()
	eng, err := mts.Open(t.Context(), mts.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return eng
}

func closePrecisionEngine(t *testing.T, eng *mts.Engine) {
	t.Helper()
	if err := eng.Close(t.Context()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
