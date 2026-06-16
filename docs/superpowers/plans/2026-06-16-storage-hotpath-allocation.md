# Storage Hotpath Allocation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 降低存储层查询与落盘热路径中的临时 `map`、临时 slice 和 syscall 开销。

**Architecture:** 保持现有 SSTable 二进制格式不变，只优化内存算法和 block I/O 辅助类型。查询合并采用有序快路径，乱序输入保留正确性兜底。

**Tech Stack:** Go、pprof、现有 `internal/engine`、`internal/sstable`、`tests/e2e`。

---

### Task 1: 规格与计划

**Files:**
- Create: `docs/superpowers/specs/2026-06-16-storage-hotpath-allocation-design.md`
- Create: `docs/superpowers/plans/2026-06-16-storage-hotpath-allocation.md`

- [x] **Step 1: 写入设计文档**

记录本轮优化边界：查询合并、block 读写、时间戳收集，不修改存储格式。

- [x] **Step 2: 写入实施计划**

按测试、实现、验证拆分任务，并在执行过程中更新状态。

### Task 2: 查询合并快路径

**Files:**
- Modify: `internal/engine/engine_test.go`
- Modify: `internal/engine/shard.go`

- [x] **Step 1: 增加有序合并测试**

覆盖多列同 key、有序样本、相同 timestamp 取最大 `WriteSeq`、结果按 timestamp 排序。

- [x] **Step 2: 确认测试失败**

运行：`go test -count=1 ./internal/engine -run 'TestMergeColumnData' -timeout 180s`

- [x] **Step 3: 实现低 map 分配合并**

按 column key 排序后处理连续分组；有序样本走 k-way 归并，乱序样本走 map fallback。

- [x] **Step 4: 确认测试通过**

运行：`go test -count=1 ./internal/engine -run 'TestMergeColumnData' -timeout 180s`

实现备注：有序合并测试先失败于 `49 allocs/run`，改为按 key 原地排序、连续分组、单列/双列/多列有序归并后通过，阈值为 `<=20 allocs/run`。补充了单列有序重复 timestamp 的 LWW 测试。

### Task 3: SSTable block 读写复用

**Files:**
- Modify: `internal/sstable/internal_test.go`
- Modify: `internal/sstable/block.go`
- Modify: `internal/sstable/read.go`
- Modify: `internal/sstable/write.go`

- [x] **Step 1: 增加 block buffer 和 writer 测试**

覆盖 read payload 释放、释放后 clone payload 仍可用、顺序 writer offset 正确。

- [x] **Step 2: 确认测试失败**

运行：`go test -count=1 ./internal/sstable -run 'TestBlock' -timeout 180s`

- [x] **Step 3: 实现 read buffer 与 blockWriter**

读取路径 decode 完成后释放 frame；写入路径使用 `blockWriter` 顺序偏移。

- [x] **Step 4: 确认测试通过**

运行：`go test -count=1 ./internal/sstable -run 'TestBlock|TestPart' -timeout 180s`

实现备注：新增 `readBlockPayloadFrom` 和 `blockWriter` 后定向测试通过。第一次 pprof 暴露 `releaseBlockFrame` 装箱分配，随后改为池化 `*blockFrame` 句柄，移除该热点。

### Task 4: 写入分组和时间戳收集

**Files:**
- Modify: `internal/sstable/internal_test.go`
- Modify: `internal/sstable/convert.go`

- [x] **Step 1: 增加有序稀疏时间戳测试**

覆盖多列有序稀疏样本不依赖 map 也能输出去重后的有序时间戳。

- [x] **Step 2: 确认测试失败**

运行：`go test -count=1 ./internal/sstable -run 'TestCollectTimestamps|TestGroupColumns' -timeout 180s`

- [x] **Step 3: 实现排序分组和线性时间戳归并**

`groupColumns` 排序后按 series 聚合；`collectTimestamps` 对有序样本线性归并，乱序兜底。

- [x] **Step 4: 确认测试通过**

运行：`go test -count=1 ./internal/sstable -run 'TestCollectTimestamps|TestGroupColumns' -timeout 180s`

实现备注：有序稀疏时间戳测试先失败于 `14 allocs/run`，线性归并后通过，阈值为 `<=8 allocs/run`。`writeColumns` 已切到连续 series 分组，兼容 `groupColumns` 保持不修改调用方输入。

### Task 5: 性能复测与质量门禁

**Files:**
- Modify: `docs/benchmarks/storage-engine-phase10.md`

- [x] **Step 1: 运行定向测试**

运行：`go test -count=1 ./internal/engine ./internal/sstable -timeout 180s`

实现备注：定向测试已通过，并执行 100K wide10 query pprof。总耗时约 `3.20s`，alloc_space 约 `1.45GB`，alloc_objects 约 `6.89M`。

- [x] **Step 2: 运行格式化和 lint**

运行：`goimports-reviser -project-name codeberg.org/mts/mts -recursive -format -rm-unused .`
运行：`golangci-lint run --timeout 12m`

实现备注：`goimports-reviser` 已完成，`golangci-lint run --timeout 12m` 输出 `0 issues.`。

- [x] **Step 3: 运行全量测试和覆盖率**

运行：`go test -count=1 ./... -coverprofile=coverage.out -timeout 600s`
验收：总覆盖率不低于 90%。

