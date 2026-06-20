# Commercial Query Gap Closure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补齐 MTS 查询系统与商用时序数据库查询能力之间的关键差距，优先保障大结果集查询的稳定性、可观测性和可演进性。

**Architecture:** 保持 Builder/Analyzer/Planner/Optimizer/Physical/QueryService/Engine 分层。P0 能力优先在 QueryService 和 Engine iterator 层闭环，避免把服务端 API 固化为全量 materialization；后续语义能力在 Analyzer/Planner 层扩展。

**Tech Stack:** Go、`internal/queryservice`、`internal/queryexec`、`internal/engine`、`internal/querylang`、`go test`、`goimports-reviser`、`golangci-lint`。

---

## 当前与商用时序数据库的差距

1. **流式结果接口不足**：InfluxDB、VictoriaMetrics、Prometheus remote read 等查询链路都支持边读边传或分块输出；当前 `QueryService.Query` 默认 materialize `[]Row` / `[]ColumnSeries`，大结果集 RSS 风险仍然存在。
2. **输出预算不统一**：列路径已有 `MaxSamples` budget，row 路径缺少统一输出 row/sample budget，调用方不加 limit 时仍可能无限增长。
3. **算子级 profile 仍不足**：当前已补阶段级 profile，但还没有每个实际执行 stream/operator 的输入输出行数、耗时、跳过量和错误。
4. **成本优化器缺失**：Optimizer 还没有基于 series count、field count、shard count、limit、order、window 的成本估计，也不会基于成本调整执行顺序。
5. **查询语言能力仍弱**：当前是 Builder/结构化 Query，没有 SQL parser，也没有 PromQL/MetricsQL 完整语义。
6. **聚合语义仍不完整**：缺少 percentile/quantile、top/bottom、fill、align、downsample retention 等常见时序查询语义。
7. **并行 scan 调度不足**：缺少按 shard/part 的查询 worker pool、背压、优先级、公平调度和慢查询取消。
8. **二级索引仍有限**：已有 series/tag 过滤和 field page stats，但没有通用倒排索引、tag cardinality 统计、field value bloom/zone map 的完整组合。
9. **分页游标不稳定**：`LIMIT/OFFSET` 已支持，但没有稳定 cursor token，深分页仍可能退化。
10. **服务化协议不完整**：缺少 HTTP/gRPC 查询 API、流式响应协议、错误码规范和客户端超时/取消传播契约。

## EARS 总清单

- When 查询结果可能很大时，系统应提供 row/column streaming API，而不是强制 materialize 全量结果。
- When row 查询设置 `Budget.MaxSamples` 时，系统应按输出 row 数执行预算限制，并在超限时关闭上游 stream。
- When streaming 查询被调用方关闭时，系统应释放 admission slot 和底层 iterator。
- When streaming 查询上下文取消时，系统应停止读取并返回 context 错误。
- When 查询执行经过 QueryService 时，系统应保留 analyze/logical_plan/optimize/physical_plan 阶段 profile，并在 stream 关闭时补齐 execute profile。
- When Optimizer 构建计划时，系统应基于 series/field/shard/limit/order/window 生成成本估计。
- When 查询包含可下推 predicate 时，系统应优先执行高选择性 tag/time pushdown，再执行 field/page pruning。
- When 查询请求使用深分页时，系统应提供稳定 cursor token，避免反复 offset 扫描。
- When 查询使用常见时序函数 quantile/top/bottom/fill 时，系统应在 Analyzer 阶段校验类型，并在执行阶段返回确定语义。
- When 服务层暴露查询能力时，系统应提供清晰错误码、超时、取消和流式响应契约。

## Task 1: QueryService Streaming API

**状态:** 已完成。

**实现备注:** 已新增 `StreamResult`、`StreamingExecutor` 和 `Service.QueryStream`。Service 会包装 row/column stream，确保调用方消费到 EOF 或显式 Close 时释放 admission slot。`LayeredExecutor` 支持 `LayeredStreamReader` fast path；Engine 通过 `QuerySpecRowStream` / `QuerySpecColumnStream` 复用现有 iterator。

**EARS:** When 调用方使用 streaming 查询时，系统应返回 row/column stream，并在关闭 stream 时释放 admission slot。

- [x] 写失败测试：`TestLayeredExecutorQueryStreamRows` 对 row 查询返回 `Rows` stream，消费后能拿到数据。
- [x] 写失败测试：`TestServiceQueryStreamReleasesAdmissionOnClose` 验证提前关闭 stream 会释放 admission slot，后续查询可继续进入。
- [x] 在 `queryservice` 中新增 `StreamResult`、`StreamingExecutor`、`Service.QueryStream`。
- [x] 在 `LayeredExecutor` 中支持 streaming reader fast path。
- [x] 在 `Engine` 中新增 `QuerySpecRowStream` / `QuerySpecColumnStream` 适配现有 iterator。
- [x] 验证：`timeout 180s go test ./internal/queryservice ./internal/engine ./internal/queryexec -run 'TestServiceQueryStream|TestLayeredExecutorQueryStreamRows|TestBudgetRowStreamStopsWhenMaxSamplesExceeded|TestQueryRowsAppliesOutputBudget' -count=1 -timeout 180s`。

## Task 2: Row Output Budget

**状态:** 已完成。

**实现备注:** 已新增 `queryexec.NewBudgetRowStream`，按输出 row 数执行 `Budget.MaxSamples` 限制；Engine row 输出链路在排序和分页之后接入预算，超限时返回 `ErrReadBudgetExceeded` 并关闭上游。

