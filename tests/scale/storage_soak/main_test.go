package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	mts "codeberg.org/mts/mts"
	"codeberg.org/mts/mts/internal/faultinject"
	"codeberg.org/mts/mts/internal/storagefs"
)

func TestRunSmoke(t *testing.T) {
	if err := run([]string{"-seed", "7", "-duration", time.Millisecond.String()}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
}

func TestMainFunction(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"storage_soak", "-seed", "7", "-duration", time.Millisecond.String()}
	main()
	os.Args = oldArgs
}

func TestRunRejectsInvalidArgs(t *testing.T) {
	if err := run([]string{"-duration", "bad"}); err == nil {
		t.Fatal("run(invalid args) error = nil, want error")
	}
}

func TestSoakPoints(t *testing.T) {
	points := soakPoints(2, 3)
	if len(points) != 3 {
		t.Fatalf("soakPoints() len = %d, want 3", len(points))
	}
	if points[0].Timestamp != 2000 || points[2].Fields["value"].Int64 != 2002 {
		t.Fatalf("soakPoints() = %#v, want iteration timestamps", points)
	}
}

func TestSoakReportIncludesCompactionHealth(t *testing.T) {
	report := soakReport{
		PartCount:         3,
		LevelDistribution: map[int]int{0: 1, 1: 2},
		HealthDegraded:    true,
		CompactionBacklog: 2,
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	text := string(data)
	for _, key := range []string{
		"part_count",
		"level_distribution",
		"health_degraded",
		"compaction_backlog",
	} {
		if !strings.Contains(text, key) {
			t.Fatalf("soak report json %s missing %s", text, key)
		}
	}
}

func TestRunSoakPropagatesStorageFault(t *testing.T) {
	fs := faultinject.NewFS()
	fs.FailNext(faultinject.OpWrite, os.ErrPermission)
	restore := storagefs.SetFaultController(fs)
	_, err := runSoak(t.TempDir(), 1, time.Millisecond)
	restore()
	if err == nil {
		t.Fatal("runSoak(write fault) error = nil, want error")
	}
}

func TestVerifyRowsRejectsIntegrityErrors(t *testing.T) {
	expected := map[int64]int64{1: 1}
	valid := []mts.Row{{Timestamp: 1, Fields: map[string]mts.FieldValue{"value": mts.Int64Value(1)}}}
	if err := verifyRows(valid, expected, 7); err != nil {
		t.Fatalf("verifyRows(valid) error = %v", err)
	}
	for name, rows := range map[string][]mts.Row{
		"count":     {},
		"timestamp": {{Timestamp: 2, Fields: map[string]mts.FieldValue{"value": mts.Int64Value(2)}}},
		"missing":   {{Timestamp: 1, Fields: map[string]mts.FieldValue{}}},
		"type":      {{Timestamp: 1, Fields: map[string]mts.FieldValue{"value": mts.Float64Value(1)}}},
		"value":     {{Timestamp: 1, Fields: map[string]mts.FieldValue{"value": mts.Int64Value(2)}}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := verifyRows(rows, expected, 7); err == nil {
				t.Fatal("verifyRows() error = nil, want error")
			}
		})
	}
	duplicates := []mts.Row{
		{Timestamp: 1, Fields: map[string]mts.FieldValue{"value": mts.Int64Value(1)}},
		{Timestamp: 1, Fields: map[string]mts.FieldValue{"value": mts.Int64Value(1)}},
	}
	if err := verifyRows(duplicates, map[int64]int64{1: 1, 2: 2}, 7); err == nil {
		t.Fatal("verifyRows(duplicate) error = nil, want error")
	}
}
