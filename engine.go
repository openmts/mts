package mts

import (
	"context"
	"time"

	storageengine "github.com/openmts/mts/internal/engine"
	"github.com/openmts/mts/internal/model"
	"github.com/openmts/mts/internal/observability"
)

// Close 关闭 Engine 并释放本地资源。
func (e *Engine) Close(ctx context.Context) error {
	return publicError(e.runtime.Close(ctx))
}

// Write 写入点数据。
//
// 便于兼容按点构造的调用方；每个 Point 的 Tags/Fields map 会带来额外分配与转换开销。
// 高吞吐或宽字段场景请优先使用 WriteTypedBatch；若已有同构 []Point，可改用
// WritePointsAsTypedBatch。
func (e *Engine) Write(ctx context.Context, points []Point, opts WriteOptions) error {
	converted, err := toModelPoints(points)
	if err != nil {
		return err
	}
	return publicError(e.runtime.Storage().Write(ctx, converted, toModelWriteOptions(opts)))
}

// WriteTypedBatch 写入按列组织的批量数据。
//
// 这是推荐的高性能写入入口：调用方直接提供列式 tag/field，可显著降低分配与
// CPU 开销。宽表、大批量、持续高吞吐场景应优先使用本方法，而不是 Write。
func (e *Engine) WriteTypedBatch(ctx context.Context, batch TypedBatch, opts WriteOptions) error {
	converted, err := toModelTypedBatch(batch)
	if err != nil {
		return err
	}
	return publicError(e.runtime.Storage().WriteTypedBatch(ctx, converted, toModelWriteOptions(opts)))
}

// WritePointsAsTypedBatch 将同构 []Point 转换为列式 TypedBatch 后写入。
//
// 适合调用方仍持有 []Point，但希望走 WriteTypedBatch 热路径的场景。
// 转换规则与 PointsToTypedBatch 相同；若 batch 异构则返回错误。
// 性能介于 Write 与直接 WriteTypedBatch 之间：仍有一次 []Point→列式转换成本，
// 新采集路径若可直接构造 TypedBatch，请优先调用 WriteTypedBatch。
func (e *Engine) WritePointsAsTypedBatch(ctx context.Context, points []Point, opts WriteOptions) error {
	batch, err := PointsToTypedBatch(points)
	if err != nil {
		return err
	}
	return e.WriteTypedBatch(ctx, batch, opts)
}

// Flush 将当前内存数据刷写为本地 SSTable。
func (e *Engine) Flush(ctx context.Context) error {
	return publicError(e.runtime.Storage().Flush(ctx))
}

// QueryColumns 返回完整列式查询结果。
//
// 该方法会 materialize 完整结果，生产场景的大结果查询优先使用
// QueryColumnIterator 并配置 Limit 或 QueryBudget。
//
// 性能提示：宽表扫描、聚合、导出等场景优先列式 API（QueryColumnIterator），
// 避免 QueryRowIterator 的行拼装与 map 分配；再配合 Fields 投影只读必要字段。
func (e *Engine) QueryColumns(ctx context.Context, query Query) ([]ColumnSeries, error) {
	converted, factor, err := toModelQuery(query)
	if err != nil {
		return nil, err
	}
	columns, err := e.runtime.Storage().QueryColumns(ctx, converted)
	if err != nil {
		return nil, publicError(err)
	}
	return fromModelColumnSeriesList(columns, factor), nil
}

// QueryColumnIterator 以 iterator 方式返回列式查询结果。
func (e *Engine) QueryColumnIterator(ctx context.Context, query Query) (ColumnIterator, error) {
	converted, factor, err := toModelQuery(query)
	if err != nil {
		return nil, err
	}
	inner, err := e.runtime.Storage().QueryColumnIterator(ctx, converted)
	if err != nil {
		return nil, publicError(err)
	}
	return columnIterator{inner: inner, factor: factor}, nil
}

// QueryWithExplain 返回列式查询结果、查询计划说明和执行统计。
func (e *Engine) QueryWithExplain(ctx context.Context, query Query) (QueryResult, error) {
	converted, factor, err := toModelQuery(query)
	if err != nil {
		return QueryResult{}, err
	}
	columns, explain, stats, err := e.runtime.Storage().QueryWithExplain(ctx, converted)
	if err != nil {
		return QueryResult{}, publicError(err)
	}
	return QueryResult{
		Columns: fromModelColumnSeriesList(columns, factor),
		Explain: fromModelQueryExplain(explain),
		Stats:   fromModelQueryStats(stats),
	}, nil
}

// QueryStatsSnapshot 返回最近一次查询统计快照。
func (e *Engine) QueryStatsSnapshot() QueryStats {
	return fromModelQueryStats(e.runtime.Storage().QueryStatsSnapshot())
}

// QueryRows 返回完整行式查询结果。
//
// 该方法适合小结果集。生产场景的大结果查询优先使用 QueryRowIterator 并配置
// Limit 或 QueryBudget。
func (e *Engine) QueryRows(ctx context.Context, query Query) ([]Row, error) {
	converted, factor, err := toModelQuery(query)
	if err != nil {
		return nil, err
	}
	rows, err := e.runtime.Storage().QueryRows(ctx, converted)
	if err != nil {
		return nil, publicError(err)
	}
	return fromModelRows(rows, factor), nil
}

