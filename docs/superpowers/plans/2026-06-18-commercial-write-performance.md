# Commercial Write Performance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 10M wide10 写入链路从高分配、高 RSS 的对象化路径推进到可验证的商业级写入基础。

**Architecture:** 本计划按“先测量、再消除已证实热点、最后固化回归门禁”的顺序执行。第一批执行聚焦 O(1) MemTable 内存计数和 SSTable 校验 syscall 缓存，后续批次再推进 streaming flush、typed batch 和写入流水线。

**Tech Stack:** Go、pprof、`internal/memtable`、`internal/sstable`、`tests/pprof/storage_engine`、`go test`、`goimports-reviser`、`golangci-lint`。

---

## File Map

- Modify: `internal/memtable/memtable.go`
  - 为 MemTable、Snapshot、columnBuffer 增加增量内存计数。
  - 保留 `approxTableDataBytes` 作为测试/校验辅助，不再用于热路径 `ApproxMemoryBytes`。
- Modify: `internal/memtable/internal_test.go`
  - 增加 O(1) 计数防回归测试。
  - 覆盖 reserve、append、snapshot、restore、release 的字节计数语义。
- Modify: `internal/memtable/memtable_test.go`
  - 调整现有近似内存测试，要求 snapshot 字节数与 active 转移一致。
- Modify: `internal/sstable/types.go`
  - 在 `Part` 中保存组件文件大小缓存。
- Modify: `internal/sstable/read.go`
  - 打开 part 时初始化组件文件大小缓存。
- Modify: `internal/sstable/validate.go`
  - 将每个 block ref 的重复 `Stat` 改为基于缓存文件大小校验。
- Modify: `internal/sstable/internal_test.go` and `internal/sstable/sstable_test.go`
  - 覆盖缓存校验仍能发现越界、缺失、截断和损坏。
- Modify: `docs/superpowers/specs/2026-06-18-commercial-write-performance-design.md`
  - 更新已执行任务状态和实测备注。
- Modify: `docs/benchmarks/storage-engine-10m-pprof-2026-06-18.md`
  - 若本轮运行性能用例，则追加 before/after 数据。

## Task 1: Baseline And Guard Rails

- [x] Step 1: Read current pprof report and confirm the baseline bottlenecks.
  - Command: `sed -n '1,220p' docs/benchmarks/storage-engine-10m-pprof-2026-06-18.md`
  - Expected: report contains 10M write duration, RSS peak, total alloc, CPU and alloc hot paths.
- [x] Step 2: Keep the benchmark report unmodified until code verification has fresh output.
  - Expected: no benchmark claim is made without a fresh command result.
  - Implementation note: benchmark report remains unchanged in this implementation batch.

## Task 2: MemTable O(1) Memory Accounting

- [x] Step 1: Add a regression test proving `ApproxMemoryBytes` does not call full table scanning.
  - File: `internal/memtable/internal_test.go`
  - Expected command: `go test -run 'TestMemTableApproxMemoryBytesAvoidsFullScan' ./internal/memtable -count=1 -timeout 120s`
  - Expected RED: test fails before implementation because `ApproxMemoryBytes` still calls `approxTableDataBytes`.
  - Implementation note: RED verified with `ApproxMemoryBytes() full scans = 1, want 0`.
- [x] Step 2: Add incremental byte fields and update paths.
  - File: `internal/memtable/memtable.go`
  - Required behavior:
    - `New()` starts at table base bytes.
    - New columns add map-entry bytes and column struct bytes.
    - Slice growth adds only capacity deltas.
    - String values add payload length.
    - Snapshot transfers active bytes without scanning.
    - Restore merges bytes consistently.
  - Implementation note: `MemTable`、`Snapshot`、`columnBuffer` now track bytes incrementally; full scan remains only as test helper.
- [x] Step 3: Run focused tests.
  - Command: `go test -run 'TestMemTableApproxMemoryBytes|TestMemTableStoreAdapterSnapshotAndRestore|TestShardFlushFailureRestoresSnapshot' ./internal/memtable ./internal/engine -count=1 -timeout 180s`
  - Expected: all selected tests pass.
  - Implementation note: ran `go test ./internal/memtable -count=1 -timeout 120s` and `go test ./internal/memtable ./internal/sstable ./internal/engine -count=1 -timeout 300s`.

## Task 3: SSTable Validation Stat Reduction

- [x] Step 1: Add or update tests proving cached file sizes still catch invalid refs.
  - Files: `internal/sstable/internal_test.go`, `internal/sstable/sstable_test.go`
  - Expected command: `go test -run 'TestOpenPart|TestValidate' ./internal/sstable -count=1 -timeout 180s`
  - Implementation note: added `TestValidateBlockRefWithinSize`; existing missing component, out-of-bounds metadata and checksum corruption tests were rerun.
- [x] Step 2: Cache component file sizes at open and reuse them for block ref checks.
  - Files: `internal/sstable/types.go`, `internal/sstable/read.go`, `internal/sstable/validate.go`
  - Required behavior:
    - Component presence and directory checks still run once.
    - Metadata/index/value/time refs use cached sizes.
    - Missing size entries return explicit errors.
    - Corruption and checksum detection remains active.
  - Implementation note: `Part` now stores `componentSizes`; validation uses `validatePartBlockRef` and `validateBlockRefWithinSize`.
- [x] Step 3: Run focused SSTable tests.
  - Command: `go test ./internal/sstable -count=1 -timeout 240s`
  - Expected: all SSTable tests pass.
  - Implementation note: ran targeted corruption tests and package regression through combined package test.

