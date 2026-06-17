package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	mts "codeberg.org/mts/mts"
)

type soakReport struct {
	Seed       int64         `json:"seed"`
	Duration   time.Duration `json:"duration"`
	Iterations int           `json:"iterations"`
	Rows       int           `json:"rows"`
	OK         bool          `json:"ok"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) (err error) {
	flags := flag.NewFlagSet("storage_soak", flag.ContinueOnError)
	seed := flags.Int64("seed", 1, "seed")
	duration := flags.Duration("duration", time.Second, "duration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "mts-soak-*")
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
	report, err := runSoak(dir, *seed, *duration)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}

func runSoak(dir string, seed int64, duration time.Duration) (soakReport, error) {
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{
		Path:               dir,
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 64,
		Compaction:         mts.CompactionOptions{Enabled: true, Level0PartLimit: 2, MaxCascadeSteps: 4},
	})
	if err != nil {
		return soakReport{}, fmt.Errorf("open engine: %w", err)
	}
	random := rand.New(rand.NewSource(seed))
	deadline := time.Now().Add(duration)
	iterations := 0
	expected := make(map[int64]int64)
	for iterations == 0 || time.Now().Before(deadline) {
		points := soakPoints(iterations, random.Intn(8)+1)
		for _, point := range points {
			expected[point.Timestamp] = point.Fields["value"].Int64
		}
		if err := eng.Write(ctx, points, mts.WriteOptions{}); err != nil {
			closeErr := eng.Close(ctx)
			return soakReport{}, errors.Join(fmt.Errorf("write points: %w", err), closeErr)
		}
		if iterations%3 == 0 {
			if err := eng.Flush(ctx); err != nil {
				closeErr := eng.Close(ctx)
				return soakReport{}, errors.Join(fmt.Errorf("flush: %w", err), closeErr)
			}
		}
		if iterations%5 == 0 {
			if err := eng.Compact(ctx); err != nil {
				closeErr := eng.Close(ctx)
				return soakReport{}, errors.Join(fmt.Errorf("compact: %w", err), closeErr)
			}
		}
		queryEnd := int64((iterations + 1) * 1000)
		rows, err := eng.QueryRows(ctx, mts.Query{Measurement: "soak", StartTime: 0, EndTime: queryEnd})
		if err != nil {
			closeErr := eng.Close(ctx)
			return soakReport{}, errors.Join(fmt.Errorf("query rows: %w", err), closeErr)
		}
		if err := verifyRows(rows, expected, seed); err != nil {
			closeErr := eng.Close(ctx)
			return soakReport{}, errors.Join(err, closeErr)
		}
		iterations++
	}
	if err := eng.Close(ctx); err != nil {
		return soakReport{}, err
	}
	return soakReport{Seed: seed, Duration: duration, Iterations: iterations, Rows: len(expected), OK: true}, nil
}

func verifyRows(rows []mts.Row, expected map[int64]int64, seed int64) error {
	if len(rows) != len(expected) {
		return fmt.Errorf("rows=%d want=%d seed=%d", len(rows), len(expected), seed)
	}
	seen := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		want, ok := expected[row.Timestamp]
		if !ok {
			return fmt.Errorf("unexpected timestamp=%d seed=%d", row.Timestamp, seed)
		}
		if _, duplicate := seen[row.Timestamp]; duplicate {
			return fmt.Errorf("duplicate timestamp=%d seed=%d", row.Timestamp, seed)
		}
		seen[row.Timestamp] = struct{}{}
		value, ok := row.Fields["value"]
		if !ok {
			return fmt.Errorf("missing value field timestamp=%d seed=%d", row.Timestamp, seed)
		}
		if value.Type != mts.FieldInt64 || value.Int64 != want {
			return fmt.Errorf("value timestamp=%d got=%+v want=%d seed=%d", row.Timestamp, value, want, seed)
		}
	}
	return nil
}

func soakPoints(iteration int, count int) []mts.Point {
	points := make([]mts.Point, 0, count)
	base := iteration * 1000
	for index := range count {
		timestamp := int64(base + index)
		points = append(points, mts.Point{
			Measurement: "soak",
			Tags:        map[string]string{"host": fmt.Sprintf("host-%02d", iteration%4)},
			Timestamp:   timestamp,
			Fields:      map[string]mts.FieldValue{"value": mts.Int64Value(timestamp)},
		})
	}
	return points
}
