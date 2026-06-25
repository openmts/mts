# mts-server P0/P1 API 补齐设计

## 用户问题本质

当前 `mts-server` 只暴露少量验证接口，无法覆盖项目已经具备的用户系统、数据查询/写入、配置查询、元数据、运维观测和降采样能力；需要把这些能力补齐为外部用户可访问的 HTTP 与 gRPC API。

## EARS 需求

### 数据面

- When 外部用户提交点写入请求时，系统应通过 `/api/v1/data/write` 和 gRPC `Write` 写入 `Point` 数据，并遵守请求中的 `precision`。
- When 外部用户提交 typed batch 写入请求时，系统应通过 `/api/v1/data/write/typed` 和 gRPC `WriteTypedBatch` 写入 `TypedBatch` 数据。
- When 外部用户查询行式数据时，系统应通过 `/api/v1/data/query/rows` 和 gRPC `QueryRows` 返回 `Row` 列表。
- When 外部用户查询列式数据时，系统应通过 `/api/v1/data/query/columns` 和 gRPC `QueryColumns` 返回 `ColumnSeries` 列表。
- When 外部用户请求查询说明时，系统应通过 `/api/v1/data/query/explain` 和 gRPC `QueryWithExplain` 返回查询结果、explain 和 stats。
- When 外部用户请求流式查询时，系统应通过 `/api/v1/data/query/stream` 返回 NDJSON 记录，并通过 gRPC `QueryRowsStream` 逐行发送结果。
- When 外部用户查询查询统计时，系统应通过 `/api/v1/data/query/stats` 和 gRPC `QueryStats` 返回最近一次查询统计。

### 用户权限面

- When 管理员创建用户时，系统应通过 `/api/v1/users` 和 gRPC `CreateUser` 创建本地用户。
- When 管理员查询用户时，系统应支持用户列表、单用户查询、更新、删除的 HTTP 与 gRPC 接口。
- When 管理员管理 DB 权限时，系统应支持授予、撤销、列出和检查用户 DB 级权限。
- If 用户被禁用或缺少目标 database 权限，系统应拒绝对应数据面请求。

### 元数据面

- When 管理员管理 database 和 retention policy 时，系统应提供创建、删除、创建/更新 retention policy、列出 retention policy 的 HTTP 与 gRPC 接口。
- When 数据读取用户发现数据目录时，系统应提供 measurement、field、series 列表接口。

### 配置面

- When 管理员查询配置时，系统应返回原始启动配置、生效配置和配置 schema。
- While 首批 API 补齐阶段，系统不应支持运行时修改配置。

### 运维观测面

- When 管理员触发维护时，系统应提供 flush、compact、retention apply 接口。
- When 管理员查询运行状态时，系统应提供 health、maintenance errors、storage memory、compaction stats 和 Prometheus metrics。

### 降采样面

- When 管理员管理降采样策略时，系统应提供策略创建/更新、列表、启用、禁用、删除、重置、状态查询接口。
- When 管理员手动运行降采样时，系统应提供 run、run-range、repair、dry-run 接口。

### 横切需求

- When 请求进入管理面时，系统应在配置了 admin token 时校验 token；未配置 token 时保持本地开发兼容模式。
- When 请求携带用户身份时，系统应根据 DB 级权限校验数据面读写和元数据发现权限。
- If 请求格式、权限、资源状态或内部执行失败，系统应返回统一错误响应和合适的 HTTP/gRPC 状态码。
- The system shall keep current single-node、LocalMetadataStore、Builder/API 查询边界，不引入分布式能力、外部元数据系统或 SQL parser。

## 架构设计

保持 `cmd/mts-server` 作为服务入口，继续复用当前手写 JSON HTTP 和 gRPC JSON codec，避免本轮引入 protobuf 生成链导致范围失控。代码按责任拆分：

- `http.go` 只负责路由注册和通用 HTTP 协议辅助。
- `http_data.go`、`http_admin.go`、`http_users.go` 分别承载数据面、管理面和用户面 handler。
- `runtime.go` 承载调用根包 `Engine` 的业务适配方法。
- `protocol_types.go` 定义请求/响应 DTO。
- `auth.go` 定义 admin token 和用户 DB 权限校验。
- `grpc.go` 注册所有 P0/P1 unary RPC 和一个行式 streaming RPC。

## API 命名空间

- 数据面：`/api/v1/data/*`
- 管理面：`/api/v1/admin/*`
- 用户权限面：`/api/v1/users/*`、`/api/v1/authz/*`
- 标准观测：`/healthz`、`/readyz`、`/metrics`

## 安全设计

配置增加 `auth.admin_token`。当 token 非空时，管理面和用户面 HTTP 请求必须携带 `Authorization: Bearer <token>` 或 `X-MTS-Admin-Token: <token>`；gRPC 请求通过 metadata 中的 `authorization` 或 `x-mts-admin-token` 传入。token 比较使用 constant-time compare。

数据面支持可选用户身份：HTTP 使用 `X-MTS-User`，gRPC 使用 metadata `x-mts-user`。如果未传用户身份，保持兼容不做 DB 权限校验；如果传入用户身份，则 read/write 请求按 database 权限校验。

## 错误模型

HTTP 错误响应统一为：

```json
{"ok":false,"code":"bad_request","message":"..."}
```

gRPC 使用 `codes.InvalidArgument`、`NotFound`、`AlreadyExists`、`PermissionDenied`、`Unauthenticated`、`Internal` 等标准状态码。

## 非目标

- 不实现 SQL、InfluxQL、PromQL 或 MetricsQL parser。
- 不实现密码登录、token 签发、session、OAuth、OIDC。
- 不实现运行时配置修改。
- 不默认开放 storage repair、restore、migrate 远程 API。
- 不把本轮重构为 protobuf 代码生成体系；后续可单独补 proto/OpenAPI 契约。

## 验收标准

- `cmd/mts-server` HTTP 测试覆盖 P0/P1 主要接口成功路径、方法错误、权限错误和业务错误。
- `cmd/mts-server` gRPC 测试覆盖 P0/P1 主要 RPC 和状态码。
- `tests/e2e/mts_server_protocols` 覆盖 HTTP/gRPC 的写入、查询、用户、元数据、配置、运维、降采样 smoke。
- `make ci`、`govulncheck ./...`、`git diff --check` 通过。
