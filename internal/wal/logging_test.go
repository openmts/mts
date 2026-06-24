package wal

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
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
	var buf bytes.Buffer
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
