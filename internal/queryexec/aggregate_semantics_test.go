package queryexec

import (
	"math"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestFirstLastAggregatesUseBoundaryTimestamps(t *testing.T) {
	stream := NewAggregateColumnStream(
		NewSliceColumnSeriesStream([]model.ColumnSeries{{
			SeriesID:   1,
			FieldName:  "value",
			FieldType:  model.FieldInt64,
			Timestamps: []int64{30, 10, 20},
			Values: []model.FieldValue{
				model.Int64Value(3),
				model.Int64Value(1),
				model.Int64Value(2),
			},
		}}),
		[]model.AggregateSpec{
			{Field: "value", Function: "first"},
			{Field: "value", Function: "last"},
		},
	)

	if !stream.Next() {
		t.Fatalf("Next(first) = false err=%v", stream.Err())
	}
	first := stream.Column()
	if first.Timestamps[0] != 10 || first.Values[0].Int64 != 1 {
		t.Fatalf("first aggregate = %#v, want timestamp 10 value 1", first)
	}
	if !stream.Next() {
		t.Fatalf("Next(last) = false err=%v", stream.Err())
	}
	last := stream.Column()
	if last.Timestamps[0] != 30 || last.Values[0].Int64 != 3 {
		t.Fatalf("last aggregate = %#v, want timestamp 30 value 3", last)
	}
}

func TestAggregateWindowMergesAcrossShardAndPartBoundaries(t *testing.T) {
	stream := NewAggregateColumnStream(
		NewSliceColumnSeriesStream([]model.ColumnSeries{{
			SeriesID:  1,
			FieldName: "value",
			FieldType: model.FieldFloat64,
			Timestamps: []int64{
				int64(1500 * time.Millisecond),
				int64(500 * time.Millisecond),
				int64(2500 * time.Millisecond),
			},
			Values: []model.FieldValue{
				model.Float64Value(2),
				model.Float64Value(1),
				model.Float64Value(3),
			},
		}}),
		[]model.AggregateSpec{{Field: "value", Function: "sum"}},
		time.Second,
	)

	if !stream.Next() {
		t.Fatalf("Next(sum) = false err=%v", stream.Err())
	}
	column := stream.Column()
	wantTimes := []int64{0, int64(time.Second), int64(2 * time.Second)}
	wantValues := []float64{1, 2, 3}
	if len(column.Timestamps) != len(wantTimes) {
		t.Fatalf("window count = %d, want %d column=%#v", len(column.Timestamps), len(wantTimes), column)
	}
	for index := range wantTimes {
		if column.Timestamps[index] != wantTimes[index] || column.Values[index].Float64 != wantValues[index] {
			t.Fatalf("window %d = (%d,%v), want (%d,%v)", index, column.Timestamps[index], column.Values[index].Float64, wantTimes[index], wantValues[index])
		}
	}
}

func TestTimeSeriesAggregatesCoverRateDeltaIncreaseAndSpread(t *testing.T) {
	stream := NewAggregateColumnStream(
		NewSliceColumnSeriesStream([]model.ColumnSeries{{
			SeriesID:   1,
			FieldName:  "counter",
			FieldType:  model.FieldFloat64,
			Timestamps: []int64{0, int64(time.Second), int64(2 * time.Second), int64(3 * time.Second)},
			Values: []model.FieldValue{
				model.Float64Value(1),
				model.Float64Value(4),
				model.Float64Value(2),
				model.Float64Value(8),
			},
		}}),
		[]model.AggregateSpec{
			{Field: "counter", Function: "rate"},
			{Field: "counter", Function: "irate"},
			{Field: "counter", Function: "increase"},
			{Field: "counter", Function: "delta"},
			{Field: "counter", Function: "difference"},
			{Field: "counter", Function: "spread"},
		},
	)
	want := map[string]float64{
		"rate(counter)":       11.0 / 3.0,
		"irate(counter)":      6,
		"increase(counter)":   11,
		"delta(counter)":      7,
		"difference(counter)": 3,
		"spread(counter)":     7,
	}
	for range want {
		if !stream.Next() {
			t.Fatalf("Next() = false err=%v", stream.Err())
		}
		column := stream.Column()
		expected, ok := want[column.FieldName]
		if !ok {
			t.Fatalf("field = %q, want one of %#v", column.FieldName, want)
		}
		if got := column.Values[0].Float64; math.Abs(got-expected) > 0.000001 {
			t.Fatalf("%s = %v, want %v", column.FieldName, got, expected)
		}
	}
	if stream.Next() {
		t.Fatal("Next(after timeseries aggregates) = true, want false")
	}
}

func TestModeAggregateHandlesStringBoolIntAndFloatKeys(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value model.FieldValue
	}{
		{name: "string", value: model.StringValue("ok")},
		{name: "bool", value: model.BoolValue(true)},
		{name: "int", value: model.Int64Value(7)},
		{name: "float", value: model.Float64Value(1.5)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stream := NewAggregateColumnStream(
				NewSliceColumnSeriesStream([]model.ColumnSeries{{
					SeriesID:   1,
					FieldName:  "value",
					FieldType:  tt.value.Type,
					Timestamps: []int64{1, 2, 3},
					Values:     []model.FieldValue{tt.value, tt.value, tt.value},
				}}),
				[]model.AggregateSpec{{Field: "value", Function: "mode"}},
			)
			if !stream.Next() {
				t.Fatalf("Next() = false err=%v", stream.Err())
			}
			if fieldValuesCompare(stream.Column().Values[0], tt.value) != 0 {
				t.Fatalf("mode value = %#v, want %#v", stream.Column().Values[0], tt.value)
			}
		})
	}
}

