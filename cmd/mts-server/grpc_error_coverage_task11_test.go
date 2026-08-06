package main

import (
	"context"
	"errors"
	"testing"

	mts "github.com/openmts/mts"
)

func TestGRPCWrappersPropagateCancelledContext(t *testing.T) {
	runtime := openTestRuntime(t)
	base, cancel := context.WithCancel(context.Background())
	cancel()
	adminCtx := context.WithValue(base, grpcAdminAuthorizedContextKey{}, true)
	policy := testDownsamplePolicy()
	policy.Name = "cancelled-policy"

	calls := []struct {
		name string
		fn   func() (any, error)
	}{
		{name: "typed write", fn: func() (any, error) { return grpcWriteTypedBatch(runtime, base, &typedWriteRequest{}) }},
		{name: "query columns", fn: func() (any, error) { return grpcQueryColumns(runtime, base, &queryRequest{Query: testQuery()}) }},
		{name: "query explain", fn: func() (any, error) { return grpcQueryWithExplain(runtime, base, &queryRequest{Query: testQuery()}) }},
		{name: "update user", fn: func() (any, error) { return grpcUpdateUser(runtime, adminCtx, &mts.User{Name: "admin"}) }},
		{name: "get user", fn: func() (any, error) { return grpcGetUser(runtime, adminCtx, &userNameRequest{Name: "admin"}) }},
		{name: "list users", fn: func() (any, error) { return grpcListUsers(runtime, adminCtx, nil) }},
		{name: "revoke grant", fn: func() (any, error) {
			return grpcRevokeDatabasePermission(runtime, adminCtx, &databasePermissionRequest{})
		}},
		{name: "list grants", fn: func() (any, error) {
			return grpcListDatabasePermissions(runtime, adminCtx, &userNameRequest{Name: "admin"})
		}},
		{name: "list databases", fn: func() (any, error) { return grpcListDatabases(runtime, adminCtx, nil) }},
		{name: "drop database", fn: func() (any, error) { return grpcDropDatabase(runtime, adminCtx, &databaseRequest{Name: "missing"}) }},
		{name: "create retention", fn: func() (any, error) {
			return grpcCreateRetentionPolicy(runtime, adminCtx, &grpcRetentionPolicyRequest{})
		}},
		{name: "list retention", fn: func() (any, error) { return grpcListRetentionPolicies(runtime, adminCtx, &databaseRequest{}) }},
		{name: "list measurements", fn: func() (any, error) { return grpcListMeasurements(runtime, base, &metadataRequest{}) }},
		{name: "list fields", fn: func() (any, error) { return grpcListFields(runtime, base, &metadataRequest{}) }},
		{name: "list series", fn: func() (any, error) { return grpcListSeries(runtime, base, &metadataRequest{}) }},
		{name: "apply retention", fn: func() (any, error) { return grpcApplyRetention(runtime, adminCtx, &retentionApplyRequest{}) }},
		{name: "list policies", fn: func() (any, error) { return grpcListDownsamplePolicies(runtime, adminCtx, nil) }},
		{name: "disable policy", fn: func() (any, error) {
			return grpcDisableDownsamplePolicy(runtime, adminCtx, &downsamplePolicyRequest{Name: policy.Name})
		}},
		{name: "drop policy", fn: func() (any, error) {
			return grpcDropDownsamplePolicy(runtime, adminCtx, &downsamplePolicyRequest{Name: policy.Name})
		}},
		{name: "reset policy", fn: func() (any, error) {
			return grpcResetDownsamplePolicy(runtime, adminCtx, &grpcDownsampleResetRequest{Name: policy.Name})
		}},
		{name: "policy statuses", fn: func() (any, error) { return grpcDownsamplePolicyStatuses(runtime, adminCtx, nil) }},
		{name: "run policy", fn: func() (any, error) {
			return grpcRunDownsamplePolicy(runtime, adminCtx, &downsamplePolicyRangeRequest{Name: policy.Name})
		}},
		{name: "repair policy", fn: func() (any, error) {
			return grpcRepairDownsamplePolicy(runtime, adminCtx, &downsamplePolicyRangeRequest{Name: policy.Name})
		}},
		{name: "dry run policy", fn: func() (any, error) {
			return grpcDryRunDownsamplePolicy(runtime, adminCtx, &downsamplePolicyRangeRequest{Name: policy.Name})
		}},
	}
	for _, call := range calls {
		t.Run(call.name, func(t *testing.T) {
			if _, err := call.fn(); err == nil {
				t.Fatal("error = nil, want cancelled or downstream error")
			}
		})
	}
}

func TestGRPCWrapperValidationAndSnapshotErrors(t *testing.T) {
	runtime := openTestRuntime(t)
	ctx := context.WithValue(context.Background(), grpcAdminAuthorizedContextKey{}, true)
	if _, err := grpcSetUserPassword(runtime, ctx, &setUserPasswordRequest{UserName: "admin", Password: "x"}); err == nil {
		t.Fatal("grpcSetUserPassword(short) error = nil")
	}
	if _, err := grpcSetUserPassword(runtime, ctx, &setUserPasswordRequest{UserName: "missing", Password: "secret12"}); err == nil {
		t.Fatal("grpcSetUserPassword(missing) error = nil")
	}
	if _, err := grpcGetUser(runtime, ctx, &userNameRequest{Name: "missing"}); !errors.Is(err, mts.ErrUserNotFound) {
		t.Fatalf("grpcGetUser(missing) error = %v", err)
	}
	if _, err := grpcDeleteStorageSnapshot(runtime, ctx, nil); err == nil {
		t.Fatal("grpcDeleteStorageSnapshot(nil) error = nil")
	}
	if _, err := grpcDeleteStorageSnapshot(runtime, ctx, &storageSnapshotDeleteRequest{Name: "snapshot-missing.json"}); err == nil {
		t.Fatal("grpcDeleteStorageSnapshot(missing) error = nil")
	}
	audit, err := grpcListAudit(runtime, ctx, nil)
	if err != nil || audit.(auditListResponse).Path != routeAdminAudit {
		t.Fatalf("grpcListAudit(nil) = %#v, %v", audit, err)
	}
}
