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
		grpcMethodWritePointsAsTypedBatch:  {},
		grpcMethodQueryRows:                {},
		grpcMethodQueryColumns:             {},
		grpcMethodQueryWithExplain:         {},
		grpcMethodQueryStats:               {},
		grpcMethodGetDataLimits:            {},
		grpcMethodDelete:                   {},
		grpcMethodLogin:                    {},
		grpcMethodLogout:                   {},
		grpcMethodChangePassword:           {},
		grpcMethodGetSession:               {},
		grpcMethodSetUserPassword:          {},
		grpcMethodCreateUser:               {},
		grpcMethodUpdateUser:               {},
		grpcMethodGetUser:                  {},
		grpcMethodListUsers:                {},
		grpcMethodDeleteUser:               {},
		grpcMethodBatchUpdateUserDisabled:  {},
		grpcMethodGrantDatabasePermission:  {},
		grpcMethodRevokeDatabasePermission: {},
		grpcMethodListDatabasePermissions:  {},
		grpcMethodCheckDatabasePermission:  {},
		grpcMethodCreateDatabase:           {},
		grpcMethodListDatabases:            {},
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
		grpcMethodMaintenanceStats:         {},
		grpcMethodOpsStatus:                {},
		grpcMethodStorageMemory:            {},
		grpcMethodCompactionStats:          {},
		grpcMethodAdminHealth:              {},
		grpcMethodStorageValidate:          {},
		grpcMethodStorageSnapshot:          {},
		grpcMethodListStorageSnapshots:     {},
		grpcMethodDeleteStorageSnapshot:    {},
		grpcMethodListAudit:                {},
		grpcMethodStorageExport:            {},
		grpcMethodCreateDownsamplePolicy:   {},
		grpcMethodListDownsamplePolicies:   {},
		grpcMethodEnableDownsamplePolicy:   {},
		grpcMethodDisableDownsamplePolicy:  {},
		grpcMethodBatchDownsamplePolicies:  {},
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
		routeDataWritePointsTyped,
		routeDataQueryRows,
		routeDataQueryColumns,
		routeDataQueryExplain,
		routeDataQueryStream,
		routeDataDelete,
		routeDataQueryStats,
		routeDataLimits,
		routeDataDatabases,
		routeDataDatabasesPrefix,
		routeAuthLogin,
		routeAuthLogout,
		routeAuthPassword,
		routeAuthPasswordPolicy,
		routeUsers,
		routeUsersPrefix,
		routeUsersBatchDisabled,
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
		routeAdminStatsMaintenance,
		routeAdminStatsStorageMemory,
		routeAdminStatsCompaction,
		routeAdminHealth,
		routeAdminDoctor,
		routeAdminVersion,
		routeAdminDownsamplePolicies,
		routeAdminDownsamplePrefix,
		routeAdminDownsampleStatuses,
		routeAdminDownsampleBatch,
		routeAdminAPISpec,
		routeAdminErrorCodes,
		routeAdminConfigValidate,
		routeAdminConfigReload,
		routeAdminStorageValidate,
		routeAdminStorageSnapshot,
		routeAdminStorageDataSnapshot,
		routeAdminStorageRestoreDrill,
		routeAdminStorageDataSnapshots,
		routeAdminStorageSnapshots,
		routeAdminAudit,
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
	foundWriteResp := false
	foundFlushResp := false
	foundBatchDisabledResp := false
	for _, ns := range spec.Namespaces {
		for _, ep := range ns.Endpoints {
			if ep.Method == http.MethodPost && ep.Path == routeDataWrite {
				foundWrite = true
				if ep.Response != "" {
					foundWriteResp = true
				}
			}
			if ep.Method == http.MethodGet && ep.Path == routeAdminAPISpec {
				foundAPISpec = true
			}
			if ep.Method == http.MethodPost && ep.Path == routeAdminFlush && ep.Response != "" {
				foundFlushResp = true
			}
			if ep.Method == http.MethodPost && ep.Path == routeUsersBatchDisabled && ep.Response != "" {
				foundBatchDisabledResp = true
			}
		}
	}
	if !foundWrite {
		t.Fatal("api spec missing data write endpoint")
	}
	if !foundAPISpec {
		t.Fatal("api spec missing admin api-spec endpoint")
	}
	if !foundWriteResp {
		t.Fatal("api spec write endpoint missing response hint")
	}
	if !foundFlushResp {
		t.Fatal("api spec flush endpoint missing response hint")
	}
	if !foundBatchDisabledResp {
		t.Fatal("api spec batch-disabled endpoint missing response hint")
	}
}
