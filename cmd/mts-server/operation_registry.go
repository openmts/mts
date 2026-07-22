package main

import (
	"context"
	"net/http"

	"google.golang.org/grpc"

	mts "github.com/openmts/mts"
)

// operationAuth 描述 API 契约中的认证要求（人类可读）。
type operationAuth string

const (
	authNone      operationAuth = "none"
	authDataToken operationAuth = "data token optional; when require_user is true, user bearer token and DB permission are required"
	authUserPass  operationAuth = "user password"
	authUserToken operationAuth = "user bearer token"
	authAdmin     operationAuth = "admin token or admin user bearer token"
)

// operation 是 HTTP/gRPC 双协议操作的统一登记项。
// 新增固定路径 API 时，应先在此登记，再由 registry 生成挂载与契约。
type operation struct {
	Name         string
	Namespace    string
	Description  string
	Auth         operationAuth
	HTTPMethods  []string
	HTTPPaths    []string
	HTTPHandler  func(*serverRuntime, http.ResponseWriter, *http.Request)
	GRPCMethod   string
	GRPCRequest  any
	GRPCFn       func(*serverRuntime, context.Context, any) (any, error)
	GRPCHandler  grpc.MethodHandler
	ResourceOnly bool   // 仅资源/前缀路由元数据，不自动挂载
	ResponseHint string // 契约响应说明（API Spec response 字段）
}

