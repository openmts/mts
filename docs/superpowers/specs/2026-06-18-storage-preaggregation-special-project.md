# 存储层预聚合专项规格

## 状态

本专项当前只完成设计与任务拆解，暂不进入实现。原因是该能力会改变 SSTable 文件结构、查询执行路径和 LSM 一致性判断，对当前存储层属于高风险能力。实施前必须先确认查询语义、文件格式治理、回退策略和一致性验证矩阵已经充分成熟。

## 目标

在存储层为聚合查询提供可验证的预计算能力，通过 page-level 统计摘要降低大范围聚合查询的 value page 读取量。所有预聚合结果必须与现有样本扫描聚合结果一致；只要无法证明安全命中，就必须回退现有查询路径。

## 非目标

- 本专项不在当前阶段实现代码。
- 本专项不把近似统计结果伪装成精确结果。
- 本专项不为了聚合性能牺牲 LSM 覆盖写、tombstone、乱序写和重启恢复的一致性。
- 本专项不在第一阶段实现全量 rollup materialized view 或 continuous aggregate。

## 参考系统

- InfluxDB：提供 `count/sum/mean/median/mode/min/max/first/last/stddev/spread/difference/derivative/moving_average` 等函数，常通过 continuous query 或 downsampling 做物化聚合。
- Prometheus：以 range-vector 函数为核心，包括 `rate/irate/increase/delta/deriv/changes/avg_over_time/min_over_time/max_over_time/sum_over_time/count_over_time/stddev_over_time/quantile_over_time`。
- VictoriaMetrics：MetricsQL rollup 函数更丰富，包括 `rollup_rate/rollup_delta/rate/irate/increase/delta/mode_over_time/median_over_time/stddev_over_time/range_*`。
- TimescaleDB：continuous aggregates 通过物化结果、invalidation 和 refresh 控制一致性。
- IoTDB：提供面向时序列的 `count/sum/avg/max/min/first/last/max_time/min_time/extreme/mode` 等内置聚合。

## 函数分层

### 精确可合并函数

这些函数可通过 page stats 精确合并：

- `count`
- `sum`
- `avg` / `mean`
- `min`
- `max`
- `spread` / `range`
- `stddev` / `stdvar`
- `first`
- `last`
- `min_time`
- `max_time`
- `present`

### 需要边界样本的函数

这些函数可通过 page stats 辅助，但必须记录边界样本和变化计数：

- `difference`
- `non_negative_difference`
- `delta`
- `increase`
- `rate`
- `irate`
- `derivative` / `deriv`
- `changes`
- `resets`

### 需要受控频率结构的函数

这些函数只能在 exact frequency 未超过配置阈值时命中：

- `mode`
- `distinct`
- `count_values`

当 distinct 数超过阈值时，系统必须标记 stats 不可用，并回退样本扫描。

### 暂不作为精确预聚合目标的函数

这些函数需要分布结构或排序结构，第一阶段只预留扩展位置，不返回近似结果：

- `median`
- `quantile`
- `histogram_quantile`
- `mad`
- `topk`
- `bottomk`

## EARS 需求

- 当 SSTable 写入 value page 时，系统应为该 page 生成可校验的聚合统计摘要。
- 当字段类型是 `float64` 或 `int64` 时，系统应生成 `count/sum/avg/min/max/spread/stddev/stdvar/first/last/difference/rate/irate/changes/resets` 所需摘要。
- 当字段类型是 `string` 或 `bool` 时，系统应只生成语义正确的 `count/first/last/mode/changes` 摘要。
- 当 `mode` 的 distinct 数超过配置阈值时，系统应标记 `modeAvailable=false`，查询应回退样本扫描。
- 当查询范围完整覆盖 value page 且没有一致性风险时，系统应使用 page stats 计算聚合结果。
- 当查询范围只部分覆盖 value page 时，系统应回退读取该 page 的原始样本。
- 当查询涉及 tombstone、MemTable 未落盘数据、跨 level 覆盖写或重叠 part 时，系统应回退样本扫描。
- 当预聚合结果与现有样本扫描结果不一致时，系统应把该情况视为正确性缺陷。
- 当预聚合快路径生效时，系统应在 query stats 中记录 stats 命中、覆盖 page 数和回退次数。
- 当 SSTable 文件损坏或 stats 校验失败时，系统应拒绝使用该 stats，并返回可诊断错误或回退到样本扫描。

## 一致性原则

1. 预聚合摘要只是一种加速索引，不是事实源；事实源仍是 WAL、MemTable、SSTable 原始样本。
2. 快路径必须先证明安全，再使用 stats。
3. 对不能证明安全的查询，回退样本扫描是正确行为。
4. Compaction 输出 stats 时必须以合并后的可见样本为输入，不能直接盲目合并输入 part stats。
5. 所有预聚合函数必须具备与样本扫描结果逐项对照的测试。

## 暂缓实施原因

当前存储层正在补齐元数据接口化、文件格式治理、compaction 观测和长期压测能力。预聚合会引入新的 SSTable 元数据格式、查询计划分支和一致性证明逻辑，当前立即实施会扩大存储层风险面。因此本专项先作为明确的未来能力储备，等底层格式与查询执行器进一步稳定后再启动。
