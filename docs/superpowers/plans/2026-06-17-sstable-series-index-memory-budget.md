# SSTable Series Index And Memory Budget Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 SSTable 增加 seriesID 二级索引，并为存储层增加总 MemTable 样本阈值保护。

**Architecture:** SSTable 写入阶段生成 `series_index.bin`，查询带 series 过滤时通过二级索引直达 index row。Engine 写入后根据全局 MemTable 样本总量执行软阈值 flush 和硬阈值拒写。

**Tech Stack:** Go、现有 `codec` envelope、二进制 blockRef、现有 engine/shard/memtable 接口。

---

### Task 1: SSTable 二级索引测试

**Files:**
- Modify: `internal/sstable/internal_test.go`

- [x] 写失败测试：单 series 查询应只读取目标 index row block，并返回正确列。
- [x] 写失败测试：不存在的 series 查询应返回空结果。
- [x] 运行 `go test ./internal/sstable -run 'TestPartSeriesIndex' -timeout 180s`，预期失败。

### Task 2: SSTable 二级索引实现

**Files:**
- Modify: `internal/sstable/types.go`
- Modify: `internal/sstable/write.go`
- Modify: `internal/sstable/stream_write.go`
- Modify: `internal/sstable/metadata_encoding.go`
- Modify: `internal/sstable/read.go`
- Modify: `internal/sstable/scan.go`

- [x] 新增 `series_index.bin` 常量、metadata 引用和 series index row 类型。
- [x] 写 part 时按 series 写独立 index row block，并写入 series index。
- [x] metadata 编码追加 series index blockRef。
- [x] 查询带 `SeriesIDs` 时通过 series index 定位 row block。
- [x] 保持无 series 过滤查询的流式扫描行为。

### Task 3: 存储内存阈值测试

**Files:**
- Modify: `internal/engine/engine_test.go`
- Modify: `types_test.go`

- [x] 写失败测试：软阈值触发 flush 并产生 SSTable。
- [x] 写失败测试：硬阈值小于单批写入样本数时返回错误。
- [x] 写失败测试：公共 `mts.Options` 和内部 `model.Options` 字段保持一致。

### Task 4: 存储内存阈值实现

**Files:**
- Modify: `internal/model/types.go`
- Modify: `types.go`
- Modify: `internal/engine/paths.go`
- Modify: `internal/engine/engine.go`
- Modify: `internal/engine/shard.go`
- Modify: `internal/engine/ports.go`

- [x] 新增 `StorageMemoryOptions`。
- [x] 为 memStore 暴露 `ApproxMemorySamples`。
- [x] Engine 写入后统计所有 shard MemTable 样本总量。
- [x] 达到软阈值时执行全局 flush。
- [x] flush 后仍超过硬阈值时返回错误。

### Task 5: 验证与收尾

**Files:**
- Modify: `docs/superpowers/plans/2026-06-17-sstable-series-index-memory-budget.md`

- [x] 运行 `go test ./internal/sstable ./internal/engine -timeout 5m`。
- [x] 运行 `go test ./... -timeout 10m`。
- [x] 运行 `gofmt`，如环境可用运行 `goimports-reviser` 和 `golangci-lint`。
- [x] 清理临时产物。

**实现备注：**
- `go test ./... -timeout 10m` 通过。
- `goimports-reviser -rm-unused -format ./...` 通过。
- `golangci-lint run --timeout 10m` 通过，输出 `0 issues.`。
- 1M wide10 pprof 读取采样从历史 `22846ms/1000 queries` 降到 `2987ms/1000 queries`，约提升 `7.65x`；落盘从约 `130.8MB` 增至约 `143.5MB`。