func TestAggregateErrorBranchesAndNumericHelpers(t *testing.T) {
	if _, err := aggregateValues(nil, "sum"); err == nil {
		t.Fatal("aggregateValues(empty) error = nil, want error")
	}
	if _, err := aggregateValues([]model.FieldValue{model.StringValue("bad")}, "sum"); err == nil {
		t.Fatal("aggregateValues(sum string) error = nil, want error")
	}
	if _, err := aggregateValues([]model.FieldValue{model.BoolValue(true)}, "min"); err == nil {
		t.Fatal("aggregateValues(min bool) error = nil, want error")
	}
	if _, err := aggregateValues([]model.FieldValue{model.Int64Value(1)}, "missing"); err == nil {
		t.Fatal("aggregateValues(unsupported) error = nil, want error")
	}
	if got := aggregateTimestamp(model.ColumnSeries{}, "last"); got != 0 {
		t.Fatalf("aggregateTimestamp(empty) = %d, want 0", got)
	}
	if got := aggregateTimestamp(model.ColumnSeries{Timestamps: []int64{1, 2}}, "last"); got != 2 {
		t.Fatalf("aggregateTimestamp(last) = %d, want 2", got)
	}

	state := newIncrementalAggregateState("spread")
	if _, _, err := state.value(); err == nil {
		t.Fatal("empty incremental value error = nil, want error")
	}
	if err := state.add(1, model.StringValue("ok")); err == nil {
		t.Fatal("spread string add error = nil, want numeric error")
	}

	state = newIncrementalAggregateState("spread")
	if err := state.add(1, model.Int64Value(2)); err != nil {
		t.Fatalf("state add first error = %v", err)
	}
	if err := state.add(2, model.Int64Value(7)); err != nil {
		t.Fatalf("state add second error = %v", err)
	}
	value, _, err := state.value()
	if err != nil {
		t.Fatalf("state value error = %v", err)
	}
	if value.Type != model.FieldInt64 || value.Int64 != 5 {
		t.Fatalf("spread value = %#v, want int64 5", value)
	}
}

func TestTimeSeriesAggregateErrorBranches(t *testing.T) {
	if _, err := aggregateRate([]model.FieldValue{model.Float64Value(1)}, nil); err == nil {
		t.Fatal("aggregateRate(single value) error = nil, want error")
	}
	if _, err := aggregateRate(
		[]model.FieldValue{model.Float64Value(1), model.Float64Value(2)},
		[]int64{2, 1},
	); err == nil {
		t.Fatal("aggregateRate(non-increasing timestamps) error = nil, want error")
	}
	if _, err := aggregateIRate([]model.FieldValue{model.Float64Value(1)}, nil); err == nil {
		t.Fatal("aggregateIRate(single value) error = nil, want error")
	}
	if _, err := aggregateIRate(
		[]model.FieldValue{model.Float64Value(1), model.Float64Value(2)},
		[]int64{2, 1},
	); err == nil {
		t.Fatal("aggregateIRate(non-increasing timestamps) error = nil, want error")
	}
	if _, err := aggregateIncrease([]model.FieldValue{model.StringValue("bad"), model.StringValue("worse")}); err == nil {
		t.Fatal("aggregateIncrease(non numeric) error = nil, want error")
	}
	if _, err := aggregateDelta([]model.FieldValue{model.StringValue("bad"), model.StringValue("worse")}); err == nil {
		t.Fatal("aggregateDelta(non numeric) error = nil, want error")
	}
	if value, ok, err := transformValue(
		model.Float64Value(2),
		model.Float64Value(3),
		2,
		1,
		"derivative",
	); err != nil || ok || value.Type != 0 {
		t.Fatalf("transformValue(non-increasing derivative) = %#v %v %v, want zero false nil", value, ok, err)
	}
}
