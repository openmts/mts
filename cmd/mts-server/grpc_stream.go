package main

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mts "github.com/openmts/mts"
)

func grpcStreamsFromRegistry() []grpc.StreamDesc {
	return []grpc.StreamDesc{
		{
			StreamName:    grpcMethodQueryStream,
			Handler:       grpcQueryStreamHandler,
			ServerStreams: true,
		},
	}
}

func grpcQueryStreamHandler(service any, stream grpc.ServerStream) error {
	r := service.(*grpcService).runtime
	grpcSem := r.grpcSem
	if !acquireGRPC(grpcSem) {
		return status.Error(codes.ResourceExhausted, "too many concurrent grpc requests")
	}
	defer releaseGRPC(grpcSem)

	var req queryStreamRequest
	if err := stream.RecvMsg(&req); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	ctx := stream.Context()
	if err := r.authorizeGRPCDatabase(ctx, req.Query.Database, mts.DatabasePermissionRead); err != nil {
		return grpcError(err)
	}
	format, err := normalizeStreamFormat(req.Format, req.Mode)
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	query, err := r.limitedQuery(req.Query)
	if err != nil {
		return grpcError(err)
	}
	switch format {
	case streamTypeColumn:
		return streamGRPCColumns(r, stream, query)
	default:
		return streamGRPCRows(r, stream, query)
	}
}

func streamGRPCRows(r *serverRuntime, stream grpc.ServerStream, query mts.Query) error {
	rows, err := r.engine.QueryRowIterator(stream.Context(), query)
	if err != nil {
		return grpcError(err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		row := rows.Row()
		if err := stream.SendMsg(streamRecord{Type: streamTypeRow, Row: &row}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return grpcError(err)
	}
	stats := r.queryStats()
	return stream.SendMsg(streamRecord{Type: streamTypeEnd, Stats: &stats})
}

func streamGRPCColumns(r *serverRuntime, stream grpc.ServerStream, query mts.Query) error {
	columns, err := r.engine.QueryColumnIterator(stream.Context(), query)
	if err != nil {
		return grpcError(err)
	}
	defer func() { _ = columns.Close() }()
	for columns.Next() {
		column := columns.Column()
		if err := stream.SendMsg(streamRecord{Type: streamTypeColumn, Column: &column}); err != nil {
			return err
		}
	}
	if err := columns.Err(); err != nil {
		return grpcError(err)
	}
	stats := r.queryStats()
	return stream.SendMsg(streamRecord{Type: streamTypeEnd, Stats: &stats})
}
