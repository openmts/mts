# Storage Engine Phase 4 MemTable Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace MemTable's per-sample timestamp map with append-only column buffers to reduce wide write allocation and GC pressure while preserving query, flush and restore semantics.

**Architecture:** Keep WAL, SSTable and public engine APIs unchanged. MemTable writes append to `(seriesID, fieldID)` buffers; snapshot output performs LWW compaction and sorting for query/flush.

**Tech Stack:** Go 1.26.2, standard library only, `go test`, `pprof`, `goimports-reviser`, `gofmt`, `golangci-lint`.

---

## Constraints And Budgets

- 预计耗时：4-8 小时；硬超时：16 小时。
- 单包测试超时：180s；全量测试超时：600s；benchmark 超时：900s；百万级 pprof smoke 超时：1200s；lint 超时：12m。
- 不改变公开 API，不改变 WAL/SSTable 二进制格式。
- 新增目录权限保持 `0700`，新增文件权限保持 `0600`。
- 禁止在 for 循环中使用 defer。
- 每个行为变更先写失败测试，再实现。
- 每完成一个 task，更新本计划 checkbox 和实现备注。

## File Structure

- Modify: `internal/memtable/memtable.go` - replace table data layout and add snapshot column output.
- Modify: `internal/memtable/memtable_test.go` - cover append-buffer LWW, snapshot, restore and sorted columns.
- Modify: `internal/engine/shard.go` - use snapshot column output during flush.
- Modify: `internal/engine/engine_test.go` - verify flush/SSTable LWW after duplicate timestamp writes.
- Create: `docs/benchmarks/storage-engine-phase4.md` - record Phase 4 benchmark and pprof results.

## Task 1: MemTable Append Buffer Tests

**Files:**
- Modify: `internal/memtable/memtable_test.go`

- [x] **Step 1: Add failing tests**

Add tests for:

```go
func TestAppendBufferKeepsLatestWriteSeq(t *testing.T) {
    mt := memtable.New()
    points := []model.ResolvedPoint{
        resolvedPoint(1, 10, 1, model.Float64Value(1)),
        resolvedPoint(1, 10, 3, model.Float64Value(3)),
        resolvedPoint(1, 10, 2, model.Float64Value(2)),
    }
    if err := mt.ApplyBatch(points); err != nil {
        t.Fatalf("ApplyBatch() error = %v", err)
    }
    got := mt.Query(memtable.Query{Start: 0, End: 20})
    if len(got) != 1 || len(got[0].Samples) != 1 {
        t.Fatalf("Query() = %#v, want one compacted sample", got)
    }
    if got[0].Samples[0].Value.Float64 != 3 {
        t.Fatalf("latest value = %v, want 3", got[0].Samples[0].Value.Float64)
    }
}

func TestSnapshotColumnsAreSortedAndCompacted(t *testing.T) {
    mt := memtable.New()
    points := []model.ResolvedPoint{
        resolvedPoint(2, 20, 1, model.Int64Value(20)),
        resolvedPoint(1, 10, 1, model.Float64Value(1)),
        resolvedPoint(1, 10, 2, model.Float64Value(2)),
        resolvedPoint(1, 5, 3, model.Float64Value(5)),
    }
    if err := mt.ApplyBatch(points); err != nil {
        t.Fatalf("ApplyBatch() error = %v", err)
    }
    got := mt.Snapshot().Columns(memtable.Query{Start: 0, End: 30})
    if len(got) != 2 {
        t.Fatalf("Columns() len = %d, want 2", len(got))
    }
    if got[0].SeriesID != 1 || got[1].SeriesID != 2 {
        t.Fatalf("Columns() order = %#v", got)
    }
    samples := got[0].Samples
    if len(samples) != 2 || samples[0].Timestamp != 5 || samples[1].Timestamp != 10 {
        t.Fatalf("Samples order = %#v, want timestamps 5,10", samples)
    }
    if samples[1].Value.Float64 != 2 {
        t.Fatalf("LWW value = %v, want 2", samples[1].Value.Float64)
    }
}
```

- [x] **Step 2: Run red test**

Run:

```bash
go test ./internal/memtable -run 'TestAppendBufferKeepsLatestWriteSeq|TestSnapshotColumnsAreSortedAndCompacted' -timeout 180s
```

Expected: FAIL because `Snapshot.Columns` is not defined or current SampleCount semantics differ from append-buffer design.

实现备注：已新增 `TestAppendBufferKeepsLatestWriteSeq` 和 `TestSnapshotColumnsAreSortedAndCompacted`。执行 `go test ./internal/memtable -run 'TestAppendBufferKeepsLatestWriteSeq|TestSnapshotColumnsAreSortedAndCompacted' -timeout 180s` 按预期失败，错误为 `Snapshot.Columns undefined`。

## Task 2: Implement Append Buffer MemTable

**Files:**
- Modify: `internal/memtable/memtable.go`

- [x] **Step 1: Replace internal table data**

Implement:

```go
type columnKey struct {
    seriesID uint64
    fieldID  uint32
}

type columnBuffer struct {
    seriesID  uint64
    fieldID   uint32
    fieldType model.FieldType
    samples   []model.VersionedSample
}

type tableData map[columnKey]*columnBuffer
```

- [x] **Step 2: Change apply and restore to append**

`applyField` appends one `VersionedSample` and returns no LWW boolean. `Apply` and `ApplyBatch` increment `sampleCount` for every appended field sample. `Restore` appends snapshot column samples into current buffers and increments `sampleCount` by restored sample slice lengths.

