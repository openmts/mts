package queryexec

import (
	"math"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

func TestGroupAggregateColumnStreamMergesSeriesByTag(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{
		{
			SeriesID:    1,
			Measurement: "cpu",
			Tags:        map[string]string{"region": "east", "host": "a"},
			FieldName:   "usage",
			FieldType:   model.FieldFloat64,
			Timestamps:  []int64{1, 2},
			Values:      []model.FieldValue{model.Float64Value(1), model.Float64Value(2)},
		},
		{
			SeriesID:    2,
			Measurement: "cpu",
			Tags:        map[string]string{"region": "east", "host": "b"},
			FieldName:   "usage",
			FieldType:   model.FieldFloat64,
			Timestamps:  []int64{1, 3},
			Values:      []model.FieldValue{model.Float64Value(3), model.Float64Value(4)},
		},
	})
	stream := NewGroupAggregateColumnStream(
		source,
		[]model.AggregateSpec{{Field: "usage", Function: "sum"}},
		model.QueryGroup{Tags: []string{"region"}},
		0,
	)

	if !stream.Next() {
		t.Fatalf("Next() = false err=%v", stream.Err())
	}
	column := stream.Column()
	if column.Tags["region"] != "east" || len(column.Tags) != 1 {
		t.Fatalf("tags = %#v, want only region=east", column.Tags)
	}
	if column.FieldName != "sum(usage)" {
		t.Fatalf("field = %q, want sum(usage)", column.FieldName)
	}
	if got := column.Values[0].Float64; got != 10 {
		t.Fatalf("sum = %v, want 10", got)
	}
	if stream.Next() {
		t.Fatal("Next(after group) = true, want false")
	}
}

func TestGroupAggregateColumnStreamUsesIncrementalSelectorAndNumericState(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{
		{
			SeriesID:    1,
			Measurement: "cpu",
			Tags:        map[string]string{"region": "east"},
			FieldName:   "usage",
			FieldType:   model.FieldInt64,
			Timestamps:  []int64{3, 1},
			Values:      []model.FieldValue{model.Int64Value(7), model.Int64Value(2)},
		},
		{
			SeriesID:    2,
			Measurement: "cpu",
			Tags:        map[string]string{"region": "east"},
			FieldName:   "usage",
			FieldType:   model.FieldInt64,
			Timestamps:  []int64{2, 4},
			Values:      []model.FieldValue{model.Int64Value(5), model.Int64Value(9)},
		},
	})
	stream := NewGroupAggregateColumnStream(
		source,
		[]model.AggregateSpec{
			{Field: "usage", Function: "count"},
			{Field: "usage", Function: "sum"},
			{Field: "usage", Function: "avg"},
			{Field: "usage", Function: "min"},
			{Field: "usage", Function: "max"},
			{Field: "usage", Function: "first"},
			{Field: "usage", Function: "last"},
		},
		model.QueryGroup{Tags: []string{"region"}},
		0,
	)
	want := map[string]model.FieldValue{
		"count(usage)": model.Int64Value(4),
		"sum(usage)":   model.Int64Value(23),
		"avg(usage)":   model.Float64Value(5.75),
		"min(usage)":   model.Int64Value(2),
		"max(usage)":   model.Int64Value(9),
		"first(usage)": model.Int64Value(2),
		"last(usage)":  model.Int64Value(9),
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
		if fieldValuesCompare(column.Values[0], expected) != 0 {
			t.Fatalf("%s = %#v, want %#v", column.FieldName, column.Values[0], expected)
		}
	}
	if stream.Next() {
		t.Fatal("Next(after aggregates) = true, want false")
	}
}

