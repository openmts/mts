package main

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/metadata"
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

func newGRPCServer(runtime *serverRuntime) (*grpc.Server, error) {
	cfg := runtime.currentConfig()
	options := []grpc.ServerOption{grpc.UnaryInterceptor(runtime.grpcUnaryInterceptor)}
	if cfg.GRPC.MaxRecvMsgBytes > 0 {
		options = append(options, grpc.MaxRecvMsgSize(cfg.GRPC.MaxRecvMsgBytes))
	}
	if cfg.GRPC.MaxSendMsgBytes > 0 {
		options = append(options, grpc.MaxSendMsgSize(cfg.GRPC.MaxSendMsgBytes))
	}
	if cfg.GRPC.TLS.Enabled {
		tlsConfig, err := buildTLSConfig(cfg.GRPC.TLS)
		if err != nil {
			return nil, err
		}
		options = append(options, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}
	server := grpc.NewServer(options...)
	server.RegisterService(grpcServiceDesc(), &grpcService{runtime: runtime})
	return server, nil
}

func (r *serverRuntime) grpcUnaryInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	requestID := grpcMetadataValue(ctx, strings.ToLower(headerRequestID))
	if strings.TrimSpace(requestID) == "" {
		requestID = r.nextRequestID()
	}
	ctx = context.WithValue(ctx, contextRequestID, requestID)
	if timeout := time.Duration(r.currentConfig().Limits.RequestTimeout); timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	_ = grpc.SetHeader(ctx, metadata.Pairs(strings.ToLower(headerRequestID), requestID))
	grpcSem := r.grpcSem
	if !acquireGRPC(grpcSem) {
		return nil, status.Error(codes.ResourceExhausted, "too many concurrent grpc requests")
	}
	defer releaseGRPC(grpcSem)
	start := time.Now()
	resp, err := handler(ctx, req)
	code := status.Code(err)
	duration := time.Since(start)
	r.metrics.observe("grpc", info.FullMethod, code.String(), duration)
	if r.currentConfig().Observability.AccessLog {
		r.currentLogger().InfoContext(ctx, "grpc request",
			"request_id", requestID,
			"method", info.FullMethod,
			"code", code.String(),
			"duration", duration.String(),
		)
	}
	return resp, err
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
		{MethodName: grpcMethodHealth, Handler: grpcHealthHandler},
		{MethodName: grpcMethodWrite, Handler: grpcWriteHandler},
		{MethodName: grpcMethodWriteTypedBatch, Handler: unaryHandler(grpcMethodWriteTypedBatch, &typedWriteRequest{}, grpcWriteTypedBatch)},
		{MethodName: grpcMethodQueryRows, Handler: grpcQueryRowsHandler},
		{MethodName: grpcMethodQueryColumns, Handler: unaryHandler(grpcMethodQueryColumns, &queryRequest{}, grpcQueryColumns)},
		{MethodName: grpcMethodQueryWithExplain, Handler: unaryHandler(grpcMethodQueryWithExplain, &queryRequest{}, grpcQueryWithExplain)},
		{MethodName: grpcMethodQueryStats, Handler: unaryHandler(grpcMethodQueryStats, &emptyRequest{}, grpcQueryStats)},
		{MethodName: grpcMethodLogin, Handler: unaryHandler(grpcMethodLogin, &loginRequest{}, grpcLogin)},
		{MethodName: grpcMethodLogout, Handler: unaryHandler(grpcMethodLogout, &logoutRequest{}, grpcLogout)},
		{MethodName: grpcMethodChangePassword, Handler: unaryHandler(grpcMethodChangePassword, &changePasswordRequest{}, grpcChangePassword)},
		{MethodName: grpcMethodSetUserPassword, Handler: unaryHandler(grpcMethodSetUserPassword, &setUserPasswordRequest{}, grpcSetUserPassword)},
		{MethodName: grpcMethodCreateUser, Handler: unaryHandler(grpcMethodCreateUser, &createUserRequest{}, grpcCreateUser)},
		{MethodName: grpcMethodUpdateUser, Handler: unaryHandler(grpcMethodUpdateUser, &mts.User{}, grpcUpdateUser)},
		{MethodName: grpcMethodGetUser, Handler: unaryHandler(grpcMethodGetUser, &userNameRequest{}, grpcGetUser)},
		{MethodName: grpcMethodListUsers, Handler: unaryHandler(grpcMethodListUsers, &emptyRequest{}, grpcListUsers)},
		{MethodName: grpcMethodDeleteUser, Handler: unaryHandler(grpcMethodDeleteUser, &userNameRequest{}, grpcDeleteUser)},
		{
			MethodName: grpcMethodGrantDatabasePermission,
			Handler:    unaryHandler(grpcMethodGrantDatabasePermission, &databasePermissionRequest{}, grpcGrantDatabasePermission),
		},
		{
			MethodName: grpcMethodRevokeDatabasePermission,
			Handler:    unaryHandler(grpcMethodRevokeDatabasePermission, &databasePermissionRequest{}, grpcRevokeDatabasePermission),
		},
		{
			MethodName: grpcMethodListDatabasePermissions,
			Handler:    unaryHandler(grpcMethodListDatabasePermissions, &userNameRequest{}, grpcListDatabasePermissions),
		},
		{
			MethodName: grpcMethodCheckDatabasePermission,
			Handler:    unaryHandler(grpcMethodCheckDatabasePermission, &databasePermissionRequest{}, grpcCheckDatabasePermission),
		},
		{MethodName: grpcMethodCreateDatabase, Handler: unaryHandler(grpcMethodCreateDatabase, &databaseRequest{}, grpcCreateDatabase)},
		{MethodName: grpcMethodDropDatabase, Handler: unaryHandler(grpcMethodDropDatabase, &databaseRequest{}, grpcDropDatabase)},
		{
			MethodName: grpcMethodCreateRetentionPolicy,
			Handler:    unaryHandler(grpcMethodCreateRetentionPolicy, &grpcRetentionPolicyRequest{}, grpcCreateRetentionPolicy),
		},
		{
			MethodName: grpcMethodListRetentionPolicies,
			Handler:    unaryHandler(grpcMethodListRetentionPolicies, &databaseRequest{}, grpcListRetentionPolicies),
		},
		{MethodName: grpcMethodListMeasurements, Handler: unaryHandler(grpcMethodListMeasurements, &metadataRequest{}, grpcListMeasurements)},
		{MethodName: grpcMethodListFields, Handler: unaryHandler(grpcMethodListFields, &metadataRequest{}, grpcListFields)},
		{MethodName: grpcMethodListSeries, Handler: unaryHandler(grpcMethodListSeries, &metadataRequest{}, grpcListSeries)},
		{MethodName: grpcMethodGetConfig, Handler: unaryHandler(grpcMethodGetConfig, &emptyRequest{}, grpcGetConfig)},
		{MethodName: grpcMethodGetEffectiveConfig, Handler: unaryHandler(grpcMethodGetEffectiveConfig, &emptyRequest{}, grpcGetConfig)},
		{MethodName: grpcMethodGetConfigSchema, Handler: unaryHandler(grpcMethodGetConfigSchema, &emptyRequest{}, grpcGetConfigSchema)},
		{MethodName: grpcMethodValidateConfig, Handler: unaryHandler(grpcMethodValidateConfig, &configValidateRequest{}, grpcValidateConfig)},
		{MethodName: grpcMethodReloadConfig, Handler: unaryHandler(grpcMethodReloadConfig, &emptyRequest{}, grpcReloadConfig)},
		{MethodName: grpcMethodGetAPISpec, Handler: unaryHandler(grpcMethodGetAPISpec, &emptyRequest{}, grpcGetAPISpec)},
		{MethodName: grpcMethodGetErrorCodes, Handler: unaryHandler(grpcMethodGetErrorCodes, &emptyRequest{}, grpcGetErrorCodes)},
		{MethodName: grpcMethodFlush, Handler: grpcFlushHandler},
		{MethodName: grpcMethodCompact, Handler: grpcCompactHandler},
		{MethodName: grpcMethodApplyRetention, Handler: unaryHandler(grpcMethodApplyRetention, &retentionApplyRequest{}, grpcApplyRetention)},
		{MethodName: grpcMethodMaintenanceErrors, Handler: unaryHandler(grpcMethodMaintenanceErrors, &emptyRequest{}, grpcMaintenanceErrors)},
		{MethodName: grpcMethodStorageMemory, Handler: unaryHandler(grpcMethodStorageMemory, &emptyRequest{}, grpcStorageMemory)},
		{MethodName: grpcMethodCompactionStats, Handler: unaryHandler(grpcMethodCompactionStats, &emptyRequest{}, grpcCompactionStats)},
		{MethodName: grpcMethodStorageValidate, Handler: unaryHandler(grpcMethodStorageValidate, &emptyRequest{}, grpcStorageValidate)},
		{MethodName: grpcMethodStorageSnapshot, Handler: unaryHandler(grpcMethodStorageSnapshot, &emptyRequest{}, grpcStorageSnapshot)},
		{MethodName: grpcMethodStorageExport, Handler: unaryHandler(grpcMethodStorageExport, &emptyRequest{}, grpcStorageExport)},
		{
			MethodName: grpcMethodCreateDownsamplePolicy,
			Handler:    unaryHandler(grpcMethodCreateDownsamplePolicy, &mts.DownsamplePolicy{}, grpcCreateDownsamplePolicy),
		},
		{
			MethodName: grpcMethodListDownsamplePolicies,
			Handler:    unaryHandler(grpcMethodListDownsamplePolicies, &emptyRequest{}, grpcListDownsamplePolicies),
		},
		{
			MethodName: grpcMethodEnableDownsamplePolicy,
			Handler:    unaryHandler(grpcMethodEnableDownsamplePolicy, &downsamplePolicyRequest{}, grpcEnableDownsamplePolicy),
		},
		{
			MethodName: grpcMethodDisableDownsamplePolicy,
			Handler:    unaryHandler(grpcMethodDisableDownsamplePolicy, &downsamplePolicyRequest{}, grpcDisableDownsamplePolicy),
		},
		{
			MethodName: grpcMethodDropDownsamplePolicy,
			Handler:    unaryHandler(grpcMethodDropDownsamplePolicy, &downsamplePolicyRequest{}, grpcDropDownsamplePolicy),
		},
		{
			MethodName: grpcMethodResetDownsamplePolicy,
			Handler:    unaryHandler(grpcMethodResetDownsamplePolicy, &grpcDownsampleResetRequest{}, grpcResetDownsamplePolicy),
		},
		{
			MethodName: grpcMethodDownsamplePolicyStatuses,
			Handler:    unaryHandler(grpcMethodDownsamplePolicyStatuses, &emptyRequest{}, grpcDownsamplePolicyStatuses),
		},
		{
			MethodName: grpcMethodRunDownsamplePolicy,
			Handler:    unaryHandler(grpcMethodRunDownsamplePolicy, &downsamplePolicyRangeRequest{}, grpcRunDownsamplePolicy),
		},
		{
			MethodName: grpcMethodRunDownsamplePolicyRange,
			Handler:    unaryHandler(grpcMethodRunDownsamplePolicyRange, &downsamplePolicyRangeRequest{}, grpcRunDownsamplePolicyRange),
		},
		{
			MethodName: grpcMethodRepairDownsamplePolicy,
			Handler:    unaryHandler(grpcMethodRepairDownsamplePolicy, &downsamplePolicyRangeRequest{}, grpcRepairDownsamplePolicy),
		},
		{
			MethodName: grpcMethodDryRunDownsamplePolicy,
			Handler:    unaryHandler(grpcMethodDryRunDownsamplePolicy, &downsamplePolicyRangeRequest{}, grpcDryRunDownsamplePolicy),
		},
	}
	return &grpc.ServiceDesc{ServiceName: grpcServiceName, HandlerType: (*grpcServiceServer)(nil), Methods: methods}
}

