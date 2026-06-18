package queryexec

import (
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestMergeColumnDataStreamsKeepsNewestSequence(t *testing.T) {
	first := NewSliceColumnDataStream([]model.ColumnData{
		columnDataForMergeTest(1, 1, 10, 1),
		columnDataForMergeTest(2, 1, 10, 1),
	})
	second := NewSliceColumnDataStream([]model.ColumnData{
		columnDataForMergeTest(1, 1, 10, 3),
		columnDataForMergeTest(1, 2, 10, 2),
	})
	stream := MergeColumnDataStreams(first, second)

	got := collectColumnDataStream(t, stream)
	if len(got) != 3 {
		t.Fatalf("column count = %d, want 3", len(got))
	}
	if got[0].SeriesID != 1 || got[0].FieldID != 1 {
		t.Fatalf("first column key = (%d,%d), want (1,1)", got[0].SeriesID, got[0].FieldID)
	}
	if got[0].Samples[0].WriteSeq != 3 {
		t.Fatalf("merged write seq = %d, want 3", got[0].Samples[0].WriteSeq)
	}
	if got[1].SeriesID != 1 || got[1].FieldID != 2 {
		t.Fatalf("second column key = (%d,%d), want (1,2)", got[1].SeriesID, got[1].FieldID)
	}
	if got[2].SeriesID != 2 || got[2].FieldID != 1 {
		t.Fatalf("third column key = (%d,%d), want (2,1)", got[2].SeriesID, got[2].FieldID)
	}
}

func collectColumnDataStream(t *testing.T, stream ColumnDataStream) []model.ColumnData {
	t.Helper()
	columns := make([]model.ColumnData, 0)
	for stream.Next() {
		columns = append(columns, stream.ColumnData())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream Err() = %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("stream Close() error = %v", err)
	}
	return columns
}

func columnDataForMergeTest(
	seriesID uint64,
	fieldID uint32,
	timestamp int64,
	writeSeq uint64,
) model.ColumnData {
	return model.ColumnData{
		SeriesID:  seriesID,
		FieldID:   fieldID,
		FieldType: model.FieldFloat64,
		Samples: []model.VersionedSample{
			{
				Timestamp: timestamp,
				WriteSeq:  writeSeq,
				Value:     model.Float64Value(float64(writeSeq)),
			},
		},
	}
}
