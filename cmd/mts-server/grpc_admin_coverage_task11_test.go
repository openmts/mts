package main

import (
	"context"
	"testing"

	mts "github.com/openmts/mts"
)

func TestGRPCAdminWrapperSuccessContracts(t *testing.T) {
	runtime := openTestRuntime(t)
	ctx := context.WithValue(context.Background(), grpcAdminAuthorizedContextKey{}, true)

	assertGRPCPath(t, routeUsers, func() (any, error) {
		return grpcCreateUser(runtime, ctx, &createUserRequest{
			User:     mts.User{Name: "task11-user", Role: mts.UserRoleUser},
			Password: "secret12",
		})
	})
	got, err := grpcGetUser(runtime, ctx, &userNameRequest{Name: "task11-user"})
	if err != nil || got.(userResponse).User.Name != "task11-user" {
		t.Fatalf("grpcGetUser() = %#v, %v", got, err)
	}
	user := got.(userResponse).User
	user.Disabled = true
	assertGRPCPath(t, routeUsersPrefix+user.Name, func() (any, error) { return grpcUpdateUser(runtime, ctx, &user) })
	listed, err := grpcListUsers(runtime, ctx, nil)
	if err != nil || len(listed.(usersResponse).Users) < 2 {
		t.Fatalf("grpcListUsers() = %#v, %v", listed, err)
	}
	user.Disabled = false
	assertGRPCPath(t, routeUsersPrefix+user.Name, func() (any, error) { return grpcUpdateUser(runtime, ctx, &user) })

	grant := databasePermissionRequest{
		UserName:   user.Name,
		Database:   "default",
		Permission: mts.DatabasePermissionRead,
	}
	assertGRPCPath(t, routeUsersPrefix+user.Name+"/database-permissions", func() (any, error) {
		return grpcGrantDatabasePermission(runtime, ctx, &grant)
	})
	checked, err := grpcCheckDatabasePermission(runtime, ctx, &grant)
	if err != nil || !checked.(authzDatabaseCheckResponse).Allowed {
		t.Fatalf("grpcCheckDatabasePermission() = %#v, %v", checked, err)
	}
	grants, err := grpcListDatabasePermissions(runtime, ctx, &userNameRequest{Name: user.Name})
	if err != nil || len(grants.(databasePermissionsResponse).Grants) != 1 {
		t.Fatalf("grpcListDatabasePermissions() = %#v, %v", grants, err)
	}
	assertGRPCPath(t, routeUsersPrefix+user.Name+"/database-permissions", func() (any, error) {
		return grpcRevokeDatabasePermission(runtime, ctx, &grant)
	})

	assertGRPCPath(t, routeAdminDatabases, func() (any, error) {
		return grpcCreateDatabase(runtime, ctx, &databaseRequest{Name: "task11-db"})
	})
	databases, err := grpcListDatabases(runtime, ctx, nil)
	if err != nil || len(databases.(databasesResponse).Databases) != 1 || databases.(databasesResponse).Databases[0] != "task11-db" {
		t.Fatalf("grpcListDatabases() = %#v, %v", databases, err)
	}
	policy := mts.RetentionPolicy{Name: "task11-rp"}
	assertGRPCPath(t, routeAdminDatabasesPrefix+"task11-db/retention-policies", func() (any, error) {
		return grpcCreateRetentionPolicy(runtime, ctx, &grpcRetentionPolicyRequest{
			Database: "task11-db",
			Policy:   policy,
		})
	})
	policies, err := grpcListRetentionPolicies(runtime, ctx, &databaseRequest{Name: "task11-db"})
	if err != nil || len(policies.(retentionPoliciesResponse).Policies) == 0 {
		t.Fatalf("grpcListRetentionPolicies() = %#v, %v", policies, err)
	}

	measurements, err := grpcListMeasurements(runtime, ctx, &metadataRequest{Database: "default"})
	if err != nil || measurements.(measurementsResponse).Path == "" {
		t.Fatalf("grpcListMeasurements() = %#v, %v", measurements, err)
	}
	fields, err := grpcListFields(runtime, ctx, &metadataRequest{Database: "default", Measurement: "cpu"})
	if err != nil || fields.(fieldsResponse).Path == "" {
		t.Fatalf("grpcListFields() = %#v, %v", fields, err)
	}
	series, err := grpcListSeries(runtime, ctx, &metadataRequest{Database: "default", Measurement: "cpu"})
	if err != nil || series.(seriesResponse).Path == "" {
		t.Fatalf("grpcListSeries() = %#v, %v", series, err)
	}

	pathCalls := []struct {
		want string
		fn   func() (any, error)
	}{
		{want: routeAdminConfigEffective, fn: func() (any, error) { return grpcGetConfig(runtime, ctx, nil) }},
		{want: routeAdminConfigSchema, fn: func() (any, error) { return grpcGetConfigSchema(runtime, ctx, nil) }},
		{want: routeAdminConfigValidate, fn: func() (any, error) {
			return grpcValidateConfig(runtime, ctx, &configValidateRequest{Config: runtime.currentConfig()})
		}},
		{want: routeAdminAPISpec, fn: func() (any, error) { return grpcGetAPISpec(runtime, ctx, nil) }},
		{want: routeAdminErrorCodes, fn: func() (any, error) { return grpcGetErrorCodes(runtime, ctx, nil) }},
		{want: routeAdminMaintenanceErrors, fn: func() (any, error) { return grpcMaintenanceErrors(runtime, ctx, nil) }},
		{want: routeAdminStatsMaintenance, fn: func() (any, error) { return grpcMaintenanceStats(runtime, ctx, nil) }},
		{want: routeAdminOpsStatus, fn: func() (any, error) { return grpcOpsStatus(runtime, ctx, nil) }},
		{want: routeAdminHealth, fn: func() (any, error) { return grpcAdminHealth(runtime, ctx, nil) }},
		{want: routeAdminStatsStorageMemory, fn: func() (any, error) { return grpcStorageMemory(runtime, ctx, nil) }},
		{want: routeAdminStatsCompaction, fn: func() (any, error) { return grpcCompactionStats(runtime, ctx, nil) }},
		{want: routeAdminStorageValidate, fn: func() (any, error) { return grpcStorageValidate(runtime, ctx, nil) }},
		{want: routeAdminStorageExport, fn: func() (any, error) { return grpcStorageExport(runtime, ctx, nil) }},
	}
	for _, call := range pathCalls {
		assertResponsePath(t, call.want, call.fn)
	}

	downsample := testDownsamplePolicy()
	downsample.Name = "task11-downsample"
	assertGRPCPath(t, routeAdminDownsamplePolicies, func() (any, error) {
		return grpcCreateDownsamplePolicy(runtime, ctx, &downsample)
	})
	listedPolicies, err := grpcListDownsamplePolicies(runtime, ctx, nil)
	if err != nil || len(listedPolicies.(downsamplePoliciesResponse).Policies) == 0 {
		t.Fatalf("grpcListDownsamplePolicies() = %#v, %v", listedPolicies, err)
	}
	request := &downsamplePolicyRequest{Name: downsample.Name}
	assertGRPCPath(t, routeAdminDownsamplePrefix+downsample.Name+"/disable", func() (any, error) {
		return grpcDisableDownsamplePolicy(runtime, ctx, request)
	})
	assertGRPCPath(t, routeAdminDownsamplePrefix+downsample.Name+"/enable", func() (any, error) {
		return grpcEnableDownsamplePolicy(runtime, ctx, request)
	})
	statuses, err := grpcDownsamplePolicyStatuses(runtime, ctx, nil)
	if err != nil || statuses.(downsampleStatusesResponse).Summary == nil {
		t.Fatalf("grpcDownsamplePolicyStatuses() = %#v, %v", statuses, err)
	}
	if _, err := grpcDropDownsamplePolicy(runtime, ctx, 1); err == nil {
		t.Fatal("grpcDropDownsamplePolicy(invalid) error = nil")
	}
	assertGRPCPath(t, routeAdminDownsamplePrefix+downsample.Name, func() (any, error) {
		return grpcDropDownsamplePolicy(runtime, ctx, request)
	})

	batchUser := mts.User{Name: "task11-batch", Role: mts.UserRoleUser}
	seedUserWithPassword(t, runtime, batchUser, "secret12")
	batch, err := grpcBatchUpdateUserDisabled(runtime, ctx, &batchUserDisabledRequest{
		Names:    []string{batchUser.Name},
		Disabled: true,
	})
	if err != nil || batch.(batchMutationResponse).OKCount != 1 {
		t.Fatalf("grpcBatchUpdateUserDisabled() = %#v, %v", batch, err)
	}
	if _, err := grpcBatchDownsamplePolicies(runtime, ctx, &batchDownsampleRequest{Names: []string{"missing"}, Action: "disable"}); err != nil {
		t.Fatalf("grpcBatchDownsamplePolicies() error = %v", err)
	}

	assertGRPCPath(t, routeUsersPrefix+user.Name, func() (any, error) {
		return grpcDeleteUser(runtime, ctx, &userNameRequest{Name: user.Name})
	})
	assertGRPCPath(t, routeAdminDatabasesPrefix+"task11-db", func() (any, error) {
		return grpcDropDatabase(runtime, ctx, &databaseRequest{Name: "task11-db"})
	})
}

func assertGRPCPath(t *testing.T, want string, call func() (any, error)) {
	t.Helper()
	value, err := call()
	if err != nil {
		t.Fatalf("gRPC wrapper error = %v", err)
	}
	resp, ok := value.(okResponse)
	if !ok || !resp.OK || resp.Path != want {
		t.Fatalf("gRPC wrapper response = %#v, want path %q", value, want)
	}
}

func assertResponsePath(t *testing.T, want string, call func() (any, error)) {
	t.Helper()
	value, err := call()
	if err != nil {
		t.Fatalf("gRPC response error = %v", err)
	}
	data, marshalErr := jsonCodec{}.Marshal(value)
	if marshalErr != nil {
		t.Fatalf("Marshal(response) error = %v", marshalErr)
	}
	var path struct {
		Path string `json:"path"`
	}
	if unmarshalErr := (jsonCodec{}).Unmarshal(data, &path); unmarshalErr != nil || path.Path != want {
		t.Fatalf("response path = %q, error = %v, want %q", path.Path, unmarshalErr, want)
	}
}