func grpcHealthHandler(service any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	handler := func(ctx context.Context, _ any) (any, error) {
		return service.(*grpcService).runtime.health(), nil
	}
	return invokeGRPCUnary(ctx, &emptyRequest{}, decode, interceptor, grpcFullMethod(grpcMethodHealth), handler)
}

func grpcWriteHandler(service any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	handler := func(ctx context.Context, req any) (any, error) {
		writeReq := req.(*writeRequest)
		for _, db := range writeRequestDatabases(*writeReq) {
			dbName := db
			if dbName == "__default__" {
				dbName = ""
			}
			if err := service.(*grpcService).runtime.authorizeGRPCDatabase(
				ctx,
				dbName,
				mts.DatabasePermissionWrite,
			); err != nil {
				return nil, grpcError(err)
			}
		}
		if err := service.(*grpcService).runtime.write(ctx, *writeReq); err != nil {
			return nil, grpcError(err)
		}
		return writeResponse{OK: true}, nil
	}
	return invokeGRPCUnary(ctx, &writeRequest{}, decode, interceptor, grpcFullMethod(grpcMethodWrite), handler)
}

func grpcQueryRowsHandler(service any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	handler := func(ctx context.Context, req any) (any, error) {
		queryReq := req.(*queryRowsRequest)
		if err := service.(*grpcService).runtime.authorizeGRPCDatabase(
			ctx,
			queryReq.Query.Database,
			mts.DatabasePermissionRead,
		); err != nil {
			return nil, grpcError(err)
		}
		rows, err := service.(*grpcService).runtime.queryRows(ctx, *queryReq)
		if err != nil {
			return nil, grpcError(err)
		}
		return queryRowsResponse{Rows: rows}, nil
	}
	return invokeGRPCUnary(ctx, &queryRowsRequest{}, decode, interceptor, grpcFullMethod(grpcMethodQueryRows), handler)
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
	return invokeGRPCUnary(ctx, &emptyRequest{}, decode, interceptor, grpcFullMethod(grpcMethodFlush), handler)
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
	return invokeGRPCUnary(ctx, &emptyRequest{}, decode, interceptor, grpcFullMethod(grpcMethodCompact), handler)
}

