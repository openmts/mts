# 商用级查询系统专项规格

## 状态

本专项已进入实施阶段。本轮先完成七层查询系统的基础架构闭环，建立 `querylang -> queryanalyzer -> queryplanner -> queryoptimizer -> queryphysical -> queryexec -> queryservice` 的稳定包边界、错误码和基础行为，并保持现有 Engine 查询主链路兼容。

## 目标

将 mts 查询能力从当前结构化 `model.Query` 和存储层 streaming executor，升级为可商用查询系统。系统应具备清晰的查询语言模型、语义分析、逻辑计划、优化器、物理计划、算子执行和服务治理能力，并保持现有查询结果语义兼容。

## 非目标

- 本专项不替换现有稳定查询 API，实施时应采用兼容迁移。
- 本专项不引入不受控的 SQL 完整语法范围。
- 本专项不以牺牲读一致性、内存可控性和错误可诊断性换取功能扩张。

## 架构层级

### 1. querylang

查询语言和 AST 层。负责表达 measurement、tag filter、field filter、时间范围、函数、窗口、排序、分页和输出格式。第一阶段可同时支持结构化 Query AST 和将来文本查询语言解析。

### 2. queryanalyzer

语义分析层。负责 database、retention policy、measurement、series、field schema、字段类型、函数签名、聚合合法性、窗口合法性、错误码归一化。

### 3. queryplanner

逻辑计划层。把 AST 转换为稳定的 logical plan，节点包括 `Scan`、`Filter`、`Project`、`Aggregate`、`Window`、`Sort`、`Limit`、`Explain`。

### 4. queryoptimizer

优化器层。负责 series/field/time 下推、shard/part/page 裁剪、limit 下推、first/last 边界快路径、读放大估算、预算校验、预聚合候选判断。

### 5. queryphysical

物理计划层。把 logical plan 转换为可执行 physical plan，选择 column stream、row stream、stats-only scan、merge stream、aggregate stream、window stream 等执行形态。

### 6. queryexec

算子执行层。提供可取消、可关闭、可观测、内存受控的 operator pipeline。所有算子必须显式处理错误、context、预算、stats 和资源释放。

### 7. queryservice

服务边界层。提供统一查询入口、结果流、explain、profile、慢查询、metrics、trace id、错误码、并发控制和租户级资源治理。

## 总体 EARS

- 当用户提交查询请求时，系统应先构建 AST 或结构化 QuerySpec，再进入语义分析。
- 当查询引用不存在的 measurement、field 或不支持的聚合函数时，系统应返回稳定错误码和可诊断信息。
- 当查询可下推 series、field、time、shard、part 或 page 过滤时，系统应在 explain 中明确记录下推项。
- 当查询会超过预算或 admission 限制时，系统应在执行前拒绝，或在执行中可诊断地中止。
- 当查询执行过程中 context 取消时，系统应关闭所有下游 reader 和 operator。
- 当查询返回大量结果时，系统应优先通过流式接口输出，不应强制全量物化。
- 当查询发生错误时，系统应返回带阶段信息的错误，说明错误来自解析、语义分析、计划、优化、执行或服务治理。
- 当查询完成时，系统应输出 query stats，覆盖耗时、扫描范围、读放大、返回样本、预算命中和错误状态。

## 兼容性原则

1. 现有 `model.Query` 作为兼容入口保留，并映射到新 QuerySpec。
2. `QueryColumns`、`QueryRows`、`QueryColumnIterator`、`QueryRowIterator` 行为保持兼容。
3. 新查询系统先作为内部分层重构和扩展，不强制一次性暴露完整文本查询语言。
4. 查询语义以现有样本扫描结果为事实基线。

## 商用验收方向

- 语义明确：每个函数、字段类型和窗口规则有定义。
- 可解释：每个查询能输出 logical plan、physical plan 和下推信息。
- 可治理：支持 timeout、并发限制、预算限制、慢查询和 metrics。
- 可扩展：新增函数、算子、索引和预聚合能力不需要侵入 Engine 主流程。
- 可验证：查询语义、错误路径、资源释放和读放大均有单元测试与 e2e 覆盖。
