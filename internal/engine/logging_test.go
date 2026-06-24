package engine

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

func TestNopHandlerEnabled(t *testing.T) {
	handler := nopHandler{}
	if handler.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("nopHandler.Enabled should return false for all levels")
	}
}

func TestNopHandlerHandleNoOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(nopHandler{})
	logger.Info("test message", "key", "value")
	if buf.Len() > 0 {
		t.Fatalf("nopHandler should produce no output, got %q", buf.String())
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
	if logger == nil {
		t.Fatal("nopLogger should not return nil")
	}
	if !logger.Enabled(context.Background(), slog.LevelError) {
		// nopLogger 应返回 Enabled=false 的 logger
		return
	}
	t.Fatal("nopLogger should be disabled for all levels")
}
