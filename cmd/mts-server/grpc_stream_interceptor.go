package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type grpcServerStreamContext struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *grpcServerStreamContext) Context() context.Context {
	return s.ctx
}

func (s *grpcServerStreamContext) RecvMsg(message any) error {
	result := make(chan error, 1)
	go func() {
		result <- s.ServerStream.RecvMsg(message)
	}()
	select {
	case err := <-result:
		return err
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

func (r *serverRuntime) grpcStreamInterceptor(
	service any,
	stream grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	cfg := r.currentConfig()
	requestID := grpcMetadataValue(stream.Context(), strings.ToLower(headerRequestID))
	if strings.TrimSpace(requestID) == "" {
		requestID = r.nextRequestID()
	}
	ctx := context.WithValue(stream.Context(), contextRequestID, requestID)
	if timeout := time.Duration(cfg.Limits.RequestTimeout); timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if err := stream.SetHeader(metadata.Pairs(strings.ToLower(headerRequestID), requestID)); err != nil {
		return status.Error(codes.Internal, "failed to set response headers")
	}
	if !r.grpcLimiter.tryAcquire() {
		return status.Error(codes.ResourceExhausted, "too many concurrent grpc requests")
	}
	defer r.grpcLimiter.release()
	start := time.Now()
	err := handler(service, &grpcServerStreamContext{ServerStream: stream, ctx: ctx})
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		err = grpcError(ctx, err)
	}
	code := status.Code(err)
	duration := time.Since(start)
	r.metrics.observe("grpc", info.FullMethod, code.String(), duration)
	if cfg.Observability.AccessLog {
		r.currentLogger().InfoContext(ctx, "grpc stream request",
			"request_id", requestID,
			"method", info.FullMethod,
			"code", code.String(),
			"duration", duration.String(),
		)
	}
	return err
}
