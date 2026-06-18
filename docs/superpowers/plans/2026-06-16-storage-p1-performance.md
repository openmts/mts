# Storage P1 Performance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 一次性完成 compaction per-series streaming、SSTable value streaming decoder、WAL v3 字典化三项 P1 性能优化。

**Architecture:** 保持现有公开 API 与旧磁盘格式可读。Compaction 从 batch 列集合改为单 series 查询、归并、过滤、写出；SSTable v3 page 解码直接 append 样本，减少中间数组；WAL 保持 frame version 不变，新增 batch payload v3 并兼容 v2 replay。

**Tech Stack:** Go、LSM/SSTable、WAL 二进制编码、pprof、`go test`、`golangci-lint`、`goimports-reviser`。

---

### Task 1: Compaction Per-Series Streaming

**Files:**
- Modify: `internal/engine/lifecycle.go`
- Modify: `internal/sstable/series_reader.go`
- Modify: `internal/engine/engine_test.go`

- [x] **Step 1: 写失败测试**

新增测试覆盖跨 part、重复 timestamp、tombstone 后 compaction 结果正确，且 compaction 输出路径按单 series 调用。

- [x] **Step 2: 运行失败测试**

运行：`go test -count=1 ./internal/engine -run 'TestCompactionStreamsOneSeriesAtATime' -timeout 180s`

预期：失败，因为尚未提供单 series compaction 路径或测试 hook。

- [x] **Step 3: 实现单 series 查询**

新增 `SeriesBatchReader.QuerySeriesID(seriesID uint64)`，并在 engine compaction 中新增 `queryCompactionSeries`。

- [x] **Step 4: 改造输出循环**

`writeStreamingCompactionOutputsLocked` 遍历全局 seriesIDs，每次只生成当前 series 的 columns，merge、tombstone、write 后释放。

- [x] **Step 5: 运行定向测试**

运行：`go test -count=1 ./internal/engine ./internal/sstable -run 'TestCompactionStreamsOneSeriesAtATime|TestSeriesBatchReader' -timeout 180s`

验收：测试通过，现有 compaction 查询结果不变。

实现备注：新增 `SeriesBatchReader.QuerySeriesID` 与 `Shard.queryCompactionSeries`，`writeStreamingCompactionOutputsLocked` 改为遍历全局 seriesIDs 并逐个写出。已运行 `go test -count=1 ./internal/engine ./internal/sstable -run 'TestCompactionStreamsOneSeriesAtATime|TestSeriesBatchReader' -timeout 180s` 通过。

### Task 2: SSTable Value Streaming Decoder

**Files:**
- Modify: `internal/sstable/encoding.go`
- Modify: `internal/sstable/compression.go`
- Modify: `internal/sstable/internal_test.go`
- Modify: `internal/sstable/encoding_test.go`

- [x] **Step 1: 写失败测试**

新增 allocation 测试，要求 v3 plain value block 解码不再构造 typed values slice，并覆盖 bool page 直接输出样本。

- [x] **Step 2: 运行失败测试**

运行：`go test -count=1 ./internal/sstable -run 'TestUnmarshalValueBlockWithTimestampsStreamsSamples|TestReadBoolSamplesDirect' -timeout 180s`

预期：失败，因为当前路径仍通过多段中间数组组合样本。

- [x] **Step 3: 实现 sample appender**

新增 `sampleAppender`、`readSamplesInto`、`readBoolSamplesInto`，v3 plain page 直接 append 到结果样本切片。

- [x] **Step 4: 保持兼容路径**

v2 与 v5 解码保留兼容；v5 可复用新 filter helper，但不在本轮强行改压缩 payload 结构。

- [x] **Step 5: 运行定向测试**

运行：`go test -count=1 ./internal/sstable -run 'TestUnmarshalValueBlockWithTimestamps|TestReadBoolSamplesDirect|TestWritePartWithCompressionOptionsRoundTrips' -timeout 180s`

验收：测试通过，损坏 payload 仍返回错误。

