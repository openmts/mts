package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	mts "github.com/openmts/mts"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("simple_integrity failed: %v", err)
	}
	log.Print("simple_integrity passed")
}

func run() (err error) {
	dir, err := os.MkdirTemp("", "mts-e2e-simple-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("set temp dir permissions: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(dir))
	}()
	return runWithDir(dir)
}

func runWithDir(dir string) (err error) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 2,
	})
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	defer func() {
		err = errors.Join(err, eng.Close(ctx))
	}()

	if err := writeScenario(ctx, eng); err != nil {
		return err
	}
	if err := eng.Flush(ctx); err != nil {
		return fmt.Errorf("flush engine: %w", err)
	}
	if err := eng.Compact(ctx); err != nil {
		return fmt.Errorf("compact engine: %w", err)
	}

	rows, err := eng.QueryRows(ctx, mts.Query{
		Measurement: "e2e",
		Tags:        map[string]string{"host": "a"},
		StartTime:   0,
		EndTime:     int64(time.Hour),
	})
	if err != nil {
		return fmt.Errorf("query rows: %w", err)
	}
	return assertRows(rows)
}

func writeScenario(ctx context.Context, eng *mts.Engine) error {
	points := []mts.Point{
		newPoint("a", 10, 1.5, 1, "old", false),
		newPoint("a", 10, 2.5, 2, "ok", true),
		newPoint("b", 20, 3.5, 3, "skip", false),
	}
	if err := eng.Write(ctx, points, mts.WriteOptions{Sync: true}); err != nil {
		return fmt.Errorf("write points: %w", err)
	}
	return nil
}

func newPoint(host string, ts int64, usage float64, count int64, state string, active bool) mts.Point {
	return mts.Point{
		Measurement: "e2e",
		Tags:        map[string]string{"host": host},
		Timestamp:   ts,
		Fields: map[string]mts.FieldValue{
			"active": mts.BoolValue(active),
			"count":  mts.Int64Value(count),
			"state":  mts.StringValue(state),
			"usage":  mts.Float64Value(usage),
		},
	}
}

func assertRows(rows []mts.Row) error {
	if len(rows) != 1 {
		return fmt.Errorf("row count = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.Timestamp != 10 {
		return fmt.Errorf("timestamp = %d, want 10", row.Timestamp)
	}
	checks := []struct {
		name string
		ok   bool
	}{
		{name: "usage", ok: row.Fields["usage"].Float64 == 2.5},
		{name: "count", ok: row.Fields["count"].Int64 == 2},
		{name: "state", ok: row.Fields["state"].String == "ok"},
		{name: "active", ok: row.Fields["active"].Bool},
	}
	for _, check := range checks {
		if !check.ok {
			return fmt.Errorf("field %s mismatch: %#v", check.name, row.Fields[check.name])
		}
	}
	return nil
}
