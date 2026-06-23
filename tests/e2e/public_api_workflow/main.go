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

const databaseName = "metrics"

const retentionName = "rp_hot"

const measurementName = "cpu"

func main() {
	if err := run(); err != nil {
		log.Fatalf("public_api_workflow failed: %v", err)
	}
	log.Print("public_api_workflow passed")
}

func run() (err error) {
	dir, err := os.MkdirTemp("", "mts-e2e-public-api-*")
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
	eng, err := openPublicEngine(ctx, dir)
	if err != nil {
		return err
	}
	if err := prepareMetadata(ctx, eng); err != nil {
		return errors.Join(err, eng.Close(ctx))
	}
	if err := writeTypedScenario(ctx, eng); err != nil {
		return errors.Join(err, eng.Close(ctx))
	}
	if err := eng.Flush(ctx); err != nil {
		return errors.Join(fmt.Errorf("flush: %w", err), eng.Close(ctx))
	}
	if err := eng.Close(ctx); err != nil {
		return fmt.Errorf("close before reopen: %w", err)
	}
	reopened, err := openPublicEngine(ctx, dir)
	if err != nil {
		return fmt.Errorf("reopen engine: %w", err)
	}
	defer func() {
		err = errors.Join(err, reopened.Close(ctx))
	}()
	return verifyPublicWorkflow(ctx, reopened)
}

func openPublicEngine(ctx context.Context, dir string) (*mts.Engine, error) {
	opts := mts.DefaultOptions(dir)
	opts.DefaultDatabase = databaseName
	opts.DefaultRetentionPolicy = retentionName
	opts.MemTableMaxSamples = 3
	opts.Compression = mts.CompressionOptions{
		Enabled:       true,
		Algorithm:     "snappy",
		MinPageValues: 1,
	}
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("validate options: %w", err)
	}
	eng, err := mts.Open(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("open engine: %w", err)
	}
	return eng, nil
}

func prepareMetadata(ctx context.Context, eng *mts.Engine) error {
	if err := eng.CreateDatabase(ctx, databaseName); err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	policy := mts.RetentionPolicy{Name: retentionName, Duration: 2 * time.Hour}
	if err := eng.CreateRetentionPolicy(ctx, databaseName, policy); err != nil {
		return fmt.Errorf("create retention policy: %w", err)
	}
	return nil
}

func writeTypedScenario(ctx context.Context, eng *mts.Engine) error {
	if err := eng.WriteTypedBatch(ctx, publicTypedBatch(), mts.WriteOptions{Sync: true}); err != nil {
		return fmt.Errorf("write typed batch: %w", err)
	}
	point := mts.Point{
		Database:        databaseName,
		RetentionPolicy: retentionName,
		Measurement:     measurementName,
		Tags:            map[string]string{"host": "api-2", "region": "west"},
		Timestamp:       int64(5 * time.Second),
		Fields: map[string]mts.FieldValue{
			"active": mts.BoolValue(true),
			"cores":  mts.Int64Value(16),
			"state":  mts.StringValue("warm"),
			"usage":  mts.Float64Value(0.64),
		},
	}
	if err := eng.Write(ctx, []mts.Point{point}, mts.WriteOptions{Sync: true}); err != nil {
		return fmt.Errorf("write point: %w", err)
	}
	return nil
}

func publicTypedBatch() mts.TypedBatch {
	return mts.TypedBatch{
		Database:        databaseName,
		RetentionPolicy: retentionName,
		Measurement:     measurementName,
		Tags: []mts.TagColumn{
			{Name: "host", Values: []string{"api-1", "api-2", "api-1", "api-3"}},
			{Name: "region", Values: []string{"east", "east", "east", "west"}},
		},
		Timestamps: []int64{
			int64(time.Second),
			int64(2 * time.Second),
			int64(3 * time.Second),
			int64(4 * time.Second),
		},
		Fields: []mts.TypedFieldColumn{
			{Name: "usage", Type: mts.FieldFloat64, Float64Values: []float64{0.40, 0.70, 0.90, 0.20}},
			{Name: "cores", Type: mts.FieldInt64, Int64Values: []int64{2, 4, 8, 4}},
			{Name: "state", Type: mts.FieldString, StringValues: []string{"cold", "warm", "hot", "idle"}},
			{Name: "active", Type: mts.FieldBool, BoolValues: []bool{true, true, false, false}},
		},
	}
}
