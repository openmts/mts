package mts_test

import (
	"context"
	"fmt"
	"os"
	"time"

	mts "github.com/openmts/mts"
)

func ExampleOpen() {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "mts-example-*")
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = os.RemoveAll(dir)
	}()

	engine, err := mts.Open(ctx, mts.DefaultOptions(dir))
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = engine.Close(ctx)
	}()

	err = engine.Write(ctx, []mts.Point{{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   time.Unix(0, 10).UnixNano(),
		Fields: map[string]mts.FieldValue{
			"usage": mts.Float64Value(0.42),
		},
	}}, mts.WriteOptions{Sync: true})
	if err != nil {
		panic(err)
	}

	query, err := mts.NewQuery().
		From("", "", "cpu").
		Where(mts.TagEq("host", "a")).
		TimeRange(0, time.Second.Nanoseconds()).
		Build()
	if err != nil {
		panic(err)
	}
	rows, err := engine.QueryRows(ctx, query)
	if err != nil {
		panic(err)
	}

	fmt.Printf("rows=%d usage=%.2f\n", len(rows), rows[0].Fields["usage"].Float64)

	// Output:
	// rows=1 usage=0.42
}

func ExampleNewQuery() {
	query, err := mts.NewQuery().
		Select("usage").
		From("metrics", "autogen", "cpu").
		Where(mts.TagEq("host", "a")).
		Aggregate("mean", "usage").
		GroupByTime(time.Minute).
		OrderByTimeDesc().
		Limit(100).
		Build()
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s/%s/%s fields=%d aggregate=%s limit=%d\n",
		query.Database,
		query.RetentionPolicy,
		query.Measurement,
		len(query.Fields),
		query.Aggregates[0].Function,
		query.Limit,
	)

	// Output:
	// metrics/autogen/cpu fields=1 aggregate=avg limit=100
}

func ExampleEngine_CreateDownsamplePolicy() {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "mts-downsample-example-*")
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = os.RemoveAll(dir)
	}()

	engine, err := mts.Open(ctx, mts.DefaultOptions(dir))
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = engine.Close(ctx)
	}()

	policy := mts.DownsamplePolicy{
		Name:              "cpu_1m",
		SourceDatabase:    "default",
		SourceRetention:   "autogen",
		SourceMeasurement: "cpu",
		TargetDatabase:    "default",
		TargetRetention:   "autogen",
		TargetMeasurement: "cpu_1m",
		Interval:          time.Minute,
		RefreshInterval:   time.Minute,
		Functions: []mts.DownsampleFunction{
			{Function: mts.AggregateAvg, Field: "usage", As: "avg_usage"},
		},
		GroupByTags: []string{"host"},
		Enabled:     true,
	}
	if err := engine.CreateDownsamplePolicy(ctx, policy); err != nil {
		panic(err)
	}
	policies, err := engine.ListDownsamplePolicies(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Printf("policies=%d\n", len(policies))

	// Output:
	// policies=1
}
