package main

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	mts "github.com/openmts/mts"
)

func TestGRPCP0P1DataUsersMetadataAdminAndDownsample(t *testing.T) {
	runtime := openTestRuntime(t)
	conn := openBufconnClient(t, runtime)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	}()
	ctx := context.Background()
	if err := invokeGRPC(ctx, conn, "CreateUser", &mts.User{Name: "grpc-alice"}, &okResponse{}); err != nil {
		t.Fatalf("CreateUser error = %v", err)
	}
	for _, permission := range []mts.DatabasePermission{mts.DatabasePermissionRead, mts.DatabasePermissionWrite} {
		req := databasePermissionRequest{UserName: "grpc-alice", Database: "default", Permission: permission}
		if err := invokeGRPC(ctx, conn, "GrantDatabasePermission", &req, &okResponse{}); err != nil {
			t.Fatalf("GrantDatabasePermission(%s) error = %v", permission, err)
		}
	}

	batch := mts.TypedBatch{
		Measurement: "cpu",
		Tags:        []mts.TagColumn{{Name: "host", Values: []string{"grpc-1"}}},
		Timestamps:  []int64{3},
		Fields: []mts.TypedFieldColumn{{
			Name:          "usage",
			Type:          mts.FieldFloat64,
			Float64Values: []float64{0.8},
		}},
	}
	if err := invokeGRPC(ctx, conn, "WriteTypedBatch", &typedWriteRequest{Batch: batch}, &writeResponse{}); err != nil {
		t.Fatalf("WriteTypedBatch error = %v", err)
	}
	var columns queryColumnsResponse
	if err := invokeGRPC(ctx, conn, "QueryColumns", &queryRequest{Query: testQuery()}, &columns); err != nil {
		t.Fatalf("QueryColumns error = %v", err)
	}
	if len(columns.Columns) != 1 || len(columns.Columns[0].Timestamps) != 1 {
		t.Fatalf("columns = %#v, want one point", columns.Columns)
	}
	var explain queryExplainResponse
	if err := invokeGRPC(ctx, conn, "QueryWithExplain", &queryRequest{Query: testQuery()}, &explain); err != nil {
		t.Fatalf("QueryWithExplain error = %v", err)
	}
	if explain.Result.Explain.Measurement != "cpu" {
		t.Fatalf("explain = %#v, want cpu", explain.Result.Explain)
	}
	var fields fieldsResponse
	if err := invokeGRPC(ctx, conn, "ListFields", &metadataRequest{Database: "default", Measurement: "cpu"}, &fields); err != nil {
		t.Fatalf("ListFields error = %v", err)
	}
	if len(fields.Fields) != 1 {
		t.Fatalf("fields = %#v, want one", fields.Fields)
	}
	var cfg configResponse
	if err := invokeGRPC(ctx, conn, "GetEffectiveConfig", &emptyRequest{}, &cfg); err != nil {
		t.Fatalf("GetEffectiveConfig error = %v", err)
	}
	if cfg.Config.DataDir == "" {
		t.Fatalf("config = %#v, want data dir", cfg.Config)
	}
	policy := testDownsamplePolicy()
	policy.Name = "grpc_rollup_cpu"
	if err := invokeGRPC(ctx, conn, "CreateDownsamplePolicy", &policy, &okResponse{}); err != nil {
		t.Fatalf("CreateDownsamplePolicy error = %v", err)
	}
	var dryRun downsampleDryRunResponse
	if err := invokeGRPC(ctx, conn, "DryRunDownsamplePolicy", &downsamplePolicyRangeRequest{
		Name:      policy.Name,
		StartUnix: 1,
		EndUnix:   int64(time.Hour / time.Second),
	}, &dryRun); err != nil {
		t.Fatalf("DryRunDownsamplePolicy error = %v", err)
	}
}

func TestGRPCRequireUserAuthenticatesBearerToken(t *testing.T) {
	runtime := openTestRuntime(t)
	runtime.config.Auth.RequireUser = true
	conn := openBufconnClient(t, runtime)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	}()
	ctx := context.Background()
	seedUserWithPassword(t, runtime, mts.User{Name: "grpc-auth"}, "secret")
	seedDatabasePermission(t, runtime, "grpc-auth", "default", mts.DatabasePermissionWrite)

	var writeResp writeResponse
	err := invokeGRPC(ctx, conn, "Write", &writeRequest{Points: []mts.Point{testPoint()}}, &writeResp)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("Write(no token) code = %v, want Unauthenticated, err=%v", status.Code(err), err)
	}
	badCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer bad-token"))
	err = invokeGRPC(badCtx, conn, "Write", &writeRequest{Points: []mts.Point{testPoint()}}, &writeResp)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("Write(bad token) code = %v, want Unauthenticated, err=%v", status.Code(err), err)
	}

	var login authTokenResponse
	invokeOK(t, ctx, conn, "Login", &loginRequest{UserName: "grpc-auth", Password: "secret", TTLSeconds: 60}, &login)
	if login.Token.Token == "" {
		t.Fatal("login token is empty")
	}
	goodCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+login.Token.Token))
	invokeOK(t, goodCtx, conn, "Write", &writeRequest{Points: []mts.Point{testPoint()}}, &writeResp)
}

