package queryexec

import (
	"math"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestAggregateColumnStreamAggregatesNumericColumns(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{{
		SeriesID:    1,
		FieldName:   "value",
		FieldType:   model.FieldFloat64,
		Timestamps:  []int64{1, 2, 3},
		Values:      []model.FieldValue{model.Float64Value(1), model.Float64Value(2), model.Float64Value(3)},
		Measurement: "cpu",
	}})
	stream := NewAggregateColumnStream(source, []model.AggregateSpec{
		{Field: "value", Function: "sum"},
		{Field: "value", Function: "avg"},
	})

	if !stream.Next() {
		t.Fatal("Next(sum) = false, want true")
	}
	sum := stream.Column()
	if sum.FieldName != "sum(value)" || sum.Values[0].Float64 != 6 {
		t.Fatalf("sum column = %#v, want sum(value)=6", sum)
	}
	if !stream.Next() {
		t.Fatal("Next(avg) = false, want true")
	}
	avg := stream.Column()
	if avg.FieldName != "avg(value)" || avg.Values[0].Float64 != 2 {
		t.Fatalf("avg column = %#v, want avg(value)=2", avg)
	}
	if stream.Next() {
		t.Fatal("Next(after aggregates) = true, want false")
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
}

func TestAggregateColumnStreamWindowsAreLeftClosedRightOpen(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{{
		SeriesID:   1,
		FieldName:  "value",
		FieldType:  model.FieldInt64,
		Timestamps: []int64{0, int64(time.Second) - 1, int64(time.Second)},
		Values: []model.FieldValue{
			model.Int64Value(1),
			model.Int64Value(2),
			model.Int64Value(3),
		},
	}})
	stream := NewAggregateColumnStream(
		source,
		[]model.AggregateSpec{{Field: "value", Function: "count"}},
		time.Second,
	)

	if !stream.Next() {
		t.Fatal("Next() = false, want true")
	}
	column := stream.Column()
	if len(column.Values) != 2 {
		t.Fatalf("window count = %d, want 2", len(column.Values))
	}
	if column.Values[0].Int64 != 2 || column.Values[1].Int64 != 1 {
		t.Fatalf("counts = %#v, want 2 and 1", column.Values)
	}
}

func TestAggregateColumnStreamSupportsMinMaxFirstLastAndErrors(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{
		{
			SeriesID:   1,
			FieldName:  "value",
			FieldType:  model.FieldInt64,
			Timestamps: []int64{1, 2, 3},
			Values: []model.FieldValue{
				model.Int64Value(3),
				model.Int64Value(1),
				model.Int64Value(2),
			},
		},
		{
			SeriesID:   1,
			FieldName:  "name",
			FieldType:  model.FieldString,
			Timestamps: []int64{1},
			Values:     []model.FieldValue{model.StringValue("a")},
		},
	})
	stream := NewAggregateColumnStream(source, []model.AggregateSpec{
		{Field: "value", Function: "min"},
		{Field: "value", Function: "max"},
		{Field: "value", Function: "first"},
		{Field: "value", Function: "last"},
		{Field: "name", Function: "sum"},
	})

	want := []int64{1, 3, 3, 2}
	for index, expected := range want {
		if !stream.Next() {
			t.Fatalf("Next(%d) = false, want true err=%v", index, stream.Err())
		}
		if got := stream.Column().Values[0].Int64; got != expected {
			t.Fatalf("aggregate %d = %d, want %d", index, got, expected)
		}
	}
	if stream.Next() {
		t.Fatal("Next(error aggregate) = true, want false")
	}
	if stream.Err() == nil {
		t.Fatal("Err() = nil, want unsupported aggregate error")
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestAggregateColumnStreamFloatMinMaxAndInvalidFunction(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{
		{
			SeriesID:   1,
			FieldName:  "f",
			FieldType:  model.FieldFloat64,
			Timestamps: []int64{1, 2},
			Values: []model.FieldValue{
				model.Float64Value(9),
				model.Float64Value(4),
			},
		},
		{
			SeriesID:   1,
			FieldName:  "b",
			FieldType:  model.FieldBool,
			Timestamps: []int64{1},
			Values:     []model.FieldValue{model.BoolValue(true)},
		},
	})
	stream := NewAggregateColumnStream(source, []model.AggregateSpec{
		{Field: "f", Function: "min"},
		{Field: "f", Function: "max"},
		{Field: "b", Function: "avg"},
	})
	if !stream.Next() {
		t.Fatalf("Next(min) = false err=%v", stream.Err())
	}
	if got := stream.Column().Values[0].Float64; got != 4 {
		t.Fatalf("min = %v, want 4", got)
	}
	if !stream.Next() {
		t.Fatalf("Next(max) = false err=%v", stream.Err())
	}
	if got := stream.Column().Values[0].Float64; got != 9 {
		t.Fatalf("max = %v, want 9", got)
	}
	if stream.Next() {
		t.Fatal("Next(bool avg) = true, want false")
	}
	if stream.Err() == nil {
		t.Fatal("Err() = nil, want bool avg error")
	}
}

func TestAggregateColumnStreamRejectsEmptyFunction(t *testing.T) {
	stream := NewAggregateColumnStream(
		NewSliceColumnSeriesStream([]model.ColumnSeries{{
			FieldName:  "v",
			Timestamps: []int64{1},
			Values:     []model.FieldValue{model.Int64Value(1)},
		}}),
		[]model.AggregateSpec{{Field: "v"}},
	)
	if stream.Next() {
		t.Fatal("Next(empty function) = true, want false")
	}
	if stream.Err() == nil {
		t.Fatal("Err() = nil, want empty function error")
	}
}