func operationCatalog() []operation {
	return []operation{
		// system
		{
			Name:        "health",
			Namespace:   "system",
			Description: "liveness/readiness health",
			Auth:        authNone,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeHealth, routeReady},
			HTTPHandler: (*serverRuntime).handleHealth,
			GRPCMethod:  grpcMethodHealth,
			GRPCHandler: grpcHealthHandler,
		},
		{
			Name:        "metrics",
			Namespace:   "system",
			Description: "prometheus metrics",
			Auth:        authNone,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeMetrics},
			HTTPHandler: (*serverRuntime).handleMetrics,
		},

		// data
		{
			Name:        "write",
			Namespace:   "data",
			Description: "write point batch (response includes admin_op_busy/last)",
			Auth:        authDataToken,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeDataWrite},
			HTTPHandler: (*serverRuntime).handleWrite,
			GRPCMethod:  grpcMethodWrite,
			GRPCHandler: grpcWriteHandler,
		},
		{
			Name:        "write_typed",
			Namespace:   "data",
			Description: "write typed column batch (response includes admin_op_busy/last)",
			Auth:        authDataToken,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeDataWriteTyped},
			HTTPHandler: (*serverRuntime).handleWriteTyped,
			GRPCMethod:  grpcMethodWriteTypedBatch,
			GRPCRequest: &typedWriteRequest{},
			GRPCFn:      grpcWriteTypedBatch,
		},
		{
			Name:        "write_points_typed",
			Namespace:   "data",
			Description: "convert []Point to typed batch and write (response includes admin_op_busy/last)",
			Auth:        authDataToken,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeDataWritePointsTyped},
			HTTPHandler: (*serverRuntime).handleWritePointsTyped,
			GRPCMethod:  grpcMethodWritePointsAsTypedBatch,
			GRPCRequest: &writeRequest{},
			GRPCFn:      grpcWritePointsAsTypedBatch,
		},
		{
			Name:        "query_rows",
			Namespace:   "data",
			Description: "query row result",
			Auth:        authDataToken,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeDataQueryRows},
			HTTPHandler: (*serverRuntime).handleQueryRows,
			GRPCMethod:  grpcMethodQueryRows,
			GRPCHandler: grpcQueryRowsHandler,
		},
		{
			Name:        "query_columns",
			Namespace:   "data",
			Description: "query column result",
			Auth:        authDataToken,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeDataQueryColumns},
			HTTPHandler: (*serverRuntime).handleQueryColumns,
			GRPCMethod:  grpcMethodQueryColumns,
			GRPCRequest: &queryRequest{},
			GRPCFn:      grpcQueryColumns,
		},
		{
			Name:        "query_explain",
			Namespace:   "data",
			Description: "query with execution explain",
			Auth:        authDataToken,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeDataQueryExplain},
			HTTPHandler: (*serverRuntime).handleQueryExplain,
			GRPCMethod:  grpcMethodQueryWithExplain,
			GRPCRequest: &queryRequest{},
			GRPCFn:      grpcQueryWithExplain,
		},
		{
			Name:        "query_stream",
			Namespace:   "data",
			Description: "query NDJSON stream (row|column)",
			Auth:        authDataToken,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeDataQueryStream},
			HTTPHandler: (*serverRuntime).handleQueryStream,
		},
		{
			Name:        "delete",
			Namespace:   "data",
			Description: "delete series data by time range",
			Auth:        authDataToken,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeDataDelete},
			HTTPHandler: (*serverRuntime).handleDelete,
			GRPCMethod:  grpcMethodDelete,
			GRPCRequest: &deleteRequest{},
			GRPCFn:      grpcDelete,
		},
		{
			Name:        "query_stats",
			Namespace:   "data",
			Description: "latest query stats (includes admin_op_busy/last)",
			Auth:        authDataToken,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeDataQueryStats},
			HTTPHandler: (*serverRuntime).handleQueryStats,
			GRPCMethod:  grpcMethodQueryStats,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcQueryStats,
		},
		{
			Name:        "list_data_databases",
			Namespace:   "data",
			Description: "list databases readable by current user",
			Auth:        authDataToken,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeDataDatabases},
			HTTPHandler: (*serverRuntime).handleDataDatabases,
		},
		{
			Name:         "data_database_metadata",
			Namespace:    "data",
			Description:  "list measurements/fields/series/retention-policies under database prefix (includes admin_op_busy/last on list responses)",
			Auth:         authDataToken,
			HTTPMethods:  []string{http.MethodGet},
			HTTPPaths:    []string{routeDataDatabasesPrefix},
			HTTPHandler:  (*serverRuntime).handleDataDatabase,
			ResourceOnly: true,
		},
		{
			Name:        "list_measurements",
			Namespace:   "data",
			Description: "list measurements",
			Auth:        authDataToken,
			GRPCMethod:  grpcMethodListMeasurements,
			GRPCRequest: &metadataRequest{},
			GRPCFn:      grpcListMeasurements,
		},
		{
			Name:        "list_fields",
			Namespace:   "data",
			Description: "list fields (includes admin_op_busy/last)",
			Auth:        authDataToken,
			GRPCMethod:  grpcMethodListFields,
			GRPCRequest: &metadataRequest{},
			GRPCFn:      grpcListFields,
		},
		{
			Name:        "list_series",
			Namespace:   "data",
			Description: "list series (includes admin_op_busy/last)",
			Auth:        authDataToken,
			GRPCMethod:  grpcMethodListSeries,
			GRPCRequest: &metadataRequest{},
			GRPCFn:      grpcListSeries,
		},

		// auth
		{
			Name:        "login",
			Namespace:   "auth",
			Description: "issue user bearer token",
			Auth:        authUserPass,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeAuthLogin},
			HTTPHandler: (*serverRuntime).handleLogin,
			GRPCMethod:  grpcMethodLogin,
			GRPCRequest: &loginRequest{},
			GRPCFn:      grpcLogin,
		},
		{
			Name:        "logout",
			Namespace:   "auth",
			Description: "revoke user bearer token",
			Auth:        authUserToken,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeAuthLogout},
			HTTPHandler: (*serverRuntime).handleLogout,
			GRPCMethod:  grpcMethodLogout,
			GRPCRequest: &logoutRequest{},
			GRPCFn:      grpcLogout,
		},
		{
			Name:        "change_password",
			Namespace:   "auth",
			Description: "change user password (wrong old password -> bad_request; success clears must_change and revokes tokens; response includes admin_op_busy/last)",
			Auth:        authUserPass,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeAuthPassword},
			HTTPHandler: (*serverRuntime).handleChangePassword,
			GRPCMethod:  grpcMethodChangePassword,
			GRPCRequest: &changePasswordRequest{},
			GRPCFn:      grpcChangePassword,
		},
		{
			Name:        "get_session",
			Namespace:   "auth",
			Description: "inspect current user bearer session (includes admin_op_busy/last)",
			Auth:        authUserToken,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAuthSession},
			HTTPHandler: (*serverRuntime).handleSession,
			GRPCMethod:  grpcMethodGetSession,
			GRPCRequest: &okResponse{},
			GRPCFn:      grpcGetSession,
		},
		{
			Name:        "check_database_permission",
			Namespace:   "authz",
			Description: "check database permission",
			Auth:        authUserToken,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeAuthzDatabaseCheck},
			HTTPHandler: (*serverRuntime).handleAuthzDatabaseCheck,
			GRPCMethod:  grpcMethodCheckDatabasePermission,
			GRPCRequest: &databasePermissionRequest{},
			GRPCFn:      grpcCheckDatabasePermission,
		},

		// users (collection + resource)
		{
			Name:        "users_collection",
			Namespace:   "users",
			Description: "list or create users",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet, http.MethodPost},
			HTTPPaths:   []string{routeUsers},
			HTTPHandler: (*serverRuntime).handleUsers,
		},
		{
			Name:         "users_resource",
			Namespace:    "users",
			Description:  "user resource and database permissions (PUT .../password -> setPasswordResponse with user_name; disable via PUT user revokes tokens)",
			Auth:         authAdmin,
			HTTPMethods:  []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPost},
			HTTPPaths:    []string{routeUsersPrefix},
			HTTPHandler:  (*serverRuntime).handleUserResource,
			ResourceOnly: true,
		},
		{
			Name:        "create_user",
			Namespace:   "users",
			Description: "create user",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodCreateUser,
			GRPCRequest: &createUserRequest{},
			GRPCFn:      grpcCreateUser,
		},
		{
			Name:        "update_user",
			Namespace:   "users",
			Description: "update user (disabling revokes target tokens)",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodUpdateUser,
			GRPCRequest: &mts.User{},
			GRPCFn:      grpcUpdateUser,
		},
		{
			Name:        "get_user",
			Namespace:   "users",
			Description: "get user",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodGetUser,
			GRPCRequest: &userNameRequest{},
			GRPCFn:      grpcGetUser,
		},
		{
			Name:        "list_users",
			Namespace:   "users",
			Description: "list users (includes admin_op_busy/last)",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodListUsers,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcListUsers,
		},
		{
			Name:        "batch_update_user_disabled",
			Namespace:   "users",
			Description: "batch set user disabled flags (disabling revokes target tokens; response includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeUsersBatchDisabled},
			HTTPHandler: (*serverRuntime).handleBatchUserDisabled,
			GRPCMethod:  grpcMethodBatchUpdateUserDisabled,
			GRPCRequest: &batchUserDisabledRequest{},
			GRPCFn:      grpcBatchUpdateUserDisabled,
		},
		{
			Name:        "delete_user",
			Namespace:   "users",
			Description: "delete user",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodDeleteUser,
			GRPCRequest: &userNameRequest{},
			GRPCFn:      grpcDeleteUser,
		},
		{
			Name:         "set_user_password",
			Namespace:    "users",
			Description:  "admin set user password (revokes target tokens; clears must_change; response includes user_name and admin_op_busy/last)",
			Auth:         authAdmin,
			GRPCMethod:   grpcMethodSetUserPassword,
			GRPCRequest:  &setUserPasswordRequest{},
			GRPCFn:       grpcSetUserPassword,
			ResponseHint: "setPasswordResponse{ok,user_name,admin_op_busy,op,started_at_unix,last}",
		},
		{
			Name:        "grant_database_permission",
			Namespace:   "users",
			Description: "grant DB permission",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodGrantDatabasePermission,
			GRPCRequest: &databasePermissionRequest{},
			GRPCFn:      grpcGrantDatabasePermission,
		},
		{
			Name:        "revoke_database_permission",
			Namespace:   "users",
			Description: "revoke DB permission",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodRevokeDatabasePermission,
			GRPCRequest: &databasePermissionRequest{},
			GRPCFn:      grpcRevokeDatabasePermission,
		},
		{
			Name:        "list_database_permissions",
			Namespace:   "users",
			Description: "list DB permissions (includes admin_op_busy/last)",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodListDatabasePermissions,
			GRPCRequest: &userNameRequest{},
			GRPCFn:      grpcListDatabasePermissions,
		},

		// admin databases
		{
			Name:        "admin_databases",
			Namespace:   "admin",
			Description: "list or create databases (GET includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet, http.MethodPost},
			HTTPPaths:   []string{routeAdminDatabases},
			HTTPHandler: (*serverRuntime).handleAdminDatabases,
		},
		{
			Name:         "admin_database_resource",
			Namespace:    "admin",
			Description:  "drop database or manage retention policies",
			Auth:         authAdmin,
			HTTPMethods:  []string{http.MethodGet, http.MethodPost, http.MethodDelete},
			HTTPPaths:    []string{routeAdminDatabasesPrefix},
			HTTPHandler:  (*serverRuntime).handleAdminDatabaseResource,
			ResourceOnly: true,
		},
		{
			Name:        "create_database",
			Namespace:   "admin",
			Description: "create database",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodCreateDatabase,
			GRPCRequest: &databaseRequest{},
			GRPCFn:      grpcCreateDatabase,
		},
		{
			Name:        "list_databases",
			Namespace:   "admin",
			Description: "list databases (includes admin_op_busy/last)",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodListDatabases,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcListDatabases,
		},
		{
			Name:        "drop_database",
			Namespace:   "admin",
			Description: "drop database",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodDropDatabase,
			GRPCRequest: &databaseRequest{},
			GRPCFn:      grpcDropDatabase,
		},
		{
			Name:        "create_retention_policy",
			Namespace:   "admin",
			Description: "create retention policy",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodCreateRetentionPolicy,
			GRPCRequest: &grpcRetentionPolicyRequest{},
			GRPCFn:      grpcCreateRetentionPolicy,
		},
		{
			Name:        "list_retention_policies",
			Namespace:   "admin",
			Description: "list retention policies (includes admin_op_busy/last)",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodListRetentionPolicies,
			GRPCRequest: &databaseRequest{},
			GRPCFn:      grpcListRetentionPolicies,
		},

		// admin config / contract
		{
			Name:        "get_config",
			Namespace:   "admin",
			Description: "read config (includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAdminConfig, routeAdminConfigEffective},
			HTTPHandler: (*serverRuntime).handleConfig,
			GRPCMethod:  grpcMethodGetConfig,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcGetConfig,
		},
		{
			Name:        "get_effective_config",
			Namespace:   "admin",
			Description: "read effective config (includes admin_op_busy/last)",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodGetEffectiveConfig,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcGetConfig,
		},
		{
			Name:        "get_config_schema",
			Namespace:   "admin",
			Description: "read config schema (includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAdminConfigSchema},
			HTTPHandler: (*serverRuntime).handleConfigSchema,
			GRPCMethod:  grpcMethodGetConfigSchema,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcGetConfigSchema,
		},
		{
			Name:        "validate_config",
			Namespace:   "admin",
			Description: "validate config payload (response includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeAdminConfigValidate},
			HTTPHandler: (*serverRuntime).handleValidateConfig,
			GRPCMethod:  grpcMethodValidateConfig,
			GRPCRequest: &configValidateRequest{},
			GRPCFn:      grpcValidateConfig,
		},
		{
			Name:        "reload_config",
			Namespace:   "admin",
			Description: "reload hot fields (response includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeAdminConfigReload},
			HTTPHandler: (*serverRuntime).handleReloadConfig,
			GRPCMethod:  grpcMethodReloadConfig,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcReloadConfig,
		},
		{
			Name:        "get_api_spec",
			Namespace:   "admin",
			Description: "read API contract (includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAdminAPISpec},
			HTTPHandler: (*serverRuntime).handleAPISpec,
			GRPCMethod:  grpcMethodGetAPISpec,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcGetAPISpec,
		},
		{
			Name:        "get_error_codes",
			Namespace:   "admin",
			Description: "read error code contract (includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAdminErrorCodes},
			HTTPHandler: (*serverRuntime).handleErrorCodes,
			GRPCMethod:  grpcMethodGetErrorCodes,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcGetErrorCodes,
		},

		// maintenance
		{
			Name:        "flush",
			Namespace:   "admin",
			Description: "flush memtables (response includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeAdminFlush},
			HTTPHandler: (*serverRuntime).handleFlush,
			GRPCMethod:  grpcMethodFlush,
			GRPCHandler: grpcFlushHandler,
		},
		{
			Name:        "compact",
			Namespace:   "admin",
			Description: "run compaction (response includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeAdminCompact},
			HTTPHandler: (*serverRuntime).handleCompact,
			GRPCMethod:  grpcMethodCompact,
			GRPCHandler: grpcCompactHandler,
		},
		{
			Name:        "apply_retention",
			Namespace:   "admin",
			Description: "apply retention (response includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeAdminRetentionApply},
			HTTPHandler: (*serverRuntime).handleApplyRetention,
			GRPCMethod:  grpcMethodApplyRetention,
			GRPCRequest: &retentionApplyRequest{},
			GRPCFn:      grpcApplyRetention,
		},
		{
			Name:        "maintenance_errors",
			Namespace:   "admin",
			Description: "maintenance error list with admin_op_busy/last",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAdminMaintenanceErrors},
			HTTPHandler: (*serverRuntime).handleMaintenanceErrors,
			GRPCMethod:  grpcMethodMaintenanceErrors,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcMaintenanceErrors,
		},
		{
			Name:        "maintenance_stats",
			Namespace:   "admin",
			Description: "maintenance task stats snapshot (includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAdminStatsMaintenance},
			HTTPHandler: (*serverRuntime).handleMaintenanceStats,
			GRPCMethod:  grpcMethodMaintenanceStats,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcMaintenanceStats,
		},
		{
			Name:        "ops_status",
			Namespace:   "admin",
			Description: "lightweight admin heavy-op busy/last status (admin_op_busy, op, started_at_unix, last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAdminOpsStatus},
			HTTPHandler: (*serverRuntime).handleOpsStatus,
			GRPCMethod:  grpcMethodOpsStatus,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcOpsStatus,
		},
		{
			Name:        "storage_memory",
			Namespace:   "admin",
			Description: "storage memory snapshot (includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAdminStatsStorageMemory},
			HTTPHandler: (*serverRuntime).handleStorageMemory,
			GRPCMethod:  grpcMethodStorageMemory,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcStorageMemory,
		},
		{
			Name:        "compaction_stats",
			Namespace:   "admin",
			Description: "compaction stats (includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAdminStatsCompaction},
			HTTPHandler: (*serverRuntime).handleCompactionStats,
			GRPCMethod:  grpcMethodCompactionStats,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcCompactionStats,
		},
		{
			Name:        "admin_doctor",
			Namespace:   "admin",
			Description: "production doctor readiness checks (includes admin_op_busy/last when served over HTTP)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAdminDoctor},
			HTTPHandler: (*serverRuntime).handleAdminDoctor,
		},
		{
			Name:        "admin_version",
			Namespace:   "admin",
			Description: "server build version information (includes admin_op_busy/last for dashboard alignment)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAdminVersion},
			HTTPHandler: (*serverRuntime).handleAdminVersion,
		},
		{
			Name:        "admin_health",
			Namespace:   "admin",
			Description: "admin health snapshot with admin_op_busy/last for dashboard polling alignment",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAdminHealth},
			HTTPHandler: (*serverRuntime).handleAdminHealth,
			GRPCMethod:  grpcMethodAdminHealth,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcAdminHealth,
		},
		{
			Name:        "storage_validate",
			Namespace:   "admin",
			Description: "validate local storage state (includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeAdminStorageValidate},
			HTTPHandler: (*serverRuntime).handleStorageValidate,
			GRPCMethod:  grpcMethodStorageValidate,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcStorageValidate,
		},
		{
			Name:        "storage_data_snapshot",
			Namespace:   "admin",
			Description: "snapshot live data_dir into backup root (response includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeAdminStorageDataSnapshot},
			HTTPHandler: (*serverRuntime).handleStorageDataSnapshot,
		},
		{
			Name:        "storage_restore_drill",
			Namespace:   "admin",
			Description: "restore data-snapshot to side-path under backup root (response includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeAdminStorageRestoreDrill},
			HTTPHandler: (*serverRuntime).handleStorageRestoreDrill,
		},
		{
			Name:        "storage_data_snapshots",
			Namespace:   "admin",
			Description: "list data-snapshot and restore-drill directories (includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAdminStorageDataSnapshots},
			HTTPHandler: (*serverRuntime).handleListStorageDataSnapshots,
		},
		{
			Name:        "storage_snapshot",
			Namespace:   "admin",
			Description: "write local manifest snapshot (response includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeAdminStorageSnapshot},
			HTTPHandler: (*serverRuntime).handleStorageSnapshot,
			GRPCMethod:  grpcMethodStorageSnapshot,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcStorageSnapshot,
		},
		{
			Name:        "list_storage_snapshots",
			Namespace:   "admin",
			Description: "list or delete local storage snapshots (GET includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet, http.MethodDelete},
			HTTPPaths:   []string{routeAdminStorageSnapshots},
			HTTPHandler: (*serverRuntime).handleStorageSnapshots,
			GRPCMethod:  grpcMethodListStorageSnapshots,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcListStorageSnapshots,
		},
		{
			Name:        "delete_storage_snapshot",
			Namespace:   "admin",
			Description: "delete a local storage snapshot by name (gRPC)",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodDeleteStorageSnapshot,
			GRPCRequest: &storageSnapshotDeleteRequest{},
			GRPCFn:      grpcDeleteStorageSnapshot,
		},
		{
			Name:        "list_audit",
			Namespace:   "admin",
			Description: "list audit events with optional filters (includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAdminAudit},
			HTTPHandler: (*serverRuntime).handleListAudit,
			GRPCMethod:  grpcMethodListAudit,
			GRPCRequest: &auditListRequest{},
			GRPCFn:      grpcListAudit,
		},

		{
			Name:        "storage_export",
			Namespace:   "admin",
			Description: "export server metadata summary (includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAdminStorageExport},
			HTTPHandler: (*serverRuntime).handleStorageExport,
			GRPCMethod:  grpcMethodStorageExport,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcStorageExport,
		},

		// downsample
		{
			Name:        "downsample_policies",
			Namespace:   "admin",
			Description: "list or upsert downsample policies (GET includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut},
			HTTPPaths:   []string{routeAdminDownsamplePolicies},
			HTTPHandler: (*serverRuntime).handleDownsamplePolicies,
		},
		{
			Name:         "downsample_policy_resource",
			Namespace:    "admin",
			Description:  "downsample policy actions",
			Auth:         authAdmin,
			HTTPMethods:  []string{http.MethodGet, http.MethodPost, http.MethodDelete},
			HTTPPaths:    []string{routeAdminDownsamplePrefix},
			HTTPHandler:  (*serverRuntime).handleDownsamplePolicyResource,
			ResourceOnly: true,
		},
		{
			Name:        "downsample_statuses",
			Namespace:   "admin",
			Description: "downsample policy statuses (includes admin_op_busy/last)",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodGet},
			HTTPPaths:   []string{routeAdminDownsampleStatuses},
			HTTPHandler: (*serverRuntime).handleDownsampleStatuses,
			GRPCMethod:  grpcMethodDownsamplePolicyStatuses,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcDownsamplePolicyStatuses,
		},
		{
			Name:        "create_downsample_policy",
			Namespace:   "admin",
			Description: "create downsample policy",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodCreateDownsamplePolicy,
			GRPCRequest: &mts.DownsamplePolicy{},
			GRPCFn:      grpcCreateDownsamplePolicy,
		},
		{
			Name:        "list_downsample_policies",
			Namespace:   "admin",
			Description: "list downsample policies (includes admin_op_busy/last)",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodListDownsamplePolicies,
			GRPCRequest: &emptyRequest{},
			GRPCFn:      grpcListDownsamplePolicies,
		},
		{
			Name:        "enable_downsample_policy",
			Namespace:   "admin",
			Description: "enable downsample policy",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodEnableDownsamplePolicy,
			GRPCRequest: &downsamplePolicyRequest{},
			GRPCFn:      grpcEnableDownsamplePolicy,
		},
		{
			Name:        "disable_downsample_policy",
			Namespace:   "admin",
			Description: "disable downsample policy",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodDisableDownsamplePolicy,
			GRPCRequest: &downsamplePolicyRequest{},
			GRPCFn:      grpcDisableDownsamplePolicy,
		},
		{
			Name:        "batch_downsample_policies",
			Namespace:   "admin",
			Description: "batch enable or disable downsample policies",
			Auth:        authAdmin,
			HTTPMethods: []string{http.MethodPost},
			HTTPPaths:   []string{routeAdminDownsampleBatch},
			HTTPHandler: (*serverRuntime).handleBatchDownsamplePolicies,
			GRPCMethod:  grpcMethodBatchDownsamplePolicies,
			GRPCRequest: &batchDownsampleRequest{},
			GRPCFn:      grpcBatchDownsamplePolicies,
		},
		{
			Name:        "drop_downsample_policy",
			Namespace:   "admin",
			Description: "drop downsample policy with optional target cleanup",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodDropDownsamplePolicy,
			GRPCRequest: &grpcDownsampleDropRequest{},
			GRPCFn:      grpcDropDownsamplePolicy,
		},
		{
			Name:        "reset_downsample_policy",
			Namespace:   "admin",
			Description: "reset downsample policy",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodResetDownsamplePolicy,
			GRPCRequest: &grpcDownsampleResetRequest{},
			GRPCFn:      grpcResetDownsamplePolicy,
		},
		{
			Name:        "run_downsample_policy",
			Namespace:   "admin",
			Description: "run downsample policy (response includes admin_op_busy/last)",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodRunDownsamplePolicy,
			GRPCRequest: &downsamplePolicyRangeRequest{},
			GRPCFn:      grpcRunDownsamplePolicy,
		},
		{
			Name:        "run_downsample_policy_range",
			Namespace:   "admin",
			Description: "run downsample policy range (response includes admin_op_busy/last)",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodRunDownsamplePolicyRange,
			GRPCRequest: &downsamplePolicyRangeRequest{},
			GRPCFn:      grpcRunDownsamplePolicyRange,
		},
		{
			Name:        "repair_downsample_policy",
			Namespace:   "admin",
			Description: "repair downsample policy (response includes admin_op_busy/last)",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodRepairDownsamplePolicy,
			GRPCRequest: &downsamplePolicyRangeRequest{},
			GRPCFn:      grpcRepairDownsamplePolicy,
		},
		{
			Name:        "dry_run_downsample_policy",
			Namespace:   "admin",
			Description: "dry-run downsample policy (response includes admin_op_busy/last)",
			Auth:        authAdmin,
			GRPCMethod:  grpcMethodDryRunDownsamplePolicy,
			GRPCRequest: &downsamplePolicyRangeRequest{},
			GRPCFn:      grpcDryRunDownsamplePolicy,
		},
	}
}