实现备注：全量测试通过，总覆盖率 `90.0%`。补充 fallback 分支测试后满足覆盖率门禁。

- [x] **Step 4: 运行 e2e**

按 `tests/e2e/*` 逐个 `go build` 和运行，完成后清理构建产物。

实现备注：`compaction_integrity`、`flush_manifest_recovery`、`no_json_storage`、`query_pruning`、`retention`、`simple_integrity`、`wal_recovery` 均已 build/run 通过。已清理 e2e 二进制、`coverage.out` 和本轮 `/tmp` pprof 产物。

### Task 6: SSTable index streaming 直读

**Files:**
- Modify: `internal/codec/envelope.go`
- Modify: `internal/codec/codec_test.go`
- Modify: `internal/sstable/metadata_encoding.go`
- Modify: `internal/sstable/series_reader.go`
- Modify: `internal/sstable/internal_test.go`

- [x] **Step 1: 增加 envelope view 与 QuerySeriesIDs 直读测试**

覆盖 `UnmarshalEnvelopeView` 不复制 payload、`QuerySeriesIDs` 在 index 损坏时返回错误、字段过滤只读取命中字段对应 value block。

- [x] **Step 2: 实现 envelope 只读 view**

保留 `UnmarshalEnvelope` 的复制语义，新增只在临时 decode 中使用的 `UnmarshalEnvelopeView`。

- [x] **Step 3: 实现 index row streaming decoder**

逐行读取 index header；不命中 series/time 的行直接跳过 column refs；命中行只保留查询字段需要的 column refs。

- [x] **Step 4: 切换 Part.QuerySeriesIDs**

大候选 direct fallback 不再调用 `loadIndexRows`，避免全量 `[]indexRow` 和 `[]columnRef` 物化。

实现备注：`UnmarshalEnvelopeView` 保留 payload view，普通 `UnmarshalEnvelope` 继续复制；`Part.Query`、`Part.SeriesIDs`、`Part.QuerySeriesIDs` 已切到 index stream，未命中 row 仅跳过 column refs，字段过滤只保留命中 refs。stream 查询复用 `[]columnRef` scratch，1M compact pprof 中 `readFilteredColumnRefs` 已从 top alloc 移除，`QuerySeriesIDs` 累计 alloc 从约 `3423 MB` 降至约 `3149 MB`。定向测试 `go test -count=1 ./internal/codec ./internal/sstable ./internal/memtable -timeout 180s` 已通过。

### Task 7: MemTable reserve 增长策略

**Files:**
- Modify: `internal/memtable/memtable.go`
- Modify: `internal/memtable/internal_test.go`

- [x] **Step 1: 调整容量测试**

容量验收从精确值改为满足容量、支持增长留量，避免下一批写入频繁小扩容。

- [x] **Step 2: 实现自适应增长**

小增量采用受限留量增长，大批量一次性写入保持接近目标容量，降低 RSS 过度膨胀。

实现备注：`columnBuffer.reserve` 对小批增量增加最多 8 个 sample 的受限 slack，对超过当前容量的大批预留保持目标容量，避免 wide 写入场景成千上万列同时翻倍。1M wide10 写入复测 RSS 峰值 `238688 KB`，低于本轮错误翻倍策略的 `302368 KB`，也低于前序记录约 `245128 KB`；耗时 `13.46s`。定向测试已覆盖增长留量。

### Task 8: 剩余 compaction 与 page index 热点

**Files:**
- Create: `internal/memtable/pool.go`
- Modify: `internal/memtable/memtable.go`
- Modify: `internal/memtable/internal_test.go`
- Modify: `internal/engine/lifecycle.go`
- Modify: `internal/engine/engine_test.go`
- Modify: `internal/sstable/encoding.go`
- Modify: `internal/sstable/read.go`
- Modify: `internal/sstable/internal_test.go`

- [x] **Step 1: 复核剩余 pprof 热点**

确认剩余可处理项为 compaction 中间分组、value page index materialize、MemTable table map 重建。列对象池化经过复测会抬高 RSS，已剔除。

- [x] **Step 2: 实现 compaction 回调分组**

`writeStreamingCompactionOutputsLocked` 直接通过 `forEachCompactionSeriesGroup` 消费 series group，不再构造 `[][]ColumnData`。

- [x] **Step 3: 实现 value page index streaming 读取**

热路径改为 `readValuePagesFromIndexPayload`，两遍扫描 index payload 计算命中 page 容量并读取命中 page，不再构造 `valuePageIndex.Pages`。

- [x] **Step 4: 实现 MemTable tableData 复用**

`New` 和 `SnapshotAndReset` 复用 table map；`Snapshot.Release` 清空并回收小表 map。列对象池化经复测 RSS 不合格，已撤销。

- [x] **Step 5: 验证与复测**

验证：`go test -count=1 ./... -coverprofile=coverage.out -timeout 600s` 总覆盖率 `90.0%`；`golangci-lint run --timeout 12m` 输出 `0 issues.`；`tests/e2e/*` 全部 build/run 通过。

性能：最终 1M wide10 写入 `13.43s`，RSS `231020 KB`，SSTable `2`，落盘 `93.8 MB`；最终 1M compact `52.46s`，RSS `151412 KB`，SSTable `1`，落盘 `89.0 MB`。compact alloc_space 从上一轮约 `15.25 GB` 降至约 `14.49 GB`，`unmarshalValuePageIndex` 不再出现在热点中。
