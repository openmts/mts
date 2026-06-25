# mts-server Production Hardening Design

## 用户问题本质

用户希望 `mts-server` 不只是可运行的 API 示例程序，而是可以作为 MTS 单机服务端对外提供的启动程序；本轮范围覆盖传输安全、认证、API 契约、运行管理、观测、资源治理、配置管理、数据运维和权限增强，不包含发布制品/流水线。

## 范围

- 保持单机 Engine 与 LocalMetadataStore 边界。
- 保持 HTTP JSON 与现有 gRPC JSON codec，不引入 protobuf 生成链。
- 不实现 SQL、InfluxQL、PromQL 或 MetricsQL parser。
- 不接入外部 IAM、外部配置中心、分布式元数据或发布流水线。

## EARS 需求

### 1. 传输安全

- When 配置 `http.tls.enabled=true` 时，系统应使用配置的证书和私钥启动 HTTPS 监听。
- When 配置 `grpc.tls.enabled=true` 时，系统应使用配置的证书和私钥启动 TLS gRPC 监听。
- When 配置 `client_ca_file` 且 `client_auth=true` 时，系统应要求客户端证书通过 CA 校验。
- If TLS 已启用但证书、私钥或 CA 配置无效，则系统应在启动前失败并返回可诊断错误。
- While TLS 已启用，系统应使用 TLS 1.2 或更高版本。

### 2. 认证机制

- When 配置 `auth.data_tokens` 时，系统应要求数据面请求携带有效数据 token。
- When HTTP 请求携带 `Authorization: Bearer <token>` 或 `X-MTS-Data-Token` 时，系统应用常量时间比较校验数据 token。
- When gRPC 请求携带 `authorization` 或 `x-mts-data-token` metadata 时，系统应校验数据 token。
- When 配置 `auth.require_user=true` 时，系统应拒绝未携带用户身份的数据面请求。
- If 数据 token 无效或缺失，则系统应返回 HTTP 401 或 gRPC Unauthenticated。

### 3. API 契约

- When 用户访问 `/api/v1/admin/api-spec` 时，系统应返回当前 HTTP API 的机器可读契约。
- When 用户访问 `/api/v1/admin/error-codes` 时，系统应返回稳定错误码、HTTP 状态和说明。
- When gRPC 调用 `GetAPISpec` 或 `GetErrorCodes` 时，系统应返回同等契约信息。
- The system shall expose API version、命名空间、路径、方法、认证要求和描述。

### 4. 运行管理

- When 用户执行 `mts-server validate-config --config <path>` 时，系统应校验 YAML 配置并返回清晰结果。
- When 用户执行 `mts-server version` 时，系统应输出版本、提交和构建时间。
- When 用户执行 `mts-server doctor --config <path>` 时，系统应校验配置、数据目录可创建性和监听地址格式。
- When 用户执行 `mts-server init-config --output <path>` 时，系统应以 0600 权限写出默认 YAML 配置。
- If 输出配置文件已存在且未指定 `--force`，系统应拒绝覆盖。

### 5. 观测能力

- When HTTP 请求完成时，系统应记录结构化访问日志，字段包含 method、path、status、duration、request_id。
- When gRPC 请求完成时，系统应记录结构化访问日志，字段包含 method、code、duration、request_id。
- When 请求未携带 `X-Request-ID` 或 gRPC `x-request-id` 时，系统应生成请求 ID 并返回或透传。
- When 用户访问 `/metrics` 时，系统应暴露 HTTP/gRPC 请求总数、错误总数和耗时指标。
- When 配置 `observability.pprof.enabled=true` 时，系统应挂载 pprof 端点且要求 admin token。

### 6. 资源保护和服务治理

- When HTTP 请求体超过 `limits.max_request_body_bytes` 时，系统应拒绝请求并返回 413。
- When 写入点数超过 `limits.max_write_points` 时，系统应拒绝请求并返回 400。
- When 查询未设置 limit 且配置了 `limits.default_query_limit` 时，系统应注入默认 limit。
- When 查询 limit 超过 `limits.max_query_limit` 时，系统应拒绝请求。
- When 配置请求超时、读写超时或 idle timeout 时，系统应把配置应用到 HTTP server。
- When 配置 gRPC 最大收发消息大小时，系统应应用到 gRPC server。
- When 配置并发限制时，系统应在超限时返回 HTTP 429 或 gRPC ResourceExhausted。

### 7. 配置查询、校验和重载

- When 用户访问 `/api/v1/admin/config/validate` 时，系统应校验请求体中的配置并返回结果。
- When 用户访问 `/api/v1/admin/config/reload` 时，系统应从原配置文件重载允许热更新的字段。
- When 收到 SIGHUP 时，系统应执行同样的允许项重载。
- The system shall only hot reload auth、limits、observability 和 log level，不重载监听地址、TLS 或 engine 数据目录。
- If 热重载失败，则系统应保留原运行时配置。

### 8. 数据运维

- When 用户访问 `/api/v1/admin/storage/validate` 时，系统应检查数据目录和 Engine 健康状态。
- When 用户访问 `/api/v1/admin/storage/snapshot` 时，系统应在配置的备份目录生成本地 manifest 快照文件。
- When 用户访问 `/api/v1/admin/storage/export` 时，系统应导出当前服务端元信息、健康、用户和权限摘要。
- If 备份目录不存在，系统应以 0700 权限创建目录；快照文件应以 0600 权限创建。

### 9. 权限增强

- When 配置 `auth.require_user=true` 时，数据面读写必须同时具备有效数据 token 和用户 DB 权限。
- When 用户禁用时，系统应拒绝该用户的数据面访问。
- When 管理员访问 `/api/v1/users/{name}/audit` 时，系统应返回该用户的创建、更新、授权和撤销事件摘要。
- The system shall keep 权限粒度到 DB 级别，不扩展 measurement 级权限。

## 设计

新增横切能力尽量拆到 `cmd/mts-server` 下的小文件：TLS、auth、limits、observability、API contract、operations、CLI commands。`serverRuntime` 继续持有 Engine，同时增加可热更新的运行时配置快照、请求指标和审计日志。HTTP 通过 middleware 串接 request id、body limit、并发限制、access log 和 metrics；gRPC 通过 unary interceptor 处理同类逻辑。配置文件保留 YAML，新增字段都有默认值，默认仍保持本地开发兼容模式。

## 验证策略

- 单包测试覆盖配置校验、CLI 命令、TLS 配置构造、认证、HTTP/gRPC 限流、观测指标、API 契约、配置校验/重载、数据运维和用户审计。
- e2e 扩展覆盖至少一个 HTTP 和一个 gRPC 生产增强 smoke。
- 最终执行 `go test ./cmd/mts-server`、`go test ./tests/e2e/mts_server_protocols`、`make ci`、`govulncheck ./...`、`git diff --check` 和产物扫描。

## Self-review

- 无 TBD/TODO 占位。
- 范围与用户排除第 10 点一致。
- 不引入分布式能力、外部 IAM 或 parser。
- 每条 EARS 均可在本轮用配置、handler、interceptor 或 CLI 测试验证。
