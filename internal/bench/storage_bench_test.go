package bench

import (
	"context"
	"fmt"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

var benchmarkRowSink mts.Row

var benchmarkColumnSink mts.ColumnSeries

func BenchmarkEngineWriteBatch(b *testing.B) {
	benchmarkEngineWrite(b, makeBenchPoints)
}

func BenchmarkEngineWriteWideBatch(b *testing.B) {
	benchmarkEngineWriteWide(b, makeWideBenchPoints)
}

func benchmarkEngineWriteWide(
	b *testing.B,
	makePoints func(count int, series int) []mts.Point,
) {
	ctx := context.Background()
	for _, points := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("points=%d", points), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				dir := b.TempDir()
				eng, err := mts.Open(ctx, mts.Options{
					Path:               dir,
					ShardDuration:      time.Hour,
					MemTableMaxSamples: points*10 + 1,
				})
				if err != nil {
					b.Fatalf("Open() error = %v", err)
				}
				if err := eng.Write(ctx, makePoints(points, 100), mts.WriteOptions{Sync: true}); err != nil {
					closeErr := eng.Close(ctx)
					b.Fatalf("Write() error = %v close = %v", err, closeErr)
				}
				if err := eng.Close(ctx); err != nil {
					b.Fatalf("Close() error = %v", err)
				}
			}
		})
	}
}

func BenchmarkEngineWriteTypedWideBatch(b *testing.B) {
	benchmarkEngineWriteTyped(b, makeWideBenchTypedBatch)
}

// BenchmarkEngineWritePointsAsTypedWideBatch 对比：[]Point -> PointsToTypedBatch -> WriteTypedBatch。
// 与 WriteWide / TypedWide 使用同一 wide 字段布局与 MemTable 策略。
func BenchmarkEngineWritePointsAsTypedWideBatch(b *testing.B) {
	benchmarkEngineWritePointsAsTypedWide(b, makeWideBenchPoints)
}

func benchmarkEngineWritePointsAsTypedWide(
	b *testing.B,
	makePoints func(count int, series int) []mts.Point,
) {
	ctx := context.Background()
	for _, points := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("points=%d", points), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				dir := b.TempDir()
				eng, err := mts.Open(ctx, mts.Options{
					Path:               dir,
					ShardDuration:      time.Hour,
					MemTableMaxSamples: points*10 + 1,
				})
				if err != nil {
					b.Fatalf("Open() error = %v", err)
				}
				if err := eng.WritePointsAsTypedBatch(ctx, makePoints(points, 100), mts.WriteOptions{Sync: true}); err != nil {
					closeErr := eng.Close(ctx)
					b.Fatalf("WritePointsAsTypedBatch() error = %v close = %v", err, closeErr)
				}
				if err := eng.Close(ctx); err != nil {
					b.Fatalf("Close() error = %v", err)
				}
			}
		})
	}
}

func benchmarkEngineWriteTyped(
	b *testing.B,
	makeBatch func(count int, series int) mts.TypedBatch,
) {
	ctx := context.Background()
	for _, points := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("points=%d", points), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				dir := b.TempDir()
				eng, err := mts.Open(ctx, mts.Options{
					Path:               dir,
					ShardDuration:      time.Hour,
					MemTableMaxSamples: points*10 + 1, // wide10: 1 point ~= 10 samples
				})
				if err != nil {
					b.Fatalf("Open() error = %v", err)
				}
				if err := eng.WriteTypedBatch(ctx, makeBatch(points, 100), mts.WriteOptions{Sync: true}); err != nil {
					closeErr := eng.Close(ctx)
					b.Fatalf("WriteTypedBatch() error = %v close = %v", err, closeErr)
				}
				if err := eng.Close(ctx); err != nil {
					b.Fatalf("Close() error = %v", err)
				}
			}
		})
	}
}

func BenchmarkEngineQueryRowIterator(b *testing.B) {
	for _, points := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("points=%d", points), func(b *testing.B) {
			ctx := context.Background()
			eng, query := prepareQueryBenchmark(b, points)
			b.ReportAllocs()
			for b.Loop() {
				rows := consumeRowIterator(b, ctx, eng, query)
				if rows != points {
					b.Fatalf("row iterator rows = %d, want %d", rows, points)
				}
			}
		})
	}
}

