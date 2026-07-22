package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	return &grpc.ServiceDesc{
		ServiceName: grpcServiceName,
		HandlerType: (*grpcServiceServer)(nil),
		Methods:     grpcMethodsFromRegistry(),
		Streams:     grpcStreamsFromRegistry(),
	}
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
				return nil, grpcError(ctx, err)
			}
		}
		if err := service.(*grpcService).runtime.write(ctx, *writeReq); err != nil {
			return nil, grpcError(ctx, err)
		}
		return service.(*grpcService).runtime.attachAdminOpToWrite(writeResponse{OK: true}), nil
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
			return nil, grpcError(ctx, err)
		}
		rows, err := service.(*grpcService).runtime.queryRows(ctx, *queryReq)
		if err != nil {
			return nil, grpcError(ctx, err)
		}
		return queryRowsResponse{Rows: rows, Stats: service.(*grpcService).runtime.queryStats()}, nil
	}
	return invokeGRPCUnary(ctx, &queryRowsRequest{}, decode, interceptor, grpcFullMethod(grpcMethodQueryRows), handler)
}

func grpcFlushHandler(service any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	handler := func(ctx context.Context, _ any) (any, error) {
		if err := service.(*grpcService).runtime.requireGRPCAdmin(ctx); err != nil {
			return nil, grpcError(ctx, err)
		}
		if err := service.(*grpcService).runtime.flush(ctx); err != nil {
			return nil, grpcError(ctx, err)
		}
		return service.(*grpcService).runtime.attachAdminOpToMaintenance(maintenanceResponse{OK: true}), nil
	}
	return invokeGRPCUnary(ctx, &emptyRequest{}, decode, interceptor, grpcFullMethod(grpcMethodFlush), handler)
}

