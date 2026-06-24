# slog 结构化日志集成设计

## 背景

MTS 工程具备完善的 Prometheus 指标体系（114+ 指标），但生产代码中**零结构化日志**。存在 6 处静默吞掉的错误（`_ =` 模式），以及 HTTP Serve 异常退出无记录等盲点。指标提供计数聚合，但缺少事件上下文（哪个 shard、哪个 policy、具体错误详情），排障时无法快速定位根因。

本设计在关键位置集成 `log/slog`，与现有指标体系互补，形成"指标看趋势 + 日志看上下文"的双层可观测性。

## 范围

本次包含：

- 在公开 `Options` 和内部 `model.Options` 新增 `Logger *slog.Logger` 字段。
- 创建 nopHandler 作为 nil Logger 的默认值，零开销丢弃所有日志。
- 通过 `Engine` → `Shard` 链路传递 Logger。
- 在 `internal/service.Options` 新增 `Logger` 字段，HTTP 服务独立注入。
- 在 6 处静默吞错位置补充 WARN 日志。
- 在生命周期事件（Open/Close/Compaction/Retention/Downsample/Service 启停）补充 INFO 日志。
- 在持久化失败（manifest 写入、WAL sync）补充 ERROR 日志。
- 在 `cmd/mts-storage` 设置 stderr TextHandler。
- 为每个日志点编写测试用例。

本次不包含：

- 高频读写路径（Write/Query/memtable/SSTable 读取）加日志——依赖现有指标。
- 分布式追踪（OpenTelemetry）——超出单机项目边界。
- 日志轮转、文件 handler——由调用方或基础设施层负责。
- 改变现有指标体系或审计机制。

## 设计决策

### Logger 注入方式

采用 Options 注入（方案 A），遵循现有 `Options` + `Open` API 模式。

- 公开 `Options.Logger` 为 `*slog.Logger`，零值表示使用 discard logger。
- `normalizeOptions` 在 `Logger == nil` 时填充 nopHandler logger，保证下游永不需 nil 检查。
- `Engine` struct 持有 `logger *slog.Logger`，传递到 `ShardOptions.Logger`。
- `Shard` struct 持有 `logger *slog.Logger`，用于 shard 级日志。
- `internal/service.Options.Logger` 独立注入，与引擎解耦。

### nopHandler

在 `internal/engine` 新建 `logging.go`，定义 `nopHandler` 实现 `slog.Handler` 接口，`Enabled` 恒返回 `false`，`Handle` 为空操作。这是 nil Logger 的归一化目标，性能等价于无日志。

### 日志级别策略

| 级别 | 适用场景 | 示例 |
|------|----------|------|
| ERROR | 数据持久化失败、不可恢复的内部错误 | manifest 写入失败、WAL sync 失败、Serve 异常退出 |
| WARN | 被吞的后台错误、恢复问题、资源不足、鉴权失败 | 后台 compaction 失败、降采样失败、磁盘空间不足、WAL 定时 sync 失败 |
| INFO | 生命周期事件、手动操作完成 | 引擎 Open/Close、Compaction 完成、Retention 删除、Downsample 运行、Service 启停 |
| DEBUG | 跳过原因、flush 触发 | compaction skip memory_busy、segment roll |

### 日志位置清单（约 40 个日志点）

#### P0 数据安全（6 处静默吞错 + 持久化失败）

| 文件 | 函数 | 级别 | 消息 |
|------|------|------|------|
| `internal/engine/background.go:34` | `backgroundCompactionLoop` | WARN | `background compaction failed` |
| `internal/engine/downsample_scheduler.go:50` | `scanDownsamplePolicies` | WARN | `list downsample policies failed` |
| `internal/engine/downsample_scheduler.go:66` | `shouldRunDownsamplePolicy` | WARN | `read downsample watermark failed` |
| `internal/engine/downsample_scheduler.go:86` | `startDownsamplePolicyRun` | WARN | `downsample policy run failed` |
| `internal/wal/wal.go:436` | `intervalSyncLoop` | WARN | `wal interval sync failed` |
| `internal/service/server.go:67` | `Start` goroutine | ERROR | `http server stopped unexpectedly` |

#### P0 持久化关键失败

| 文件 | 函数 | 级别 | 消息 |
|------|------|------|------|
| `internal/sstable/manifest.go` | `WriteManifest` | ERROR | `manifest write failed` |
| `internal/engine/shard.go:91` | `OpenShard` WAL replay | WARN | `wal replay failed` |

#### P1 生命周期事件

| 文件 | 函数 | 级别 | 消息 |
|------|------|------|------|
| `internal/engine/engine.go:66` | `Open` | INFO | `engine opened` (path, shard_count) |
| `internal/engine/engine.go:119` | `Close` | INFO | `engine closed` |
| `internal/engine/lifecycle.go` | `CompactWithResult` | INFO/WARN | `compaction completed/failed` (shard, duration) |
| `internal/engine/lifecycle.go` | `ApplyRetention` | INFO | `retention applied` (deleted_shards, bytes) |
| `internal/engine/downsample_executor.go` | `RunDownsamplePolicy` | INFO/WARN | `downsample policy run completed/failed` |
| `internal/engine/background.go:8` | `startBackgroundCompaction` | INFO | `background compaction started` (interval) |
| `internal/engine/downsample_scheduler.go:12` | `startDownsampleScheduler` | INFO | `downsample scheduler started` |
| `internal/service/server.go:60` | `Start` | INFO | `service listening` (addr) |
| `internal/service/server.go:74` | `Shutdown` | INFO | `service shutdown` |
| `internal/engine/shard.go:58` | `OpenShard` | INFO | `shard opened` (id, wal_records) |