func BenchmarkEngineQueryColumnIterator(b *testing.B) {
	for _, points := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("points=%d", points), func(b *testing.B) {
			ctx := context.Background()
			eng, query := prepareQueryBenchmark(b, points)
			b.ReportAllocs()
			for b.Loop() {
				values := consumeColumnIterator(b, ctx, eng, query)
				if values != points {
					b.Fatalf("column iterator values = %d, want %d", values, points)
				}
			}
		})
	}
}

func benchmarkEngineWrite(
	b *testing.B,
	makePoints func(count int, series int) []mts.Point,
) {
	ctx := context.Background()
	for _, points := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("points=%d", points), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				dir := b.TempDir()
				eng, err := mts.Open(ctx, mts.Options{
					Path:               dir,
					ShardDuration:      time.Hour,
					MemTableMaxSamples: points + 1,
				})
				if err != nil {
					b.Fatalf("Open() error = %v", err)
				}
				if err := eng.Write(ctx, makePoints(points, 100), mts.WriteOptions{Sync: true}); err != nil {
					closeErr := eng.Close(ctx)
					b.Fatalf("Write() error = %v close = %v", err, closeErr)
				}
				if err := eng.Close(ctx); err != nil {
					b.Fatalf("Close() error = %v", err)
				}
			}
		})
	}
}

func prepareQueryBenchmark(b *testing.B, points int) (*mts.Engine, mts.Query) {
	b.Helper()
	ctx := context.Background()
	eng, err := mts.Open(ctx, mts.Options{
		Path:               b.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: points + 1,
	})
	if err != nil {
		b.Fatalf("Open() error = %v", err)
	}
	b.Cleanup(func() {
		if err := eng.Close(ctx); err != nil {
			b.Errorf("Close() error = %v", err)
		}
	})
	if err := eng.WriteTypedBatch(ctx, makeBenchTypedBatch(points, 100), mts.WriteOptions{Sync: true}); err != nil {
		b.Fatalf("WriteTypedBatch() error = %v", err)
	}
	if err := eng.Flush(ctx); err != nil {
		b.Fatalf("Flush() error = %v", err)
	}
	query, err := mts.NewQuery().
		Select("value").
		From("", "", "bench").
		TimeRange(0, int64(points-1)).
		Build()
	if err != nil {
		b.Fatalf("Build() error = %v", err)
	}
	return eng, query
}

func consumeRowIterator(b *testing.B, ctx context.Context, eng *mts.Engine, query mts.Query) int {
	b.Helper()
	iter, err := eng.QueryRowIterator(ctx, query)
	if err != nil {
		b.Fatalf("QueryRowIterator() error = %v", err)
	}
	rows := 0
	for iter.Next() {
		benchmarkRowSink = iter.Row()
		rows++
	}
	if err := iter.Err(); err != nil {
		closeErr := iter.Close()
		b.Fatalf("row iterator error = %v close = %v", err, closeErr)
	}
	if err := iter.Close(); err != nil {
		b.Fatalf("row iterator close error = %v", err)
	}
	return rows
}

func consumeColumnIterator(b *testing.B, ctx context.Context, eng *mts.Engine, query mts.Query) int {
	b.Helper()
	iter, err := eng.QueryColumnIterator(ctx, query)
	if err != nil {
		b.Fatalf("QueryColumnIterator() error = %v", err)
	}
	values := 0
	for iter.Next() {
		column := iter.Column()
		benchmarkColumnSink = column
		values += len(column.Values)
	}
	if err := iter.Err(); err != nil {
		closeErr := iter.Close()
		b.Fatalf("column iterator error = %v close = %v", err, closeErr)
	}
	if err := iter.Close(); err != nil {
		b.Fatalf("column iterator close error = %v", err)
	}
	return values
}

