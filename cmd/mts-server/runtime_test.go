package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mts "github.com/openmts/mts"
)

func TestRuntimeStartAndShutdown(t *testing.T) {
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.HTTP.Addr = "127.0.0.1:0"
	cfg.GRPC.Enabled = false
	runtime, err := openRuntime(context.Background(), cfg)
	if err != nil {
		t.Fatalf("openRuntime() error = %v", err)
	}
	if err := runtime.start(); err != nil {
		t.Fatalf("start() error = %v", err)
	}
	resp, err := http.Get("http://" + runtime.httpLn.Addr().String() + "/readyz")
	if err != nil {
		t.Fatalf("Get(readyz) error = %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("Close(readyz body) error = %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("readyz status = %d, want 200", resp.StatusCode)
	}
	if err := runtime.shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
}

func TestRuntimeStartWithGRPC(t *testing.T) {
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.HTTP.Enabled = false
	cfg.GRPC.Addr = "127.0.0.1:0"
	runtime, err := openRuntime(context.Background(), cfg)
	if err != nil {
		t.Fatalf("openRuntime() error = %v", err)
	}
	if err := runtime.start(); err != nil {
		t.Fatalf("start() error = %v", err)
	}
	if runtime.grpcLn == nil {
		t.Fatal("grpc listener is nil, want listener")
	}
	if err := runtime.shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
}

func TestRuntimeStartFailsWhenAddrInUse(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Fatalf("Close(listener) error = %v", err)
		}
	}()
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.HTTP.Addr = listener.Addr().String()
	cfg.GRPC.Enabled = false
	runtime, err := openRuntime(context.Background(), cfg)
	if err != nil {
		t.Fatalf("openRuntime() error = %v", err)
	}
	if err := runtime.start(); err == nil {
		t.Fatal("start() error = nil, want address in use error")
	}
	if err := runtime.shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown() error = %v", err)
	}
}

func TestRunServerStopsWhenContextCanceled(t *testing.T) {
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.HTTP.Addr = "127.0.0.1:0"
	cfg.GRPC.Enabled = false
	cfg.Shutdown = durationText(time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	if err := runServer(ctx, cfg, logger); err != nil {
		t.Fatalf("runServer(canceled) error = %v", err)
	}
}

func TestRunServerReturnsStartError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Fatalf("Close(listener) error = %v", err)
		}
	}()
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.HTTP.Addr = listener.Addr().String()
	cfg.GRPC.Enabled = false
	cfg.Shutdown = durationText(time.Second)
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	if err := runServer(context.Background(), cfg, logger); err == nil {
		t.Fatal("runServer(addr in use) error = nil, want error")
	}
}

func TestRuntimeReportsHTTPServeError(t *testing.T) {
	runtime := &serverRuntime{serveErr: make(chan error, 1)}
	server := &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})}
	listener := failingListener{err: errors.New("accept failed")}
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			runtime.reportServeError(err)
		}
	}()
	select {
	case err := <-runtime.serveErrors():
		if err == nil {
			t.Fatal("serve error = nil, want error")
		}
	case <-time.After(time.Second):
		t.Fatal("serve error not reported")
	}
}

func TestRuntimeServeErrors(t *testing.T) {
	runtime := &serverRuntime{serveErr: make(chan error, 1)}
	wantErr := errors.New("listener failed")
	runtime.reportServeError(wantErr)
	select {
	case gotErr := <-runtime.serveErrors():
		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("serve error = %v, want %v", gotErr, wantErr)
		}
	case <-time.After(time.Second):
		t.Fatal("serve error not reported")
	}
}

