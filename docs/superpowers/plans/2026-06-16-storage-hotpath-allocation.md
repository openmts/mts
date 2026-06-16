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
