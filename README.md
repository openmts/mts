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

## 存储压缩

SSTable part 最终布局为 `metadata.bin` + `pack.bin`（逻辑组件 section）。


高密度写入可打开 SSTable payload 压缩，并放大 value page 以提升压缩窗口：

```go
opts := mts.DefaultOptions(dir)
opts.Compression = mts.CompressionOptions{
    Enabled:          true,
    Algorithm:        "zstd", // 全局显式算法；留空时 L0=snappy、L1+=zstd
    ZstdLevel:        "default", // fastest|default|better|best
    ValuePageSamples: 4096,  // 默认 1024；更大通常更省盘
    // Float 默认 xor=Gorilla 位打包；Timestamp 自动 const-step；Int/writeSeq 自动 RLE
    // OmitWriteSeq: true, // 不需要写序时可进一步省空间
    MinPageValues:    1,
}
```

## 批量写入

**性能建议（按优先顺序）：**

1. **首选** `WriteTypedBatch`：调用方直接构造列式 `TypedBatch`，宽字段/高吞吐场景收益最大。
2. **次选** `WritePointsAsTypedBatch` / `PointsToTypedBatch`：已有同构 `[]Point` 时，转列式后再写。
3. **兼容** `Write([]Point)`：便于迁移与小批量，但 Tags/Fields map 开销更高，不适合持续高吞吐主路径。

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

若调用侧仍是 `[]Point`，可用包级转换函数走列式路径：

```go
batch, err := mts.PointsToTypedBatch(points)
if err != nil {
	// 异构 batch：measurement/字段集合/类型不一致
	return err
}
err = engine.WriteTypedBatch(ctx, batch, mts.WriteOptions{Sync: true})
// 或一步完成：
// err = engine.WritePointsAsTypedBatch(ctx, points, mts.WriteOptions{Sync: true})
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
go run ./cmd/mts-server serve            # 不带 --config 时自动生成并使用 ~/.mts/mts-server.yaml
go run ./cmd/mts-server validate-config --config configs/mts-server.yaml
go run ./cmd/mts-server doctor --config configs/mts-server.yaml
go run ./cmd/mts-server init-config --output ./mts-server.yaml
go run ./cmd/mts-server version
```

HTTP 默认监听 `127.0.0.1:8086`，gRPC 默认监听 `127.0.0.1:9096`。HTTP API 按数据面、管理面和用户权限面拆分：数据面使用 `/api/v1/data/*`，管理面使用 `/api/v1/admin/*`，用户权限面使用 `/api/v1/users/*` 和 `/api/v1/authz/*`。管理员可通过 `GET /api/v1/users/access-grants?limit=100&cursor=<username>` 分页读取用户与 database 授权快照，`limit` 范围为 1 到 200。服务不会创建默认管理员；首次部署应先配置强随机 `auth.admin_token`，再通过受保护的用户 API 创建管理员账号和初始密码。开启 `auth.require_user` 后，管理员可使用登录后的 Bearer token 访问管理接口，普通用户只能访问已授权的数据面并修改自己的密码。配置 `auth.data_tokens` 后，数据面需要 `X-MTS-Data-Token` 或 Bearer token。生产部署可开启 HTTP/gRPC TLS、请求限制、访问日志、pprof、API 契约、配置校验/热重载和本地 storage snapshot/export。

```bash
curl http://127.0.0.1:8086/healthz
curl -X POST http://127.0.0.1:8086/api/v1/data/write -d @points.json
curl -X POST http://127.0.0.1:8086/api/v1/data/write/typed -d @typed-batch.json
curl -X POST http://127.0.0.1:8086/api/v1/data/write/points-typed -d @points.json
curl -X POST http://127.0.0.1:8086/api/v1/data/query/rows -d @query.json
curl -X POST http://127.0.0.1:8086/api/v1/data/query/columns -d @query.json
curl -X POST http://127.0.0.1:8086/api/v1/data/query/stream -d '{"query":{...},"format":"column"}'
# gRPC 服务端流：QueryStream（row/column），消息类型同 NDJSON streamRecord
curl -X POST http://127.0.0.1:8086/api/v1/data/query/explain -d @query.json
curl -X POST http://127.0.0.1:8086/api/v1/data/delete -d @delete.json
curl -X POST http://127.0.0.1:8086/api/v1/admin/flush
curl http://127.0.0.1:8086/api/v1/admin/stats/maintenance
# gRPC 管理面补充：ListDatabases / AdminHealth；DropDownsamplePolicy 支持 cleanup options
# engine 配置可透出 query_*_cache、compression.omit_write_seq/value_page_samples、storage_memory 等
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
# 子路径部署：前端 VITE_BASE=/mts/ make dashboard，服务端 http.dashboard_base: /mts/
```

容器化部署直接拉取 GHCR 镜像（已内嵌 Dashboard），Docker 运行方式见 [docs/ops/docker-runbook.md](docs/ops/docker-runbook.md)。

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
make dashboard-gate
make e2e
```

常用场景可直接执行 `make e2e-public-api`、`make fault-matrix`、`make storage-100k`、`make bench-query`、`make pprof-storage`、`make mts-server-test`；完整商用门禁执行 `make ci`。

商用交付门禁要求每个含生产语句的 Go 包代码行覆盖率达到 90% 以上。Dashboard 门禁包含 lint、单测覆盖率、安全 DOM sink 扫描和高危依赖审计，覆盖率阈值为 lines/functions 90%、branches 70%；`tests/**` 下的 e2e、fault、scale 和 pprof harness 作为行为验证单独执行。

## GitHub CI

`.github/workflows/` 提供三个工作流：

- `ci.yml`：push 触发，执行格式化检查、`make unit`、覆盖率门禁（>=90%）、`golangci-lint`、Go 漏洞检查（`govulncheck`）与前端 `npm audit`，任一失败即阻塞。
- `pre-release.yml`：main 分支 push 后构建 mts-server 跨平台二进制（linux/darwin/windows × amd64/arm64）并发布 `dev` GitHub Pre-release，同时构建并推送 `ghcr.io/<owner>/mts-server:dev` 镜像（linux/amd64、linux/arm64）。
- `release.yml`：推送 `v*` tag 时构建并发布 mts-server 正式版本，生成校验和并创建 GitHub Release，同时构建并推送 `ghcr.io/<owner>/mts-server:<tag>` 镜像。

所有构建先 `npm run build` 构建并嵌入 mts-dashboard，再以 `CGO_ENABLED=0` 静态编译，版本信息通过 `-ldflags -X main.version/commit/builtAt` 注入。镜像基于 `deploy/docker/mts-server.yaml`（监听 `0.0.0.0`、数据目录 `/data`），Dockerfile 见仓库根目录。

镜像推送使用 `GITHUB_TOKEN` 认证 GHCR，workflow 中声明 `packages: write` 权限，无需额外配置凭据；首次推送后如需公开拉取，请在仓库 Packages 页面将镜像可见性设为 public。