**EARS:** When row 查询设置 `Budget.MaxSamples` 时，系统应最多返回该数量的 row；When 超限发生时，系统应返回 `ErrReadBudgetExceeded` 并关闭上游。

- [x] 写失败测试：`TestBudgetRowStreamStopsWhenMaxSamplesExceeded` 覆盖 `NewBudgetRowStream` 超过 MaxSamples 后返回 budget error。
- [x] 写失败测试：`TestQueryRowsAppliesOutputBudget` 覆盖 Engine `QueryRows` 在 row 输出超过 MaxSamples 时返回 budget error。
- [x] 实现 `queryexec.NewBudgetRowStream`。
- [x] 在 `Engine.QueryRowIterator` 输出路径接入 row budget。
- [x] 验证：`timeout 180s go test ./internal/queryservice ./internal/engine ./internal/queryexec -run 'TestServiceQueryStream|TestLayeredExecutorQueryStreamRows|TestBudgetRowStreamStopsWhenMaxSamplesExceeded|TestQueryRowsAppliesOutputBudget' -count=1 -timeout 180s`。

## Task 3: Operator-Level Runtime Profile

**状态:** 已完成。

**实现备注:** 已新增 `queryexec.NewProfiledRowStream` / `NewProfiledColumnStream`，可记录 stream 消费后的 rows/columns/samples、耗时和错误。QueryService streaming 路径已用该 wrapper 更新 `execute` profile entry。

**EARS:** When stream operator 被消费时，系统应记录 operator ID、输入输出数量、耗时和错误。

- [x] 为 `ColumnStream` / `RowStream` 增加 profile wrapper。
- [x] QueryService streaming 输出 profile 合并 stage profile 和 execute stream profile。
- [x] 增加 `TestProfiledRowStreamRecordsRowsAndDuration`、`TestProfiledColumnStreamRecordsColumnsSamplesAndErrors`、`TestProfiledRowStreamRecordsCloseErrors`。
- [x] 验证：`timeout 180s go test ./internal/queryservice ./internal/engine ./internal/queryexec -run 'TestServiceQueryStream|TestLayeredExecutorQueryStreamRows|TestBudgetRowStreamStopsWhenMaxSamplesExceeded|TestQueryRowsAppliesOutputBudget|TestProfiled.*Stream' -count=1 -timeout 180s`。

## Task 4: Cost-Based Plan Metadata

**状态:** 已完成。

**实现备注:** 已在 `model.QueryExplain` 中新增 `QueryCost`，Engine `BuildQueryPlan` 会输出 series/field/shard/limit/offset/window/order/cursor/estimated samples/plan class 成本元数据；Optimizer 新增 `Strategy`，可标记 `scan`、`bounded_scan`、`ordered_scan`、`aggregate` 等执行策略。

**EARS:** When 构建 QueryPlan 时，系统应输出 series/field/shard/limit/order/window 成本估计，供 Optimizer 和 Explain 使用。

- [x] 扩展 `QueryExplain` 成本字段。
- [x] 在 Engine plan 阶段填充成本估计。
- [x] Optimizer 根据成本标记执行策略。
- [x] 验证：`timeout 180s go test ./internal/queryservice ./internal/engine ./internal/queryexec ./internal/querylang ./internal/queryoptimizer -run 'Test.*(Cursor|Cost|HTTPHandler|Optimize|QueryStream|BudgetRow|BuildQueryPlan|QueryRowsResumes|Builder)' -count=1 -timeout 180s`。

## Task 5: Cursor Pagination

**状态:** 已完成。

**实现备注:** 已新增固定长度二进制 cursor token，使用 URL-safe base64 传输，包含 magic/version、排序方向、seriesID 和 timestamp。Builder/Query DTO 已支持 cursor，Engine row 查询在排序/分页前按 cursor resume，column 查询在分页前过滤样本；`ORDER BY time DESC` 行排序增加 `(timestamp desc, seriesID asc)` 稳定 tie-breaker。

**EARS:** When 调用方使用 cursor 查询时，系统应从 cursor 指定时间/series 位置继续扫描，避免深 offset。

- [x] 定义 cursor token 二进制结构。
- [x] Builder/Query DTO 增加 cursor 字段。
- [x] Row/Column scan 支持 cursor resume。
- [x] 验证：`timeout 180s go test ./internal/queryservice ./internal/engine ./internal/queryexec ./internal/querylang ./internal/queryoptimizer -run 'Test.*(Cursor|Cost|HTTPHandler|Optimize|QueryStream|BudgetRow|BuildQueryPlan|QueryRowsResumes|Builder)' -count=1 -timeout 180s`。

## Task 6: Query Protocol Surface

**状态:** 已完成。

**实现备注:** 已新增 `queryservice.NewHTTPHandler`，提供 `/query` JSON materialized 查询和 `/query/stream` NDJSON streaming 查询；错误响应统一为 `ErrorCode`，覆盖 admission、streaming unsupported 和通用查询错误，支持请求 context 取消传播到 `Service.Query` / `Service.QueryStream`。

**EARS:** When MTS 以服务方式暴露查询时，系统应提供 HTTP/gRPC 流式查询接口、错误码和取消传播。

- [x] 定义查询 API 请求/响应 DTO。
- [x] 实现 streaming response。
- [x] 增加服务层协议测试。
- [x] 验证：`timeout 180s go test ./internal/queryservice ./internal/engine ./internal/queryexec ./internal/querylang ./internal/queryoptimizer -run 'Test.*(Cursor|Cost|HTTPHandler|Optimize|QueryStream|BudgetRow|BuildQueryPlan|QueryRowsResumes|Builder)' -count=1 -timeout 180s`。
