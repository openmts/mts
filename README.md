# MTS

MTS 是一个单机嵌入式时序存储库，面向 Go 应用内的本地时序数据写入、查询、元数据管理、compaction、retention 和降采样场景。

当前项目只承诺单机本地目录能力：不做分布式查询、分布式存储、外部元数据系统，也不提供 SQL、InfluxQL、PromQL 或 MetricsQL parser。查询入口以 Go Builder/API 为主。

当前稳定对外接口包括 Go library 根包 `github.com/openmts/mts`、本地维护 CLI `cmd/mts-storage` 和服务进程 `cmd/mts-server`。`internal/queryservice` 和 `internal/service` 中的内部 HTTP 实现不作为当前外部稳定 API。

稳定性、磁盘格式与 experimental 边界见 `docs/compatibility.md`。生产查询保护、删除回收与 Nightly 门禁见 `docs/ops/`。POC 存储路径使用列式 WAL（formatID=2）、SSTable metadata 内嵌组件 size、顺序数值默认专用编码，以及跨 shard 有界并行写/compact。

## 安装

```bash
go get github.com/openmts/mts
```

## 快速开始

```go
ctx := context.Background()
engine, err := mts.Open(ctx, mts.DefaultOptions("/var/lib/mts"))
if err != nil {
	return err
}
defer func() {
	_ = engine.Close(ctx)
}()

err = engine.Write(ctx, []mts.Point{{
	Measurement: "cpu",
	Tags:        map[string]string{"host": "a"},
	Timestamp:   time.Now().UnixNano(),
	Fields: map[string]mts.FieldValue{
		"usage": mts.Float64Value(0.42),
	},
}}, mts.WriteOptions{Sync: true})
if err != nil {
	return err
}

query, err := mts.NewQuery().
	From("", "", "cpu").
	Where(mts.TagEq("host", "a")).
	TimeRangeTime(time.Unix(0, 0), time.Now()).
	Limit(100).
	Build()
if err != nil {
	return err
}

rows, err := engine.QueryRows(ctx, query)
if err != nil {
	return err
}
_ = rows
```

`QueryRows` 和 `QueryColumns` 适合小结果集。生产路径建议优先使用 `QueryRowIterator` 或 `QueryColumnIterator`，并配置 `Limit` 或 `QueryBudget`。

MTS 内部统一按纳秒存储时间。公开 API 中 `Point.Precision`、`TypedBatch.Precision` 和 `Query.Precision` 可声明输入/返回时间戳单位，零值保持纳秒兼容：

```go
err := engine.Write(ctx, []mts.Point{{
	Measurement: "cpu",
	Timestamp:   time.Now().UnixMilli(),
	Precision:   mts.PrecisionMillisecond,
	Fields:      map[string]mts.FieldValue{"usage": mts.Float64Value(0.42)},
}}, mts.WriteOptions{})

query, err := mts.NewQuery().
	From("", "", "cpu").
	Precision(mts.PrecisionMillisecond).
	TimeRange(startMillis, endMillis).
	Build()
```

## Builder 查询

```go
query, err := mts.NewQuery().
	Select("usage").
	From("metrics", "autogen", "cpu").
	Where(
		mts.TagEq("host", "a"),
		mts.FieldGT("usage", mts.Float64Value(0.8)),
	).
	Aggregate(mts.AggregateMean, "usage").
	GroupByTime(time.Minute).
	OrderByTimeDesc().
	Limit(100).
	Build()
```

聚合函数支持矩阵见 [docs/query/builder-aggregate-functions.md](docs/query/builder-aggregate-functions.md)。

## 用户与权限

MTS 默认使用内部本地用户管理实现。用户角色分为 `admin` 和 `user`，系统级管理操作需要管理员角色；database 权限支持 `read`、`write`、`admin`，其中 database `admin` 隐含读写权限。默认本地实现支持密码认证、token 签发和密码变更；`mts-server` 创建用户 API 可携带初始密码，但用户查询结果永不返回密码。具体实现和落盘格式不作为 public API 暴露。

```go
err := engine.CreateUser(ctx, mts.User{Name: "alice", DisplayName: "Alice"})
if err != nil {
	return err
}

err = engine.GrantDatabasePermission(ctx, "alice", "metrics", mts.DatabasePermissionRead)
if err != nil {
	return err
}

err = engine.CheckUserDatabasePermission(ctx, "alice", "metrics", mts.DatabasePermissionRead)
```

如需接入第三方权限系统，应在 MTS 仓库内为用户模块新增 provider，并通过内部 runtime 组合层接入。

## 批量写入

高吞吐写入优先使用 `WriteTypedBatch`，避免每个点都构造字段 map：

```go
batch := mts.TypedBatch{
	Measurement: "cpu",
	Precision:   mts.PrecisionSecond,
	Tags: []mts.TagColumn{
		{Name: "host", Values: []string{"a", "a", "b"}},
	},
	Timestamps: []int64{10, 20, 30},
	Fields: []mts.TypedFieldColumn{
		{Name: "usage", Type: mts.FieldFloat64, Float64Values: []float64{0.1, 0.2, 0.3}},
	},
}

err := engine.WriteTypedBatch(ctx, batch, mts.WriteOptions{Sync: true})
```

