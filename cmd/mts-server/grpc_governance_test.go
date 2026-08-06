package main

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	mts "github.com/openmts/mts"
)

func TestHTTPAndGRPCLoginClampOverflowingTTL(t *testing.T) {
	runtime := openTestRuntime(t)
	seedUserWithPassword(t, runtime, mts.User{Name: "ttl-user"}, "secret12")
	server := httptest.NewServer(runtime.httpHandler())
	t.Cleanup(server.Close)
	var httpLogin authTokenResponse
	postJSON(t, server.URL+routeAuthLogin, loginRequest{
		UserName: "ttl-user", Password: "secret12", TTLSeconds: math.MaxInt64,
	}, http.StatusOK, &httpLogin)
	assertClampedAuthTTL(t, httpLogin)

	conn := openBufconnClient(t, runtime)
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	})
	var grpcLogin authTokenResponse
	invokeOK(t, context.Background(), conn, "Login", &loginRequest{
		UserName: "ttl-user", Password: "secret12", TTLSeconds: math.MaxInt64,
	}, &grpcLogin)
	assertClampedAuthTTL(t, grpcLogin)
}

func TestGRPCErrorMapsCancellationAndHidesInternalDetails(t *testing.T) {
	if code := status.Code(grpcErrorPlain(context.Canceled)); code != codes.Canceled {
		t.Fatalf("canceled code = %v, want Canceled", code)
	}
	if code := status.Code(grpcErrorPlain(context.DeadlineExceeded)); code != codes.DeadlineExceeded {
		t.Fatalf("deadline code = %v, want DeadlineExceeded", code)
	}
	internal := grpcErrorPlain(errors.New("open /secret/storage/path: permission denied"))
	if code := status.Code(internal); code != codes.Internal {
		t.Fatalf("internal code = %v, want Internal", code)
	}
	if message := status.Convert(internal).Message(); message != string(errorCodeInternal) {
		t.Fatalf("internal message = %q, want %q", message, errorCodeInternal)
	}
}

func assertClampedAuthTTL(t *testing.T, response authTokenResponse) {
	t.Helper()
	maxSeconds := int64(maxAuthTTL.Seconds())
	if response.RemainingSeconds <= 0 || response.RemainingSeconds > maxSeconds {
		t.Fatalf("remaining seconds = %d, want 1..%d", response.RemainingSeconds, maxSeconds)
	}
}

func TestGRPCStreamInterceptorAppliesTimeoutRequestIDAndLimiter(t *testing.T) {
	runtime := openTestRuntime(t)
	runtime.mu.Lock()
	runtime.config.Limits.RequestTimeout = durationText(20 * time.Millisecond)
	runtime.config.Limits.MaxConcurrentGRPC = 1
	runtime.applyLimitState(runtime.config)
	runtime.mu.Unlock()
	stream := &governanceServerStream{ctx: context.Background()}
	info := &grpc.StreamServerInfo{FullMethod: grpcFullMethod(grpcMethodQueryStream)}
	handler := func(_ any, stream grpc.ServerStream) error {
		if requestIDFromContext(stream.Context()) == "" {
			t.Error("request ID missing from stream context")
		}
		<-stream.Context().Done()
		return stream.Context().Err()
	}
	err := runtime.grpcStreamInterceptor(nil, stream, info, handler)
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("stream interceptor code = %v, want DeadlineExceeded", status.Code(err))
	}
	requestIDs := stream.header.Get(strings.ToLower(headerRequestID))
	if len(requestIDs) == 0 || requestIDs[0] == "" {
		t.Fatal("request ID header missing")
	}
	if _, inFlight := runtime.grpcLimiter.snapshot(); inFlight != 0 {
		t.Fatalf("gRPC in-flight = %d, want 0", inFlight)
	}
}

func TestGRPCUnaryInterceptorMapsDeadlineAndReleasesLimiter(t *testing.T) {
	runtime := openTestRuntime(t)
	runtime.mu.Lock()
	runtime.config.Limits.RequestTimeout = durationText(20 * time.Millisecond)
	runtime.config.Limits.MaxConcurrentGRPC = 1
	runtime.applyLimitState(runtime.config)
	runtime.mu.Unlock()
	handler := func(ctx context.Context, _ any) (any, error) {
		<-ctx.Done()
		return nil, nil
	}
	_, err := runtime.grpcUnaryInterceptor(
		context.Background(),
		&emptyRequest{},
		&grpc.UnaryServerInfo{FullMethod: grpcFullMethod(grpcMethodHealth)},
		handler,
	)
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("unary interceptor code = %v, want DeadlineExceeded", status.Code(err))
	}
	if _, inFlight := runtime.grpcLimiter.snapshot(); inFlight != 0 {
		t.Fatalf("gRPC in-flight = %d, want 0", inFlight)
	}
}

func TestGRPCStreamInterceptorTimesOutSlowInitialRead(t *testing.T) {
	runtime := openTestRuntime(t)
	runtime.mu.Lock()
	runtime.config.Limits.RequestTimeout = durationText(20 * time.Millisecond)
	runtime.mu.Unlock()
	conn := openBufconnClient(t, runtime)
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{
		StreamName:    grpcMethodQueryStream,
		ServerStreams: true,
	}, grpcFullMethod(grpcMethodQueryStream))
	if err != nil {
		t.Fatalf("NewStream() error = %v", err)
	}
	start := time.Now()
	err = stream.RecvMsg(&streamRecord{})
	elapsed := time.Since(start)
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("RecvMsg() code = %v, want DeadlineExceeded", status.Code(err))
	}
	if elapsed > 150*time.Millisecond {
		t.Fatalf("slow initial read elapsed = %s, want <= 150ms", elapsed)
	}
}

func TestGRPCHandlerHidesInternalStoragePath(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	secretPath := filepath.Join(t.TempDir(), "secret-backup-target")
	if err := os.WriteFile(secretPath, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("WriteFile(secret path) error = %v", err)
	}
	runtime.mu.Lock()
	runtime.config.Backup.Dir = secretPath
	runtime.mu.Unlock()
	conn := openBufconnClient(t, runtime)
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	})
	ctx := metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer test-admin-token"),
	)
	err := invokeGRPC(ctx, conn, grpcMethodStorageSnapshot, &emptyRequest{}, &storageSnapshotResponse{})
	if status.Code(err) != codes.Internal {
		t.Fatalf("StorageSnapshot code = %v, want Internal", status.Code(err))
	}
	message := status.Convert(err).Message()
	if message != string(errorCodeInternal) {
		t.Fatalf("StorageSnapshot message = %q, want %q", message, errorCodeInternal)
	}
	if strings.Contains(message, secretPath) {
		t.Fatalf("StorageSnapshot message leaks path %q", secretPath)
	}
}

type governanceServerStream struct {
	mu      sync.Mutex
	ctx     context.Context
	header  metadata.MD
	trailer metadata.MD
}

func (s *governanceServerStream) SetHeader(header metadata.MD) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.header = metadata.Join(s.header, header)
	return nil
}

func (s *governanceServerStream) SendHeader(header metadata.MD) error {
	return s.SetHeader(header)
}

func (s *governanceServerStream) SetTrailer(trailer metadata.MD) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trailer = metadata.Join(s.trailer, trailer)
}

func (s *governanceServerStream) Context() context.Context { return s.ctx }

func (*governanceServerStream) SendMsg(any) error { return nil }

func (*governanceServerStream) RecvMsg(any) error { return nil }