// QueryRowIterator 以 iterator 方式返回行式查询结果。
func (e *Engine) QueryRowIterator(ctx context.Context, query Query) (RowIterator, error) {
	converted, factor, err := toModelQuery(query)
	if err != nil {
		return nil, err
	}
	inner, err := e.runtime.Storage().QueryRowIterator(ctx, converted)
	if err != nil {
		return nil, publicError(err)
	}
	return rowIterator{inner: inner, factor: factor}, nil
}

// Compact 对所有 shard 执行一次手动 compaction。
func (e *Engine) Compact(ctx context.Context) error {
	return publicError(e.runtime.Storage().Compact(ctx))
}

// CompactWithResult 对所有 shard 执行一次手动 compaction 并返回结果。
func (e *Engine) CompactWithResult(ctx context.Context) (CompactionResult, error) {
	result, err := e.runtime.Storage().CompactWithResult(ctx)
	return fromCompactionResult(result), publicError(err)
}

// ApplyRetention 按 now 应用本地 retention 清理。
func (e *Engine) ApplyRetention(ctx context.Context, now time.Time) error {
	return publicError(e.runtime.Storage().ApplyRetention(ctx, now))
}

// MaintenanceErrors 返回后台维护任务记录的错误。
func (e *Engine) MaintenanceErrors(ctx context.Context) []error {
	return e.runtime.Storage().MaintenanceErrors(ctx)
}

// MaintenanceStatsSnapshot 返回后台 compact/downsample 的并发、跳过与失败快照。
func (e *Engine) MaintenanceStatsSnapshot() MaintenanceStats {
	return fromMaintenanceStats(e.runtime.Storage().MaintenanceStatsSnapshot())
}

// StorageMemorySnapshot 返回存储层内存占用快照。
func (e *Engine) StorageMemorySnapshot() StorageMemorySnapshot {
	return fromStorageMemorySnapshot(e.runtime.Storage().StorageMemorySnapshot())
}

// CompactionStatsSnapshot 返回 compaction 统计快照。
func (e *Engine) CompactionStatsSnapshot() CompactionStats {
	return fromCompactionStats(e.runtime.Storage().CompactionStatsSnapshot())
}

// HealthSnapshot 返回 Engine 健康状态快照。
func (e *Engine) HealthSnapshot() HealthSnapshot {
	health := e.runtime.Storage().HealthSnapshot()
	return HealthSnapshot{
		Healthy: health.Healthy,
		Ready:   health.Ready,
		Reasons: append([]string(nil), health.Reasons...),
		Checks:  fromHealthChecks(health.Checks),
	}
}

// PrometheusMetrics 返回 Engine 指标的 Prometheus 文本格式。
func (e *Engine) PrometheusMetrics() string {
	return observability.PrometheusText(e.runtime.Storage().MetricsSnapshot())
}

func fromHealthChecks(checks []storageengine.HealthCheck) []HealthCheck {
	out := make([]HealthCheck, len(checks))
	for index, check := range checks {
		out[index] = HealthCheck{
			Name:   check.Name,
			Status: check.Status,
			Reason: check.Reason,
		}
	}
	return out
}

// CreateDatabase 创建本地 database 元数据。
func (e *Engine) CreateDatabase(ctx context.Context, name string) error {
	return publicError(e.runtime.Storage().CreateDatabase(ctx, name))
}

// ListDatabases 列出所有 database。
func (e *Engine) ListDatabases(ctx context.Context) ([]string, error) {
	databases, err := e.runtime.Storage().ListDatabases(ctx)
	return databases, publicError(err)
}

// DropDatabase 删除本地 database 元数据。
func (e *Engine) DropDatabase(ctx context.Context, name string) error {
	return publicError(e.runtime.Storage().DropDatabase(ctx, name))
}

// CreateRetentionPolicy 创建或更新本地 retention policy。
func (e *Engine) CreateRetentionPolicy(ctx context.Context, database string, policy RetentionPolicy) error {
	return publicError(e.runtime.Storage().CreateRetentionPolicy(ctx, database, toModelRetentionPolicy(policy)))
}

// ListRetentionPolicies 列出 database 下的 retention policy。
func (e *Engine) ListRetentionPolicies(ctx context.Context, database string) ([]RetentionPolicy, error) {
	policies, err := e.runtime.Storage().ListRetentionPolicies(ctx, database)
	if err != nil {
		return nil, publicError(err)
	}
	return fromModelRetentionPolicies(policies), nil
}

// ListMeasurements 列出 database 下的 measurement。
func (e *Engine) ListMeasurements(ctx context.Context, database string) ([]string, error) {
	measurements, err := e.runtime.Storage().ListMeasurements(ctx, database)
	return measurements, publicError(err)
}

// ListFields 列出 measurement 下的字段 schema。
func (e *Engine) ListFields(ctx context.Context, database string, measurement string) ([]FieldSchema, error) {
	fields, err := e.runtime.Storage().ListFields(ctx, database, measurement)
	if err != nil {
		return nil, publicError(err)
	}
	return fromModelFieldSchemas(fields), nil
}

