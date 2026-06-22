package main

import (
	"testing"

	mts "github.com/openmts/mts"
)

func TestMainSmoke(t *testing.T) {
	main()
}

func TestRunRejectsInvalidTempDir(t *testing.T) {
	t.Setenv("TMPDIR", "/dev/null")
	if err := run(); err == nil {
		t.Fatal("run(invalid temp dir) error = nil, want error")
	}
}

func TestAssertAggregateColumnsRejectsInvalidResults(t *testing.T) {
	validCount := mts.ColumnSeries{
		FieldName: "count(value)",
		Values:    []mts.FieldValue{mts.Int64Value(2), mts.Int64Value(1)},
	}
	validSum := mts.ColumnSeries{
		FieldName: "sum(value)",
		Values:    []mts.FieldValue{mts.Float64Value(3), mts.Float64Value(3)},
	}
	validAvg := mts.ColumnSeries{
		FieldName: "avg(value)",
		Values:    []mts.FieldValue{mts.Float64Value(1.5), mts.Float64Value(3)},
	}
	cases := map[string][]mts.ColumnSeries{
		"count":      {validCount},
		"bad_count":  {{FieldName: "count(value)", Values: []mts.FieldValue{mts.Int64Value(1), mts.Int64Value(1)}}, validSum, validAvg},
		"unexpected": {validCount, validSum, {FieldName: "max(value)"}},
		"length":     {validCount, {FieldName: "sum(value)", Values: []mts.FieldValue{mts.Float64Value(3)}}, validAvg},
		"value":      {validCount, {FieldName: "sum(value)", Values: []mts.FieldValue{mts.Float64Value(4), mts.Float64Value(3)}}, validAvg},
	}
	for name, columns := range cases {
		t.Run(name, func(t *testing.T) {
			if err := assertAggregateColumns(columns); err == nil {
				t.Fatal("assertAggregateColumns() error = nil, want error")
			}
		})
	}
}

func TestRunWithDirRejectsInvalidPath(t *testing.T) {
	if err := runWithDir("bad\x00path"); err == nil {
		t.Fatal("runWithDir(invalid) error = nil, want error")
	}
}
