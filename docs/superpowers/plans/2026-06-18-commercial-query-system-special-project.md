# Commercial Query System Special Project Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 mts 查询能力建设为可商用查询系统，覆盖查询语言、语义分析、逻辑计划、优化器、物理计划、算子执行和服务治理七个层级。

**Architecture:** 查询请求从 `querylang` 进入，经过 `queryanalyzer` 校验，再由 `queryplanner` 生成 logical plan，`queryoptimizer` 选择下推和预算策略，`queryphysical` 生成 physical plan，`queryexec` 执行 operator pipeline，最终由 `queryservice` 统一暴露结果、错误、explain、profile 和 metrics。

**Tech Stack:** Go、`internal/model`、`internal/engine`、`internal/queryexec`、`internal/sstable`、`internal/catalog`、`internal/service`、`tests/e2e`、`tests/pprof`。

---

## 当前决策

本专项已进入实施阶段。本轮采用兼容式分层引入：先建立七层查询系统的内部契约、错误码、基础计划与执行管线，不直接替换现有 Engine 查询主链路。

## Task 1: querylang 查询语言与 AST 层

**状态:** 已完成基础实现。

**实现备注:** 已新增 `QuerySpec`、`Scope`、`TimeRange`、`Aggregate`、`Output`、`FromModelQuery`、`ToModelQuery` 和结构合法性错误码。当前支持 `model.Query` 兼容转换、默认 database/rp、默认 column 输出、`mean -> avg` 归一化和非法时间范围拒绝。

**目标:** 建立稳定查询表达模型，为结构化查询和未来文本查询语言提供统一 AST。

**建议文件:**
- Create: `internal/querylang/ast.go`
- Create: `internal/querylang/spec.go`
- Create: `internal/querylang/normalize.go`
- Test: `internal/querylang/ast_test.go`
- Test: `internal/querylang/normalize_test.go`

**EARS 清单:**
- 当用户使用现有 `model.Query` 查询时，系统应能将其转换为 `querylang.QuerySpec`。
- 当查询包含 database、retention policy、measurement 时，系统应在 AST 中保留这些作用域信息。
- 当查询包含 tag filter 时，系统应以结构化谓词表达等值、存在、不存在和范围扩展能力。
- 当查询包含 field filter 时，系统应以字段选择列表表达目标字段。
- 当查询包含时间范围时，系统应明确保存闭区间或半开区间语义。
- 当查询包含 aggregate、window、limit、offset 时，系统应以独立 AST 节点表达。
- 当查询指定输出格式时，系统应能表达 column、row、aggregate、explain 和 profile 输出意图。
- 当 QuerySpec 归一化时，系统应填充默认 database、retention policy、时间范围和输出格式。
- 当 QuerySpec 中存在非法空 measurement 或非法时间范围时，系统应返回 querylang 阶段错误。
- 当未来接入文本查询语言时，系统应复用同一 QuerySpec，不应绕过 AST。

**验收标准:**
- `model.Query -> QuerySpec -> model.Query` 的兼容字段不丢失。
- QuerySpec 不依赖 Engine、SSTable 或 Catalog 包。
- AST 层不做 schema 校验，只做结构合法性校验。

## Task 2: queryanalyzer 语义分析层

**状态:** 已完成基础实现。

**实现备注:** 已新增 `SchemaProvider`、`Analyzer`、`Analysis`、函数/类型支持矩阵和语义错误码。当前覆盖 measurement/field schema 加载、字段不存在、函数不存在、函数类型不匹配、分页和窗口参数校验、`first/last` 边界需求标记。

**目标:** 在进入计划前完成 schema、函数、字段类型和聚合语义校验。

**建议文件:**
- Create: `internal/queryanalyzer/analyzer.go`
- Create: `internal/queryanalyzer/functions.go`
- Create: `internal/queryanalyzer/errors.go`
- Test: `internal/queryanalyzer/analyzer_test.go`
- Test: `internal/queryanalyzer/functions_test.go`

