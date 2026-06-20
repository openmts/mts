package queryexec

import (
	"errors"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestCursorEncodeDecodeRoundTrip(t *testing.T) {
	token, err := EncodeCursor(CursorPosition{
		SeriesID:  42,
		Timestamp: 100,
		Direction: model.QuerySortDesc,
	})
	if err != nil {
		t.Fatalf("EncodeCursor() error = %v", err)
	}
	position, err := DecodeCursor(token)
	if err != nil {
		t.Fatalf("DecodeCursor() error = %v", err)
	}
	if position.SeriesID != 42 || position.Timestamp != 100 || position.Direction != model.QuerySortDesc {
		t.Fatalf("position = %#v, want series 42 timestamp 100 desc", position)
	}
	_, err = DecodeCursor("bad")
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("DecodeCursor(bad) error = %v, want ErrInvalidCursor", err)
	}
}

func TestCursorRowStreamResumesAfterDescendingPosition(t *testing.T) {
	source := NewSliceRowStream([]model.Row{
		{SeriesID: 1, Timestamp: 10},
		{SeriesID: 2, Timestamp: 10},
		{SeriesID: 3, Timestamp: 10},
		{SeriesID: 1, Timestamp: 9},
	})
	stream := NewCursorRowStream(source, CursorPosition{
		SeriesID:  2,
		Timestamp: 10,
		Direction: model.QuerySortDesc,
	})
	var got []model.Row
	for stream.Next() {
		got = append(got, stream.Row())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Err() = %v", err)
	}
	if len(got) != 2 || got[0].SeriesID != 3 || got[1].Timestamp != 9 {
		t.Fatalf("rows = %#v, want series 3 at ts 10 then ts 9", got)
	}
}

func TestCursorColumnStreamFiltersSamplesAfterAscendingPosition(t *testing.T) {
	stream := NewCursorColumnStream(
		NewSliceColumnSeriesStream([]model.ColumnSeries{{
			SeriesID:   7,
			Timestamps: []int64{1, 2, 3},
			Values: []model.FieldValue{
				model.Int64Value(1),
				model.Int64Value(2),
				model.Int64Value(3),
			},
		}}),
		CursorPosition{SeriesID: 7, Timestamp: 2, Direction: model.QuerySortAsc},
	)
	if !stream.Next() {
		t.Fatalf("Next() = false err=%v", stream.Err())
	}
	column := stream.Column()
	if len(column.Timestamps) != 1 || column.Timestamps[0] != 3 || column.Values[0].Int64 != 3 {
		t.Fatalf("column = %#v, want only timestamp 3", column)
	}
	if stream.Next() {
		t.Fatal("Next(after one column) = true, want false")
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Err() = %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestCursorHelpersRejectInvalidDirectionAndUseOrderDirection(t *testing.T) {
	_, err := EncodeCursor(CursorPosition{Direction: model.QuerySortDirection(99)})
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("EncodeCursor(invalid direction) error = %v, want ErrInvalidCursor", err)
	}
	ascending := CursorFromRow(model.Row{SeriesID: 2, Timestamp: 11}, model.QueryOrder{})
	if ascending.Direction != model.QuerySortAsc || ascending.SeriesID != 2 || ascending.Timestamp != 11 {
		t.Fatalf("ascending cursor = %#v, want asc row position", ascending)
	}
	descending := CursorFromRow(model.Row{SeriesID: 3, Timestamp: 12}, model.QueryOrder{
		By:        model.QueryOrderByTime,
		Direction: model.QuerySortDesc,
	})
	if descending.Direction != model.QuerySortDesc || descending.SeriesID != 3 || descending.Timestamp != 12 {
		t.Fatalf("descending cursor = %#v, want desc row position", descending)
	}

	rowStream := NewCursorRowStream(nil, CursorPosition{})
	if rowStream.Next() {
		t.Fatal("nil cursor row stream Next() = true, want false")
	}
	if rowStream.Row().Timestamp != 0 || rowStream.Err() != nil {
		t.Fatalf("nil cursor row stream row=%#v err=%v, want zero nil", rowStream.Row(), rowStream.Err())
	}
	if err := rowStream.Close(); err != nil {
		t.Fatalf("nil cursor row stream Close() error = %v", err)
	}

	columnStream := NewCursorColumnStream(nil, CursorPosition{})
	if columnStream.Next() {
		t.Fatal("nil cursor column stream Next() = true, want false")
	}
	if columnStream.Column().FieldName != "" || columnStream.Err() != nil {
		t.Fatalf("nil cursor column stream column=%#v err=%v, want zero nil", columnStream.Column(), columnStream.Err())
	}
	if err := columnStream.Close(); err != nil {
		t.Fatalf("nil cursor column stream Close() error = %v", err)
	}
}
