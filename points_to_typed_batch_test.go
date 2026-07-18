package mts_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func TestPointsToTypedBatchEmpty(t *testing.T) {
	batch, err := mts.PointsToTypedBatch(nil)
	if err != nil {
		t.Fatalf("PointsToTypedBatch(nil) error = %v", err)
	}
	if !reflect.DeepEqual(batch, mts.TypedBatch{}) {
		t.Fatalf("batch = %#v, want empty", batch)
	}
	batch, err = mts.PointsToTypedBatch([]mts.Point{})
	if err != nil {
		t.Fatalf("PointsToTypedBatch([]) error = %v", err)
	}
	if !reflect.DeepEqual(batch, mts.TypedBatch{}) {
		t.Fatalf("batch = %#v, want empty", batch)
	}
}

func TestPointsToTypedBatchConvertsWideLayout(t *testing.T) {
	points := []mts.Point{
		{
			Database:        "db",
			RetentionPolicy: "rp",
			Measurement:     "bench",
			Precision:       mts.PrecisionSecond,
			Tags:            map[string]string{"host": "a", "region": "cn"},
			Timestamp:       10,
			Fields: map[string]mts.FieldValue{
				"active": mts.BoolValue(true),
				"f0":     mts.Float64Value(1.5),
				"i0":     mts.Int64Value(7),
				"state":  mts.StringValue("ok"),
			},
		},
		{
			Database:        "db",
			RetentionPolicy: "rp",
			Measurement:     "bench",
			Precision:       mts.PrecisionSecond,
			Tags:            map[string]string{"host": "b"},
			Timestamp:       20,
			Fields: map[string]mts.FieldValue{
				"active": mts.BoolValue(false),
				"f0":     mts.Float64Value(2.5),
				"i0":     mts.Int64Value(8),
				"state":  mts.StringValue("warn"),
			},
		},
	}
	batch, err := mts.PointsToTypedBatch(points)
	if err != nil {
		t.Fatalf("PointsToTypedBatch() error = %v", err)
	}
	want := mts.TypedBatch{
		Database:        "db",
		RetentionPolicy: "rp",
		Measurement:     "bench",
		Precision:       mts.PrecisionSecond,
		Tags: []mts.TagColumn{
			{Name: "host", Values: []string{"a", "b"}},
			{Name: "region", Values: []string{"cn", ""}},
		},
		Timestamps: []int64{10, 20},
		Fields: []mts.TypedFieldColumn{
			{Name: "active", Type: mts.FieldBool, BoolValues: []bool{true, false}},
			{Name: "f0", Type: mts.FieldFloat64, Float64Values: []float64{1.5, 2.5}},
			{Name: "i0", Type: mts.FieldInt64, Int64Values: []int64{7, 8}},
			{Name: "state", Type: mts.FieldString, StringValues: []string{"ok", "warn"}},
		},
	}
	if !reflect.DeepEqual(batch, want) {
		t.Fatalf("batch = %#v\nwant %#v", batch, want)
	}

	// 输入变更不应影响已转换结果。
	points[0].Tags["host"] = "mutated"
	points[0].Fields["f0"] = mts.Float64Value(99)
	if batch.Tags[0].Values[0] != "a" || batch.Fields[1].Float64Values[0] != 1.5 {
		t.Fatalf("batch mutated by input change: %#v", batch)
	}
}

func TestPointsToTypedBatchRejectsHeterogeneousInput(t *testing.T) {
	base := mts.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   1,
		Fields: map[string]mts.FieldValue{
			"usage": mts.Float64Value(1),
		},
	}
	tests := []struct {
		name   string
		points []mts.Point
	}{
		{
			name: "empty_measurement",
			points: []mts.Point{{
				Timestamp: 1,
				Fields:    map[string]mts.FieldValue{"usage": mts.Float64Value(1)},
			}},
		},
		{
			name: "empty_fields",
			points: []mts.Point{{
				Measurement: "cpu",
				Timestamp:   1,
			}},
		},
		{
			name: "identity_mismatch",
			points: []mts.Point{
				base,
				{
					Measurement: "mem",
					Tags:        map[string]string{"host": "a"},
					Timestamp:   2,
					Fields:      map[string]mts.FieldValue{"usage": mts.Float64Value(2)},
				},
			},
		},
		{
			name: "sparse_field",
			points: []mts.Point{
				{
					Measurement: "cpu",
					Timestamp:   1,
					Fields: map[string]mts.FieldValue{
						"usage": mts.Float64Value(1),
						"count": mts.Int64Value(1),
					},
				},
				{
					Measurement: "cpu",
					Timestamp:   2,
					Fields: map[string]mts.FieldValue{
						"usage": mts.Float64Value(2),
					},
				},
			},
		},
		{
			name: "field_type_conflict",
			points: []mts.Point{
				base,
				{
					Measurement: "cpu",
					Tags:        map[string]string{"host": "b"},
					Timestamp:   2,
					Fields: map[string]mts.FieldValue{
						"usage": mts.Int64Value(2),
					},
				},
			},
		},
		{
			name: "unsupported_field_type",
			points: []mts.Point{{
				Measurement: "cpu",
				Timestamp:   1,
				Fields: map[string]mts.FieldValue{
					"usage": {Type: 0, Float64: 1},
				},
			}},
		},
		{
			name: "unexpected_field_name",
			points: []mts.Point{
				{
					Measurement: "cpu",
					Timestamp:   1,
					Fields: map[string]mts.FieldValue{
						"usage": mts.Float64Value(1),
					},
				},
				{
					Measurement: "cpu",
					Timestamp:   2,
					Fields: map[string]mts.FieldValue{
						"other": mts.Float64Value(2),
					},
				},
			},
		},
		{
			name: "empty_field_name",
			points: []mts.Point{{
				Measurement: "cpu",
				Timestamp:   1,
				Fields: map[string]mts.FieldValue{
					"": mts.Float64Value(1),
				},
			}},
		},
		{
			name: "empty_tag_name",
			points: []mts.Point{{
				Measurement: "cpu",
				Tags:        map[string]string{"": "x"},
				Timestamp:   1,
				Fields: map[string]mts.FieldValue{
					"usage": mts.Float64Value(1),
				},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mts.PointsToTypedBatch(tt.points)
			if err == nil {
				t.Fatal("PointsToTypedBatch() error = nil, want error")
			}
			if !errors.Is(err, mts.ErrInvalidOptions) {
				t.Fatalf("error = %v, want ErrInvalidOptions", err)
			}
		})
	}
}

