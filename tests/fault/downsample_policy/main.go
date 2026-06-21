package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	mts "github.com/openmts/mts"
	"github.com/openmts/mts/internal/faultinject"
	"github.com/openmts/mts/internal/storagefs"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("downsample_policy_fault failed: %v", err)
	}
	log.Print("downsample_policy_fault passed")
}

func run() (err error) {
	dir, err := os.MkdirTemp("", "mts-fault-downsample-*")
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
	eng, err := mts.Open(ctx, mts.Options{Path: dir, ShardDuration: time.Hour})
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	defer func() {
		err = errors.Join(err, eng.Close(ctx))
	}()
	if err := eng.Write(ctx, faultRawPoints(), mts.WriteOptions{}); err != nil {
		return fmt.Errorf("write raw: %w", err)
	}
	if err := eng.CreateDownsamplePolicy(ctx, faultPolicy()); err != nil {
		return fmt.Errorf("create policy: %w", err)
	}
	faultErr := withWriteFault(func() error {
		_, err := eng.RunDownsamplePolicy(ctx, "cpu_1m", time.Unix(0, int64(2*time.Minute)))
		return err
	})
	if faultErr != nil {
		return faultErr
	}
	if !downsampleHealthDegraded(eng.HealthSnapshot()) {
		return fmt.Errorf("health downsample is not degraded after failed run")
	}
	result, err := eng.RunDownsamplePolicy(ctx, "cpu_1m", time.Unix(0, int64(2*time.Minute)))
	if err != nil {
		return fmt.Errorf("rerun policy: %w", err)
	}
	if result.WindowsProcessed != 2 || result.PointsWritten != 2 {
		return fmt.Errorf("rerun result = %#v, want watermark preserved and two windows", result)
	}
	return nil
}

func faultRawPoints() []mts.Point {
	return []mts.Point{
		faultRawPoint(0, 1),
		faultRawPoint(int64(time.Minute), 2),
	}
}

func faultRawPoint(timestamp int64, usage float64) mts.Point {
	return mts.Point{
		Database:        "metrics",
		RetentionPolicy: "autogen",
		Measurement:     "cpu",
		Tags:            map[string]string{"host": "a"},
		Timestamp:       timestamp,
		Fields:          map[string]mts.FieldValue{"usage": mts.Float64Value(usage)},
	}
}

func faultPolicy() mts.DownsamplePolicy {
	return mts.DownsamplePolicy{
		Name:              "cpu_1m",
		SourceDatabase:    "metrics",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "metrics",
		TargetRetention:   "rp_1m",
		TargetMeasurement: "cpu",
		Interval:          time.Minute,
		Functions: []mts.DownsampleFunction{{
			Function: "avg",
			Field:    "usage",
		}},
		GroupByTags:     []string{"host"},
		RefreshInterval: time.Minute,
		Lookback:        2 * time.Minute,
		Enabled:         true,
	}
}

func withWriteFault(fn func() error) error {
	fs := faultinject.NewFS()
	fs.FailNext(faultinject.OpWrite, errors.New("injected"))
	restore := storagefs.SetFaultController(fs)
	err := fn()
	restore()
	if err == nil {
		return fmt.Errorf("RunDownsamplePolicy error = nil, want injected write fault")
	}
	if !strings.Contains(err.Error(), "fault write") {
		return fmt.Errorf("RunDownsamplePolicy error = %v, want write fault", err)
	}
	return nil
}

func downsampleHealthDegraded(health mts.HealthSnapshot) bool {
	for _, check := range health.Checks {
		if check.Name == "downsample" && check.Status == "degraded" {
			return true
		}
	}
	return false
}
