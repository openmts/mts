# Storage Remaining Hotpath Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 一次性完成剩余存储热路径优化，减少 SSTable 解码、Catalog resolve、MemTable 查询和 pprof 观测中的临时分配与不可观测指标缺口。

**Architecture:** 不改变持久化格式和公开 API。SSTable 通过直接填充 samples 减少中间切片；Catalog 复用 key scratch；MemTable 查询在读锁内直接扫描；page index 对全量命中走单遍；pprof 工具补齐运行指标。

**Tech Stack:** Go、现有 mts 存储层、pprof、`go test`、`goimports-reviser`、`golangci-lint`。

---

### Task 1: SSTable Indexed Page Streaming

**Files:**
- Modify: `internal/sstable/encoding.go`
- Modify: `internal/sstable/encoding_test.go`
- Modify: `internal/sstable/compression.go`
- Modify: `internal/sstable/compression_test.go`

- [x] **Step 1: 增加 indexed page allocation 测试**
  - 在 `internal/sstable/encoding_test.go` 增加 indexed plain page 查询测试，使用 `testing.AllocsPerRun` 约束 indexed float 查询分配不随 values 中间切片增长。
- [x] **Step 2: 运行失败测试**
  - Run: `go test -count=1 ./internal/sstable -run 'TestValuePageIndexedStreamingAllocations' -timeout 180s`
  - Expected: 失败，因为当前 indexed path 会分配 timestamps/writeSeqs/samples。
- [x] **Step 3: 实现 indexed direct decoder**
  - 新增 `readIndexedSamples`、`readIndexedOrdinalsInto`、`fillIndexedSampleValues`，直接追加 query 命中的 samples。
- [x] **Step 4: 压缩 plain codec 直接填充**
  - 新增 plain codec 填充 helper，compressed page plain codec 不再先构造 `[]FieldValue`。
- [x] **Step 5: 运行定向测试**
  - Run: `go test -count=1 ./internal/sstable -run 'TestValuePage|TestCompressed' -timeout 180s`
  - Result: 通过；同时 `TestValuePageIndexedStreamingAllocations|TestCompressedPlainValueCodecStreamsSamples` 通过。

### Task 2: Catalog Multi-Tag Key Scratch

**Files:**
- Modify: `internal/catalog/key.go`
- Modify: `internal/catalog/resolve.go`
- Modify: `internal/catalog/internal_test.go`

- [x] **Step 1: 增加多 tag key allocation 测试**
  - 在 `internal/catalog/internal_test.go` 增加 `TestSeriesKeyMultiTagUsesSingleAllocationClass`，要求多 tag key 构造分配下降。
- [x] **Step 2: 运行失败测试**
  - Run: `go test -count=1 ./internal/catalog -run 'TestSeriesKeyMultiTagUsesSingleAllocationClass' -timeout 180s`
- [x] **Step 3: 实现 scratch key builder**
  - `resolveBatchCache` 增加 `tagKeys []string`。
  - `seriesKeyWithScratch(measurement, tags, scratch)` 使用 `strings.Builder` 直接构造最终 key。
- [x] **Step 4: 接入 batch resolve**
  - `resolveSeriesNoSnapshotCachedLocked` 使用 cache scratch；单点路径使用无 scratch helper。
- [x] **Step 5: 运行定向测试**
  - Run: `go test -count=1 ./internal/catalog -run 'TestSeriesKey|TestResolvePoints' -timeout 180s`
  - Result: 通过；`TestSeriesKeyMultiTagUsesSingleAllocationClass` 通过且 `<=1` alloc/run。

### Task 3: MemTable Query Without Full Clone

**Files:**
- Modify: `internal/memtable/memtable.go`
- Modify: `internal/memtable/memtable_test.go`
- Modify: `internal/memtable/internal_test.go`

- [x] **Step 1: 增加 Query 不调用 cloneData 的测试**
  - 在 internal test 中增加 clone 计数 hook 或 allocation 测试，证明 `MemTable.Query` 不再整表 clone。
- [x] **Step 2: 运行失败测试**
  - Run: `go test -count=1 ./internal/memtable -run 'TestMemTableQueryAvoidsSnapshotClone' -timeout 180s`
- [x] **Step 3: 提取 columnsFromData**
  - `Snapshot.Columns` 和 `MemTable.Query` 共享 `columnsFromData(data, query)`。
- [x] **Step 4: 改造 MemTable.Query**
  - `MemTable.Query` 持有 RLock 直接扫描 `m.data`，不调用 `Snapshot()`。