func TestAggregateColumnStreamSupportsDifferenceAndDerivative(t *testing.T) {
	stream := NewAggregateColumnStream(
		NewSliceColumnSeriesStream([]model.ColumnSeries{{
			FieldName:  "value",
			FieldType:  model.FieldFloat64,
			Timestamps: []int64{0, int64(time.Second), int64(3 * time.Second)},
			Values: []model.FieldValue{
				model.Float64Value(1),
				model.Float64Value(4),
				model.Float64Value(10),
			},
		}}),
		[]model.AggregateSpec{
			{Field: "value", Function: "difference"},
			{Field: "value", Function: "derivative"},
		},
	)

	if !stream.Next() {
		t.Fatalf("Next(difference) = false err=%v", stream.Err())
	}
	difference := stream.Column()
	if len(difference.Values) != 2 {
		t.Fatalf("difference values = %d, want 2", len(difference.Values))
	}
	if difference.Timestamps[0] != int64(time.Second) ||
		difference.Values[0].Float64 != 3 ||
		difference.Values[1].Float64 != 6 {
		t.Fatalf("difference = %#v, want [3,6]", difference)
	}
	if !stream.Next() {
		t.Fatalf("Next(derivative) = false err=%v", stream.Err())
	}
	derivative := stream.Column()
	if derivative.Values[0].Float64 != 3 || derivative.Values[1].Float64 != 3 {
		t.Fatalf("derivative = %#v, want [3,3]", derivative)
	}
}

func TestAggregateColumnStreamSupportsRateAndIRateWithReset(t *testing.T) {
	stream := NewAggregateColumnStream(
		NewSliceColumnSeriesStream([]model.ColumnSeries{{
			FieldName:  "counter",
			FieldType:  model.FieldFloat64,
			Timestamps: []int64{0, int64(time.Second), int64(2 * time.Second), int64(3 * time.Second)},
			Values: []model.FieldValue{
				model.Float64Value(10),
				model.Float64Value(15),
				model.Float64Value(3),
				model.Float64Value(8),
			},
		}}),
		[]model.AggregateSpec{
			{Field: "counter", Function: "rate"},
			{Field: "counter", Function: "irate"},
		},
	)

	if !stream.Next() {
		t.Fatalf("Next(rate) = false err=%v", stream.Err())
	}
	rate := stream.Column().Values[0].Float64
	if math.Abs(rate-(13.0/3.0)) > 0.000001 {
		t.Fatalf("rate = %v, want %v", rate, 13.0/3.0)
	}
	if !stream.Next() {
		t.Fatalf("Next(irate) = false err=%v", stream.Err())
	}
	if got := stream.Column().Values[0].Float64; got != 5 {
		t.Fatalf("irate = %v, want 5", got)
	}
}

func TestAggregateColumnStreamSupportsIncreaseDeltaTopBottom(t *testing.T) {
	stream := NewAggregateColumnStream(
		NewSliceColumnSeriesStream([]model.ColumnSeries{{
			FieldName:  "counter",
			FieldType:  model.FieldInt64,
			Timestamps: []int64{0, int64(time.Second), int64(2 * time.Second)},
			Values: []model.FieldValue{
				model.Int64Value(10),
				model.Int64Value(3),
				model.Int64Value(8),
			},
		}}),
		[]model.AggregateSpec{
			{Field: "counter", Function: "increase"},
			{Field: "counter", Function: "delta"},
			{Field: "counter", Function: "top"},
			{Field: "counter", Function: "bottom"},
		},
	)

	if !stream.Next() {
		t.Fatalf("Next(increase) = false err=%v", stream.Err())
	}
	if got := stream.Column().Values[0].Float64; got != 8 {
		t.Fatalf("increase = %v, want 8", got)
	}
	if !stream.Next() {
		t.Fatalf("Next(delta) = false err=%v", stream.Err())
	}
	if got := stream.Column().Values[0].Int64; got != -2 {
		t.Fatalf("delta = %v, want -2", got)
	}
	if !stream.Next() {
		t.Fatalf("Next(top) = false err=%v", stream.Err())
	}
	if got := stream.Column().Values[0].Int64; got != 10 {
		t.Fatalf("top = %v, want 10", got)
	}
	if !stream.Next() {
		t.Fatalf("Next(bottom) = false err=%v", stream.Err())
	}
	if got := stream.Column().Values[0].Int64; got != 3 {
		t.Fatalf("bottom = %v, want 3", got)
	}
}

func TestAggregateColumnStreamSupportsDistributionFunctions(t *testing.T) {
	stream := NewAggregateColumnStream(
		NewSliceColumnSeriesStream([]model.ColumnSeries{{
			FieldName:  "value",
			FieldType:  model.FieldFloat64,
			Timestamps: []int64{1, 2, 3, 4},
			Values: []model.FieldValue{
				model.Float64Value(1),
				model.Float64Value(2),
				model.Float64Value(2),
				model.Float64Value(5),
			},
		}}),
		[]model.AggregateSpec{
			{Field: "value", Function: "spread"},
			{Field: "value", Function: "median"},
			{Field: "value", Function: "mode"},
			{Field: "value", Function: "stdvar"},
			{Field: "value", Function: "stddev"},
		},
	)

	want := []float64{4, 2, 2, 2.25, 1.5}
	for index, expected := range want {
		if !stream.Next() {
			t.Fatalf("Next(%d) = false err=%v", index, stream.Err())
		}
		if got := stream.Column().Values[0].Float64; math.Abs(got-expected) > 0.000001 {
			t.Fatalf("aggregate %d = %v, want %v", index, got, expected)
		}
	}
}
