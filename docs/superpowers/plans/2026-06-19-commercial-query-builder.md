# Commercial Query Builder Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 mts 查询系统升级为结构化 Builder 驱动的商用主链路，覆盖 SELECT、FROM、WHERE、GROUP、ORDER、LIMIT、OFFSET、聚合和窗口。

**Architecture:** Builder 构造 `querylang.QuerySpec`，Analyzer 校验语义，Planner/Optimizer/Physical 生成可解释计划，Engine 复用现有 shard/series/field/page scan 执行并增加流式过滤、排序和分页。QueryService 默认使用分层执行器，CompatExecutor 只保留兼容用途。

**Tech Stack:** Go、`internal/querylang`、`internal/queryanalyzer`、`internal/queryplanner`、`internal/queryoptimizer`、`internal/queryphysical`、`internal/queryexec`、`internal/queryservice`、`internal/engine`、public `mts` package、`go test`、`golangci-lint`。

---

## 文件结构

- Create: `internal/querylang/builder.go`：内部 Builder、WHERE 谓词、GROUP、ORDER 构造 API。
- Modify: `internal/querylang/spec.go`：扩展 `QuerySpec`、predicate、group、order 数据结构和 `ToModelQuery` 映射。
- Modify: `internal/querylang/normalize.go`：从旧 `model.Query` 生成新 `QuerySpec`。
- Modify: `internal/model/types.go`：扩展查询 DTO，保存 where/group/order/filter 语义。
- Modify: `types.go`：public Builder 类型、查询原语 DTO 和转换逻辑。
- Modify: `engine.go`：public structured query 入口。
- Modify: `internal/queryanalyzer/*`：校验 WHERE、GROUP、ORDER、函数和类型。
- Modify: `internal/queryplanner/*`：新增 Filter、Group、Sort 节点。
- Modify: `internal/queryoptimizer/*`：新增 pushdown/post-filter/order/group 决策。
- Modify: `internal/queryphysical/*`：新增 physical operators。
- Create/Modify: `internal/queryexec/filter.go`、`internal/queryexec/order.go`：流式过滤和时间排序。
- Modify: `internal/engine/query_plan.go`、`internal/engine/query.go`：旧入口转 QuerySpec，新入口走分层主链路。
- Modify: `internal/queryservice/*`：新增 `LayeredExecutor` 并作为推荐主执行器。
- Tests: 上述包对应单元测试和 Engine 集成测试。

## Task 1: QuerySpec 与 Builder

**状态:** 已完成。

**实现备注:** 已新增内部 `querylang.Builder`、public `mts.NewQuery()`、结构化 predicate/group/order DTO，并保持旧 `model.Query` 兼容。验证命令已通过。

**EARS:** When 调用方构造 SELECT/FROM/WHERE/GROUP/ORDER/LIMIT/OFFSET 查询时，系统应生成稳定 `QuerySpec`；When 输入非法时，系统应返回明确错误。

- [x] 新增 Builder、predicate、group、order 类型和测试。
- [x] 扩展 `QuerySpec` 与旧 `model.Query` 兼容转换。
- [x] 暴露 public `mts.NewQuery()` Builder。
- [x] 验证：`timeout 180s go test ./internal/querylang . -run 'Test.*Query.*Builder|TestPublicQueryBuilder|TestQuerySpec' -count=1 -timeout 180s`。

## Task 2: Analyzer 语义校验

**状态:** 已完成。

**实现备注:** Analyzer 已校验 WHERE field predicate 类型、GROUP BY time 聚合约束、ORDER 保留，并输出 pushdown/post-filter 分类。

**EARS:** When WHERE/GROUP/ORDER/聚合类型非法时，系统应在进入 SSTable 扫描前返回语义错误。

- [x] Analyzer 校验 WHERE 字段、字段类型、tag 条件、group time、order、limit/offset。
- [x] Analysis 输出下推谓词和 post-filter 谓词。
- [x] 验证：`timeout 180s go test ./internal/queryanalyzer -run 'TestAnalyze.*Where|TestAnalyze.*Group|TestAnalyze.*Order|TestAnalyzeRejectsInvalidFieldPredicateType|TestAnalyzeClassifiesWherePredicates|TestAnalyzeRejectsWindowGroupWithoutAggregate|TestAnalyzeRejectsUnsupportedFunctionForFieldType|TestAnalyzeMarksBoundaryRequirement' -count=1 -timeout 180s`。

## Task 3: Planner、Optimizer、Physical Plan

**状态:** 已完成。

