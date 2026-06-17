# 现代 TSDB 存储能力补齐设计

## 背景

当前 mts 存储层已经具备单机嵌入式时序存储内核主链路：WAL、MemTable、SSTable、Manifest、Recovery、Retention、压缩、streaming compaction 和 level compaction。要在其上构建现代化时序数据库，还需要补齐查询执行、读放大控制、compaction 调度观测、长期压测、故障矩阵、生产级 metrics 和服务化运维能力。

## 开源参考

- Prometheus TSDB 将 Querier、ChunkQuerier、Head、Block、Compactor、Retention 和 metrics 作为核心边界。它的启发是：查询必须按时间范围拼接 head 与 blocks，并通过 merge querier 统一结果；compaction、reload、retention 都要有独立指标。
- InfluxDB TSM 将 cursor/iterator、WAL、Cache、FileStore、CompactionPlanner、scheduler 和 EngineStatistics 放在存储引擎内。它的启发是：compaction 必须可暂停、可统计 active/error/duration/queue，并且查询通过 typed cursor 分层执行。
- VictoriaMetrics storage 对缓存、索引搜索、deadline、query tracer、storage metrics、part merge 和长期运行状态做了工程化封装。它的启发是：高基数与长期压测场景下，deadline、cache、metrics 和后台维护必须成为一等能力。

## 目标

1. 建设真正的流式查询执行器，避免大查询一次性物化所有列和行。
2. 支持聚合、窗口、分页和 context 取消下推。
3. 建设严格读放大控制，包括 Part/Block/Page 级预算、level score 和查询统计。
4. 建设 compaction 调度观测，包括 backlog、score、active、duration、error 和 bytes 指标。
5. 建设 10M+ 数据规模长期压测与性能回归门禁。
6. 建设异常掉电、磁盘满、fsync 失败、manifest 失败、compaction 中断等故障矩阵。
7. 建设生产级 metrics、pprof、健康检查和服务化运维接口。

## 非目标

- 不在本阶段实现分布式集群、副本、Raft 或对象存储。
- 不在本阶段实现完整 SQL/Flux/PromQL 语言。
- 不改变现有 SSTable 二进制主体格式，除非读放大控制确实需要新增可兼容的元数据索引。

## EARS 需求

### 流式查询执行器

- When 客户端调用列式查询迭代器时，系统应按 shard、part、series、field 顺序流式返回列数据，而不是先物化完整 `[]ColumnData`。
- When 客户端调用行式查询迭代器时，系统应按 `(seriesID,timestamp)` 流式合并字段并返回 row，而不是先构造完整 `[]Row`。
- When 查询跨多个 shard 时，系统应只打开与时间范围相交的 shard reader，并在 reader 关闭时释放底层 part 引用。
- When 查询过程中任意底层 reader 返回错误时，系统应停止迭代并通过 `Err()` 暴露该错误。
- When iterator `Close()` 被调用时，系统应释放 MemTable snapshot、Part reader、page buffer 和聚合状态。

### 聚合、窗口、分页、取消下推

- When 查询请求包含聚合函数 `count/sum/min/max/avg/first/last` 时，系统应在流式执行器内按 series 和 field 聚合。
- When 查询请求包含窗口大小时，系统应按固定时间窗口输出聚合结果，并正确处理空窗口。
- When 查询请求包含 limit 和 offset 时，系统应在 shard/part/page 读取前尽可能下推分页边界。
- When 查询 context 被取消时，系统应在 catalog 解析、shard 扫描、part 查询、page 解码、merge、聚合阶段尽快返回 `context.Canceled` 或 `context.DeadlineExceeded`。
- When 聚合只需要 count/min/max 等可由 page metadata 支持的函数时，系统应优先使用 metadata 快路径；没有 metadata 时回退到样本扫描。

### 严格读放大控制

