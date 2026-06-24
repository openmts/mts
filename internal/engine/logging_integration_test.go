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
