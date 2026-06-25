package main

import (
	"context"
	"encoding/json"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"

	mts "github.com/openmts/mts"
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
	methods := []grpc.MethodDesc{
		{MethodName: "Health", Handler: grpcHealthHandler},
		{MethodName: "Write", Handler: grpcWriteHandler},
		{MethodName: "WriteTypedBatch", Handler: unaryHandler(&typedWriteRequest{}, grpcWriteTypedBatch)},
		{MethodName: "QueryRows", Handler: grpcQueryRowsHandler},
		{MethodName: "QueryColumns", Handler: unaryHandler(&queryRequest{}, grpcQueryColumns)},
		{MethodName: "QueryWithExplain", Handler: unaryHandler(&queryRequest{}, grpcQueryWithExplain)},
		{MethodName: "QueryStats", Handler: unaryHandler(&emptyRequest{}, grpcQueryStats)},
		{MethodName: "CreateUser", Handler: unaryHandler(&mts.User{}, grpcCreateUser)},
		{MethodName: "UpdateUser", Handler: unaryHandler(&mts.User{}, grpcUpdateUser)},
		{MethodName: "GetUser", Handler: unaryHandler(&userNameRequest{}, grpcGetUser)},
		{MethodName: "ListUsers", Handler: unaryHandler(&emptyRequest{}, grpcListUsers)},
		{MethodName: "DeleteUser", Handler: unaryHandler(&userNameRequest{}, grpcDeleteUser)},
		{MethodName: "GrantDatabasePermission", Handler: unaryHandler(&databasePermissionRequest{}, grpcGrantDatabasePermission)},
		{MethodName: "RevokeDatabasePermission", Handler: unaryHandler(&databasePermissionRequest{}, grpcRevokeDatabasePermission)},
		{MethodName: "ListDatabasePermissions", Handler: unaryHandler(&userNameRequest{}, grpcListDatabasePermissions)},
		{MethodName: "CheckDatabasePermission", Handler: unaryHandler(&databasePermissionRequest{}, grpcCheckDatabasePermission)},
		{MethodName: "CreateDatabase", Handler: unaryHandler(&databaseRequest{}, grpcCreateDatabase)},
		{MethodName: "DropDatabase", Handler: unaryHandler(&databaseRequest{}, grpcDropDatabase)},
		{MethodName: "CreateRetentionPolicy", Handler: unaryHandler(&grpcRetentionPolicyRequest{}, grpcCreateRetentionPolicy)},
		{MethodName: "ListRetentionPolicies", Handler: unaryHandler(&databaseRequest{}, grpcListRetentionPolicies)},
		{MethodName: "ListMeasurements", Handler: unaryHandler(&metadataRequest{}, grpcListMeasurements)},
		{MethodName: "ListFields", Handler: unaryHandler(&metadataRequest{}, grpcListFields)},
		{MethodName: "ListSeries", Handler: unaryHandler(&metadataRequest{}, grpcListSeries)},
		{MethodName: "GetConfig", Handler: unaryHandler(&emptyRequest{}, grpcGetConfig)},
		{MethodName: "GetEffectiveConfig", Handler: unaryHandler(&emptyRequest{}, grpcGetConfig)},
		{MethodName: "GetConfigSchema", Handler: unaryHandler(&emptyRequest{}, grpcGetConfigSchema)},
		{MethodName: "Flush", Handler: grpcFlushHandler},
		{MethodName: "Compact", Handler: grpcCompactHandler},
		{MethodName: "ApplyRetention", Handler: unaryHandler(&retentionApplyRequest{}, grpcApplyRetention)},
		{MethodName: "MaintenanceErrors", Handler: unaryHandler(&emptyRequest{}, grpcMaintenanceErrors)},
		{MethodName: "StorageMemory", Handler: unaryHandler(&emptyRequest{}, grpcStorageMemory)},
		{MethodName: "CompactionStats", Handler: unaryHandler(&emptyRequest{}, grpcCompactionStats)},
		{MethodName: "CreateDownsamplePolicy", Handler: unaryHandler(&mts.DownsamplePolicy{}, grpcCreateDownsamplePolicy)},
		{MethodName: "ListDownsamplePolicies", Handler: unaryHandler(&emptyRequest{}, grpcListDownsamplePolicies)},
		{MethodName: "EnableDownsamplePolicy", Handler: unaryHandler(&downsamplePolicyRequest{}, grpcEnableDownsamplePolicy)},
		{MethodName: "DisableDownsamplePolicy", Handler: unaryHandler(&downsamplePolicyRequest{}, grpcDisableDownsamplePolicy)},
		{MethodName: "DropDownsamplePolicy", Handler: unaryHandler(&downsamplePolicyRequest{}, grpcDropDownsamplePolicy)},
		{MethodName: "ResetDownsamplePolicy", Handler: unaryHandler(&grpcDownsampleResetRequest{}, grpcResetDownsamplePolicy)},
		{MethodName: "DownsamplePolicyStatuses", Handler: unaryHandler(&emptyRequest{}, grpcDownsamplePolicyStatuses)},
		{MethodName: "RunDownsamplePolicy", Handler: unaryHandler(&downsamplePolicyRangeRequest{}, grpcRunDownsamplePolicy)},
		{MethodName: "RunDownsamplePolicyRange", Handler: unaryHandler(&downsamplePolicyRangeRequest{}, grpcRunDownsamplePolicyRange)},
		{MethodName: "RepairDownsamplePolicy", Handler: unaryHandler(&downsamplePolicyRangeRequest{}, grpcRepairDownsamplePolicy)},
		{MethodName: "DryRunDownsamplePolicy", Handler: unaryHandler(&downsamplePolicyRangeRequest{}, grpcDryRunDownsamplePolicy)},
	}
	return &grpc.ServiceDesc{ServiceName: grpcServiceName, HandlerType: (*grpcServiceServer)(nil), Methods: methods}
}