func TestGroupAggregateColumnStreamUsesIncrementalDistributionState(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{
		{
			SeriesID:    1,
			Measurement: "cpu",
			Tags:        map[string]string{"region": "east"},
			FieldName:   "usage",
			FieldType:   model.FieldFloat64,
			Timestamps:  []int64{1, 2},
			Values:      []model.FieldValue{model.Float64Value(1), model.Float64Value(2)},
		},
		{
			SeriesID:    2,
			Measurement: "cpu",
			Tags:        map[string]string{"region": "east"},
			FieldName:   "usage",
			FieldType:   model.FieldFloat64,
			Timestamps:  []int64{3, 4},
			Values:      []model.FieldValue{model.Float64Value(2), model.Float64Value(5)},
		},
	})
	stream := NewGroupAggregateColumnStream(
		source,
		[]model.AggregateSpec{
			{Field: "usage", Function: "spread"},
			{Field: "usage", Function: "mode"},
			{Field: "usage", Function: "stdvar"},
			{Field: "usage", Function: "stddev"},
		},
		model.QueryGroup{Tags: []string{"region"}},
		0,
	)
	want := map[string]float64{
		"spread(usage)": 4,
		"mode(usage)":   2,
		"stdvar(usage)": 2.25,
		"stddev(usage)": 1.5,
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
		t.Fatal("Next(after aggregates) = true, want false")
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestGroupAggregateColumnStreamReportsIncrementalStateErrors(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{{
		SeriesID:    1,
		Measurement: "cpu",
		Tags:        map[string]string{"region": "east"},
		FieldName:   "state",
		FieldType:   model.FieldString,
		Timestamps:  []int64{1},
		Values:      []model.FieldValue{model.StringValue("ok")},
	}})
	stream := NewGroupAggregateColumnStream(
		source,
		[]model.AggregateSpec{{Field: "state", Function: "sum"}},
		model.QueryGroup{Tags: []string{"region"}},
		0,
	)
	if stream.Next() {
		t.Fatal("Next() = true, want false for invalid sum")
	}
	if stream.Err() == nil {
		t.Fatal("Err() = nil, want numeric aggregate error")
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestGroupAggregateColumnStreamKeepsMedianOnPointFallback(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{
		{
			SeriesID:    1,
			Measurement: "cpu",
			Tags:        map[string]string{"region": "east"},
			FieldName:   "usage",
			FieldType:   model.FieldFloat64,
			Timestamps:  []int64{1, 3},
			Values:      []model.FieldValue{model.Float64Value(1), model.Float64Value(5)},
		},
		{
			SeriesID:    2,
			Measurement: "cpu",
			Tags:        map[string]string{"region": "east"},
			FieldName:   "usage",
			FieldType:   model.FieldFloat64,
			Timestamps:  []int64{2},
			Values:      []model.FieldValue{model.Float64Value(2)},
		},
	})
	stream := NewGroupAggregateColumnStream(
		source,
		[]model.AggregateSpec{{Field: "usage", Function: "median"}},
		model.QueryGroup{Tags: []string{"region"}},
		0,
	)
	if !stream.Next() {
		t.Fatalf("Next() = false err=%v", stream.Err())
	}
	if got := stream.Column().Values[0].Float64; got != 2 {
		t.Fatalf("median = %v, want 2", got)
	}
	if stream.Next() {
		t.Fatal("Next(after median) = true, want false")
	}
}

func TestGroupAggregateColumnStreamReportsMixedTypeIncrementalErrors(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{
		{
			SeriesID:    1,
			Measurement: "cpu",
			Tags:        map[string]string{"region": "east"},
			FieldName:   "usage",
			FieldType:   model.FieldFloat64,
			Timestamps:  []int64{1},
			Values:      []model.FieldValue{model.Float64Value(1)},
		},
		{
			SeriesID:    2,
			Measurement: "cpu",
			Tags:        map[string]string{"region": "east"},
			FieldName:   "usage",
			FieldType:   model.FieldInt64,
			Timestamps:  []int64{2},
			Values:      []model.FieldValue{model.Int64Value(2)},
		},
	})
	stream := NewGroupAggregateColumnStream(
		source,
		[]model.AggregateSpec{{Field: "usage", Function: "sum"}},
		model.QueryGroup{Tags: []string{"region"}},
		0,
	)
	if stream.Next() {
		t.Fatal("Next() = true, want false for mixed field types")
	}
	if stream.Err() == nil {
		t.Fatal("Err() = nil, want mixed type error")
	}
}