**EARS 清单:**
- 当 measurement 不存在时，系统应返回稳定的 measurement-not-found 语义错误。
- 当字段不存在时，系统应返回 field-not-found 错误，且不得进入 SSTable 扫描。
- 当字段类型与聚合函数不兼容时，系统应返回 function-type-mismatch 错误。
- 当聚合函数为空或未知时，系统应返回 unsupported-function 错误。
- 当查询包含 window 且 window 小于等于 0 时，系统应返回 invalid-window 错误。
- 当查询包含 limit 或 offset 且值为负数时，系统应返回 invalid-pagination 错误。
- 当查询字段既用于原始输出又用于聚合输出时，系统应生成明确的输出列命名规则。
- 当查询需要 first/last 边界语义时，系统应在分析结果中标记 boundary requirement。
- 当查询需要样本扫描才能保证正确性时，系统应标记 scan-required。
- 当查询可以被预聚合或 stats 快路径候选处理时，系统应只标记 candidate，不应直接决定执行路径。

**验收标准:**
- Analyzer 只依赖 metadata 接口和 QuerySpec，不直接依赖 Engine。
- 所有语义错误有错误码、阶段、用户可读信息和内部原因。
- 函数支持矩阵覆盖 `float64/int64/string/bool` 四类字段。

## Task 3: queryplanner 逻辑计划层

**状态:** 已完成基础实现。

**实现备注:** 已新增 `LogicalPlan`、`Node`、`Scan/Aggregate/Project/Limit` 节点和 `Explain`。当前聚合查询生成 `Scan -> Aggregate`，普通查询生成 `Scan -> Project`，分页生成显式 `Limit` 节点。

**目标:** 把语义分析结果转换成稳定 logical plan，作为优化器输入。

**建议文件:**
- Create: `internal/queryplanner/logical_plan.go`
- Create: `internal/queryplanner/planner.go`
- Create: `internal/queryplanner/explain.go`
- Test: `internal/queryplanner/planner_test.go`
- Test: `internal/queryplanner/explain_test.go`

**EARS 清单:**
- 当查询只读取原始列时，系统应生成 `Scan -> Project` logical plan。
- 当查询包含 tag filter 时，系统应生成可下推的 filter 条件。
- 当查询包含 field filter 时，系统应把字段裁剪放在 scan 阶段之前。
- 当查询包含 aggregate 时，系统应生成 `Scan -> Aggregate` logical plan。
- 当查询包含 window aggregate 时，系统应生成 `Scan -> Window -> Aggregate` 或等价 logical node。
- 当查询包含 row 输出时，系统应显式生成 row materialization 或 row stream 节点。
- 当查询包含 limit/offset 时，系统应生成 limit 节点，并标记是否可下推。
- 当查询包含 explain 时，系统应生成可序列化 logical explain tree。
- 当查询为空结果可在 metadata 阶段确认时，系统应生成 EmptyPlan。
- 当 logical plan 生成失败时，系统应返回 planner 阶段错误。

**验收标准:**
- LogicalPlan 不包含具体 Shard、Part、SSTable reader。
- LogicalPlan 可稳定序列化，用于 explain 和测试断言。
- 现有 `BuildQueryPlan` 能迁移为 logical planner 的兼容适配层。

## Task 4: queryoptimizer 优化器层

**状态:** 已完成基础实现。

**实现备注:** 已新增 `Optimize`、`Estimate`、预算拒绝、固定下推记录和 `HasPushdown`。当前根据 field/time/tag 记录 `field_id`、`time_range`、`series_id` 下推，并在执行前拒绝超出 shard/part/sample 预算的计划。

**目标:** 基于 metadata、shard、part、page 和预算信息，选择安全的下推与读放大控制策略。

**建议文件:**
- Create: `internal/queryoptimizer/optimizer.go`
- Create: `internal/queryoptimizer/rules.go`
- Create: `internal/queryoptimizer/cost.go`
- Create: `internal/queryoptimizer/safety.go`
- Test: `internal/queryoptimizer/optimizer_test.go`
- Test: `internal/queryoptimizer/cost_test.go`