func (r *serverRuntime) mountRegistryHTTP(mux *http.ServeMux) {
	seen := make(map[string]struct{})
	for _, op := range operationCatalog() {
		if op.HTTPHandler == nil {
			continue
		}
		handler := op.HTTPHandler
		for _, path := range op.HTTPPaths {
			if path == "" {
				continue
			}
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			mux.HandleFunc(path, func(writer http.ResponseWriter, request *http.Request) {
				handler(r, writer, request)
			})
		}
	}
}

func grpcMethodsFromRegistry() []grpc.MethodDesc {
	ops := operationCatalog()
	methods := make([]grpc.MethodDesc, 0, len(ops))
	seen := make(map[string]struct{}, len(ops))
	for _, op := range ops {
		if op.GRPCMethod == "" {
			continue
		}
		if _, ok := seen[op.GRPCMethod]; ok {
			panic("duplicate grpc method in operation registry: " + op.GRPCMethod)
		}
		seen[op.GRPCMethod] = struct{}{}
		switch {
		case op.GRPCHandler != nil:
			methods = append(methods, grpc.MethodDesc{
				MethodName: op.GRPCMethod,
				Handler:    op.GRPCHandler,
			})
		case op.GRPCFn != nil:
			methods = append(methods, grpc.MethodDesc{
				MethodName: op.GRPCMethod,
				Handler:    unaryHandler(op.GRPCMethod, op.GRPCRequest, op.GRPCFn),
			})
		default:
			panic("grpc operation missing handler: " + op.GRPCMethod)
		}
	}
	return methods
}

