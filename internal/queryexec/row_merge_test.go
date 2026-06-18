package queryexec

import (
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestRowMergeStreamMergesColumnsBySeriesAndTimestamp(t *testing.T) {
	stream := NewRowMergeStream(NewSliceColumnSeriesStream([]model.ColumnSeries{
		rowMergeColumn(1, "cpu", "usage", []int64{1, 2}, []model.FieldValue{
			model.Float64Value(1),
			model.Float64Value(2),
		}),
		rowMergeColumn(1, "cpu", "state", []int64{2}, []model.FieldValue{
			model.StringValue("ok"),
		}),
		rowMergeColumn(2, "cpu", "usage", []int64{1}, []model.FieldValue{
			model.Float64Value(3),
		}),
	}), model.Query{})

	rows := collectRows(t, stream)
	if len(rows) != 3 {
		t.Fatalf("row count = %d, want 3", len(rows))
	}
	if rows[0].SeriesID != 1 || rows[0].Timestamp != 1 || rows[0].Fields["usage"].Float64 != 1 {
		t.Fatalf("first row = %#v, want series 1 timestamp 1 usage 1", rows[0])
	}
	if rows[1].SeriesID != 1 || rows[1].Timestamp != 2 || rows[1].Fields["state"].String != "ok" {
		t.Fatalf("second row = %#v, want merged state at timestamp 2", rows[1])
	}
	if rows[2].SeriesID != 2 || rows[2].Timestamp != 1 || rows[2].Fields["usage"].Float64 != 3 {
		t.Fatalf("third row = %#v, want series 2 timestamp 1 usage 3", rows[2])
	}
}

func TestRowIteratorLimitStopsSourceEarly(t *testing.T) {
	source := &recordingColumnStream{columns: []model.ColumnSeries{
		rowMergeColumn(1, "cpu", "usage", []int64{1}, []model.FieldValue{model.Float64Value(1)}),
		rowMergeColumn(2, "cpu", "usage", []int64{1}, []model.FieldValue{model.Float64Value(2)}),
		rowMergeColumn(2, "cpu", "state", []int64{1}, []model.FieldValue{model.StringValue("ok")}),
	}}
	stream := NewRowMergeStream(source, model.Query{Limit: 1})

	if !stream.Next() {
		t.Fatalf("Next() = false, want first row err=%v", stream.Err())
	}
	if got := stream.Row(); got.SeriesID != 1 {
		t.Fatalf("first row series = %d, want 1", got.SeriesID)
	}
	if source.closeCalls != 1 {
		t.Fatalf("source close calls after limit row = %d, want 1", source.closeCalls)
	}
	if source.nextCalls >= len(source.columns) {
		t.Fatalf("source Next calls = %d, want less than all columns", source.nextCalls)
	}
	if stream.Next() {
		t.Fatal("Next(after limit) = true, want false")
	}
}

func TestConcurrentRowIteratorsDoNotShareMutableRows(t *testing.T) {
	first := NewRowMergeStream(NewSliceColumnSeriesStream([]model.ColumnSeries{
		rowMergeColumn(1, "cpu", "usage", []int64{1}, []model.FieldValue{model.Float64Value(1)}),
	}), model.Query{})
	second := NewRowMergeStream(NewSliceColumnSeriesStream([]model.ColumnSeries{
		rowMergeColumn(1, "cpu", "usage", []int64{1}, []model.FieldValue{model.Float64Value(1)}),
	}), model.Query{})

	if !first.Next() || !second.Next() {
		t.Fatalf("Next() = false, first err=%v second err=%v", first.Err(), second.Err())
	}
	firstRow := first.Row()
	secondRow := second.Row()
	firstRow.Fields["usage"] = model.Float64Value(99)
	if got := secondRow.Fields["usage"].Float64; got != 1 {
		t.Fatalf("second row usage = %v, want unaffected value 1", got)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func collectRows(t *testing.T, stream RowStream) []model.Row {
	t.Helper()
	rows := make([]model.Row, 0)
	for stream.Next() {
		rows = append(rows, stream.Row())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream Err() = %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("stream Close() error = %v", err)
	}
	return rows
}

func rowMergeColumn(
	seriesID uint64,
	measurement string,
	fieldName string,
	timestamps []int64,
	values []model.FieldValue,
) model.ColumnSeries {
	return model.ColumnSeries{
		SeriesID:    seriesID,
		Measurement: measurement,
		FieldName:   fieldName,
		Timestamps:  timestamps,
		Values:      values,
	}
}
