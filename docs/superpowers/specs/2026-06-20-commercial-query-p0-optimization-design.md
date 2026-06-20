# 查询引擎 P0 商用优化专项设计

## 背景

当前查询系统已经具备结构化 Builder、基础 Analyzer/Planner/Optimizer/Physical 层、Engine 结构化入口和 QueryService 分层执行器。上一轮检视确认它仍存在 P0 商用差距：`ORDER BY time DESC LIMIT` 会在 row 路径全量加载排序，`GROUP BY tag` 没有跨 series 合并，常用时序函数不足，field predicate 主要是 post-filter，复杂表达式缺少可扩展 AST。

本专项聚焦单机查询主链路的 P0 能力闭环，不引入 SQL parser，不做分布式查询，不改变现有存储文件版本策略。

## 目标

- 将 `ORDER BY time DESC LIMIT/OFFSET` 改为可早停的流式 TopN 执行路径。
- 为 row 查询建立严格结果 materialization 边界，避免无界 RSS 增长。
- 实现跨 series 的 `GROUP BY tag` 聚合输出。
- 补齐常用时序函数的第一批商用子集。
- 引入表达式 AST 基础，让 AND/OR/NOT 与 predicate 分类可演进。
- 为 field predicate 建立 page 级下推基础，先支持数值 page min/max 过滤。

## 非目标

- 不实现 SQL parser。
- 不实现 PromQL/MetricsQL 完整语义。
- 不实现跨 measurement join、subquery、having。
- 不实现分布式 query coordinator。
- 不引入新的外部依赖。

## 架构设计

### 表达式 AST

新增 `QueryExpr`，支持：

- `Predicate`：兼容现有 `QueryPredicate`。
- `And`、`Or`、`Not`：表达复杂布尔关系。

旧 `Predicates []QueryPredicate` 保持兼容。Builder 默认把多个 `Where` 谓词组织为 AND。Analyzer 对表达式做分类：

- 可完全下推：time range、tag eq、tag in、tag exists。
- 半下推：OR 中存在可下推 tag/time 条件，但需要 post-filter 兜底。
- post-filter：field value predicate 和无法下推表达式。

### DESC LIMIT 快路径

新增 bounded row order stream。对于 `ORDER BY time DESC LIMIT N OFFSET M`：

- 不全量加载所有 row。
- 使用固定容量 min-heap 保存 `N+M` 个最新 row。
- 上游读完后输出 heap 中按时间降序排序的窗口。
- 当没有 limit 时保留现有全量排序行为，但必须受 query budget 和 query memory limit 约束。

该实现不是最终的反向 SSTable 扫描，但可以把内存从 O(total rows) 降为 O(limit+offset)，先闭合 RSS 风险。

### Row 查询 materialization 边界

`QueryRows` 对无 limit 查询增加默认预算保护：如果调用方没有显式 limit 且没有 MaxSamples，则仍允许兼容旧行为，但必须经过 query memory limit 检查。新增 iterator/cursor 相关测试，确保 limit 达到后关闭上游。

### Group Aggregate

新增 group aggregate stream：

- 对 `GROUP BY tag` 按 tag key 合并多条 series。
- 支持无 window 与 time window 两种聚合。
- 聚合状态采用增量状态机，避免先把全部样本展开成 row。
- 第一阶段覆盖 numeric aggregate 与 selector aggregate。

输出规则：

- group 结果的 `Tags` 仅保留 group key 中的 tag。
- field name 保持 `fn(field)`。
- timestamp 对 whole aggregate 使用首/末/窗口起点语义。

### 时序函数子集

新增函数：

- `difference`：相邻样本差值。
- `derivative`：差值除以时间间隔秒数。
- `rate`：窗口内首尾差值除以时间间隔，遇到 counter reset 时按 reset 后值累计。
- `irate`：使用最后两个样本计算瞬时 rate。
- `spread`：max-min。
- `median`：中位数。
- `mode`：众数。
- `stddev`、`stdvar`：总体标准差/方差。

所有函数必须定义类型约束。数值函数只支持 float64/int64；`mode` 支持四类字段。

### Field/Page 下推基础

SSTable value page index 增加可选统计：

- numeric min/max。
- bool has true/false。
- string min/max 先不作为强下推，仅作为未来扩展字段。

读路径对数值 `FieldGT/GTE/LT/LTE/Eq` 使用 page stats 跳过不可能命中的 page。没有 stats 的旧 page 按原逻辑读取，保证兼容。

### 列/聚合路径表达式过滤

列查询和聚合查询不能只按单列 field predicate 做 post-filter。对于 `usage > 0.5 AND temp > 50` 这类多字段条件，必须先按 series 将相关列重建成 row 语义，应用完整表达式，再还原为列流。非聚合列查询在过滤后执行 SELECT 字段投影，predicate-only 字段不得泄漏到输出。

复杂 `OR/NOT` 表达式下，field predicate 不能作为 AND 传入 SSTable page stats。计划层必须区分“扫描所需字段”和“可安全下推字段谓词”：扫描字段包含 SELECT 字段、aggregate 字段和 post-filter 字段；只有纯 AND 表达式或旧式扁平谓词才能启用 field page stats。

