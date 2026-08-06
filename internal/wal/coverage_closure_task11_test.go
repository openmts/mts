package wal

import (
	"reflect"
	"testing"

	"github.com/openmts/mts/internal/model"
)

func TestAppendFieldCoversTypedPayloadsAndValidation(t *testing.T) {
	fields := []model.ResolvedField{
		{FieldID: 1, FieldName: "float", Type: model.FieldFloat64, Value: model.Float64Value(1.5)},
		{FieldID: 2, FieldName: "int", Type: model.FieldInt64, Value: model.Int64Value(2)},
		{FieldID: 3, FieldName: "string", Type: model.FieldString, Value: model.StringValue("ok")},
		{FieldID: 4, FieldName: "bool", Type: model.FieldBool, Value: model.BoolValue(true)},
	}
	for index, field := range fields {
		t.Run(field.FieldName, func(t *testing.T) {
			encoded, err := appendField(nil, field, index)
			if err != nil {
				t.Fatalf("appendField() error = %v", err)
			}
			if len(encoded) == 0 {
				t.Fatal("appendField() returned empty payload")
			}
		})
	}

	mismatch := model.ResolvedField{
		FieldID:   5,
		FieldName: "mismatch",
		Type:      model.FieldInt64,
		Value:     model.Float64Value(1),
	}
	if _, err := appendField(nil, mismatch, 0); err == nil {
		t.Fatal("appendField(type mismatch) error = nil, want error")
	}
	unknown := model.ResolvedField{
		FieldID:   6,
		FieldName: "unknown",
		Type:      model.FieldType(99),
		Value:     model.FieldValue{Type: model.FieldType(99)},
	}
	if _, err := appendField(nil, unknown, 0); err == nil {
		t.Fatal("appendField(unknown type) error = nil, want error")
	}
}

func TestWALAppendTypedTracksMemoryAndReplaysSelectedRows(t *testing.T) {
	dir := t.TempDir()
	log, err := Open(dir, Options{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if got := log.ApproxMemoryBytes(); got != 0 {
		closeErr := log.Close()
		t.Fatalf("ApproxMemoryBytes(empty) = %d, want 0 close=%v", got, closeErr)
	}

	batch := model.ResolvedTypedBatch{
		Database:        "metrics",
		RetentionPolicy: "hot",
		Measurement:     "cpu",
		Tags: []model.TagColumn{
			{Name: "host", Values: []string{"a", "b"}},
		},
		Timestamps: []int64{10, 20},
		SeriesIDs:  []uint64{1, 2},
		WriteSeqs:  []uint64{7, 8},
		Fields: []model.ResolvedTypedFieldColumn{
			{
				FieldID:       1,
				Name:          "usage",
				Type:          model.FieldFloat64,
				Float64Values: []float64{1.5, 2.5},
			},
			{
				FieldID:    2,
				Name:       "active",
				Type:       model.FieldBool,
				BoolValues: []bool{true, false},
			},
		},
	}
	if err := log.AppendTyped(batch, []int{1}, false); err != nil {
		closeErr := log.Close()
		t.Fatalf("AppendTyped() error = %v close=%v", err, closeErr)
	}
	if got := log.ApproxMemoryBytes(); got <= 16 {
		closeErr := log.Close()
		t.Fatalf("ApproxMemoryBytes(after append) = %d, want payload plus record close=%v", got, closeErr)
	}
	invalid := batch
	invalid.SeriesIDs = nil
	if err := log.AppendTyped(invalid, nil, false); err == nil {
		closeErr := log.Close()
		t.Fatalf("AppendTyped(invalid) error = nil, want error close=%v", closeErr)
	}
	if err := log.FlushPending(); err != nil {
		closeErr := log.Close()
		t.Fatalf("FlushPending() error = %v close=%v", err, closeErr)
	}
	if got := log.ApproxMemoryBytes(); got != 0 {
		closeErr := log.Close()
		t.Fatalf("ApproxMemoryBytes(after flush) = %d, want 0 close=%v", got, closeErr)
	}
	if err := log.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := Open(dir, Options{})
	if err != nil {
		t.Fatalf("Open(replay) error = %v", err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Fatalf("Close(replay) error = %v", err)
		}
	}()
	points, err := reopened.Replay()
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	want := []model.ResolvedPoint{{
		Database:        "metrics",
		RetentionPolicy: "hot",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "b"},
		SeriesID:        2,
		Timestamp:       20,
		WriteSeq:        8,
		Fields: []model.ResolvedField{
			{FieldID: 1, FieldName: "usage", Type: model.FieldFloat64, Value: model.Float64Value(2.5)},
			{FieldID: 2, FieldName: "active", Type: model.FieldBool, Value: model.BoolValue(false)},
		},
	}}
	if !reflect.DeepEqual(points, want) {
		t.Fatalf("Replay() = %#v, want %#v", points, want)
	}
}
