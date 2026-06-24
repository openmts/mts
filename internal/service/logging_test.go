package service

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/openmts/mts/internal/observability"
)

func TestNopServiceHandlerEnabled(t *testing.T) {
	handler := nopServiceHandler{}
	if handler.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("nopServiceHandler.Enabled should return false for all levels")
	}
}

func TestNopServiceHandlerHandleDirectCall(t *testing.T) {
	handler := nopServiceHandler{}
	record := slog.Record{}
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle should return nil, got %v", err)
	}
}

func TestNopServiceHandlerWithAttrsAndGroup(t *testing.T) {
	handler := nopServiceHandler{}
	derived := handler.WithAttrs([]slog.Attr{slog.String("k", "v")})
	if _, ok := derived.(nopServiceHandler); !ok {
		t.Fatal("WithAttrs should return nopServiceHandler")
	}
	grouped := handler.WithGroup("group")
	if _, ok := grouped.(nopServiceHandler); !ok {
		t.Fatal("WithGroup should return nopServiceHandler")
	}
}

// TestServerNilLoggerNormalized 验证 NewServer 将 nil Logger 归一化为 nopServiceHandler。
func TestServerNilLoggerNormalized(t *testing.T) {
	registry := observability.NewRegistry()
	ops := &fakeOps{registry: registry}
	server := NewServer(Options{Addr: "127.0.0.1:0"}, ops, ops, nil)
	if server.logger == nil {
		t.Fatal("NewServer should normalize nil logger to nopServiceHandler")
	}
	if server.logger.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("default server logger should be disabled")
	}
}

// TestServerStartLogsListening 验证 Start 使用自定义 logger 输出 "service listening" 日志。
func TestServerStartLogsListening(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("Close(listener) error = %v", err)
	}
	registry := observability.NewRegistry()
	ops := &fakeOps{registry: registry}
	server := NewServer(Options{Addr: addr, Logger: logger}, ops, ops, nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "service listening") {
		t.Fatalf("logs = %q, want service listening", output)
	}
	if !strings.Contains(output, "service shutdown") {
		t.Fatalf("logs = %q, want service shutdown", output)
	}
	if !strings.Contains(output, addr) {
		t.Fatalf("logs = %q, want addr %s", output, addr)
	}
}

// TestServerStartReturnsListenError 验证监听失败时返回错误。
func TestServerStartReturnsListenError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	addr := listener.Addr().String()
	defer func() {
		if cerr := listener.Close(); cerr != nil {
			t.Fatalf("Close(listener) error = %v", cerr)
		}
	}()
	server := NewServer(Options{Addr: addr}, nil, nil, nil)
	if err := server.Start(); err == nil {
		_ = server.Shutdown(context.Background())
		t.Fatal("Start() on occupied port should return error")
	}
}

// TestServerCustomLoggerUsed 验证自定义 logger 被保留而非覆盖。
func TestServerCustomLoggerUsed(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	registry := observability.NewRegistry()
	ops := &fakeOps{registry: registry}
	server := NewServer(Options{Addr: "127.0.0.1:0", Logger: logger}, ops, ops, nil)
	if server.logger != logger {
		t.Fatal("NewServer should preserve non-nil custom logger")
	}
}

// TestServerShutdownLogsViaCustomLogger 验证 Shutdown 走自定义 logger。
func TestServerShutdownLogsViaCustomLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("Close(listener) error = %v", err)
	}
	server := NewServer(Options{Addr: addr, Logger: logger}, nil, nil, nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if !strings.Contains(buf.String(), "service shutdown") {
		t.Fatalf("logs = %q, want service shutdown", buf.String())
	}
}