func grpcCompactHandler(service any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	handler := func(ctx context.Context, _ any) (any, error) {
		if err := service.(*grpcService).runtime.requireGRPCAdmin(ctx); err != nil {
			return nil, grpcError(ctx, err)
		}
		result, err := service.(*grpcService).runtime.compact(ctx)
		if err != nil {
			return nil, grpcError(ctx, err)
		}
		return service.(*grpcService).runtime.attachAdminOpToMaintenance(maintenanceResponse{OK: true, Result: result}), nil
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
				return nil, grpcError(ctx, err)
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
	if err := r.writeTypedBatch(ctx, *request); err != nil {
		return nil, err
	}
	return r.attachAdminOpToWrite(writeResponse{OK: true}), nil
}

func grpcWritePointsAsTypedBatch(r *serverRuntime, ctx context.Context, req any) (any, error) {
	request := req.(*writeRequest)
	for _, db := range writeRequestDatabases(*request) {
		dbName := db
		if dbName == "__default__" {
			dbName = ""
		}
		if err := r.authorizeGRPCDatabase(ctx, dbName, mts.DatabasePermissionWrite); err != nil {
			return nil, err
		}
	}
	if err := r.writePointsAsTyped(ctx, *request); err != nil {
		return nil, err
	}
	return r.attachAdminOpToWrite(writeResponse{OK: true}), nil
}

func grpcDelete(r *serverRuntime, ctx context.Context, req any) (any, error) {
	request := req.(*deleteRequest)
	if err := r.authorizeGRPCDatabase(ctx, request.Request.Database, mts.DatabasePermissionWrite); err != nil {
		return nil, err
	}
	if err := r.deleteData(ctx, request.Request); err != nil {
		return nil, err
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}

func grpcQueryColumns(r *serverRuntime, ctx context.Context, req any) (any, error) {
	request := req.(*queryRequest)
	if err := r.authorizeGRPCDatabase(ctx, request.Query.Database, mts.DatabasePermissionRead); err != nil {
		return nil, err
	}
	columns, err := r.queryColumns(ctx, *request)
	if err != nil {
		return queryColumnsResponse{}, err
	}
	return queryColumnsResponse{Columns: columns, Stats: r.queryStats()}, nil
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
	return r.queryStatsPayload(), nil
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
	mustChange := false
	if user, ok, getErr := r.engine.GetUser(ctx, request.UserName); getErr == nil && ok {
		mustChange = userMustChangePassword(user)
	}
	return authTokenResponse{Token: token, MustChangePassword: mustChange}, nil
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
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}

func grpcGetSession(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	token := bearerToken(grpcMetadataValue(ctx, strings.ToLower(headerAuthorization)))
	if token == "" {
		return nil, newAPIError(errorCodeUnauthenticated, "user bearer token is required", nil)
	}
	principal, err := r.engine.VerifyToken(ctx, token)
	if err != nil {
		return nil, newAPIError(errorCodeUnauthenticated, "invalid user bearer token", err)
	}
	mustChange := false
	if user, ok, getErr := r.engine.GetUser(ctx, principal.UserName); getErr == nil && ok {
		mustChange = userMustChangePassword(user)
	}
	remaining := int64(0)
	if !principal.ExpiresAt.IsZero() {
		sec := int64(time.Until(principal.ExpiresAt).Seconds())
		if sec > 0 {
			remaining = sec
		}
	}
	return r.attachAdminOpToSession(sessionResponse{
		OK:                 true,
		UserName:           principal.UserName,
		Role:               principal.Role,
		ExpiresAt:          principal.ExpiresAt,
		MustChangePassword: mustChange,
		RemainingSeconds:   remaining,
		ServerTimeUnix:     time.Now().Unix(),
	}), nil
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
	if err := r.clearMustChangePassword(ctx, principal.UserName); err != nil {
		return nil, err
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}

func grpcSetUserPassword(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*setUserPasswordRequest)
	if err := r.engine.SetPassword(ctx, request.UserName, request.Password); err != nil {
		return nil, err
	}
	if err := r.clearMustChangePassword(ctx, request.UserName); err != nil {
		return nil, err
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}

func grpcCreateUser(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	if err := r.createUserWithInitialPassword(ctx, *req.(*createUserRequest)); err != nil {
		return nil, err
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}

func grpcUpdateUser(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	if err := r.engine.UpdateUser(ctx, *req.(*mts.User)); err != nil {
		return nil, err
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
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
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToUsers(usersResponse{Users: users}), nil
}

func grpcDeleteUser(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	if err := r.engine.DeleteUser(ctx, req.(*userNameRequest).Name); err != nil {
		return nil, err
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}

func grpcGrantDatabasePermission(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*databasePermissionRequest)
	if err := r.engine.GrantDatabasePermission(ctx, request.UserName, request.Database, request.Permission); err != nil {
		return nil, err
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}

func grpcRevokeDatabasePermission(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*databasePermissionRequest)
	if err := r.engine.RevokeDatabasePermission(ctx, request.UserName, request.Database, request.Permission); err != nil {
		return nil, err
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}

func grpcListDatabasePermissions(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	grants, err := r.engine.ListDatabasePermissions(ctx, req.(*userNameRequest).Name)
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToPermissions(databasePermissionsResponse{Grants: grants}), nil
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
	if err := r.engine.CreateDatabase(ctx, req.(*databaseRequest).Name); err != nil {
		return nil, err
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}

func grpcListDatabases(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	databases, err := r.engine.ListDatabases(ctx)
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToDatabases(databasesResponse{Databases: databases}), nil
}

func grpcDropDatabase(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	if err := r.engine.DropDatabase(ctx, req.(*databaseRequest).Name); err != nil {
		return nil, err
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}

func grpcCreateRetentionPolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*grpcRetentionPolicyRequest)
	if err := r.engine.CreateRetentionPolicy(ctx, request.Database, request.Policy); err != nil {
		return nil, err
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}

func grpcListRetentionPolicies(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	policies, err := r.engine.ListRetentionPolicies(ctx, req.(*databaseRequest).Name)
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToRetentionPolicies(retentionPoliciesResponse{Policies: policies}), nil
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
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToFields(fieldsResponse{Fields: fields}), nil
}

func grpcListSeries(r *serverRuntime, ctx context.Context, req any) (any, error) {
	request := req.(*metadataRequest)
	if err := r.authorizeGRPCDatabase(ctx, request.Database, mts.DatabasePermissionRead); err != nil {
		return nil, err
	}
	series, err := r.engine.ListSeries(ctx, request.Database, request.Measurement, request.Tags)
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToSeries(seriesResponse{Series: series}), nil
}

func grpcGetConfig(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return r.configPayload(), nil
}

func grpcGetConfigSchema(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return r.configSchemaPayload(), nil
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
	resp, err := r.reloadConfig()
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToReload(resp), nil
}

func grpcGetAPISpec(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return r.apiSpecPayload(), nil
}

func grpcGetErrorCodes(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return r.errorCodesPayload(), nil
}

func grpcApplyRetention(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	if err := r.applyRetention(ctx, *req.(*retentionApplyRequest)); err != nil {
		return nil, err
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}

func grpcMaintenanceErrors(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	busy, op, started := r.adminHeavyState()
	return maintenanceErrorsResponse{
		Errors:        r.maintenanceErrors(ctx),
		AdminOpBusy:   busy,
		Op:            op,
		StartedAtUnix: started,
		Last:          r.lastAdminHeavySnapshot(),
	}, nil
}

func grpcMaintenanceStats(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return r.maintenanceStatsPayload(), nil
}

func grpcOpsStatus(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return r.opsStatusPayload(), nil
}

func grpcAdminHealth(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return r.adminHealthPayload(), nil
}

func grpcStorageMemory(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return r.storageMemoryPayload(), nil
}

func grpcCompactionStats(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return r.compactionStatsPayload(), nil
}

func grpcStorageValidate(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return r.storageValidatePayload(), nil
}

func grpcStorageSnapshot(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	resp, err := r.storageSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToStorageSnapshot(resp), nil
}

func grpcStorageExport(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	return r.storageExportPayload(ctx), nil
}

func grpcCreateDownsamplePolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	if err := r.engine.CreateDownsamplePolicy(ctx, *req.(*mts.DownsamplePolicy)); err != nil {
		return nil, err
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}

func grpcListDownsamplePolicies(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	policies, err := r.engine.ListDownsamplePolicies(ctx)
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToDownsamplePolicies(downsamplePoliciesResponse{Policies: policies}), nil
}

func grpcEnableDownsamplePolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	if err := r.engine.EnableDownsamplePolicy(ctx, req.(*downsamplePolicyRequest).Name); err != nil {
		return nil, err
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}

func grpcDisableDownsamplePolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	if err := r.engine.DisableDownsamplePolicy(ctx, req.(*downsamplePolicyRequest).Name); err != nil {
		return nil, err
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}

func grpcDropDownsamplePolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	switch request := req.(type) {
	case *grpcDownsampleDropRequest:
		if err := r.engine.DropDownsamplePolicyWithOptions(ctx, request.Name, request.Options); err != nil {
			return nil, err
		}
		return r.attachAdminOpToOK(okResponse{OK: true}), nil
	case *downsamplePolicyRequest:
		// 兼容仅传 name 的旧请求。
		if err := r.engine.DropDownsamplePolicy(ctx, request.Name); err != nil {
			return nil, err
		}
		return r.attachAdminOpToOK(okResponse{OK: true}), nil
	default:
		return nil, newAPIError(errorCodeBadRequest, "invalid drop downsample request", nil)
	}
}

func grpcResetDownsamplePolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*grpcDownsampleResetRequest)
	if err := r.engine.ResetDownsamplePolicy(ctx, request.Name, request.Reset); err != nil {
		return nil, err
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}

func grpcDownsamplePolicyStatuses(r *serverRuntime, ctx context.Context, _ any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	statuses, err := r.engine.DownsamplePolicyStatuses(ctx, timeNow())
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToDownsampleStatuses(downsampleStatusesResponse{Statuses: statuses}), nil
}

func grpcRunDownsamplePolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*downsamplePolicyRangeRequest)
	result, err := r.engine.RunDownsamplePolicy(ctx, request.Name, unixSecondsOrNow(request.EndUnix))
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToDownsampleRun(downsampleRunResponse{Result: result}), nil
}

func grpcRunDownsamplePolicyRange(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*downsamplePolicyRangeRequest)
	result, err := r.engine.RunDownsamplePolicyRange(ctx, request.Name, unixFlexible(request.StartUnix), unixFlexible(request.EndUnix), request.Options)
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToDownsampleRun(downsampleRunResponse{Result: result}), nil
}

func grpcRepairDownsamplePolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*downsamplePolicyRangeRequest)
	result, err := r.engine.RepairDownsamplePolicy(ctx, request.Name, unixFlexible(request.StartUnix), unixFlexible(request.EndUnix))
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToDownsampleRun(downsampleRunResponse{Result: result}), nil
}

func grpcDryRunDownsamplePolicy(r *serverRuntime, ctx context.Context, req any) (any, error) {
	if err := r.requireGRPCAdmin(ctx); err != nil {
		return nil, err
	}
	request := req.(*downsamplePolicyRangeRequest)
	result, err := r.engine.DryRunDownsamplePolicy(ctx, request.Name, unixFlexible(request.StartUnix), unixFlexible(request.EndUnix))
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToDownsampleDryRun(downsampleDryRunResponse{Result: result}), nil
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

func grpcError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	classified := classifyAPIError(err)
	var apiErr apiError
	if errors.As(err, &apiErr) && (apiErr.AdminOpBusy || apiErr.Op != "") {
		pairs := []string{metadataAdminOpBusy, "true"}
		if apiErr.Op != "" {
			pairs = append(pairs, metadataAdminOp, apiErr.Op)
		}
		_ = grpc.SetHeader(ctx, metadata.Pairs(pairs...))
		_ = grpc.SetTrailer(ctx, metadata.Pairs(pairs...))
	}
	return status.Error(grpcCodeForErrorCode(classified.Code), classified.Message)
}

// grpcErrorPlain 无上下文时的兼容包装（测试/内部）。
func grpcErrorPlain(err error) error {
	return grpcError(context.Background(), err)
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

func grpcListAudit(r *serverRuntime, _ context.Context, req any) (any, error) {
	request, _ := req.(*auditListRequest)
	if request == nil {
		request = &auditListRequest{}
	}
	events := r.audit.listFiltered(*request)
	return r.attachAdminOpToAudit(auditListResponse{Events: events, Total: len(events)}), nil
}

func grpcListStorageSnapshots(r *serverRuntime, _ context.Context, _ any) (any, error) {
	resp, err := r.listStorageSnapshots()
	if err != nil {
		return nil, err
	}
	return r.attachAdminOpToSnapshots(resp), nil
}

func grpcDeleteStorageSnapshot(r *serverRuntime, ctx context.Context, req any) (any, error) {
	request, _ := req.(*storageSnapshotDeleteRequest)
	if request == nil || strings.TrimSpace(request.Name) == "" {
		return nil, grpcError(ctx, fmt.Errorf("name is required"))
	}
	if err := r.deleteStorageSnapshot(request.Name); err != nil {
		return nil, grpcError(ctx, err)
	}
	return r.attachAdminOpToOK(okResponse{OK: true}), nil
}
