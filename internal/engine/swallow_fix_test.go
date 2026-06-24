package engine

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openmts/mts/internal/faultinject"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/storagefs"
)

// newCapturingLogger 创建一个将日志写入 buf 的 slog.Logger，级别为 WARN。
func newCapturingLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))
}

// TestBackgroundCompactionLoopLogsFailure 验证后台压缩失败时输出 WARN 日志。
// 通过保留未刷盘的 memtable 并注入 OpCreate 故障，使 compactBackground 返回错误。
func TestBackgroundCompactionLoopLogsFailure(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	logger := newCapturingLogger(&buf)
	eng, err := Open(ctx, model.Options{
		Path:               t.TempDir(),
		ShardDuration:      time.Hour,
		MemTableMaxSamples: 100,
		Compaction: model.CompactionOptions{
			Enabled:            true,
			Level0PartLimit:    100,
			BackgroundInterval: 10 * time.Millisecond,
		},
		Logger: logger,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		_ = eng.Close(ctx)
	}()
	if err := eng.Write(ctx, []model.Point{{
		Measurement: "bg",
		Timestamp:   1,
		Fields:      map[string]model.FieldValue{"v": model.Float64Value(1)},
	}}, model.WriteOptions{Sync: true}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	fs := faultinject.NewFS()
	fs.Fail(faultinject.OpCreate, errors.New("injected create fault"))
	restore := storagefs.SetFaultController(fs)
	defer restore()
	waitForTest(t, 3*time.Second, func() bool {
		return strings.Contains(buf.String(), "background compaction failed")
	})
}

// TestStartDownsamplePolicyRunLogsFailure 验证降采样策略执行失败时输出 WARN 日志。
// 通过注入 OpWrite 故障，使 RunDownsamplePolicy 写入降采样结果时失败。
func TestStartDownsamplePolicyRunLogsFailure(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	logger := newCapturingLogger(&buf)
	eng, err := Open(ctx, model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
		Logger:        logger,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		_ = eng.Close(ctx)
	}()
	points := make([]model.Point, 0, 5)
	for minute := int64(0); minute < 5; minute++ {
		points = append(points, model.Point{
			Database:        "metrics",
			RetentionPolicy: "autogen",
			Measurement:     "cpu",
			Tags:            map[string]string{"host": "a"},
			Timestamp:       minute * int64(time.Minute),
			Fields:          map[string]model.FieldValue{"usage": model.Float64Value(float64(minute))},
		})
	}
	if err := eng.Write(ctx, points, model.WriteOptions{}); err != nil {
		t.Fatalf("Write(raw) error = %v", err)
	}
	if err := eng.CreateDownsamplePolicy(ctx, testDownsamplePolicy()); err != nil {
		t.Fatalf("CreateDownsamplePolicy() error = %v", err)
	}
	buf.Reset()
	fs := faultinject.NewFS()
	fs.Fail(faultinject.OpWrite, errors.New("injected write fault"))
	restore := storagefs.SetFaultController(fs)
	defer restore()
	waitForTest(t, 5*time.Second, func() bool {
		return strings.Contains(buf.String(), "downsample policy run failed")
	})
}

// flakyContext 在第二次及之后调用 Err 时返回 context.Canceled，
// 用于触发 scanDownsamplePolicies 内部 ListDownsamplePolicies 的错误分支。
type flakyContext struct {
	calls int32
}

func (c *flakyContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *flakyContext) Done() <-chan struct{}       { return nil }
func (c *flakyContext) Err() error {
	if atomic.AddInt32(&c.calls, 1) >= 2 {
		return context.Canceled
	}
	return nil
}
func (c *flakyContext) Value(_ any) any { return nil }

// TestShouldRunDownsamplePolicyLogsWatermarkError 验证读取降采样水位失败时输出 WARN 日志。
// shouldRunDownsamplePolicy 无顶层 ctx.Err 检查，直接传入已取消 context 即可触发错误分支。
func TestShouldRunDownsamplePolicyLogsWatermarkError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var buf bytes.Buffer
	eng, err := Open(context.Background(), model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
		Logger:        newCapturingLogger(&buf),
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = eng.Close(context.Background()) }()
	if eng.shouldRunDownsamplePolicy(ctx, testDownsamplePolicy()) {
		t.Fatal("shouldRunDownsamplePolicy() = true, want false on cancelled context")
	}
	if !strings.Contains(buf.String(), "read downsample watermark failed") {
		t.Fatalf("logs = %q, want read downsample watermark failed", buf.String())
	}
}

// TestScanDownsamplePoliciesLogsListError 验证列出降采样策略失败时输出 WARN 日志。
// 使用 flakyContext 使 scanDownsamplePolicies 顶层检查通过但 ListDownsamplePolicies 失败。
func TestScanDownsamplePoliciesLogsListError(t *testing.T) {
	var buf bytes.Buffer
	eng, err := Open(context.Background(), model.Options{
		Path:          t.TempDir(),
		ShardDuration: time.Hour,
		Logger:        newCapturingLogger(&buf),
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = eng.Close(context.Background()) }()
	eng.scanDownsamplePolicies(&flakyContext{})
	if !strings.Contains(buf.String(), "list downsample policies failed") {
		t.Fatalf("logs = %q, want list downsample policies failed", buf.String())
	}
}
