# Storage Next Hotpath Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 全量完成下一轮存储热路径优化，减少 SSTable、WAL、Catalog 和 benchmark gate 中的临时分配与治理缺口。

**Architecture:** 不改变公开 API 和磁盘格式。SSTable 增加 typed streaming 解码和 samples 直接编码入口；WAL/Catalog 使用局部 scratch/arena 降低 hot path 分配；benchmark gate 增加显式 baseline 更新模式。

**Tech Stack:** Go、现有 `internal/sstable`、`internal/wal`、`internal/catalog`、`scripts/storage_benchmark_gate.sh`、`go test`、`goimports-reviser`、`golangci-lint`。

---

### Task 1: SSTable Compressed Codec Streaming

**Files:**
- Modify: `internal/sstable/compression.go`
- Modify: `internal/sstable/compression_time.go`
- Modify: `internal/sstable/compression_values.go`
- Modify: `internal/sstable/compression_test.go`

- [x] **Step 1: 增加非 plain compressed streaming 测试**
  - 覆盖 float XOR、int delta、string dictionary 查询小范围时不构造完整 values slice，并验证结果正确。
- [x] **Step 2: 运行红测**
  - Run: `go test -count=1 ./internal/sstable -run 'TestCompressedCodecStreaming' -timeout 180s`
- [x] **Step 3: 实现 typed streaming decoder**
  - 新增 `readXORFloatSampleValues`、`readDeltaIntSampleValues`、`readDictionaryStringSampleValues`，由 `readCodecPayloadSamples` 调用。
- [x] **Step 4: 优化 compressed page 写入 metadata**
  - 新增 samples 入口编码 timestamp/writeSeq，替换 `splitSampleMetadata`。
- [x] **Step 5: 运行定向测试**
  - Run: `go test -count=1 ./internal/sstable -run 'TestCompressed|TestValuePage' -timeout 180s`
  - Result: 通过；compressed delta int 解码降至 `<=3` alloc/run，compressed page 写入复用 dst 后降至 `<=7` alloc/run。

### Task 2: WAL Batch Scratch And Arena

**Files:**
- Modify: `internal/wal/encoding.go`
- Modify: `internal/wal/internal_test.go`

- [x] **Step 1: 增加 WAL batch 分配测试**
  - 使用重复 identity 和 wide fields，约束 batch 编码分配不随 point 数线性增加到 refs 切片级别。
- [x] **Step 2: 运行红测**
  - Run: `go test -count=1 ./internal/wal -run 'TestEncodeBatchUsesScratchRefsArena' -timeout 180s`
- [x] **Step 3: 实现 identity scratch key**
  - `batchIdentities` 使用复用 `[]byte` 构造 map key。
- [x] **Step 4: 实现 field refs arena**
  - `batchFieldNames` 预估 field 总数并使用连续 arena 切分 refs。
- [x] **Step 5: 运行 WAL 测试**
  - Run: `go test -count=1 ./internal/wal -timeout 180s`
  - Result: 通过；`TestEncodeBatchUsesScratchRefsArena` 由约 140 alloc/run 降至门槛内。

### Task 3: Catalog Single-Point Multi-Tag Scratch

**Files:**
- Modify: `internal/catalog/catalog.go`
- Modify: `internal/catalog/resolve.go`
- Modify: `internal/catalog/internal_test.go`

- [x] **Step 1: 增加单点多 tag allocation 测试**
  - 验证 `resolveSeriesNoSnapshotLocked` 多 tag 路径复用 Catalog scratch。
- [x] **Step 2: 运行红测**
  - Run: `go test -count=1 ./internal/catalog -run 'TestResolveSeriesSinglePointMultiTagUsesScratch' -timeout 180s`
- [x] **Step 3: Catalog 增加 key scratch**
  - 在 `Catalog` 增加 `seriesKeyScratch []string`，锁内复用。
- [x] **Step 4: 单点 resolve 接入 scratch**
  - 多 tag 单点路径使用 `seriesKeyWithScratch`。
- [x] **Step 5: 运行 Catalog 测试**
  - Run: `go test -count=1 ./internal/catalog -timeout 180s`
  - Result: 通过；单点多 tag resolve 从约 4 alloc/run 降至 `<=1` alloc/run。

### Task 4: Benchmark Gate Baseline Mode

**Files:**
- Modify: `scripts/storage_benchmark_gate.sh`
- Modify: `docs/benchmarks/storage-engine-benchmark-guide.md`

- [x] **Step 1: 增加脚本参数验证**
  - 支持 `--update-baseline`，默认不修改 baseline。
- [x] **Step 2: 实现原子 baseline 更新**
  - 使用临时文件和 `mv` 写入 baseline，目录权限保持安全。
- [x] **Step 3: 文档补充**
  - 说明首次建立 baseline 和后续对比命令。
- [x] **Step 4: 脚本 smoke**
  - Run: `timeout 180s bash -n scripts/storage_benchmark_gate.sh`
  - Result: 通过；`--help` smoke 通过。

### Task 5: Integration Verification

**Files:**
- Modify: `docs/superpowers/plans/2026-06-17-storage-next-hotpath.md`

- [x] **Step 1: goimports-reviser**
  - Run: `timeout 300s goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .`
  - Result: 通过。
- [x] **Step 2: 核心包测试**
  - Run: `go test -count=1 ./internal/sstable ./internal/wal ./internal/catalog ./internal/bench ./tests/pprof/storage_engine -timeout 180s`
  - Result: 通过。
- [x] **Step 3: 全量测试与覆盖率**
  - Run: `go test -count=1 ./... -coverprofile=coverage.out -timeout 600s`
  - Run: `go tool cover -func=coverage.out | tail -1`
  - Result: 通过；总覆盖率 `90.0%`，达到仓库门槛。
- [x] **Step 4: lint**
  - Run: `golangci-lint run --timeout 12m`
  - Result: 通过；`0 issues`。
- [x] **Step 5: e2e**
  - 逐个 build/run `tests/e2e/*`，每个用例运行后删除二进制。
  - Result: 通过；实际目录 `compaction_integrity`、`flush_manifest_recovery`、`no_json_storage`、`query_pruning`、`retention`、`simple_integrity`、`wal_recovery` 均已逐个 build/run。
- [x] **Step 6: 清理产物**
  - 删除 `coverage.out`、profile、e2e/test binary。
  - Result: 完成；最终确认无 e2e/test binary、profile、coverage 产物残留。

## 自检

- Spec coverage：覆盖全部 EARS 需求。
- Placeholder scan：无 TBD/TODO/后续增强占位。
- Type consistency：所有新增函数均为包内 helper，不改变公开 API。
