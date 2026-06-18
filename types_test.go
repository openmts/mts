package mts_test

import (
	"context"
	"reflect"
	"testing"

	mts "github.com/openmts/mts"
	"github.com/openmts/mts/internal/model"
)

func TestFieldValueConstructors(t *testing.T) {
	tests := []struct {
		name  string
		value mts.FieldValue
		check func(t *testing.T, value mts.FieldValue)
	}{
		{
			name:  "float64 value",
			value: mts.Float64Value(1.5),
			check: func(t *testing.T, value mts.FieldValue) {
				t.Helper()
				if value.Type != mts.FieldFloat64 {
					t.Fatalf("Type = %v, want %v", value.Type, mts.FieldFloat64)
				}
				if value.Float64 != 1.5 {
					t.Fatalf("Float64 = %v, want 1.5", value.Float64)
				}
			},
		},
		{
			name:  "int64 value",
			value: mts.Int64Value(-2),
			check: func(t *testing.T, value mts.FieldValue) {
				t.Helper()
				if value.Type != mts.FieldInt64 {
					t.Fatalf("Type = %v, want %v", value.Type, mts.FieldInt64)
				}
				if value.Int64 != -2 {
					t.Fatalf("Int64 = %v, want -2", value.Int64)
				}
			},
		},
		{
			name:  "string value",
			value: mts.StringValue("ok"),
			check: func(t *testing.T, value mts.FieldValue) {
				t.Helper()
				if value.Type != mts.FieldString {
					t.Fatalf("Type = %v, want %v", value.Type, mts.FieldString)
				}
				if value.String != "ok" {
					t.Fatalf("String = %q, want %q", value.String, "ok")
				}
			},
		},
		{
			name:  "bool value",
			value: mts.BoolValue(true),
			check: func(t *testing.T, value mts.FieldValue) {
				t.Helper()
				if value.Type != mts.FieldBool {
					t.Fatalf("Type = %v, want %v", value.Type, mts.FieldBool)
				}
				if !value.Bool {
					t.Fatal("Bool = false, want true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, tt.value)
		})
	}
}

func TestOpenInvalidPathReturnsError(t *testing.T) {
	if _, err := mts.Open(context.Background(), mts.Options{Path: "bad\x00path"}); err == nil {
		t.Fatal("Open(invalid) error = nil, want error")
	}
}

func TestPublicTypesAreIndependentDTOs(t *testing.T) {
	cases := []struct {
		name   string
		public reflect.Type
		model  reflect.Type
	}{
		{name: "Point", public: reflect.TypeOf(mts.Point{}), model: reflect.TypeOf(model.Point{})},
		{name: "Query", public: reflect.TypeOf(mts.Query{}), model: reflect.TypeOf(model.Query{})},
		{name: "Options", public: reflect.TypeOf(mts.Options{}), model: reflect.TypeOf(model.Options{})},
		{name: "ColumnSeries", public: reflect.TypeOf(mts.ColumnSeries{}), model: reflect.TypeOf(model.ColumnSeries{})},
		{name: "Row", public: reflect.TypeOf(mts.Row{}), model: reflect.TypeOf(model.Row{})},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.public == tt.model {
				t.Fatalf("%s is an alias of internal model type; want independent public DTO", tt.name)
			}
		})
	}
}

func TestStorageMemoryOptionsArePublicAndInternalDTOs(t *testing.T) {
	if _, ok := reflect.TypeOf(mts.Options{}).FieldByName("StorageMemory"); !ok {
		t.Fatal("public Options missing StorageMemory field")
	}
	if _, ok := reflect.TypeOf(model.Options{}).FieldByName("StorageMemory"); !ok {
		t.Fatal("model Options missing StorageMemory field")
	}
	if reflect.TypeOf(mts.StorageMemoryOptions{}) == reflect.TypeOf(model.StorageMemoryOptions{}) {
		t.Fatal("StorageMemoryOptions is an alias of internal model type; want independent public DTO")
	}
	publicType := reflect.TypeOf(mts.StorageMemoryOptions{})
	for _, field := range []string{
		"SoftBytesLimit",
		"HardBytesLimit",
		"QueryBytesLimit",
		"FlushBytesLimit",
		"CompactionBytesLimit",
		"CompressionBytesLimit",
	} {
		if _, ok := publicType.FieldByName(field); !ok {
			t.Fatalf("public StorageMemoryOptions missing %s", field)
		}
	}
}
