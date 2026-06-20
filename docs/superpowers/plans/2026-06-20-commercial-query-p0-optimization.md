# Commercial Query P0 Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补齐查询引擎 P0 商用短板：低内存 DESC LIMIT、跨 series 分组聚合、常用时序函数、表达式 AST 基础和 field/page 下推。

**Architecture:** 保持现有 Builder/Analyzer/Planner/Engine/QueryExec 分层，优先在 `queryexec` 增加可测试算子，再由 Engine 接入。SSTable page stats 使用兼容式扩展：有统计则下推，无统计则走旧读取路径。

**Tech Stack:** Go、`internal/querylang`、`internal/queryanalyzer`、`internal/queryexec`、`internal/engine`、`internal/sstable`、`go test`、`goimports-reviser`、`golangci-lint`。

---

## 文件结构

- Modify: `internal/model/types.go`：表达式 AST、predicate 统计下推标记、QueryExplain 扩展。
- Modify: `types.go`：public DTO 同步表达式 AST 和 explain 字段。
- Modify: `query_builder.go`、`internal/querylang/builder.go`、`internal/querylang/spec.go`：Builder 构造表达式 AST。
- Modify: `internal/queryanalyzer/functions.go`、`internal/queryanalyzer/analyzer.go`：函数白名单、表达式分类和类型校验。
- Create/Modify: `internal/queryexec/order.go`、`internal/queryexec/group_aggregate.go`、`internal/queryexec/aggregate.go`：bounded desc、group aggregate、时序函数。
- Modify: `internal/engine/query.go`、`internal/engine/query_plan.go`：接入 bounded desc、group aggregate、explain 标记。
- Modify: `internal/sstable/types.go`、`internal/sstable/encoding.go`、`internal/sstable/write.go`、`internal/sstable/read.go`：value page stats 编解码和 page pruning。
- Tests: 上述包对应单元测试与 Engine 集成测试。

## Task 1: 表达式 AST 与 Analyzer 基础

**状态:** 已完成。

**实现备注:** 已新增 public/internal/model 表达式 AST，Builder 支持 `WhereExpr`、`And/Or/Not`，Analyzer 可递归分类 pushdown 与 post-filter。Engine row 路径增加表达式过滤兜底，复杂 OR 不再被错误平铺成 tag AND。

**EARS:** When 查询表达式包含 AND/OR/NOT 时，系统应能构造 AST、保留兼容谓词列表，并把可下推与 post-filter 谓词分类。

- [x] 写失败测试：Builder 生成表达式 AST。
- [x] 写失败测试：Analyzer 分类 AND/OR/NOT 表达式。
- [x] 实现 AST DTO、Builder 方法和 Analyzer 递归分类。
- [x] 验证：`timeout 180s go test ./internal/querylang ./internal/queryanalyzer . -run 'Test.*Expression|Test.*Predicate' -count=1 -timeout 180s`。

## Task 2: Bounded DESC LIMIT 与 row streaming 内存约束

**状态:** 已完成。

**实现备注:** `NewOrderedRowStream` 支持 limit/offset hint，DESC+LIMIT 使用固定容量 min-heap，仅保留 `limit+offset` 条最新 row；Engine row 查询传入 limit/offset 并保留现有分页输出。

**EARS:** When 查询使用 `ORDER BY time DESC LIMIT N OFFSET M` 时，系统应只保留 `N+M` 条候选 row，并输出正确降序分页结果。

- [x] 写失败测试：`NewOrderedRowStream` 在 desc+limit+offset 下不读取后再全量保存。
- [x] 写失败测试：Engine row 查询 desc limit 返回正确数据并关闭上游。
- [x] 实现 bounded row order stream 和 Engine 参数接入。
- [x] 验证：`timeout 180s go test ./internal/queryexec ./internal/engine -run 'Test.*Ordered.*Limit|Test.*Desc.*Limit|Test.*Row.*Limit' -count=1 -timeout 180s`。

## Task 3: 跨 series Group Aggregate

**状态:** 已完成。

**实现备注:** 新增 `GroupAggregateColumnStream`，按 group tag、field、function 合并多 series 聚合状态，支持 whole aggregate 和 time window aggregate；Engine 聚合分支已接入。

**EARS:** When 查询包含 `GROUP BY tag` 聚合时，系统应按 group tag 合并多个 series 的聚合状态。

- [x] 写失败测试：多个 series 同 group tag 的 sum/avg/count 合并为单个结果。
- [x] 写失败测试：group tag + window 同时聚合。
- [x] 实现 group aggregate stream 和 Engine 接入。
- [x] 验证：`timeout 180s go test ./internal/queryexec ./internal/engine -run 'Test.*Group.*Aggregate|Test.*GroupBy' -count=1 -timeout 180s`。

## Task 4: 常用时序函数

**状态:** 已完成。

