package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestGRPCStreamInterceptorPreservesRequestIDAndAccessLog(t *testing.T) {
	runtime := openTestRuntime(t)
	var output bytes.Buffer
	runtime.setLogger(slog.New(slog.NewTextHandler(&output, nil)))
	runtime.mu.Lock()
	runtime.config.Limits.RequestTimeout = 0
	runtime.config.Observability.AccessLog = true
	runtime.mu.Unlock()
	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs(strings.ToLower(headerRequestID), "client-request-id"),
	)
	stream := &governanceServerStream{ctx: ctx}
	handler := func(_ any, stream grpc.ServerStream) error {
		if got := requestIDFromContext(stream.Context()); got != "client-request-id" {
			t.Fatalf("request ID = %q, want client-request-id", got)
		}
		return nil
	}
	err := runtime.grpcStreamInterceptor(
		nil,
		stream,
		&grpc.StreamServerInfo{FullMethod: grpcFullMethod(grpcMethodQueryStream)},
		handler,
	)
	if err != nil {
		t.Fatalf("grpcStreamInterceptor() error = %v", err)
	}
	requestIDs := stream.header.Get(strings.ToLower(headerRequestID))
	if len(requestIDs) != 1 || requestIDs[0] != "client-request-id" {
		t.Fatalf("request ID header = %v, want client-request-id", requestIDs)
	}
	if logOutput := output.String(); !strings.Contains(logOutput, "client-request-id") {
		t.Fatalf("access log = %q, want request ID", logOutput)
	}
}

func TestGRPCStreamInterceptorRejectsHeaderFailureAndLimit(t *testing.T) {
	t.Run("header failure", func(t *testing.T) {
		runtime := openTestRuntime(t)
		stream := &headerErrorServerStream{
			governanceServerStream: governanceServerStream{ctx: context.Background()},
			err:                    errors.New("header write failed"),
		}
		called := false
		err := runtime.grpcStreamInterceptor(
			nil,
			stream,
			&grpc.StreamServerInfo{FullMethod: grpcFullMethod(grpcMethodQueryStream)},
			func(any, grpc.ServerStream) error {
				called = true
				return nil
			},
		)
		if status.Code(err) != codes.Internal {
			t.Fatalf("header failure code = %v, want Internal", status.Code(err))
		}
		if called {
			t.Fatal("handler called after header failure")
		}
	})

	t.Run("limit", func(t *testing.T) {
		runtime := openTestRuntime(t)
		runtime.grpcLimiter.setLimit(1)
		if !runtime.grpcLimiter.tryAcquire() {
			t.Fatal("failed to occupy gRPC limiter")
		}
		defer runtime.grpcLimiter.release()
		called := false
		err := runtime.grpcStreamInterceptor(
			nil,
			&governanceServerStream{ctx: context.Background()},
			&grpc.StreamServerInfo{FullMethod: grpcFullMethod(grpcMethodQueryStream)},
			func(any, grpc.ServerStream) error {
				called = true
				return nil
			},
		)
		if status.Code(err) != codes.ResourceExhausted {
			t.Fatalf("limit code = %v, want ResourceExhausted", status.Code(err))
		}
		if called {
			t.Fatal("handler called after limiter rejection")
		}
	})
}

func TestGRPCUnaryInterceptorPreservesRequestIDLogsAndLimit(t *testing.T) {
	runtime := openTestRuntime(t)
	var output bytes.Buffer
	runtime.setLogger(slog.New(slog.NewTextHandler(&output, nil)))
	runtime.mu.Lock()
	runtime.config.Limits.RequestTimeout = 0
	runtime.config.Observability.AccessLog = true
	runtime.mu.Unlock()
	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs(strings.ToLower(headerRequestID), "unary-request-id"),
	)
	_, err := runtime.grpcUnaryInterceptor(
		ctx,
		&emptyRequest{},
		&grpc.UnaryServerInfo{FullMethod: grpcFullMethod(grpcMethodHealth)},
		func(ctx context.Context, _ any) (any, error) {
			if got := requestIDFromContext(ctx); got != "unary-request-id" {
				t.Fatalf("request ID = %q, want unary-request-id", got)
			}
			return &emptyRequest{}, nil
		},
	)
	if err != nil {
		t.Fatalf("grpcUnaryInterceptor() error = %v", err)
	}
	if logOutput := output.String(); !strings.Contains(logOutput, "unary-request-id") {
		t.Fatalf("access log = %q, want request ID", logOutput)
	}

	runtime.grpcLimiter.setLimit(1)
	if !runtime.grpcLimiter.tryAcquire() {
		t.Fatal("failed to occupy gRPC limiter")
	}
	defer runtime.grpcLimiter.release()
	called := false
	_, err = runtime.grpcUnaryInterceptor(
		context.Background(),
		&emptyRequest{},
		&grpc.UnaryServerInfo{FullMethod: grpcFullMethod(grpcMethodHealth)},
		func(context.Context, any) (any, error) {
			called = true
			return nil, nil
		},
	)
	if status.Code(err) != codes.ResourceExhausted {
		t.Fatalf("limit code = %v, want ResourceExhausted", status.Code(err))
	}
	if called {
		t.Fatal("handler called after limiter rejection")
	}
}

func TestGRPCErrorPreservesPublicErrorsAndBusyCode(t *testing.T) {
	badRequest := grpcError(
		context.Background(),
		newAPIError(errorCodeBadRequest, "invalid request", nil),
	)
	if status.Code(badRequest) != codes.InvalidArgument {
		t.Fatalf("bad request code = %v, want InvalidArgument", status.Code(badRequest))
	}
	if message := status.Convert(badRequest).Message(); message != "invalid request" {
		t.Fatalf("bad request message = %q, want invalid request", message)
	}
	busy := grpcError(context.Background(), newAdminHeavyBusyError("compact"))
	if status.Code(busy) != codes.ResourceExhausted {
		t.Fatalf("busy code = %v, want ResourceExhausted", status.Code(busy))
	}
}

type headerErrorServerStream struct {
	governanceServerStream
	err error
}

func (s *headerErrorServerStream) SetHeader(metadata.MD) error {
	return s.err
}