func apiSpecFromRegistry() apiSpecResponse {
	namespaces := make(map[string]*apiNamespace)
	order := make([]string, 0)
	for _, op := range operationCatalog() {
		if len(op.HTTPPaths) == 0 || len(op.HTTPMethods) == 0 {
			continue
		}
		ns, ok := namespaces[op.Namespace]
		if !ok {
			ns = &apiNamespace{
				Name:     op.Namespace,
				BasePath: namespaceBasePath(op.Namespace),
			}
			namespaces[op.Namespace] = ns
			order = append(order, op.Namespace)
		}
		for _, path := range op.HTTPPaths {
			for _, method := range op.HTTPMethods {
				ns.Endpoints = append(ns.Endpoints, apiEndpoint{
					Method:      method,
					Path:        path,
					Auth:        string(op.Auth),
					Description: op.Description,
					Response:    op.ResponseHint,
				})
			}
		}
	}
	out := apiSpecResponse{Version: "v1", Namespaces: make([]apiNamespace, 0, len(order))}
	for _, name := range order {
		out.Namespaces = append(out.Namespaces, *namespaces[name])
	}
	return out
}

func namespaceBasePath(namespace string) string {
	switch namespace {
	case "data":
		return "/api/v1/data"
	case "auth", "authz":
		return "/api/v1/auth"
	case "users":
		return "/api/v1/users"
	case "admin":
		return "/api/v1/admin"
	case "system":
		return "/"
	default:
		return "/api/v1/" + namespace
	}
}