**实现备注:** Analyzer 和 QueryExec 已支持 `difference/derivative/rate/irate/spread/median/mode/stddev/stdvar`。`rate/irate` 使用真实时间戳并处理 counter reset；`difference/derivative` 输出 transform 序列。

**EARS:** When 查询函数为 difference/derivative/rate/irate/spread/median/mode/stddev/stdvar 时，系统应返回正确结果或明确类型错误。

- [x] 写失败测试：数值函数和 selector/统计函数语义。
- [x] 写失败测试：Analyzer 拒绝非法字段类型。
- [x] 实现函数规则和 aggregate executor。
- [x] 验证：`timeout 180s go test ./internal/queryexec ./internal/queryanalyzer -run 'Test.*Aggregate|Test.*Function|Test.*Rate|Test.*Derivative|Test.*Mode' -count=1 -timeout 180s`。

## Task 5: Field/Page 级下推基础

**状态:** 已完成。

**实现备注:** SSTable value page index 增加可选 numeric min/max stats。写入路径为 float64/int64 page 生成 stats；读取路径根据 field predicate 跳过不可能命中的 page，并对读取出的样本做 predicate 过滤。无 stats page 保持旧读取路径。

**EARS:** When value page stats 可证明 field predicate 不可能命中时，系统应跳过该 page；When stats 缺失时，系统应走旧读取路径。

- [x] 写失败测试：SSTable 数值 page stats 能跳过 page。
- [x] 写失败测试：缺 stats page 不丢数据。
- [x] 实现 page stats 编解码和 read pruning。
- [x] 验证：`timeout 240s go test ./internal/sstable ./internal/engine -run 'Test.*Page.*Predicate|Test.*Field.*Prun|Test.*Compression' -count=1 -timeout 240s`。

## Task 6: 文档、格式化、Lint 与全量验证

**状态:** 已完成。

**实现备注:** `goimports-reviser`、`golangci-lint`、定向测试、全量测试和覆盖率命令均已执行。覆盖率命令通过，但仓库仍有多个历史包低于 90%。

**EARS:** When P0 优化完成后，系统应通过格式化、lint、定向测试和全量测试，并更新任务计划状态。

- [x] 运行 `timeout 300s goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .`。
- [x] 运行 `timeout 720s golangci-lint run ./...`。
- [x] 运行 `timeout 300s go test ./internal/queryexec ./internal/queryanalyzer ./internal/querylang ./internal/engine ./internal/sstable -count=1 -timeout 300s`。
- [x] 运行 `timeout 600s go test ./... -count=1 -timeout 10m`。
- [x] 更新本计划每个 Task 状态和实现备注。

## Task 7: 列/聚合路径完整表达式过滤

**状态:** 已完成。

**实现备注:** 新增 `NewExprFilteredColumnStream`，按 series 将列流重建为 row 语义后应用完整 `QueryExpr`，再还原为列流；非聚合列查询新增 `NewProjectedColumnStream`，确保 predicate-only 字段只参与过滤不进入 SELECT 输出。计划层已区分扫描字段和可安全下推字段谓词，复杂 OR/NOT 下禁用 field page stats 强下推，避免错误读剪枝。聚合查询的扫描字段集合已补充 aggregate fields，修复聚合字段不同于过滤字段时漏扫的问题。

**EARS:** When column/aggregate 查询包含多字段 WHERE 表达式时，系统应按 row 语义过滤；When OR/NOT 表达式包含 field predicate 时，系统应禁用 field page stats 强下推。

- [x] 写失败测试：`TestExprFilteredColumnStreamAppliesMultiFieldRowExpression` 覆盖 queryexec series 级表达式列过滤。
- [x] 写失败测试：`TestQueryColumnsAppliesExpressionFilterBeforeProjection` 覆盖列查询过滤后投影。
- [x] 写失败测试：`TestQueryAggregateScansAggregateFieldWithDifferentFilterField` 覆盖聚合字段和过滤字段不同的扫描计划。
- [x] 写失败测试：`TestBuildQueryPlanKeepsOrFieldPredicatesOutOfPageStatsPushdown` 覆盖 OR field predicate 禁止 page stats 下推。
- [x] 验证：`timeout 180s go test ./internal/queryexec ./internal/engine -run 'TestExprFilteredColumnStream|TestQueryColumnsAppliesExpressionFilterBeforeProjection|TestQueryAggregateScansAggregateFieldWithDifferentFilterField|TestBuildQueryPlanKeepsOrFieldPredicatesOutOfPageStatsPushdown|TestQueryRowsAppliesStructuredTagFieldOrderAndProjection|TestQueryRowsAppliesExpressionOrWithoutUnsafeTagPushdown' -count=1 -timeout 180s`。

## Task 8: Group Aggregate 增量状态优化

**状态:** 已完成。