// ListSeries 按 measurement 和 tag 过滤列出 series。
func (e *Engine) ListSeries(
	ctx context.Context,
	database string,
	measurement string,
	tags map[string]string,
) ([]Series, error) {
	series, err := e.runtime.Storage().ListSeries(ctx, database, measurement, tags)
	if err != nil {
		return nil, publicError(err)
	}
	return fromModelSeriesList(series), nil
}

// CreateUser 创建一个用户。
func (e *Engine) CreateUser(ctx context.Context, user User) error {
	return e.runtime.Users().CreateUser(ctx, toRuntimeUser(user))
}

// UpdateUser 更新用户显示名、禁用状态和 metadata。
func (e *Engine) UpdateUser(ctx context.Context, user User) error {
	return e.runtime.Users().UpdateUser(ctx, toRuntimeUser(user))
}

// GetUser 查询用户。
func (e *Engine) GetUser(ctx context.Context, name string) (User, bool, error) {
	user, ok, err := e.runtime.Users().GetUser(ctx, name)
	return fromRuntimeUser(user), ok, err
}

// ListUsers 按用户名排序列出用户。
func (e *Engine) ListUsers(ctx context.Context) ([]User, error) {
	users, err := e.runtime.Users().ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]User, len(users))
	for index, user := range users {
		out[index] = fromRuntimeUser(user)
	}
	return out, nil
}

// DeleteUser 删除用户及其 DB 权限。
func (e *Engine) DeleteUser(ctx context.Context, name string) error {
	return e.runtime.Users().DeleteUser(ctx, name)
}

// GrantDatabasePermission 授予用户 database 权限。
func (e *Engine) GrantDatabasePermission(
	ctx context.Context,
	userName string,
	database string,
	permission DatabasePermission,
) error {
	return e.runtime.Users().GrantDatabasePermission(ctx, userName, database, runtimePermission(permission))
}

// RevokeDatabasePermission 撤销用户 database 权限。
func (e *Engine) RevokeDatabasePermission(
	ctx context.Context,
	userName string,
	database string,
	permission DatabasePermission,
) error {
	return e.runtime.Users().RevokeDatabasePermission(ctx, userName, database, runtimePermission(permission))
}

// ListDatabasePermissions 列出用户 DB 权限。
func (e *Engine) ListDatabasePermissions(ctx context.Context, userName string) ([]DatabaseGrant, error) {
	grants, err := e.runtime.Users().ListDatabasePermissions(ctx, userName)
	if err != nil {
		return nil, err
	}
	out := make([]DatabaseGrant, len(grants))
	for index, grant := range grants {
		out[index] = fromRuntimeGrant(grant)
	}
	return out, nil
}

// CheckUserDatabasePermission 校验用户是否拥有 database 权限。
func (e *Engine) CheckUserDatabasePermission(
	ctx context.Context,
	userName string,
	database string,
	permission DatabasePermission,
) error {
	return e.runtime.Users().CheckDatabasePermission(ctx, userName, database, runtimePermission(permission))
}

func (e *Engine) SetPassword(ctx context.Context, userName string, password string) error {
	return e.runtime.Users().SetPassword(ctx, userName, password)
}

func (e *Engine) ChangePassword(
	ctx context.Context,
	userName string,
	oldPassword string,
	newPassword string,
) error {
	return e.runtime.Users().ChangePassword(ctx, userName, oldPassword, newPassword)
}

func (e *Engine) Authenticate(ctx context.Context, credentials Credentials, ttl time.Duration) (AuthToken, error) {
	token, err := e.runtime.Users().Authenticate(ctx, toRuntimeCredentials(credentials), ttl)
	if err != nil {
		return AuthToken{}, err
	}
	return AuthToken{Token: token.Token, UserName: token.UserName, ExpiresAt: token.ExpiresAt}, nil
}

func (e *Engine) VerifyToken(ctx context.Context, token string) (Principal, error) {
	principal, err := e.runtime.Users().VerifyToken(ctx, token)
	if err != nil {
		return Principal{}, err
	}
	return Principal{UserName: principal.UserName}, nil
}

func (e *Engine) RevokeToken(ctx context.Context, token string) error {
	return e.runtime.Users().RevokeToken(ctx, token)
}

type columnIterator struct {
	inner  model.ColumnIterator
	factor int64
}

type rowIterator struct {
	inner  model.RowIterator
	factor int64
}

func (i columnIterator) Next() bool {
	return i.inner.Next()
}

func (i columnIterator) Column() ColumnSeries {
	return fromModelColumnSeries(i.inner.Column(), i.factor)
}

func (i columnIterator) Err() error {
	return i.inner.Err()
}

func (i columnIterator) Close() error {
	return i.inner.Close()
}

func (i rowIterator) Next() bool {
	return i.inner.Next()
}

func (i rowIterator) Row() Row {
	return fromModelRow(i.inner.Row(), i.factor)
}

func (i rowIterator) Err() error {
	return i.inner.Err()
}

func (i rowIterator) Close() error {
	return i.inner.Close()
}