func unaryHandler(methodName string, prototype any, fn func(*serverRuntime, context.Context, any) (any, error)) grpc.MethodHandler {
	return func(service any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		req := newGRPCRequest(prototype)
		method := grpcFullMethod(methodName)
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

func newGRPCRequest(prototype any) any {
	typ := reflect.TypeOf(prototype)
	if typ == nil || typ.Kind() != reflect.Pointer {
		return prototype
	}
	return reflect.New(typ.Elem()).Interface()
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

func grpcLogin(r *serverRuntime, ctx context.Context, req any) (any, error) {
	request := req.(*loginRequest)
	ttl := defaultAuthTTL
	if request.TTLSeconds > 0 {
		ttl = time.Duration(request.TTLSeconds) * time.Second
	}
	token, err := r.engine.Authenticate(ctx, mts.Credentials{
		UserName: request.UserName,
		Password: request.Password,
	}, ttl)
	if err != nil {
		return nil, newAPIError(errorCodeUnauthenticated, "invalid credentials", err)
	}
	return authTokenResponse{Token: token}, nil
}

func grpcLogout(r *serverRuntime, ctx context.Context, req any) (any, error) {
	token := strings.TrimSpace(req.(*logoutRequest).Token)
	if token == "" {
		token = bearerToken(grpcMetadataValue(ctx, strings.ToLower(headerAuthorization)))
	}
	if token == "" {
		return nil, newAPIError(errorCodeUnauthenticated, "user bearer token is required", nil)
	}
	if err := r.engine.RevokeToken(ctx, token); err != nil {
		return nil, newAPIError(errorCodeUnauthenticated, "invalid user bearer token", err)
	}
	return okResponse{OK: true}, nil
}

func grpcChangePassword(r *serverRuntime, ctx context.Context, req any) (any, error) {
	request := req.(*changePasswordRequest)
	token := bearerToken(grpcMetadataValue(ctx, strings.ToLower(headerAuthorization)))
	if token == "" {
		return nil, newAPIError(errorCodeUnauthenticated, "user bearer token is required", nil)
	}
	principal, err := r.engine.VerifyToken(ctx, token)
	if err != nil {
		return nil, newAPIError(errorCodeUnauthenticated, "invalid user bearer token", err)
	}
	if strings.TrimSpace(request.UserName) != principal.UserName {
		return nil, mts.ErrPermissionDenied
	}
	if err := r.engine.ChangePassword(ctx, request.UserName, request.OldPassword, request.NewPassword); err != nil {
		return nil, newAPIError(errorCodeUnauthenticated, "invalid credentials", err)
	}
	return okResponse{OK: true}, nil
}

func grpcSetUserPassword(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*setUserPasswordRequest)
	return okResponse{OK: true}, r.engine.SetPassword(ctx, request.UserName, request.Password)
}

func grpcCreateUser(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return okResponse{OK: true}, r.createUserWithInitialPassword(ctx, *req.(*createUserRequest))
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

func grpcValidateConfig(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	resp := r.validateConfigPayload(req.(*configValidateRequest).Config)
	if !resp.OK {
		return nil, newAPIError(errorCodeBadRequest, resp.Error, nil)
	}
	return resp, nil
}

func grpcReloadConfig(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return r.reloadConfig()
}

func grpcGetAPISpec(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return apiSpec(), nil
}

func grpcGetErrorCodes(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return errorCodeSpecs(), nil
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

func grpcStorageValidate(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return r.storageValidate(), nil
}

func grpcStorageSnapshot(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return r.storageSnapshot(ctx)
}

func grpcStorageExport(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return storageExportResponse{Export: r.storageExport(ctx)}, nil
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
	classified := classifyAPIError(err)
	return status.Error(grpcCodeForErrorCode(classified.Code), classified.Message)
}

func grpcCodeForErrorCode(code errorCode) codes.Code {
	switch code {
	case errorCodeUnauthenticated:
		return codes.Unauthenticated
	case errorCodePermissionDenied:
		return codes.PermissionDenied
	case errorCodeResourceExhausted:
		return codes.ResourceExhausted
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

func grpcStatusCodeText(err error) string {
	if err == nil {
		return strconv.Itoa(int(codes.OK))
	}
	return status.Code(err).String()
}

func invokeGRPC(ctx context.Context, conn *grpc.ClientConn, method string, in any, out any) error {
	return conn.Invoke(ctx, grpcFullMethod(method), in, out, grpc.ForceCodec(jsonCodec{}))
}

func grpcFullMethod(method string) string {
	return "/" + grpcServiceName + "/" + method
}