func TestGRPCRequireUserBootstrapsDefaultAdmin(t *testing.T) {
	runtime := openTestRuntimeRequireUser(t)
	conn := openBufconnClient(t, runtime)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	}()

	ctx := context.Background()
	err := invokeGRPC(ctx, conn, "ListUsers", &emptyRequest{}, &usersResponse{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("ListUsers(no token) code = %v, want Unauthenticated, err=%v", status.Code(err), err)
	}

	var login authTokenResponse
	invokeOK(t, ctx, conn, "Login", &loginRequest{UserName: "admin", Password: "admin"}, &login)
	adminCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+login.Token.Token))
	var users usersResponse
	invokeOK(t, adminCtx, conn, "ListUsers", &emptyRequest{}, &users)
	if len(users.Users) != 1 || users.Users[0].Name != "admin" || users.Users[0].Role != mts.UserRoleAdmin {
		t.Fatalf("users = %#v, want default admin", users.Users)
	}
}

func TestGRPCCreateUserAcceptsInitialPassword(t *testing.T) {
	runtime := openTestRuntime(t)
	conn := openBufconnClient(t, runtime)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	}()

	ctx := context.Background()
	invokeOK(t, ctx, conn, "CreateUser", &createUserRequest{
		User:     mts.User{Name: "grpc-created-with-password"},
		Password: "secret",
	}, &okResponse{})

	var login authTokenResponse
	invokeOK(t, ctx, conn, "Login", &loginRequest{
		UserName: "grpc-created-with-password",
		Password: "secret",
	}, &login)
	if login.Token.Token == "" {
		t.Fatal("login token is empty")
	}

	var user userResponse
	invokeOK(t, ctx, conn, "GetUser", &userNameRequest{Name: "grpc-created-with-password"}, &user)
	if user.User.Name != "grpc-created-with-password" {
		t.Fatalf("user = %#v, want grpc-created-with-password", user.User)
	}
}

func TestGRPCCreateUserRollsBackWhenInitialPasswordInvalid(t *testing.T) {
	runtime := openTestRuntime(t)
	conn := openBufconnClient(t, runtime)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	}()

	ctx := context.Background()
	err := invokeGRPC(ctx, conn, "CreateUser", &createUserRequest{
		User:     mts.User{Name: "grpc-rollback-user"},
		Password: " ",
	}, &okResponse{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("CreateUser(invalid password) code = %v, want Unauthenticated, err=%v", status.Code(err), err)
	}
	err = invokeGRPC(ctx, conn, "GetUser", &userNameRequest{Name: "grpc-rollback-user"}, &userResponse{})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("GetUser(rollback) code = %v, want NotFound, err=%v", status.Code(err), err)
	}
}