### Group Aggregate 增量状态

`GROUP BY tag` 对可增量计算的函数使用 compact state，而不是为每个 group 保存全量 `[]aggregatePoint` 后排序。首批增量函数包括 `count/sum/avg/min/max/first/last/spread/mode/stddev/stdvar`。`median/rate/irate/difference/derivative` 保持保守 fallback，继续依赖完整序列，避免破坏顺序语义。

### 表达式级 Tag Pushdown

复杂表达式不能简单平铺成 AND，但其中只包含 tag/time 条件的表达式可以在 catalog 层安全裁剪 series。计划层应对 `AND/OR/NOT` 递归求值：

- 对 tag/time-only 表达式，直接在 catalog snapshot 的 series tags 上计算布尔结果。
- 对包含 field predicate 的表达式，保留原有 row/column post-filter 兜底，不对 series 做错误裁剪。
- 对 `OR` 表达式，如 `host=a OR host=b`，应只扫描满足任一分支的 series，而不是扫描 measurement 下全部 series。

### 查询阶段 Profile

查询服务层应输出阶段级 profile，用于定位慢查询时间花费：

- `analyze`：语义分析和字段校验。
- `logical_plan`：逻辑计划生成。
- `optimize`：优化器执行。
- `physical_plan`：物理计划生成。
- `execute`：实际调用 Engine 执行查询。

每个 profile entry 应包含阶段 ID、输出行/列/样本规模、耗时和错误信息。profile 不替代存储层 `QueryStats`，而是补齐服务层执行链路观测。

## EARS 清单

- When 查询使用 `ORDER BY time DESC LIMIT 2000` 时，系统应只保留 bounded TopN row buffer，而不是 materialize 全部 row。
- When 查询使用 `ORDER BY time DESC LIMIT N OFFSET M` 时，系统应返回跳过前 M 条后的 N 条降序结果。
- When DESC 查询没有 LIMIT 时，系统应保持正确结果，并受已有 query budget/query memory limit 保护。
- When row 查询达到 LIMIT 时，系统应关闭上游 stream。
- When 查询包含 `GROUP BY tag` 聚合时，系统应把相同 group tag 的多个 series 合并为一个聚合结果。
- When 查询包含 `GROUP BY tag,time(window)` 时，系统应按 group key 和 window 同时聚合。
- When group tag 缺失时，系统应使用空字符串作为该 tag 的 group 值，并保持结果可解释。
- When 查询函数为 `difference` 时，系统应返回相邻数值差。
- When 查询函数为 `derivative` 时，系统应按秒返回相邻样本变化率。
- When 查询函数为 `rate` 或 `irate` 时，系统应按 counter 语义处理 reset。
- When 查询函数为 `spread/median/mode/stddev/stdvar` 时，系统应返回稳定且类型正确的聚合结果。
- When 非数值字段使用数值函数时，系统应在 Analyzer 阶段返回语义错误。
- When 查询表达式包含 AND/OR/NOT 时，系统应能构造 AST 并保留兼容谓词列表。
- When 表达式无法完全下推时，系统应保留 post-filter 兜底，不能返回不满足条件的数据。
- When column 查询包含多字段 field expression 时，系统应按 row 语义过滤后再输出列，不能对每列独立过滤导致时间戳错配。
- When aggregate 查询的聚合字段不同于过滤字段时，系统应同时扫描聚合字段和过滤字段，并在过滤后只聚合目标字段。
- When 查询表达式包含 OR/NOT field predicate 时，系统应禁用 field page stats 强下推，避免把 OR/NOT 错当成 AND。
- When SSTable value page 具备数值 min/max 统计且 field predicate 不可能命中时，系统应跳过该 page。
- When SSTable value page 缺少统计时，系统应按旧路径读取，不能丢数据。
- When group aggregate 使用可增量函数时，系统应使用 group/window 增量状态，避免保存全量 points。
- When group aggregate 使用 median/rate/irate 等顺序敏感函数时，系统应保留完整序列 fallback，优先保证结果正确。
- When WHERE 表达式仅包含 tag/time 条件且包含 OR/NOT 时，系统应在 catalog 层裁剪 series，减少后续 shard/part 扫描。
- When WHERE 表达式混合 field predicate 时，系统应保留 post-filter 兜底，不能用 tag pushdown 错误排除可能命中的 series。
- When QueryService 执行查询时，系统应返回 analyze/logical_plan/optimize/physical_plan/execute 阶段 profile。
- When 查询执行失败时，系统应在对应 profile entry 中记录错误信息，便于诊断。
- When explain 输出时，系统应标记 `field_page_stats`、`bounded_desc_order`、`group_aggregate` 等新能力。

## 验收标准

- 新增单元测试先失败后通过，覆盖 bounded DESC、group aggregate、时序函数、表达式 AST 和 page stats pruning。
- 定向测试通过：`go test ./internal/queryexec ./internal/queryanalyzer ./internal/querylang ./internal/engine ./internal/sstable -count=1 -timeout 300s`。
- 全量测试通过：`go test ./... -count=1 -timeout 10m`。
- `goimports-reviser` 和 `golangci-lint run ./...` 通过。
