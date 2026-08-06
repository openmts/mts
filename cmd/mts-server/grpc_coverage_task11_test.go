package main

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/metadata"

	mts "github.com/openmts/mts"
)

func TestGRPCDataWrapperContracts(t *testing.T) {
	runtime := openTestRuntime(t)
	ctx := context.Background()

	batch := mts.TypedBatch{
		Database:    "default",
		Measurement: "cpu",
		Timestamps:  []int64{1},
		Fields: []mts.TypedFieldColumn{{
			Name:          "usage",
			Type:          mts.FieldFloat64,
			Float64Values: []float64{0.7},
		}},
	}
	typed, err := grpcWriteTypedBatch(runtime, ctx, &typedWriteRequest{Batch: batch})
	if err != nil {
		t.Fatalf("grpcWriteTypedBatch() error = %v", err)
	}
	if resp := typed.(writeResponse); !resp.OK || resp.Mode != "typed" || resp.Points != 1 {
		t.Fatalf("grpcWriteTypedBatch() = %#v", resp)
	}

	points, err := grpcWritePointsAsTypedBatch(runtime, ctx, &writeRequest{Points: []mts.Point{testPoint()}})
	if err != nil {
		t.Fatalf("grpcWritePointsAsTypedBatch() error = %v", err)
	}
	if resp := points.(writeResponse); !resp.OK || resp.Mode != "points_typed" || resp.Points != 1 {
		t.Fatalf("grpcWritePointsAsTypedBatch() = %#v", resp)
	}

	deleted, err := grpcDelete(runtime, ctx, &deleteRequest{Request: mts.DeleteRequest{
		Database:    "default",
		Measurement: "cpu",
		StartTime:   0,
		EndTime:     10,
	}})
	if err != nil {
		t.Fatalf("grpcDelete() error = %v", err)
	}
	if resp := deleted.(deleteResponse); !resp.OK || resp.Measurement != "cpu" {
		t.Fatalf("grpcDelete() = %#v", resp)
	}

	limits, err := grpcGetDataLimits(runtime, ctx, nil)
	if err != nil || limits.(dataLimitsResponse).Path != routeDataLimits {
		t.Fatalf("grpcGetDataLimits() = %#v, %v", limits, err)
	}
	contract, err := grpcGetDataContract(runtime, ctx, nil)
	if err != nil || contract.(dataContractResponse).Version == 0 {
		t.Fatalf("grpcGetDataContract() = %#v, %v", contract, err)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := grpcWritePointsAsTypedBatch(runtime, cancelled, &writeRequest{Points: []mts.Point{testPoint()}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("grpcWritePointsAsTypedBatch(cancelled) error = %v", err)
	}
	if _, err := grpcDelete(runtime, cancelled, &deleteRequest{Request: mts.DeleteRequest{Measurement: "cpu"}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("grpcDelete(cancelled) error = %v", err)
	}
}

func TestGRPCSessionWrapperContracts(t *testing.T) {
	runtime := openTestRuntime(t)
	seedUserWithPassword(t, runtime, mts.User{Name: "session-user"}, "secret12")
	token, err := runtime.engine.Authenticate(context.Background(), mts.Credentials{
		UserName: "session-user",
		Password: "secret12",
	}, normalizeAuthTTL(0))
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token.Token))
	got, err := grpcGetSession(runtime, ctx, nil)
	if err != nil {
		t.Fatalf("grpcGetSession() error = %v", err)
	}
	resp := got.(sessionResponse)
	if !resp.OK || resp.UserName != "session-user" || resp.Path != routeAuthSession {
		t.Fatalf("grpcGetSession() = %#v", resp)
	}
	if _, err := grpcGetSession(runtime, context.Background(), nil); err == nil {
		t.Fatal("grpcGetSession(no token) error = nil")
	}
	badCtx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer bad-token"))
	if _, err := grpcGetSession(runtime, badCtx, nil); err == nil {
		t.Fatal("grpcGetSession(bad token) error = nil")
	}
}

func TestGRPCAdminWrappersRejectMissingCredentials(t *testing.T) {
	runtime := openTestRuntimeWithAdminToken(t)
	ctx := context.Background()
	policy := testDownsamplePolicy()
	calls := []struct {
		name string
		fn   func() (any, error)
	}{
		{name: "set password", fn: func() (any, error) { return grpcSetUserPassword(runtime, ctx, &setUserPasswordRequest{}) }},
		{name: "create user", fn: func() (any, error) { return grpcCreateUser(runtime, ctx, &createUserRequest{}) }},
		{name: "update user", fn: func() (any, error) { return grpcUpdateUser(runtime, ctx, &mts.User{}) }},
		{name: "get user", fn: func() (any, error) { return grpcGetUser(runtime, ctx, &userNameRequest{}) }},
		{name: "list users", fn: func() (any, error) { return grpcListUsers(runtime, ctx, nil) }},
		{name: "delete user", fn: func() (any, error) { return grpcDeleteUser(runtime, ctx, &userNameRequest{}) }},
		{name: "grant", fn: func() (any, error) { return grpcGrantDatabasePermission(runtime, ctx, &databasePermissionRequest{}) }},
		{name: "revoke", fn: func() (any, error) { return grpcRevokeDatabasePermission(runtime, ctx, &databasePermissionRequest{}) }},
		{name: "list grants", fn: func() (any, error) { return grpcListDatabasePermissions(runtime, ctx, &userNameRequest{}) }},
		{name: "check grant", fn: func() (any, error) { return grpcCheckDatabasePermission(runtime, ctx, &databasePermissionRequest{}) }},
		{name: "create database", fn: func() (any, error) { return grpcCreateDatabase(runtime, ctx, &databaseRequest{}) }},
		{name: "list databases", fn: func() (any, error) { return grpcListDatabases(runtime, ctx, nil) }},
		{name: "drop database", fn: func() (any, error) { return grpcDropDatabase(runtime, ctx, &databaseRequest{}) }},
		{name: "create retention", fn: func() (any, error) { return grpcCreateRetentionPolicy(runtime, ctx, &grpcRetentionPolicyRequest{}) }},
		{name: "list retention", fn: func() (any, error) { return grpcListRetentionPolicies(runtime, ctx, &databaseRequest{}) }},
		{name: "get config", fn: func() (any, error) { return grpcGetConfig(runtime, ctx, nil) }},
		{name: "get config schema", fn: func() (any, error) { return grpcGetConfigSchema(runtime, ctx, nil) }},
		{name: "validate config", fn: func() (any, error) { return grpcValidateConfig(runtime, ctx, &configValidateRequest{}) }},
		{name: "reload config", fn: func() (any, error) { return grpcReloadConfig(runtime, ctx, nil) }},
		{name: "api spec", fn: func() (any, error) { return grpcGetAPISpec(runtime, ctx, nil) }},
		{name: "error codes", fn: func() (any, error) { return grpcGetErrorCodes(runtime, ctx, nil) }},
		{name: "apply retention", fn: func() (any, error) { return grpcApplyRetention(runtime, ctx, &retentionApplyRequest{}) }},
		{name: "maintenance errors", fn: func() (any, error) { return grpcMaintenanceErrors(runtime, ctx, nil) }},
		{name: "maintenance stats", fn: func() (any, error) { return grpcMaintenanceStats(runtime, ctx, nil) }},
		{name: "ops status", fn: func() (any, error) { return grpcOpsStatus(runtime, ctx, nil) }},
		{name: "health", fn: func() (any, error) { return grpcAdminHealth(runtime, ctx, nil) }},
		{name: "memory", fn: func() (any, error) { return grpcStorageMemory(runtime, ctx, nil) }},
		{name: "compaction stats", fn: func() (any, error) { return grpcCompactionStats(runtime, ctx, nil) }},
		{name: "validate storage", fn: func() (any, error) { return grpcStorageValidate(runtime, ctx, nil) }},
		{name: "snapshot", fn: func() (any, error) { return grpcStorageSnapshot(runtime, ctx, nil) }},
		{name: "export", fn: func() (any, error) { return grpcStorageExport(runtime, ctx, nil) }},
		{name: "create policy", fn: func() (any, error) { return grpcCreateDownsamplePolicy(runtime, ctx, &policy) }},
		{name: "list policies", fn: func() (any, error) { return grpcListDownsamplePolicies(runtime, ctx, nil) }},
		{name: "enable policy", fn: func() (any, error) { return grpcEnableDownsamplePolicy(runtime, ctx, &downsamplePolicyRequest{}) }},
		{name: "disable policy", fn: func() (any, error) { return grpcDisableDownsamplePolicy(runtime, ctx, &downsamplePolicyRequest{}) }},
		{name: "drop policy", fn: func() (any, error) { return grpcDropDownsamplePolicy(runtime, ctx, &downsamplePolicyRequest{}) }},
		{name: "reset policy", fn: func() (any, error) { return grpcResetDownsamplePolicy(runtime, ctx, &grpcDownsampleResetRequest{}) }},
		{name: "policy statuses", fn: func() (any, error) { return grpcDownsamplePolicyStatuses(runtime, ctx, nil) }},
		{name: "run policy", fn: func() (any, error) { return grpcRunDownsamplePolicy(runtime, ctx, &downsamplePolicyRangeRequest{}) }},
		{name: "run range", fn: func() (any, error) {
			return grpcRunDownsamplePolicyRange(runtime, ctx, &downsamplePolicyRangeRequest{})
		}},
		{name: "repair policy", fn: func() (any, error) { return grpcRepairDownsamplePolicy(runtime, ctx, &downsamplePolicyRangeRequest{}) }},
		{name: "dry run policy", fn: func() (any, error) { return grpcDryRunDownsamplePolicy(runtime, ctx, &downsamplePolicyRangeRequest{}) }},
		{name: "batch users", fn: func() (any, error) { return grpcBatchUpdateUserDisabled(runtime, ctx, &batchUserDisabledRequest{}) }},
		{name: "batch policies", fn: func() (any, error) { return grpcBatchDownsamplePolicies(runtime, ctx, &batchDownsampleRequest{}) }},
	}
	for _, call := range calls {
		t.Run(call.name, func(t *testing.T) {
			if _, err := call.fn(); err == nil {
				t.Fatal("error = nil, want authentication failure")
			}
		})
	}
}
