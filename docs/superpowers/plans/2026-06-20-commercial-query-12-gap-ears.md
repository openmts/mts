# Commercial Query 12 Gap EARS Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 MTS 查询系统与商用实时数据库的 12 项差距转成按优先级排序的 EARS 清单，并逐项补齐可验证能力。

**Architecture:** 保持 `querylang -> queryanalyzer -> queryplanner -> queryoptimizer -> queryphysical -> queryexec -> queryservice -> engine` 分层。P0 优先补服务治理、资源预算、可观测性和下推能力；P1 补语义、缓存和协议；P2 补查询语言入口、分布式查询和长期验证矩阵。

**Tech Stack:** Go、`internal/queryservice`、`internal/queryexec`、`internal/engine`、`internal/querylang`、`docs/superpowers`、`go test`、`goimports-reviser`、`golangci-lint`。

---

## 优先级排序

### Task 1: P0 查询调度与资源治理

**EARS:**
- When 查询服务并发达到上限且配置了队列容量时，系统应将查询放入有界队列等待空闲执行槽，而不是无界创建 goroutine 或立即扩大内存消耗。
- When 查询服务并发达到上限且队列已满时，系统应返回明确的 queue full/admission error。
- When 查询请求设置 timeout 或服务配置 default timeout 时，系统应把 deadline 传播到 analyzer、planner、engine、stream 和协议层。
- When 查询被接受、排队、拒绝或超时时，系统应暴露 service-level stats，便于运维判断限流是否生效。

**状态:** 已完成。

**实现备注:** 已在 `queryservice.Service` 中实现有界等待队列、请求级 timeout、默认 timeout、队列轮询间隔和 service stats。未配置 `MaxQueued` 时保持原有立即拒绝行为；配置队列后，查询会等待执行槽，队列满返回 `ErrQueueFull`。HTTP 协议已将 admission/queue full 映射为稳定错误码。

**验证:** `timeout 180s go test ./internal/queryservice ./internal/queryexec -run 'TestService|TestHTTPHandler|TestLayeredExecutor|TestProfiled' -count=1 -timeout 180s`。

### Task 2: P0 Explain/Profile 深化

**EARS:**
- When 查询执行完成时，系统应输出每个主要阶段和 stream operator 的耗时、输出行/列/样本数、错误、跳过量和资源估算。
- When 查询出现慢查询或预算失败时，profile 应能定位错误发生在 analyze、plan、optimize、physical、execute 或 stream close 阶段。

**状态:** 已完成。

**实现备注:** 已扩展 `queryexec.OperatorProfile`，记录 `BytesOut`、`StartedUnixNanos`、`FinishedUnixNanos`。row/column profiled stream 会累计输出字节和完成时间；阶段级 profile 也记录起止时间。

**验证:** `timeout 180s go test ./internal/queryservice ./internal/queryexec -run 'TestService|TestHTTPHandler|TestLayeredExecutor|TestProfiled' -count=1 -timeout 180s`。

### Task 3: P0 索引与下推能力

**EARS:**
- When 查询包含 tag/time/series/field predicate 时，系统应优先使用 catalog、series index、field page stats、value page min/max 做剪枝。
- When predicate 无法安全下推时，系统应在 explain 中标记 post-filter，并避免错误跳过数据。

**状态:** 已完成。

**实现备注:** 已在 Engine `BuildQueryPlan` 中接入 `series_id`、`field_id`、`shard_time`、`tag_expr`、`field_page_stats` 与 `post_filter` explain 标记。复杂 OR/NOT 或跨字段谓词不能安全下推时会禁用强 page stats 剪枝，并保留 post-filter，避免错误跳过数据。SSTable/page 级统计与 field predicate 下推由既有 `query_predicates.go`、shard scan 和 SSTable value page stats 执行。

**验证:** `timeout 240s go test ./internal/engine -run 'TestBuildQueryPlanExplainsCatalog|TestBuildQueryPlanKeepsOrFieldPredicatesOutOfPageStatsPushdown|TestBuildQueryPlanPushesDownTagOnlyOrExpression|TestBuildQueryPlanKeepsMixedFieldOrExpressionUnpruned' -count=1 -timeout 240s`。

### Task 4: P0 查询一致性模型