实现备注：`unmarshalValueBlockWithTimestamps` 对 aligned v3 page 新增直接样本填充路径，读取 writeSeq 时创建目标样本，读取 values 时原地填值；bool 直接按 bit 解码，避免 `[]bool` 中间数组。已运行 `go test -count=1 ./internal/sstable -run 'TestUnmarshalValueBlockWithTimestampsStreamsAlignedFloatSamples|TestReadBoolSamplesDirect|TestUnmarshalValueBlockWithTimestamps|TestWritePartWithCompressionOptionsRoundTrips' -timeout 180s` 通过。

### Task 3: WAL v3 Dictionary Batch

**Files:**
- Modify: `internal/wal/encoding.go`
- Modify: `internal/wal/internal_test.go`
- Modify: `internal/wal/wal_test.go`

- [x] **Step 1: 写失败测试**

新增测试：
- v3 batch round-trip 后字段、tags、identity 与 v2 一致。
- 重复 identity/field name 的 v3 payload 小于 v2 payload。
- v2 payload 仍可 decode。
- v3 非法 identity_ref 或 field_name_ref 返回错误。

- [x] **Step 2: 运行失败测试**

运行：`go test -count=1 ./internal/wal -run 'TestWALBatchV3|TestDecodeBatchV2Compatibility' -timeout 180s`

预期：失败，因为 batch version 仍为 v2。

- [x] **Step 3: 实现 v3 编码**

新增 batch version 3，构造 batch 级 identity table 与 field name table，默认 `encodeBatch` 写 v3。

- [x] **Step 4: 实现 v2/v3 解码分派**

`decodeBatch` 根据首字节分派旧 v2 解码与新 v3 解码，保持 `ReplayRecords` 行为不变。

- [x] **Step 5: 运行定向测试**

运行：`go test -count=1 ./internal/wal -timeout 180s`

验收：全部 WAL 测试通过。

实现备注：`encodeBatch` 默认写 v3，batch 级 identity table 与 field name table 去重；`decodeBatch` 支持 v2/v3 分派，v3 引用越界和截断 payload 均返回错误。已运行 `go test -count=1 ./internal/wal -timeout 180s` 通过。

### Task 4: 集成验证与性能复测

**Files:**
- Modify: `docs/superpowers/plans/2026-06-16-storage-p1-performance.md`
- Optional Modify: `docs/benchmarks/storage-engine-benchmark-guide.md`

- [x] **Step 1: 格式化**

运行：`goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .`

- [x] **Step 2: 定向回归**

运行：`go test -count=1 ./internal/wal ./internal/sstable ./internal/engine -timeout 180s`

- [x] **Step 3: 全量测试与覆盖率**

运行：`go test -count=1 ./... -coverprofile=coverage.out -timeout 600s`

验收：总覆盖率 `>=90.0%`。

- [x] **Step 4: Lint**

运行：`golangci-lint run --timeout 12m`

- [x] **Step 5: pprof smoke**

构建并运行 100K wide10 write/query/compact smoke，记录耗时、RSS/heap、SSTable 数与落盘大小。

- [x] **Step 6: 清理临时产物**

删除 `coverage.out`、临时 pprof 二进制和本轮 `/tmp` profile 文件。

实现备注：已执行 `goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .`；`go test -count=1 ./internal/wal ./internal/sstable ./internal/engine -timeout 180s` 通过；`go test -count=1 ./... -coverprofile=coverage.out -timeout 600s` 通过，总覆盖率 `90.0%`；`golangci-lint run --timeout 12m` 输出 `0 issues.`；逐个 build/run `tests/e2e/*` 全部通过并清理二进制。100K wide10 pprof smoke：write `sstable_count=1 data_dir_bytes=8622275 heap_alloc_bytes=349786552 heap_sys_bytes=439418880`；query `sstable_count=1 data_dir_bytes=8622275 heap_alloc_bytes=375349736 heap_sys_bytes=426803200`；compact `sstable_count=1 data_dir_bytes=8622277 heap_alloc_bytes=180416440 heap_sys_bytes=359694336`。已删除 `/tmp/mts-p1-smoke` 和 `coverage.out`。

## 自检

- Spec coverage：三项 P1 优化分别对应 Task 1、Task 2、Task 3，Task 4 覆盖验证。
- Placeholder scan：无 TBD/TODO/后续增强占位。
- Type consistency：新增函数限定在 engine/sstable/wal 内部，不改变公开 API。