**实现备注:** LogicalPlan 新增 Filter/Group/Sort，Optimizer 记录 post_filter/group/order/limit 决策，PhysicalPlan 新增 filter/group/sort operator。修复了构建逻辑树时 `&root` 导致的自引用问题。

**EARS:** When 查询包含 Filter/Group/Sort/Limit 时，系统应生成可解释的 logical/physical plan，并记录 pushdown 与 post-filter 决策。

- [x] LogicalPlan 增加 Filter、Group、Sort 节点。
- [x] Optimizer 输出 time/series/field/predicate/order/limit pushdown 标记。
- [x] PhysicalPlan 增加 Scan、Filter、Aggregate、Group、Sort、Limit、Project operator 描述。
- [x] 验证：`timeout 180s go test ./internal/queryplanner ./internal/queryoptimizer ./internal/queryphysical -count=1 -timeout 180s`。

## Task 4: QueryExec 过滤与排序算子

**状态:** 已完成。

**实现备注:** `queryexec` 新增 column/row field filter、time desc order stream 和 row pagination，供 Engine 主链路复用。

**EARS:** When WHERE field 比较或 ORDER BY time desc 存在时，系统应在流式阶段过滤和排序，并受预算限制。

- [x] 新增 ColumnStream field predicate filter。
- [x] 新增 RowStream field predicate filter。
- [x] 新增 column/row time order stream，支持 asc passthrough 和 desc bounded reverse。
- [x] 验证：`timeout 180s go test ./internal/queryexec -run 'Test.*Filter|Test.*Order|Test.*Pagination|Test.*Aggregate' -count=1 -timeout 180s`。

## Task 5: Engine 分层查询主链路

**状态:** 已完成。

**实现备注:** Engine 新增 `QuerySpecColumns/Rows/WithExplain`，BuildQueryPlan 支持结构化 tag predicate、扫描字段扩展、post-filter 记录；查询执行接入 field filter、projection、order 和 row pagination。默认未指定 EndTime 的查询归一化为无上界。

**EARS:** When 旧 `model.Query` 或新 `QuerySpec` 查询进入 Engine 时，系统应走同一分析、计划、优化、物理计划和流式执行链路。

- [x] Engine 新增 `QuerySpecColumns`、`QuerySpecRows`、`QuerySpecWithExplain`。
- [x] 旧 `QueryColumns`、`QueryRows`、`QueryWithExplain` 共享扩展后的 `model.Query` 主链路。
- [x] BuildQueryPlan 支持非 eq tag predicate 的 series 集合过滤和 field post-filter。
- [x] 执行链路接入 field filter、order、group/window、limit/offset。
- [x] 验证：`timeout 240s go test ./internal/engine -run 'TestQueryRowsAppliesStructuredTagFieldOrderAndProjection|TestQueryColumnsAppliesFieldFilterAndTimeDescOrder|TestQuerySpecRowsUsesStructuredMainPath|TestBuildQueryPlanReturnsEmptyWhenCatalogMisses|TestEngineLifecycleAndQueries' -count=1 -timeout 240s`。

## Task 6: QueryService 分层执行器

**状态:** 已完成。

**实现备注:** 新增 `LayeredExecutor`，queryservice 默认可通过该执行器跑 QuerySpec、Analyzer、Planner、Optimizer、PhysicalPlan 后再调用结构化 reader；`CompatExecutor` 保留。

**EARS:** When queryservice 执行查询时，系统应默认使用分层执行器，CompatExecutor 仅作为兼容后端保留。

- [x] 新增 `LayeredExecutor` 接口适配 Engine 结构化查询入口。
- [x] `Service.Query` 可通过 `Result` 输出 stats、explain、logical root、physical operators 和 pushdowns。
- [x] 验证：`timeout 180s go test ./internal/queryservice -run 'Test.*Layered|Test.*Service|TestCompat' -count=1 -timeout 180s`。

## Task 7: 集成验证与文档状态更新

**状态:** 已完成。

**实现备注:** 定向查询包测试、全量测试、goimports-reviser 和 golangci-lint 均已通过。覆盖率命令已通过执行，但项目整体仍存在多个包低于 90% 的历史门禁缺口，本专项新增的 `querylang` 已达到 `90.9%`。

**EARS:** When 所有查询原语完成后，系统应通过定向测试、全量测试和 lint，并更新本计划状态。

- [x] 运行定向查询包测试。
- [x] 运行 `timeout 600s go test ./... -count=1 -timeout 10m`。
- [x] 运行 `timeout 720s golangci-lint run ./...`。
- [x] 更新本计划每个 Task 状态和实现备注。
