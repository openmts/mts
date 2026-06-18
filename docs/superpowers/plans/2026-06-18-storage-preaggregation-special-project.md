# Storage Preaggregation Special Project Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 mts 存储层建立 page-level 预聚合能力，在保证数据完整性和一致性的前提下减少大范围聚合查询读取量。

**Architecture:** SSTable 写入时生成 page-level aggregate stats；查询 planner 在证明安全时走 stats 聚合快路径，否则回退现有样本扫描。Compaction 以合并后的可见样本重建 stats，避免 LSM 覆盖写和 tombstone 破坏聚合正确性。

**Tech Stack:** Go、`internal/sstable`、`internal/engine`、`internal/queryexec`、二进制 SSTable metadata、现有 query stats 和 e2e 测试框架。

---

## 当前决策

本专项当前仅拆解任务，不进入实现阶段。实施前需要重新确认存储层文件格式治理、查询执行器稳定性和长期一致性验证能力已经满足要求。

## 文件职责规划

- `internal/model/types.go`：扩展聚合函数枚举或规范化辅助类型，避免字符串散落。
- `internal/sstable/aggregate_stats.go`：定义 page-level stats、更新器、合并器和函数支持矩阵。
- `internal/sstable/aggregate_stats_encoding.go`：负责 stats 二进制编码、解码和校验。
- `internal/sstable/write.go`：在 value page 写入阶段生成 stats。
- `internal/sstable/metadata_encoding.go`：把 stats 写入 value page index。
- `internal/sstable/read.go`：提供 stats-only 扫描能力，不直接替代现有样本扫描。
- `internal/engine/query_plan.go`：识别聚合查询是否具备 stats 下推条件。
- `internal/engine/query_aggregate_stats.go`：执行跨 shard、跨 part 的安全聚合合并。
- `internal/queryexec/aggregate.go`：复用聚合语义，保证 stats 结果和样本扫描结果一致。
- `docs/storage/file-format.md`：记录新增 SSTable stats 编码格式。
- `docs/storage/metrics.md`：记录新增 query stats 和 metrics。

## 子任务清单

### Task 1: 聚合语义矩阵与函数规范

**Files:**
- Modify: `internal/model/types.go`
- Create: `internal/queryexec/aggregate_registry.go`
- Test: `internal/queryexec/aggregate_registry_test.go`
- Update: `docs/superpowers/specs/2026-06-18-storage-preaggregation-special-project.md`

- [ ] **Step 1: 定义聚合函数分类**

验收标准：所有函数被明确归入 `exact mergeable`、`boundary-assisted`、`frequency-limited`、`sample-scan-only` 四类。

- [ ] **Step 2: 统一函数名称规范化**

验收标准：`avg` 和 `mean` 被视为同义函数；`spread` 和 `range` 被视为同义函数；未知函数返回明确错误。

- [ ] **Step 3: 为每个函数定义支持字段类型**

验收标准：`float64/int64/string/bool` 的支持矩阵有单元测试覆盖。

### Task 2: Page Aggregate Stats 数据模型

**Files:**
- Create: `internal/sstable/aggregate_stats.go`
- Test: `internal/sstable/aggregate_stats_test.go`

- [ ] **Step 1: 定义 stats 结构**

验收标准：结构能表达 `count/sum/min/max/sumSquares/first/second/penultimate/last/changes/resets/mode` 所需信息。

- [ ] **Step 2: 实现样本流式更新器**

验收标准：不需要额外构造临时 map，除 mode frequency 以外只用常量级状态。

- [ ] **Step 3: 实现 stats 合并器**

验收标准：合并多个完整覆盖 page stats 后，结果与对原始样本整体聚合一致。

- [ ] **Step 4: 实现 mode frequency 上限策略**

验收标准：distinct 数不超过阈值时返回精确 mode；超过阈值时 `modeAvailable=false`。

### Task 3: Stats 二进制编码与格式治理

**Files:**
- Create: `internal/sstable/aggregate_stats_encoding.go`
- Modify: `internal/sstable/metadata_encoding.go`
- Test: `internal/sstable/aggregate_stats_encoding_test.go`
- Update: `docs/storage/file-format.md`

- [ ] **Step 1: 设计 stats payload 编码**

验收标准：编码包含 magic、flags、field type、count、数值摘要、边界样本、frequency 列表和 checksum 保护。

- [ ] **Step 2: 解码时校验字段类型和 payload 完整性**

验收标准：截断、非法类型、负 count、frequency 溢出都会返回错误。

- [ ] **Step 3: 文件格式文档更新**

验收标准：`docs/storage/file-format.md` 写清 value page stats 的字段顺序和适用范围。

### Task 4: SSTable 写入阶段生成 Stats

**Files:**
- Modify: `internal/sstable/write.go`
- Modify: `internal/sstable/types.go`
- Test: `internal/sstable/internal_test.go`

- [ ] **Step 1: value page 写入时生成 stats**

验收标准：每个 `valuePageRef` 带有对应 stats 引用或内联 stats。

- [ ] **Step 2: 空 page 和不支持类型处理**

验收标准：空 page 不生成可用 stats；不支持函数不影响正常写入。

- [ ] **Step 3: 压缩开启时保持 stats 独立可读**

验收标准：读取 stats 不需要解压 value page payload。

### Task 5: SSTable Stats-only 扫描接口

**Files:**
- Modify: `internal/sstable/types.go`
- Modify: `internal/sstable/read.go`
- Create: `internal/sstable/aggregate_scan.go`
- Test: `internal/sstable/aggregate_scan_test.go`