- When 查询命中多个 level 的 Part 时，系统应记录每次查询扫描的 shard 数、part 数、index block 数、value page 数、样本数和跳过数。
- When 单次查询扫描资源超过配置预算时，系统应返回明确的读预算错误。
- When L1+ compaction 完成后，系统应保持同 level 内 `(seriesID,time)` 范围尽量不重叠，并在检测到 overlap 时触发修复计划或暴露指标。
- When Part metadata 足以判断不相交时，系统应在打开 value block 前跳过该 Part。
- When 查询指定 fields 时，系统应保证未命中 field 的 value page 不被读取。

### Compaction 调度观测

- When compaction planner 生成计划时，系统应记录 level、候选 part 数、输入字节、估算输出字节、score 和 reason。
- When compaction 正在执行时，系统应暴露 active compaction 数、当前 level、输入输出字节、耗时和失败数。
- When compaction backlog 超过阈值时，系统应通过 metrics 和 health 状态暴露 degraded。
- When 手动 compact、后台 compact、retention 触发 compact 同时发生时，系统应通过调度器串行化同一 shard 的文件集合，并避免同一 Part 被重复选择。

### 10M+ 长期压测

- When 运行 10M wide10 写入压测时，系统应输出写入耗时、吞吐、RSS peak、heap alloc、GC、SSTable 数、data dir bytes 和 compaction backlog。
- When 运行 10M 查询压测时，系统应输出冷读/热读延迟、扫描 part/page/sample 数、吞吐、RSS peak 和错误数。
- When 压测结果超过基线阈值时，系统应以非零退出码失败。
- When 压测结束时，系统应清理临时数据目录、二进制、profile 和 coverage 产物，除非显式指定保留目录。

### 异常故障矩阵

- When WAL append、fsync、segment rollover、checkpoint、manifest write、part write、part close、remove old part 任一步失败时，系统应保持已提交数据可读，并保证重启后状态一致。
- When 进程在 flush 或 compaction 的关键点退出时，系统重启后应只加载已提交 manifest 中的 Part，并清理临时或孤儿目录。
- When 磁盘空间不足或文件系统返回短写时，系统应返回明确错误并避免损坏 manifest。
- When retention 与查询、写入、compaction 并发发生时，系统应保证 reader 不访问已关闭文件句柄。

### Metrics 与服务化运维

- When 引擎打开时，系统应注册存储 metrics collector，提供 WAL、MemTable、SSTable、query、compaction、retention、recovery、error、RSS/heap 指标。
- When 服务进程启动时，系统应提供 `/metrics`、`/debug/pprof/`、`/healthz`、`/readyz` 和 `/admin/compact` 运维端点。
- When storage backlog、WAL replay、compaction 或磁盘空间进入异常状态时，`/readyz` 应返回非 ready 并带结构化原因。
- When 管理端点执行手动 compact 或 metrics snapshot 时，系统应使用 context timeout 并记录审计日志。

## 架构边界

新增能力按以下边界拆分：

- `internal/queryexec`：流式 scan、merge、aggregate、window、pagination、cancellation。
- `internal/observability`：metrics registry、runtime metrics、query stats、compaction stats。
- `internal/faultinject`：文件系统、WAL、Manifest、PartWriter 故障注入接口。
- `internal/service`：HTTP 运维服务、health、pprof、metrics、admin endpoints。
- `tests/scale`：10M+ 长期压测入口和基线文件。
- `tests/fault`：异常掉电、磁盘满、fsync 失败矩阵。

`internal/engine` 保持 orchestration 角色，只消费接口，不承担全部查询和观测细节。

## 验证策略

- 单元测试覆盖每个执行器节点、聚合函数、窗口边界、分页、取消、读预算和指标计数。
- e2e 覆盖真实写入、flush、level compaction、跨 shard streaming query、聚合查询、服务化 metrics/health/admin。
- fault tests 覆盖关键持久化失败点和重启恢复。
- scale tests 覆盖 10M write/query/compact/restart，并输出机器可解析指标。
- 全量门禁保持 `go test ./...`、coverage、lint、e2e build/run 和产物清理。