- [x] **Step 5: 运行定向测试**
  - Run: `go test -count=1 ./internal/memtable -timeout 180s`
  - Result: 通过；实时 Query 不调用 `cloneData`，并复制返回样本避免共享内部 slice。

### Task 4: Value Page Index Adaptive Scan

**Files:**
- Modify: `internal/sstable/read.go`
- Modify: `internal/sstable/internal_test.go`

- [x] **Step 1: 增加全量 page index 单遍测试**
  - 构造覆盖全部 page 的 query，断言只扫描一次 page refs。
- [x] **Step 2: 运行失败测试**
  - Run: `go test -count=1 ./internal/sstable -run 'TestValuePageIndexFullRangeUsesSinglePass' -timeout 180s`
- [x] **Step 3: 实现全量覆盖识别**
  - 读取 header 后判断 query 覆盖 page index min/max，命中时单遍读取所有 page。
- [x] **Step 4: 保留窄查询两遍路径**
  - 部分命中仍调用 `matchingValuePageIndexHeader` 预估容量。
- [x] **Step 5: 运行定向测试**
  - Run: `go test -count=1 ./internal/sstable -run 'TestValuePageIndex|TestPart' -timeout 180s`
  - Result: 通过；full-range 复用首轮 refs 扫描，部分命中保留容量预估路径。

### Task 5: Pprof Metrics

**Files:**
- Modify: `tests/pprof/storage_engine/main.go`
- Modify: `tests/pprof/storage_engine/main_test.go`
- Optional Modify: `tests/pprof/README.md`

- [x] **Step 1: 增加 metrics 测试**
  - 测试 `collectRunMetrics` 返回 `TotalAlloc`、`Mallocs`、`NumGC` 字段，Linux 下 RSS 字段允许为 0 但解析函数必须可测。
- [x] **Step 2: 运行失败测试**
  - Run: `go test -count=1 ./tests/pprof/storage_engine -run 'TestCollectRunMetrics' -timeout 180s`
- [x] **Step 3: 扩展 runMetrics**
  - 增加 GC、总分配、malloc/free、RSS/VmHWM 字段。
- [x] **Step 4: 输出稳定 metrics 行**
  - log 行追加新增字段，保持现有字段名不变。
- [x] **Step 5: 运行定向测试**
  - Run: `go test -count=1 ./tests/pprof/storage_engine -timeout 180s`
  - Result: 通过；补齐 `heap_total_alloc_bytes`、`mallocs`、`frees`、`num_gc`、`pause_total_ns`、`rss_bytes`、`rss_peak_bytes`。

### Task 6: 集成验证

**Files:**
- Modify: `docs/superpowers/plans/2026-06-17-storage-remaining-hotpath.md`

- [x] **Step 1: 运行 goimports-reviser**
  - Run: `timeout 300s goimports-reviser -project-name codeberg.org/mts/mts -recursive -format -rm-unused .`
  - Result: 通过。
- [x] **Step 2: 运行核心包测试**
  - Run: `go test -count=1 ./internal/wal ./internal/sstable ./internal/catalog ./internal/memtable ./internal/engine ./tests/pprof/storage_engine -timeout 180s`
  - Result: 通过。
- [x] **Step 3: 运行全量测试与覆盖率**
  - Run: `go test -count=1 ./... -coverprofile=coverage.out -timeout 600s`
  - Run: `go tool cover -func=coverage.out | tail -1`
  - Result: 通过；总覆盖率 90.0%。
- [x] **Step 4: 运行 lint**
  - Run: `golangci-lint run --timeout 12m`
  - Result: 通过，0 issues。
- [x] **Step 5: 逐个运行 e2e**
  - Run: 对 `tests/e2e/*` 逐个 `go build` 和运行，完成后删除二进制。
  - Result: 实际 7 个 e2e 目录全部 build/run 通过并清理二进制。
- [x] **Step 6: 清理临时产物**
  - 删除 `coverage.out`、e2e binary、临时 profile。
  - Result: 已清理 `coverage.out`、`/tmp/mts-sstable-mem.out` 和 e2e `.testbin`。

## 自检

- Spec coverage：Task 1 覆盖 SSTable streaming，Task 2 覆盖 Catalog key，Task 3 覆盖 MemTable query，Task 4 覆盖 page index 自适应，Task 5 覆盖 pprof metrics，Task 6 覆盖验证。
- Placeholder scan：无 TBD/TODO/后续增强占位。
- Type consistency：不改变公开 API，不新增持久化格式版本。