- [ ] **Step 1: 定义 aggregate scan query**

验收标准：查询包含 series filter、field filter、time range、aggregate specs 和安全约束。

- [ ] **Step 2: 完整覆盖 page 时返回 stats**

验收标准：查询不读取 value page payload，`ValuePagesRead` 不增长。

- [ ] **Step 3: 部分覆盖 page 时回退样本扫描**

验收标准：跨 page 边界查询能混合 stats 和样本结果，并保持最终结果正确。

### Task 6: LSM 一致性与安全命中判断

**Files:**
- Create: `internal/engine/query_aggregate_safety.go`
- Test: `internal/engine/query_aggregate_safety_test.go`

- [ ] **Step 1: 检测 MemTable 参与情况**

验收标准：查询范围内存在未落盘 MemTable 数据时，不使用全 stats 快路径。

- [ ] **Step 2: 检测 tombstone 影响**

验收标准：任一 tombstone 与查询范围相交时，对受影响列回退样本扫描。

- [ ] **Step 3: 检测 part/level 重叠**

验收标准：同一 `(seriesID, fieldID, time range)` 存在多个可见 part 重叠时，禁止盲目合并 stats。

- [ ] **Step 4: compaction 输出重建 stats**

验收标准：compaction 后输出 part 的 stats 来自去重和 tombstone 过滤后的可见样本。

### Task 7: Engine 聚合下推执行器

**Files:**
- Create: `internal/engine/query_aggregate_stats.go`
- Modify: `internal/engine/query.go`
- Modify: `internal/engine/shard_scan.go`
- Test: `internal/engine/query_aggregate_stats_test.go`

- [ ] **Step 1: planner 识别可下推聚合**

验收标准：只对受支持函数、字段类型、安全查询启用 stats 下推。

- [ ] **Step 2: 执行 stats 聚合快路径**

验收标准：`count/sum/avg/min/max/spread/stddev/first/last` 结果与样本扫描一致。

- [ ] **Step 3: 执行边界样本辅助函数**

验收标准：`difference/rate/irate/changes/resets` 在完整覆盖 page 且边界样本充分时结果正确。

- [ ] **Step 4: fallback 与现有 queryexec 聚合一致**

验收标准：任一不安全条件触发时，结果与当前样本扫描路径一致。

### Task 8: Query Stats 与 Metrics

**Files:**
- Modify: `internal/model/types.go`
- Modify: `internal/engine/query_stats.go`
- Modify: `internal/engine/metrics.go`
- Update: `docs/storage/metrics.md`
- Test: `internal/engine/query_stats_test.go`

- [ ] **Step 1: 增加 stats 命中指标**

验收标准：记录 `AggregateStatsRead`、`AggregatePagesCovered`、`AggregateFallbacks`。

- [ ] **Step 2: metrics 暴露聚合快路径指标**

验收标准：Prometheus text 中包含聚合 stats 命中和回退计数。

### Task 9: 一致性对照测试矩阵

**Files:**
- Create: `tests/e2e/preaggregation_integrity/main.go`
- Create: `tests/e2e/preaggregation_fallback/main.go`
- Test: `internal/engine/query_aggregate_stats_test.go`

- [ ] **Step 1: 对照样本扫描与 stats 快路径**

验收标准：对 `count/sum/avg/min/max/stddev/first/last/rate/irate/difference/mode` 做逐项结果对比。

- [ ] **Step 2: 覆盖乱序写和覆盖写**

验收标准：重复 timestamp 取最高 writeSeq，stats 快路径不得返回旧值。

- [ ] **Step 3: 覆盖 tombstone**

验收标准：删除范围内样本不参与聚合。

- [ ] **Step 4: 覆盖重启恢复**

验收标准：重启后 stats 可用性和聚合结果保持一致。

### Task 10: 性能与容量验证

**Files:**
- Modify: `tests/pprof/storage_engine/main.go`
- Create: `docs/benchmarks/storage-preaggregation.md`

- [ ] **Step 1: 增加聚合查询 pprof 模式**

验收标准：能分别输出 stats 快路径和样本扫描路径的 CPU、RSS、读 page 数和耗时。

- [ ] **Step 2: 记录存储空间增量**

验收标准：输出启用 stats 后 SSTable index/metadata 增量占比。

- [ ] **Step 3: 定义启动门槛**

验收标准：只有当聚合查询耗时显著降低且文件增量可控时，才建议进入实现。

## 依赖顺序

1. Task 1 必须先完成，因为后续所有编码和执行器都依赖函数语义矩阵。
2. Task 2 和 Task 3 完成后，才能修改 SSTable 写入格式。
3. Task 4 完成后，才能实现 stats-only 扫描。
4. Task 5 和 Task 6 必须同时通过，才能接入 Engine 查询下推。
5. Task 7 完成后必须立刻执行 Task 9 的一致性矩阵。
6. Task 10 只在正确性完全通过后执行，性能数据不能替代正确性验收。

## 暂缓实施验收

当前阶段的验收标准是：

- 专项规格已落地到 `docs/superpowers/specs/2026-06-18-storage-preaggregation-special-project.md`。
- 子任务计划已落地到 `docs/superpowers/plans/2026-06-18-storage-preaggregation-special-project.md`。
- 没有修改任何生产代码。
- 后续若恢复实施，应先从 Task 1 开始，不得跳过语义矩阵和一致性安全判断。