**EARS 清单:**
- 当 tag filter 可解析为 seriesID 集合时，系统应下推 seriesID filter。
- 当 field filter 可解析为 fieldID 集合时，系统应下推 fieldID filter。
- 当时间范围不覆盖某个 shard 时，系统应跳过该 shard。
- 当时间范围不覆盖某个 part 时，系统应跳过该 part。
- 当 value page index 可判断 page 不相交时，系统应跳过该 page。
- 当查询只需要 first 或 last 且无 window 时，系统应选择 boundary page 快路径。
- 当 limit 可以在 row 或 column 层早停时，系统应标记 limit pushdown。
- 当查询预算低于预估扫描成本时，系统应在执行前拒绝或降级计划。
- 当 part/level 存在覆盖写或 tombstone 风险时，系统应禁用不安全的 stats 或预聚合路径。
- 当优化器做出下推决策时，系统应在 explain 中记录规则名、输入估算和输出估算。

**验收标准:**
- 优化规则按固定顺序执行，输出可重复。
- Cost 模型先使用可解释的启发式，不引入不可诊断黑盒。
- 任一优化规则不得改变查询语义；不确定时必须回退保守计划。

## Task 5: queryphysical 物理计划层

**状态:** 已完成基础实现。

**实现备注:** 已新增 `PhysicalPlan`、物理 `Operator` 描述和 builder。当前从 logical tree 生成稳定 operator 序列，聚合计划选择 column 输出，普通计划选择 row 输出。

**目标:** 把 logical plan 转换成可执行 physical plan，并选择具体 stream 与 scan 形态。

**建议文件:**
- Create: `internal/queryphysical/physical_plan.go`
- Create: `internal/queryphysical/builder.go`
- Create: `internal/queryphysical/operators.go`
- Test: `internal/queryphysical/builder_test.go`

**EARS 清单:**
- 当 logical scan 已包含 shard/part/page 下推时，系统应生成带下推参数的 physical scan node。
- 当输出为列式结果时，系统应选择 column stream physical plan。
- 当输出为行式结果时，系统应选择 row stream physical plan。
- 当聚合可在列流上执行时，系统应选择 aggregate column operator。
- 当窗口聚合存在时，系统应选择 window aggregate operator。
- 当查询需要排序但输入不能保证有序时，系统应选择 bounded sort 或拒绝无预算排序。
- 当查询包含 limit/offset 时，系统应把 limit 放到最早安全位置。
- 当查询需要 explain 或 profile 时，系统应在 physical plan 中保留 operator id。
- 当物理计划需要打开 shard 或 part reader 时，系统应定义明确的生命周期和 Close 顺序。
- 当 physical plan 构建失败时，系统应返回 physical-planning 阶段错误。

**验收标准:**
- PhysicalPlan 明确区分 plan 描述和运行时 reader。
- 每个 physical operator 都有稳定 id、输入、输出和资源预算。
- 现有 Engine 查询路径可作为第一版 physical executor 的后端。

## Task 6: queryexec 算子执行层

**状态:** 已完成基础实现。

**实现备注:** 已新增统一 `Operator` 接口、`Pipeline`、`Profile` 和基础计数算子测试工具。当前支持 context 取消、limit 早停、幂等关闭、错误记录和 operator rows profile。

**目标:** 将当前 streaming 查询执行器升级为可组合 operator pipeline。

**建议文件:**
- Modify: `internal/queryexec/types.go`
- Create: `internal/queryexec/operator.go`
- Create: `internal/queryexec/pipeline.go`
- Create: `internal/queryexec/profile.go`
- Test: `internal/queryexec/operator_test.go`
- Test: `internal/queryexec/pipeline_test.go`

**EARS 清单:**
- 当 operator 启动时，系统应接收 context、预算、stats recorder 和 profile recorder。
- 当 context 取消时，系统应停止当前 operator 并关闭所有上游资源。
- 当上游返回错误时，系统应停止 pipeline，并保留错误阶段和 operator id。
- 当 operator 达到内存预算时，系统应返回 read-budget 或 memory-budget 错误。
- 当 operator 达到 limit 早停条件时，系统应关闭上游并返回已读取结果。
- 当 aggregate operator 遇到不支持类型时，系统应返回函数类型错误。
- 当 row merge operator 处理宽表行时，系统应避免全量结果集 map 物化。
- 当 operator 输出 batch 或 stream 时，系统应保证调用方可以逐步消费。
- 当 Close 被重复调用时，系统应保持幂等。
- 当 pipeline 完成时，系统应输出每个 operator 的 rows、columns、samples、bytes、duration 和 errors。

