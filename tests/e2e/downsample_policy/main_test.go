package main

import (
	"path/filepath"
	"testing"
	"time"

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

func TestAssertRowsRejectsInvalidResults(t *testing.T) {
	valid := mts.Row{
		Timestamp: int64(2 * time.Minute),
		Tags:      map[string]string{"host": "a"},
		Fields: map[string]mts.FieldValue{
			"avg_usage":   mts.Float64Value(7),
			"max_usage":   mts.Float64Value(12),
			"count_usage": mts.Int64Value(2),
		},
	}
	cases := map[string][]mts.Row{
		"empty": {},
		"avg": {{
			Timestamp: valid.Timestamp,
			Tags:      valid.Tags,
			Fields: map[string]mts.FieldValue{
				"avg_usage":   mts.Float64Value(8),
				"max_usage":   mts.Float64Value(12),
				"count_usage": mts.Int64Value(2),
			},
		}},
		"tags": {{
			Timestamp: valid.Timestamp,
			Tags:      map[string]string{"host": "a", "region": "east"},
			Fields:    valid.Fields,
		}},
	}
	for name, rows := range cases {
		t.Run(name, func(t *testing.T) {
			if err := assertRows(rows); err == nil {
				t.Fatal("assertRows() error = nil, want error")
			}
		})
	}
}

func TestRunWithDirRejectsInvalidPath(t *testing.T) {
	if err := runWithDir(filepath.Join("bad\x00path")); err == nil {
		t.Fatal("runWithDir(invalid) error = nil, want error")
	}
}
