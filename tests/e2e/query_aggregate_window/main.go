package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	mts "codeberg.org/mts/mts"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("query_aggregate_window failed: %v", err)
	}
	log.Print("query_aggregate_window passed")
}

func run() (err error) {
	dir, err := os.MkdirTemp("", "mts-e2e-aggregate-*")
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
	eng, err := mts.Open(ctx, mts.Options{Path: dir, ShardDuration: time.Hour, MemTableMaxSamples: 32})
	if err != nil {
		return fmt.Errorf("open engine: %w", err)
	}
	defer func() {
		err = errors.Join(err, eng.Close(ctx))
	}()
	if err := eng.Write(ctx, aggregatePoints(), mts.WriteOptions{}); err != nil {
		return fmt.Errorf("write points: %w", err)
	}
	columns, err := eng.QueryColumns(ctx, mts.Query{
		Measurement: "aggregate",
		StartTime:   0,
		EndTime:     int64(2 * time.Second),
		Aggregates: []mts.AggregateSpec{
			{Field: "value", Function: "sum"},
			{Field: "value", Function: "avg"},
			{Field: "value", Function: "count"},
		},
		Window: time.Second,
	})
	if err != nil {
		return fmt.Errorf("query columns: %w", err)
	}
	return assertAggregateColumns(columns)
}

func aggregatePoints() []mts.Point {
	return []mts.Point{
		aggregatePoint(0, 1),
		aggregatePoint(int64(time.Second)-1, 2),
		aggregatePoint(int64(time.Second), 3),
	}
}

func aggregatePoint(timestamp int64, value float64) mts.Point {
	return mts.Point{
		Measurement: "aggregate",
		Timestamp:   timestamp,
		Fields:      map[string]mts.FieldValue{"value": mts.Float64Value(value)},
	}
}

func assertAggregateColumns(columns []mts.ColumnSeries) error {
	if len(columns) != 3 {
		return fmt.Errorf("aggregate column count = %d, want 3", len(columns))
	}
	checks := map[string][]float64{
		"sum(value)": {3, 3},
		"avg(value)": {1.5, 3},
	}
	for _, column := range columns {
		if column.FieldName == "count(value)" {
			if len(column.Values) != 2 || column.Values[0].Int64 != 2 || column.Values[1].Int64 != 1 {
				return fmt.Errorf("count column = %#v, want [2 1]", column.Values)
			}
			continue
		}
		want, ok := checks[column.FieldName]
		if !ok {
			return fmt.Errorf("unexpected aggregate column %q", column.FieldName)
		}
		if len(column.Values) != len(want) {
			return fmt.Errorf("%s value count = %d, want %d", column.FieldName, len(column.Values), len(want))
		}
		for index, expected := range want {
			if column.Values[index].Float64 != expected {
				return fmt.Errorf("%s[%d] = %v, want %v", column.FieldName, index, column.Values[index].Float64, expected)
			}
		}
	}
	return nil
}
