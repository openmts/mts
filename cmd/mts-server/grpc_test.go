package main

import (
	"context"
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
	invokeOK(t, ctx, conn, "CreateUser", &mts.User{Name: "grpc-auth"}, &okResponse{})
	invokeOK(t, ctx, conn, "SetUserPassword", &setUserPasswordRequest{UserName: "grpc-auth", Password: "secret"}, &okResponse{})
	invokeOK(t, ctx, conn, "GrantDatabasePermission", &databasePermissionRequest{
		UserName: "grpc-auth", Database: "default", Permission: mts.DatabasePermissionWrite,
	}, &okResponse{})

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