func makeBenchPoints(count int, series int) []mts.Point {
	points := make([]mts.Point, 0, count)
	for index := range count {
		host := fmt.Sprintf("host-%04d", index%series)
		points = append(points, mts.Point{
			Measurement: "bench",
			Tags:        map[string]string{"host": host},
			Timestamp:   int64(index),
			Fields: map[string]mts.FieldValue{
				"value":  mts.Float64Value(float64(index)),
				"count":  mts.Int64Value(int64(index)),
				"active": mts.BoolValue(index%2 == 0),
				"state":  mts.StringValue("ok"),
			},
		})
	}
	return points
}

func makeBenchTypedBatch(count int, series int) mts.TypedBatch {
	hosts := make([]string, 0, count)
	timestamps := make([]int64, 0, count)
	values := make([]float64, 0, count)
	for index := range count {
		hosts = append(hosts, fmt.Sprintf("host-%04d", index%series))
		timestamps = append(timestamps, int64(index))
		values = append(values, float64(index))
	}
	return mts.TypedBatch{
		Measurement: "bench",
		Tags: []mts.TagColumn{
			{Name: "host", Values: hosts},
		},
		Timestamps: timestamps,
		Fields: []mts.TypedFieldColumn{
			{Name: "value", Type: mts.FieldFloat64, Float64Values: values},
		},
	}
}

func makeWideBenchPoints(count int, series int) []mts.Point {
	points := make([]mts.Point, 0, count)
	for index := range count {
		host := fmt.Sprintf("host-%04d", index%series)
		points = append(points, mts.Point{
			Measurement: "bench",
			Tags:        map[string]string{"host": host},
			Timestamp:   int64(index),
			Fields: map[string]mts.FieldValue{
				"active": mts.BoolValue(index%2 == 0),
				"f0":     mts.Float64Value(float64(index)),
				"f1":     mts.Float64Value(float64(index) + 0.1),
				"f2":     mts.Float64Value(float64(index) + 0.2),
				"f3":     mts.Float64Value(float64(index) + 0.3),
				"f4":     mts.Float64Value(float64(index) + 0.4),
				"i0":     mts.Int64Value(int64(index)),
				"i1":     mts.Int64Value(int64(index % series)),
				"i2":     mts.Int64Value(int64(index % 86400)),
				"state":  mts.StringValue("ok"),
			},
		})
	}
	return points
}

func makeWideBenchTypedBatch(count int, series int) mts.TypedBatch {
	hosts := make([]string, count)
	timestamps := make([]int64, count)
	active := make([]bool, count)
	f0 := make([]float64, count)
	f1 := make([]float64, count)
	f2 := make([]float64, count)
	f3 := make([]float64, count)
	f4 := make([]float64, count)
	i0 := make([]int64, count)
	i1 := make([]int64, count)
	i2 := make([]int64, count)
	state := make([]string, count)
	for index := range count {
		hosts[index] = fmt.Sprintf("host-%04d", index%series)
		timestamps[index] = int64(index)
		active[index] = index%2 == 0
		f0[index] = float64(index)
		f1[index] = float64(index) + 0.1
		f2[index] = float64(index) + 0.2
		f3[index] = float64(index) + 0.3
		f4[index] = float64(index) + 0.4
		i0[index] = int64(index)
		i1[index] = int64(index % series)
		i2[index] = int64(index % 86400)
		state[index] = "ok"
	}
	return mts.TypedBatch{
		Measurement: "bench",
		Tags: []mts.TagColumn{
			{Name: "host", Values: hosts},
		},
		Timestamps: timestamps,
		Fields: []mts.TypedFieldColumn{
			{Name: "active", Type: mts.FieldBool, BoolValues: active},
			{Name: "f0", Type: mts.FieldFloat64, Float64Values: f0},
			{Name: "f1", Type: mts.FieldFloat64, Float64Values: f1},
			{Name: "f2", Type: mts.FieldFloat64, Float64Values: f2},
			{Name: "f3", Type: mts.FieldFloat64, Float64Values: f3},
			{Name: "f4", Type: mts.FieldFloat64, Float64Values: f4},
			{Name: "i0", Type: mts.FieldInt64, Int64Values: i0},
			{Name: "i1", Type: mts.FieldInt64, Int64Values: i1},
			{Name: "i2", Type: mts.FieldInt64, Int64Values: i2},
			{Name: "state", Type: mts.FieldString, StringValues: state},
		},
	}
}
