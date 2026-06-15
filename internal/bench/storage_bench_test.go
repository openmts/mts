package bench

import (
	"context"
	"fmt"
	"testing"
	"time"

	mts "codeberg.org/mts/mts"
)

func BenchmarkEngineWriteBatch(b *testing.B) {
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
				if err := eng.Write(ctx, makeBenchPoints(points, 100), mts.WriteOptions{Sync: true}); err != nil {
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