## 降采样

MTS 支持本地降采样策略，结果写入显式目标 retention policy。查询降采样结果时使用 `FromDownsamplePolicy`：

```go
query, err := mts.NewQuery().
	FromDownsamplePolicy(policy).
	Select("avg_usage").
	TimeRange(start, end).
	Build()
```

详细运维说明见 [docs/storage/downsample-runbook.md](docs/storage/downsample-runbook.md)。

## 本地维护工具

`cmd/mts-storage` 提供本地目录检查、修复、迁移、快照和恢复：

```bash
go install github.com/openmts/mts/cmd/mts-storage@latest

go run ./cmd/mts-storage check /var/lib/mts
go run ./cmd/mts-storage repair --dry-run /var/lib/mts
go run ./cmd/mts-storage snapshot /var/lib/mts /backup/mts-snapshot
go run ./cmd/mts-storage restore /backup/mts-snapshot /var/lib/mts-restored
```

## 服务进程

`cmd/mts-server` 可按 YAML 配置文件启动单机 MTS Engine，并同时提供 HTTP 与 gRPC 结构化 API。默认示例配置在 [configs/mts-server.yaml](configs/mts-server.yaml)，文件内包含完整配置说明和注释：

```bash
go run ./cmd/mts-server serve --config configs/mts-server.yaml
go run ./cmd/mts-server validate-config --config configs/mts-server.yaml
go run ./cmd/mts-server doctor --config configs/mts-server.yaml
go run ./cmd/mts-server init-config --output ./mts-server.yaml
go run ./cmd/mts-server version
```

HTTP 默认监听 `127.0.0.1:8086`，gRPC 默认监听 `127.0.0.1:9096`。HTTP API 按数据面、管理面和用户权限面拆分：数据面使用 `/api/v1/data/*`，管理面使用 `/api/v1/admin/*`，用户权限面使用 `/api/v1/users/*` 和 `/api/v1/authz/*`。配置 `auth.admin_token` 后，管理面和用户权限面需要服务级 admin token；开启 `auth.require_user` 后，服务会预置管理员账号 `admin/admin`，可登录后使用管理员 Bearer token 访问管理接口，普通用户只能访问已授权的数据面并修改自己的密码。首次登录后应修改默认管理员密码。配置 `auth.data_tokens` 后，数据面需要 `X-MTS-Data-Token` 或 Bearer token。生产部署可开启 HTTP/gRPC TLS、请求限制、访问日志、pprof、API 契约、配置校验/热重载和本地 storage snapshot/export。

```bash
curl http://127.0.0.1:8086/healthz
curl -X POST http://127.0.0.1:8086/api/v1/data/write -d @points.json
curl -X POST http://127.0.0.1:8086/api/v1/data/write/typed -d @typed-batch.json
curl -X POST http://127.0.0.1:8086/api/v1/data/query/rows -d @query.json
curl -X POST http://127.0.0.1:8086/api/v1/data/query/columns -d @query.json
curl -X POST http://127.0.0.1:8086/api/v1/data/query/explain -d @query.json
curl -X POST http://127.0.0.1:8086/api/v1/admin/flush
curl -X POST http://127.0.0.1:8086/api/v1/admin/compact
curl http://127.0.0.1:8086/api/v1/admin/api-spec
curl -X POST http://127.0.0.1:8086/api/v1/admin/storage/validate
curl http://127.0.0.1:8086/metrics
```

服务入口不提供 SQL、InfluxQL、PromQL 或 MetricsQL parser；查询请求使用与 Go API 一致的结构化 `Query` JSON。

启动后浏览器访问 `http://127.0.0.1:8086/` 可直接打开内置管理页面（Dashboard），支持仪表盘概览、数据库管理、用户权限管理、配置热重载、运维操作、降采样管理、数据查询、审计日志和存储快照导出。构建时自动嵌入前端产物：

```bash
make dashboard           # 构建前端并生成嵌入产物
go build ./cmd/mts-server  # 编译含 Dashboard 的二进制
```

## 结构化日志

`Options.Logger` 接收 `*slog.Logger`，nil 时使用零开销 nopHandler。日志仅在生命周期事件（Open/Close/Compaction/Retention/Downsample）和错误路径记录，高频读写路径依赖指标体系：

```go
opts := mts.DefaultOptions("/var/lib/mts")
opts.Logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
engine, err := mts.Open(ctx, opts)
```

`Options.WAL.Logger` 可单独指定 WAL 日志器，nil 时继承 `Options.Logger`。

## 质量门禁

常规改动完成后执行：

```bash
make fmt
make test
make lint
make coverage
make e2e
```

常用场景可直接执行 `make e2e-public-api`、`make fault-matrix`、`make storage-100k`、`make bench-query`、`make pprof-storage`、`make mts-server-test`；完整商用门禁执行 `make ci`。

商用交付门禁要求生产包代码行覆盖率达到 90% 以上；`tests/**` 下的 e2e、fault、scale 和 pprof harness 作为行为验证单独执行。
