# 商用查询 Builder 与主链路专项设计

## 背景

当前 mts 查询系统已经有 `querylang`、`queryanalyzer`、`queryplanner`、`queryoptimizer`、`queryphysical`、`queryexec`、`queryservice` 七层包边界，但主查询路径仍主要通过 `model.Query` 兼容旧 Engine 查询能力。系统暂不支持 SQL parser，本专项不实现 SQL 字符串解析，而是提供结构化 Builder API，让调用方通过 `Select`、`From`、`Where`、`GroupBy`、`OrderBy`、`Limit`、`Offset` 等查询原语构造查询。

目标是把查询主链路从“兼容封装”推进到“结构化查询语义 + 分层计划 + 流式执行”的商用基础。底层扫描继续复用现有 shard、series、field、part、page 下推能力；无法安全下推的条件必须在流式阶段过滤，并在 explain 中明确标记。

## 范围

本专项包含：

- 新增查询 Builder API。
- 扩展内部 `QuerySpec` AST，支持 SELECT、FROM、WHERE、GROUP BY、ORDER BY、LIMIT、OFFSET、聚合和窗口。
- 支持 WHERE：时间范围、tag 等值/不等值/存在/in、field 等值/不等值/比较。
- 支持 GROUP BY：tag 列表和 time window。
- 支持 ORDER BY：time asc 和 time desc。
- 将 `queryservice` 默认执行路径切换为分层查询执行器。
- 保持现有 `model.Query`、`QueryColumns`、`QueryRows` 兼容。

本专项不包含：

- SQL parser。
- 跨 measurement join。
- 正则匹配、模糊匹配和复杂布尔表达式优化。
- 分布式查询。
- 预聚合物化视图。

## 查询语义

### SELECT

`Select(fields...)` 选择原始字段。字段为空时表示查询 measurement 下全部字段。`Aggregate(function, field)` 表示聚合输出，聚合函数覆盖当前已支持的 `count/sum/avg/mean/min/max/first/last`，其中 `mean` 归一化为 `avg`。

### FROM

`From(database, retentionPolicy, measurement)` 选择查询源。database 和 retention policy 允许为空，Engine 会按默认配置补齐；measurement 必填。

### WHERE

WHERE 使用结构化谓词表示：

- `TimeRange(start, end)`：映射为 shard/part/page 时间过滤。
- `TagEq(name, value)`：可下推到 catalog series 过滤。
- `TagNe(name, value)`、`TagExists(name)`、`TagIn(name, values...)`：先通过 catalog 获取候选 series，再在查询计划阶段过滤 series 集合。
- `FieldEq/FieldNe/FieldGT/FieldGTE/FieldLT/FieldLTE(field, value)`：字段值条件先作为流式 post-filter 执行。

多个 WHERE 条件按 AND 语义组合。不支持 OR。空 WHERE 表示全范围查询，但仍受 query budget 限制。

### GROUP BY

`GroupByTags(tags...)` 表示聚合结果按 tag 维度分组。第一阶段存储层天然按 series 输出，tag group 的语义以 series tag 为边界，不在本轮跨 series 合并 tag group。

`GroupByTime(window)` 等价于窗口聚合，窗口必须大于 0。

### ORDER BY

`OrderByTimeAsc()` 和 `OrderByTimeDesc()` 控制输出时间顺序。默认 asc。asc 复用现有时间有序路径；desc 在列流或行流阶段按 bounded slice 反转单列/单 series 数据，不允许无预算地全量跨 series 排序。

### LIMIT/OFFSET

LIMIT/OFFSET 必须尽量在流式阶段早停。行查询按 row 计数；列查询按样本计数；聚合查询按聚合样本计数。

## 架构

### Builder 层

新增 `querylang.Builder` 作为内部构造器，并在 public package 暴露 `mts.NewQuery()`。Builder 只构造结构化查询，不访问 Engine、Catalog 或 SSTable。

### Analyzer 层

Analyzer 校验 measurement、field、聚合函数、WHERE 字段类型、GROUP/WINDOW、ORDER 和分页。可下推条件和必须 post-filter 的条件在 Analysis 中标记。

### Logical Planner 层

LogicalPlan 显式表示：

- `Scan`
- `Filter`
- `Aggregate`
- `Group`
- `Sort`
- `Limit`
- `Project`

### Optimizer 层

Optimizer 基于 Analysis 和估算信息生成 pushdown 决策：