## Task 4: Formatting, Lint And Performance Smoke

- [x] Step 1: Run format tools.
  - Command: `timeout 300s goimports-reviser -rm-unused -set-alias -format ./...`
  - Expected: command exits 0, or if unavailable, record the missing tool explicitly.
  - Implementation note: command exited 0.
- [x] Step 2: Run targeted package tests.
  - Command: `go test ./internal/memtable ./internal/sstable ./internal/engine -count=1 -timeout 300s`
  - Expected: all selected packages pass.
  - Implementation note: command passed for all three packages.
- [x] Step 3: Run lightweight performance smoke.
  - Command: `go test ./tests/pprof/storage_engine -run TestStorageEnginePprof -count=1 -timeout 20m`
  - Expected: command either completes with a fresh report or failure output is recorded for follow-up.
  - Implementation note: the named test does not exist; ran `go test ./tests/pprof/storage_engine -count=1 -timeout 300s`, plus no-profile scale runs for 1M and 10M write.
- [x] Step 4: Run lint if code changes compile.
  - Command: `timeout 720s golangci-lint run ./...`
  - Expected: command exits 0, or findings are fixed/recorded.
  - Implementation note: command exited 0 with `0 issues`.

## Task 5: Update Design Status

- [x] Step 1: Mark completed EARS items in the design document.
  - File: `docs/superpowers/specs/2026-06-18-commercial-write-performance-design.md`
  - Expected: Task 2 and Task 8 include implementation notes and verification command references.
- [x] Step 2: Leave unimplemented commercial tasks explicit.
  - Expected: streaming flush、series-wide MemTable、typed ingestion、WAL typed batch、adaptive backpressure、pipeline remain visible as pending implementation tasks instead of being silently claimed complete.
  - Implementation note:本轮仅闭环 Task 2 和 Task 8 的第一阶段实现，未把后续结构性任务标记为完成。

## Task 6: Typed Columnar Write Path

- [x] Step 1: Add typed resolved batch model and Catalog resolver.
  - Files: `internal/model/types.go`, `internal/catalog/typed_batch.go`
  - Expected: typed resolver returns `seriesIDs` and resolved field columns without per-row field arena.
  - Implementation note: added `ResolvedTypedBatch` and `ResolveTypedBatchColumns`; test `TestCatalogResolveTypedBatchColumnsBorrowsValues` verifies value slices are borrowed.
- [x] Step 2: Add WAL typed encoder while keeping replay compatibility.
  - Files: `internal/wal/encoding.go`, `internal/wal/wal.go`
  - Expected: typed WAL payload decodes through existing `decodeBatch`.
  - Implementation note: `AppendTyped` encodes identities by seriesID and field refs by column order; test `TestEncodeTypedBatchIntoDecodesAsResolvedPoints` covers compatibility.
- [x] Step 3: Apply typed batch directly to MemTable.
  - Files: `internal/memtable/memtable.go`, `internal/engine/shard.go`, `internal/engine/engine.go`
  - Expected: engine typed path avoids `[]ResolvedPoint` materialization.
  - Implementation note: `WriteTypedBatch` now writes `ResolvedTypedBatch` by shard row index; `TestApplyTypedBatchWritesColumnValues` covers MemTable values.

## Task 7: Hot Allocation Reduction

- [x] Step 1: Reduce Catalog snapshot clone amplification.
  - Files: `internal/catalog/persist.go`
  - Expected: checkpoint threshold scales with catalog size while `Close` still forces snapshot.
  - Implementation note: `TestCatalogCheckpointThresholdScalesWithSeriesCount` covers the adaptive threshold.
- [x] Step 2: Reuse SSTable index row payload buffers.
  - Files: `internal/sstable/metadata_encoding.go`, `internal/sstable/write.go`
  - Expected: per-row index encoding reuses caller buffer.
  - Implementation note: `TestEncodeIndexRowsIntoReusesDestination` covers buffer reuse and decode compatibility.
- [x] Step 3: Replace GC-sensitive pools and reflective hot sorting.
  - Files: `internal/memtable/pool.go`, `internal/memtable/memtable.go`, `internal/sstable/write.go`
  - Expected: MemTable table/key/column temporary structures are bounded and reusable; hot sorting avoids `sort.Slice` reflection.
  - Implementation note: added bounded freelists for `tableData`、`columnBuffer`、`columnKey`; sorting uses `slices.SortFunc`.

## Task 8: Performance Verification

- [x] Step 1: Run 1M high-cardinality pprof.
  - Command: `go run ./tests/pprof/storage_engine -mode=write -field-layout=wide10 -points=1000000 -series=100000 -write-batch-size=4096 -memtable-max-samples=8192 -flush-on-exit -ingest-path=typed`
  - Result: workload `24.820s`, RSS peak `366,682,112`, total alloc `2,678,244,272`.
- [x] Step 2: Run 10M scale gate.
  - Command: `go run ./tests/scale/storage_10m -profile soak -mode write -batch-size 4096 -ingest-path typed`
  - Result: workload `22.997944130s`, throughput `434,821 points/s`, RSS peak `83,591,168`, total alloc `8,391,999,880`.
- [x] Step 3: Update benchmark report.
  - File: `docs/benchmarks/storage-engine-10m-pprof-2026-06-18.md`
  - Implementation note: recorded batch 2 comparison against prior typed result.