func grpcHealthHandler(service any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	handler := func(ctx context.Context, _ any) (any, error) {
		return service.(*grpcService).runtime.health(), nil
	}
	return invokeGRPCUnary(ctx, &emptyRequest{}, decode, interceptor, "/mts.v1.MTSServer/Health", handler)
}

func grpcWriteHandler(service any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	handler := func(ctx context.Context, req any) (any, error) {
		writeReq := req.(*writeRequest)
		if err := service.(*grpcService).runtime.write(ctx, *writeReq); err != nil {
			return nil, grpcError(err)
		}
		return writeResponse{OK: true}, nil
	}
	return invokeGRPCUnary(ctx, &writeRequest{}, decode, interceptor, "/mts.v1.MTSServer/Write", handler)
}

func grpcQueryRowsHandler(service any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	handler := func(ctx context.Context, req any) (any, error) {
		queryReq := req.(*queryRowsRequest)
		rows, err := service.(*grpcService).runtime.queryRows(ctx, *queryReq)
		if err != nil {
			return nil, grpcError(err)
		}
		return queryRowsResponse{Rows: rows}, nil
	}
	return invokeGRPCUnary(ctx, &queryRowsRequest{}, decode, interceptor, "/mts.v1.MTSServer/QueryRows", handler)
}

func grpcFlushHandler(service any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	handler := func(ctx context.Context, _ any) (any, error) {
		if err := service.(*grpcService).runtime.requireGRPCAdmin(ctx); err != nil {
			return nil, grpcError(err)
		}
		if err := service.(*grpcService).runtime.flush(ctx); err != nil {
			return nil, grpcError(err)
		}
		return maintenanceResponse{OK: true}, nil
	}
	return invokeGRPCUnary(ctx, &emptyRequest{}, decode, interceptor, "/mts.v1.MTSServer/Flush", handler)
}

func grpcCompactHandler(service any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	handler := func(ctx context.Context, _ any) (any, error) {
		if err := service.(*grpcService).runtime.requireGRPCAdmin(ctx); err != nil {
			return nil, grpcError(err)
		}
		result, err := service.(*grpcService).runtime.compact(ctx)
		if err != nil {
			return nil, grpcError(err)
		}
		return maintenanceResponse{OK: true, Result: result}, nil
	}
	return invokeGRPCUnary(ctx, &emptyRequest{}, decode, interceptor, "/mts.v1.MTSServer/Compact", handler)
}

func unaryHandler(req any, fn func(*serverRuntime, context.Context, any) (any, error)) grpc.MethodHandler {
	return func(service any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		method := "/" + grpcServiceName
		handler := func(ctx context.Context, decoded any) (any, error) {
			resp, err := fn(service.(*grpcService).runtime, ctx, decoded)
			if err != nil {
				return nil, grpcError(err)
			}
			return resp, nil
		}
		return invokeGRPCUnary(ctx, req, decode, interceptor, method, handler)
	}
}

func grpcWriteTypedBatch(r *serverRuntime, ctx context.Context, req any) (any, error) {
	request := req.(*typedWriteRequest)
	if err := r.authorizeGRPCDatabase(ctx, request.Batch.Database, mts.DatabasePermissionWrite); err != nil {
		return nil, err
	}
	return writeResponse{OK: true}, r.writeTypedBatch(ctx, *request)
}