- `time_range`
- `series_id`
- `field_id`
- `post_filter`
- `order_time`
- `limit`

### Physical/Executor 层

PhysicalPlan 生成稳定 operator 描述。执行层通过新的 `queryservice.LayeredExecutor` 驱动 Engine 的计划和流式扫描接口。现有 `CompatExecutor` 保留给兼容测试，但不作为 queryservice 默认推荐路径。

### Engine 集成

Engine 新增结构化查询入口，内部将 `QuerySpec` 转为带过滤、排序、分组语义的执行请求。旧 `model.Query` 先转换为 `QuerySpec` 再进入同一主链路，避免新旧查询分叉。

## EARS 清单

- When 调用方使用 `NewQuery().Select("usage").From("metrics", "autogen", "cpu").Build()` 时，系统应生成包含 SELECT 和 FROM 的稳定 `QuerySpec`。
- When measurement 为空时，Builder `Build()` 应返回明确错误。
- When LIMIT 或 OFFSET 为负数时，Builder 或 Analyzer 应返回明确错误。
- When WHERE 包含时间范围时，系统应将其映射到 `QuerySpec.TimeRange` 并在 Engine 查询计划中跳过不相交 shard。
- When WHERE 包含 `TagEq` 时，系统应下推到 catalog series 过滤。
- When WHERE 包含 `TagNe`、`TagExists` 或 `TagIn` 时，系统应解析候选 series，并按 tag predicate 过滤 series 集合。
- When WHERE 包含 field 比较时，系统应在流式执行阶段过滤样本，不应把不满足条件的样本返回给调用方。
- When WHERE field 比较的字段未在 SELECT 或 Aggregate 中出现时，系统应仍能使用该字段过滤，但输出只包含 SELECT 字段。
- When WHERE field 比较类型与字段 schema 不兼容时，系统应返回语义错误。
- When 查询包含 GROUP BY tag 时，系统应在计划中保留分组键，并在 explain 中输出 group pushdown 或 group retained。
- When 查询包含 GROUP BY time window 时，系统应执行窗口聚合，并拒绝非聚合窗口查询。
- When 查询包含 ORDER BY time asc 时，系统应保持时间升序输出。
- When 查询包含 ORDER BY time desc 时，系统应输出时间降序结果，并受预算限制。
- When 查询包含 LIMIT 时，系统应在流式输出达到限制后关闭上游 reader。
- When 查询包含 OFFSET 时，系统应跳过指定数量的行或样本后再输出。
- When 查询包含聚合函数 `mean` 时，系统应归一化为 `avg`。
- When 查询函数不支持字段类型时，系统应返回语义错误。
- When 查询 context 被取消时，系统应在 Builder 后的分析、计划、扫描、过滤、聚合、排序和输出阶段停止。
- When 查询生成 explain 时，系统应输出 logical plan、physical plan、pushdowns、post filters、order、group 和预算信息。
- When queryservice 执行查询时，系统应默认使用分层查询执行器，而不是仅调用兼容执行器。
- When 旧 `model.Query` 入口被调用时，系统应转换为 `QuerySpec` 并走同一分层主链路。
- When 查询超出 MaxShards、MaxParts 或 MaxSamples 时，系统应返回预算错误。
- When 查询结果为空时，系统应避免打开不必要的 value reader。
- When 查询完成或失败时，系统应记录 query stats 和 profile 信息。

## 验收标准

- Builder 单元测试覆盖 SELECT、FROM、WHERE、GROUP、ORDER、LIMIT、OFFSET、聚合和非法输入。
- Analyzer 单元测试覆盖字段存在性、类型兼容、函数兼容、窗口、分页、ORDER 和 WHERE 语义。
- Planner/Optimizer/Physical 单元测试断言新增 Filter、Group、Sort、Limit 节点和 pushdown 结果。
- Engine 查询测试覆盖 tag eq/in/exists/ne、field 比较、time range、order desc、limit/offset 和聚合窗口。
- QueryService 测试证明默认执行器走分层路径，CompatExecutor 仅保留兼容用途。
- `go test ./internal/querylang ./internal/queryanalyzer ./internal/queryplanner ./internal/queryoptimizer ./internal/queryphysical ./internal/queryexec ./internal/queryservice ./internal/engine . -count=1 -timeout 300s` 通过。
- `go test ./... -count=1 -timeout 10m` 和 `golangci-lint run ./...` 通过。
