package main

import (
	"time"

	mts "github.com/openmts/mts"
)

type errorCode string

const (
	errorCodeBadRequest        errorCode = "bad_request"
	errorCodeUnauthenticated   errorCode = "unauthenticated"
	errorCodePermissionDenied  errorCode = "permission_denied"
	errorCodeResourceExhausted errorCode = "resource_exhausted"
	errorCodeNotFound          errorCode = "not_found"
	errorCodeAlreadyExists     errorCode = "already_exists"
	errorCodeInternal          errorCode = "internal"
)

type okResponse struct {
	OK            bool                  `json:"ok"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type errorResponse struct {
	OK          bool      `json:"ok"`
	Code        errorCode `json:"code"`
	Message     string    `json:"message"`
	Error       string    `json:"error,omitempty"`
	AdminOpBusy bool      `json:"admin_op_busy,omitempty"`
	Op          string    `json:"op,omitempty"`
}

type writeRequest struct {
	Points  []mts.Point      `json:"points"`
	Options mts.WriteOptions `json:"options"`
}

type writeResponse struct {
	OK            bool                  `json:"ok"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type typedWriteRequest struct {
	Batch   mts.TypedBatch   `json:"batch"`
	Options mts.WriteOptions `json:"options"`
}

type queryRequest struct {
	Query mts.Query `json:"query"`
}

type queryRowsRequest = queryRequest

type queryRowsResponse struct {
	Rows  []mts.Row      `json:"rows"`
	Stats mts.QueryStats `json:"stats"`
}

type queryColumnsResponse struct {
	Columns []mts.ColumnSeries `json:"columns"`
	Stats   mts.QueryStats     `json:"stats"`
}

type queryExplainResponse struct {
	Result mts.QueryResult `json:"result"`
}

type queryStatsResponse struct {
	Stats         mts.QueryStats        `json:"stats"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type streamRecord struct {
	Type   string            `json:"type"`
	Row    *mts.Row          `json:"row,omitempty"`
	Column *mts.ColumnSeries `json:"column,omitempty"`
	Error  *errorResponse    `json:"error,omitempty"`
	Stats  *mts.QueryStats   `json:"stats,omitempty"`
}

// queryStreamRequest 控制 NDJSON 流式查询形态。
// Format: "row"（默认）或 "column"；兼容 Mode 字段。
type queryStreamRequest struct {
	Query  mts.Query `json:"query"`
	Format string    `json:"format,omitempty"`
	Mode   string    `json:"mode,omitempty"`
}

type deleteRequest struct {
	Request mts.DeleteRequest `json:"request"`
}

// adminHeavyLastResult 最近一次管理重操作结果（空闲时供 Dashboard 展示）。
type adminHeavyLastResult struct {
	Op             string `json:"op,omitempty"`
	OK             bool   `json:"ok"`
	Error          string `json:"error,omitempty"`
	StartedAtUnix  int64  `json:"started_at_unix,omitempty"`
	FinishedAtUnix int64  `json:"finished_at_unix,omitempty"`
	DurationMs     int64  `json:"duration_ms,omitempty"`
}

type maintenanceStatsResponse struct {
	Stats         mts.MaintenanceStats  `json:"stats"`
	AdminOpBusy   bool                  `json:"admin_op_busy"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

// opsStatusResponse 轻量运维互斥状态，供 Dashboard 高频轮询。
type opsStatusResponse struct {
	AdminOpBusy   bool                  `json:"admin_op_busy"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type userResponse struct {
	User mts.User `json:"user"`
}

type userNameRequest struct {
	Name string `json:"name"`
}

type usersResponse struct {
	Users         []mts.User            `json:"users"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

// batchUserDisabledRequest 批量设置用户 disabled 状态。
type batchUserDisabledRequest struct {
	Names    []string `json:"names"`
	Disabled bool     `json:"disabled"`
}

// batchItemResult 单项批处理结果。
type batchItemResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // ok | skip | error
	Message string `json:"message,omitempty"`
}

// batchMutationResponse 批量写操作汇总。
type batchMutationResponse struct {
	OK            bool                  `json:"ok"`
	OKCount       int                   `json:"ok_count"`
	Skip          int                   `json:"skip_count"`
	Fail          int                   `json:"fail_count"`
	Items         []batchItemResult     `json:"items"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

// batchDownsampleRequest 批量启用/禁用降采样策略。
type batchDownsampleRequest struct {
	Names  []string `json:"names"`
	Action string   `json:"action"` // enable | disable
}

type createUserRequest struct {
	mts.User
	Password string `json:"password,omitempty"`
}

type databasePermissionsResponse struct {
	Grants        []mts.DatabaseGrant   `json:"grants"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type loginRequest struct {
	UserName   string `json:"user_name"`
	Password   string `json:"password"`
	TTLSeconds int64  `json:"ttl_seconds,omitempty"`
}

type authTokenResponse struct {
	Token              mts.AuthToken `json:"token"`
	MustChangePassword bool          `json:"must_change_password,omitempty"`
	// RemainingSeconds / ServerTimeUnix 与 GET /auth/session 对齐，供登录即校准。
	RemainingSeconds int64 `json:"remaining_seconds"`
	ServerTimeUnix   int64 `json:"server_time_unix"`
}

// passwordPolicyResponse 公开密码策略（与 Dashboard 校验对齐，无需登录）。
type passwordPolicyResponse struct {
	OK                     bool     `json:"ok"`
	MinLength              int      `json:"min_length"`
	ForbiddenDefaults      []string `json:"forbidden_defaults"`
	RequireChangeBootstrap bool     `json:"require_change_bootstrap"`
	DefaultAuthTTLSeconds  int64    `json:"default_auth_ttl_seconds"`
	MaxAuthTTLSeconds      int64    `json:"max_auth_ttl_seconds"`
	Version                int      `json:"version"`
}

type sessionResponse struct {
	OK                 bool                  `json:"ok"`
	UserName           string                `json:"user_name"`
	Role               mts.UserRole          `json:"role,omitempty"`
	ExpiresAt          time.Time             `json:"expires_at"`
	MustChangePassword bool                  `json:"must_change_password,omitempty"`
	RemainingSeconds   int64                 `json:"remaining_seconds"`
	ServerTimeUnix     int64                 `json:"server_time_unix"`
	AdminOpBusy        bool                  `json:"admin_op_busy,omitempty"`
	Op                 string                `json:"op,omitempty"`
	StartedAtUnix      int64                 `json:"started_at_unix,omitempty"`
	Last               *adminHeavyLastResult `json:"last,omitempty"`
}

type logoutRequest struct {
	Token string `json:"token,omitempty"`
}

type passwordRequest struct {
	Password string `json:"password"`
}

type setUserPasswordRequest struct {
	UserName string `json:"user_name"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	UserName    string `json:"user_name"`
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// changePasswordResponse 改密成功体：会话已撤销，客户端须重新登录；must_change 恒为 false。
type changePasswordResponse struct {
	OK                 bool                  `json:"ok"`
	MustChangePassword bool                  `json:"must_change_password"`
	AdminOpBusy        bool                  `json:"admin_op_busy,omitempty"`
	Op                 string                `json:"op,omitempty"`
	StartedAtUnix      int64                 `json:"started_at_unix,omitempty"`
	Last               *adminHeavyLastResult `json:"last,omitempty"`
}

// setPasswordResponse 管理员设置密码成功响应（含目标用户，便于 dashboard 对齐会话边界）
type setPasswordResponse struct {
	OK            bool                  `json:"ok"`
	UserName      string                `json:"user_name,omitempty"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type authzDatabaseCheckRequest struct {
	UserName   string                 `json:"user_name"`
	Database   string                 `json:"database"`
	Permission mts.DatabasePermission `json:"permission"`
}

type databasePermissionRequest struct {
	UserName   string                 `json:"user_name"`
	Database   string                 `json:"database"`
	Permission mts.DatabasePermission `json:"permission"`
}

type authzDatabaseCheckResponse struct {
	Allowed bool `json:"allowed"`
}

type databaseRequest struct {
	Name string `json:"name"`
}

type databasesResponse struct {
	Databases     []string              `json:"databases"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type retentionPolicyRequest struct {
	Policy mts.RetentionPolicy `json:"policy"`
}

type grpcRetentionPolicyRequest struct {
	Database string              `json:"database"`
	Policy   mts.RetentionPolicy `json:"policy"`
}

type retentionPoliciesResponse struct {
	Policies      []mts.RetentionPolicy `json:"policies"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type measurementsResponse struct {
	// Databases 为正式字段；Measurements 保留兼容历史客户端。
	Databases     []string              `json:"databases,omitempty"`
	Measurements  []string              `json:"measurements,omitempty"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type fieldsResponse struct {
	Fields        []mts.FieldSchema     `json:"fields"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type metadataRequest struct {
	Database    string            `json:"database"`
	Measurement string            `json:"measurement"`
	Tags        map[string]string `json:"tags,omitempty"`
}

type seriesResponse struct {
	Series        []mts.Series          `json:"series"`
	Total         int                   `json:"total,omitempty"`
	Truncated     bool                  `json:"truncated,omitempty"`
	Limit         int                   `json:"limit,omitempty"`
	Offset        int                   `json:"offset,omitempty"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type configResponse struct {
	Config        config                `json:"config"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type configSchemaResponse struct {
	Fields        []configFieldSchema   `json:"fields"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type configValidateRequest struct {
	Config config `json:"config"`
}

type configValidateResponse struct {
	OK            bool                  `json:"ok"`
	Error         string                `json:"error,omitempty"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type reloadConfigResponse struct {
	OK            bool                  `json:"ok"`
	Fields        []string              `json:"fields"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type configFieldSchema struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type maintenanceResponse struct {
	OK            bool                  `json:"ok"`
	Result        mts.CompactionResult  `json:"result,omitempty"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type retentionApplyRequest struct {
	NowUnixNanos int64 `json:"now_unix_nanos"`
}

type maintenanceErrorsResponse struct {
	Errors        []string              `json:"errors"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type versionResponse struct {
	Version       string                `json:"version"`
	Commit        string                `json:"commit"`
	BuiltAt       string                `json:"built_at"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type apiSpecResponse struct {
	Version       string                `json:"version"`
	Namespaces    []apiNamespace        `json:"namespaces"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type apiNamespace struct {
	Name      string        `json:"name"`
	BasePath  string        `json:"base_path"`
	Endpoints []apiEndpoint `json:"endpoints"`
}

type apiEndpoint struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Auth        string `json:"auth"`
	Description string `json:"description"`
	// Response 为人类可读响应说明（可选，便于 Dashboard/契约浏览对齐）
	Response string `json:"response,omitempty"`
}

type errorCodesResponse struct {
	Codes         []errorCodeSpec       `json:"codes"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type errorCodeSpec struct {
	Code        errorCode `json:"code"`
	HTTPStatus  int       `json:"http_status"`
	GRPCCode    string    `json:"grpc_code"`
	Description string    `json:"description"`
}

type storageValidateResponse struct {
	OK            bool                  `json:"ok"`
	DataDir       string                `json:"data_dir"`
	Health        mts.HealthSnapshot    `json:"health"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type adminHealthResponse struct {
	Health        mts.HealthSnapshot    `json:"health"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type storageSnapshotResponse struct {
	OK            bool                  `json:"ok"`
	Path          string                `json:"path"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type storageDataSnapshotRequest struct {
	Flush *bool `json:"flush,omitempty"`
}

type storageDataSnapshotResponse struct {
	OK            bool                  `json:"ok"`
	Path          string                `json:"path"`
	Source        string                `json:"source"`
	Files         int                   `json:"files"`
	Bytes         int64                 `json:"bytes"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type storageRestoreDrillRequest struct {
	SourcePath string `json:"source_path,omitempty"`
}

type storageRestoreDrillResponse struct {
	OK            bool                  `json:"ok"`
	Source        string                `json:"source"`
	Target        string                `json:"target"`
	Files         int                   `json:"files"`
	Bytes         int64                 `json:"bytes"`
	CheckIssues   int                   `json:"check_issues"`
	CheckFatals   int                   `json:"check_fatals"`
	CheckRoot     string                `json:"check_root"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type storageDataSnapshotInfo struct {
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Path      string    `json:"path"`
	SizeBytes int64     `json:"size_bytes"`
	ModTime   time.Time `json:"mod_time"`
}

type storageDataSnapshotsResponse struct {
	Snapshots     []storageDataSnapshotInfo `json:"snapshots"`
	AdminOpBusy   bool                      `json:"admin_op_busy,omitempty"`
	Op            string                    `json:"op,omitempty"`
	StartedAtUnix int64                     `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult     `json:"last,omitempty"`
}

type storageExportResponse struct {
	Export        storageExport         `json:"export"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type storageExport struct {
	GeneratedAt time.Time                      `json:"generated_at"`
	Config      config                         `json:"config"`
	Health      mts.HealthSnapshot             `json:"health"`
	Users       []mts.User                     `json:"users"`
	Grants      map[string][]mts.DatabaseGrant `json:"grants"`
}

type storageMemoryResponse struct {
	Snapshot      mts.StorageMemorySnapshot `json:"snapshot"`
	AdminOpBusy   bool                      `json:"admin_op_busy,omitempty"`
	Op            string                    `json:"op,omitempty"`
	StartedAtUnix int64                     `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult     `json:"last,omitempty"`
}

type compactionStatsResponse struct {
	Stats         mts.CompactionStats   `json:"stats"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type downsamplePoliciesResponse struct {
	Policies      []mts.DownsamplePolicy `json:"policies"`
	AdminOpBusy   bool                   `json:"admin_op_busy,omitempty"`
	Op            string                 `json:"op,omitempty"`
	StartedAtUnix int64                  `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult  `json:"last,omitempty"`
}

type downsamplePolicyResponse struct {
	Policy        mts.DownsamplePolicy  `json:"policy"`
	AdminOpBusy   bool                  `json:"admin_op_busy,omitempty"`
	Op            string                `json:"op,omitempty"`
	StartedAtUnix int64                 `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult `json:"last,omitempty"`
}

type downsampleStatusSummary struct {
	Total         int   `json:"total"`
	Enabled       int   `json:"enabled"`
	Active        int   `json:"active"`
	Error         int   `json:"error"`
	Lagging       int   `json:"lagging"`
	MaxLagSeconds int64 `json:"max_lag_seconds"`
}

type downsampleStatusesResponse struct {
	Statuses      []mts.DownsamplePolicyStatus `json:"statuses"`
	Summary       *downsampleStatusSummary     `json:"summary,omitempty"`
	AdminOpBusy   bool                         `json:"admin_op_busy,omitempty"`
	Op            string                       `json:"op,omitempty"`
	StartedAtUnix int64                        `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult        `json:"last,omitempty"`
}

type downsamplePolicyStatusResponse struct {
	Status        mts.DownsamplePolicyStatus `json:"status"`
	AdminOpBusy   bool                       `json:"admin_op_busy,omitempty"`
	Op            string                     `json:"op,omitempty"`
	StartedAtUnix int64                      `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult      `json:"last,omitempty"`
}

type downsampleResetRequest struct {
	Reset mts.DownsampleReset `json:"reset"`
}

type grpcDownsampleResetRequest struct {
	Name  string              `json:"name"`
	Reset mts.DownsampleReset `json:"reset"`
}

type downsampleDropRequest struct {
	Options mts.DownsampleDropOptions `json:"options"`
}

type grpcDownsampleDropRequest struct {
	Name    string                    `json:"name"`
	Options mts.DownsampleDropOptions `json:"options"`
}

type downsampleRunRequest struct {
	NowUnix int64 `json:"now_unix"`
}

type downsampleRangeRequest struct {
	StartUnix int64                      `json:"start_unix"`
	EndUnix   int64                      `json:"end_unix"`
	Options   mts.DownsampleRangeOptions `json:"options"`
}

type downsamplePolicyRequest struct {
	Name string `json:"name"`
}

type downsamplePolicyRangeRequest struct {
	Name      string                     `json:"name"`
	StartUnix int64                      `json:"start_unix"`
	EndUnix   int64                      `json:"end_unix"`
	Options   mts.DownsampleRangeOptions `json:"options"`
}

type downsampleRunResponse struct {
	Result        mts.DownsampleRunResult `json:"result"`
	AdminOpBusy   bool                    `json:"admin_op_busy,omitempty"`
	Op            string                  `json:"op,omitempty"`
	StartedAtUnix int64                   `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult   `json:"last,omitempty"`
}

type downsampleDryRunResponse struct {
	Result        mts.DownsampleDryRunResult `json:"result"`
	AdminOpBusy   bool                       `json:"admin_op_busy,omitempty"`
	Op            string                     `json:"op,omitempty"`
	StartedAtUnix int64                      `json:"started_at_unix,omitempty"`
	Last          *adminHeavyLastResult      `json:"last,omitempty"`
}

type emptyRequest struct{}

func unixNanosOrNow(value int64) time.Time {
	if value == 0 {
		return time.Now()
	}
	return time.Unix(0, value)
}

type storageSnapshotDeleteRequest struct {
	Name string `json:"name"`
}
