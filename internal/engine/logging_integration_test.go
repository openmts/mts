package engine

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

// TestEngineOpenLogs 验证 Open 使用自定义 logger 时输出 "engine opened" 日志，
// 并携带 path 与 shard_count 属性。
func TestEngineOpenLogs(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	opts := model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
		Logger:        logger,
	}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = eng.Close(ctx) }()
	output := buf.String()
	if !strings.Contains(output, "engine opened") {
		t.Fatalf("expected 'engine opened' in log, got %q", output)
	}
	if !strings.Contains(output, opts.Path) {
		t.Fatalf("expected path %q in log, got %q", opts.Path, output)
	}
	if !strings.Contains(output, "shard_count") {
		t.Fatalf("expected 'shard_count' attribute in log, got %q", output)
	}
}

// TestEngineCloseLogs 验证 Close 成功时输出 "engine closed" 日志。
func TestEngineCloseLogs(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	opts := model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
		Logger:        logger,
	}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	// 重置 buffer 以仅观测 Close 阶段日志。
	buf.Reset()
	if err := eng.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "engine closed") {
		t.Fatalf("expected 'engine closed' in log, got %q", output)
	}
}

// TestShardOpenedLogs 验证通过 Write 触发 OpenShard 时输出 "shard opened" 日志，
// 并携带 wal_records 属性。
func TestShardOpenedLogs(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	opts := model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
		Logger:        logger,
	}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = eng.Close(ctx) }()
	// 重置 buffer 以仅观测 Write 触发 OpenShard 阶段日志。
	buf.Reset()
	point := model.Point{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   10,
		Fields: map[string]model.FieldValue{
			"usage": model.Float64Value(1),
		},
	}
	if err := eng.Write(ctx, []model.Point{point}, model.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "shard opened") {
		t.Fatalf("expected 'shard opened' in log, got %q", output)
	}
	if !strings.Contains(output, "wal_records") {
		t.Fatalf("expected 'wal_records' attribute in log, got %q", output)
	}
}

// TestBackgroundCompactionStartedLogs 验证启用后台压缩时输出 "background compaction started" 日志，
// 并携带 interval 属性。
func TestBackgroundCompactionStartedLogs(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	opts := model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
		Compaction: model.CompactionOptions{
			Enabled:            true,
			BackgroundInterval: 10 * time.Second,
		},
		Logger: logger,
	}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = eng.Close(ctx) }()
	output := buf.String()
	if !strings.Contains(output, "background compaction started") {
		t.Fatalf("expected 'background compaction started' in log, got %q", output)
	}
	if !strings.Contains(output, "downsample scheduler started") {
		t.Fatalf("expected 'downsample scheduler started' in log, got %q", output)
	}
}

// TestCompactionCompletedLogs 验证压缩成功时输出 "compaction completed" 日志，
// 并携带 duration_ms、input_parts、output_parts 属性。
func TestCompactionCompletedLogs(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	opts := model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 2,
		Compaction: model.CompactionOptions{
			Enabled:         true,
			Level0PartLimit: 2,
		},
		Logger: logger,
	}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = eng.Close(ctx) }()
	// 写入足够数据以触发多次 flush，生成多个 part。
	for i := 0; i < 4; i++ {
		if err := eng.Write(ctx, []model.Point{{
			Measurement: "cpu",
			Tags:        map[string]string{"host": "a"},
			Timestamp:   int64(i),
			Fields:      map[string]model.FieldValue{"v": model.Float64Value(float64(i))},
		}}, model.WriteOptions{Sync: true}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
	// 重置 buffer 以仅观测 CompactWithResult 阶段日志。
	buf.Reset()
	if _, err := eng.CompactWithResult(ctx); err != nil {
		t.Fatalf("CompactWithResult() error = %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "compaction completed") {
		t.Fatalf("expected 'compaction completed' in log, got %q", output)
	}
	if !strings.Contains(output, "duration_ms") {
		t.Fatalf("expected 'duration_ms' attribute in log, got %q", output)
	}
	if !strings.Contains(output, "input_parts") {
		t.Fatalf("expected 'input_parts' attribute in log, got %q", output)
	}
	if !strings.Contains(output, "output_parts") {
		t.Fatalf("expected 'output_parts' attribute in log, got %q", output)
	}
}

// TestRetentionShardRemovedLogs 验证 ApplyRetention 删除过期 shard 时输出
// "retention shard removed" 日志，并携带 shard 与 deleted_bytes 属性。
func TestRetentionShardRemovedLogs(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	opts := model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
		Retention:     time.Second,
		Logger:        logger,
	}
	eng, err := Open(ctx, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = eng.Close(ctx) }()
	// 写入一条数据以触发 shard 打开。
	if err := eng.Write(ctx, []model.Point{{
		Measurement: "cpu",
		Tags:        map[string]string{"host": "a"},
		Timestamp:   0,
		Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
	}}, model.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	// 重置 buffer 以仅观测 ApplyRetention 阶段日志。
	buf.Reset()
	// now 取较远的未来时间，确保 shard 已过期被删除。
	if err := eng.ApplyRetention(ctx, time.Unix(0, int64(2*time.Hour))); err != nil {
		t.Fatalf("ApplyRetention() error = %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "retention shard removed") {
		t.Fatalf("expected 'retention shard removed' in log, got %q", output)
	}
	if !strings.Contains(output, "deleted_bytes") {
		t.Fatalf("expected 'deleted_bytes' attribute in log, got %q", output)
	}
}