func grpcQueryColumns(r *serverRuntime, ctx context.Context, req any) (any, error) {
	request := req.(*queryRequest)
	if err := r.authorizeGRPCDatabase(ctx, request.Query.Database, mts.DatabasePermissionRead); err != nil {
		return nil, err
	}
	columns, err := r.queryColumns(ctx, *request)
	return queryColumnsResponse{Columns: columns}, err
}

func grpcQueryWithExplain(r *serverRuntime, ctx context.Context, req any) (any, error) {
	request := req.(*queryRequest)
	if err := r.authorizeGRPCDatabase(ctx, request.Query.Database, mts.DatabasePermissionRead); err != nil {
		return nil, err
	}
	result, err := r.queryWithExplain(ctx, *request)
	return queryExplainResponse{Result: result}, err
}

func grpcQueryStats(r *serverRuntime, _ context.Context, _ any) (any, error) {
	return queryStatsResponse{Stats: r.queryStats()}, nil
}

func grpcCreateUser(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return okResponse{OK: true}, r.engine.CreateUser(ctx, *req.(*mts.User))
}

func grpcUpdateUser(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return okResponse{OK: true}, r.engine.UpdateUser(ctx, *req.(*mts.User))
}

func grpcGetUser(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	user, ok, err := r.engine.GetUser(ctx, req.(*userNameRequest).Name)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, mts.ErrUserNotFound
	}
	return userResponse{User: user}, nil
}

func grpcListUsers(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	users, err := r.engine.ListUsers(ctx)
	return usersResponse{Users: users}, err
}

func grpcDeleteUser(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return okResponse{OK: true}, r.engine.DeleteUser(ctx, req.(*userNameRequest).Name)
}

func grpcGrantDatabasePermission(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*databasePermissionRequest)
	return okResponse{OK: true}, r.engine.GrantDatabasePermission(ctx, request.UserName, request.Database, request.Permission)
}

func grpcRevokeDatabasePermission(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*databasePermissionRequest)
	return okResponse{OK: true}, r.engine.RevokeDatabasePermission(ctx, request.UserName, request.Database, request.Permission)
}

func grpcListDatabasePermissions(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	grants, err := r.engine.ListDatabasePermissions(ctx, req.(*userNameRequest).Name)
	return databasePermissionsResponse{Grants: grants}, err
}

func grpcCheckDatabasePermission(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*databasePermissionRequest)
	err := r.engine.CheckUserDatabasePermission(ctx, request.UserName, request.Database, request.Permission)
	return authzDatabaseCheckResponse{Allowed: err == nil}, nil
}

func grpcCreateDatabase(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return okResponse{OK: true}, r.engine.CreateDatabase(ctx, req.(*databaseRequest).Name)
}

func grpcDropDatabase(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return okResponse{OK: true}, r.engine.DropDatabase(ctx, req.(*databaseRequest).Name)
}

func grpcCreateRetentionPolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*grpcRetentionPolicyRequest)
	return okResponse{OK: true}, r.engine.CreateRetentionPolicy(ctx, request.Database, request.Policy)
}

func grpcListRetentionPolicies(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	policies, err := r.engine.ListRetentionPolicies(ctx, req.(*databaseRequest).Name)
	return retentionPoliciesResponse{Policies: policies}, err
}

func grpcListMeasurements(r *serverRuntime, ctx context.Context, req any) (any, error) {
	request := req.(*metadataRequest)
	if err := r.authorizeGRPCDatabase(ctx, request.Database, mts.DatabasePermissionRead); err != nil {
		return nil, err
	}
	measurements, err := r.engine.ListMeasurements(ctx, request.Database)
	return measurementsResponse{Measurements: measurements}, err
}

func grpcListFields(r *serverRuntime, ctx context.Context, req any) (any, error) {
	request := req.(*metadataRequest)
	if err := r.authorizeGRPCDatabase(ctx, request.Database, mts.DatabasePermissionRead); err != nil {
		return nil, err
	}
	fields, err := r.engine.ListFields(ctx, request.Database, request.Measurement)
	return fieldsResponse{Fields: fields}, err
}

func grpcListSeries(r *serverRuntime, ctx context.Context, req any) (any, error) {
	request := req.(*metadataRequest)
	if err := r.authorizeGRPCDatabase(ctx, request.Database, mts.DatabasePermissionRead); err != nil {
		return nil, err
	}
	series, err := r.engine.ListSeries(ctx, request.Database, request.Measurement, request.Tags)
	return seriesResponse{Series: series}, err
}

func grpcGetConfig(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return configResponse{Config: r.effectiveConfig()}, nil
}

func grpcGetConfigSchema(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return configSchemaResponse{Fields: configSchema()}, nil
}

func grpcApplyRetention(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return okResponse{OK: true}, r.applyRetention(ctx, *req.(*retentionApplyRequest))
}

