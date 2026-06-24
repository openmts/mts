package engine

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/openmts/mts/internal/model"
)

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
		t.Fatalf("nopHandler.Handle() error = %v, want nil", err)
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

func TestNopLoggerIsDefault(t *testing.T) {
	logger := nopLogger()
	if logger.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("nopLogger should be disabled for all levels")
	}
}

// TestNormalizeOptionsWALLoggerInheritsEngineLogger 验证未显式设置 WAL logger 时
// 继承 engine logger。
func TestNormalizeOptionsWALLoggerInheritsEngineLogger(t *testing.T) {
	engineLogger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	opts := normalizeOptions(model.Options{Logger: engineLogger})
	if opts.WAL.Logger != engineLogger {
		t.Fatal("WAL logger should inherit engine logger when not set")
	}
}

// TestNormalizeOptionsWALLoggerPreservesCustomLogger 验证显式设置的 WAL logger
// 不会被 engine logger 覆盖。
func TestNormalizeOptionsWALLoggerPreservesCustomLogger(t *testing.T) {
	engineLogger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	walLogger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	opts := normalizeOptions(model.Options{
		Logger: engineLogger,
		WAL:    model.WALOptions{Logger: walLogger},
	})
	if opts.WAL.Logger != walLogger {
		t.Fatal("WAL logger should preserve custom logger when set")
	}
}

// TestNormalizeOptionsWALLoggerDefaultsToNopWhenEngineLoggerAbsent 验证
// engine logger 与 WAL logger 均未设置时 WAL logger 回退到 nopLogger。
func TestNormalizeOptionsWALLoggerDefaultsToNopWhenEngineLoggerAbsent(t *testing.T) {
	opts := normalizeOptions(model.Options{})
	if opts.WAL.Logger == nil {
		t.Fatal("WAL logger should not be nil after normalize")
	}
	if opts.WAL.Logger.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("WAL logger should be nopLogger when neither engine nor WAL logger set")
	}
}