**实现备注:** 新增 `incrementalAggregateState`，对 `count/sum/avg/min/max/first/last/spread/mode/stddev/stdvar` 使用 group/window compact state，避免每个 group 保存全量 `[]aggregatePoint`。`median/rate/irate/difference/derivative` 保留原完整序列 fallback，保证顺序语义不被性能优化破坏。

**EARS:** When group aggregate 使用可增量函数时，系统应按 group/window 维护 compact state；When 函数依赖完整排序序列时，系统应走 fallback 路径保证正确性。

- [x] 写失败测试：`TestGroupAggregateColumnStreamUsesIncrementalSelectorAndNumericState` 覆盖 `count/sum/avg/min/max/first/last` 的 group 聚合结果。
- [x] 写失败测试：`TestGroupAggregateColumnStreamUsesIncrementalDistributionState` 覆盖 `spread/mode/stdvar/stddev` 的 group 聚合结果。
- [x] 写失败测试：`TestGroupAggregateColumnStreamKeepsMedianOnPointFallback` 覆盖顺序敏感函数 fallback。
- [x] 写失败测试：`TestGroupAggregateColumnStreamReportsIncrementalStateErrors` 和 `TestGroupAggregateColumnStreamReportsMixedTypeIncrementalErrors` 覆盖增量状态错误路径。
- [x] 实现 `group_aggregate_state.go` 增量状态和 window materialization。
- [x] 验证：`timeout 180s go test ./internal/queryexec -run 'TestGroupAggregate|TestAggregateColumnStream' -count=1 -timeout 180s`。
- [x] 验证：`timeout 240s go test ./internal/engine -run 'TestQueryColumnsAppliesExpressionFilterBeforeProjection|TestQueryAggregateScansAggregateFieldWithDifferentFilterField|TestBuildQueryPlanKeepsOrFieldPredicatesOutOfPageStatsPushdown|TestQueryRowsAppliesStructuredTagFieldOrderAndProjection|TestQueryRowsAppliesExpressionOrWithoutUnsafeTagPushdown|TestQueryColumnsAppliesGroupAggregateAcrossSeries' -count=1 -timeout 240s`。

## Task 9: 表达式级 Tag Pushdown

**状态:** 已完成。

**实现备注:** `BuildQueryPlan` 现在会识别 tag/time-only 表达式，并在 catalog snapshot 的 series tags 上递归求值 `AND/OR/NOT`，对 `host=a OR host=b` 这类查询只保留命中 series。混合 field predicate 的表达式不做 series 裁剪，继续依赖 post-filter 保证正确性。

**EARS:** When WHERE 表达式仅包含 tag/time 条件且包含 OR/NOT 时，系统应在 catalog 层裁剪 series；When 表达式包含 field predicate 时，系统应保留完整候选 series 并依赖 post-filter 保证正确性。

- [x] 写失败测试：`TestBuildQueryPlanPushesDownTagOnlyOrExpression` 覆盖 `host=a OR host=b` 的 QueryPlan 只匹配两个 series。
- [x] 写失败测试：`TestBuildQueryPlanKeepsMixedFieldOrExpressionUnpruned` 覆盖 `host=a OR usage>0.8` 不进行 series 裁剪，避免漏读。
- [x] 实现 tag/time-only 表达式判定和 snapshot series 求值。
- [x] 验证：`timeout 180s go test ./internal/engine -run 'TestBuildQueryPlanPushesDownTagOnlyOrExpression|TestBuildQueryPlanKeepsMixedFieldOrExpressionUnpruned|TestBuildQueryPlanKeepsOrFieldPredicatesOutOfPageStatsPushdown' -count=1 -timeout 180s`。

## Task 10: QueryService 阶段级 Profile

**状态:** 已完成。

**实现备注:** `LayeredExecutor` 现在在 `Result.Profile` 中输出 `analyze/logical_plan/optimize/physical_plan/execute` 阶段 profile。成功执行会记录 rows/columns/samples 输出规模；失败阶段会在对应 profile entry 中记录错误信息并随错误返回。

**EARS:** When QueryService 执行查询时，系统应返回阶段 profile；When 任一阶段失败时，系统应记录对应阶段错误信息并返回错误。

- [x] 写失败测试：`TestLayeredExecutorProfilesSuccessfulRowQuery` 覆盖成功 row 查询返回 analyze/logical_plan/optimize/physical_plan/execute profile。
- [x] 写失败测试：`TestLayeredExecutorProfilesAnalyzeErrors` 覆盖 Analyzer 失败时 profile 记录 analyze 错误。
- [x] 实现阶段计时、结果规模统计和错误记录。
- [x] 验证：`timeout 180s go test ./internal/queryservice -run 'TestLayeredExecutor.*Profile|TestLayeredExecutorRunsColumnAggregatePath|TestLayeredExecutorRunsAnalyzerAndQuerySpecRows' -count=1 -timeout 180s`。
