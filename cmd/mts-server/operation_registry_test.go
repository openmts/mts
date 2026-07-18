package main

import (
	"net/http"
	"testing"
)

func TestOperationRegistryCoversAllGRPCMethods(t *testing.T) {
	want := map[string]struct{}{
		grpcMethodHealth:                   {},
		grpcMethodWrite:                    {},
		grpcMethodWriteTypedBatch:          {},
		grpcMethodQueryRows:                {},
		grpcMethodQueryColumns:             {},
		grpcMethodQueryWithExplain:         {},
		grpcMethodQueryStats:               {},
		grpcMethodLogin:                    {},
		grpcMethodLogout:                   {},
		grpcMethodChangePassword:           {},
		grpcMethodSetUserPassword:          {},
		grpcMethodCreateUser:               {},
		grpcMethodUpdateUser:               {},
		grpcMethodGetUser:                  {},
		grpcMethodListUsers:                {},
		grpcMethodDeleteUser:               {},
		grpcMethodGrantDatabasePermission:  {},
		grpcMethodRevokeDatabasePermission: {},
		grpcMethodListDatabasePermissions:  {},
		grpcMethodCheckDatabasePermission:  {},
		grpcMethodCreateDatabase:           {},
		grpcMethodDropDatabase:             {},
		grpcMethodCreateRetentionPolicy:    {},
		grpcMethodListRetentionPolicies:    {},
		grpcMethodListMeasurements:         {},
		grpcMethodListFields:               {},
		grpcMethodListSeries:               {},
		grpcMethodGetConfig:                {},
		grpcMethodGetEffectiveConfig:       {},
		grpcMethodGetConfigSchema:          {},
		grpcMethodValidateConfig:           {},
		grpcMethodReloadConfig:             {},
		grpcMethodGetAPISpec:               {},
		grpcMethodGetErrorCodes:            {},
		grpcMethodFlush:                    {},
		grpcMethodCompact:                  {},
		grpcMethodApplyRetention:           {},
		grpcMethodMaintenanceErrors:        {},
		grpcMethodStorageMemory:            {},
		grpcMethodCompactionStats:          {},
		grpcMethodStorageValidate:          {},
		grpcMethodStorageSnapshot:          {},
		grpcMethodStorageExport:            {},
		grpcMethodCreateDownsamplePolicy:   {},
		grpcMethodListDownsamplePolicies:   {},
		grpcMethodEnableDownsamplePolicy:   {},
		grpcMethodDisableDownsamplePolicy:  {},
		grpcMethodDropDownsamplePolicy:     {},
		grpcMethodResetDownsamplePolicy:    {},
		grpcMethodDownsamplePolicyStatuses: {},
		grpcMethodRunDownsamplePolicy:      {},
		grpcMethodRunDownsamplePolicyRange: {},
		grpcMethodRepairDownsamplePolicy:   {},
		grpcMethodDryRunDownsamplePolicy:   {},
	}
	got := grpcMethodsFromRegistry()
	if len(got) != len(want) {
		t.Fatalf("grpc methods = %d, want %d", len(got), len(want))
	}
	for _, method := range got {
		if _, ok := want[method.MethodName]; !ok {
			t.Fatalf("unexpected grpc method %q", method.MethodName)
		}
		delete(want, method.MethodName)
		if method.Handler == nil {
			t.Fatalf("grpc method %q handler is nil", method.MethodName)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing grpc methods: %#v", want)
	}
}

func TestOperationRegistryMountsCoreHTTPPaths(t *testing.T) {
	required := []string{
		routeHealth,
		routeReady,
		routeMetrics,
		routeDataWrite,
		routeDataWriteTyped,
		routeDataQueryRows,
		routeDataQueryColumns,
		routeDataQueryExplain,
		routeDataQueryStream,
		routeDataQueryStats,
		routeDataDatabasesPrefix,
		routeAuthLogin,
		routeAuthLogout,
		routeAuthPassword,
		routeUsers,
		routeUsersPrefix,
		routeAuthzDatabaseCheck,
		routeAdminDatabases,
		routeAdminDatabasesPrefix,
		routeAdminConfig,
		routeAdminConfigEffective,
		routeAdminConfigSchema,
		routeAdminFlush,
		routeAdminCompact,
		routeAdminRetentionApply,
		routeAdminMaintenanceErrors,
		routeAdminStatsStorageMemory,
		routeAdminStatsCompaction,
		routeAdminHealth,
		routeAdminDownsamplePolicies,
		routeAdminDownsamplePrefix,
		routeAdminDownsampleStatuses,
		routeAdminAPISpec,
		routeAdminErrorCodes,
		routeAdminConfigValidate,
		routeAdminConfigReload,
		routeAdminStorageValidate,
		routeAdminStorageSnapshot,
		routeAdminStorageExport,
	}
	seen := make(map[string]bool)
	for _, op := range operationCatalog() {
		if op.HTTPHandler == nil {
			continue
		}
		for _, path := range op.HTTPPaths {
			seen[path] = true
		}
	}
	for _, path := range required {
		if !seen[path] {
			t.Fatalf("missing http path in registry: %s", path)
		}
	}
}

func TestAPISpecFromRegistryIncludesWriteAndAdmin(t *testing.T) {
	spec := apiSpecFromRegistry()
	if spec.Version != "v1" {
		t.Fatalf("version = %q, want v1", spec.Version)
	}
	foundWrite := false
	foundAPISpec := false
	for _, ns := range spec.Namespaces {
		for _, ep := range ns.Endpoints {
			if ep.Method == http.MethodPost && ep.Path == routeDataWrite {
				foundWrite = true
			}
			if ep.Method == http.MethodGet && ep.Path == routeAdminAPISpec {
				foundAPISpec = true
			}
		}
	}
	if !foundWrite {
		t.Fatal("api spec missing data write endpoint")
	}
	if !foundAPISpec {
		t.Fatal("api spec missing admin api-spec endpoint")
	}
}
