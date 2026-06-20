package queryexec

import (
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestOrderedColumnStreamReversesTimeDescending(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{{
		FieldName:  "usage",
		Timestamps: []int64{1, 2, 3},
		Values: []model.FieldValue{
			model.Float64Value(1),
			model.Float64Value(2),
			model.Float64Value(3),
		},
	}})
	stream := NewOrderedColumnStream(source, model.QueryOrder{
		By:        model.QueryOrderByTime,
		Direction: model.QuerySortDesc,
	})
	if !stream.Next() {
		t.Fatalf("Next() = false, err = %v", stream.Err())
	}
	column := stream.Column()
	if got := column.Timestamps; len(got) != 3 || got[0] != 3 || got[2] != 1 {
		t.Fatalf("timestamps = %v, want descending", got)
	}
}

func TestOrderedRowStreamSortsTimeDescending(t *testing.T) {
	source := NewSliceRowStream([]model.Row{
		{SeriesID: 2, Timestamp: 3},
		{SeriesID: 1, Timestamp: 3},
		{SeriesID: 1, Timestamp: 2},
	})
	stream := NewOrderedRowStream(source, model.QueryOrder{
		By:        model.QueryOrderByTime,
		Direction: model.QuerySortDesc,
	})
	var timestamps []int64
	for stream.Next() {
		timestamps = append(timestamps, stream.Row().Timestamp)
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Err() = %v", err)
	}
	if len(timestamps) != 3 || timestamps[0] != 3 || timestamps[2] != 2 {
		t.Fatalf("timestamps = %v, want [3 3 2]", timestamps)
	}
	if first := stream.(*orderedRowStream).rows[0]; first.SeriesID != 1 {
		t.Fatalf("first row = %#v, want lower series id for equal timestamp", first)
	}
}

func TestOrderedRowStreamUsesBoundedBufferForDescendingLimit(t *testing.T) {
	rows := make([]model.Row, 0, 10)
	for timestamp := int64(1); timestamp <= 10; timestamp++ {
		rows = append(rows, model.Row{Timestamp: timestamp})
	}
	stream := NewOrderedRowStream(
		NewSliceRowStream(rows),
		model.QueryOrder{By: model.QueryOrderByTime, Direction: model.QuerySortDesc},
		2,
		1,
	)
	ordered, ok := stream.(*orderedRowStream)
	if !ok {
		t.Fatalf("stream type = %T, want *orderedRowStream", stream)
	}
	if !stream.Next() {
		t.Fatalf("Next() = false err=%v", stream.Err())
	}
	if buffered := len(ordered.rows); buffered != 3 {
		t.Fatalf("buffered rows = %d, want 3", buffered)
	}
	got := []int64{stream.Row().Timestamp}
	for stream.Next() {
		got = append(got, stream.Row().Timestamp)
	}
	want := []int64{10, 9, 8}
	if len(got) != len(want) {
		t.Fatalf("rows = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("row %d = %d, want %d", index, got[index], want[index])
		}
	}
}

func TestOrderedStreamsPassthroughForAscending(t *testing.T) {
	columnSource := NewSliceColumnSeriesStream([]model.ColumnSeries{{Timestamps: []int64{1}}})
	if got := NewOrderedColumnStream(columnSource, model.QueryOrder{
		By:        model.QueryOrderByTime,
		Direction: model.QuerySortAsc,
	}); got != columnSource {
		t.Fatal("ascending column order should return source")
	}
	rowSource := NewSliceRowStream([]model.Row{{Timestamp: 1}})
	if got := NewOrderedRowStream(rowSource, model.QueryOrder{
		By:        model.QueryOrderByTime,
		Direction: model.QuerySortAsc,
	}); got != rowSource {
		t.Fatal("ascending row order should return source")
	}
}

func TestOrderedStreamsExposeErrCloseAndHeapPop(t *testing.T) {
	column := &orderedColumnStream{}
	if column.Next() || column.Column().FieldName != "" || column.Err() != nil {
		t.Fatalf("nil ordered column next/column/err = unexpected")
	}
	if err := column.Close(); err != nil {
		t.Fatalf("nil ordered column Close() error = %v", err)
	}

	row := &orderedRowStream{}
	if row.Next() || row.Row().Timestamp != 0 || row.Err() != nil {
		t.Fatalf("nil ordered row next/row/err = unexpected")
	}
	if err := row.Close(); err != nil {
		t.Fatalf("nil ordered row Close() error = %v", err)
	}

	heapRows := rowMinHeap{{Timestamp: 1}, {Timestamp: 2}}
	popped := heapRows.Pop().(model.Row)
	if popped.Timestamp != 2 || len(heapRows) != 1 {
		t.Fatalf("popped row = %#v len=%d, want timestamp 2 and len 1", popped, len(heapRows))
	}
	if !rowBeforeTimeDesc(model.Row{SeriesID: 2, Timestamp: 1}, model.Row{SeriesID: 1, Timestamp: 1}) {
		t.Fatal("rowBeforeTimeDesc tie breaker = false, want higher series first in min heap")
	}
}
