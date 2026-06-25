package main

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
)

const grpcServiceName = "mts.v1.MTSServer"

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }

func (jsonCodec) Marshal(value any) ([]byte, error) {
	return json.Marshal(value)
}

func (jsonCodec) Unmarshal(data []byte, value any) error {
	return json.Unmarshal(data, value)
}

func newGRPCServer(runtime *serverRuntime) *grpc.Server {
	server := grpc.NewServer()
	server.RegisterService(grpcServiceDesc(), &grpcService{runtime: runtime})
	return server
}

type grpcService struct {
	runtime *serverRuntime
}

type grpcServiceServer interface {
	mtsServer()
}

func (*grpcService) mtsServer() {}

func grpcServiceDesc() *grpc.ServiceDesc {
	return &grpc.ServiceDesc{
		ServiceName: grpcServiceName,
		HandlerType: (*grpcServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Health", Handler: grpcHealthHandler},
			{MethodName: "Write", Handler: grpcWriteHandler},
			{MethodName: "QueryRows", Handler: grpcQueryRowsHandler},
			{MethodName: "Flush", Handler: grpcFlushHandler},
			{MethodName: "Compact", Handler: grpcCompactHandler},
		},
	}
}

func grpcHealthHandler(
	service any,
	ctx context.Context,
	decode func(any) error,
	interceptor grpc.UnaryServerInterceptor,
) (any, error) {
	handler := func(ctx context.Context, _ any) (any, error) {
		return service.(*grpcService).runtime.health(), nil
	}
	return invokeGRPCUnary(ctx, &emptyRequest{}, decode, interceptor, "/mts.v1.MTSServer/Health", handler)
}

func grpcWriteHandler(
	service any,
	ctx context.Context,
	decode func(any) error,
	interceptor grpc.UnaryServerInterceptor,
) (any, error) {
	handler := func(ctx context.Context, req any) (any, error) {
		writeReq := req.(*writeRequest)
		if err := service.(*grpcService).runtime.write(ctx, *writeReq); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return writeResponse{OK: true}, nil
	}
	return invokeGRPCUnary(ctx, &writeRequest{}, decode, interceptor, "/mts.v1.MTSServer/Write", handler)
}

func grpcQueryRowsHandler(
	service any,
	ctx context.Context,
	decode func(any) error,
	interceptor grpc.UnaryServerInterceptor,
) (any, error) {
	handler := func(ctx context.Context, req any) (any, error) {
		queryReq := req.(*queryRowsRequest)
		rows, err := service.(*grpcService).runtime.queryRows(ctx, *queryReq)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return queryRowsResponse{Rows: rows}, nil
	}
	return invokeGRPCUnary(ctx, &queryRowsRequest{}, decode, interceptor, "/mts.v1.MTSServer/QueryRows", handler)
}

func grpcFlushHandler(
	service any,
	ctx context.Context,
	decode func(any) error,
	interceptor grpc.UnaryServerInterceptor,
) (any, error) {
	handler := func(ctx context.Context, _ any) (any, error) {
		if err := service.(*grpcService).runtime.flush(ctx); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return maintenanceResponse{OK: true}, nil
	}
	return invokeGRPCUnary(ctx, &emptyRequest{}, decode, interceptor, "/mts.v1.MTSServer/Flush", handler)
}

func grpcCompactHandler(
	service any,
	ctx context.Context,
	decode func(any) error,
	interceptor grpc.UnaryServerInterceptor,
) (any, error) {
	handler := func(ctx context.Context, _ any) (any, error) {
		result, err := service.(*grpcService).runtime.compact(ctx)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return maintenanceResponse{OK: true, Result: result}, nil
	}
	return invokeGRPCUnary(ctx, &emptyRequest{}, decode, interceptor, "/mts.v1.MTSServer/Compact", handler)
}

func invokeGRPCUnary(
	ctx context.Context,
	req any,
	decode func(any) error,
	interceptor grpc.UnaryServerInterceptor,
	method string,
	handler grpc.UnaryHandler,
) (any, error) {
	if err := decode(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if interceptor == nil {
		return handler(ctx, req)
	}
	info := &grpc.UnaryServerInfo{FullMethod: method}
	return interceptor(ctx, req, info, handler)
}

type emptyRequest struct{}

func invokeGRPC(
	ctx context.Context,
	conn *grpc.ClientConn,
	method string,
	in any,
	out any,
) error {
	return conn.Invoke(ctx, "/"+grpcServiceName+"/"+method, in, out, grpc.ForceCodec(jsonCodec{}))
}
