package queryexec

import (
	"errors"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestProfiledRowStreamRecordsRowsAndDuration(t *testing.T) {
	profile := OperatorProfile{ID: "execute", Kind: "execute"}
	stream := NewProfiledRowStream(
		NewSliceRowStream([]model.Row{{Timestamp: 1}, {Timestamp: 2}}),
		&profile,
	)
	for stream.Next() {
		_ = stream.Row()
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Err() = %v", err)
	}
	if profile.RowsOut != 2 {
		t.Fatalf("RowsOut = %d, want 2", profile.RowsOut)
	}
	if profile.BytesOut <= 0 || profile.StartedUnixNanos <= 0 || profile.FinishedUnixNanos <= 0 {
		t.Fatalf("profile = %#v, want bytes and timestamps", profile)
	}
	if profile.Duration < 0 {
		t.Fatalf("Duration = %v, want non-negative", profile.Duration)
	}
}

func TestProfiledColumnStreamRecordsColumnsSamplesAndErrors(t *testing.T) {
	profile := OperatorProfile{ID: "execute", Kind: "execute"}
	stream := NewProfiledColumnStream(
		NewSliceColumnSeriesStream([]model.ColumnSeries{{
			FieldName: "usage",
			Values: []model.FieldValue{
				model.Float64Value(1),
				model.Float64Value(2),
			},
		}}),
		&profile,
	)
	if !stream.Next() {
		t.Fatalf("Next() = false err=%v", stream.Err())
	}
	if stream.Next() {
		t.Fatal("Next(after column) = true, want false")
	}
	if profile.ColumnsOut != 1 || profile.SamplesOut != 2 {
		t.Fatalf("profile = %#v, want one column and two samples", profile)
	}
	if profile.BytesOut <= 0 || profile.FinishedUnixNanos <= 0 {
		t.Fatalf("profile = %#v, want bytes and finish timestamp", profile)
	}
}

func TestProfiledRowStreamRecordsCloseErrors(t *testing.T) {
	closeErr := errors.New("close failed")
	profile := OperatorProfile{ID: "execute", Kind: "execute"}
	stream := NewProfiledRowStream(failingRowStream{closeErr: closeErr}, &profile)
	if err := stream.Close(); !errors.Is(err, closeErr) {
		t.Fatalf("Close() error = %v, want %v", err, closeErr)
	}
	if profile.Error != closeErr.Error() {
		t.Fatalf("profile error = %q, want %q", profile.Error, closeErr.Error())
	}
}

func TestProfiledColumnStreamRecordsCloseErrorsAndNilSource(t *testing.T) {
	closeErr := errors.New("column close failed")
	profile := OperatorProfile{ID: "execute", Kind: "execute"}
	stream := NewProfiledColumnStream(failingColumnStream{closeErr: closeErr}, &profile)
	if err := stream.Close(); !errors.Is(err, closeErr) {
		t.Fatalf("Close() error = %v, want %v", err, closeErr)
	}
	if profile.Error != closeErr.Error() {
		t.Fatalf("profile error = %q, want %q", profile.Error, closeErr.Error())
	}

	profile = OperatorProfile{ID: "nil"}
	nilProfiled := &profiledColumnStream{profile: &profile}
	if nilProfiled.Next() {
		t.Fatal("nil profiled column Next() = true, want false")
	}
	if got := nilProfiled.Column(); got.FieldName != "" {
		t.Fatalf("nil profiled column = %#v, want zero", got)
	}
	if err := nilProfiled.Err(); err != nil {
		t.Fatalf("nil profiled column Err() = %v, want nil", err)
	}
	if err := nilProfiled.Close(); err != nil {
		t.Fatalf("nil profiled column Close() error = %v", err)
	}
	if profile.FinishedUnixNanos == 0 {
		t.Fatalf("profile = %#v, want finish timestamp", profile)
	}
}

func TestProfileByteEstimatesCoverStringBoolAndUnknownValues(t *testing.T) {
	rowBytes := estimateRowBytes(model.Row{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Fields: map[string]model.FieldValue{
			"state": model.StringValue("ok"),
			"ready": model.BoolValue(true),
			"bad":   {Type: model.FieldType(99)},
		},
	})
	columnBytes := estimateColumnSeriesBytes(model.ColumnSeries{
		Measurement: "cpu",
		FieldName:   "state",
		Tags:        map[string]string{"host": "a"},
		Timestamps:  []int64{1},
		Values: []model.FieldValue{
			model.StringValue("ok"),
			model.BoolValue(false),
			{Type: model.FieldType(99)},
		},
	})
	if rowBytes <= 0 || columnBytes <= 0 {
		t.Fatalf("rowBytes=%d columnBytes=%d, want positive estimates", rowBytes, columnBytes)
	}
}

type failingRowStream struct {
	closeErr error
}

func (f failingRowStream) Next() bool {
	return false
}

func (f failingRowStream) Row() model.Row {
	return model.Row{}
}

func (f failingRowStream) Err() error {
	return nil
}

func (f failingRowStream) Close() error {
	return f.closeErr
}

type failingColumnStream struct {
	closeErr error
}

func (f failingColumnStream) Next() bool {
	return false
}

func (f failingColumnStream) Column() model.ColumnSeries {
	return model.ColumnSeries{}
}

func (f failingColumnStream) Err() error {
	return nil
}

func (f failingColumnStream) Close() error {
	return f.closeErr
}