func TestRuntimeMethodsRespectCanceledContext(t *testing.T) {
	runtime := openTestRuntime(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runtime.write(ctx, writeRequest{Points: []mts.Point{testPoint()}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("write(canceled) error = %v, want context.Canceled", err)
	}
	if _, err := runtime.queryRows(ctx, queryRowsRequest{Query: testQuery()}); !errors.Is(err, context.Canceled) {
		t.Fatalf("queryRows(canceled) error = %v, want context.Canceled", err)
	}
	if err := runtime.flush(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("flush(canceled) error = %v, want context.Canceled", err)
	}
	if _, err := runtime.compact(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("compact(canceled) error = %v, want context.Canceled", err)
	}
}

type failingListener struct {
	err error
}

func (l failingListener) Accept() (net.Conn, error) { return nil, l.err }

func (failingListener) Close() error { return nil }

func (failingListener) Addr() net.Addr { return testAddr("127.0.0.1:0") }

type testAddr string

func (a testAddr) Network() string { return "tcp" }

func (a testAddr) String() string { return string(a) }

func TestRuntimeMaintenanceBusy(t *testing.T) {
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.HTTP.Addr = "127.0.0.1:0"
	cfg.HTTP.Enabled = true
	cfg.GRPC.Enabled = false
	runtime, err := openRuntime(context.Background(), cfg)
	if err != nil {
		t.Fatalf("openRuntime() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.engine.Close(context.Background())
	})

	if err := runtime.tryBeginMaintenance(); err != nil {
		t.Fatalf("tryBeginMaintenance() first error = %v", err)
	}
	err = runtime.tryBeginMaintenance()
	if err == nil {
		t.Fatal("tryBeginMaintenance() second call want error")
	}
	var apiErr apiError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want apiError", err)
	}
	if apiErr.Code != errorCodeResourceExhausted {
		t.Fatalf("code = %q, want %q", apiErr.Code, errorCodeResourceExhausted)
	}
	if !errors.Is(err, mts.ErrEngineBusy) {
		t.Fatalf("unwrap = %v, want ErrEngineBusy", err)
	}
	runtime.endMaintenance()
	if err := runtime.tryBeginMaintenance(); err != nil {
		t.Fatalf("tryBeginMaintenance() after end error = %v", err)
	}
	runtime.endMaintenance()

	// concurrent flush while holding maintenance should fail
	if err := runtime.tryBeginMaintenance(); err != nil {
		t.Fatalf("hold maintenance: %v", err)
	}
	err = runtime.flush(context.Background())
	if err == nil {
		t.Fatal("flush during maintenance want error")
	}
	if !errors.Is(err, mts.ErrEngineBusy) {
		t.Fatalf("flush busy err = %v", err)
	}
	runtime.endMaintenance()
}

func TestRuntimeMaintenanceStatsPayloadBusy(t *testing.T) {
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.HTTP.Addr = "127.0.0.1:0"
	cfg.HTTP.Enabled = true
	cfg.GRPC.Enabled = false
	runtime, err := openRuntime(context.Background(), cfg)
	if err != nil {
		t.Fatalf("openRuntime() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.engine.Close(context.Background())
	})

	payload := runtime.maintenanceStatsPayload()
	if payload.AdminOpBusy {
		t.Fatal("AdminOpBusy want false initially")
	}
	if err := runtime.tryBeginMaintenance(); err != nil {
		t.Fatalf("tryBeginMaintenance: %v", err)
	}
	payload = runtime.maintenanceStatsPayload()
	if !payload.AdminOpBusy {
		t.Fatal("AdminOpBusy want true while held")
	}
	if !runtime.opsStatusPayload().AdminOpBusy {
		t.Fatal("opsStatus AdminOpBusy want true while held")
	}
	if st := runtime.opsStatusPayload(); st.Op != "maintenance" || st.StartedAtUnix <= 0 {
		t.Fatalf("opsStatus while held = %+v", st)
	}
	runtime.endMaintenance()
	payload = runtime.maintenanceStatsPayload()
	if payload.AdminOpBusy {
		t.Fatal("AdminOpBusy want false after end")
	}
	if runtime.opsStatusPayload().AdminOpBusy {
		t.Fatal("opsStatus AdminOpBusy want false after end")
	}
	if st := runtime.opsStatusPayload(); st.Op != "" || st.StartedAtUnix != 0 {
		t.Fatalf("opsStatus after end = %+v", st)
	}
}

func TestRuntimeAdminHeavySharedMutex(t *testing.T) {
	cfg := defaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.HTTP.Addr = "127.0.0.1:0"
	cfg.HTTP.Enabled = true
	cfg.GRPC.Enabled = false
	runtime, err := openRuntime(context.Background(), cfg)
	if err != nil {
		t.Fatalf("openRuntime() error = %v", err)
	}
	t.Cleanup(func() {
		_ = runtime.engine.Close(context.Background())
	})

	if err := runtime.tryBeginAdminHeavy("test"); err != nil {
		t.Fatalf("hold heavy: %v", err)
	}
	// storage snapshot should fail while held
	if _, err := runtime.storageSnapshot(context.Background()); err == nil {
		t.Fatal("storageSnapshot during heavy want error")
	} else {
		var apiErr apiError
		if !errors.As(err, &apiErr) || apiErr.Code != errorCodeResourceExhausted {
			t.Fatalf("storageSnapshot err = %v", err)
		}
		if !strings.Contains(apiErr.Message, "test") {
			t.Fatalf("busy message want current op, got %q", apiErr.Message)
		}
		if !apiErr.AdminOpBusy || apiErr.Op != "test" {
			t.Fatalf("structured busy fields = %+v", apiErr)
		}
		_, resp := apiErrorResponse(err)
		if !resp.AdminOpBusy || resp.Op != "test" {
			t.Fatalf("errorResponse busy fields = %+v", resp)
		}
		rec := httptest.NewRecorder()
		writeAPIError(rec, err)
		if rec.Header().Get(headerAdminOpBusy) != "true" {
			t.Fatalf("header busy = %q", rec.Header().Get(headerAdminOpBusy))
		}
		if rec.Header().Get(headerAdminOp) != "test" {
			t.Fatalf("header op = %q", rec.Header().Get(headerAdminOp))
		}
	}
	// flush also busy
	if err := runtime.flush(context.Background()); err == nil {
		t.Fatal("flush during heavy want error")
	}
	runtime.endAdminHeavy()
	if _, err := runtime.storageSnapshot(context.Background()); err != nil {
		t.Fatalf("storageSnapshot after release: %v", err)
	}
}