func TestWritePointsAsTypedBatchMatchesWriteSemantics(t *testing.T) {
	ctx := context.Background()
	points := []mts.Point{
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   10,
			Fields: map[string]mts.FieldValue{
				"usage":  mts.Float64Value(1.5),
				"count":  mts.Int64Value(1),
				"active": mts.BoolValue(true),
				"state":  mts.StringValue("ok"),
			},
		},
		{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "b"},
			Timestamp:   20,
			Fields: map[string]mts.FieldValue{
				"usage":  mts.Float64Value(2.5),
				"count":  mts.Int64Value(2),
				"active": mts.BoolValue(false),
				"state":  mts.StringValue("warn"),
			},
		},
	}

	writeEng, err := mts.Open(ctx, mts.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("Open(write) error = %v", err)
	}
	if err := writeEng.Write(ctx, points, mts.WriteOptions{Sync: true}); err != nil {
		closeErr := writeEng.Close(ctx)
		t.Fatalf("Write() error = %v close = %v", err, closeErr)
	}
	writeRows, err := writeEng.QueryRows(ctx, mts.Query{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     100,
	})
	if err != nil {
		closeErr := writeEng.Close(ctx)
		t.Fatalf("QueryRows(write) error = %v close = %v", err, closeErr)
	}
	if err := writeEng.Close(ctx); err != nil {
		t.Fatalf("Close(write) error = %v", err)
	}

	typedEng, err := mts.Open(ctx, mts.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
	})
	if err != nil {
		t.Fatalf("Open(typed) error = %v", err)
	}
	if err := typedEng.WritePointsAsTypedBatch(ctx, points, mts.WriteOptions{Sync: true}); err != nil {
		closeErr := typedEng.Close(ctx)
		t.Fatalf("WritePointsAsTypedBatch() error = %v close = %v", err, closeErr)
	}
	typedRows, err := typedEng.QueryRows(ctx, mts.Query{
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     100,
	})
	if err != nil {
		closeErr := typedEng.Close(ctx)
		t.Fatalf("QueryRows(typed) error = %v close = %v", err, closeErr)
	}
	if err := typedEng.Close(ctx); err != nil {
		t.Fatalf("Close(typed) error = %v", err)
	}

	if !reflect.DeepEqual(normalizeRows(writeRows), normalizeRows(typedRows)) {
		t.Fatalf("rows mismatch\nwrite=%#v\ntyped=%#v", writeRows, typedRows)
	}
}

func TestWritePointsAsTypedBatchRejectsHeterogeneousInput(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.DefaultOptions(t.TempDir()))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	err = eng.WritePointsAsTypedBatch(ctx, []mts.Point{
		{
			Measurement: "cpu",
			Timestamp:   1,
			Fields:      map[string]mts.FieldValue{"usage": mts.Float64Value(1)},
		},
		{
			Measurement: "mem",
			Timestamp:   2,
			Fields:      map[string]mts.FieldValue{"usage": mts.Float64Value(2)},
		},
	}, mts.WriteOptions{})
	closeErr := eng.Close(ctx)
	if err == nil {
		t.Fatalf("WritePointsAsTypedBatch() error = nil, want error close = %v", closeErr)
	}
	if !errors.Is(err, mts.ErrInvalidOptions) {
		t.Fatalf("error = %v, want ErrInvalidOptions", err)
	}
	if closeErr != nil {
		t.Fatalf("Close() error = %v", closeErr)
	}
}

func normalizeRows(rows []mts.Row) []mts.Row {
	out := make([]mts.Row, len(rows))
	for index, row := range rows {
		tags := make(map[string]string, len(row.Tags))
		for key, value := range row.Tags {
			tags[key] = value
		}
		fields := make(map[string]mts.FieldValue, len(row.Fields))
		for key, value := range row.Fields {
			fields[key] = value
		}
		out[index] = mts.Row{
			SeriesID:  row.SeriesID,
			Tags:      tags,
			Timestamp: row.Timestamp,
			Fields:    fields,
		}
	}
	return out
}