func grpcMaintenanceErrors(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return maintenanceErrorsResponse{Errors: r.maintenanceErrors(ctx)}, nil
}

func grpcStorageMemory(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return storageMemoryResponse{Snapshot: r.storageMemory()}, nil
}

func grpcCompactionStats(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return compactionStatsResponse{Stats: r.compactionStats()}, nil
}

func grpcCreateDownsamplePolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return okResponse{OK: true}, r.engine.CreateDownsamplePolicy(ctx, *req.(*mts.DownsamplePolicy))
}

func grpcListDownsamplePolicies(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	policies, err := r.engine.ListDownsamplePolicies(ctx)
	return downsamplePoliciesResponse{Policies: policies}, err
}

func grpcEnableDownsamplePolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return okResponse{OK: true}, r.engine.EnableDownsamplePolicy(ctx, req.(*downsamplePolicyRequest).Name)
}

func grpcDisableDownsamplePolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return okResponse{OK: true}, r.engine.DisableDownsamplePolicy(ctx, req.(*downsamplePolicyRequest).Name)
}

func grpcDropDownsamplePolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return okResponse{OK: true}, r.engine.DropDownsamplePolicy(ctx, req.(*downsamplePolicyRequest).Name)
}

func grpcResetDownsamplePolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*grpcDownsampleResetRequest)
	return okResponse{OK: true}, r.engine.ResetDownsamplePolicy(ctx, request.Name, request.Reset)
}

func grpcDownsamplePolicyStatuses(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	statuses, err := r.engine.DownsamplePolicyStatuses(ctx, timeNow())
	return downsampleStatusesResponse{Statuses: statuses}, err
}

func grpcRunDownsamplePolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*downsamplePolicyRangeRequest)
	result, err := r.engine.RunDownsamplePolicy(ctx, request.Name, unixSecondsOrNow(request.EndUnix))
	return downsampleRunResponse{Result: result}, err
}

func grpcRunDownsamplePolicyRange(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*downsamplePolicyRangeRequest)
	result, err := r.engine.RunDownsamplePolicyRange(ctx, request.Name, unixFlexible(request.StartUnix), unixFlexible(request.EndUnix), request.Options)
	return downsampleRunResponse{Result: result}, err
}

func grpcRepairDownsamplePolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*downsamplePolicyRangeRequest)
	result, err := r.engine.RepairDownsamplePolicy(ctx, request.Name, unixFlexible(request.StartUnix), unixFlexible(request.EndUnix))
	return downsampleRunResponse{Result: result}, err
}

func grpcDryRunDownsamplePolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*downsamplePolicyRangeRequest)
	result, err := r.engine.DryRunDownsamplePolicy(ctx, request.Name, unixFlexible(request.StartUnix), unixFlexible(request.EndUnix))
	return downsampleDryRunResponse{Result: result}, err
}

func invokeGRPCUnary(ctx context.Context, req any, decode func(any) error, interceptor grpc.UnaryServerInterceptor, method string, handler grpc.UnaryHandler) (any, error) {
	if err := decode(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if interceptor == nil {
		return handler(ctx, req)
	}
	info := &grpc.UnaryServerInfo{FullMethod: method}
	return interceptor(ctx, req, info, handler)
}

func grpcError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr apiError
	if errors.As(err, &apiErr) {
		return status.Error(grpcCodeForErrorCode(apiErr.Code), err.Error())
	}
	if errors.Is(err, mts.ErrPermissionDenied) {
		return status.Error(codes.PermissionDenied, err.Error())
	}
	if errors.Is(err, mts.ErrUserNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	if errors.Is(err, mts.ErrUserAlreadyExists) {
		return status.Error(codes.AlreadyExists, err.Error())
	}
	if errors.Is(err, mts.ErrInvalidUser) || errors.Is(err, mts.ErrInvalidPermission) || errors.Is(err, mts.ErrInvalidPrecision) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	return status.Error(codes.InvalidArgument, err.Error())
}

func grpcCodeForErrorCode(code errorCode) codes.Code {
	switch code {
	case errorCodeUnauthenticated:
		return codes.Unauthenticated
	case errorCodePermissionDenied:
		return codes.PermissionDenied
	case errorCodeNotFound:
		return codes.NotFound
	case errorCodeAlreadyExists:
		return codes.AlreadyExists
	case errorCodeBadRequest:
		return codes.InvalidArgument
	default:
		return codes.Internal
	}
}

func invokeGRPC(ctx context.Context, conn *grpc.ClientConn, method string, in any, out any) error {
	return conn.Invoke(ctx, "/"+grpcServiceName+"/"+method, in, out, grpc.ForceCodec(jsonCodec{}))
}
