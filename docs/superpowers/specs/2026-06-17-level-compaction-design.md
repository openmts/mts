# Level Compaction 设计

## 背景

当前 mts 的 compaction 已经具备 streaming executor：按 `seriesID` 从多个 SSTable Part 读取、归并、应用 tombstone，并通过 `PartWriter` 分片写出，避免一次性物化全部列数据。但 planner 仍是单层逻辑：只检查 L0 的 part 数或总大小，并固定输出到 L1。随着写入量增加，L1 及以上层级不会继续合并，查询需要扫描越来越多 Part，空间放大和读放大都会累积。

本阶段目标是在不改变 SSTable 二进制主体格式的前提下，实现可配置的 leveled compaction：支持 L0、L1、L2 等层级，支持每层触发阈值、输出 Part 目标大小和输出压缩配置，并保持 manifest 原子替换与 crash-safe 行为。

## 开源调研结论

Prometheus TSDB 的 `LeveledCompactor` 使用指数型 block range 规划 compaction，优先处理重叠 block，再按时间范围选择可合并 block；输出 block 的 compaction level 基于输入最大 level 加一。它的启发是：planner 与 executor 分离，planner 只决定“哪些输入可以合并”，executor 负责原子产出新 block。

InfluxDB 1.8 TSM 的 `DefaultPlanner` 暴露 `PlanLevel(level int)`，按 generation 与 level 分组，低层级达到足够数量后向上合并，并用 `filesInUse` 防止同一文件被多个计划重复选中。它的启发是：层级阈值应该逐层可配置，手动 full compaction 和后台 level compaction 应使用同一套选择规则。

VictoriaMetrics mergeset 不是传统 L0/L1/L2，但它的 part merge 策略非常适合 mts 借鉴：后台 worker 只挑选大小接近、写放大可接受的 part；新 part 写完后再原子替换旧 part；旧 part 等引用释放后删除。mts 本阶段先保持单 shard 锁下执行，重点借鉴“选择小而相近的 part、按输出大小切分、原子替换”的原则。

## 方案对比

### 方案 A：保留 L0-only，扩大单次输出

优点是改动最小，复用当前代码。缺点是 L1 以上会持续堆积，查询读放大不会收敛，无法满足长期写入场景。

### 方案 B：按层级的 size-tiered leveled compaction

每层配置 `PartLimit`、`SizeLimit`、`MaxOutputPartBytes` 和 `Compression`。planner 从低到高扫描，当前层超过阈值时选择该层全部或一组大小接近的 Part，输出到下一层。优点是和当前 streaming executor 兼容，能快速降低 part 数量，避免大内存中间态；缺点是同层时间范围不保证完全不重叠，查询仍需依赖已有 Part 级时间/series/field 裁剪。

### 方案 C：严格 RocksDB 风格 leveled compaction

L0 特殊处理重叠，L1+ 保证每层 key range 不重叠，并按 target size 与 overlapping range 挑选下一层输入。优点是读放大最优；缺点是需要完整 key range 管理、score 计算、跨层 overlap 选择和并发 compaction 隔离，当前 mts 的 series/time 组合键与 Part 元数据还不足以低风险完成。

## 选择

采用方案 B。它符合当前存储结构和用户对“本层级压缩，达到上限后转移到下一层”的要求，也能直接复用已优化过的 streaming executor。方案 C 的严格 non-overlap leveled 策略可在后续 Part 元数据增加更细 key range 后演进，但本阶段不引入弱实现。

## EARS 需求

- When compaction is enabled and a level exceeds its part limit, the system shall compact candidate SSTable Parts from that level into the next level.
- When compaction is enabled and a level exceeds its size limit, the system shall compact candidate SSTable Parts from that level into the next level.
- When a level compaction writes output Parts, the system shall use the output level's `MaxOutputPartBytes` to roll output files.
- When a level defines compression options, the system shall use that level's compression options for output Parts.
- When a level does not define compression options, the system shall inherit the engine-level compression options.
- When manual `Compact()` is called, the system shall flush MemTable first and then drain all eligible levels until no plan remains or the cascade step limit is reached.
- When background compaction runs after flush, the system shall execute bounded cascading compaction to avoid unbounded foreground stalls.
- When compaction fails before manifest commit, the system shall keep the old manifest and old Parts visible.
- When manifest commit succeeds, the system shall replace old Parts with new Parts atomically in memory and on disk, then close and remove old Parts.
- When tombstones are compacted into output Parts, the system shall clear compacted tombstones and checkpoint WAL only after manifest replacement succeeds.
- When the engine restarts, the system shall load existing Part levels from manifest and continue planning from those levels.

## 配置模型

保留旧字段用于兼容当前测试和调用方：

- `Level0PartLimit`
- `Level0SizeLimit`
- `MaxOutputPartBytes`

新增 `Levels []CompactionLevelOptions`。每个 level 配置包含：

- `Level int`
- `PartLimit int`
- `SizeLimit int64`
- `MaxOutputPartBytes int64`
- `Compression CompressionOptions`

归一化规则：

- 未配置 `Levels` 时，根据旧字段生成默认 L0 配置。
- 已配置 `Levels` 时，按 `Level` 排序并补齐缺失值。
- L0 默认 `PartLimit=4`。
- L1+ 默认 `PartLimit=4`。
- 未配置 `MaxOutputPartBytes` 时继承全局 `MaxOutputPartBytes`。
- 未配置层级压缩时继承 `Options.Compression`。

## Planner 行为

planner 在持有 shard 生命周期锁时基于 manifest 快照工作：

1. 按 level 从低到高扫描。
2. 找出当前 level 的 Part。
3. 计算 Part 数量与目录总大小。
4. 若未超过阈值且没有 tombstone 需要清理，则跳过。
5. 选择当前 level 的候选 Part，输出 level 为 `level + 1`。
6. 每次执行一个 plan，执行成功后重新基于新 manifest 规划下一步。

后台 `maybeCompactLocked()` 使用 `MaxCascadeSteps` 限制单次触发后的级联次数。手动 `Compact()` 也使用同一上限，默认值足够覆盖常规 L0 到 L4 的级联，防止异常配置导致长时间持锁。

## Executor 行为

执行器继续复用当前 per-series streaming compaction：

- 从输入 Part 收集 seriesID 并去重。
- 对每个 seriesID 查询所有输入 Part。
- 归并相同 `(seriesID, fieldID, timestamp)`，保留最大 `writeSeq`。
- 应用 tombstone。
- 按输出 level 配置决定压缩算法和 roll 大小。
- 写出一个或多个新 Part。
- manifest 提交成功后替换内存 part 列表。

## 测试策略

- 单元测试覆盖配置归一化、旧字段映射、逐层 compression 选择、planner 触发条件。
- engine 测试覆盖 L0 到 L1、L1 到 L2 级联、手动 compact drain、重启后 level 保持。
- e2e compaction integrity 覆盖写入、flush、多层 compaction、查询正确性和 manifest level。
- pprof 工具暴露 level 配置参数，便于对比不同阈值下 SSTable 数量、写入耗时和 RSS。

## 边界

本阶段不改变 SSTable 文件主体格式，不增加跨 shard compaction，不引入并行 compaction worker。当前 shard 生命周期锁保证同一 shard 内不会并发改 manifest；后续如果增加并行 compaction，需要引入类似 InfluxDB `filesInUse` 的计划占用机制。
