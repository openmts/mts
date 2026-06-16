package main

import (
	"context"
	"testing"
	"time"

	mts "codeberg.org/mts/mts"
)

func TestMainFunction(t *testing.T) {
	main()
}

func TestRun(t *testing.T) {
	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunWithDirRejectsInvalidPath(t *testing.T) {
	if err := runWithDir("bad\x00path"); err == nil {
		t.Fatal("runWithDir(invalid) error = nil, want error")
	}
}

func TestWriteScenarioRejectsClosedEngine(t *testing.T) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{Path: t.TempDir(), ShardDuration: time.Hour})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := writeScenario(ctx, eng); err == nil {
		t.Fatal("writeScenario(closed) error = nil, want error")
	}
}

func TestAssertRowsRejectsMismatch(t *testing.T) {
	tests := []struct {
		name string
		rows []mts.Row
	}{
		{name: "empty rows", rows: nil},
		{name: "wrong timestamp", rows: []mts.Row{{Timestamp: 11, Fields: validFields()}}},
		{name: "wrong usage", rows: []mts.Row{{Timestamp: 10, Fields: invalidFields("usage")}}},
		{name: "wrong count", rows: []mts.Row{{Timestamp: 10, Fields: invalidFields("count")}}},
		{name: "wrong state", rows: []mts.Row{{Timestamp: 10, Fields: invalidFields("state")}}},
		{name: "wrong active", rows: []mts.Row{{Timestamp: 10, Fields: invalidFields("active")}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := assertRows(tt.rows); err == nil {
				t.Fatal("assertRows() error = nil, want error")
			}
		})
	}
}

func validFields() map[string]mts.FieldValue {
	return map[string]mts.FieldValue{
		"active": mts.BoolValue(true),
		"count":  mts.Int64Value(2),
		"state":  mts.StringValue("ok"),
		"usage":  mts.Float64Value(2.5),
	}
}

func invalidFields(name string) map[string]mts.FieldValue {
	fields := validFields()
	switch name {
	case "active":
		fields[name] = mts.BoolValue(false)
	case "count":
		fields[name] = mts.Int64Value(1)
	case "state":
		fields[name] = mts.StringValue("bad")
	case "usage":
		fields[name] = mts.Float64Value(1.5)
	}
	return fields
}
