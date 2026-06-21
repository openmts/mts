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
		log.Fatalf("downsample_policy failed: %v", err)
	}
	log.Print("downsample_policy passed")
}

func run() (err error) {
	dir, err := os.MkdirTemp("", "mts-e2e-downsample-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("chmod temp dir: %w", err)
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
		MemTableMaxSamples: 128,
	})
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	defer func() {
		err = errors.Join(err, eng.Close(ctx))
	}()
	if err := eng.Write(ctx, rawPoints(), mts.WriteOptions{}); err != nil {
		return fmt.Errorf("write raw points: %w", err)
	}
	if err := eng.CreateDownsamplePolicy(ctx, policy()); err != nil {
		return fmt.Errorf("create policy: %w", err)
	}
	result, err := eng.RunDownsamplePolicy(ctx, "cpu_1m", time.Unix(0, int64(10*time.Minute)))
	if err != nil {
		return fmt.Errorf("run policy: %w", err)
	}
	if result.WindowsProcessed != 10 || result.PointsWritten != 10 {
		return fmt.Errorf("run result = %#v, want 10 windows and points", result)
	}
	rows, err := eng.QueryRows(ctx, mts.Query{
		Database:        "metrics",
		RetentionPolicy: "rp_1m",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a"},
		StartTime:       0,
		EndTime:         int64(10 * time.Minute),
	})
	if err != nil {
		return fmt.Errorf("query target rows: %w", err)
	}
	return assertRows(rows)
}

func rawPoints() []mts.Point {
	points := make([]mts.Point, 0, 20)
	for minute := int64(0); minute < 10; minute++ {
		base := minute * int64(time.Minute)
		points = append(points,
			rawPoint(base, float64(minute)),
			rawPoint(base+int64(30*time.Second), float64(minute+10)),
		)
	}
	return points
}

func rawPoint(timestamp int64, usage float64) mts.Point {
	return mts.Point{
		Database:        "metrics",
		RetentionPolicy: "autogen",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a", "region": "east"},
		Timestamp:       timestamp,
		Fields: map[string]mts.FieldValue{
			"usage": mts.Float64Value(usage),
		},
	}
}

func policy() mts.DownsamplePolicy {
	return mts.DownsamplePolicy{
		Name:              "cpu_1m",
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   "rp_1m",
		TargetMeasurement: "cpu",
		Interval:          time.Minute,
		Functions: []mts.DownsampleFunction{
			{Function: "avg", Field: "usage"},
			{Function: "max", Field: "usage"},
			{Function: "count", Field: "usage"},
		},
		GroupByTags:     []string{"host"},
		Delay:           0,
		RefreshInterval: time.Minute,
		Lookback:        10 * time.Minute,
		Enabled:         true,
	}
}

func assertRows(rows []mts.Row) error {
	if len(rows) != 10 {
		return fmt.Errorf("row count = %d, want 10", len(rows))
	}
	for _, row := range rows {
		minute := row.Timestamp / int64(time.Minute)
		wantAvg := float64(minute) + 5
		if row.Fields["avg_usage"].Float64 != wantAvg {
			return fmt.Errorf("avg at %d = %v, want %v", minute, row.Fields["avg_usage"], wantAvg)
		}
		wantMax := float64(minute + 10)
		if row.Fields["max_usage"].Float64 != wantMax {
			return fmt.Errorf("max at %d = %v, want %v", minute, row.Fields["max_usage"], wantMax)
		}
		if row.Fields["count_usage"].Int64 != 2 {
			return fmt.Errorf("count at %d = %v, want 2", minute, row.Fields["count_usage"])
		}
		if row.Tags["host"] != "a" ||
			row.Tags["mts_downsample_policy"] != "cpu_1m" ||
			len(row.Tags) != 2 {
			return fmt.Errorf("tags = %#v, want host=a and policy tag", row.Tags)
		}
	}
	return nil
}