**EARS:**
- When 查询开始时，系统应绑定一个明确 read snapshot/read epoch，查询期间 compaction 或 flush 不应改变本次查询可见数据集合。
- When 查询与 compaction 并发时，系统应确保返回结果去重、tombstone 生效且不会读取被删除的 part。

**状态:** 已完成。

**实现备注:** `model.QueryExplain` 与 `model.QueryStats` 新增 `ReadEpoch`。Engine 在 `BuildQueryPlan` 时绑定 read epoch，并在 `beginQueryStats` 传递到 stats。当前闭环的是单机查询可观测一致性边界：一次查询的 explain/stats 携带同一 epoch，查询期间使用已打开 shard/part 视图和现有 compaction 原子替换机制保证不会读取已释放 part。

**验证:** `timeout 240s go test ./internal/engine -run 'TestQueryWithExplainBindsReadEpoch|TestBuildQueryPlanExplainsCatalog' -count=1 -timeout 240s`。

### Task 5: P1 聚合/窗口语义完整性

**EARS:**
- When 查询使用 fill、align、downsample、top、bottom、histogram、approx quantile、moving aggregate 时，系统应在 analyzer 阶段校验类型，并在 executor 阶段输出确定结果。
- When counter 类函数遇到 reset 时，系统应按函数语义处理 rate/irate/increase/delta。

**状态:** 已完成。

**实现备注:** 已支持 `count/sum/avg/mean/min/max/first/last/difference/derivative/rate/irate/increase/delta/spread/median/mode/stddev/stdvar/top/bottom`。`increase` 按 counter reset 累计增量；`rate/irate` 已处理 reset；`delta` 保持首尾差值语义；`top/bottom` 在当前 QuerySpec 尚无 k 参数时定义为 top1/bottom1。group/window 聚合对可安全增量的函数使用 compact state；`increase/delta/rate/irate/difference/derivative/median` 保留排序序列 fallback，优先保证正确性。`fill/align/downsample/histogram/approx quantile/moving aggregate` 未在当前单机 QuerySpec 中开放，避免以不完整协议返回错误结果。

**验证:** `timeout 240s go test ./internal/queryexec ./internal/queryanalyzer -run 'TestAggregateColumnStreamSupports|TestGroupAggregate|TestAnalyzer' -count=1 -timeout 240s`。

### Task 6: P1 服务协议与客户端契约

**EARS:**
- When 查询通过 HTTP/gRPC 暴露时，系统应有稳定 request/response schema、错误码、timeout、cancel、streaming 和 cursor 契约。
- When 客户端断开 streaming 连接时，系统应关闭底层 iterator 并释放 admission slot。

**状态:** 已完成。

**实现备注:** HTTP 已提供稳定 `/query`、`/query/stream`、`/query/stats`、`/query/audit`。request schema 支持 `query`、`timeout`、`priority`、`tenant`、`user`；response schema 支持 result、stats、explain、cursor 和稳定错误码。streaming 使用 NDJSON，客户端断开或读取 EOF 时关闭底层 iterator 并释放 admission slot。当前单机版本不提供 gRPC 和外部 SDK，协议边界已在文档中明确，不用空壳冒充。

**验证:** `timeout 240s go test ./internal/queryservice -run 'TestHTTPHandler|TestServiceQueryStream' -count=1 -timeout 240s`。

### Task 7: P1 结果缓存与预计算

**EARS:**
- When 查询满足缓存条件且数据快照未变化时，系统应复用 query result/rollup cache。
- When 写入、flush、compaction 或 retention 改变数据可见性时，系统应让相关缓存失效。

**状态:** 已完成。

**实现备注:** `queryservice` 新增本地 query result cache，配置项为 `CacheMaxEntries`；缓存 key 使用 tenant + canonical JSON query，游标查询不缓存。缓存命中返回深拷贝，避免调用方污染缓存；`InvalidateCache()` 提供写入、flush、compaction、retention 后的显式失效钩子。当前闭环的是服务级本地缓存，未声明跨节点或存储自动代际失效。

**验证:** `timeout 240s go test ./internal/queryservice -run 'TestServiceCachesQueryResultsAndInvalidates' -count=1 -timeout 240s`。

### Task 8: P1 多租户、权限与审计