func TestGRPCUserRoleControlsUserManagement(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	runtime.config.Auth.RequireUser = true
	conn := openBufconnClient(t, runtime)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	}()
	ctx := context.Background()
	serviceAdminCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer test-admin-token"))
	// openTestRuntime 已 bootstrap 默认 admin，这里只重置密码。
	invokeOK(t, serviceAdminCtx, conn, "SetUserPassword", &setUserPasswordRequest{UserName: "admin", Password: "admin-secret"}, &okResponse{})
	invokeOK(t, serviceAdminCtx, conn, "CreateUser", &mts.User{Name: "alice"}, &okResponse{})
	invokeOK(t, serviceAdminCtx, conn, "SetUserPassword", &setUserPasswordRequest{UserName: "alice", Password: "alice-secret"}, &okResponse{})
	invokeOK(t, serviceAdminCtx, conn, "CreateUser", &mts.User{Name: "bob"}, &okResponse{})

	var login authTokenResponse
	invokeOK(t, ctx, conn, "Login", &loginRequest{UserName: "alice", Password: "alice-secret", TTLSeconds: 60}, &login)
	aliceCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+login.Token.Token))
	err := invokeGRPC(ctx, conn, "DeleteUser", &userNameRequest{Name: "bob"}, &okResponse{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("DeleteUser(no token) code = %v, want Unauthenticated, err=%v", status.Code(err), err)
	}
	err = invokeGRPC(aliceCtx, conn, "DeleteUser", &userNameRequest{Name: "bob"}, &okResponse{})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("DeleteUser(as user) code = %v, want PermissionDenied, err=%v", status.Code(err), err)
	}
	err = invokeGRPC(aliceCtx, conn, "SetUserPassword", &setUserPasswordRequest{UserName: "bob", Password: "next"}, &okResponse{})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("SetUserPassword(as user) code = %v, want PermissionDenied, err=%v", status.Code(err), err)
	}
	err = invokeGRPC(aliceCtx, conn, "GrantDatabasePermission", &databasePermissionRequest{
		UserName:   "bob",
		Database:   "default",
		Permission: mts.DatabasePermissionRead,
	}, &okResponse{})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("GrantDatabasePermission(as user) code = %v, want PermissionDenied, err=%v", status.Code(err), err)
	}
	err = invokeGRPC(aliceCtx, conn, "RevokeDatabasePermission", &databasePermissionRequest{
		UserName:   "bob",
		Database:   "default",
		Permission: mts.DatabasePermissionRead,
	}, &okResponse{})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("RevokeDatabasePermission(as user) code = %v, want PermissionDenied, err=%v", status.Code(err), err)
	}

	invokeOK(t, ctx, conn, "Login", &loginRequest{UserName: "admin", Password: "admin-secret", TTLSeconds: 60}, &login)
	adminCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+login.Token.Token))
	invokeOK(t, adminCtx, conn, "SetUserPassword", &setUserPasswordRequest{UserName: "bob", Password: "next"}, &okResponse{})
	invokeOK(t, adminCtx, conn, "GrantDatabasePermission", &databasePermissionRequest{
		UserName:   "bob",
		Database:   "default",
		Permission: mts.DatabasePermissionRead,
	}, &okResponse{})
	invokeOK(t, adminCtx, conn, "DeleteUser", &userNameRequest{Name: "bob"}, &okResponse{})
}

func TestGRPCUserCanOnlyChangeOwnPassword(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	conn := openBufconnClient(t, runtime)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	}()
	ctx := context.Background()
	serviceAdminCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer test-admin-token"))
	invokeOK(t, serviceAdminCtx, conn, "CreateUser", &mts.User{Name: "alice"}, &okResponse{})
	invokeOK(t, serviceAdminCtx, conn, "SetUserPassword", &setUserPasswordRequest{UserName: "alice", Password: "alice-secret"}, &okResponse{})
	invokeOK(t, serviceAdminCtx, conn, "CreateUser", &mts.User{Name: "bob"}, &okResponse{})
	invokeOK(t, serviceAdminCtx, conn, "SetUserPassword", &setUserPasswordRequest{UserName: "bob", Password: "bob-secret"}, &okResponse{})

	var login authTokenResponse
	invokeOK(t, ctx, conn, "Login", &loginRequest{UserName: "alice", Password: "alice-secret", TTLSeconds: 60}, &login)
	aliceCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+login.Token.Token))
	err := invokeGRPC(aliceCtx, conn, "ChangePassword", &changePasswordRequest{
		UserName:    "bob",
		OldPassword: "bob-secret",
		NewPassword: "blocked",
	}, &okResponse{})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("ChangePassword(other user) code = %v, want PermissionDenied, err=%v", status.Code(err), err)
	}
	invokeOK(t, aliceCtx, conn, "ChangePassword", &changePasswordRequest{
		UserName:    "alice",
		OldPassword: "alice-secret",
		NewPassword: "alice-next",
	}, &okResponse{})

	err = invokeGRPC(ctx, conn, "Login", &loginRequest{UserName: "alice", Password: "alice-secret", TTLSeconds: 60}, &authTokenResponse{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("Login(old password) code = %v, want Unauthenticated, err=%v", status.Code(err), err)
	}
	invokeOK(t, ctx, conn, "Login", &loginRequest{UserName: "alice", Password: "alice-next", TTLSeconds: 60}, &authTokenResponse{})
}