func TestGroupAggregateColumnStreamGroupsByTagAndWindow(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{
		{
			SeriesID:    1,
			Measurement: "cpu",
			Tags:        map[string]string{"region": "east"},
			FieldName:   "usage",
			FieldType:   model.FieldInt64,
			Timestamps:  []int64{0, int64(1500 * time.Millisecond)},
			Values:      []model.FieldValue{model.Int64Value(1), model.Int64Value(2)},
		},
		{
			SeriesID:    2,
			Measurement: "cpu",
			Tags:        map[string]string{"region": "east"},
			FieldName:   "usage",
			FieldType:   model.FieldInt64,
			Timestamps:  []int64{int64(500 * time.Millisecond), int64(2500 * time.Millisecond)},
			Values:      []model.FieldValue{model.Int64Value(3), model.Int64Value(4)},
		},
	})
	stream := NewGroupAggregateColumnStream(
		source,
		[]model.AggregateSpec{{Field: "usage", Function: "sum"}},
		model.QueryGroup{Tags: []string{"region"}, Window: time.Second},
		time.Second,
	)

	if !stream.Next() {
		t.Fatalf("Next() = false err=%v", stream.Err())
	}
	column := stream.Column()
	wantTimes := []int64{0, int64(time.Second), int64(2 * time.Second)}
	wantValues := []int64{4, 2, 4}
	if len(column.Timestamps) != len(wantTimes) {
		t.Fatalf("timestamps = %#v, want %#v", column.Timestamps, wantTimes)
	}
	for index := range wantTimes {
		if column.Timestamps[index] != wantTimes[index] || column.Values[index].Int64 != wantValues[index] {
			t.Fatalf(
				"point %d = (%d,%d), want (%d,%d)",
				index,
				column.Timestamps[index],
				column.Values[index].Int64,
				wantTimes[index],
				wantValues[index],
			)
		}
	}
}

func TestGroupAggregateColumnStreamUsesWindowedPointFallback(t *testing.T) {
	source := NewSliceColumnSeriesStream([]model.ColumnSeries{
		{
			SeriesID:    1,
			Measurement: "cpu",
			Tags:        map[string]string{"region": "east"},
			FieldName:   "usage",
			FieldType:   model.FieldFloat64,
			Timestamps:  []int64{0, int64(1500 * time.Millisecond)},
			Values:      []model.FieldValue{model.Float64Value(1), model.Float64Value(5)},
		},
		{
			SeriesID:    2,
			Measurement: "cpu",
			Tags:        map[string]string{"region": "east"},
			FieldName:   "usage",
			FieldType:   model.FieldFloat64,
			Timestamps:  []int64{int64(500 * time.Millisecond), int64(2500 * time.Millisecond)},
			Values:      []model.FieldValue{model.Float64Value(3), model.Float64Value(9)},
		},
	})
	stream := NewGroupAggregateColumnStream(
		source,
		[]model.AggregateSpec{{Field: "usage", Function: "median"}},
		model.QueryGroup{Tags: []string{"region"}, Window: time.Second},
		0,
	)
	if !stream.Next() {
		t.Fatalf("Next() = false err=%v", stream.Err())
	}
	column := stream.Column()
	wantTimes := []int64{0, int64(time.Second), int64(2 * time.Second)}
	wantValues := []float64{2, 5, 9}
	if len(column.Timestamps) != len(wantTimes) {
		t.Fatalf("timestamps = %#v, want %#v", column.Timestamps, wantTimes)
	}
	for index := range wantTimes {
		if column.Timestamps[index] != wantTimes[index] || column.Values[index].Float64 != wantValues[index] {
			t.Fatalf(
				"point %d = (%d,%v), want (%d,%v)",
				index,
				column.Timestamps[index],
				column.Values[index].Float64,
				wantTimes[index],
				wantValues[index],
			)
		}
	}
}