**验收标准:**
- 所有 operator 都实现统一接口。
- 所有 operator 的错误传播、Close、context cancel 有单元测试。
- 当前 `ColumnStream`、`RowStream` 能被适配为 operator。

## Task 7: queryservice 服务边界层

**状态:** 已完成基础实现。

**实现备注:** 已新增 `Service`、`Request`、`Result`、`Executor`、admission 并发控制、`ErrAdmissionRejected` 和兼容执行器。当前 queryservice 只依赖执行接口，可通过 `NewCompatExecutor` 接入现有 `QueryColumns/QueryRows` 后端。

**目标:** 建立商用查询入口，统一请求、响应、错误、explain、profile、metrics 和资源治理。

**建议文件:**
- Create: `internal/queryservice/service.go`
- Create: `internal/queryservice/request.go`
- Create: `internal/queryservice/response.go`
- Create: `internal/queryservice/admission.go`
- Create: `internal/queryservice/errors.go`
- Modify: `internal/service/server.go`
- Test: `internal/queryservice/service_test.go`
- Test: `tests/e2e/query_service/main.go`

**EARS 清单:**
- 当客户端提交查询请求时，系统应创建 query id 和 trace id。
- 当请求超过最大并发数时，系统应返回 admission-rejected 错误。
- 当请求超过默认 timeout 时，系统应取消执行并释放资源。
- 当请求要求 explain 时，系统应返回 logical plan、physical plan 和 optimizer 决策。
- 当请求要求 profile 时，系统应返回 operator 级耗时、读取量和错误。
- 当查询成功时，系统应支持 row、column 和 aggregate 结果流。
- 当查询失败时，系统应返回阶段、错误码、消息、query id 和是否可重试。
- 当查询耗时超过慢查询阈值时，系统应记录慢查询事件。
- 当服务导出 metrics 时，系统应包含 active queries、queued queries、rejected queries、query duration、query errors 和 bytes read。
- 当服务关闭时，系统应取消所有活跃查询并等待资源释放。

**验收标准:**
- queryservice 不直接依赖 SSTable 实现细节，只依赖 planner/executor 接口。
- 服务错误码对 public API 稳定。
- e2e 覆盖成功查询、explain、profile、超时、并发拒绝和慢查询记录。

## 依赖顺序

1. Task 1 必须先完成，因为后续所有层都依赖 QuerySpec。
2. Task 2 必须在 Task 3 前完成，因为 logical plan 需要语义分析结果。
3. Task 3 和 Task 4 可以先以现有 Engine 查询能力为底座逐步替换。
4. Task 5 必须在 Task 6 前完成，避免执行层直接理解 logical plan。
5. Task 6 完成后才能接 queryservice。
6. Task 7 完成后必须补充完整 e2e 和 pprof 查询门禁。

## 本轮实施验收

当前阶段的验收标准是：

- 七层查询系统包边界已落地到 `internal/querylang`、`internal/queryanalyzer`、`internal/queryplanner`、`internal/queryoptimizer`、`internal/queryphysical`、`internal/queryexec`、`internal/queryservice`。
- 现有 `model.Query` 可转换为 `QuerySpec` 并可回转为兼容查询。
- Analyzer、Planner、Optimizer、Physical Builder、Operator Pipeline 和 Query Service 均有定向单元测试。
- 当前实现不直接替换 Engine 查询主链路，后续接入应通过兼容执行器逐步迁移。
- 定向验证命令：`timeout 180s go test ./internal/querylang ./internal/queryanalyzer ./internal/queryplanner ./internal/queryoptimizer ./internal/queryphysical ./internal/queryexec ./internal/queryservice -count=1`。