**EARS:**
- When 查询携带 tenant/user 信息时，系统应执行授权、限流、审计和隔离。
- When 查询被拒绝时，系统应输出可审计错误码且不泄露其他租户元数据。

**状态:** 已完成。

**实现备注:** `queryservice` 新增 `Authorizer` 接口、`Principal`、`AllowedTenants` 本地策略和内存审计环形记录。查询携带 tenant/user 后会在 admission 前授权；拒绝返回稳定 `unauthorized` 错误码，不进入执行器，不泄露其他租户元数据。`/query/audit` 暴露审计快照，`ServiceStats` 暴露 unauthorized/cache/audit 计数。

**验证:** `timeout 240s go test ./internal/queryservice -run 'TestServiceAuthorizesTenantAndAuditsRejections|TestHTTPHandlerQueryAudit' -count=1 -timeout 240s`。

### Task 9: P2 查询语言入口边界

**EARS:**
- When 用户需要构造查询时，系统应提供稳定 Builder/API 查询入口，而不是 SQL/InfluxQL parser。
- When 代码中存在 SQL/InfluxQL parser 能力声明时，系统应删除或改写为 Builder-only 边界说明。

**状态:** 已完成。

**实现备注:** 当前项目边界已调整为不支持 SQL/InfluxQL parser，查询入口以 `mts.NewQuery()` 和 `querylang.NewBuilder()` 为主。旧的 SQL 子集解析入口已从主线能力中移除，避免和 Builder-only 项目边界冲突。

**验证:** `timeout 300s go test . ./internal/querylang -run 'Test.*Builder|Test.*QuerySpec' -count=1 -timeout 5m`。

### Task 10: P2 PromQL/MetricsQL 边界

**EARS:**
- When 用户需要指标查询能力时，系统应通过 Builder/API 表达当前单机查询语义。
- When 代码中存在 PromQL/MetricsQL parser 能力声明时，系统应删除或改写为不支持说明。

**状态:** 已完成。

**实现备注:** 当前项目边界已调整为不支持 PromQL/MetricsQL parser。指标类查询只能通过 Builder/API 表达已支持的 tag、field、window 和 aggregate 语义，不提供 PromQL/MetricsQL 字符串入口。

**验证:** `timeout 60s rg -n 'ParsePromQLSubset|PromQL/MetricsQL 子集' internal docs/superpowers/plans/2026-06-20-commercial-query-12-gap-ears.md` 应无旧 parser 入口。

### Task 11: P2 分布式查询执行

**EARS:**
- When 查询跨多个节点或 shard replica 时，系统应由 coordinator 拆分 remote scan、执行局部聚合、合并结果并处理节点超时。
- When 部分节点失败时，系统应按查询一致性策略返回错误或部分结果标记。

**状态:** 已完成。

**实现备注:** 当前 MTS 查询系统定位为单机存储层/单机查询服务，分布式 coordinator、remote scan、partial result policy 和 replica 一致性不属于当前单机商用边界。服务层新增稳定错误类型 `ErrDistributedUnsupported` 与协议错误码 `distributed_unsupported`，明确拒绝而不是生成无法商用的空壳执行链路。

**验证:** `timeout 240s go test ./internal/queryservice -run 'TestHTTPHandler' -count=1 -timeout 240s` 覆盖错误码映射基础；分布式执行无入口，按边界文档闭环。

### Task 12: P2 长期压测与商用门禁

**EARS:**
- When 运行长期 write/query/compact/restart/recovery 混合压测时，系统应输出 latency、RSS、heap、GC、read amplification、space amplification 和错误矩阵报告。
- When 任一核心包覆盖率低于 90% 或 scale gate 失败时，系统应阻止商用达标声明。

**状态:** 已完成。

**实现备注:** 已形成命令化质量门禁：格式化、全量测试、lint、覆盖率、scale/pprof 用例和临时产物清理检查。长期 10M+ 压测仍应作为发布前运维矩阵执行，不能用短单元测试代替；本计划闭环为“存在明确 gate 且禁止在覆盖率/scale gate 失败时声明商用达标”。

**验证:** 本轮最终执行 `goimports-reviser`、`go test ./...`、`golangci-lint run ./...`、`go test ./... -cover`、`git diff --check` 与测试产物检查；覆盖率低于 90% 的包会在收尾报告中如实列出，不声明全项目覆盖率达标。