#### P2 运维排障

| 文件 | 函数 | 级别 | 消息 |
|------|------|------|------|
| `internal/engine/lifecycle.go` | `preflightCompactionDiskSpace` | WARN | `compaction skipped: insufficient disk space` |
| `internal/engine/recovery_audit.go` | `partOpenRecoveryIssue` | WARN | `part open recovery issue` |
| `internal/engine/recovery_audit.go` | `partMetadataMismatchIssue` | WARN | `part metadata mismatch` |
| `internal/service/admin.go` | `authorizedAdminRequest` | WARN | `admin auth failed` |

### 结构化属性约定

| 属性键 | 类型 | 说明 |
|--------|------|------|
| `path` | string | 引擎数据目录 |
| `shard` | string | shard ID |
| `database` | string | 数据库名 |
| `policy` | string | 保留策略名 |
| `part` | string | part ID |
| `operation` | string | 操作名 |
| `duration_ms` | int64 | 耗时毫秒 |
| `error` | string | 错误消息（slog 自动通过 Error() 获取） |
| `shard_count` | int | shard 数量 |
| `deleted_shards` | int | 删除的 shard 数 |
| `bytes` | int64 | 字节数 |
| `addr` | string | 监听地址 |

### 不加日志的位置（防膨胀）

- `memtable` 包全部——纯内存操作，无实际错误。
- `sstable/read.go` 的 `Query`/`readValuePage` 等读取路径——高频，用 query stats。
- `engine/query.go` 的查询内部循环——高频，用指标。
- `queryservice` 的 CAS 循环（`tryAcquire`/`tryQueue`）——高频热循环。
- `wal.go` 的 `appendFrameLocked` 正常路径——高频，用 `mts_wal_append_*` 指标。
- `observability/metrics.go` 的 `record*Metrics`——高频指标记录。
- `catalog` 的 `ResolvePoint`/`ResolvePoints` 正常路径——高频写入路径。
- `queryexec/*` 算子内部——流式执行高频，用 query stats。

## EARS 清单

- When 调用方不设置 `Options.Logger` 时，系统应使用 nopHandler 丢弃所有日志，不产生 I/O 开销。
- When 调用方设置 `Options.Logger` 为自定义 logger 时，系统应将该 logger 传递到 Engine、Shard 和 HTTP Service。
- When 后台 compaction 失败时，系统应记录 WARN 级别日志，包含 shard 和 error 属性。
- When 后台降采样策略列表失败时，系统应记录 WARN 级别日志，包含 error 属性。
- When WAL 定时 sync 失败时，系统应记录 WARN 级别日志，包含 error 属性。
- When HTTP Serve 异常退出时，系统应记录 ERROR 级别日志，包含 error 属性。
- When manifest 写入失败时，系统应记录 ERROR 级别日志，包含 shard 和 error 属性。
- When 引擎 Open 完成时，系统应记录 INFO 级别日志，包含 path 和 shard_count 属性。
- When 引擎 Close 完成时，系统应记录 INFO 级别日志。
- When compaction 完成时，系统应记录 INFO 级别日志，包含 shard、duration_ms 属性。
- When retention 执行时，系统应记录 INFO 级别日志，包含 deleted_shards、bytes 属性。
- When 降采样策略运行完成时，系统应记录 INFO 级别日志，包含 policy 属性。
- When HTTP 服务启动时，系统应记录 INFO 级别日志，包含 addr 属性。
- When HTTP 服务关闭时，系统应记录 INFO 级别日志。
- When shard 打开完成时，系统应记录 INFO 级别日志，包含 shard 和 wal replay 记录数属性。
- When 磁盘空间不足导致 compaction 跳过时，系统应记录 WARN 级别日志。
- When part 打开出现恢复问题时，系统应记录 WARN 级别日志，包含 shard、part 属性。
- When admin 鉴权失败时，系统应记录 WARN 级别日志。
- When 高频读写路径执行时，系统不应记录日志，应依赖现有指标体系。
- When nopHandler 作为默认 logger 时，系统应零开销丢弃所有日志请求。

## 测试策略

- **nopHandler 测试**：验证 `Enabled` 返回 false、`Handle` 不产生输出。
- **Logger 传播测试**：验证 nil Logger 被归一化为 nopHandler，自定义 Logger 被正确传递到 Engine/Shard。
- **日志输出测试**：使用 `slog.NewTextHandler(&bytes.Buffer{}, nil)` 捕获日志，断言级别、消息和属性。
- **静默吞错修复测试**：触发后台 compaction 失败、降采样失败、WAL sync 失败，验证 WARN 日志输出。
- **生命周期日志测试**：Open/Close/Compaction/Retention/Downsample，验证 INFO 日志输出。
- **CLI 测试**：验证 `cmd/mts-storage` 设置 stderr handler 并输出日志。
- **覆盖率**：新增代码行覆盖率 >= 90%。