- [x] **Step 3: Add snapshot column output**

Add:

```go
func (s *Snapshot) Columns(query Query) []model.ColumnData
```

It filters columns, compacts each column by timestamp using largest `WriteSeq`, sorts samples by timestamp, and sorts columns by `SeriesID` then `FieldID`.

- [x] **Step 4: Run memtable tests**

Run:

```bash
go test ./internal/memtable -timeout 180s
```

Expected: PASS.

实现备注：已将 MemTable 内部结构改为 `(seriesID, fieldID)` 到 `columnBuffer` 的 append-only map，`Snapshot.Columns` 负责过滤、LWW 压实和排序。`SampleCount` 调整为追加样本数，用于更保守地触发 flush。执行 `go test ./internal/memtable -timeout 180s` 通过。

## Task 3: Flush Uses Snapshot Columns

**Files:**
- Modify: `internal/engine/shard.go`
- Modify: `internal/engine/engine_test.go`

- [x] **Step 1: Add engine flush LWW test**

Add a test that writes duplicate timestamp values with increasing `WriteSeq`, forces flush, queries back from SSTable, and asserts only the latest value is visible.

- [x] **Step 2: Run red/target test**

Run:

```bash
go test ./internal/engine -run 'TestShardFlush|TestEngine' -timeout 180s
```

Expected: PASS or fail only until `Snapshot.Columns` is wired. This test protects flush semantics.

- [x] **Step 3: Update flush path**

In `Shard.flushLocked`, replace `snapshot.Query(...)` with `snapshot.Columns(...)`.

- [x] **Step 4: Run engine tests**

Run:

```bash
go test ./internal/engine -timeout 180s
```

Expected: PASS.

实现备注：已新增 `TestEngineFlushPersistsLatestWriteSeq`，覆盖重复 timestamp 写入后 flush 到 SSTable 的 LWW 语义。`Shard.flushLocked` 已改为直接调用 `snapshot.Columns`。执行 `go test ./internal/engine -run 'TestEngineFlushPersistsLatestWriteSeq|TestShard|TestEngine' -timeout 180s` 通过。

## Task 4: Benchmark And Profile Evidence

**Files:**
- Create: `docs/benchmarks/storage-engine-phase4.md`

- [x] **Step 1: Run benchmark**

Run:

```bash
go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s
```

Expected: benchmark output includes `BenchmarkEngineWriteWideBatch`.

- [x] **Step 2: Run pprof smoke**

Run:

```bash
timeout 1200s go run ./tests/pprof/storage_engine -mode=write -field-layout=wide10 -points 1000000 -series 10000 -mem-profile /tmp/mts-wide10-phase4-mem.prof
go tool pprof -inuse_space -top /tmp/mts-wide10-phase4-mem.prof
go tool pprof -alloc_space -top /tmp/mts-wide10-phase4-mem.prof
rm -f /tmp/mts-wide10-phase4-mem.prof
```

Expected: workload completes and memory profile is captured.

- [x] **Step 3: Record results**

Write Phase 4 benchmark and pprof summary to `docs/benchmarks/storage-engine-phase4.md`, including comparison against Phase 3 wide10 `~63ms/op`, `~87.8MB/op`, `~181.2k allocs/op`.

实现备注：已执行 `go test ./internal/bench -bench=. -benchmem -count=3 -timeout 900s`，wide10 10K 约 `46.7ms/op`、`66.4MB/op`、`167.6k allocs/op`。已执行 1M wide10 pprof smoke，写入约 `19.5s`，in-use heap 约 `10.8MB`，alloc_space 约 `9.25GB`。结果已记录到 `docs/benchmarks/storage-engine-phase4.md`。

## Task 5: Final Verification

**Files:**
- Modify as needed: touched Go files and docs

- [x] **Step 1: Format**

Run:

```bash
goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .
gofmt -w internal/memtable/memtable.go internal/memtable/memtable_test.go internal/engine/shard.go internal/engine/engine_test.go internal/bench/storage_bench_test.go tests/pprof/storage_engine/main.go tests/pprof/storage_engine/main_test.go
```

- [x] **Step 2: Full tests and coverage**

Run:

```bash
go test ./... -coverprofile=coverage.out -timeout 600s
go tool cover -func=coverage.out | tail -1
```

Expected: total coverage `>=90%`.

- [x] **Step 3: Lint**

Run:

```bash
golangci-lint run --timeout 12m
```

Expected: no issues.

- [x] **Step 4: E2E**

For each subdirectory under `tests/e2e`, run:

```bash
go build
./<test_binary>
rm -f ./<test_binary>
```

Expected: all e2e binaries exit successfully and temporary binaries are removed.

- [x] **Step 5: Clean temporary artifacts**

Run:

```bash
rm -f coverage.out /tmp/mts-wide10-phase4-mem.prof
find tests/e2e -maxdepth 2 -type f -perm /111 -delete
```

Expected: no coverage/profile/e2e binary artifacts remain.

实现备注：已执行 `goimports-reviser -project-name github.com/openmts/mts -recursive -format -rm-unused .`、`gofmt`、`go test ./... -coverprofile=coverage.out -timeout 600s`、`go tool cover -func=coverage.out | tail -1`，总覆盖率 `90.0%`。`golangci-lint run --timeout 12m` 输出 `0 issues.`。`tests/e2e` 下 7 个用例均已 build/run 通过，临时二进制、`coverage.out` 和 `/tmp/mts-wide10-phase4-mem.prof` 已清理。
