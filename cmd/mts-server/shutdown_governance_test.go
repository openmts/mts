package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRuntimeStartRollbackUsesConfiguredShutdownTimeout(t *testing.T) {
	grpcListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen(gRPC blocker) error = %v", err)
	}
	t.Cleanup(func() {
		if err := grpcListener.Close(); err != nil {
			t.Fatalf("Close(gRPC blocker) error = %v", err)
		}
	})

	runtime := openShutdownTestRuntime(t, grpcListener.Addr().String())
	request := startBlockingHTTP(t, runtime.httpServer, "127.0.0.1:0", true)
	releaseTimer := time.AfterFunc(300*time.Millisecond, request.release)
	defer releaseTimer.Stop()
	start := time.Now()
	startErr := runtime.start()
	elapsed := time.Since(start)
	request.release()
	if startErr == nil {
		t.Fatal("start() error = nil, want gRPC listener error")
	}
	assertShutdownBudget(t, elapsed, "start rollback")
	request.wait(t)
}

func TestServeRuntimeUsesConfiguredShutdownTimeoutAfterServeError(t *testing.T) {
	runtime := openStartedShutdownTestRuntime(t)
	request := startBlockingHTTP(t, runtime.httpServer, runtime.httpLn.Addr().String(), false)
	if err := runtime.httpLn.Close(); err != nil {
		t.Fatalf("Close(HTTP listener) error = %v", err)
	}

	start := time.Now()
	serveErr := serveRuntime(context.Background(), runtime, slog.Default(), nil)
	elapsed := time.Since(start)
	request.release()
	if serveErr == nil {
		t.Fatal("serveRuntime() error = nil, want Serve error")
	}
	assertShutdownBudget(t, elapsed, "Serve error shutdown")
	request.wait(t)
}

func TestServeRuntimeSignalUsesConfiguredShutdownTimeout(t *testing.T) {
	runtime := openStartedShutdownTestRuntime(t)
	request := startBlockingHTTP(t, runtime.httpServer, runtime.httpLn.Addr().String(), false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	serveErr := serveRuntime(ctx, runtime, slog.Default(), nil)
	elapsed := time.Since(start)
	request.release()
	if !errors.Is(serveErr, context.DeadlineExceeded) {
		t.Fatalf("serveRuntime(signal) error = %v, want deadline exceeded", serveErr)
	}
	assertShutdownBudget(t, elapsed, "signal shutdown")
	request.wait(t)
}

func TestServeRuntimeHandlesSIGHUPReloadResult(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*testing.T, *serverRuntime)
		message string
	}{
		{
			name: "success",
			prepare: func(t *testing.T, runtime *serverRuntime) {
				runtime.config.ConfigPath = writeRuntimeConfig(t, runtime.currentConfig())
			},
			message: "config reloaded",
		},
		{
			name: "failure",
			prepare: func(t *testing.T, runtime *serverRuntime) {
				runtime.config.ConfigPath = t.TempDir() + "/missing.yaml"
			},
			message: "reload config failed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtime := openShutdownTestRuntime(t, "127.0.0.1:0")
			runtime.config.GRPC.Enabled = false
			test.prepare(t, runtime)
			var output bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&output, nil))
			sighup := make(chan os.Signal, 1)
			sighup <- syscall.SIGHUP
			ctx, cancel := context.WithCancel(context.Background())
			timer := time.AfterFunc(30*time.Millisecond, cancel)
			serveErr := serveRuntime(ctx, runtime, logger, sighup)
			if !timer.Stop() {
				<-ctx.Done()
			}
			cancel()
			if serveErr != nil {
				t.Fatalf("serveRuntime(SIGHUP) error = %v", serveErr)
			}
			if !strings.Contains(output.String(), test.message) {
				t.Fatalf("log output = %q, want %q", output.String(), test.message)
			}
		})
	}
}

type blockingHTTPRequest struct {
	releaseRequest chan struct{}
	requestDone    chan error
	serveDone      chan error
}

func startBlockingHTTP(
	t *testing.T,
	server *http.Server,
	address string,
	serve bool,
) blockingHTTPRequest {
	t.Helper()
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{}, 1)
	server.Handler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(requestStarted)
		<-releaseRequest
	})
	var serveDone chan error
	if serve {
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Fatalf("Listen(HTTP) error = %v", err)
		}
		serveDone = make(chan error, 1)
		go func() {
			serveDone <- server.Serve(listener)
		}()
		address = listener.Addr().String()
	}
	requestDone := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + address)
		if err == nil {
			err = response.Body.Close()
		}
		requestDone <- err
	}()
	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		releaseRequest <- struct{}{}
		t.Fatal("blocking HTTP request did not start")
	}
	return blockingHTTPRequest{
		releaseRequest: releaseRequest,
		requestDone:    requestDone,
		serveDone:      serveDone,
	}
}

func (r blockingHTTPRequest) release() {
	select {
	case r.releaseRequest <- struct{}{}:
	default:
	}
}

func (r blockingHTTPRequest) wait(t *testing.T) {
	t.Helper()
	select {
	case err := <-r.requestDone:
		if err != nil {
			t.Fatalf("blocking HTTP request error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("blocking HTTP request did not finish")
	}
	if r.serveDone == nil {
		return
	}
	select {
	case err := <-r.serveDone:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Serve() error = %v, want http.ErrServerClosed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("HTTP server did not stop")
	}
}

func openShutdownTestRuntime(t *testing.T, grpcAddress string) *serverRuntime {
	t.Helper()
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.HTTP.Addr = "127.0.0.1:0"
	cfg.GRPC.Addr = grpcAddress
	cfg.Shutdown = durationText(30 * time.Millisecond)
	runtime, err := openRuntime(context.Background(), cfg)
	if err != nil {
		t.Fatalf("openRuntime() error = %v", err)
	}
	return runtime
}

func openStartedShutdownTestRuntime(t *testing.T) *serverRuntime {
	t.Helper()
	runtime := openShutdownTestRuntime(t, "127.0.0.1:0")
	runtime.config.GRPC.Enabled = false
	runtime.grpcServer = nil
	if err := runtime.start(); err != nil {
		t.Fatalf("start() error = %v", err)
	}
	return runtime
}

func assertShutdownBudget(t *testing.T, elapsed time.Duration, operation string) {
	t.Helper()
	if elapsed > 150*time.Millisecond {
		t.Fatalf("%s elapsed = %s, want <= 150ms", operation, elapsed)
	}
}
