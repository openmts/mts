package wal

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openmts/mts/internal/model"
)

type concurrentBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *concurrentBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *concurrentBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestNopHandlerEnabled(t *testing.T) {
	handler := nopHandler{}
	if handler.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("nopHandler.Enabled should return false for all levels")
	}
}

func TestNopHandlerHandleDirectCall(t *testing.T) {
	handler := nopHandler{}
	record := slog.Record{}
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle should return nil, got %v", err)
	}
}

func TestNopHandlerWithAttrsAndGroup(t *testing.T) {
	handler := nopHandler{}
	derived := handler.WithAttrs([]slog.Attr{slog.String("k", "v")})
	if _, ok := derived.(nopHandler); !ok {
		t.Fatal("WithAttrs should return nopHandler")
	}
	grouped := handler.WithGroup("group")
	if _, ok := grouped.(nopHandler); !ok {
		t.Fatal("WithGroup should return nopHandler")
	}
}

func TestNopLogger(t *testing.T) {
	logger := nopLogger()
	if logger.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("nopLogger should be disabled for all levels")
	}
}

// TestOpenInitializesLogger 验证 Open 后 Log.logger 为非 nil 的 nopLogger。
func TestOpenInitializesLogger(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "wal")
	log, err := Open(dir, Options{
		Sync:          true,
		BatchInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if cerr := log.Close(); cerr != nil {
			t.Fatalf("Close() error = %v", cerr)
		}
	}()
	if log.logger == nil {
		t.Fatal("WAL logger should not be nil after Open")
	}
	if log.logger.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("default WAL logger should be disabled (nopLogger)")
	}
}

// TestWALOpenWithCustomLogger 验证可通过 Options.Logger 注入自定义 logger。
func TestWALOpenWithCustomLogger(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "wal")
	var buf concurrentBuffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	log, err := Open(dir, Options{
		BatchInterval: time.Hour,
		Logger:        logger,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if cerr := log.Close(); cerr != nil {
			t.Fatalf("Close() error = %v", cerr)
		}
	}()
	if log.logger != logger {
		t.Fatal("WAL should use the provided logger")
	}
	if !log.logger.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("provided logger should be enabled for warn level")
	}
}

// TestIntervalSyncLoopErrorLogs 验证 intervalSyncLoop 在 FlushPending 失败时记录 WARN 日志。
func TestIntervalSyncLoopErrorLogs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "wal")
	var buf concurrentBuffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))
	log, err := Open(dir, Options{
		Sync:          false,
		BatchInterval: 10 * time.Millisecond,
		Logger:        logger,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	record := model.ResolvedPoint{
		Measurement: "cpu",
		Timestamp:   1,
		Fields: []model.ResolvedField{{
			FieldName: "v",
			Type:      model.FieldFloat64,
			Value:     model.Float64Value(1),
		}},
	}
	if err := log.Append([]model.ResolvedPoint{record}, false); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	// 关闭底层文件以触发周期同步失败
	log.mu.Lock()
	if log.file != nil {
		_ = log.file.Close()
	}
	log.mu.Unlock()
	// 等待周期同步触发并记录失败日志
	waitForWALTest(t, time.Second, func() bool {
		return strings.Contains(buf.String(), "wal interval sync failed")
	})
	// 文件已被手动关闭，Close 预期返回 "file already closed" 错误，此处忽略
	_ = log.Close()
}