func TestGRPCWriteAndQueryRows(t *testing.T) {
	runtime := openTestRuntime(t)
	conn := openBufconnClient(t, runtime)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	}()

	ctx := context.Background()
	var writeResp writeResponse
	if err := invokeGRPC(ctx, conn, "Write", &writeRequest{Points: []mts.Point{testPoint()}}, &writeResp); err != nil {
		t.Fatalf("Invoke Write error = %v", err)
	}
	if !writeResp.OK {
		t.Fatal("Write OK = false, want true")
	}

	var rowsResp queryRowsResponse
	if err := invokeGRPC(ctx, conn, "QueryRows", &queryRowsRequest{Query: testQuery()}, &rowsResp); err != nil {
		t.Fatalf("Invoke QueryRows error = %v", err)
	}
	if len(rowsResp.Rows) != 1 || rowsResp.Rows[0].Fields["usage"].Float64 != 0.7 {
		t.Fatalf("rows = %#v, want one usage row", rowsResp.Rows)
	}
}

func TestGRPCHealth(t *testing.T) {
	runtime := openTestRuntime(t)
	conn := openBufconnClient(t, runtime)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	}()

	var health mts.HealthSnapshot
	if err := invokeGRPC(context.Background(), conn, "Health", &emptyRequest{}, &health); err != nil {
		t.Fatalf("Invoke Health error = %v", err)
	}
	if !health.Healthy {
		t.Fatalf("health = %#v, want healthy", health)
	}
}

func TestGRPCMaintenanceAndErrors(t *testing.T) {
	runtime := openTestRuntime(t)
	conn := openBufconnClient(t, runtime)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	}()

	ctx := context.Background()

	var flushResp maintenanceResponse
	if err := invokeGRPC(ctx, conn, "Flush", &emptyRequest{}, &flushResp); err != nil {
		t.Fatalf("Invoke Flush error = %v", err)
	}
	if !flushResp.OK {
		t.Fatal("Flush OK = false, want true")
	}

	var compactResp maintenanceResponse
	if err := invokeGRPC(ctx, conn, "Compact", &emptyRequest{}, &compactResp); err != nil {
		t.Fatalf("Invoke Compact error = %v", err)
	}
	if !compactResp.OK {
		t.Fatal("Compact OK = false, want true")
	}

	var writeResp writeResponse
	err := invokeGRPC(ctx, conn, "Write", "bad request", &writeResp)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("Invoke Write(bad) error = %v, want InvalidArgument", err)
	}
}

func TestGRPCHandlerWithInterceptor(t *testing.T) {
	runtime := openTestRuntime(t)
	called := false
	interceptor := func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		called = true
		if info.FullMethod != "/mts.v1.MTSServer/Health" {
			t.Fatalf("FullMethod = %s, want health method", info.FullMethod)
		}
		return handler(ctx, req)
	}
	resp, err := grpcHealthHandler(
		&grpcService{runtime: runtime},
		context.Background(),
		func(value any) error { return jsonCodec{}.Unmarshal([]byte(`{}`), value) },
		interceptor,
	)
	if err != nil {
		t.Fatalf("grpcHealthHandler() error = %v", err)
	}
	if !called {
		t.Fatal("interceptor was not called")
	}
	health, ok := resp.(mts.HealthSnapshot)
	if !ok || !health.Healthy {
		t.Fatalf("health response = %#v, want healthy snapshot", resp)
	}
}

func TestGRPCWriteHandlerBusinessError(t *testing.T) {
	runtime := openTestRuntime(t)
	decode := func(value any) error {
		return jsonCodec{}.Unmarshal([]byte(`{"points":[{"measurement":" "}]}`), value)
	}
	_, err := grpcWriteHandler(&grpcService{runtime: runtime}, context.Background(), decode, nil)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("grpcWriteHandler error = %v, want InvalidArgument", err)
	}
}

func TestGRPCQueryRowsHandlerBusinessError(t *testing.T) {
	runtime := openTestRuntime(t)
	decode := func(value any) error {
		return jsonCodec{}.Unmarshal([]byte(`{"query":{"measurement":"cpu","precision":"minute"}}`), value)
	}
	_, err := grpcQueryRowsHandler(&grpcService{runtime: runtime}, context.Background(), decode, nil)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("grpcQueryRowsHandler error = %v, want InvalidArgument", err)
	}
}

func TestGRPCServiceMarker(t *testing.T) {
	service := &grpcService{}
	service.mtsServer()
}

func TestGRPCListDatabasesAdminHealthAndDropDownsampleOptions(t *testing.T) {
	runtime := openTestRuntime(t)
	conn := openBufconnClient(t, runtime)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	}()
	ctx := context.Background()
	if err := invokeGRPC(ctx, conn, "CreateDatabase", &databaseRequest{Name: "metrics"}, &okResponse{}); err != nil {
		t.Fatalf("CreateDatabase error = %v", err)
	}
	var dbs databasesResponse
	if err := invokeGRPC(ctx, conn, "ListDatabases", &emptyRequest{}, &dbs); err != nil {
		t.Fatalf("ListDatabases error = %v", err)
	}
	if len(dbs.Databases) == 0 {
		t.Fatalf("databases = %#v, want non-empty", dbs.Databases)
	}
	var health adminHealthResponse
	if err := invokeGRPC(ctx, conn, "AdminHealth", &emptyRequest{}, &health); err != nil {
		t.Fatalf("AdminHealth error = %v", err)
	}
	if !health.Health.Ready && !health.Health.Healthy {
		// open runtime should be healthy/ready in tests; accept either flag true.
		t.Fatalf("health = %#v, want healthy runtime", health.Health)
	}
	policy := testDownsamplePolicy()
	policy.Name = "drop_opts_policy"
	if err := invokeGRPC(ctx, conn, "CreateDownsamplePolicy", &policy, &okResponse{}); err != nil {
		t.Fatalf("CreateDownsamplePolicy error = %v", err)
	}
	if err := invokeGRPC(ctx, conn, "DropDownsamplePolicy", &grpcDownsampleDropRequest{
		Name:    policy.Name,
		Options: mts.DownsampleDropOptions{CleanupTarget: false},
	}, &okResponse{}); err != nil {
		t.Fatalf("DropDownsamplePolicy(options) error = %v", err)
	}
}

func TestGRPCQueryStreamRowAndColumn(t *testing.T) {
	runtime := openTestRuntime(t)
	conn := openBufconnClient(t, runtime)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("Close(conn) error = %v", err)
		}
	}()
	ctx := context.Background()
	batch := mts.TypedBatch{
		Measurement: "cpu",
		Tags:        []mts.TagColumn{{Name: "host", Values: []string{"stream-1", "stream-1"}}},
		Timestamps:  []int64{1, 2},
		Fields: []mts.TypedFieldColumn{{
			Name:          "usage",
			Type:          mts.FieldFloat64,
			Float64Values: []float64{0.1, 0.2},
		}},
	}
	if err := invokeGRPC(ctx, conn, "WriteTypedBatch", &typedWriteRequest{Batch: batch}, &writeResponse{}); err != nil {
		t.Fatalf("WriteTypedBatch error = %v", err)
	}
	rowRecords, err := invokeGRPCQueryStream(ctx, conn, queryStreamRequest{Query: testQuery(), Format: "row"})
	if err != nil {
		t.Fatalf("QueryStream(row) error = %v", err)
	}
	if !streamRecordsContainType(rowRecords, streamTypeRow) || !streamRecordsContainType(rowRecords, streamTypeEnd) {
		t.Fatalf("row stream = %#v, want row and end", rowRecords)
	}
	columnRecords, err := invokeGRPCQueryStream(ctx, conn, queryStreamRequest{Query: testQuery(), Format: "column"})
	if err != nil {
		t.Fatalf("QueryStream(column) error = %v", err)
	}
	if !streamRecordsContainType(columnRecords, streamTypeColumn) || !streamRecordsContainType(columnRecords, streamTypeEnd) {
		t.Fatalf("column stream = %#v, want column and end", columnRecords)
	}
}

func invokeGRPCQueryStream(ctx context.Context, conn *grpc.ClientConn, req queryStreamRequest) ([]streamRecord, error) {
	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{
		StreamName:    grpcMethodQueryStream,
		ServerStreams: true,
	}, grpcFullMethod(grpcMethodQueryStream))
	if err != nil {
		return nil, err
	}
	if err := stream.SendMsg(&req); err != nil {
		return nil, err
	}
	if err := stream.CloseSend(); err != nil {
		return nil, err
	}
	var out []streamRecord
	for {
		var rec streamRecord
		err := stream.RecvMsg(&rec)
		if err != nil {
			if err == io.EOF {
				return out, nil
			}
			return out, err
		}
		out = append(out, rec)
		if rec.Type == streamTypeEnd || rec.Type == streamTypeError {
			return out, nil
		}
	}
}

func streamRecordsContainType(records []streamRecord, want string) bool {
	for _, rec := range records {
		if rec.Type == want {
			return true
		}
	}
	return false
}

func openBufconnClient(t *testing.T, runtime *serverRuntime) *grpc.ClientConn {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server, err := newGRPCServer(runtime)
	if err != nil {
		t.Fatalf("newGRPCServer() error = %v", err)
	}
	go func() {
		if err := server.Serve(listener); err != nil {
			t.Errorf("Serve(bufconn) error = %v", err)
		}
	}()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})),
	)
	if err != nil {
		t.Fatalf("NewClient(bufconn) error = %v", err)
	}
	return conn
}
